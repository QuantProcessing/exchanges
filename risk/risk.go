package risk

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantProcessing/exchanges/cache"
	"github.com/QuantProcessing/exchanges/kernel"
	"github.com/QuantProcessing/exchanges/model"
	"github.com/shopspring/decimal"
)

var ErrRiskRejected = errors.New("risk rejected")
var ErrRiskQueueFull = errors.New("risk queue full")

const defaultQueueSize = 1024

type Health struct {
	kernel.Health
	CommandQueueDepth    int
	EventQueueDepth      int
	CommandQueueCapacity int
	EventQueueCapacity   int
	TradingState         TradingState
	TradingStateReason   string
	ProcessedCommands    int64
	RejectedCommands     int64
	ProcessedEvents      int64
	DroppedCommands      int64
	DroppedEvents        int64
	ThrottledCommands    int64
}

type Decision struct {
	Order model.SubmitOrder
	Error error
	Event *model.ExecutionEvent
}

func (d Decision) Accepted() bool {
	return d.Error == nil
}

type riskCommand struct {
	order model.SubmitOrder
	reply chan Decision
}

type TradingState string

const (
	TradingStateActive   TradingState = "active"
	TradingStateHalted   TradingState = "halted"
	TradingStateReducing TradingState = "reducing"
)

type Limits struct {
	MaxOrderNotional    decimal.Decimal
	MaxPositionNotional decimal.Decimal
	MaxAccountExposure  decimal.Decimal
}

type Config struct {
	MaxOrderNotional     decimal.Decimal
	MaxPositionNotional  decimal.Decimal
	MaxAccountExposure   decimal.Decimal
	ExposureCurrency     model.Currency
	TradingState         TradingState
	QueueSize            int
	Clock                kernel.Clock
	MaxCommandsPerWindow int
	CommandRateWindow    time.Duration
	MaxOpenOrders        int
	AccountLimits        map[model.AccountID]Limits
	StrategyLimits       map[model.StrategyID]Limits
	InstrumentLimits     map[model.InstrumentID]Limits
}

type resolvedLimits struct {
	Limits
	orderSource    string
	positionSource string
	exposureSource string
}

type Engine struct {
	mu        sync.RWMutex
	cache     *cache.Cache
	cfg       Config
	stateNote string
	clock     kernel.Clock
	component *kernel.Component
	cmdQueue  chan riskCommand
	evtQueue  chan model.ExecutionEvent
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	cmdWindow []time.Time

	processedCommands atomic.Int64
	rejectedCommands  atomic.Int64
	processedEvents   atomic.Int64
	droppedCommands   atomic.Int64
	droppedEvents     atomic.Int64
	throttledCommands atomic.Int64
}

func NewEngine(c *cache.Cache, cfg Config) *Engine {
	if c == nil {
		c = cache.New()
	}
	clock := cfg.Clock
	if clock == nil {
		clock = kernel.LiveClock{}
	}
	e := &Engine{
		cache:     c,
		cfg:       cfg,
		clock:     clock,
		component: kernel.NewComponent("risk.engine", kernel.ComponentHooks{}),
	}
	e.ensureQueues()
	return e
}

func (e *Engine) Start(ctx context.Context) error {
	e.ensureQueues()
	if e.component == nil {
		e.component = kernel.NewComponent("risk.engine", kernel.ComponentHooks{})
	}
	if e.component.State() == kernel.ComponentStateRunning {
		return nil
	}
	if err := e.component.Start(ctx); err != nil {
		return err
	}
	runCtx, cancel := context.WithCancel(context.Background())
	e.cancel = cancel
	e.wg.Add(2)
	go e.runCommandQueue(runCtx)
	go e.runEventQueue(runCtx)
	return nil
}

func (e *Engine) Stop(ctx context.Context) error {
	if e.component == nil {
		return nil
	}
	if e.cancel != nil {
		e.cancel()
		e.cancel = nil
	}
	e.wg.Wait()
	return e.component.Stop(ctx)
}

