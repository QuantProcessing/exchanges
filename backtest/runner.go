package backtest

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/QuantProcessing/exchanges/account"
	"github.com/QuantProcessing/exchanges/bus"
	"github.com/QuantProcessing/exchanges/cache"
	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/portfolio"
	"github.com/QuantProcessing/exchanges/strategy"
	"github.com/shopspring/decimal"
)

type Event struct {
	At      time.Time
	Topic   string
	Message any
}

type Config struct {
	Bus          *bus.Bus
	Cache        *cache.Cache
	Portfolio    *portfolio.Portfolio
	FillModel    FillModel
	OrderLatency time.Duration
	Events       []Event
	Strategies   []strategy.Strategy
}

type Result struct {
	EventsProcessed int
	Cache           *cache.Cache
	Portfolio       *portfolio.Portfolio
}

type Runner struct {
	bus     *bus.Bus
	events  []Event
	engine  *strategy.Engine
	runtime *runtime
}

func NewRunner(cfg Config) *Runner {
	b := cfg.Bus
	if b == nil {
		b = bus.New()
	}
	c := cfg.Cache
	if c == nil {
		c = cache.New()
	}
	pf := cfg.Portfolio
	if pf == nil {
		portfolioCache := cache.New()
		for _, inst := range c.Instruments() {
			_ = portfolioCache.PutInstrument(inst)
		}
		pf = portfolio.New(portfolioCache)
	}
	fillModel := cfg.FillModel
	if fillModel == nil {
		fillModel = DefaultFillModel()
	}
	rt := newRuntime(b, c, pf, fillModel, cfg.OrderLatency)
	engine := strategy.NewEngine(b, strategy.WithRuntime(rt), strategy.WithSynchronousDispatch())
	rt.engine = engine
	for _, s := range cfg.Strategies {
		_ = engine.Add(s)
	}
	return &Runner{bus: b, events: append([]Event(nil), cfg.Events...), engine: engine, runtime: rt}
}

func (r *Runner) Run(ctx context.Context) (Result, error) {
	sort.SliceStable(r.events, func(i, j int) bool {
		return r.events[i].At.Before(r.events[j].At)
	})
	if len(r.events) > 0 {
		r.runtime.setTime(r.events[0].At)
	}
	if err := r.engine.Start(ctx); err != nil {
		return Result{}, err
	}
	result := Result{Cache: r.runtime.cache, Portfolio: r.runtime.pf}
	var runErr error
	for _, event := range r.events {
		if err := r.runtime.DispatchTimersDue(ctx, event.At); err != nil {
			runErr = errors.Join(runErr, err)
			break
		}
		if err := r.runtime.ExpireOpenOrders(ctx, event.At); err != nil {
			runErr = errors.Join(runErr, err)
			break
		}
		r.runtime.setTime(event.At)
		r.runtime.RecordMarket(event.Message)
		if event.Topic == strategy.TopicMarketData {
			if err := r.runtime.MatchOpenOrders(ctx); err != nil {
				runErr = errors.Join(runErr, err)
				break
			}
		}
		env := bus.Envelope{Topic: event.Topic, Message: event.Message, Timestamp: event.At}
		if err := r.engine.Process(ctx, env); err != nil {
			runErr = errors.Join(runErr, err)
			break
		}
		if event.Topic == strategy.TopicMarketData {
			if err := r.runtime.MatchOpenOrders(ctx); err != nil {
				runErr = errors.Join(runErr, err)
				break
			}
		}
		result.EventsProcessed++
	}
	runErr = errors.Join(runErr, r.engine.Stop(ctx))
	return result, runErr
}

type runtime struct {
	bus              *bus.Bus
	engine           *strategy.Engine
	cache            *cache.Cache
	pf               *portfolio.Portfolio
	reconciler       *account.Reconciler
	lastTicker       map[model.InstrumentID]model.Ticker
	lastBooks        map[model.InstrumentID]model.OrderBook
	lastTrades       map[model.InstrumentID]model.TradeTick
	lastQuotes       map[model.InstrumentID]model.QuoteTick
	lastBars         map[model.InstrumentID]model.Bar
	consumed         map[model.InstrumentID]map[string]decimal.Decimal
	trailing         map[string]decimal.Decimal
	trailingTriggers map[string]decimal.Decimal
	accounts         map[model.AccountID]struct{}
	subs             map[string]model.SubscribeMarketData
	factories        map[model.AccountID]*model.OrderFactory
	heldChildren     map[parentOrderKey][]model.SubmitOrder
	orderListMembers map[orderListKey][]model.ClientOrderID
	eligibleAt       map[latencyOrderKey]time.Time
	fillModel        FillModel
	orderLatency     time.Duration
	currentTime      time.Time
	timers           map[string]timerState
	nextOrder        int
	nextTrade        int
}

type timerState struct {
	name     string
	interval time.Duration
	next     time.Time
}

type parentOrderKey struct {
	accountID     model.AccountID
	clientOrderID model.ClientOrderID
}

type orderListKey struct {
	accountID   model.AccountID
	orderListID model.OrderListID
}

type latencyOrderKey struct {
	accountID model.AccountID
	orderID   model.OrderID
}

func newRuntime(b *bus.Bus, c *cache.Cache, pf *portfolio.Portfolio, fillModel FillModel, orderLatency time.Duration) *runtime {
	return &runtime{
		bus:              b,
		cache:            c,
		pf:               pf,
		reconciler:       account.NewReconciler(c),
		lastTicker:       make(map[model.InstrumentID]model.Ticker),
		lastBooks:        make(map[model.InstrumentID]model.OrderBook),
		lastTrades:       make(map[model.InstrumentID]model.TradeTick),
		lastQuotes:       make(map[model.InstrumentID]model.QuoteTick),
		lastBars:         make(map[model.InstrumentID]model.Bar),
		consumed:         make(map[model.InstrumentID]map[string]decimal.Decimal),
		trailing:         make(map[string]decimal.Decimal),
		trailingTriggers: make(map[string]decimal.Decimal),
		accounts:         make(map[model.AccountID]struct{}),
		subs:             make(map[string]model.SubscribeMarketData),
		factories:        make(map[model.AccountID]*model.OrderFactory),
		heldChildren:     make(map[parentOrderKey][]model.SubmitOrder),
		orderListMembers: make(map[orderListKey][]model.ClientOrderID),
		eligibleAt:       make(map[latencyOrderKey]time.Time),
		fillModel:        fillModel,
		orderLatency:     orderLatency,
		timers:           make(map[string]timerState),
	}
}

func (r *runtime) Cache() *cache.Cache { return r.cache }

func (r *runtime) Portfolio() *portfolio.Portfolio { return r.pf }

func (r *runtime) Clock() strategy.Clock { return runtimeClock{runtime: r} }

func (r *runtime) SetTimer(_ context.Context, name string, interval time.Duration) error {
	if err := strategy.ValidateTimer(name, interval); err != nil {
		return err
	}
	r.timers[name] = timerState{
		name:     name,
		interval: interval,
		next:     r.now().Add(interval),
	}
	return nil
}

func (r *runtime) CancelTimer(_ context.Context, name string) error {
	delete(r.timers, name)
	return nil
}

func (r *runtime) DispatchTimersDue(ctx context.Context, until time.Time) error {
	for {
		timer, ok := r.nextDueTimer(until)
		if !ok {
			return nil
		}
		r.setTime(timer.next)
		event := strategy.TimerEvent{Name: timer.name, Timestamp: timer.next}
		if err := r.engine.Process(ctx, bus.Envelope{
			Topic:     strategy.TopicTimer,
			Message:   event,
			Timestamp: timer.next,
		}); err != nil {
			return err
		}
		if err := r.bus.Publish(ctx, strategy.TopicTimer, event); err != nil {
			return err
		}
		if current, ok := r.timers[timer.name]; ok && current.next.Equal(timer.next) {
			current.next = current.next.Add(current.interval)
			r.timers[timer.name] = current
		}
	}
}

func (r *runtime) nextDueTimer(until time.Time) (timerState, bool) {
	var next timerState
	for _, timer := range r.timers {
		if timer.next.After(until) {
			continue
		}
		if next.name == "" || timer.next.Before(next.next) || timer.next.Equal(next.next) && timer.name < next.name {
			next = timer
		}
	}
	return next, next.name != ""
}

func (r *runtime) setTime(t time.Time) {
	r.currentTime = t
}

func (r *runtime) now() time.Time {
	return r.currentTime
}

type runtimeClock struct {
	runtime *runtime
}

func (c runtimeClock) Now() time.Time {
	return c.runtime.now()
}

func (r *runtime) OrderFactory(accountID model.AccountID) *model.OrderFactory {
	if r.factories[accountID] == nil {
		r.factories[accountID] = model.NewOrderFactory(accountID)
	}
	return r.factories[accountID]
}

