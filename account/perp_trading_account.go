package account

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/shopspring/decimal"
)

// PerpOrderParams is the strongly-typed order parameters for perpetual
// futures markets. Leverage is intentionally absent — it is a per-symbol
// setting set via PerpTradingAccount.SetLeverage, not a per-order flag.
type PerpOrderParams struct {
	Symbol      string
	Market      exchanges.MarketRef
	Side        exchanges.OrderSide
	Type        exchanges.OrderType
	Quantity    decimal.Decimal
	Price       decimal.Decimal
	TimeInForce exchanges.TimeInForce
	ReduceOnly  bool
	PostOnly    bool
	Slippage    decimal.Decimal
	ClientID    string
}

func (p *PerpOrderParams) toGeneric() *exchanges.OrderParams {
	if p == nil {
		return nil
	}
	return &exchanges.OrderParams{
		Symbol:      p.Symbol,
		Market:      p.Market,
		Side:        p.Side,
		Type:        p.Type,
		Quantity:    p.Quantity,
		Price:       p.Price,
		TimeInForce: p.TimeInForce,
		ReduceOnly:  p.ReduceOnly,
		PostOnly:    p.PostOnly,
		Slippage:    p.Slippage,
		ClientID:    p.ClientID,
	}
}

// PerpTradingAccount is the trading-account runtime for perpetual futures
// venues. It tracks positions, margin balance, and forwards leverage
// changes to the underlying adapter.
type PerpTradingAccount struct {
	baseTradeClient

	perpAdp exchanges.PerpExchange

	posMu       sync.RWMutex
	positions   map[string]*exchanges.Position
	positionBus *eventBus[exchanges.Position]
	balance     decimal.Decimal
}

func NewPerpTradingAccount(adp exchanges.PerpExchange, logger exchanges.Logger) *PerpTradingAccount {
	return &PerpTradingAccount{
		baseTradeClient: newBaseTradeClient(adp, logger),
		perpAdp:         adp,
		positions:       make(map[string]*exchanges.Position),
		positionBus:     newEventBus[exchanges.Position](),
	}
}

// =============================================================================
// Strongly-typed Place / leverage forwarding
// =============================================================================

func (a *PerpTradingAccount) Place(ctx context.Context, params *PerpOrderParams) (*OrderFlow, error) {
	return a.placeGeneric(ctx, params.toGeneric())
}

func (a *PerpTradingAccount) PlaceWS(ctx context.Context, params *PerpOrderParams) (*OrderFlow, error) {
	return a.placeGenericWS(ctx, params.toGeneric())
}

func (a *PerpTradingAccount) SetLeverage(ctx context.Context, symbol string, leverage int) error {
	return a.perpAdp.SetLeverage(ctx, symbol, leverage)
}

func (a *PerpTradingAccount) ModifyOrder(ctx context.Context, orderID, symbol string, params *exchanges.ModifyOrderParams) (*exchanges.Order, error) {
	return a.perpAdp.ModifyOrder(ctx, orderID, symbol, params)
}

func (a *PerpTradingAccount) ModifyOrderWS(ctx context.Context, orderID, symbol string, params *exchanges.ModifyOrderParams) error {
	return a.perpAdp.ModifyOrderWS(ctx, orderID, symbol, params)
}

// =============================================================================
// State access
// =============================================================================

func (a *PerpTradingAccount) Position(symbol string) (*exchanges.Position, bool) {
	a.posMu.RLock()
	defer a.posMu.RUnlock()
	p, ok := a.positions[symbol]
	if !ok {
		return nil, false
	}
	c := *p
	return &c, true
}

func (a *PerpTradingAccount) Positions() []exchanges.Position {
	a.posMu.RLock()
	defer a.posMu.RUnlock()
	out := make([]exchanges.Position, 0, len(a.positions))
	for _, p := range a.positions {
		out = append(out, *p)
	}
	return out
}

