package nautilusstyle

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/QuantProcessing/exchanges/backtest"
	"github.com/QuantProcessing/exchanges/cache"
	"github.com/QuantProcessing/exchanges/live"
	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/strategy"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestBracketStrategyRunsInBacktestHarness(t *testing.T) {
	cfg := exampleBracketConfig("example-account")
	strat, err := NewBracketStrategy(cfg)
	require.NoError(t, err)

	engine := backtest.NewEngine(backtest.EngineConfig{})
	engine.AddStrategy(strategy.NewTyped("example-bracket", strat))
	engine.AddData(exampleBookEvent(10, "100", "101"))
	engine.AddData(exampleBookEvent(11, "110", "111"))

	result, err := engine.Run(context.Background())
	require.NoError(t, err)
	require.True(t, strat.Submitted())
	require.NotEmpty(t, strat.OrderListID())

	entry, ok := result.Cache.OrderByClientID(cfg.AccountID, "example-account-1")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusFilled, entry.Status)
	require.Equal(t, cfg.CommandID, entry.Metadata.CommandID)
	require.Equal(t, model.StrategyID("example-bracket"), entry.Metadata.StrategyID)

	stopLoss, ok := result.Cache.OrderByClientID(cfg.AccountID, "example-account-2")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusCanceled, stopLoss.Status)

	takeProfit, ok := result.Cache.OrderByClientID(cfg.AccountID, "example-account-3")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusFilled, takeProfit.Status)
	require.NotEmpty(t, strat.Fills())
}

func TestBracketStrategyRunsInLiveHarness(t *testing.T) {
	cfg := exampleBracketConfig("live-account")
	strat, err := NewBracketStrategy(cfg)
	require.NoError(t, err)
	data := newExampleDataClient(cfg.InstrumentID)
	exec := newExampleExecutionClient(cfg.AccountID, cfg.InstrumentID)
	node, err := live.NewTradingNode(live.NodeConfig{
		Cache:            cache.New(),
		DataClients:      []venue.DataClient{data},
		ExecutionClients: []venue.ExecutionClient{exec},
		Strategies:       []strategy.Strategy{strategy.NewTyped("example-bracket-live", strat)},
	})
	require.NoError(t, err)
	require.NoError(t, node.Start(context.Background()))
	defer node.Stop(context.Background())

	data.EmitOrderBook(exampleOrderBook(cfg.InstrumentID, "100", "101", time.Unix(20, 0)))
	require.Eventually(t, func() bool {
		order, ok := exec.LastSubmit()
		return ok &&
			order.ClientOrderID == "live-account-1" &&
			order.OrderListID != "" &&
			order.Metadata.CommandID == cfg.CommandID &&
			order.Metadata.StrategyID == "example-bracket-live"
	}, time.Second, 10*time.Millisecond)

	cached, ok := node.Cache().OrderByClientID(cfg.AccountID, "live-account-1")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusAccepted, cached.Status)
	require.True(t, strat.Submitted())
}

func exampleBracketConfig(accountID model.AccountID) BracketStrategyConfig {
	return BracketStrategyConfig{
		AccountID:    accountID,
		InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
		Side:         model.OrderSideBuy,
		Quantity:     decimal.RequireFromString("1"),
		EntryPrice:   decimal.RequireFromString("101"),
		TakeProfit:   decimal.RequireFromString("110"),
		StopLoss:     decimal.RequireFromString("99"),
		Depth:        2,
		CommandID:    "example-bracket-command",
	}
}

func exampleBookEvent(ts int64, bid string, ask string) backtest.Event {
	instID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	at := time.Unix(ts, 0)
	return backtest.Event{
		At:    at,
		Topic: strategy.TopicMarketData,
		Message: model.MarketEvent{
			OrderBook: ptrOrderBook(exampleOrderBook(instID, bid, ask, at)),
		},
	}
}

func exampleOrderBook(instID model.InstrumentID, bid string, ask string, ts time.Time) model.OrderBook {
	return model.OrderBook{
		InstrumentID: instID,
		Bids: []model.OrderBookLevel{{
			Price: decimal.RequireFromString(bid),
			Size:  decimal.RequireFromString("2"),
		}},
		Asks: []model.OrderBookLevel{{
			Price: decimal.RequireFromString(ask),
			Size:  decimal.RequireFromString("2"),
		}},
		Timestamp: ts,
	}
}

func ptrOrderBook(book model.OrderBook) *model.OrderBook {
	return &book
}

type exampleDataClient struct {
	mu       sync.Mutex
	provider *exampleProvider
	events   chan model.MarketEvent
	subs     []model.SubscribeMarketData
}

func newExampleDataClient(instID model.InstrumentID) *exampleDataClient {
	return &exampleDataClient{provider: newExampleProvider(instID), events: make(chan model.MarketEvent, 8)}
}

