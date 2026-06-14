package godemo

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/QuantProcessing/exchanges/cache"
	"github.com/QuantProcessing/exchanges/live"
	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/portfolio"
	"github.com/QuantProcessing/exchanges/risk"
	"github.com/QuantProcessing/exchanges/strategy"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/shopspring/decimal"
)

const (
	accountID     model.AccountID     = "demo-account"
	clientOrderID model.ClientOrderID = "demo-imbalance-1"
	instrumentRaw                     = "BTCUSDT"
)

var instrumentID = model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")

type DemoResult struct {
	SignalTriggered bool
	FinalOrder      model.OrderStatusReport
	Fills           []model.FillReport
	Position        model.PositionStatusReport
	Exposure        decimal.Decimal
	EventLog        []string
}

func RunDemo(ctx context.Context) (DemoResult, error) {
	c := cache.New()
	pf := portfolio.New(c)
	data := newDemoDataClient()
	exec := newDemoExecutionClient()
	strat := newImbalanceStrategy()
	node, err := live.NewTradingNode(live.NodeConfig{
		Cache:     c,
		Portfolio: pf,
		Risk: risk.NewEngine(c, risk.Config{
			MaxOrderNotional: decimal.RequireFromString("10"),
		}),
		DataClients:      []venue.DataClient{data},
		ExecutionClients: []venue.ExecutionClient{exec},
		Strategies:       []strategy.Strategy{strategy.NewTyped("go-demo-orderbook-imbalance", strat)},
	})
	if err != nil {
		return DemoResult{}, err
	}
	if err := node.Start(ctx); err != nil {
		return DemoResult{}, err
	}
	defer node.Stop(context.Background())

	data.EmitOrderBook(model.OrderBook{
		InstrumentID: instrumentID,
		Bids: []model.OrderBookLevel{
			{Price: decimal.RequireFromString("100"), Size: decimal.RequireFromString("3")},
		},
		Asks: []model.OrderBookLevel{
			{Price: decimal.RequireFromString("101"), Size: decimal.RequireFromString("1")},
		},
		Timestamp: time.Now(),
	})

	select {
	case <-strat.done:
	case <-ctx.Done():
		return DemoResult{}, ctx.Err()
	}

	order, ok := node.Cache().OrderByClientID(accountID, clientOrderID)
	if !ok {
		return DemoResult{}, fmt.Errorf("demo order not found in cache")
	}
	position, ok := node.Cache().PositionByInstrument(accountID, instrumentID)
	if !ok {
		return DemoResult{}, fmt.Errorf("demo position not found in cache")
	}
	return DemoResult{
		SignalTriggered: strat.signalTriggered(),
		FinalOrder:      order,
		Fills:           node.Cache().FillsForOrder(accountID, order.OrderID),
		Position:        position,
		Exposure:        node.Portfolio().Exposure(accountID, "USDT"),
		EventLog:        strat.eventLog(),
	}, nil
}

type imbalanceStrategy struct {
	mu        sync.Mutex
	runtime   strategy.Runtime
	submitted bool
	signaled  bool
	done      chan struct{}
	doneOnce  sync.Once
	events    []string
}

func newImbalanceStrategy() *imbalanceStrategy {
	return &imbalanceStrategy{done: make(chan struct{})}
}

func (s *imbalanceStrategy) OnStart(ctx context.Context, rt strategy.Runtime) error {
	s.runtime = rt
	return rt.SubscribeOrderBookDepth(ctx, instrumentID, 2)
}

func (s *imbalanceStrategy) OnStop(context.Context) error { return nil }

func (s *imbalanceStrategy) OnOrderBook(ctx context.Context, book model.OrderBook) error {
	s.record("market:order_book")
	if len(book.Bids) == 0 || len(book.Asks) == 0 {
		return nil
	}
	s.mu.Lock()
	if s.submitted {
		s.mu.Unlock()
		return nil
	}
	imbalanced := book.Bids[0].Size.GreaterThan(book.Asks[0].Size.Mul(decimal.NewFromInt(2)))
	if !imbalanced {
		s.mu.Unlock()
		return nil
	}
	s.submitted = true
	s.signaled = true
	s.mu.Unlock()
	order := s.runtime.OrderFactory(accountID).Limit(
		book.InstrumentID,
		model.OrderSideBuy,
		decimal.RequireFromString("0.01"),
		book.Asks[0].Price,
		model.WithClientOrderID(clientOrderID),
	)
	_, err := s.runtime.SubmitOrder(ctx, order)
	return err
}

func (s *imbalanceStrategy) OnOrderStatus(_ context.Context, report model.OrderStatusReport) error {
	s.record("execution:order:" + string(report.Status))
	return nil
}

func (s *imbalanceStrategy) OnOrderFilled(context.Context, model.FillReport) error {
	s.record("execution:fill")
	s.doneOnce.Do(func() { close(s.done) })
	return nil
}

func (s *imbalanceStrategy) record(event string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
}

func (s *imbalanceStrategy) eventLog() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.events))
	copy(out, s.events)
	return out
}

func (s *imbalanceStrategy) signalTriggered() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.signaled
}

type demoProvider struct {
	inst model.Instrument
}

func newDemoProvider() *demoProvider {
	return &demoProvider{inst: model.Instrument{
		ID:        instrumentID,
		RawSymbol: instrumentRaw,
		Type:      model.InstrumentTypeSpot,
		Base:      "BTC",
		Quote:     "USDT",
		PriceTick: decimal.RequireFromString("0.01"),
		SizeTick:  decimal.RequireFromString("0.0001"),
		Status:    model.InstrumentStatusTrading,
	}}
}

