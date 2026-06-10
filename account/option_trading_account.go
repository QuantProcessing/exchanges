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

// OptionOrderParams is the strongly-typed order parameters for option markets.
//
// Instrument carries the full typed identity of the contract (underlying /
// expiry / strike / kind / settlement). Quantity is in contracts; Price is
// the premium quoted in the contract's settlement currency.
type OptionOrderParams struct {
	Instrument  *exchanges.OptionInstrument
	Side        exchanges.OrderSide
	Type        exchanges.OrderType
	Quantity    decimal.Decimal // number of contracts
	Price       decimal.Decimal // premium per contract, in settlement currency
	TimeInForce exchanges.TimeInForce
	ReduceOnly  bool
	PostOnly    bool
	ClientID    string
}

func (p *OptionOrderParams) toGeneric(symbol string) *exchanges.OrderParams {
	if p == nil {
		return nil
	}
	return &exchanges.OrderParams{
		Symbol:      symbol,
		Side:        p.Side,
		Type:        p.Type,
		Quantity:    p.Quantity,
		Price:       p.Price,
		TimeInForce: p.TimeInForce,
		ReduceOnly:  p.ReduceOnly,
		PostOnly:    p.PostOnly,
		ClientID:    p.ClientID,
	}
}

// OptionTradingAccount is the trading-account runtime for option markets.
// It tracks per-instrument positions, exposes a portfolio Greeks aggregator,
// and uses *OptionInstrument (not bare strings) as the typed instrument
// identity for placement.
type OptionTradingAccount struct {
	baseTradeClient

	optionAdp exchanges.OptionExchange

	posMu       sync.RWMutex
	positions   map[string]*exchanges.Position // key = venue-formatted instrument ID
	positionBus *eventBus[exchanges.Position]
}

func NewOptionTradingAccount(adp exchanges.OptionExchange, logger exchanges.Logger) *OptionTradingAccount {
	return &OptionTradingAccount{
		baseTradeClient: newBaseTradeClient(adp, logger),
		optionAdp:       adp,
		positions:       make(map[string]*exchanges.Position),
		positionBus:     newEventBus[exchanges.Position](),
	}
}

// =============================================================================
// Strongly-typed Place
// =============================================================================

func (a *OptionTradingAccount) Place(ctx context.Context, params *OptionOrderParams) (*OrderFlow, error) {
	if params == nil || params.Instrument == nil {
		return nil, fmt.Errorf("option_account: Place requires an instrument")
	}
	symbol := a.optionAdp.FormatInstrument(params.Instrument)
	if symbol == "" {
		return nil, fmt.Errorf("option_account: adapter returned empty instrument ID")
	}
	return a.placeGeneric(ctx, params.toGeneric(symbol))
}

func (a *OptionTradingAccount) PlaceWS(ctx context.Context, params *OptionOrderParams) (*OrderFlow, error) {
	if params == nil || params.Instrument == nil {
		return nil, fmt.Errorf("option_account: PlaceWS requires an instrument")
	}
	symbol := a.optionAdp.FormatInstrument(params.Instrument)
	if symbol == "" {
		return nil, fmt.Errorf("option_account: adapter returned empty instrument ID")
	}
	return a.placeGenericWS(ctx, params.toGeneric(symbol))
}

// =============================================================================
// State access
// =============================================================================

func (a *OptionTradingAccount) Position(instrumentID string) (*exchanges.Position, bool) {
	a.posMu.RLock()
	defer a.posMu.RUnlock()
	p, ok := a.positions[instrumentID]
	if !ok {
		return nil, false
	}
	c := *p
	return &c, true
}

func (a *OptionTradingAccount) Positions() []exchanges.Position {
	a.posMu.RLock()
	defer a.posMu.RUnlock()
	out := make([]exchanges.Position, 0, len(a.positions))
	for _, p := range a.positions {
		out = append(out, *p)
	}
	return out
}

func (a *OptionTradingAccount) SubscribePositions() *Subscription[exchanges.Position] {
	return a.positionBus.Subscribe()
}

func (a *OptionTradingAccount) Health() TradingAccountHealth {
	return a.healthSnapshot()
}

// PortfolioGreeks aggregates Greeks across all tracked option positions,
// weighting each leg by signed Quantity × ContractSize. Long-call delta
// adds positively; short-call delta adds negatively (Position.Side determines sign).
//
// Spot/perp positions in the cache (shouldn't normally appear, since the
// adapter is OptionExchange) are skipped silently.
func (a *OptionTradingAccount) PortfolioGreeks() exchanges.Greeks {
	a.posMu.RLock()
	defer a.posMu.RUnlock()

	var total exchanges.Greeks
	for _, p := range a.positions {
		if p == nil || p.Option == nil {
			continue
		}
		size := p.Option.ContractSize
		if size.IsZero() {
			size = decimal.NewFromInt(1)
		}
		signed := p.Quantity.Mul(size)
		if p.Side == exchanges.PositionSideShort {
			signed = signed.Neg()
		}
		total.Delta = total.Delta.Add(p.Option.Greeks.Delta.Mul(signed))
		total.Gamma = total.Gamma.Add(p.Option.Greeks.Gamma.Mul(signed))
		total.Vega = total.Vega.Add(p.Option.Greeks.Vega.Mul(signed))
		total.Theta = total.Theta.Add(p.Option.Greeks.Theta.Mul(signed))
		total.Rho = total.Rho.Add(p.Option.Greeks.Rho.Mul(signed))
		// IV is not meaningful in aggregate; leave it zero.
	}
	return total
}