func (r *runtime) SubscribeMarketData(_ context.Context, sub model.SubscribeMarketData) error {
	if err := sub.Validate(); err != nil {
		return err
	}
	r.subs[sub.Key()] = sub
	return nil
}

func (r *runtime) UnsubscribeMarketData(_ context.Context, sub model.SubscribeMarketData) error {
	if err := sub.Validate(); err != nil {
		return err
	}
	delete(r.subs, sub.Key())
	return nil
}

func (r *runtime) SubscribeTicker(ctx context.Context, instrumentID model.InstrumentID) error {
	return r.SubscribeMarketData(ctx, model.SubscribeMarketData{
		InstrumentID: instrumentID,
		Type:         model.MarketDataTypeTicker,
	})
}

func (r *runtime) UnsubscribeTicker(ctx context.Context, instrumentID model.InstrumentID) error {
	return r.UnsubscribeMarketData(ctx, model.SubscribeMarketData{
		InstrumentID: instrumentID,
		Type:         model.MarketDataTypeTicker,
	})
}

func (r *runtime) SubscribeTradeTicks(ctx context.Context, instrumentID model.InstrumentID) error {
	return r.SubscribeMarketData(ctx, model.SubscribeMarketData{
		InstrumentID: instrumentID,
		Type:         model.MarketDataTypeTradeTick,
	})
}

func (r *runtime) UnsubscribeTradeTicks(ctx context.Context, instrumentID model.InstrumentID) error {
	return r.UnsubscribeMarketData(ctx, model.SubscribeMarketData{
		InstrumentID: instrumentID,
		Type:         model.MarketDataTypeTradeTick,
	})
}

func (r *runtime) SubscribeQuoteTicks(ctx context.Context, instrumentID model.InstrumentID) error {
	return r.SubscribeMarketData(ctx, model.SubscribeMarketData{
		InstrumentID: instrumentID,
		Type:         model.MarketDataTypeQuoteTick,
	})
}

func (r *runtime) UnsubscribeQuoteTicks(ctx context.Context, instrumentID model.InstrumentID) error {
	return r.UnsubscribeMarketData(ctx, model.SubscribeMarketData{
		InstrumentID: instrumentID,
		Type:         model.MarketDataTypeQuoteTick,
	})
}

func (r *runtime) SubscribeBars(ctx context.Context, barType model.BarType) error {
	barType = barType.Canonical()
	return r.SubscribeMarketData(ctx, model.SubscribeMarketData{
		InstrumentID: barType.InstrumentID,
		Type:         model.MarketDataTypeBar,
		BarType:      barType,
	})
}

func (r *runtime) UnsubscribeBars(ctx context.Context, barType model.BarType) error {
	barType = barType.Canonical()
	return r.UnsubscribeMarketData(ctx, model.SubscribeMarketData{
		InstrumentID: barType.InstrumentID,
		Type:         model.MarketDataTypeBar,
		BarType:      barType,
	})
}

func (r *runtime) SubscribeOrderBookDepth(ctx context.Context, instrumentID model.InstrumentID, depth int) error {
	return r.SubscribeMarketData(ctx, model.SubscribeMarketData{
		InstrumentID: instrumentID,
		Type:         model.MarketDataTypeOrderBook,
		Depth:        depth,
	})
}

func (r *runtime) UnsubscribeOrderBookDepth(ctx context.Context, instrumentID model.InstrumentID, depth int) error {
	return r.UnsubscribeMarketData(ctx, model.SubscribeMarketData{
		InstrumentID: instrumentID,
		Type:         model.MarketDataTypeOrderBook,
		Depth:        depth,
	})
}

func (r *runtime) RecordMarket(message any) {
	event, ok := message.(model.MarketEvent)
	if !ok {
		return
	}
	_ = r.cache.PutMarketEvent(event)
	if r.pf != nil {
		_ = r.pf.ApplyMarketEvent(event)
	}
	if event.Ticker != nil {
		r.lastTicker[event.Ticker.InstrumentID] = *event.Ticker
		r.consumed[event.Ticker.InstrumentID] = make(map[string]decimal.Decimal)
	}
	if event.OrderBook != nil {
		r.lastBooks[event.OrderBook.InstrumentID] = cloneBook(*event.OrderBook)
		r.consumed[event.OrderBook.InstrumentID] = make(map[string]decimal.Decimal)
	}
	if event.Trade != nil {
		r.lastTrades[event.Trade.InstrumentID] = *event.Trade
		r.consumed[event.Trade.InstrumentID] = make(map[string]decimal.Decimal)
	}
	if event.Quote != nil {
		r.lastQuotes[event.Quote.InstrumentID] = *event.Quote
		r.consumed[event.Quote.InstrumentID] = make(map[string]decimal.Decimal)
	}
	if event.Bar != nil {
		instrumentID := event.Bar.BarType.Canonical().InstrumentID
		r.lastBars[instrumentID] = *event.Bar
		r.consumed[instrumentID] = make(map[string]decimal.Decimal)
	}
}

func (r *runtime) SubmitOrder(ctx context.Context, order model.SubmitOrder) (model.OrderStatusReport, error) {
	report, err := r.acceptOrder(ctx, order)
	if err != nil {
		return model.OrderStatusReport{}, err
	}
	if err := r.matchOrder(ctx, report); err != nil {
		return model.OrderStatusReport{}, err
	}
	return report, nil
}

func (r *runtime) SubmitOrderList(ctx context.Context, list model.OrderList) ([]model.OrderStatusReport, error) {
	list = list.WithCommandMetadataDefaults()
	if err := list.Validate(); err != nil {
		return nil, err
	}
	r.indexOrderList(list)
	reports := make([]model.OrderStatusReport, 0, len(list.Orders))
	for _, order := range list.Orders {
		if order.ParentClientOrderID != "" {
			continue
		}
		report, err := r.SubmitOrder(ctx, order)
		if err != nil {
			return reports, err
		}
		reports = append(reports, report)
	}
	return reports, nil
}

func (r *runtime) ModifyOrder(ctx context.Context, modify model.ModifyOrder) (model.OrderStatusReport, error) {
	if err := modify.Validate(); err != nil {
		_ = r.dispatchOrderModifyRejected(ctx, modify, model.OrderStatusAccepted, nil, err)
		return model.OrderStatusReport{}, err
	}
	existing, ok := r.findOrder(modify)
	if !ok {
		err := fmt.Errorf("%w: order not found", model.ErrInvalidOrder)
		_ = r.dispatchOrderModifyRejected(ctx, modify, model.OrderStatusAccepted, nil, err)
		return model.OrderStatusReport{}, err
	}
	modify = fillBacktestModifyIdentity(modify, existing)
	updated, _, err := model.ApplyOrderModification(existing, modify)
	if err != nil {
		_ = r.dispatchOrderModifyRejected(ctx, modify, model.OrderStatusAccepted, &existing, err)
		return model.OrderStatusReport{}, err
	}
	updated.Metadata = modify.Metadata.WithDefaults(updated.Metadata)
	pending := existing
	pending.Metadata = modify.Metadata.WithDefaults(existing.Metadata)
	pending.Status = model.OrderStatusPendingUpdate
	pending.LastUpdatedTime = r.now()
	if err := r.apply(ctx, model.ExecutionEvent{Order: &pending}); err != nil {
		return model.OrderStatusReport{}, err
	}
	if err := r.apply(ctx, model.ExecutionEvent{Lifecycle: ptrOrderLifecycle(backtestOrderLifecycleFromReport(pending, model.OrderEventPendingUpdate, existing.Status, ""))}); err != nil {
		return model.OrderStatusReport{}, err
	}
	if err := r.apply(ctx, model.ExecutionEvent{Order: &updated}); err != nil {
		return model.OrderStatusReport{}, err
	}
	if err := r.apply(ctx, model.ExecutionEvent{Lifecycle: ptrOrderLifecycle(backtestOrderLifecycleFromReport(updated, model.OrderEventUpdated, model.OrderStatusPendingUpdate, ""))}); err != nil {
		return model.OrderStatusReport{}, err
	}
	if err := r.matchOrder(ctx, updated); err != nil {
		return model.OrderStatusReport{}, err
	}
	current, ok := r.cache.Order(updated.AccountID, updated.OrderID)
	if ok {
		return current, nil
	}
	return updated, nil
}

