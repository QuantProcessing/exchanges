package nautilusstyle

import (
	"context"
	"fmt"
	"sync"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/strategy"
	"github.com/shopspring/decimal"
)

type BracketStrategyConfig struct {
	AccountID    model.AccountID
	InstrumentID model.InstrumentID
	Side         model.OrderSide
	Quantity     decimal.Decimal
	EntryPrice   decimal.Decimal
	TakeProfit   decimal.Decimal
	StopLoss     decimal.Decimal
	Depth        int
	CommandID    model.CommandID
}

func (c BracketStrategyConfig) Validate() error {
	if c.AccountID == "" {
		return fmt.Errorf("%w: account id is required", model.ErrInvalidAccount)
	}
	if err := c.InstrumentID.Validate(); err != nil {
		return err
	}
	if c.Side != model.OrderSideBuy && c.Side != model.OrderSideSell {
		return fmt.Errorf("%w: side must be buy or sell", model.ErrInvalidOrder)
	}
	if !c.Quantity.IsPositive() {
		return fmt.Errorf("%w: quantity must be positive", model.ErrInvalidOrder)
	}
	if !c.EntryPrice.IsPositive() || !c.TakeProfit.IsPositive() || !c.StopLoss.IsPositive() {
		return fmt.Errorf("%w: bracket prices must be positive", model.ErrInvalidOrder)
	}
	if c.Depth <= 0 {
		return fmt.Errorf("%w: depth must be positive", model.ErrInvalidMarketData)
	}
	return nil
}

type BracketStrategy struct {
	cfg BracketStrategyConfig

	mu         sync.Mutex
	runtime    strategy.Runtime
	submitted  bool
	orderList  model.OrderListID
	reports    []model.OrderStatusReport
	lifecycle  []model.OrderLifecycleEvent
	fillEvents []model.FillReport
}

func NewBracketStrategy(cfg BracketStrategyConfig) (*BracketStrategy, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &BracketStrategy{cfg: cfg}, nil
}

func (s *BracketStrategy) OnStart(ctx context.Context, rt strategy.Runtime) error {
	s.runtime = rt
	return rt.SubscribeOrderBookDepth(ctx, s.cfg.InstrumentID, s.cfg.Depth)
}

func (s *BracketStrategy) OnStop(context.Context) error {
	return nil
}

func (s *BracketStrategy) OnOrderBook(ctx context.Context, book model.OrderBook) error {
	if book.InstrumentID != s.cfg.InstrumentID || !s.entryTouched(book) {
		return nil
	}
	s.mu.Lock()
	if s.submitted {
		s.mu.Unlock()
		return nil
	}
	s.submitted = true
	s.mu.Unlock()

	list := s.runtime.OrderFactory(s.cfg.AccountID).Bracket(model.BracketOrderRequest{
		InstrumentID: s.cfg.InstrumentID,
		Side:         s.cfg.Side,
		Quantity:     s.cfg.Quantity,
		EntryPrice:   s.cfg.EntryPrice,
		TakeProfit:   s.cfg.TakeProfit,
		StopLoss:     s.cfg.StopLoss,
	})
	list.Metadata = model.CommandMetadata{CommandID: s.cfg.CommandID}
	reports, err := s.runtime.SubmitOrderList(ctx, list)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.orderList = list.ID
	s.reports = append(s.reports, reports...)
	s.mu.Unlock()
	return nil
}

func (s *BracketStrategy) OnOrderStatus(_ context.Context, report model.OrderStatusReport) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reports = append(s.reports, report)
	return nil
}

func (s *BracketStrategy) OnOrderLifecycle(_ context.Context, event model.OrderLifecycleEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lifecycle = append(s.lifecycle, event)
	return nil
}

func (s *BracketStrategy) OnOrderFilled(_ context.Context, fill model.FillReport) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fillEvents = append(s.fillEvents, fill)
	return nil
}

func (s *BracketStrategy) Submitted() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.submitted
}

func (s *BracketStrategy) OrderListID() model.OrderListID {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.orderList
}

func (s *BracketStrategy) Reports() []model.OrderStatusReport {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]model.OrderStatusReport(nil), s.reports...)
}

func (s *BracketStrategy) LifecycleEvents() []model.OrderLifecycleEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]model.OrderLifecycleEvent(nil), s.lifecycle...)
}

func (s *BracketStrategy) Fills() []model.FillReport {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]model.FillReport(nil), s.fillEvents...)
}

func (s *BracketStrategy) entryTouched(book model.OrderBook) bool {
	switch s.cfg.Side {
	case model.OrderSideBuy:
		return len(book.Asks) > 0 && book.Asks[0].Price.LessThanOrEqual(s.cfg.EntryPrice)
	case model.OrderSideSell:
		return len(book.Bids) > 0 && book.Bids[0].Price.GreaterThanOrEqual(s.cfg.EntryPrice)
	default:
		return false
	}
}
