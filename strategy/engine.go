package strategy

import (
	"context"
	"errors"
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
)

type Runtime interface {
	Cache() *cache.Cache
	Portfolio() *portfolio.Portfolio
	Clock() Clock
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
	SubscribeBars(context.Context, model.BarType) error
	UnsubscribeBars(context.Context, model.BarType) error
	SubscribeOrderBookDepth(context.Context, model.InstrumentID, int) error
	UnsubscribeOrderBookDepth(context.Context, model.InstrumentID, int) error
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
	subs          []bus.Subscription
	events        chan bus.Envelope
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	asyncDispatch bool
}

type Option func(*Engine)

func WithRuntime(runtime Runtime) Option {
	return func(e *Engine) {
		e.runtime = runtime
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
	e := &Engine{bus: b, asyncDispatch: true}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

func (e *Engine) Add(s Strategy) error {
	e.strategies = append(e.strategies, s)
	return nil
}

func (e *Engine) Start(ctx context.Context) error {
	runCtx, cancel := context.WithCancel(context.Background())
	e.cancel = cancel
	for _, s := range e.strategies {
		if err := s.OnStart(ctx, e.runtime); err != nil {
			return err
		}
	}
	if !e.asyncDispatch {
		return nil
	}
	e.subs = []bus.Subscription{
		e.bus.Subscribe(TopicExecution, 64),
		e.bus.Subscribe(TopicMarketData, 64),
		e.bus.Subscribe(TopicTimer, 64),
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
		case env := <-e.events:
			for _, s := range e.strategies {
				_ = s.OnEvent(ctx, env)
			}
		}
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