func (r *runtime) acceptOrder(ctx context.Context, order model.SubmitOrder) (model.OrderStatusReport, error) {
	if err := order.Validate(); err != nil {
		return model.OrderStatusReport{}, err
	}
	if err := r.apply(ctx, model.ExecutionEvent{Lifecycle: ptrOrderLifecycle(backtestOrderSubmittedLifecycle(order))}); err != nil {
		return model.OrderStatusReport{}, err
	}
	r.nextOrder++
	report := model.OrderStatusReport{
		Metadata:            order.Metadata,
		AccountID:           order.AccountID,
		InstrumentID:        order.InstrumentID,
		OrderListID:         order.OrderListID,
		OrderID:             model.OrderID(fmt.Sprintf("bt-order-%d", r.nextOrder)),
		ParentClientOrderID: order.ParentClientOrderID,
		ClientOrderID:       order.ClientOrderID,
		Status:              model.OrderStatusAccepted,
		Side:                order.Side,
		Type:                order.Type,
		Contingency:         order.Contingency,
		Quantity:            order.Quantity,
		FilledQuantity:      decimal.Zero,
		LeavesQuantity:      order.Quantity,
		Price:               order.Price,
		TriggerPrice:        order.TriggerPrice,
		ActivationPrice:     order.ActivationPrice,
		TrailingOffset:      order.TrailingOffset,
		PostOnly:            order.PostOnly,
		ReduceOnly:          order.ReduceOnly,
		TimeInForce:         order.TimeInForce,
		ExpireTime:          order.ExpireTime,
		LastUpdatedTime:     r.now(),
	}
	if err := r.apply(ctx, model.ExecutionEvent{Order: &report}); err != nil {
		return model.OrderStatusReport{}, err
	}
	if err := r.apply(ctx, model.ExecutionEvent{Lifecycle: ptrOrderLifecycle(backtestOrderLifecycleFromReport(report, model.OrderEventAccepted, model.OrderStatusSubmitted, ""))}); err != nil {
		return model.OrderStatusReport{}, err
	}
	r.registerOrderLatency(report)
	r.accounts[order.AccountID] = struct{}{}
	return report, nil
}

func (r *runtime) CancelOrder(ctx context.Context, cancel model.CancelOrder) (model.OrderStatusReport, error) {
	if err := cancel.Validate(); err != nil {
		_ = r.dispatchOrderCancelRejected(ctx, cancel, model.OrderStatusAccepted, nil, err)
		return model.OrderStatusReport{}, err
	}
	order, ok := r.cache.Order(cancel.AccountID, cancel.OrderID)
	if !ok && cancel.ClientOrderID != "" {
		order, ok = r.cache.OrderByClientID(cancel.AccountID, cancel.ClientOrderID)
	}
	if !ok {
		err := fmt.Errorf("%w: order not found", model.ErrInvalidOrder)
		_ = r.dispatchOrderCancelRejected(ctx, cancel, model.OrderStatusAccepted, nil, err)
		return model.OrderStatusReport{}, err
	}
	if !order.Status.IsOpen() || order.Status == model.OrderStatusPendingCancel {
		err := fmt.Errorf("%w: order is not cancelable", model.ErrInvalidOrder)
		_ = r.dispatchOrderCancelRejected(ctx, fillBacktestCancelIdentity(cancel, order), model.OrderStatusAccepted, &order, err)
		return model.OrderStatusReport{}, err
	}
	cancel = fillBacktestCancelIdentity(cancel, order)
	pending := order
	pending.Metadata = cancel.Metadata.WithDefaults(order.Metadata)
	pending.Status = model.OrderStatusPendingCancel
	pending.LastUpdatedTime = r.now()
	if err := r.apply(ctx, model.ExecutionEvent{Order: &pending}); err != nil {
		return model.OrderStatusReport{}, err
	}
	if err := r.apply(ctx, model.ExecutionEvent{Lifecycle: ptrOrderLifecycle(backtestOrderLifecycleFromReport(pending, model.OrderEventPendingCancel, order.Status, ""))}); err != nil {
		return model.OrderStatusReport{}, err
	}
	order.Status = model.OrderStatusCanceled
	order.Metadata = cancel.Metadata.WithDefaults(order.Metadata)
	order.LeavesQuantity = decimal.Zero
	if err := r.apply(ctx, model.ExecutionEvent{Order: &order}); err != nil {
		return model.OrderStatusReport{}, err
	}
	if err := r.apply(ctx, model.ExecutionEvent{Lifecycle: ptrOrderLifecycle(backtestOrderLifecycleFromReport(order, model.OrderEventCanceled, model.OrderStatusPendingCancel, ""))}); err != nil {
		return model.OrderStatusReport{}, err
	}
	return order, nil
}

func (r *runtime) BatchCancelOrders(ctx context.Context, batch model.BatchCancelOrders) ([]model.OrderStatusReport, error) {
	if err := batch.Validate(); err != nil {
		return nil, err
	}
	reports := make([]model.OrderStatusReport, 0, len(batch.Cancels))
	var batchErr error
	for _, cancel := range batch.Cancels {
		cancel.Metadata = cancel.Metadata.WithDefaults(batch.Metadata)
		cancel = fillBacktestBatchCancelIdentity(cancel, batch.AccountID, batch.InstrumentID)
		report, err := r.CancelOrder(ctx, cancel)
		if err != nil {
			batchErr = errors.Join(batchErr, err)
			continue
		}
		reports = append(reports, report)
	}
	return reports, batchErr
}

func (r *runtime) CancelAllOrders(ctx context.Context, cancelAll model.CancelAllOrders) ([]model.OrderStatusReport, error) {
	if err := cancelAll.Validate(); err != nil {
		return nil, err
	}
	orders := r.cache.OpenOrders(cancelAll.AccountID)
	batch := model.BatchCancelOrders{
		Metadata:     cancelAll.Metadata,
		AccountID:    cancelAll.AccountID,
		InstrumentID: cancelAll.InstrumentID,
	}
	for _, order := range orders {
		if !cancelAll.MatchesOrder(order) {
			continue
		}
		batch.Cancels = append(batch.Cancels, model.CancelOrder{
			Metadata:      cancelAll.Metadata,
			AccountID:     order.AccountID,
			InstrumentID:  order.InstrumentID,
			OrderID:       order.OrderID,
			ClientOrderID: order.ClientOrderID,
		})
	}
	if len(batch.Cancels) == 0 {
		return nil, nil
	}
	return r.BatchCancelOrders(ctx, batch)
}

func (r *runtime) QueryOrder(_ context.Context, query model.QueryOrder) (model.OrderStatusReport, error) {
	if err := query.Validate(); err != nil {
		return model.OrderStatusReport{}, err
	}
	if order, ok := r.findQueriedOrder(query); ok {
		order.Metadata = query.Metadata.WithDefaults(order.Metadata)
		return order, nil
	}
	return model.OrderStatusReport{}, fmt.Errorf("%w: order not found", model.ErrInvalidOrder)
}

func (r *runtime) QueryAccount(ctx context.Context, query model.QueryAccount) (model.AccountSnapshot, error) {
	if err := query.Validate(); err != nil {
		return model.AccountSnapshot{}, err
	}
	if snapshot, ok := r.cache.Account(query.AccountID); ok {
		if err := r.dispatchExecution(ctx, model.ExecutionEvent{Account: &snapshot}); err != nil {
			return model.AccountSnapshot{}, err
		}
		return snapshot, nil
	}
	snapshot := model.AccountSnapshot{AccountID: query.AccountID, Venue: model.Venue("BACKTEST")}
	if err := r.apply(ctx, model.ExecutionEvent{Account: &snapshot}); err != nil {
		return model.AccountSnapshot{}, err
	}
	return snapshot, nil
}

func (r *runtime) findOrder(modify model.ModifyOrder) (model.OrderStatusReport, bool) {
	if modify.OrderID != "" {
		if order, ok := r.cache.Order(modify.AccountID, modify.OrderID); ok {
			return order, true
		}
	}
	if modify.ClientOrderID != "" {
		if order, ok := r.cache.OrderByClientID(modify.AccountID, modify.ClientOrderID); ok {
			return order, true
		}
	}
	if modify.VenueOrderID != "" {
		if order, ok := r.cache.OrderByVenueID(modify.AccountID, modify.VenueOrderID); ok {
			return order, true
		}
	}
	return model.OrderStatusReport{}, false
}

func (r *runtime) findQueriedOrder(query model.QueryOrder) (model.OrderStatusReport, bool) {
	if query.OrderID != "" {
		if order, ok := r.cache.Order(query.AccountID, query.OrderID); ok {
			return order, true
		}
	}
	if query.ClientOrderID != "" {
		if order, ok := r.cache.OrderByClientID(query.AccountID, query.ClientOrderID); ok {
			return order, true
		}
	}
	if query.VenueOrderID != "" {
		if order, ok := r.cache.OrderByVenueID(query.AccountID, query.VenueOrderID); ok {
			return order, true
		}
	}
	return model.OrderStatusReport{}, false
}

func (r *runtime) dispatchOrderModifyRejected(ctx context.Context, modify model.ModifyOrder, previous model.OrderStatus, report *model.OrderStatusReport, cause error) error {
	lifecycle := model.OrderLifecycleEvent{
		Metadata:       modify.Metadata,
		AccountID:      modify.AccountID,
		InstrumentID:   modify.InstrumentID,
		OrderID:        modify.OrderID,
		ClientOrderID:  modify.ClientOrderID,
		VenueOrderID:   modify.VenueOrderID,
		Kind:           model.OrderEventModifyRejected,
		PreviousStatus: previous,
		Status:         model.OrderStatusAccepted,
		Report:         report,
	}
	if cause != nil {
		lifecycle.Reason = cause.Error()
	}
	event := model.ExecutionEvent{Lifecycle: &lifecycle}
	if err := event.Validate(); err != nil {
		return nil
	}
	return r.apply(ctx, event)
}