func (e *Engine) Health() Health {
	if e == nil || e.component == nil {
		return Health{Health: kernel.Health{ID: "risk.engine", State: kernel.ComponentStateInitialized}}
	}
	e.ensureQueues()
	cfg, stateNote := e.configSnapshot()
	return Health{
		Health:               e.component.Health(),
		CommandQueueDepth:    len(e.cmdQueue),
		EventQueueDepth:      len(e.evtQueue),
		CommandQueueCapacity: cap(e.cmdQueue),
		EventQueueCapacity:   cap(e.evtQueue),
		TradingState:         normalizedTradingState(cfg.TradingState),
		TradingStateReason:   stateNote,
		ProcessedCommands:    e.processedCommands.Load(),
		RejectedCommands:     e.rejectedCommands.Load(),
		ProcessedEvents:      e.processedEvents.Load(),
		DroppedCommands:      e.droppedCommands.Load(),
		DroppedEvents:        e.droppedEvents.Load(),
		ThrottledCommands:    e.throttledCommands.Load(),
	}
}

func (e *Engine) SetTradingState(state TradingState, reason string) error {
	state = normalizedTradingState(state)
	switch state {
	case TradingStateActive, TradingStateHalted, TradingStateReducing:
	default:
		return fmt.Errorf("%w: invalid trading state %q", ErrRiskRejected, state)
	}
	e.mu.Lock()
	e.cfg.TradingState = state
	if state == TradingStateActive {
		e.stateNote = ""
	} else {
		e.stateNote = reason
	}
	e.mu.Unlock()
	return nil
}

func (e *Engine) EngageKillSwitch(reason string) error {
	return e.SetTradingState(TradingStateHalted, reason)
}

func (e *Engine) SetReducingOnly(reason string) error {
	return e.SetTradingState(TradingStateReducing, reason)
}

func (e *Engine) ResumeTrading() error {
	return e.SetTradingState(TradingStateActive, "")
}

func (e *Engine) configSnapshot() (Config, string) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.cfg, e.stateNote
}

func normalizedTradingState(state TradingState) TradingState {
	if state == "" {
		return TradingStateActive
	}
	return state
}

func (e *Engine) Execute(ctx context.Context, order model.SubmitOrder) (<-chan Decision, error) {
	e.ensureQueues()
	reply := make(chan Decision, 1)
	cmd := riskCommand{order: order, reply: reply}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case e.cmdQueue <- cmd:
		return reply, nil
	default:
		e.droppedCommands.Add(1)
		return nil, ErrRiskQueueFull
	}
}

func (e *Engine) Process(ctx context.Context, event model.ExecutionEvent) error {
	if err := event.Validate(); err != nil {
		return err
	}
	e.ensureQueues()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case e.evtQueue <- event:
		return nil
	default:
		e.droppedEvents.Add(1)
		return ErrRiskQueueFull
	}
}

func (e *Engine) ensureQueues() {
	if e.cmdQueue != nil && e.evtQueue != nil {
		return
	}
	size := e.cfg.QueueSize
	if size <= 0 {
		size = defaultQueueSize
	}
	if e.cmdQueue == nil {
		e.cmdQueue = make(chan riskCommand, size)
	}
	if e.evtQueue == nil {
		e.evtQueue = make(chan model.ExecutionEvent, size)
	}
}

func (e *Engine) runCommandQueue(ctx context.Context) {
	defer e.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case cmd := <-e.cmdQueue:
			e.handleCommand(cmd)
		}
	}
}

func (e *Engine) handleCommand(cmd riskCommand) {
	err := e.Check(cmd.order)
	decision := Decision{Order: cmd.order, Error: err}
	if err != nil {
		e.rejectedCommands.Add(1)
		if event, ok := OrderDeniedEvent(cmd.order, err); ok {
			decision.Event = &event
		}
	}
	e.processedCommands.Add(1)
	cmd.reply <- decision
	close(cmd.reply)
}

func (e *Engine) runEventQueue(ctx context.Context) {
	defer e.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-e.evtQueue:
			if err := e.applyExecutionEvent(event); err != nil && e.component != nil {
				e.component.Degrade(err)
			}
			e.processedEvents.Add(1)
		}
	}
}

