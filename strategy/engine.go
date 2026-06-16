package strategy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/QuantProcessing/exchanges/bus"
	"github.com/QuantProcessing/exchanges/cache"
	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/portfolio"
)

const (
	eventuallyWait = time.Second
	eventuallyTick = 10 * time.Millisecond
)

const (
	TopicExecution  = "execution"
	TopicMarketData = "market.data"
	TopicTimer      = "timer"
	TopicError      = "error"
)

type Runtime interface {
	Cache() *cache.Cache
	Portfolio() *portfolio.Portfolio
	Clock() Clock
	Logger() *slog.Logger
	SetTimer(context.Context, string, time.Duration) error
	CancelTimer(context.Context, string) error
	OrderFactory(model.AccountID) *model.OrderFactory
	SubscribeMarketData(context.Context, model.SubscribeMarketData) error
	UnsubscribeMarketData(context.Context, model.SubscribeMarketData) error
	SubscribeTicker(context.Context, model.InstrumentID) error
	UnsubscribeTicker(context.Context, model.InstrumentID) error
	SubscribeTradeTicks(context.Context, model.InstrumentID) error
	UnsubscribeTradeTicks(context.Context, model.InstrumentID) error
	SubscribeQuoteTicks(context.Context, model.InstrumentID) error
	UnsubscribeQuoteTicks(context.Context, model.InstrumentID) error
	SubscribeFundingRates(context.Context, model.InstrumentID) error
	UnsubscribeFundingRates(context.Context, model.InstrumentID) error
	SubscribeBars(context.Context, model.BarType) error
	UnsubscribeBars(context.Context, model.BarType) error
	SubscribeOrderBookDepth(context.Context, model.InstrumentID, int) error
	UnsubscribeOrderBookDepth(context.Context, model.InstrumentID, int) error
	RequestData(context.Context, model.DataRequest) (model.DataResponse, error)
	SubmitOrder(context.Context, model.SubmitOrder) (model.OrderStatusReport, error)
	SubmitOrderList(context.Context, model.OrderList) ([]model.OrderStatusReport, error)
	ModifyOrder(context.Context, model.ModifyOrder) (model.OrderStatusReport, error)
	CancelOrder(context.Context, model.CancelOrder) (model.OrderStatusReport, error)
	BatchCancelOrders(context.Context, model.BatchCancelOrders) ([]model.OrderStatusReport, error)
	CancelAllOrders(context.Context, model.CancelAllOrders) ([]model.OrderStatusReport, error)
	QueryOrder(context.Context, model.QueryOrder) (model.OrderStatusReport, error)
	QueryAccount(context.Context, model.QueryAccount) (model.AccountSnapshot, error)
}

type Strategy interface {
	ID() string
	OnStart(context.Context, Runtime) error
	OnEvent(context.Context, bus.Envelope) error
	OnStop(context.Context) error
}

type Engine struct {
	bus           *bus.Bus
	runtime       Runtime
	strategies    []Strategy
	actors        []*strategyActor
	identity      map[Strategy]model.StrategyID
	ids           map[model.StrategyID]struct{}
	traderID      model.TraderID
	subs          []bus.Subscription
	events        chan bus.Envelope
	errs          chan error
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	asyncDispatch bool
}

type Option func(*Engine)

type strategyActor struct {
	strategy Strategy
	id       model.StrategyID
	events   chan bus.Envelope
}

func WithRuntime(runtime Runtime) Option {
	return func(e *Engine) {
		e.runtime = runtime
	}
}

func WithTraderID(traderID model.TraderID) Option {
	return func(e *Engine) {
		e.traderID = traderID
	}
}

func WithSynchronousDispatch() Option {
	return func(e *Engine) {
		e.asyncDispatch = false
	}
}

