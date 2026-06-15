package strategy

import (
	"context"
	"errors"

	"github.com/QuantProcessing/exchanges/bus"
	"github.com/QuantProcessing/exchanges/model"
)

type TypedStrategy struct {
	id      string
	handler any
	runtime Runtime
}

func NewTyped(id string, handler any) *TypedStrategy {
	return &TypedStrategy{id: id, handler: handler}
}

func NewTypedWithConfig(cfg StrategyConfig, handler any) (*TypedStrategy, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return NewTyped(string(cfg.ID), handler), nil
}

func (s *TypedStrategy) ID() string {
	if s.id != "" {
		return s.id
	}
	if h, ok := s.handler.(interface{ ID() string }); ok {
		return h.ID()
	}
	return "typed-strategy"
}

func (s *TypedStrategy) OnStart(ctx context.Context, rt Runtime) error {
	s.runtime = rt
	if h, ok := s.handler.(interface {
		OnStart(context.Context, Runtime) error
	}); ok {
		return h.OnStart(ctx, rt)
	}
	return nil
}

func (s *TypedStrategy) OnEvent(ctx context.Context, env bus.Envelope) error {
	return errors.Join(s.dispatchTyped(ctx, env), s.dispatchRaw(ctx, env))
}

func (s *TypedStrategy) OnStop(ctx context.Context) error {
	if h, ok := s.handler.(interface{ OnStop(context.Context) error }); ok {
		return h.OnStop(ctx)
	}
	return nil
}

func (s *TypedStrategy) dispatchRaw(ctx context.Context, env bus.Envelope) error {
	if h, ok := s.handler.(interface {
		OnEvent(context.Context, bus.Envelope) error
	}); ok {
		return h.OnEvent(ctx, env)
	}
	return nil
}

func (s *TypedStrategy) dispatchTyped(ctx context.Context, env bus.Envelope) error {
	switch msg := env.Message.(type) {
	case model.MarketEvent:
		return s.dispatchMarket(ctx, msg)
	case model.ExecutionEvent:
		return s.dispatchExecution(ctx, msg)
	case TimerEvent:
		return s.dispatchTimer(ctx, msg)
	case ErrorEvent:
		return s.dispatchError(ctx, msg)
	case error:
		return s.dispatchError(ctx, ErrorEvent{Err: msg})
	default:
		return nil
	}
}

func (s *TypedStrategy) dispatchError(ctx context.Context, event ErrorEvent) error {
	if h, ok := s.handler.(interface {
		OnError(context.Context, ErrorEvent) error
	}); ok {
		return h.OnError(ctx, event)
	}
	return nil
}

func (s *TypedStrategy) dispatchTimer(ctx context.Context, event TimerEvent) error {
	if h, ok := s.handler.(interface {
		OnTimer(context.Context, TimerEvent) error
	}); ok {
		return h.OnTimer(ctx, event)
	}
	return nil
}

func (s *TypedStrategy) dispatchMarket(ctx context.Context, event model.MarketEvent) error {
	var dispatchErr error
	if event.Ticker != nil {
		if h, ok := s.handler.(interface {
			OnTicker(context.Context, model.Ticker) error
		}); ok {
			dispatchErr = errors.Join(dispatchErr, h.OnTicker(ctx, *event.Ticker))
		}
	}
	if event.OrderBook != nil {
		if h, ok := s.handler.(interface {
			OnOrderBook(context.Context, model.OrderBook) error
		}); ok {
			dispatchErr = errors.Join(dispatchErr, h.OnOrderBook(ctx, *event.OrderBook))
		}
	}
	if event.Trade != nil {
		if h, ok := s.handler.(interface {
			OnTradeTick(context.Context, model.TradeTick) error
		}); ok {
			dispatchErr = errors.Join(dispatchErr, h.OnTradeTick(ctx, *event.Trade))
		}
	}
	if event.Quote != nil {
		if h, ok := s.handler.(interface {
			OnQuoteTick(context.Context, model.QuoteTick) error
		}); ok {
			dispatchErr = errors.Join(dispatchErr, h.OnQuoteTick(ctx, *event.Quote))
		}
	}
	if event.Bar != nil {
		if h, ok := s.handler.(interface {
			OnBar(context.Context, model.Bar) error
		}); ok {
			dispatchErr = errors.Join(dispatchErr, h.OnBar(ctx, *event.Bar))
		}
	}
	if event.Custom != nil {
		if h, ok := s.handler.(interface {
			OnCustomData(context.Context, model.CustomData) error
		}); ok {
			dispatchErr = errors.Join(dispatchErr, h.OnCustomData(ctx, *event.Custom))
		}
	}
	return dispatchErr
}

