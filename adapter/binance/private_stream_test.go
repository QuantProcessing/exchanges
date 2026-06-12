package binance

import (
	"context"
	"testing"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/sdk/binance/perp"
	"github.com/QuantProcessing/exchanges/sdk/binance/spot"
	"github.com/stretchr/testify/require"
)

func TestSpotPrivateStreamEmitsOrderAndFillReports(t *testing.T) {
	user := &fakeSpotUserStream{}
	api := &fakeSpotAPIStream{}
	var orders []model.OrderStatusReport
	var events []model.OrderEvent
	var fills []model.FillReport
	stream := newSpotPrivateStream(
		"binance-spot-master",
		user,
		api,
		nil,
		nil,
		func(report model.OrderStatusReport) { orders = append(orders, report) },
		func(report model.FillReport) { fills = append(fills, report) },
	)
	stream.emitOrderEvent = func(event model.OrderEvent) { events = append(events, event) }

	require.NoError(t, stream.Connect(context.Background()))
	require.True(t, user.connected)

	user.executionReport(&spot.ExecutionReportEvent{
		EventType:                "executionReport",
		EventTime:                1000,
		Symbol:                   "BTCUSDT",
		ClientOrderID:            "client-1",
		Side:                     "BUY",
		OrderType:                "LIMIT",
		OrderStatus:              "FILLED",
		OrderID:                  123,
		Quantity:                 "0.2",
		CumulativeFilledQuantity: "0.2",
		LastExecutedQuantity:     "0.2",
		LastExecutedPrice:        "100",
		CommissionAmount:         "0.01",
		CommissionAsset:          "USDT",
		TransactionTime:          1001,
		TradeID:                  999,
	})

	require.Len(t, orders, 1)
	require.Equal(t, model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"), orders[0].InstrumentID)
	require.Equal(t, model.OrderStatusFilled, orders[0].Status)
	require.Len(t, events, 1)
	require.Equal(t, model.OrderEventFilled, events[0].Type)
	require.Equal(t, model.OrderStatusFilled, events[0].Status)
	require.Len(t, fills, 1)
	require.Equal(t, model.TradeID("999"), fills[0].TradeID)
}

func TestSpotPrivateStreamRunsResubscribeHookAfterAPIReconnect(t *testing.T) {
	user := &fakeSpotUserStream{}
	api := &fakeSpotAPIStream{}
	hookCalls := 0
	stream := newSpotPrivateStream("acct", user, api, func(context.Context) error {
		hookCalls++
		return nil
	}, nil, nil, nil)

	require.NoError(t, stream.Connect(context.Background()))
	api.reconnect()
	require.Equal(t, 1, hookCalls)
	require.Equal(t, 2, user.connectCalls)
}

func TestPerpPrivateStreamEmitsOrderFillAndPositions(t *testing.T) {
	user := &fakePerpUserStream{}
	var orders []model.OrderStatusReport
	var events []model.OrderEvent
	var fills []model.FillReport
	var positions []model.PositionStatusReport
	stream := newPerpPrivateStream(
		"binance-usdt-futures-master",
		user,
		nil,
		func(report model.OrderStatusReport) { orders = append(orders, report) },
		func(report model.FillReport) { fills = append(fills, report) },
		func(report model.PositionStatusReport) { positions = append(positions, report) },
	)
	stream.emitOrderEvent = func(event model.OrderEvent) { events = append(events, event) }

	require.NoError(t, stream.Connect(context.Background()))
	require.True(t, user.connected)

	order := &perp.OrderUpdateEvent{EventTime: 2000}
	order.Order.Symbol = "BTCUSDT"
	order.Order.ClientOrderID = "client-2"
	order.Order.Side = "SELL"
	order.Order.OrderType = "MARKET"
	order.Order.OrderStatus = "PARTIALLY_FILLED"
	order.Order.OrderID = 456
	order.Order.OriginalQty = "1"
	order.Order.AccumulatedFilledQty = "0.4"
	order.Order.LastFilledQty = "0.4"
	order.Order.LastFilledPrice = "101"
	order.Order.Commission = "0.02"
	order.Order.CommissionAsset = "USDT"
	order.Order.TradeTime = 2001
	order.Order.TradeID = 777
	user.orderUpdate(order)

	account := &perp.AccountUpdateEvent{EventTime: 3000}
	account.UpdateData.Positions = append(account.UpdateData.Positions, struct {
		Symbol              string `json:"s"`
		PositionAmount      string `json:"pa"`
		EntryPrice          string `json:"ep"`
		AccumulatedRealized string `json:"cr"`
		UnrealizedPnL       string `json:"up"`
		MarginType          string `json:"mt"`
		IsolatedWallet      string `json:"iw"`
		PositionSide        string `json:"ps"`
	}{
		Symbol:         "BTCUSDT",
		PositionAmount: "-0.5",
		EntryPrice:     "100",
		UnrealizedPnL:  "2",
	})
	user.accountUpdate(account)

	require.Len(t, orders, 1)
	require.Equal(t, model.OrderStatusPartiallyFilled, orders[0].Status)
	require.Len(t, events, 1)
	require.Equal(t, model.OrderEventPartiallyFilled, events[0].Type)
	require.Equal(t, model.OrderStatusPartiallyFilled, events[0].Status)
	require.Len(t, fills, 1)
	require.Equal(t, model.TradeID("777"), fills[0].TradeID)
	require.Len(t, positions, 1)
	require.Equal(t, model.PositionSideShort, positions[0].Side)
	require.Equal(t, "0.5", positions[0].Quantity.String())
}

func TestPerpPrivateStreamRunsResubscribeHook(t *testing.T) {
	user := &fakePerpUserStream{}
	hookCalls := 0
	stream := newPerpPrivateStream("acct", user, func(context.Context) error {
		hookCalls++
		return nil
	}, nil, nil, nil)

	require.NoError(t, stream.Connect(context.Background()))
	user.resubscribe()
	require.Equal(t, 1, hookCalls)
}

type fakeSpotUserStream struct {
	connected       bool
	closed          bool
	connectCalls    int
	executionReport func(*spot.ExecutionReportEvent)
	accountPosition func(*spot.AccountPositionEvent)
}

func (f *fakeSpotUserStream) Connect() error {
	f.connected = true
	f.connectCalls++
	return nil
}

func (f *fakeSpotUserStream) Close() { f.closed = true }

func (f *fakeSpotUserStream) SubscribeExecutionReport(h func(*spot.ExecutionReportEvent)) {
	f.executionReport = h
}

func (f *fakeSpotUserStream) SubscribeAccountPosition(h func(*spot.AccountPositionEvent)) {
	f.accountPosition = h
}

type fakeSpotAPIStream struct {
	closed        bool
	postReconnect func()
}

func (f *fakeSpotAPIStream) Close() { f.closed = true }

func (f *fakeSpotAPIStream) SetPostReconnect(h func()) {
	f.postReconnect = h
}

func (f *fakeSpotAPIStream) reconnect() {
	if f.postReconnect != nil {
		f.postReconnect()
	}
}

type fakePerpUserStream struct {
	connected     bool
	closed        bool
	accountUpdate func(*perp.AccountUpdateEvent)
	orderUpdate   func(*perp.OrderUpdateEvent)
	onResubscribe func()
}

func (f *fakePerpUserStream) Connect() error {
	f.connected = true
	return nil
}

func (f *fakePerpUserStream) Close() { f.closed = true }

func (f *fakePerpUserStream) SubscribeAccountUpdate(h func(*perp.AccountUpdateEvent)) {
	f.accountUpdate = h
}

func (f *fakePerpUserStream) SubscribeOrderUpdate(h func(*perp.OrderUpdateEvent)) {
	f.orderUpdate = h
}

func (f *fakePerpUserStream) SetOnResubscribe(h func()) {
	f.onResubscribe = h
}

func (f *fakePerpUserStream) resubscribe() {
	if f.onResubscribe != nil {
		f.onResubscribe()
	}
}