func NewEngine(b *bus.Bus, opts ...Option) *Engine {
	if b == nil {
		b = bus.New()
	}
	e := &Engine{
		bus:           b,
		identity:      make(map[Strategy]model.StrategyID),
		ids:           make(map[model.StrategyID]struct{}),
		asyncDispatch: true,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

func (e *Engine) Add(s Strategy) error {
	if s == nil {
		return fmt.Errorf("%w: strategy is required", ErrInvalidStrategyConfig)
	}
	id := model.StrategyID(s.ID())
	if id == "" {
		return fmt.Errorf("%w: strategy id is required", ErrInvalidStrategyConfig)
	}
	if _, exists := e.ids[id]; exists {
		return fmt.Errorf("%w: duplicate strategy id %s", ErrInvalidStrategyConfig, id)
	}
	if e.identity == nil {
		e.identity = make(map[Strategy]model.StrategyID)
	}
	if e.ids == nil {
		e.ids = make(map[model.StrategyID]struct{})
	}
	e.identity[s] = id
	e.ids[id] = struct{}{}
	e.strategies = append(e.strategies, s)
	return nil
}

func (e *Engine) Start(ctx context.Context) error {
	runCtx, cancel := context.WithCancel(context.Background())
	e.cancel = cancel
	e.errs = make(chan error, 1)
	for _, s := range e.strategies {
		if err := s.OnStart(ctx, e.runtimeForStrategy(s)); err != nil {
			return err
		}
	}
	if !e.asyncDispatch {
		return nil
	}
	e.actors = make([]*strategyActor, 0, len(e.strategies))
	for _, s := range e.strategies {
		actor := &strategyActor{
			strategy: s,
			id:       e.strategyID(s),
			events:   make(chan bus.Envelope, 128),
		}
		e.actors = append(e.actors, actor)
		e.wg.Add(1)
		go e.runActor(runCtx, actor)
	}
	e.subs = []bus.Subscription{
		e.bus.Subscribe(TopicExecution, 64),
		e.bus.Subscribe(TopicMarketData, 64),
		e.bus.Subscribe(TopicTimer, 64),
		e.bus.Subscribe(TopicError, 64),
	}
	e.events = make(chan bus.Envelope, 128)
	e.wg.Add(1)
	go e.dispatch(runCtx)
	for _, sub := range e.subs {
		e.wg.Add(1)
		go e.forward(runCtx, sub)
	}
	return nil
}

func (e *Engine) runtimeForStrategy(s Strategy) Runtime {
	if e.runtime == nil || s == nil {
		return e.runtime
	}
	return commandMetadataRuntime{
		Runtime:    e.runtime,
		traderID:   e.traderID,
		strategyID: e.strategyID(s),
	}
}

type commandMetadataRuntime struct {
	Runtime
	traderID   model.TraderID
	strategyID model.StrategyID
}

func (r commandMetadataRuntime) commandMetadata(metadata model.CommandMetadata) model.CommandMetadata {
	defaults := model.CommandMetadata{
		TraderID:   r.traderID,
		StrategyID: r.strategyID,
		TsInit:     r.Clock().Now(),
	}
	return metadata.WithDefaults(defaults)
}

func (r commandMetadataRuntime) Logger() *slog.Logger {
	base := loggerOrDiscard(nil)
	if r.Runtime != nil {
		base = loggerOrDiscard(r.Runtime.Logger())
	}
	attrs := make([]any, 0, 4)
	if r.traderID != "" {
		attrs = append(attrs, "trader_id", string(r.traderID))
	}
	if r.strategyID != "" {
		attrs = append(attrs, "strategy_id", string(r.strategyID))
	}
	return base.With(attrs...)
}

func (r commandMetadataRuntime) RequestData(ctx context.Context, request model.DataRequest) (model.DataResponse, error) {
	request.Metadata = r.commandMetadata(request.Metadata)
	return r.Runtime.RequestData(ctx, request)
}

func (r commandMetadataRuntime) SubmitOrder(ctx context.Context, order model.SubmitOrder) (model.OrderStatusReport, error) {
	order.Metadata = r.commandMetadata(order.Metadata)
	return r.Runtime.SubmitOrder(ctx, order)
}

func (r commandMetadataRuntime) SubmitOrderList(ctx context.Context, list model.OrderList) ([]model.OrderStatusReport, error) {
	list.Metadata = r.commandMetadata(list.Metadata)
	list = list.WithCommandMetadataDefaults()
	return r.Runtime.SubmitOrderList(ctx, list)
}

func (r commandMetadataRuntime) ModifyOrder(ctx context.Context, modify model.ModifyOrder) (model.OrderStatusReport, error) {
	modify.Metadata = r.commandMetadata(modify.Metadata)
	return r.Runtime.ModifyOrder(ctx, modify)
}

func (r commandMetadataRuntime) CancelOrder(ctx context.Context, cancel model.CancelOrder) (model.OrderStatusReport, error) {
	cancel.Metadata = r.commandMetadata(cancel.Metadata)
	return r.Runtime.CancelOrder(ctx, cancel)
}

func (r commandMetadataRuntime) BatchCancelOrders(ctx context.Context, batch model.BatchCancelOrders) ([]model.OrderStatusReport, error) {
	batch.Metadata = r.commandMetadata(batch.Metadata)
	for i := range batch.Cancels {
		batch.Cancels[i].Metadata = batch.Cancels[i].Metadata.WithDefaults(batch.Metadata)
	}
	return r.Runtime.BatchCancelOrders(ctx, batch)
}

func (r commandMetadataRuntime) CancelAllOrders(ctx context.Context, cancelAll model.CancelAllOrders) ([]model.OrderStatusReport, error) {
	cancelAll.Metadata = r.commandMetadata(cancelAll.Metadata)
	return r.Runtime.CancelAllOrders(ctx, cancelAll)
}

func (r commandMetadataRuntime) QueryOrder(ctx context.Context, query model.QueryOrder) (model.OrderStatusReport, error) {
	query.Metadata = r.commandMetadata(query.Metadata)
	return r.Runtime.QueryOrder(ctx, query)
}

func (r commandMetadataRuntime) QueryAccount(ctx context.Context, query model.QueryAccount) (model.AccountSnapshot, error) {
	query.Metadata = r.commandMetadata(query.Metadata)
	return r.Runtime.QueryAccount(ctx, query)
}

func (e *Engine) forward(ctx context.Context, sub bus.Subscription) {
	defer e.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case env, ok := <-sub.C():
			if !ok {
				return
			}
			select {
			case <-ctx.Done():
				return
			case e.events <- env:
			}
		}
	}
}

