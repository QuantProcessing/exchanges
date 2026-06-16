package examples

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/QuantProcessing/exchanges/cache"
	"github.com/QuantProcessing/exchanges/live"
	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/risk"
	"github.com/QuantProcessing/exchanges/strategy"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/shopspring/decimal"
)

type LiveNodeResult struct {
	SubmittedOrder model.OrderStatusReport
	Fills          []model.FillReport
	Position       model.PositionStatusReport
	ExposureUSDT   decimal.Decimal
	EventLog       []string
}

// RunLiveNodeWithInMemoryVenue is a full local live-node assembly. It is useful
// for paper trading, integration tests, and adapter development because the
// strategy, risk, execution, cache, portfolio, and callbacks are real; only the
// venue network edge is replaced by in-memory clients.
func RunLiveNodeWithInMemoryVenue(ctx context.Context) (LiveNodeResult, error) {
	accountID := model.AccountID("live-memory-account")
	instrumentID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	c := cache.New()
	dataClient := newMemoryDataClient(instrumentID)
	executionClient := newMemoryExecutionClient(accountID, instrumentID)
	strat := newLiveNodeStrategy(accountID, instrumentID)

	node, err := live.NewNodeBuilder().
		WithCache(c).
		WithRiskConfig(risk.Config{
			MaxOrderNotional: decimal.RequireFromString("10"),
			ExposureCurrency: "USDT",
		}).
		AddDataClient(dataClient).
		AddExecutionClient(executionClient).
		AddStrategy(strategy.NewTyped("live-memory-imbalance", strat)).
		Build()
	if err != nil {
		return LiveNodeResult{}, err
	}
	if err := node.Start(ctx); err != nil {
		return LiveNodeResult{}, err
	}
	defer node.Stop(context.Background())

	dataClient.EmitOrderBook(memoryOrderBook(instrumentID, "100.00", "101.00", "3", "1"))

	select {
	case <-strat.done:
	case <-ctx.Done():
		return LiveNodeResult{}, ctx.Err()
	}

	order, ok := node.Cache().OrderByClientID(accountID, "live-memory-1")
	if !ok {
		return LiveNodeResult{}, fmt.Errorf("submitted order not found")
	}
	position, ok := node.Cache().PositionByInstrument(accountID, instrumentID)
	if !ok {
		return LiveNodeResult{}, fmt.Errorf("position not found")
	}
	return LiveNodeResult{
		SubmittedOrder: order,
		Fills:          node.Cache().FillsForOrder(accountID, order.OrderID),
		Position:       position,
		ExposureUSDT:   node.Portfolio().Exposure(accountID, "USDT"),
		EventLog:       strat.EventLog(),
	}, nil
}

type liveNodeStrategy struct {
	mu           sync.Mutex
	runtime      strategy.Runtime
	accountID    model.AccountID
	instrumentID model.InstrumentID
	submitted    bool
	done         chan struct{}
	doneOnce     sync.Once
	events       []string
}

func newLiveNodeStrategy(accountID model.AccountID, instrumentID model.InstrumentID) *liveNodeStrategy {
	return &liveNodeStrategy{accountID: accountID, instrumentID: instrumentID, done: make(chan struct{})}
}

func (s *liveNodeStrategy) OnStart(ctx context.Context, rt strategy.Runtime) error {
	s.runtime = rt
	return rt.SubscribeOrderBookDepth(ctx, s.instrumentID, 2)
}

func (s *liveNodeStrategy) OnOrderBook(ctx context.Context, book model.OrderBook) error {
	s.record("market:order_book")
	if book.InstrumentID != s.instrumentID || len(book.Bids) == 0 || len(book.Asks) == 0 {
		return nil
	}
	imbalanced := book.Bids[0].Size.GreaterThan(book.Asks[0].Size.Mul(decimal.NewFromInt(2)))
	if !imbalanced {
		return nil
	}

	s.mu.Lock()
	if s.submitted {
		s.mu.Unlock()
		return nil
	}
	s.submitted = true
	s.mu.Unlock()

	order := s.runtime.OrderFactory(s.accountID).Limit(
		book.InstrumentID,
		model.OrderSideBuy,
		decimal.RequireFromString("0.01"),
		book.Asks[0].Price,
		model.WithClientOrderID("live-memory-1"),
	)
	_, err := s.runtime.SubmitOrder(ctx, order)
	return err
}