// =============================================================================
// Start / Close
// =============================================================================

func (a *OptionTradingAccount) Start(ctx context.Context) (err error) {
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

	a.logger.Infow("option_account: fetching initial account state")
	acc, accErr := a.optionAdp.FetchAccount(runCtx)
	if accErr != nil {
		return fmt.Errorf("option_account: failed to get initial state: %w", accErr)
	}
	a.applyAccountSnapshot(runGen, acc)

	// Pull option positions explicitly: FetchAccount may not include them
	// (especially on venues where options share a separate user-data namespace).
	if posErr := a.refreshOptionPositions(runCtx, runGen); posErr != nil {
		if !errors.Is(posErr, exchanges.ErrNotSupported) {
			return fmt.Errorf("option_account: initial FetchOptionPositions: %w", posErr)
		}
		a.logger.Warnw("option_account: FetchOptionPositions not supported", "error", posErr)
	}

	if err := a.startOrderAndFillStreamsWithPolicy(runCtx, runGen, false); err != nil {
		return err
	}

	if streamable, ok := a.optionAdp.(exchanges.Streamable); ok {
		if watchErr := streamable.WatchPositions(runCtx, func(p *exchanges.Position) {
			a.applyPositionUpdate(runGen, p)
		}); watchErr != nil {
			if !errors.Is(watchErr, exchanges.ErrNotSupported) {
				a.markStreamError(StreamPositions, watchErr)
				return fmt.Errorf("option_account: WatchPositions failed: %w", watchErr)
			}
			a.markStreamUnsupported(StreamPositions, watchErr)
			a.logger.Warnw("option_account: WatchPositions not supported", "error", watchErr)
		} else {
			a.markStreamReady(StreamPositions)
		}
	} else {
		a.markStreamUnsupported(StreamPositions, exchanges.ErrNotSupported)
	}

	if runErr := runCtx.Err(); runErr != nil {
		return runErr
	}
	if !a.finishRunStart(runGen) {
		return context.Canceled
	}

	go a.periodicRefresh(runCtx, time.Minute, runGen, func(acc *exchanges.Account) {
		a.applyAccountSnapshot(runGen, acc)
		if err := a.refreshOptionPositions(runCtx, runGen); err != nil {
			if runCtx.Err() == nil && a.isActiveRun(runGen) && !errors.Is(err, exchanges.ErrNotSupported) {
				a.logger.Errorw("option_account: periodic position refresh failed", "error", err)
			}
		}
	})
	return nil
}

func (a *OptionTradingAccount) Close() {
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

func (a *OptionTradingAccount) Cancel(ctx context.Context, orderID, symbol string) error {
	if err := a.optionAdp.CancelOrder(ctx, orderID, symbol); err != nil {
		return err
	}
	order, err := a.optionAdp.FetchOrderByID(ctx, orderID, symbol)
	if err != nil {
		a.logger.Warnw("option_account: fetch cancelled order failed", "order_id", orderID, "symbol", symbol, "error", err)
		return nil
	}
	a.applyCurrentOrderUpdate(order)
	return nil
}

// =============================================================================
// Internals
// =============================================================================

func (a *OptionTradingAccount) applyAccountSnapshot(runGen uint64, acc *exchanges.Account) {
	if !a.isActiveRun(runGen) {
		return
	}
	if acc == nil {
		acc = &exchanges.Account{}
	}
	a.applyOrderSnapshot(runGen, acc.Orders)

	// Only seed option-flagged positions from the generic Account payload;
	// adapters that share a unified account may include perp positions here.
	next := make(map[string]*exchanges.Position)
	for _, p := range acc.Positions {
		if p.InstrumentType != exchanges.InstrumentTypeOption {
			continue
		}
		c := p
		next[p.Symbol] = &c
	}

	a.posMu.Lock()
	if a.isActiveRun(runGen) {
		a.positions = next
	}
	a.posMu.Unlock()
	a.markSnapshotLoaded()
}

func (a *OptionTradingAccount) refreshOptionPositions(ctx context.Context, runGen uint64) error {
	positions, err := a.optionAdp.FetchOptionPositions(ctx)
	if err != nil {
		return err
	}
	if !a.isActiveRun(runGen) {
		return nil
	}
	next := make(map[string]*exchanges.Position, len(positions))
	for _, p := range positions {
		c := p
		next[p.Symbol] = &c
	}
	a.posMu.Lock()
	if a.isActiveRun(runGen) {
		a.positions = next
	}
	a.posMu.Unlock()
	return nil
}

func (a *OptionTradingAccount) applyPositionUpdate(runGen uint64, p *exchanges.Position) {
	if p == nil || !a.isActiveRun(runGen) {
		return
	}
	if p.InstrumentType != exchanges.InstrumentTypeOption {
		// Adapters that share a stream across markets may forward perp
		// positions; OptionTradingAccount only tracks option positions.
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

	dropped := a.positionBus.Publish(p)
	a.markStreamEvent(StreamPositions, dropped)
}

func (a *OptionTradingAccount) resetState() {
	a.resetOrderCache()
	a.posMu.Lock()
	a.positions = make(map[string]*exchanges.Position)
	a.posMu.Unlock()
}