func (e *Engine) dispatch(ctx context.Context) {
	defer e.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case env, ok := <-e.events:
			if !ok {
				return
			}
			for _, actor := range e.actors {
				select {
				case <-ctx.Done():
					return
				case actor.events <- env:
				}
			}
		}
	}
}

func (e *Engine) runActor(ctx context.Context, actor *strategyActor) {
	defer e.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case env, ok := <-actor.events:
			if !ok {
				return
			}
			if err := actor.strategy.OnEvent(ctx, env); err != nil {
				e.reportError(fmt.Errorf("strategy %s event handler failed: %w", actor.id, err))
			}
		}
	}
}

func (e *Engine) strategyID(s Strategy) model.StrategyID {
	if e == nil || s == nil {
		return ""
	}
	if id, ok := e.identity[s]; ok {
		return id
	}
	return model.StrategyID(s.ID())
}

func (e *Engine) Errors() <-chan error {
	if e == nil {
		return nil
	}
	return e.errs
}

func (e *Engine) reportError(err error) {
	if err == nil || e.errs == nil {
		return
	}
	select {
	case e.errs <- err:
	default:
	}
}

func (e *Engine) Process(ctx context.Context, env bus.Envelope) error {
	var processErr error
	for _, s := range e.strategies {
		processErr = errors.Join(processErr, s.OnEvent(ctx, env))
	}
	return processErr
}

func (e *Engine) Stop(ctx context.Context) error {
	if e.cancel != nil {
		e.cancel()
	}
	e.wg.Wait()
	for _, sub := range e.subs {
		if sub != nil {
			_ = sub.Close()
		}
	}
	var stopErr error
	for _, s := range e.strategies {
		stopErr = errors.Join(stopErr, s.OnStop(ctx))
	}
	return stopErr
}