func (e *Engine) applyExecutionEvent(event model.ExecutionEvent) error {
	if event.Account != nil {
		e.cache.PutAccount(*event.Account)
		return nil
	}
	if event.Order != nil {
		return e.cache.PutOrder(*event.Order)
	}
	if event.Fill != nil {
		_, err := e.cache.PutFill(*event.Fill)
		return err
	}
	if event.Position != nil {
		return e.cache.PutPosition(*event.Position)
	}
	if event.Lifecycle != nil && event.Lifecycle.Report != nil {
		return e.cache.PutOrder(*event.Lifecycle.Report)
	}
	return nil
}

func (e *Engine) Check(order model.SubmitOrder) error {
	return e.check(order, true, false)
}

func (e *Engine) CheckExistingOrder(order model.SubmitOrder) error {
	return e.check(order, false, true)
}

func (e *Engine) check(order model.SubmitOrder, rejectDuplicateClientID bool, existingOrder bool) error {
	if err := order.Validate(); err != nil {
		return err
	}
	cfg, stateNote := e.configSnapshot()
	if err := e.checkCommandRate(cfg); err != nil {
		return err
	}
	if rejectDuplicateClientID && order.ClientOrderID != "" {
		if _, ok := e.cache.OrderByClientID(order.AccountID, order.ClientOrderID); ok {
			return fmt.Errorf("%w: duplicate client order ID %s", ErrRiskRejected, order.ClientOrderID)
		}
	}
	if !existingOrder && cfg.MaxOpenOrders > 0 && len(e.cache.OpenOrders(order.AccountID)) >= cfg.MaxOpenOrders {
		return fmt.Errorf("%w: max open orders exceeded", ErrRiskRejected)
	}
	inst, ok := e.cache.Instrument(order.InstrumentID)
	if !ok {
		return fmt.Errorf("%w: %s", model.ErrInstrumentNotFound, order.InstrumentID.String())
	}
	if inst.Status != model.InstrumentStatusTrading {
		return fmt.Errorf("%w: instrument is not trading", ErrRiskRejected)
	}
	switch normalizedTradingState(cfg.TradingState) {
	case TradingStateActive:
	case TradingStateHalted:
		return stateRejectedError("trading state halted", stateNote)
	case TradingStateReducing:
		if e.increasesCurrentExposure(order) {
			return stateRejectedError("reducing state rejects exposure increase", stateNote)
		}
	default:
		return fmt.Errorf("%w: invalid trading state %q", ErrRiskRejected, cfg.TradingState)
	}
	if err := inst.ValidateSize(order.Quantity); err != nil {
		return err
	}
	if requiresLimitPrice(order.Type) {
		if err := inst.ValidatePrice(order.Price); err != nil {
			return err
		}
	}
	if order.TriggerPrice.IsPositive() {
		if err := inst.ValidatePrice(order.TriggerPrice); err != nil {
			return err
		}
	}
	if order.ActivationPrice.IsPositive() {
		if err := inst.ValidatePrice(order.ActivationPrice); err != nil {
			return err
		}
	}
	limits := resolveLimits(order, cfg)
	price := e.estimatedPrice(order)
	if limits.MaxOrderNotional.IsPositive() {
		if !price.IsPositive() {
			return fmt.Errorf("%w: cannot estimate order notional", ErrRiskRejected)
		}
		if price.Mul(order.Quantity).GreaterThan(limits.MaxOrderNotional) {
			return limitExceededError(limits.orderSource, "max order notional exceeded")
		}
	}
	if limits.MaxPositionNotional.IsPositive() {
		if !price.IsPositive() {
			return fmt.Errorf("%w: cannot estimate position notional", ErrRiskRejected)
		}
		if e.projectedPositionNotional(order, price).GreaterThan(limits.MaxPositionNotional) {
			return limitExceededError(limits.positionSource, "max position notional exceeded")
		}
	}
	if limits.MaxAccountExposure.IsPositive() {
		if !price.IsPositive() {
			return fmt.Errorf("%w: cannot estimate account exposure", ErrRiskRejected)
		}
		exposure, ok := e.projectedAccountExposure(order, price, cfg)
		if !ok {
			return fmt.Errorf("%w: cannot estimate account exposure", ErrRiskRejected)
		}
		if exposure.GreaterThan(limits.MaxAccountExposure) {
			return limitExceededError(limits.exposureSource, "max account exposure exceeded")
		}
	}
	if err := e.checkAvailableInitialMargin(order, inst, price); err != nil {
		return err
	}
	if order.ReduceOnly && e.increasesCurrentExposure(order) {
		return fmt.Errorf("%w: reduce-only would increase exposure", ErrRiskRejected)
	}
	return nil
}

