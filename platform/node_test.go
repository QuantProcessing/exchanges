package platform

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestNodeStartsDataOnly(t *testing.T) {
	data := newFakeDataClient()
	node := NewNode(Config{})
	require.NoError(t, node.AddDataClient("binance-spot-data", data))

	require.NoError(t, node.Start(context.Background()))
	require.True(t, data.connected)
	require.Equal(t, 1, data.provider.loadAllCalls)
	require.True(t, node.Health().Ready)

	require.NoError(t, node.Stop(context.Background()))
	require.False(t, data.connected)
}

func TestNodeStartsMixedClientsAndPublishesStartupEvents(t *testing.T) {
	data := newFakeDataClient()
	exec := newFakePlatformExecution()
	node := NewNode(Config{})
	require.NoError(t, node.AddDataClient("binance-spot-data", data))
	require.NoError(t, node.AddExecutionClient("binance-perp-exec", exec))

	_, events := node.Bus().Subscribe("events.execution", 8)

	require.NoError(t, node.Start(context.Background()))
	t.Cleanup(func() { require.NoError(t, node.Stop(context.Background())) })

	require.Equal(t, []string{"query_account", "order_reports:BTC-USDT-PERP.BINANCE", "fill_reports:BTC-USDT-PERP.BINANCE", "position_reports:BTC-USDT-PERP.BINANCE", "connect"}, exec.callsSnapshot())

	var sawAccount bool
	var sawOrder bool
	require.Eventually(t, func() bool {
		for {
			select {
			case ev := <-events:
				msg, ok := ev.Message.(model.ExecutionEvent)
				if !ok {
					continue
				}
				if msg.AccountState != nil {
					sawAccount = true
				}
				if msg.Order != nil {
					sawOrder = true
				}
			default:
				return sawAccount && sawOrder
			}
		}
	}, time.Second, 10*time.Millisecond)
}

type fakeDataClient struct {
	provider  *fakeInstrumentProvider
	connected bool
}

var _ venue.DataClient = (*fakeDataClient)(nil)

func newFakeDataClient() *fakeDataClient {
	return &fakeDataClient{provider: newFakeInstrumentProvider()}
}

func (f *fakeDataClient) Venue() model.Venue { return model.VenueBinance }

func (f *fakeDataClient) ClientID() string { return "fake-data" }

func (f *fakeDataClient) Instruments() venue.InstrumentProvider { return f.provider }

func (f *fakeDataClient) Connect(context.Context) error {
	f.connected = true
	return f.provider.LoadAll(context.Background())
}

func (f *fakeDataClient) Disconnect(context.Context) error {
	f.connected = false
	return nil
}

func (f *fakeDataClient) Health() venue.DataHealth { return venue.DataHealth{Connected: f.connected} }

func (f *fakeDataClient) FetchTicker(context.Context, model.InstrumentID) (model.Ticker, error) {
	return model.Ticker{}, nil
}

func (f *fakeDataClient) FetchOrderBook(context.Context, model.InstrumentID, int) (model.OrderBook, error) {
	return model.OrderBook{}, nil
}

func (f *fakeDataClient) FetchTrades(context.Context, model.InstrumentID, venue.TradeQuery) ([]model.Trade, error) {
	return nil, nil
}

func (f *fakeDataClient) FetchBars(context.Context, model.InstrumentID, model.BarSpec, venue.BarQuery) ([]model.Bar, error) {
	return nil, nil
}

func (f *fakeDataClient) SubscribeTicker(context.Context, model.InstrumentID, venue.TickerHandler) (venue.Subscription, error) {
	return nil, nil
}

func (f *fakeDataClient) SubscribeOrderBook(context.Context, model.InstrumentID, int, venue.OrderBookHandler) (venue.Subscription, error) {
	return nil, nil
}

func (f *fakeDataClient) SubscribeTrades(context.Context, model.InstrumentID, venue.TradeHandler) (venue.Subscription, error) {
	return nil, nil
}

func (f *fakeDataClient) SubscribeBars(context.Context, model.InstrumentID, model.BarSpec, venue.BarHandler) (venue.Subscription, error) {
	return nil, nil
}

type fakeInstrumentProvider struct {
	loadAllCalls int
	instruments  []model.Instrument
}