func (s *TypedStrategy) dispatchExecution(ctx context.Context, event model.ExecutionEvent) error {
	var dispatchErr error
	if event.Account != nil {
		if h, ok := s.handler.(interface {
			OnAccount(context.Context, model.AccountSnapshot) error
		}); ok {
			dispatchErr = errors.Join(dispatchErr, h.OnAccount(ctx, *event.Account))
		}
	}
	if event.Order != nil {
		dispatchErr = errors.Join(dispatchErr, s.dispatchOrder(ctx, *event.Order))
	}
	if event.Lifecycle != nil {
		dispatchErr = errors.Join(dispatchErr, s.dispatchLifecycle(ctx, *event.Lifecycle))
	}
	if event.Fill != nil {
		if h, ok := s.handler.(interface {
			OnOrderFilled(context.Context, model.FillReport) error
		}); ok {
			dispatchErr = errors.Join(dispatchErr, h.OnOrderFilled(ctx, *event.Fill))
		}
	}
	if event.Position != nil {
		if h, ok := s.handler.(interface {
			OnPosition(context.Context, model.PositionStatusReport) error
		}); ok {
			dispatchErr = errors.Join(dispatchErr, h.OnPosition(ctx, *event.Position))
		}
	}
	if event.PositionLifecycle != nil {
		dispatchErr = errors.Join(dispatchErr, s.dispatchPositionLifecycle(ctx, *event.PositionLifecycle))
	}
	return dispatchErr
}

func (s *TypedStrategy) dispatchPositionLifecycle(ctx context.Context, event model.PositionLifecycleEvent) error {
	var dispatchErr error
	if h, ok := s.handler.(interface {
		OnPositionLifecycle(context.Context, model.PositionLifecycleEvent) error
	}); ok {
		dispatchErr = errors.Join(dispatchErr, h.OnPositionLifecycle(ctx, event))
	}
	switch event.Kind {
	case model.PositionEventOpened:
		if h, ok := s.handler.(interface {
			OnPositionOpened(context.Context, model.PositionLifecycleEvent) error
		}); ok {
			dispatchErr = errors.Join(dispatchErr, h.OnPositionOpened(ctx, event))
		}
	case model.PositionEventChanged:
		if h, ok := s.handler.(interface {
			OnPositionChanged(context.Context, model.PositionLifecycleEvent) error
		}); ok {
			dispatchErr = errors.Join(dispatchErr, h.OnPositionChanged(ctx, event))
		}
	case model.PositionEventClosed:
		if h, ok := s.handler.(interface {
			OnPositionClosed(context.Context, model.PositionLifecycleEvent) error
		}); ok {
			dispatchErr = errors.Join(dispatchErr, h.OnPositionClosed(ctx, event))
		}
	}
	return dispatchErr
}