func (e *Engine) checkCommandRate(cfg Config) error {
	if cfg.MaxCommandsPerWindow <= 0 {
		return nil
	}
	window := cfg.CommandRateWindow
	if window <= 0 {
		window = time.Second
	}
	now := e.clock.Now()
	cutoff := now.Add(-window)

	e.mu.Lock()
	defer e.mu.Unlock()
	kept := e.cmdWindow[:0]
	for _, ts := range e.cmdWindow {
		if ts.After(cutoff) {
			kept = append(kept, ts)
		}
	}
	e.cmdWindow = kept
	if len(e.cmdWindow) >= cfg.MaxCommandsPerWindow {
		e.throttledCommands.Add(1)
		return fmt.Errorf("%w: command rate limit exceeded", ErrRiskRejected)
	}
	e.cmdWindow = append(e.cmdWindow, now)
	return nil
}

func resolveLimits(order model.SubmitOrder, cfg Config) resolvedLimits {
	resolved := resolvedLimits{
		Limits: Limits{
			MaxOrderNotional:    cfg.MaxOrderNotional,
			MaxPositionNotional: cfg.MaxPositionNotional,
			MaxAccountExposure:  cfg.MaxAccountExposure,
		},
	}
	if limit, ok := cfg.AccountLimits[order.AccountID]; ok {
		resolved.apply("account "+string(order.AccountID), limit)
	}
	if order.Metadata.StrategyID != "" {
		if limit, ok := cfg.StrategyLimits[order.Metadata.StrategyID]; ok {
			resolved.apply("strategy "+string(order.Metadata.StrategyID), limit)
		}
	}
	if limit, ok := cfg.InstrumentLimits[order.InstrumentID]; ok {
		resolved.apply("instrument "+order.InstrumentID.String(), limit)
	}
	return resolved
}

func (l *resolvedLimits) apply(source string, limit Limits) {
	l.MaxOrderNotional, l.orderSource = tighterLimit(l.MaxOrderNotional, l.orderSource, limit.MaxOrderNotional, source)
	l.MaxPositionNotional, l.positionSource = tighterLimit(l.MaxPositionNotional, l.positionSource, limit.MaxPositionNotional, source)
	l.MaxAccountExposure, l.exposureSource = tighterLimit(l.MaxAccountExposure, l.exposureSource, limit.MaxAccountExposure, source)
}

func tighterLimit(current decimal.Decimal, currentSource string, candidate decimal.Decimal, candidateSource string) (decimal.Decimal, string) {
	if !candidate.IsPositive() {
		return current, currentSource
	}
	if !current.IsPositive() || candidate.LessThan(current) {
		return candidate, candidateSource
	}
	return current, currentSource
}

func limitExceededError(source string, message string) error {
	if source == "" {
		return fmt.Errorf("%w: %s", ErrRiskRejected, message)
	}
	return fmt.Errorf("%w: %s %s", ErrRiskRejected, source, message)
}

func OrderDeniedEvent(order model.SubmitOrder, cause error) (model.ExecutionEvent, bool) {
	lifecycle := model.OrderLifecycleEvent{
		Metadata:      order.Metadata,
		AccountID:     order.AccountID,
		InstrumentID:  order.InstrumentID,
		ClientOrderID: order.ClientOrderID,
		Kind:          model.OrderEventDenied,
		Status:        model.OrderStatusDenied,
	}
	if cause != nil {
		lifecycle.Reason = cause.Error()
	}
	event := model.ExecutionEvent{Lifecycle: &lifecycle}
	if err := event.Validate(); err != nil {
		return model.ExecutionEvent{}, false
	}
	return event, true
}