func (a *PerpTradingAccount) Balance() decimal.Decimal {
	a.posMu.RLock()
	defer a.posMu.RUnlock()
	return a.balance
}

func (a *PerpTradingAccount) SubscribePositions() *Subscription[exchanges.Position] {
	return a.positionBus.Subscribe()
}

// =============================================================================
// Start / Close
// =============================================================================

func (a *PerpTradingAccount) Start(ctx context.Context) (err error) {
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

	a.logger.Infow("perp_account: fetching initial account state")
	acc, err := a.perpAdp.FetchAccount(runCtx)
	if err != nil {
		return fmt.Errorf("perp_account: failed to get initial state: %w", err)
	}
	a.applySnapshot(runGen, acc)
	orderCount, posCount := 0, 0
	if acc != nil {
		orderCount = len(acc.Orders)
		posCount = len(acc.Positions)
	}
	a.logger.Infow("perp_account: initial state loaded", "orders", orderCount, "positions", posCount)

	if err := a.startOrderAndFillStreams(runCtx, runGen); err != nil {
		return err
	}

	if watchErr := a.perpAdp.WatchPositions(runCtx, func(p *exchanges.Position) {
		a.applyPositionUpdate(runGen, p)
	}); watchErr != nil {
		if !errors.Is(watchErr, exchanges.ErrNotSupported) {
			a.markStreamError(StreamPositions, watchErr)
			return fmt.Errorf("perp_account: WatchPositions failed: %w", watchErr)
		}
		a.markStreamUnsupported(StreamPositions, watchErr)
		a.logger.Warnw("perp_account: WatchPositions not supported", "error", watchErr)
	} else {
		a.markStreamReady(StreamPositions)
	}

	if runErr := runCtx.Err(); runErr != nil {
		return runErr
	}
	if !a.finishRunStart(runGen) {
		return context.Canceled
	}

	go a.periodicRefresh(runCtx, time.Minute, runGen, func(acc *exchanges.Account) {
		a.applySnapshot(runGen, acc)
	})
	return nil
}

func (a *PerpTradingAccount) Close() {
	if cancel := a.closeRun(); cancel != nil {
		cancel()
	}

	a.lifecycleMu.Lock()
	defer a.lifecycleMu.Unlock()

	a.resetState()
	a.closeBaseStreams()
	a.positionBus.Close()
	a.clearClosingFlag()
}

// =============================================================================
// Snapshot / update internals
// =============================================================================

func (a *PerpTradingAccount) applySnapshot(runGen uint64, acc *exchanges.Account) {
	if !a.isActiveRun(runGen) {
		return
	}
	if acc == nil {
		acc = &exchanges.Account{}
	}

	a.applyOrderSnapshot(runGen, acc.Orders)

	next := make(map[string]*exchanges.Position, len(acc.Positions))
	for _, p := range acc.Positions {
		c := p
		next[p.Symbol] = &c
	}

	a.posMu.Lock()
	if a.isActiveRun(runGen) {
		a.positions = next
		a.balance = acc.TotalBalance
	}
	a.posMu.Unlock()
	a.markSnapshotLoaded()
}

func (a *PerpTradingAccount) applyPositionUpdate(runGen uint64, p *exchanges.Position) {
	if p == nil || !a.isActiveRun(runGen) {
		return
	}
	a.posMu.Lock()
	if !a.isActiveRun(runGen) {
		a.posMu.Unlock()
		return
	}
	c := *p
	a.positions[p.Symbol] = &c
	a.posMu.Unlock()

	if !a.isActiveRun(runGen) {
		return
	}
	dropped := a.positionBus.Publish(p)
	a.markStreamEvent(StreamPositions, dropped)
}

func (a *PerpTradingAccount) Health() TradingAccountHealth {
	return a.healthSnapshot()
}

func (a *PerpTradingAccount) resetState() {
	a.resetOrderCache()
	a.posMu.Lock()
	a.positions = make(map[string]*exchanges.Position)
	a.balance = decimal.Zero
	a.posMu.Unlock()
}