func (s *TypedStrategy) dispatchLifecycle(ctx context.Context, event model.OrderLifecycleEvent) error {
	var dispatchErr error
	if h, ok := s.handler.(interface {
		OnOrderLifecycle(context.Context, model.OrderLifecycleEvent) error
	}); ok {
		dispatchErr = errors.Join(dispatchErr, h.OnOrderLifecycle(ctx, event))
	}
	switch event.Kind {
	case model.OrderEventDenied:
		if h, ok := s.handler.(interface {
			OnOrderDenied(context.Context, model.OrderLifecycleEvent) error
		}); ok {
			dispatchErr = errors.Join(dispatchErr, h.OnOrderDenied(ctx, event))
		}
	case model.OrderEventEmulated:
		if h, ok := s.handler.(interface {
			OnOrderEmulated(context.Context, model.OrderLifecycleEvent) error
		}); ok {
			dispatchErr = errors.Join(dispatchErr, h.OnOrderEmulated(ctx, event))
		}
	case model.OrderEventReleased:
		if h, ok := s.handler.(interface {
			OnOrderReleased(context.Context, model.OrderLifecycleEvent) error
		}); ok {
			dispatchErr = errors.Join(dispatchErr, h.OnOrderReleased(ctx, event))
		}
	case model.OrderEventTriggered:
		if h, ok := s.handler.(interface {
			OnOrderTriggered(context.Context, model.OrderLifecycleEvent) error
		}); ok {
			dispatchErr = errors.Join(dispatchErr, h.OnOrderTriggered(ctx, event))
		}
	case model.OrderEventPendingUpdate:
		if h, ok := s.handler.(interface {
			OnOrderPendingUpdate(context.Context, model.OrderLifecycleEvent) error
		}); ok {
			dispatchErr = errors.Join(dispatchErr, h.OnOrderPendingUpdate(ctx, event))
		}
	case model.OrderEventUpdated:
		if h, ok := s.handler.(interface {
			OnOrderUpdated(context.Context, model.OrderLifecycleEvent) error
		}); ok {
			dispatchErr = errors.Join(dispatchErr, h.OnOrderUpdated(ctx, event))
		}
	case model.OrderEventModifyRejected:
		if h, ok := s.handler.(interface {
			OnOrderModifyRejected(context.Context, model.OrderLifecycleEvent) error
		}); ok {
			dispatchErr = errors.Join(dispatchErr, h.OnOrderModifyRejected(ctx, event))
		}
	case model.OrderEventPendingCancel:
		if h, ok := s.handler.(interface {
			OnOrderPendingCancel(context.Context, model.OrderLifecycleEvent) error
		}); ok {
			dispatchErr = errors.Join(dispatchErr, h.OnOrderPendingCancel(ctx, event))
		}
	case model.OrderEventCancelRejected:
		if h, ok := s.handler.(interface {
			OnOrderCancelRejected(context.Context, model.OrderLifecycleEvent) error
		}); ok {
			dispatchErr = errors.Join(dispatchErr, h.OnOrderCancelRejected(ctx, event))
		}
	case model.OrderEventExpired:
		if h, ok := s.handler.(interface {
			OnOrderExpired(context.Context, model.OrderLifecycleEvent) error
		}); ok {
			dispatchErr = errors.Join(dispatchErr, h.OnOrderExpired(ctx, event))
		}
	}
	return dispatchErr
}

func (s *TypedStrategy) dispatchOrder(ctx context.Context, report model.OrderStatusReport) error {
	var dispatchErr error
	if h, ok := s.handler.(interface {
		OnOrderStatus(context.Context, model.OrderStatusReport) error
	}); ok {
		dispatchErr = errors.Join(dispatchErr, h.OnOrderStatus(ctx, report))
	}
	switch report.Status {
	case model.OrderStatusSubmitted:
		if h, ok := s.handler.(interface {
			OnOrderSubmitted(context.Context, model.OrderStatusReport) error
		}); ok {
			dispatchErr = errors.Join(dispatchErr, h.OnOrderSubmitted(ctx, report))
		}
	case model.OrderStatusAccepted:
		if h, ok := s.handler.(interface {
			OnOrderAccepted(context.Context, model.OrderStatusReport) error
		}); ok {
			dispatchErr = errors.Join(dispatchErr, h.OnOrderAccepted(ctx, report))
		}
	case model.OrderStatusPartiallyFilled:
		if h, ok := s.handler.(interface {
			OnOrderPartiallyFilled(context.Context, model.OrderStatusReport) error
		}); ok {
			dispatchErr = errors.Join(dispatchErr, h.OnOrderPartiallyFilled(ctx, report))
		}
	case model.OrderStatusCanceled:
		if h, ok := s.handler.(interface {
			OnOrderCanceled(context.Context, model.OrderStatusReport) error
		}); ok {
			dispatchErr = errors.Join(dispatchErr, h.OnOrderCanceled(ctx, report))
		}
	case model.OrderStatusRejected:
		if h, ok := s.handler.(interface {
			OnOrderRejected(context.Context, model.OrderStatusReport) error
		}); ok {
			dispatchErr = errors.Join(dispatchErr, h.OnOrderRejected(ctx, report))
		}
	}
	return dispatchErr
}