func (p *demoProvider) LoadAll(context.Context) error { return nil }
func (p *demoProvider) Get(id model.InstrumentID) (model.Instrument, bool) {
	return p.inst, p.inst.ID == id
}
func (p *demoProvider) List() []model.Instrument { return []model.Instrument{p.inst} }

type demoDataClient struct {
	provider *demoProvider
	events   chan model.MarketEvent
}

func newDemoDataClient() *demoDataClient {
	return &demoDataClient{provider: newDemoProvider(), events: make(chan model.MarketEvent, 8)}
}

func (c *demoDataClient) Venue() model.Venue                    { return "BINANCE" }
func (c *demoDataClient) ClientID() string                      { return "demo-binance-data" }
func (c *demoDataClient) Instruments() venue.InstrumentProvider { return c.provider }
func (c *demoDataClient) Connect(context.Context) error         { return nil }
func (c *demoDataClient) Disconnect(context.Context) error      { return nil }
func (c *demoDataClient) Health() venue.DataHealth {
	return venue.DataHealth{Connected: true, InstrumentReady: true}
}
func (c *demoDataClient) FetchTicker(context.Context, model.InstrumentID) (model.Ticker, error) {
	return model.Ticker{}, model.ErrNotSupported
}
func (c *demoDataClient) FetchOrderBook(context.Context, model.InstrumentID, int) (model.OrderBook, error) {
	return model.OrderBook{}, model.ErrNotSupported
}
func (c *demoDataClient) SubscribeMarketData(context.Context, model.SubscribeMarketData) error {
	return nil
}
func (c *demoDataClient) UnsubscribeMarketData(context.Context, model.SubscribeMarketData) error {
	return nil
}
func (c *demoDataClient) Events() <-chan model.MarketEvent { return c.events }
func (c *demoDataClient) EmitOrderBook(book model.OrderBook) {
	c.events <- model.MarketEvent{OrderBook: &book}
}

type demoExecutionClient struct {
	events chan model.ExecutionEvent
}

func newDemoExecutionClient() *demoExecutionClient {
	return &demoExecutionClient{events: make(chan model.ExecutionEvent, 8)}
}

func (c *demoExecutionClient) Venue() model.Venue               { return "BINANCE" }
func (c *demoExecutionClient) AccountID() model.AccountID       { return accountID }
func (c *demoExecutionClient) Connect(context.Context) error    { return nil }
func (c *demoExecutionClient) Disconnect(context.Context) error { return nil }
func (c *demoExecutionClient) Health() venue.ExecutionHealth {
	return venue.ExecutionHealth{Connected: true, AccountReady: true}
}
func (c *demoExecutionClient) QueryAccount(context.Context) (model.AccountSnapshot, error) {
	return model.AccountSnapshot{
		AccountID: accountID,
		Venue:     "BINANCE",
		Balances:  []model.Balance{{Currency: "USDT", Free: "100", Total: "100"}},
		Timestamp: time.Now(),
	}, nil
}
func (c *demoExecutionClient) SubmitOrder(_ context.Context, cmd model.SubmitOrder) (model.OrderStatusReport, error) {
	report := model.OrderStatusReport{
		AccountID:       cmd.AccountID,
		InstrumentID:    cmd.InstrumentID,
		OrderID:         "demo-order-1",
		ClientOrderID:   cmd.ClientOrderID,
		Status:          model.OrderStatusAccepted,
		Side:            cmd.Side,
		Type:            cmd.Type,
		Quantity:        cmd.Quantity,
		LeavesQuantity:  cmd.Quantity,
		Price:           cmd.Price,
		LastUpdatedTime: time.Now(),
	}
	go c.emitFill(cmd)
	return report, nil
}
func (c *demoExecutionClient) emitFill(cmd model.SubmitOrder) {
	time.Sleep(10 * time.Millisecond)
	now := time.Now()
	filled := model.OrderStatusReport{
		AccountID:       cmd.AccountID,
		InstrumentID:    cmd.InstrumentID,
		OrderID:         "demo-order-1",
		ClientOrderID:   cmd.ClientOrderID,
		Status:          model.OrderStatusFilled,
		Side:            cmd.Side,
		Type:            cmd.Type,
		Quantity:        cmd.Quantity,
		FilledQuantity:  cmd.Quantity,
		LeavesQuantity:  decimal.Zero,
		Price:           cmd.Price,
		AveragePrice:    cmd.Price,
		LastUpdatedTime: now,
	}
	fill := model.FillReport{
		AccountID:     cmd.AccountID,
		InstrumentID:  cmd.InstrumentID,
		OrderID:       "demo-order-1",
		ClientOrderID: cmd.ClientOrderID,
		TradeID:       "demo-trade-1",
		Side:          cmd.Side,
		Price:         cmd.Price,
		Quantity:      cmd.Quantity,
		Fee:           decimal.Zero,
		FeeCurrency:   "USDT",
		Timestamp:     now,
	}
	c.events <- model.ExecutionEvent{Order: &filled}
	c.events <- model.ExecutionEvent{Fill: &fill}
}
func (c *demoExecutionClient) CancelOrder(context.Context, model.CancelOrder) (model.OrderStatusReport, error) {
	return model.OrderStatusReport{}, model.ErrNotSupported
}
func (c *demoExecutionClient) GenerateOrderStatusReports(context.Context, model.InstrumentID) ([]model.OrderStatusReport, error) {
	return nil, nil
}
func (c *demoExecutionClient) Events() <-chan model.ExecutionEvent { return c.events }