func (s *liveNodeStrategy) OnOrderStatus(_ context.Context, report model.OrderStatusReport) error {
	s.record("execution:order:" + string(report.Status))
	return nil
}

func (s *liveNodeStrategy) OnOrderFilled(_ context.Context, fill model.FillReport) error {
	s.record("execution:fill:" + string(fill.TradeID))
	s.doneOnce.Do(func() { close(s.done) })
	return nil
}

func (s *liveNodeStrategy) EventLog() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.events...)
}

func (s *liveNodeStrategy) record(event string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
}

type memoryInstrumentProvider struct {
	instrument model.Instrument
}

func newMemoryInstrumentProvider(instrumentID model.InstrumentID) *memoryInstrumentProvider {
	return &memoryInstrumentProvider{instrument: backtestSpotInstrument(instrumentID)}
}

func (p *memoryInstrumentProvider) LoadAll(context.Context) error { return nil }
func (p *memoryInstrumentProvider) Get(id model.InstrumentID) (model.Instrument, bool) {
	return p.instrument, p.instrument.ID == id
}
func (p *memoryInstrumentProvider) List() []model.Instrument {
	return []model.Instrument{p.instrument}
}

type memoryDataClient struct {
	mu            sync.Mutex
	provider      *memoryInstrumentProvider
	events        chan model.MarketEvent
	subscriptions []model.SubscribeMarketData
}

func newMemoryDataClient(instrumentID model.InstrumentID) *memoryDataClient {
	return &memoryDataClient{
		provider: newMemoryInstrumentProvider(instrumentID),
		events:   make(chan model.MarketEvent, 8),
	}
}

func (c *memoryDataClient) Venue() model.Venue                    { return c.provider.instrument.ID.Venue }
func (c *memoryDataClient) ClientID() string                      { return "memory-data" }
func (c *memoryDataClient) Instruments() venue.InstrumentProvider { return c.provider }
func (c *memoryDataClient) Connect(context.Context) error         { return nil }
func (c *memoryDataClient) Disconnect(context.Context) error      { return nil }
func (c *memoryDataClient) Health() venue.DataHealth {
	return venue.DataHealth{Connected: true, InstrumentReady: true, LastEventTime: time.Now()}
}
func (c *memoryDataClient) FetchTicker(context.Context, model.InstrumentID) (model.Ticker, error) {
	return model.Ticker{}, model.ErrNotSupported
}
func (c *memoryDataClient) FetchOrderBook(context.Context, model.InstrumentID, int) (model.OrderBook, error) {
	return model.OrderBook{}, model.ErrNotSupported
}
func (c *memoryDataClient) SubscribeMarketData(_ context.Context, sub model.SubscribeMarketData) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.subscriptions = append(c.subscriptions, sub)
	return nil
}
func (c *memoryDataClient) UnsubscribeMarketData(context.Context, model.SubscribeMarketData) error {
	return nil
}
func (c *memoryDataClient) Events() <-chan model.MarketEvent { return c.events }
func (c *memoryDataClient) EmitOrderBook(book model.OrderBook) {
	c.events <- model.MarketEvent{OrderBook: &book}
}

type memoryExecutionClient struct {
	mu           sync.Mutex
	accountID    model.AccountID
	instrumentID model.InstrumentID
	events       chan model.ExecutionEvent
	nextOrder    int
	nextTrade    int
}

func newMemoryExecutionClient(accountID model.AccountID, instrumentID model.InstrumentID) *memoryExecutionClient {
	return &memoryExecutionClient{accountID: accountID, instrumentID: instrumentID, events: make(chan model.ExecutionEvent, 8)}
}