func stateRejectedError(message string, note string) error {
	if note == "" {
		return fmt.Errorf("%w: %s", ErrRiskRejected, message)
	}
	return fmt.Errorf("%w: %s: %s", ErrRiskRejected, message, note)
}

func (e *Engine) estimatedPrice(order model.SubmitOrder) decimal.Decimal {
	if order.Price.IsPositive() {
		return order.Price
	}
	if quote, ok := e.cache.QuoteTick(order.InstrumentID); ok {
		if order.Side == model.OrderSideBuy && quote.AskPrice.IsPositive() {
			return quote.AskPrice
		}
		if order.Side == model.OrderSideSell && quote.BidPrice.IsPositive() {
			return quote.BidPrice
		}
	}
	if ticker, ok := e.cache.Ticker(order.InstrumentID); ok {
		if order.Side == model.OrderSideBuy && ticker.Ask.IsPositive() {
			return ticker.Ask
		}
		if order.Side == model.OrderSideSell && ticker.Bid.IsPositive() {
			return ticker.Bid
		}
		if ticker.Last.IsPositive() {
			return ticker.Last
		}
	}
	if book, ok := e.cache.OrderBook(order.InstrumentID); ok {
		if order.Side == model.OrderSideBuy && len(book.Asks) > 0 {
			return book.Asks[0].Price
		}
		if order.Side == model.OrderSideSell && len(book.Bids) > 0 {
			return book.Bids[0].Price
		}
	}
	if trade, ok := e.cache.TradeTick(order.InstrumentID); ok && trade.Price.IsPositive() {
		return trade.Price
	}
	if bar, ok := e.cache.LatestBar(order.InstrumentID); ok && bar.Close.IsPositive() {
		return bar.Close
	}
	return decimal.Zero
}

func (e *Engine) projectedPositionNotional(order model.SubmitOrder, price decimal.Decimal) decimal.Decimal {
	return e.projectedSignedPosition(order).Abs().Mul(price)
}

func (e *Engine) projectedAccountExposure(order model.SubmitOrder, orderPrice decimal.Decimal, cfg Config) (decimal.Decimal, bool) {
	total := decimal.Zero
	target := e.accountExposureCurrency(order.AccountID, cfg)
	for _, inst := range e.cache.Instruments() {
		projected := e.signedPositionWithOpenOrders(order.AccountID, inst.ID)
		if inst.ID == order.InstrumentID {
			projected = applySignedOrder(projected, order.Side, order.Quantity)
		}
		if projected.IsZero() {
			continue
		}
		price := orderPrice
		if inst.ID != order.InstrumentID {
			price = e.exposurePrice(order.AccountID, inst.ID, projected)
		}
		if price.IsPositive() {
			value := projected.Abs().Mul(price)
			if target != "" {
				converted, ok := e.convertAmount(value, marginCurrency(inst), target)
				if !ok {
					return decimal.Zero, false
				}
				value = converted
			}
			total = total.Add(value)
		}
	}
	return total, true
}

func (e *Engine) accountExposureCurrency(accountID model.AccountID, cfg Config) model.Currency {
	if cfg.ExposureCurrency != "" {
		return cfg.ExposureCurrency
	}
	account, ok := e.cache.Account(accountID)
	if !ok {
		return ""
	}
	return account.BaseCurrency
}