func (r *runtime) dispatchOrderCancelRejected(ctx context.Context, cancel model.CancelOrder, previous model.OrderStatus, report *model.OrderStatusReport, cause error) error {
	lifecycle := model.OrderLifecycleEvent{
		Metadata:       cancel.Metadata,
		AccountID:      cancel.AccountID,
		InstrumentID:   cancel.InstrumentID,
		OrderID:        cancel.OrderID,
		ClientOrderID:  cancel.ClientOrderID,
		Kind:           model.OrderEventCancelRejected,
		PreviousStatus: previous,
		Status:         model.OrderStatusAccepted,
		Report:         report,
	}
	if cause != nil {
		lifecycle.Reason = cause.Error()
	}
	event := model.ExecutionEvent{Lifecycle: &lifecycle}
	if err := event.Validate(); err != nil {
		return nil
	}
	return r.apply(ctx, event)
}

func (r *runtime) MatchOpenOrders(ctx context.Context) error {
	for accountID := range r.accounts {
		for _, order := range r.cache.OpenOrders(accountID) {
			if err := r.matchOrder(ctx, order); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *runtime) ExpireOpenOrders(ctx context.Context, until time.Time) error {
	for accountID := range r.accounts {
		for _, order := range r.cache.OpenOrders(accountID) {
			if !shouldExpire(order, until) {
				continue
			}
			if err := r.expireOrder(ctx, order); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *runtime) expireOrder(ctx context.Context, order model.OrderStatusReport) error {
	previous := order.Status
	r.setTime(order.ExpireTime)
	order.Status = model.OrderStatusExpired
	order.LeavesQuantity = decimal.Zero
	order.LastUpdatedTime = r.now()
	if err := r.apply(ctx, model.ExecutionEvent{Order: &order}); err != nil {
		return err
	}
	return r.apply(ctx, model.ExecutionEvent{Lifecycle: ptrOrderLifecycle(backtestOrderLifecycleFromReport(order, model.OrderEventExpired, previous, ""))})
}

func shouldExpire(order model.OrderStatusReport, until time.Time) bool {
	return order.TimeInForce == model.TimeInForceGTD &&
		order.Status.IsOpen() &&
		!order.ExpireTime.IsZero() &&
		!order.ExpireTime.After(until)
}

func (r *runtime) matchOrder(ctx context.Context, order model.OrderStatusReport) error {
	if !order.Status.IsOpen() {
		return nil
	}
	if !r.orderEligible(order) {
		return nil
	}
	if order.Type == model.OrderTypeLimit && order.TimeInForce == model.TimeInForceFOK && !r.canFullyFillImmediate(order) {
		return r.cancelUnfilledRemainder(ctx, order.AccountID, order.OrderID)
	}
	switch order.Type {
	case model.OrderTypeMarket:
		matched, err := r.matchAgainstBook(ctx, order)
		if err != nil {
			return err
		}
		if matched {
			return r.cancelUnfilledMarketRemainder(ctx, order.AccountID, order.OrderID)
		}
		matched, err = r.matchAgainstQuote(ctx, order)
		if err != nil {
			return err
		}
		if matched {
			return r.cancelUnfilledMarketRemainder(ctx, order.AccountID, order.OrderID)
		}
		matched, err = r.matchAgainstTrade(ctx, order)
		if err != nil {
			return err
		}
		if matched {
			return r.cancelUnfilledMarketRemainder(ctx, order.AccountID, order.OrderID)
		}
		matched, err = r.matchAgainstBar(ctx, order)
		if err != nil {
			return err
		}
		if matched {
			return r.cancelUnfilledMarketRemainder(ctx, order.AccountID, order.OrderID)
		}
		return r.matchMarketOrderAgainstTicker(ctx, order)
	case model.OrderTypeMarketToLimit:
		return r.matchMarketToLimit(ctx, order)
	case model.OrderTypeLimit:
		if _, err := r.matchAgainstBook(ctx, order); err != nil {
			return err
		}
		if _, err := r.matchAgainstQuote(ctx, order); err != nil {
			return err
		}
		if _, err := r.matchAgainstTrade(ctx, order); err != nil {
			return err
		}
		if _, err := r.matchAgainstBar(ctx, order); err != nil {
			return err
		}
		return r.cancelImmediateRemainder(ctx, order)
	case model.OrderTypeStopMarket:
		return r.matchTriggeredMarket(ctx, order, stopTrigger)
	case model.OrderTypeMarketIfTouched:
		return r.matchTriggeredMarket(ctx, order, touchedTrigger)
	case model.OrderTypeStopLimit:
		return r.matchTriggeredLimit(ctx, order, stopTrigger)
	case model.OrderTypeLimitIfTouched:
		return r.matchTriggeredLimit(ctx, order, touchedTrigger)
	case model.OrderTypeTrailingStopMarket:
		return r.matchTrailingMarket(ctx, order)
	case model.OrderTypeTrailingStopLimit:
		return r.matchTrailingLimit(ctx, order)
	default:
		return nil
	}
}

func (r *runtime) registerOrderLatency(order model.OrderStatusReport) {
	if r.orderLatency <= 0 {
		return
	}
	r.eligibleAt[latencyOrderKey{accountID: order.AccountID, orderID: order.OrderID}] = r.now().Add(r.orderLatency)
}

func (r *runtime) orderEligible(order model.OrderStatusReport) bool {
	eligibleAt, ok := r.eligibleAt[latencyOrderKey{accountID: order.AccountID, orderID: order.OrderID}]
	if !ok {
		return true
	}
	return !r.now().Before(eligibleAt)
}

func (r *runtime) matchMarketToLimit(ctx context.Context, order model.OrderStatusReport) error {
	firstPrice := r.firstBookPrice(order)
	if !firstPrice.IsPositive() {
		firstPrice = r.currentPrice(order)
	}
	matched, err := r.matchAgainstBook(ctx, order)
	if err != nil || !matched {
		matched, err = r.matchAgainstQuote(ctx, order)
		if err != nil {
			return err
		}
	}
	if !matched {
		matched, err = r.matchAgainstTrade(ctx, order)
		if err != nil {
			return err
		}
	}
	if !matched {
		matched, err = r.matchAgainstBar(ctx, order)
		if err != nil {
			return err
		}
	}
	if !matched {
		if err := r.matchMarketOrderAgainstTicker(ctx, order); err != nil {
			return err
		}
	}
	resting, ok := r.cache.Order(order.AccountID, order.OrderID)
	if !ok || !resting.Status.IsOpen() || !resting.Price.IsZero() || !firstPrice.IsPositive() {
		return nil
	}
	resting.Price = firstPrice
	return r.apply(ctx, model.ExecutionEvent{Order: &resting})
}

type triggerPolicy func(model.OrderStatusReport, decimal.Decimal) bool

func (r *runtime) matchTriggeredMarket(ctx context.Context, order model.OrderStatusReport, policy triggerPolicy) error {
	if !r.isTriggered(order, policy) {
		return nil
	}
	triggered, err := r.markTriggered(ctx, order)
	if err != nil {
		return err
	}
	matched, err := r.matchAgainstBook(ctx, triggered)
	if err != nil {
		return err
	}
	if matched {
		return r.cancelUnfilledMarketRemainder(ctx, triggered.AccountID, triggered.OrderID)
	}
	matched, err = r.matchAgainstQuote(ctx, triggered)
	if err != nil {
		return err
	}
	if matched {
		return r.cancelUnfilledMarketRemainder(ctx, triggered.AccountID, triggered.OrderID)
	}
	matched, err = r.matchAgainstTrade(ctx, triggered)
	if err != nil {
		return err
	}
	if matched {
		return r.cancelUnfilledMarketRemainder(ctx, triggered.AccountID, triggered.OrderID)
	}
	matched, err = r.matchAgainstBar(ctx, triggered)
	if err != nil {
		return err
	}
	if matched {
		return r.cancelUnfilledMarketRemainder(ctx, triggered.AccountID, triggered.OrderID)
	}
	return r.matchMarketOrderAgainstTicker(ctx, triggered)
}

func (r *runtime) matchTriggeredLimit(ctx context.Context, order model.OrderStatusReport, policy triggerPolicy) error {
	if !r.isTriggered(order, policy) {
		return nil
	}
	triggered, err := r.markTriggered(ctx, order)
	if err != nil {
		return err
	}
	if _, err = r.matchAgainstBook(ctx, triggered); err != nil {
		return err
	}
	if _, err = r.matchAgainstQuote(ctx, triggered); err != nil {
		return err
	}
	if _, err = r.matchAgainstTrade(ctx, triggered); err != nil {
		return err
	}
	_, err = r.matchAgainstBar(ctx, triggered)
	return err
}

func (r *runtime) matchTrailingMarket(ctx context.Context, order model.OrderStatusReport) error {
	if !r.isTrailingTriggered(order) {
		return nil
	}
	if triggerPrice, ok := r.trailingTriggers[trailingKey(order)]; ok {
		order.TriggerPrice = triggerPrice
	}
	triggered, err := r.markTriggered(ctx, order)
	if err != nil {
		return err
	}
	matched, err := r.matchAgainstBook(ctx, triggered)
	if err != nil {
		return err
	}
	if matched {
		return r.cancelUnfilledMarketRemainder(ctx, triggered.AccountID, triggered.OrderID)
	}
	matched, err = r.matchAgainstQuote(ctx, triggered)
	if err != nil {
		return err
	}
	if matched {
		return r.cancelUnfilledMarketRemainder(ctx, triggered.AccountID, triggered.OrderID)
	}
	matched, err = r.matchAgainstTrade(ctx, triggered)
	if err != nil {
		return err
	}
	if matched {
		return r.cancelUnfilledMarketRemainder(ctx, triggered.AccountID, triggered.OrderID)
	}
	matched, err = r.matchAgainstBar(ctx, triggered)
	if err != nil {
		return err
	}
	if matched {
		return r.cancelUnfilledMarketRemainder(ctx, triggered.AccountID, triggered.OrderID)
	}
	return r.matchMarketOrderAgainstTicker(ctx, triggered)
}

func (r *runtime) matchTrailingLimit(ctx context.Context, order model.OrderStatusReport) error {
	if !r.isTrailingTriggered(order) {
		return nil
	}
	if triggerPrice, ok := r.trailingTriggers[trailingKey(order)]; ok {
		order.TriggerPrice = triggerPrice
	}
	triggered, err := r.markTriggered(ctx, order)
	if err != nil {
		return err
	}
	if _, err = r.matchAgainstBook(ctx, triggered); err != nil {
		return err
	}
	if _, err = r.matchAgainstQuote(ctx, triggered); err != nil {
		return err
	}
	if _, err = r.matchAgainstTrade(ctx, triggered); err != nil {
		return err
	}
	_, err = r.matchAgainstBar(ctx, triggered)
	return err
}

func (r *runtime) isTriggered(order model.OrderStatusReport, policy triggerPolicy) bool {
	if order.Status == model.OrderStatusTriggered {
		return true
	}
	if !order.TriggerPrice.IsPositive() {
		return false
	}
	for _, price := range r.latestTriggerPrices(order) {
		if policy(order, price) {
			return true
		}
	}
	return false
}

func (r *runtime) latestTriggerPrices(order model.OrderStatusReport) []decimal.Decimal {
	var prices []decimal.Decimal
	var ts time.Time
	var seen bool
	consider := func(candidate decimal.Decimal, candidateTime time.Time) {
		if !candidate.IsPositive() {
			return
		}
		if !seen || candidateTime.After(ts) {
			prices = []decimal.Decimal{candidate}
			ts = candidateTime
			seen = true
			return
		}
		if candidateTime.Equal(ts) {
			prices = append(prices, candidate)
		}
	}
	if ticker, ok := r.lastTicker[order.InstrumentID]; ok {
		consider(ticker.Last, ticker.Timestamp)
	}
	if quote, ok := r.lastQuotes[order.InstrumentID]; ok {
		if order.Side == model.OrderSideBuy {
			consider(quote.AskPrice, quote.Timestamp)
		} else {
			consider(quote.BidPrice, quote.Timestamp)
		}
	}
	if trade, ok := r.lastTrades[order.InstrumentID]; ok {
		consider(trade.Price, trade.Timestamp)
	}
	if bar, ok := r.lastBars[order.InstrumentID]; ok {
		consider(bar.High, bar.Timestamp)
		consider(bar.Low, bar.Timestamp)
	}
	return prices
}

func (r *runtime) currentPrice(order model.OrderStatusReport) decimal.Decimal {
	if price := r.latestOrderPrice(order); price.IsPositive() {
		return price
	}
	book, ok := r.lastBooks[order.InstrumentID]
	if !ok {
		return decimal.Zero
	}
	if order.Side == model.OrderSideBuy && len(book.Asks) > 0 {
		return book.Asks[0].Price
	}
	if order.Side == model.OrderSideSell && len(book.Bids) > 0 {
		return book.Bids[0].Price
	}
	return decimal.Zero
}

func (r *runtime) latestOrderPrice(order model.OrderStatusReport) decimal.Decimal {
	var price decimal.Decimal
	var ts time.Time
	var seen bool
	consider := func(candidate decimal.Decimal, candidateTime time.Time) {
		if !candidate.IsPositive() {
			return
		}
		if !seen || candidateTime.After(ts) || candidateTime.Equal(ts) {
			price = candidate
			ts = candidateTime
			seen = true
		}
	}
	if ticker, ok := r.lastTicker[order.InstrumentID]; ok {
		consider(ticker.Last, ticker.Timestamp)
	}
	if quote, ok := r.lastQuotes[order.InstrumentID]; ok {
		if order.Side == model.OrderSideBuy {
			consider(quote.AskPrice, quote.Timestamp)
		} else {
			consider(quote.BidPrice, quote.Timestamp)
		}
	}
	if trade, ok := r.lastTrades[order.InstrumentID]; ok {
		consider(trade.Price, trade.Timestamp)
	}
	if bar, ok := r.lastBars[order.InstrumentID]; ok {
		consider(bar.Close, bar.Timestamp)
	}
	return price
}

func (r *runtime) isTrailingTriggered(order model.OrderStatusReport) bool {
	if order.Status == model.OrderStatusTriggered {
		return true
	}
	if !order.TrailingOffset.IsPositive() {
		return false
	}
	favorable, adverse, ok := r.latestTrailingRange(order)
	if !ok {
		return false
	}
	key := trailingKey(order)
	delete(r.trailingTriggers, key)
	extreme, active := r.trailing[key]
	if !active {
		if order.ActivationPrice.IsPositive() {
			if order.Side == model.OrderSideSell && favorable.LessThan(order.ActivationPrice) {
				return false
			}
			if order.Side == model.OrderSideBuy && favorable.GreaterThan(order.ActivationPrice) {
				return false
			}
		}
		r.trailing[key] = favorable
		extreme = favorable
	}
	if order.Side == model.OrderSideSell {
		if favorable.GreaterThan(extreme) {
			r.trailing[key] = favorable
			extreme = favorable
		}
		triggerPrice := extreme.Sub(order.TrailingOffset)
		if adverse.LessThanOrEqual(triggerPrice) {
			r.trailingTriggers[key] = triggerPrice
			return true
		}
		return false
	}
	if favorable.LessThan(extreme) {
		r.trailing[key] = favorable
		extreme = favorable
	}
	triggerPrice := extreme.Add(order.TrailingOffset)
	if adverse.GreaterThanOrEqual(triggerPrice) {
		r.trailingTriggers[key] = triggerPrice
		return true
	}
	return false
}

func (r *runtime) latestTrailingRange(order model.OrderStatusReport) (decimal.Decimal, decimal.Decimal, bool) {
	var favorable decimal.Decimal
	var adverse decimal.Decimal
	var ts time.Time
	var seen bool
	consider := func(candidateFavorable decimal.Decimal, candidateAdverse decimal.Decimal, candidateTime time.Time) {
		if !candidateFavorable.IsPositive() || !candidateAdverse.IsPositive() {
			return
		}
		if !seen || candidateTime.After(ts) {
			favorable = candidateFavorable
			adverse = candidateAdverse
			ts = candidateTime
			seen = true
			return
		}
		if !candidateTime.Equal(ts) {
			return
		}
		if order.Side == model.OrderSideSell {
			favorable = decimal.Max(favorable, candidateFavorable)
			adverse = decimal.Min(adverse, candidateAdverse)
			return
		}
		favorable = decimal.Min(favorable, candidateFavorable)
		adverse = decimal.Max(adverse, candidateAdverse)
	}
	considerSingle := func(candidate decimal.Decimal, candidateTime time.Time) {
		consider(candidate, candidate, candidateTime)
	}
	if ticker, ok := r.lastTicker[order.InstrumentID]; ok {
		considerSingle(ticker.Last, ticker.Timestamp)
	}
	if quote, ok := r.lastQuotes[order.InstrumentID]; ok {
		if order.Side == model.OrderSideBuy {
			considerSingle(quote.AskPrice, quote.Timestamp)
		} else {
			considerSingle(quote.BidPrice, quote.Timestamp)
		}
	}
	if trade, ok := r.lastTrades[order.InstrumentID]; ok {
		considerSingle(trade.Price, trade.Timestamp)
	}
	if bar, ok := r.lastBars[order.InstrumentID]; ok {
		if order.Side == model.OrderSideSell {
			consider(bar.High, bar.Low, bar.Timestamp)
		} else {
			consider(bar.Low, bar.High, bar.Timestamp)
		}
	}
	return favorable, adverse, seen
}

func (r *runtime) markTriggered(ctx context.Context, order model.OrderStatusReport) (model.OrderStatusReport, error) {
	if order.Status == model.OrderStatusTriggered {
		return order, nil
	}
	order.Status = model.OrderStatusTriggered
	if err := r.apply(ctx, model.ExecutionEvent{Order: &order}); err != nil {
		return model.OrderStatusReport{}, err
	}
	return order, nil
}

func (r *runtime) matchMarketOrderAgainstTicker(ctx context.Context, order model.OrderStatusReport) error {
	ticker, ok := r.lastTicker[order.InstrumentID]
	if !ok || !ticker.Last.IsPositive() {
		return nil
	}
	quantity := openQuantity(order)
	if !quantity.IsPositive() {
		return nil
	}
	fill := r.newFillReportWithSource(order, ticker.Last, quantity, ticker.Timestamp, FillSourceTicker)
	return r.applyFillAndPosition(ctx, fill)
}

func (r *runtime) matchAgainstQuote(ctx context.Context, order model.OrderStatusReport) (bool, error) {
	quote, ok := r.lastQuotes[order.InstrumentID]
	if !ok {
		return false, nil
	}
	price := quote.AskPrice
	available := quote.AskSize
	if order.Side == model.OrderSideSell {
		price = quote.BidPrice
		available = quote.BidSize
	}
	if !price.IsPositive() || !available.IsPositive() || !canMatch(order, price) {
		return false, nil
	}
	quantity := openQuantity(order)
	if !quantity.IsPositive() {
		return false, nil
	}
	available = available.Sub(r.consumedAt(order.InstrumentID, price))
	fillQty := decimal.Min(quantity, available)
	if !fillQty.IsPositive() {
		return false, nil
	}
	if !r.shouldFillLimitTouch(order, price, fillQty, quote.Timestamp, FillSourceQuoteTick) {
		return false, nil
	}
	fill := r.newFillReportWithSource(order, price, fillQty, quote.Timestamp, FillSourceQuoteTick)
	if err := r.applyFillAndPosition(ctx, fill); err != nil {
		return false, err
	}
	r.recordConsumption(order.InstrumentID, price, fillQty)
	return true, nil
}

func (r *runtime) matchAgainstTrade(ctx context.Context, order model.OrderStatusReport) (bool, error) {
	trade, ok := r.lastTrades[order.InstrumentID]
	if !ok || !trade.Price.IsPositive() || !canMatch(order, trade.Price) {
		return false, nil
	}
	quantity := openQuantity(order)
	if !quantity.IsPositive() {
		return false, nil
	}
	if !r.shouldFillLimitTouch(order, trade.Price, quantity, trade.Timestamp, FillSourceTradeTick) {
		return false, nil
	}
	fill := r.newFillReportWithSource(order, trade.Price, quantity, trade.Timestamp, FillSourceTradeTick)
	if err := r.applyFillAndPosition(ctx, fill); err != nil {
		return false, err
	}
	return true, nil
}

func (r *runtime) matchAgainstBar(ctx context.Context, order model.OrderStatusReport) (bool, error) {
	bar, ok := r.lastBars[order.InstrumentID]
	if !ok {
		return false, nil
	}
	price, ok := barMatchPrice(order, bar)
	if !ok {
		return false, nil
	}
	quantity := openQuantity(order)
	if !quantity.IsPositive() {
		return false, nil
	}
	if !r.shouldFillLimitTouch(order, price, quantity, bar.Timestamp, FillSourceBar) {
		return false, nil
	}
	fill := r.newFillReportWithSource(order, price, quantity, bar.Timestamp, FillSourceBar)
	if err := r.applyFillAndPosition(ctx, fill); err != nil {
		return false, err
	}
	return true, nil
}

func barMatchPrice(order model.OrderStatusReport, bar model.Bar) (decimal.Decimal, bool) {
	switch order.Type {
	case model.OrderTypeLimit, model.OrderTypeStopLimit, model.OrderTypeLimitIfTouched, model.OrderTypeTrailingStopLimit:
		if !order.Price.IsPositive() {
			return decimal.Zero, false
		}
		if order.Side == model.OrderSideBuy && bar.Low.IsPositive() && bar.Low.LessThanOrEqual(order.Price) {
			return order.Price, true
		}
		if order.Side == model.OrderSideSell && bar.High.IsPositive() && bar.High.GreaterThanOrEqual(order.Price) {
			return order.Price, true
		}
		return decimal.Zero, false
	case model.OrderTypeStopMarket, model.OrderTypeMarketIfTouched, model.OrderTypeTrailingStopMarket:
		if order.Status == model.OrderStatusTriggered && order.TriggerPrice.IsPositive() {
			return order.TriggerPrice, true
		}
		if !bar.Close.IsPositive() || !canMatch(order, bar.Close) {
			return decimal.Zero, false
		}
		return bar.Close, true
	default:
		if !bar.Close.IsPositive() || !canMatch(order, bar.Close) {
			return decimal.Zero, false
		}
		return bar.Close, true
	}
}

func (r *runtime) matchAgainstBook(ctx context.Context, order model.OrderStatusReport) (bool, error) {
	book, ok := r.lastBooks[order.InstrumentID]
	if !ok {
		return false, nil
	}
	quantity := openQuantity(order)
	if !quantity.IsPositive() {
		return false, nil
	}
	var levels []model.OrderBookLevel
	if order.Side == model.OrderSideBuy {
		levels = append([]model.OrderBookLevel(nil), book.Asks...)
	} else {
		levels = append([]model.OrderBookLevel(nil), book.Bids...)
	}
	if len(levels) == 0 {
		return false, nil
	}
	matched := false
	for i := range levels {
		if !quantity.IsPositive() {
			break
		}
		if !canMatch(order, levels[i].Price) {
			break
		}
		available := levels[i].Size.Sub(r.consumedAt(order.InstrumentID, levels[i].Price))
		fillQty := decimal.Min(quantity, available)
		if !fillQty.IsPositive() {
			continue
		}
		if !r.shouldFillLimitTouch(order, levels[i].Price, fillQty, book.Timestamp, FillSourceOrderBook) {
			continue
		}
		fill := r.newFillReportWithSource(order, levels[i].Price, fillQty, book.Timestamp, FillSourceOrderBook)
		if err := r.applyFillAndPosition(ctx, fill); err != nil {
			return matched, err
		}
		r.recordConsumption(order.InstrumentID, levels[i].Price, fillQty)
		quantity = quantity.Sub(fillQty)
		matched = true
	}
	return matched, nil
}

func (r *runtime) newFillReport(order model.OrderStatusReport, price decimal.Decimal, quantity decimal.Decimal, ts time.Time) model.FillReport {
	return r.newFillReportWithSource(order, price, quantity, ts, FillSourceOrderBook)
}

func (r *runtime) newFillReportWithSource(order model.OrderStatusReport, price decimal.Decimal, quantity decimal.Decimal, ts time.Time, source FillSource) model.FillReport {
	price = r.fillPrice(order, price, quantity, ts, source)
	r.nextTrade++
	fill := model.FillReport{
		AccountID:     order.AccountID,
		InstrumentID:  order.InstrumentID,
		OrderID:       order.OrderID,
		ClientOrderID: order.ClientOrderID,
		TradeID:       model.TradeID(fmt.Sprintf("bt-trade-%d", r.nextTrade)),
		Side:          order.Side,
		Price:         price,
		Quantity:      quantity,
		Timestamp:     ts,
	}
	r.applyFillFee(order, &fill)
	return fill
}

func (r *runtime) shouldFillLimitTouch(order model.OrderStatusReport, price decimal.Decimal, quantity decimal.Decimal, ts time.Time, source FillSource) bool {
	if r.fillModel == nil {
		return true
	}
	inst, _ := r.cache.Instrument(order.InstrumentID)
	return r.fillModel.ShouldFillLimitTouch(FillContext{
		Order:      order,
		Instrument: inst,
		Source:     source,
		Price:      price,
		Quantity:   quantity,
		Timestamp:  ts,
		LimitTouch: isLimitTouch(order, price),
		Taker:      !backtestFillIsMaker(order, ts),
	})
}

func (r *runtime) fillPrice(order model.OrderStatusReport, price decimal.Decimal, quantity decimal.Decimal, ts time.Time, source FillSource) decimal.Decimal {
	if r.fillModel == nil {
		return price
	}
	inst, _ := r.cache.Instrument(order.InstrumentID)
	return r.fillModel.ApplySlippage(FillContext{
		Order:      order,
		Instrument: inst,
		Source:     source,
		Price:      price,
		Quantity:   quantity,
		Timestamp:  ts,
		LimitTouch: isLimitTouch(order, price),
		Taker:      !backtestFillIsMaker(order, ts),
	}, price)
}

func (r *runtime) applyFillFee(order model.OrderStatusReport, fill *model.FillReport) {
	inst, ok := r.cache.Instrument(order.InstrumentID)
	if !ok {
		return
	}
	rate := backtestFillFeeRate(order, fill.Timestamp, inst)
	if !rate.IsPositive() {
		return
	}
	fill.Fee = fill.Price.Mul(fill.Quantity).Mul(rate)
	fill.FeeCurrency = backtestFillFeeCurrency(inst)
}

func backtestFillFeeRate(order model.OrderStatusReport, fillTime time.Time, inst model.Instrument) decimal.Decimal {
	if backtestFillIsMaker(order, fillTime) {
		return inst.MakerFee
	}
	return inst.TakerFee
}

func backtestFillIsMaker(order model.OrderStatusReport, fillTime time.Time) bool {
	if order.PostOnly {
		return true
	}
	if !backtestOrderTypeCanRest(order.Type) {
		return false
	}
	if order.LastUpdatedTime.IsZero() || fillTime.IsZero() {
		return false
	}
	return fillTime.After(order.LastUpdatedTime)
}

func backtestOrderTypeCanRest(orderType model.OrderType) bool {
	switch orderType {
	case model.OrderTypeLimit, model.OrderTypeStopLimit, model.OrderTypeLimitIfTouched, model.OrderTypeTrailingStopLimit:
		return true
	default:
		return false
	}
}

func backtestFillFeeCurrency(inst model.Instrument) model.Currency {
	if inst.Settle != "" {
		return inst.Settle
	}
	return inst.Quote
}

func (r *runtime) canFullyFillImmediate(order model.OrderStatusReport) bool {
	quantity := openQuantity(order)
	if !quantity.IsPositive() {
		return true
	}
	available := r.availableImmediateQuantity(order, quantity)
	return available.GreaterThanOrEqual(quantity)
}

func (r *runtime) availableImmediateQuantity(order model.OrderStatusReport, cap decimal.Decimal) decimal.Decimal {
	total := decimal.Zero
	add := func(quantity decimal.Decimal) {
		if !quantity.IsPositive() || total.GreaterThanOrEqual(cap) {
			return
		}
		remaining := cap.Sub(total)
		total = total.Add(decimal.Min(quantity, remaining))
	}
	if book, ok := r.lastBooks[order.InstrumentID]; ok {
		levels := book.Asks
		if order.Side == model.OrderSideSell {
			levels = book.Bids
		}
		for _, level := range levels {
			if !canMatch(order, level.Price) {
				break
			}
			add(level.Size.Sub(r.consumedAt(order.InstrumentID, level.Price)))
		}
	}
	if quote, ok := r.lastQuotes[order.InstrumentID]; ok {
		price := quote.AskPrice
		available := quote.AskSize
		if order.Side == model.OrderSideSell {
			price = quote.BidPrice
			available = quote.BidSize
		}
		if price.IsPositive() && canMatch(order, price) {
			add(available.Sub(r.consumedAt(order.InstrumentID, price)))
		}
	}
	if trade, ok := r.lastTrades[order.InstrumentID]; ok && trade.Price.IsPositive() && canMatch(order, trade.Price) {
		add(cap)
	}
	if bar, ok := r.lastBars[order.InstrumentID]; ok {
		if _, ok := barMatchPrice(order, bar); ok {
			add(cap)
		}
	}
	if ticker, ok := r.lastTicker[order.InstrumentID]; ok && ticker.Last.IsPositive() && canMatch(order, ticker.Last) {
		add(cap)
	}
	return total
}

func (r *runtime) consumedAt(instrumentID model.InstrumentID, price decimal.Decimal) decimal.Decimal {
	if r.consumed[instrumentID] == nil {
		return decimal.Zero
	}
	return r.consumed[instrumentID][price.String()]
}

func (r *runtime) recordConsumption(instrumentID model.InstrumentID, price decimal.Decimal, quantity decimal.Decimal) {
	if r.consumed[instrumentID] == nil {
		r.consumed[instrumentID] = make(map[string]decimal.Decimal)
	}
	key := price.String()
	r.consumed[instrumentID][key] = r.consumed[instrumentID][key].Add(quantity)
}

func (r *runtime) cancelImmediateRemainder(ctx context.Context, order model.OrderStatusReport) error {
	if order.TimeInForce != model.TimeInForceIOC && order.TimeInForce != model.TimeInForceFOK {
		return nil
	}
	return r.cancelUnfilledRemainder(ctx, order.AccountID, order.OrderID)
}

func (r *runtime) cancelUnfilledMarketRemainder(ctx context.Context, accountID model.AccountID, orderID model.OrderID) error {
	return r.cancelUnfilledRemainder(ctx, accountID, orderID)
}

func (r *runtime) cancelUnfilledRemainder(ctx context.Context, accountID model.AccountID, orderID model.OrderID) error {
	order, ok := r.cache.Order(accountID, orderID)
	if !ok || !order.Status.IsOpen() || order.LeavesQuantity.IsZero() {
		return nil
	}
	previous := order.Status
	order.Status = model.OrderStatusCanceled
	order.LeavesQuantity = decimal.Zero
	order.LastUpdatedTime = r.now()
	if err := r.apply(ctx, model.ExecutionEvent{Order: &order}); err != nil {
		return err
	}
	return r.apply(ctx, model.ExecutionEvent{Lifecycle: ptrOrderLifecycle(backtestOrderLifecycleFromReport(order, model.OrderEventCanceled, previous, ""))})
}

func (r *runtime) applyFillAndPosition(ctx context.Context, fill model.FillReport) error {
	var previous *model.PositionStatusReport
	if existing, ok := r.cache.PositionByInstrument(fill.AccountID, fill.InstrumentID); ok {
		previous = &existing
	}
	position := r.nextPosition(fill)
	if err := r.apply(ctx, model.ExecutionEvent{Fill: &fill}); err != nil {
		return err
	}
	if err := r.apply(ctx, model.ExecutionEvent{Position: &position}); err != nil {
		return err
	}
	if r.pf != nil {
		if err := r.pf.ApplyFillWithPosition(fill, previous, position); err != nil {
			return err
		}
	}
	order, ok := r.cache.Order(fill.AccountID, fill.OrderID)
	if !ok || order.Status != model.OrderStatusFilled {
		return nil
	}
	if err := r.releaseHeldChildren(ctx, order); err != nil {
		return err
	}
	return r.cancelOcoSiblings(ctx, order)
}

func (r *runtime) indexOrderList(list model.OrderList) {
	for _, order := range list.Orders {
		listKey := orderListKey{accountID: order.AccountID, orderListID: list.ID}
		r.orderListMembers[listKey] = appendUniqueClientOrderID(r.orderListMembers[listKey], order.ClientOrderID)
		if order.ParentClientOrderID == "" {
			continue
		}
		parentKey := parentOrderKey{accountID: order.AccountID, clientOrderID: order.ParentClientOrderID}
		r.heldChildren[parentKey] = append(r.heldChildren[parentKey], order)
	}
}

func (r *runtime) releaseHeldChildren(ctx context.Context, parent model.OrderStatusReport) error {
	key := parentOrderKey{accountID: parent.AccountID, clientOrderID: parent.ClientOrderID}
	children := append([]model.SubmitOrder(nil), r.heldChildren[key]...)
	delete(r.heldChildren, key)
	accepted := make([]model.OrderStatusReport, 0, len(children))
	for _, child := range children {
		report, err := r.acceptOrder(ctx, child)
		if err != nil {
			return err
		}
		accepted = append(accepted, report)
	}
	for _, report := range accepted {
		current, ok := r.cache.Order(report.AccountID, report.OrderID)
		if ok && current.Status.IsOpen() {
			if err := r.matchOrder(ctx, current); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *runtime) cancelOcoSiblings(ctx context.Context, filled model.OrderStatusReport) error {
	if filled.OrderListID == "" || filled.Contingency != model.ContingencyTypeOCO {
		return nil
	}
	key := orderListKey{accountID: filled.AccountID, orderListID: filled.OrderListID}
	for _, clientOrderID := range r.orderListMembers[key] {
		if clientOrderID == "" || clientOrderID == filled.ClientOrderID {
			continue
		}
		sibling, ok := r.cache.OrderByClientID(filled.AccountID, clientOrderID)
		if !ok || !sibling.Status.IsOpen() {
			continue
		}
		_, err := r.CancelOrder(ctx, model.CancelOrder{
			AccountID:     sibling.AccountID,
			InstrumentID:  sibling.InstrumentID,
			OrderID:       sibling.OrderID,
			ClientOrderID: sibling.ClientOrderID,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func appendUniqueClientOrderID(ids []model.ClientOrderID, id model.ClientOrderID) []model.ClientOrderID {
	for _, existing := range ids {
		if existing == id {
			return ids
		}
	}
	return append(ids, id)
}

func (r *runtime) nextPosition(fill model.FillReport) model.PositionStatusReport {
	positionID := model.PositionID(fill.InstrumentID.String())
	existing, ok := r.cache.Position(fill.AccountID, positionID)
	if !ok {
		side := model.PositionSideLong
		if fill.Side == model.OrderSideSell {
			side = model.PositionSideShort
		}
		return model.PositionStatusReport{
			AccountID:    fill.AccountID,
			InstrumentID: fill.InstrumentID,
			PositionID:   positionID,
			Side:         side,
			Quantity:     fill.Quantity,
			EntryPrice:   fill.Price,
			Timestamp:    fill.Timestamp,
		}
	}
	currentSigned := signedPosition(existing)
	fillSigned := signedFill(fill)
	nextSigned := currentSigned.Add(fillSigned)
	nextQty := nextSigned.Abs()
	entry := fill.Price
	if currentSigned.Sign()*fillSigned.Sign() > 0 && nextQty.IsPositive() {
		entry = existing.EntryPrice.Mul(existing.Quantity).Add(fill.Price.Mul(fill.Quantity)).Div(nextQty)
	} else if currentSigned.Abs().GreaterThan(fillSigned.Abs()) {
		entry = existing.EntryPrice
	}
	return model.PositionStatusReport{
		AccountID:    fill.AccountID,
		InstrumentID: fill.InstrumentID,
		PositionID:   positionID,
		Side:         sideFromSigned(nextSigned),
		Quantity:     nextQty,
		EntryPrice:   entry,
		Timestamp:    fill.Timestamp,
	}
}

func (r *runtime) apply(ctx context.Context, event model.ExecutionEvent) error {
	positionLifecycle := r.derivePositionLifecycle(event)
	if err := r.reconciler.Apply(event); err != nil {
		return err
	}
	if err := r.dispatchExecution(ctx, event); err != nil {
		return err
	}
	if positionLifecycle != nil {
		return r.dispatchExecution(ctx, model.ExecutionEvent{PositionLifecycle: positionLifecycle})
	}
	return nil
}

func (r *runtime) dispatchExecution(ctx context.Context, event model.ExecutionEvent) error {
	if r.engine != nil {
		env := bus.Envelope{Topic: strategy.TopicExecution, Message: event, Timestamp: r.now()}
		if err := r.engine.Process(ctx, env); err != nil {
			return err
		}
	}
	return r.bus.Publish(ctx, strategy.TopicExecution, event)
}

func (r *runtime) derivePositionLifecycle(event model.ExecutionEvent) *model.PositionLifecycleEvent {
	if event.Position == nil {
		return nil
	}
	var previous *model.PositionStatusReport
	if existing, ok := r.cache.Position(event.Position.AccountID, event.Position.PositionID); ok {
		previous = &existing
	}
	lifecycle, ok := model.NewPositionLifecycleEvent(previous, *event.Position)
	if !ok {
		return nil
	}
	return &lifecycle
}

func openQuantity(order model.OrderStatusReport) decimal.Decimal {
	if order.LeavesQuantity.IsPositive() {
		return order.LeavesQuantity
	}
	return order.Quantity.Sub(order.FilledQuantity)
}

func canMatch(order model.OrderStatusReport, price decimal.Decimal) bool {
	if order.Type == model.OrderTypeMarket ||
		order.Type == model.OrderTypeMarketToLimit ||
		order.Type == model.OrderTypeStopMarket ||
		order.Type == model.OrderTypeMarketIfTouched ||
		order.Type == model.OrderTypeTrailingStopMarket {
		return true
	}
	if order.Side == model.OrderSideBuy {
		return price.LessThanOrEqual(order.Price)
	}
	return price.GreaterThanOrEqual(order.Price)
}

func isLimitTouch(order model.OrderStatusReport, price decimal.Decimal) bool {
	if !order.Price.IsPositive() {
		return false
	}
	switch order.Type {
	case model.OrderTypeLimit, model.OrderTypeStopLimit, model.OrderTypeLimitIfTouched, model.OrderTypeTrailingStopLimit:
		return price.Equal(order.Price)
	default:
		return false
	}
}

func (r *runtime) firstBookPrice(order model.OrderStatusReport) decimal.Decimal {
	book, ok := r.lastBooks[order.InstrumentID]
	if !ok {
		return decimal.Zero
	}
	if order.Side == model.OrderSideBuy && len(book.Asks) > 0 {
		return book.Asks[0].Price
	}
	if order.Side == model.OrderSideSell && len(book.Bids) > 0 {
		return book.Bids[0].Price
	}
	return decimal.Zero
}

func stopTrigger(order model.OrderStatusReport, price decimal.Decimal) bool {
	if order.Side == model.OrderSideBuy {
		return price.GreaterThanOrEqual(order.TriggerPrice)
	}
	return price.LessThanOrEqual(order.TriggerPrice)
}

func touchedTrigger(order model.OrderStatusReport, price decimal.Decimal) bool {
	if order.Side == model.OrderSideBuy {
		return price.LessThanOrEqual(order.TriggerPrice)
	}
	return price.GreaterThanOrEqual(order.TriggerPrice)
}

func trailingKey(order model.OrderStatusReport) string {
	return string(order.AccountID) + "/" + string(order.OrderID)
}

func fillBacktestModifyIdentity(modify model.ModifyOrder, existing model.OrderStatusReport) model.ModifyOrder {
	if modify.OrderID == "" {
		modify.OrderID = existing.OrderID
	}
	if modify.ClientOrderID == "" {
		modify.ClientOrderID = existing.ClientOrderID
	}
	if modify.VenueOrderID == "" {
		modify.VenueOrderID = existing.VenueOrderID
	}
	if modify.InstrumentID == (model.InstrumentID{}) {
		modify.InstrumentID = existing.InstrumentID
	}
	return modify
}

func fillBacktestCancelIdentity(cancel model.CancelOrder, existing model.OrderStatusReport) model.CancelOrder {
	if cancel.OrderID == "" {
		cancel.OrderID = existing.OrderID
	}
	if cancel.ClientOrderID == "" {
		cancel.ClientOrderID = existing.ClientOrderID
	}
	if cancel.InstrumentID == (model.InstrumentID{}) {
		cancel.InstrumentID = existing.InstrumentID
	}
	return cancel
}

func fillBacktestBatchCancelIdentity(cancel model.CancelOrder, accountID model.AccountID, instrumentID model.InstrumentID) model.CancelOrder {
	if cancel.AccountID == "" {
		cancel.AccountID = accountID
	}
	if cancel.InstrumentID == (model.InstrumentID{}) {
		cancel.InstrumentID = instrumentID
	}
	return cancel
}

func backtestOrderLifecycleFromReport(report model.OrderStatusReport, kind model.OrderEventKind, previous model.OrderStatus, reason string) model.OrderLifecycleEvent {
	return model.OrderLifecycleEvent{
		Metadata:       report.Metadata,
		AccountID:      report.AccountID,
		InstrumentID:   report.InstrumentID,
		OrderID:        report.OrderID,
		ClientOrderID:  report.ClientOrderID,
		VenueOrderID:   report.VenueOrderID,
		Kind:           kind,
		PreviousStatus: previous,
		Status:         report.Status,
		Reason:         reason,
		Report:         &report,
	}
}

func backtestOrderSubmittedLifecycle(order model.SubmitOrder) model.OrderLifecycleEvent {
	return model.OrderLifecycleEvent{
		Metadata:       order.Metadata,
		AccountID:      order.AccountID,
		InstrumentID:   order.InstrumentID,
		ClientOrderID:  order.ClientOrderID,
		Kind:           model.OrderEventSubmitted,
		PreviousStatus: model.OrderStatusInitialized,
		Status:         model.OrderStatusSubmitted,
	}
}

func ptrOrderLifecycle(event model.OrderLifecycleEvent) *model.OrderLifecycleEvent {
	return &event
}

func compactLevels(levels []model.OrderBookLevel) []model.OrderBookLevel {
	out := make([]model.OrderBookLevel, 0, len(levels))
	for _, level := range levels {
		if level.Size.IsPositive() {
			out = append(out, level)
		}
	}
	return out
}

func cloneBook(book model.OrderBook) model.OrderBook {
	book.Bids = append([]model.OrderBookLevel(nil), book.Bids...)
	book.Asks = append([]model.OrderBookLevel(nil), book.Asks...)
	return book
}

func signedPosition(position model.PositionStatusReport) decimal.Decimal {
	if position.Side == model.PositionSideShort {
		return position.Quantity.Neg()
	}
	return position.Quantity
}

func signedFill(fill model.FillReport) decimal.Decimal {
	if fill.Side == model.OrderSideSell {
		return fill.Quantity.Neg()
	}
	return fill.Quantity
}

func sideFromSigned(value decimal.Decimal) model.PositionSide {
	switch {
	case value.IsPositive():
		return model.PositionSideLong
	case value.IsNegative():
		return model.PositionSideShort
	default:
		return model.PositionSideFlat
	}
}