func (c *memoryExecutionClient) Venue() model.Venue               { return c.instrumentID.Venue }
func (c *memoryExecutionClient) AccountID() model.AccountID       { return c.accountID }
func (c *memoryExecutionClient) Connect(context.Context) error    { return nil }
func (c *memoryExecutionClient) Disconnect(context.Context) error { return nil }
func (c *memoryExecutionClient) Health() venue.ExecutionHealth {
	return venue.ExecutionHealth{Connected: true, AccountReady: true, LastEventTime: time.Now()}
}
func (c *memoryExecutionClient) QueryAccount(context.Context) (model.AccountSnapshot, error) {
	return model.AccountSnapshot{
		AccountID:    c.accountID,
		Venue:        c.instrumentID.Venue,
		Type:         model.AccountTypeCash,
		BaseCurrency: "USDT",
		Balances: []model.Balance{{
			Currency: "USDT",
			Free:     "1000",
			Locked:   "0",
			Total:    "1000",
		}},
		Timestamp: time.Now(),
	}, nil
}
func (c *memoryExecutionClient) SubmitOrder(_ context.Context, order model.SubmitOrder) (model.OrderStatusReport, error) {
	c.mu.Lock()
	c.nextOrder++
	orderID := model.OrderID(fmt.Sprintf("memory-order-%d", c.nextOrder))
	c.mu.Unlock()

	accepted := model.OrderStatusReport{
		Metadata:        order.Metadata,
		AccountID:       order.AccountID,
		InstrumentID:    order.InstrumentID,
		OrderID:         orderID,
		ClientOrderID:   order.ClientOrderID,
		Status:          model.OrderStatusAccepted,
		Side:            order.Side,
		Type:            order.Type,
		Quantity:        order.Quantity,
		LeavesQuantity:  order.Quantity,
		Price:           order.Price,
		TimeInForce:     order.TimeInForce,
		LastUpdatedTime: time.Now(),
	}
	go c.emitFill(order, orderID)
	return accepted, nil
}
func (c *memoryExecutionClient) CancelOrder(_ context.Context, cancel model.CancelOrder) (model.OrderStatusReport, error) {
	return model.OrderStatusReport{
		AccountID:       cancel.AccountID,
		InstrumentID:    cancel.InstrumentID,
		OrderID:         cancel.OrderID,
		ClientOrderID:   cancel.ClientOrderID,
		Status:          model.OrderStatusCanceled,
		LastUpdatedTime: time.Now(),
	}, nil
}
func (c *memoryExecutionClient) GenerateOrderStatusReports(context.Context, model.InstrumentID) ([]model.OrderStatusReport, error) {
	return nil, nil
}
func (c *memoryExecutionClient) Events() <-chan model.ExecutionEvent { return c.events }

func (c *memoryExecutionClient) emitFill(order model.SubmitOrder, orderID model.OrderID) {
	time.Sleep(10 * time.Millisecond)
	c.mu.Lock()
	c.nextTrade++
	tradeID := model.TradeID(fmt.Sprintf("memory-trade-%d", c.nextTrade))
	c.mu.Unlock()
	now := time.Now()
	filled := model.OrderStatusReport{
		Metadata:        order.Metadata,
		AccountID:       order.AccountID,
		InstrumentID:    order.InstrumentID,
		OrderID:         orderID,
		ClientOrderID:   order.ClientOrderID,
		Status:          model.OrderStatusFilled,
		Side:            order.Side,
		Type:            order.Type,
		Quantity:        order.Quantity,
		FilledQuantity:  order.Quantity,
		LeavesQuantity:  decimal.Zero,
		Price:           order.Price,
		AveragePrice:    order.Price,
		TimeInForce:     order.TimeInForce,
		LastUpdatedTime: now,
	}
	fill := model.FillReport{
		AccountID:     order.AccountID,
		InstrumentID:  order.InstrumentID,
		OrderID:       orderID,
		ClientOrderID: order.ClientOrderID,
		TradeID:       tradeID,
		Side:          order.Side,
		Price:         order.Price,
		Quantity:      order.Quantity,
		Fee:           decimal.Zero,
		FeeCurrency:   "USDT",
		Timestamp:     now,
	}
	c.events <- model.ExecutionEvent{Order: &filled}
	c.events <- model.ExecutionEvent{Fill: &fill}
}

func memoryOrderBook(instrumentID model.InstrumentID, bidPrice string, askPrice string, bidSize string, askSize string) model.OrderBook {
	return model.OrderBook{
		InstrumentID: instrumentID,
		Bids: []model.OrderBookLevel{{
			Price: decimal.RequireFromString(bidPrice),
			Size:  decimal.RequireFromString(bidSize),
		}},
		Asks: []model.OrderBookLevel{{
			Price: decimal.RequireFromString(askPrice),
			Size:  decimal.RequireFromString(askSize),
		}},
		Timestamp: time.Now(),
	}
}