func (e *Engine) checkAvailableInitialMargin(order model.SubmitOrder, inst model.Instrument, price decimal.Decimal) error {
	if !inst.MarginInit.IsPositive() {
		return nil
	}
	if !e.increasesProjectedExposure(order) {
		return nil
	}
	if !price.IsPositive() {
		return fmt.Errorf("%w: cannot estimate initial margin", ErrRiskRejected)
	}
	account, ok := e.cache.Account(order.AccountID)
	if !ok {
		return nil
	}
	if account.Type != "" && account.Type != model.AccountTypeMargin {
		return nil
	}
	currency := marginCurrency(inst)
	if currency == "" {
		return nil
	}
	available, ok := e.availableMargin(account, currency)
	if !ok {
		return fmt.Errorf("%w: missing margin balance for %s", ErrRiskRejected, currency)
	}
	available = available.Sub(e.openOrderInitialMarginReservation(order.AccountID, currency))
	if available.IsNegative() {
		available = decimal.Zero
	}
	required := e.incrementalInitialMargin(order, price, inst.MarginInit)
	if required.GreaterThan(available) {
		return fmt.Errorf("%w: available initial margin exceeded", ErrRiskRejected)
	}
	return nil
}

func (e *Engine) availableMargin(account model.AccountSnapshot, currency model.Currency) (decimal.Decimal, bool) {
	equity := decimal.Zero
	locked := decimal.Zero
	foundBalance := false
	for _, balance := range account.Balances {
		if balance.Currency != currency {
			continue
		}
		total, balanceLocked, _, err := balance.Amounts()
		if err != nil {
			return decimal.Zero, false
		}
		equity = equity.Add(total)
		locked = locked.Add(balanceLocked)
		foundBalance = true
	}
	if !foundBalance {
		return decimal.Zero, false
	}
	for _, inst := range e.cache.Instruments() {
		if marginCurrency(inst) != currency {
			continue
		}
		position, ok := e.cache.PositionByInstrument(account.AccountID, inst.ID)
		if !ok || position.Side == model.PositionSideFlat || position.Quantity.IsZero() {
			continue
		}
		equity = equity.Add(unrealizedPnL(position, e.positionMark(position)))
	}
	initialMargin := decimal.Zero
	for _, margin := range account.Margins {
		if margin.Currency != currency {
			continue
		}
		initial, _, err := margin.Amounts()
		if err != nil {
			return decimal.Zero, false
		}
		initialMargin = initialMargin.Add(initial)
	}
	available := equity.Sub(locked).Sub(initialMargin)
	if available.IsNegative() {
		return decimal.Zero, true
	}
	return available, true
}

func (e *Engine) incrementalInitialMargin(order model.SubmitOrder, price decimal.Decimal, marginRate decimal.Decimal) decimal.Decimal {
	current := e.signedPositionWithOpenOrders(order.AccountID, order.InstrumentID)
	projected := e.projectedSignedPosition(order)
	currentNotional := current.Abs().Mul(price)
	projectedNotional := projected.Abs().Mul(price)
	incremental := projectedNotional.Sub(currentNotional)
	if incremental.IsNegative() {
		return decimal.Zero
	}
	return incremental.Mul(marginRate)
}

func (e *Engine) projectedSignedPosition(order model.SubmitOrder) decimal.Decimal {
	current := e.signedPositionWithOpenOrders(order.AccountID, order.InstrumentID)
	return applySignedOrder(current, order.Side, order.Quantity)
}

func (e *Engine) signedPositionWithOpenOrders(accountID model.AccountID, instrumentID model.InstrumentID) decimal.Decimal {
	current := decimal.Zero
	if position, ok := e.cache.PositionByInstrument(accountID, instrumentID); ok {
		current = signedPosition(position)
	}
	for _, order := range e.cache.OpenOrders(accountID) {
		if order.InstrumentID != instrumentID || order.ReduceOnly {
			continue
		}
		leaves := openOrderLeaves(order)
		if !leaves.IsPositive() {
			continue
		}
		current = applySignedOrder(current, order.Side, leaves)
	}
	return current
}

func (e *Engine) exposurePrice(accountID model.AccountID, instrumentID model.InstrumentID, signed decimal.Decimal) decimal.Decimal {
	side := model.PositionSideLong
	if signed.IsNegative() {
		side = model.PositionSideShort
	}
	if mark := e.markPriceForSide(instrumentID, side); mark.IsPositive() {
		return mark
	}
	if position, ok := e.cache.PositionByInstrument(accountID, instrumentID); ok && position.EntryPrice.IsPositive() {
		return position.EntryPrice
	}
	for _, order := range e.cache.OpenOrders(accountID) {
		if order.InstrumentID == instrumentID && order.Price.IsPositive() {
			return order.Price
		}
	}
	return decimal.Zero
}