func newFakeInstrumentProvider() *fakeInstrumentProvider {
	return &fakeInstrumentProvider{
		instruments: []model.Instrument{{
			ID:        model.MustInstrumentID("BTC-USDT-PERP.BINANCE"),
			RawSymbol: "BTCUSDT",
			Type:      model.InstrumentTypeCryptoPerp,
			Base:      model.BTC,
			Quote:     model.USDT,
			Settle:    model.USDT,
			PriceStep: decimal.RequireFromString("0.1"),
			SizeStep:  decimal.RequireFromString("0.001"),
		}},
	}
}

func (f *fakeInstrumentProvider) LoadAll(context.Context) error {
	f.loadAllCalls++
	return nil
}

func (f *fakeInstrumentProvider) Load(_ context.Context, id model.InstrumentID) (model.Instrument, error) {
	for _, inst := range f.instruments {
		if inst.ID == id {
			return inst, nil
		}
	}
	return model.Instrument{}, model.ErrInstrumentNotLoaded
}

func (f *fakeInstrumentProvider) Find(context.Context, venue.InstrumentQuery) ([]model.Instrument, error) {
	return append([]model.Instrument(nil), f.instruments...), nil
}

func (f *fakeInstrumentProvider) Get(id model.InstrumentID) (model.Instrument, bool) {
	for _, inst := range f.instruments {
		if inst.ID == id {
			return inst, true
		}
	}
	return model.Instrument{}, false
}

func (f *fakeInstrumentProvider) List() []model.Instrument {
	return append([]model.Instrument(nil), f.instruments...)
}

type fakePlatformExecution struct {
	mu        sync.Mutex
	events    chan model.ExecutionEvent
	connected bool
	calls     []string
}

var _ venue.ExecutionClient = (*fakePlatformExecution)(nil)

func newFakePlatformExecution() *fakePlatformExecution {
	return &fakePlatformExecution{events: make(chan model.ExecutionEvent, 8)}
}

func (f *fakePlatformExecution) AccountID() model.AccountID { return "acct-1" }

func (f *fakePlatformExecution) Venue() model.Venue { return model.VenueBinance }

func (f *fakePlatformExecution) Connect(context.Context) error {
	f.record("connect")
	f.connected = true
	return nil
}

func (f *fakePlatformExecution) Disconnect(context.Context) error {
	f.connected = false
	return nil
}

func (f *fakePlatformExecution) Health() venue.ExecutionHealth {
	return venue.ExecutionHealth{Connected: f.connected}
}

func (f *fakePlatformExecution) SubmitOrder(context.Context, model.SubmitOrder) error { return nil }

func (f *fakePlatformExecution) ModifyOrder(context.Context, model.ModifyOrder) error {
	return model.ErrNotSupported
}

func (f *fakePlatformExecution) CancelOrder(context.Context, model.CancelOrder) error { return nil }

func (f *fakePlatformExecution) CancelAllOrders(context.Context, model.CancelAllOrders) error {
	return nil
}

func (f *fakePlatformExecution) QueryAccount(context.Context) error {
	f.record("query_account")
	f.events <- model.ExecutionEvent{AccountState: &model.AccountState{
		AccountID: "acct-1",
		Venue:     model.VenueBinance,
		Type:      model.AccountTypeMargin,
		EventTime: time.Now(),
	}}
	return nil
}

func (f *fakePlatformExecution) GenerateOrderStatusReports(_ context.Context, q venue.OrderStatusQuery) ([]model.OrderStatusReport, error) {
	f.record("order_reports:" + q.InstrumentID.String())
	return []model.OrderStatusReport{{
		AccountID:    "acct-1",
		InstrumentID: q.InstrumentID,
		OrderID:      "venue-1",
		ClientID:     "client-1",
		Status:       model.OrderStatusAccepted,
		Side:         model.OrderSideBuy,
		Type:         model.OrderTypeLimit,
		Quantity:     decimal.NewFromInt(1),
		EventTime:    time.Now(),
	}}, nil
}

func (f *fakePlatformExecution) GenerateFillReports(_ context.Context, q venue.FillQuery) ([]model.FillReport, error) {
	f.record("fill_reports:" + q.InstrumentID.String())
	return nil, nil
}

func (f *fakePlatformExecution) GeneratePositionStatusReports(_ context.Context, q venue.PositionQuery) ([]model.PositionStatusReport, error) {
	f.record("position_reports:" + q.InstrumentID.String())
	return nil, nil
}

func (f *fakePlatformExecution) Events() <-chan model.ExecutionEvent { return f.events }

func (f *fakePlatformExecution) record(call string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, call)
}

func (f *fakePlatformExecution) callsSnapshot() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}
