package account

import (
	"context"
	"fmt"
	"sync"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/shopspring/decimal"
)

// SpotOrderParams is the strongly-typed order parameters for spot markets.
// Notable absences vs PerpOrderParams: no ReduceOnly (spot has no positions
// to reduce), no Slippage flag (handled per-adapter if at all).
type SpotOrderParams struct {
	Symbol      string
	Side        exchanges.OrderSide
	Type        exchanges.OrderType
	Quantity    decimal.Decimal
	Price       decimal.Decimal
	TimeInForce exchanges.TimeInForce
	PostOnly    bool
	ClientID    string
}

func (p *SpotOrderParams) toGeneric() *exchanges.OrderParams {
	if p == nil {
		return nil
	}
	return &exchanges.OrderParams{
		Symbol:      p.Symbol,
		Side:        p.Side,
		Type:        p.Type,
		Quantity:    p.Quantity,
		Price:       p.Price,
		TimeInForce: p.TimeInForce,
		PostOnly:    p.PostOnly,
		ClientID:    p.ClientID,
	}
}

// SpotTradingAccount is the trading-account runtime for spot markets.
// It tracks per-asset balances rather than positions, and intentionally
// does not expose Leverage / Position / WatchPositions semantics.
type SpotTradingAccount struct {
	baseTradeClient

	spotAdp exchanges.SpotExchange

	balMu      sync.RWMutex
	balances   map[string]exchanges.SpotBalance // asset → balance
	balanceBus *eventBus[exchanges.SpotBalance]
}

func NewSpotTradingAccount(adp exchanges.SpotExchange, logger exchanges.Logger) *SpotTradingAccount {
	return &SpotTradingAccount{
		baseTradeClient: newBaseTradeClient(adp, logger),
		spotAdp:         adp,
		balances:        make(map[string]exchanges.SpotBalance),
		balanceBus:      newEventBus[exchanges.SpotBalance](),
	}
}

// =============================================================================
// Strongly-typed Place / transfer forwarding
// =============================================================================

func (a *SpotTradingAccount) Place(ctx context.Context, params *SpotOrderParams) (*OrderFlow, error) {
	return a.placeGeneric(ctx, params.toGeneric())
}

func (a *SpotTradingAccount) PlaceWS(ctx context.Context, params *SpotOrderParams) (*OrderFlow, error) {
	return a.placeGenericWS(ctx, params.toGeneric())
}

func (a *SpotTradingAccount) Transfer(ctx context.Context, params *exchanges.TransferParams) error {
	return a.spotAdp.TransferAsset(ctx, params)
}

// =============================================================================
// State access
// =============================================================================

func (a *SpotTradingAccount) Balance(asset string) (exchanges.SpotBalance, bool) {
	a.balMu.RLock()
	defer a.balMu.RUnlock()
	b, ok := a.balances[asset]
	return b, ok
}

func (a *SpotTradingAccount) Balances() []exchanges.SpotBalance {
	a.balMu.RLock()
	defer a.balMu.RUnlock()
	out := make([]exchanges.SpotBalance, 0, len(a.balances))
	for _, b := range a.balances {
		out = append(out, b)
	}
	return out
}

func (a *SpotTradingAccount) SubscribeBalances() *Subscription[exchanges.SpotBalance] {
	return a.balanceBus.Subscribe()
}

func (a *SpotTradingAccount) Health() TradingAccountHealth {
	return a.healthSnapshot()
}

// =============================================================================
// Start / Close
// =============================================================================

func (a *SpotTradingAccount) Start(ctx context.Context) (err error) {
	a.lifecycleMu.Lock()
	defer a.lifecycleMu.Unlock()

	a.runMu.RLock()
	alreadyStarted := a.started
	closing := a.closing
	a.runMu.RUnlock()
	if closing {
		return context.Canceled
	}
	if alreadyStarted {
		return nil
	}

	runCtx, runCancel := context.WithCancel(ctx)
	runGen, ok := a.beginRun(runCancel)
	if !ok {
		runCancel()
		return context.Canceled
	}
	defer func() {
		if err == nil {
			return
		}
		if cancel := a.failRunStart(runGen); cancel != nil {
			cancel()
		} else {
			runCancel()
		}
		a.resetState()
	}()

	a.logger.Infow("spot_account: fetching initial account state")
	acc, err := a.spotAdp.FetchAccount(runCtx)
	if err != nil {
		return fmt.Errorf("spot_account: failed to get initial state: %w", err)
	}
	a.applyAccountSnapshot(runGen, acc)
	if err := a.refreshBalances(runCtx, runGen); err != nil {
		return fmt.Errorf("spot_account: failed to fetch balances: %w", err)
	}

	if err := a.startOrderAndFillStreams(runCtx, runGen); err != nil {
		return err
	}

	if runErr := runCtx.Err(); runErr != nil {
		return runErr
	}
	if !a.finishRunStart(runGen) {
		return context.Canceled
	}

	go a.periodicRefresh(runCtx, time.Minute, runGen, func(acc *exchanges.Account) {
		a.applyAccountSnapshot(runGen, acc)
		// SpotBalances is a separate REST endpoint; refresh it on the same cadence.
		if err := a.refreshBalances(runCtx, runGen); err != nil {
			if runCtx.Err() == nil && a.isActiveRun(runGen) {
				a.logger.Errorw("spot_account: periodic balance refresh failed", "error", err)
			}
		}
	})
	return nil
}

func (a *SpotTradingAccount) Close() {
	if cancel := a.closeRun(); cancel != nil {
		cancel()
	}

	a.lifecycleMu.Lock()
	defer a.lifecycleMu.Unlock()

	a.resetState()
	a.closeBaseStreams()
	a.balanceBus.Close()
	a.clearClosingFlag()
}

// =============================================================================
// Snapshot / update internals
// =============================================================================

func (a *SpotTradingAccount) applyAccountSnapshot(runGen uint64, acc *exchanges.Account) {
	if !a.isActiveRun(runGen) {
		return
	}
	if acc == nil {
		acc = &exchanges.Account{}
	}
	a.applyOrderSnapshot(runGen, acc.Orders)
	a.markSnapshotLoaded()
}

func (a *SpotTradingAccount) refreshBalances(ctx context.Context, runGen uint64) error {
	balances, err := a.spotAdp.FetchSpotBalances(ctx)
	if err != nil {
		return err
	}
	if !a.isActiveRun(runGen) {
		return nil
	}

	next := make(map[string]exchanges.SpotBalance, len(balances))
	for _, b := range balances {
		next[b.Asset] = b
	}

	a.balMu.Lock()
	if !a.isActiveRun(runGen) {
		a.balMu.Unlock()
		return nil
	}
	prev := a.balances
	a.balances = next
	a.balMu.Unlock()

	for asset, b := range next {
		if old, ok := prev[asset]; !ok || !old.Total.Equal(b.Total) || !old.Free.Equal(b.Free) {
			cp := b
			dropped := a.balanceBus.Publish(&cp)
			a.markStreamEvent(StreamBalances, dropped)
		}
	}
	return nil
}

func (a *SpotTradingAccount) resetState() {
	a.resetOrderCache()
	a.balMu.Lock()
	a.balances = make(map[string]exchanges.SpotBalance)
	a.balMu.Unlock()
}