func (e *Engine) openOrderInitialMarginReservation(accountID model.AccountID, currency model.Currency) decimal.Decimal {
	reserved := decimal.Zero
	signedByInstrument := make(map[model.InstrumentID]decimal.Decimal)
	for _, inst := range e.cache.Instruments() {
		if marginCurrency(inst) != currency || !inst.MarginInit.IsPositive() {
			continue
		}
		if position, ok := e.cache.PositionByInstrument(accountID, inst.ID); ok {
			signedByInstrument[inst.ID] = signedPosition(position)
		}
	}
	for _, order := range e.cache.OpenOrders(accountID) {
		if order.ReduceOnly {
			continue
		}
		inst, ok := e.cache.Instrument(order.InstrumentID)
		if !ok || marginCurrency(inst) != currency || !inst.MarginInit.IsPositive() {
			continue
		}
		leaves := openOrderLeaves(order)
		if !leaves.IsPositive() {
			continue
		}
		price := order.Price
		if !price.IsPositive() {
			price = e.exposurePrice(accountID, order.InstrumentID, signedByInstrument[order.InstrumentID])
		}
		if !price.IsPositive() {
			continue
		}
		current := signedByInstrument[order.InstrumentID]
		projected := applySignedOrder(current, order.Side, leaves)
		incremental := projected.Abs().Sub(current.Abs())
		if incremental.IsPositive() {
			reserved = reserved.Add(incremental.Mul(price).Mul(inst.MarginInit))
		}
		signedByInstrument[order.InstrumentID] = projected
	}
	return reserved
}

func (e *Engine) positionMark(position model.PositionStatusReport) decimal.Decimal {
	if mark := e.markPrice(position); mark.IsPositive() {
		return mark
	}
	return position.EntryPrice
}

func (e *Engine) markPrice(position model.PositionStatusReport) decimal.Decimal {
	return e.markPriceForSide(position.InstrumentID, position.Side)
}

func (e *Engine) markPriceForSide(instrumentID model.InstrumentID, side model.PositionSide) decimal.Decimal {
	if quote, ok := e.cache.QuoteTick(instrumentID); ok {
		if side == model.PositionSideLong && quote.BidPrice.IsPositive() {
			return quote.BidPrice
		}
		if side == model.PositionSideShort && quote.AskPrice.IsPositive() {
			return quote.AskPrice
		}
	}
	if ticker, ok := e.cache.Ticker(instrumentID); ok {
		if ticker.Last.IsPositive() {
			return ticker.Last
		}
	}
	if book, ok := e.cache.OrderBook(instrumentID); ok {
		if side == model.PositionSideLong && len(book.Bids) > 0 {
			return book.Bids[0].Price
		}
		if side == model.PositionSideShort && len(book.Asks) > 0 {
			return book.Asks[0].Price
		}
	}
	if trade, ok := e.cache.TradeTick(instrumentID); ok && trade.Price.IsPositive() {
		return trade.Price
	}
	if bar, ok := e.cache.LatestBar(instrumentID); ok && bar.Close.IsPositive() {
		return bar.Close
	}
	return decimal.Zero
}

func unrealizedPnL(position model.PositionStatusReport, mark decimal.Decimal) decimal.Decimal {
	if !mark.IsPositive() || position.Side == model.PositionSideFlat || position.Quantity.IsZero() {
		return decimal.Zero
	}
	if position.Side == model.PositionSideShort {
		return position.EntryPrice.Sub(mark).Mul(position.Quantity)
	}
	return mark.Sub(position.EntryPrice).Mul(position.Quantity)
}

func marginCurrency(inst model.Instrument) model.Currency {
	if inst.Settle != "" {
		return inst.Settle
	}
	return inst.Quote
}