func (c *exampleDataClient) Venue() model.Venue                    { return c.provider.inst.ID.Venue }
func (c *exampleDataClient) ClientID() string                      { return "example-data" }
func (c *exampleDataClient) Instruments() venue.InstrumentProvider { return c.provider }
func (c *exampleDataClient) Connect(context.Context) error         { return nil }
func (c *exampleDataClient) Disconnect(context.Context) error      { return nil }
func (c *exampleDataClient) Health() venue.DataHealth {
	return venue.DataHealth{Connected: true, InstrumentReady: true}
}
func (c *exampleDataClient) FetchTicker(context.Context, model.InstrumentID) (model.Ticker, error) {
	return model.Ticker{}, model.ErrNotSupported
}
func (c *exampleDataClient) FetchOrderBook(context.Context, model.InstrumentID, int) (model.OrderBook, error) {
	return model.OrderBook{}, model.ErrNotSupported
}
func (c *exampleDataClient) SubscribeMarketData(_ context.Context, sub model.SubscribeMarketData) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.subs = append(c.subs, sub)
	return nil
}
func (c *exampleDataClient) UnsubscribeMarketData(context.Context, model.SubscribeMarketData) error {
	return nil
}
func (c *exampleDataClient) Events() <-chan model.MarketEvent { return c.events }
func (c *exampleDataClient) EmitOrderBook(book model.OrderBook) {
	c.events <- model.MarketEvent{OrderBook: &book}
}

type exampleProvider struct {
	inst model.Instrument
}

func newExampleProvider(instID model.InstrumentID) *exampleProvider {
	return &exampleProvider{inst: model.Instrument{
		ID:        instID,
		RawSymbol: "BTCUSDT",
		Type:      model.InstrumentTypeSpot,
		Base:      "BTC",
		Quote:     "USDT",
		PriceTick: decimal.RequireFromString("0.01"),
		SizeTick:  decimal.RequireFromString("0.0001"),
		Status:    model.InstrumentStatusTrading,
	}}
}

func (p *exampleProvider) LoadAll(context.Context) error { return nil }
func (p *exampleProvider) Get(id model.InstrumentID) (model.Instrument, bool) {
	return p.inst, p.inst.ID == id
}
func (p *exampleProvider) List() []model.Instrument { return []model.Instrument{p.inst} }

type exampleExecutionClient struct {
	mu           sync.Mutex
	accountID    model.AccountID
	instrumentID model.InstrumentID
	events       chan model.ExecutionEvent
	lastSubmit   model.SubmitOrder
	submitted    bool
	nextID       int
}

func newExampleExecutionClient(accountID model.AccountID, instrumentID model.InstrumentID) *exampleExecutionClient {
	return &exampleExecutionClient{accountID: accountID, instrumentID: instrumentID, events: make(chan model.ExecutionEvent, 4)}
}

func (c *exampleExecutionClient) Venue() model.Venue               { return c.instrumentID.Venue }
func (c *exampleExecutionClient) AccountID() model.AccountID       { return c.accountID }
func (c *exampleExecutionClient) Connect(context.Context) error    { return nil }
func (c *exampleExecutionClient) Disconnect(context.Context) error { return nil }
func (c *exampleExecutionClient) Health() venue.ExecutionHealth {
	return venue.ExecutionHealth{Connected: true, AccountReady: true}
}
func (c *exampleExecutionClient) QueryAccount(context.Context) (model.AccountSnapshot, error) {
	return model.AccountSnapshot{AccountID: c.accountID, Venue: c.instrumentID.Venue}, nil
}
func (c *exampleExecutionClient) SubmitOrder(_ context.Context, order model.SubmitOrder) (model.OrderStatusReport, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.nextID++
	c.lastSubmit = order
	c.submitted = true
	return model.OrderStatusReport{
		AccountID:      order.AccountID,
		InstrumentID:   order.InstrumentID,
		OrderID:        model.OrderID(fmt.Sprintf("example-live-order-%d", c.nextID)),
		ClientOrderID:  order.ClientOrderID,
		OrderListID:    order.OrderListID,
		Side:           order.Side,
		Type:           order.Type,
		Status:         model.OrderStatusAccepted,
		Quantity:       order.Quantity,
		FilledQuantity: decimal.Zero,
		LeavesQuantity: order.Quantity,
		Price:          order.Price,
	}, nil
}
func (c *exampleExecutionClient) CancelOrder(_ context.Context, cancel model.CancelOrder) (model.OrderStatusReport, error) {
	return model.OrderStatusReport{
		AccountID:     cancel.AccountID,
		InstrumentID:  cancel.InstrumentID,
		OrderID:       cancel.OrderID,
		ClientOrderID: cancel.ClientOrderID,
		Status:        model.OrderStatusCanceled,
	}, nil
}
func (c *exampleExecutionClient) GenerateOrderStatusReports(context.Context, model.InstrumentID) ([]model.OrderStatusReport, error) {
	return nil, nil
}
func (c *exampleExecutionClient) Events() <-chan model.ExecutionEvent { return c.events }
func (c *exampleExecutionClient) LastSubmit() (model.SubmitOrder, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastSubmit, c.submitted
}