func (e *Engine) convertAmount(amount decimal.Decimal, from model.Currency, to model.Currency) (decimal.Decimal, bool) {
	if from == "" || to == "" {
		return decimal.Zero, false
	}
	if from == to {
		return amount, true
	}
	rate, ok := e.exchangeRate(from, to)
	if !ok {
		return decimal.Zero, false
	}
	return amount.Mul(rate), true
}

func (e *Engine) exchangeRate(from model.Currency, to model.Currency) (decimal.Decimal, bool) {
	for _, inst := range e.cache.Instruments() {
		price, ok := e.xratePrice(inst.ID)
		if !ok || !price.IsPositive() {
			continue
		}
		if inst.Base == from && inst.Quote == to {
			return price, true
		}
		if inst.Base == to && inst.Quote == from {
			return decimal.NewFromInt(1).Div(price), true
		}
	}
	return decimal.Zero, false
}

func (e *Engine) xratePrice(instrumentID model.InstrumentID) (decimal.Decimal, bool) {
	if quote, ok := e.cache.QuoteTick(instrumentID); ok {
		switch {
		case quote.BidPrice.IsPositive() && quote.AskPrice.IsPositive():
			return quote.BidPrice.Add(quote.AskPrice).Div(decimal.NewFromInt(2)), true
		case quote.BidPrice.IsPositive():
			return quote.BidPrice, true
		case quote.AskPrice.IsPositive():
			return quote.AskPrice, true
		}
	}
	if ticker, ok := e.cache.Ticker(instrumentID); ok {
		if ticker.Bid.IsPositive() && ticker.Ask.IsPositive() {
			return ticker.Bid.Add(ticker.Ask).Div(decimal.NewFromInt(2)), true
		}
		if ticker.Last.IsPositive() {
			return ticker.Last, true
		}
	}
	if trade, ok := e.cache.TradeTick(instrumentID); ok && trade.Price.IsPositive() {
		return trade.Price, true
	}
	if bar, ok := e.cache.LatestBar(instrumentID); ok && bar.Close.IsPositive() {
		return bar.Close, true
	}
	return decimal.Zero, false
}

func requiresLimitPrice(t model.OrderType) bool {
	return t == model.OrderTypeLimit ||
		t == model.OrderTypeStopLimit ||
		t == model.OrderTypeLimitIfTouched ||
		t == model.OrderTypeTrailingStopLimit
}

func signedPosition(position model.PositionStatusReport) decimal.Decimal {
	if position.Side == model.PositionSideShort {
		return position.Quantity.Neg()
	}
	return position.Quantity
}

func openOrderLeaves(order model.OrderStatusReport) decimal.Decimal {
	if order.LeavesQuantity.IsPositive() {
		return order.LeavesQuantity
	}
	if order.Quantity.IsPositive() {
		leaves := order.Quantity.Sub(order.FilledQuantity)
		if leaves.IsPositive() {
			return leaves
		}
	}
	return decimal.Zero
}

func applySignedOrder(current decimal.Decimal, side model.OrderSide, quantity decimal.Decimal) decimal.Decimal {
	if side == model.OrderSideSell {
		return current.Sub(quantity)
	}
	return current.Add(quantity)
}

func (e *Engine) increasesCurrentExposure(order model.SubmitOrder) bool {
	current := decimal.Zero
	if position, ok := e.cache.PositionByInstrument(order.AccountID, order.InstrumentID); ok {
		current = signedPosition(position)
	}
	projected := applySignedOrder(current, order.Side, order.Quantity)
	return exposureIncreased(current, projected)
}

func (e *Engine) increasesProjectedExposure(order model.SubmitOrder) bool {
	current := e.signedPositionWithOpenOrders(order.AccountID, order.InstrumentID)
	projected := applySignedOrder(current, order.Side, order.Quantity)
	return exposureIncreased(current, projected)
}

func exposureIncreased(current, projected decimal.Decimal) bool {
	if current.IsZero() {
		return !projected.IsZero()
	}
	if projected.IsZero() {
		return false
	}
	if current.Sign()*projected.Sign() < 0 {
		return true
	}
	return projected.Abs().GreaterThan(current.Abs())
}
