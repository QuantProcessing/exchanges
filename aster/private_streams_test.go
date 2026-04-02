package aster

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"unsafe"

	exchanges "github.com/QuantProcessing/exchanges"
	perpsdk "github.com/QuantProcessing/exchanges/aster/sdk/perp"
	spotsdk "github.com/QuantProcessing/exchanges/aster/sdk/spot"
	"github.com/gorilla/websocket"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestPerpWatchOrdersAndFillsShareSingleNativeHandler(t *testing.T) {
	wsAccount := &perpsdk.WsAccountClient{
		WsClient: &perpsdk.WsClient{Conn: &websocket.Conn{}},
	}
	adp := &Adapter{
		quoteCurrency: "USDT",
		apiKey:        "key",
		secretKey:     "secret",
		wsAccount:     wsAccount,
	}

	var orders []*exchanges.Order
	var fills []*exchanges.Fill

	require.NoError(t, adp.WatchOrders(context.Background(), func(order *exchanges.Order) {
		orders = append(orders, order)
	}))
	require.NoError(t, adp.WatchFills(context.Background(), func(fill *exchanges.Fill) {
		fills = append(fills, fill)
	}))

	require.Equal(t, 1, callbackCount(wsAccount, "orderUpdateCallbacks"))

	event := perpsdk.OrderUpdateEvent{EventTime: 1700000003, TransactionTime: 1700000002}
	event.Order.Symbol = "BTCUSDT"
	event.Order.ClientOrderID = "client-1"
	event.Order.Side = "BUY"
	event.Order.OrderType = "LIMIT"
	event.Order.ExecutionType = "TRADE"
	event.Order.OrderStatus = "PARTIALLY_FILLED"
	event.Order.OrderID = 66
	event.Order.OriginalQty = "2"
	event.Order.OriginalPrice = "101.25"
	event.Order.AveragePrice = "100.75"
	event.Order.LastFilledPrice = "101.50"
	event.Order.AccumulatedFilledQty = "0.5"
	event.Order.LastFilledQty = "0.25"
	event.Order.Commission = "0.01"
	event.Order.CommissionAsset = "USDT"
	event.Order.TradeTime = 1700000001
	event.Order.TradeID = 55
	event.Order.IsMaker = false

	invokeCallback(t, wsAccount, "orderUpdateCallbacks", 0, &event)

	require.Len(t, orders, 1)
	require.Len(t, fills, 1)

	order := orders[0]
	require.Equal(t, "66", order.OrderID)
	require.Equal(t, "BTC", order.Symbol)
	require.Equal(t, exchanges.OrderSideBuy, order.Side)
	require.Equal(t, exchanges.OrderTypeLimit, order.Type)
	require.True(t, order.Quantity.Equal(decimal.RequireFromString("2")))
	require.True(t, order.Price.Equal(decimal.RequireFromString("101.25")))
	require.True(t, order.OrderPrice.Equal(decimal.RequireFromString("101.25")))
	require.True(t, order.FilledQuantity.Equal(decimal.RequireFromString("0.5")))
	require.Equal(t, exchanges.OrderStatusPartiallyFilled, order.Status)
	require.EqualValues(t, 1700000001, order.Timestamp)
	require.Equal(t, "client-1", order.ClientOrderID)
	require.True(t, order.AverageFillPrice.IsZero())
	require.True(t, order.LastFillPrice.IsZero())
	require.True(t, order.LastFillQuantity.IsZero())

	fill := fills[0]
	require.Equal(t, "55", fill.TradeID)
	require.Equal(t, "66", fill.OrderID)
	require.Equal(t, "BTC", fill.Symbol)
	require.Equal(t, exchanges.OrderSideBuy, fill.Side)
	require.True(t, fill.Price.Equal(decimal.RequireFromString("101.50")))
	require.True(t, fill.Quantity.Equal(decimal.RequireFromString("0.25")))
	require.True(t, fill.Fee.Equal(decimal.RequireFromString("0.01")))
	require.Equal(t, "USDT", fill.FeeAsset)
	require.False(t, fill.IsMaker)
	require.EqualValues(t, 1700000001, fill.Timestamp)
}

func TestSpotWatchOrdersAndFillsFanOutFromSingleNativeHandler(t *testing.T) {
	wsClient := spotsdk.NewWsClient(context.Background(), "")
	wsClient.Conn = &websocket.Conn{}
	wsAccount := &spotsdk.WsAccountClient{
		WsClient: wsClient,
	}
	adp := &SpotAdapter{
		quoteCurrency: "USDT",
		apiKey:        "key",
		secretKey:     "secret",
		wsAccount:     wsAccount,
	}

	var orders []*exchanges.Order
	var fills []*exchanges.Fill

	require.NoError(t, adp.WatchOrders(context.Background(), func(order *exchanges.Order) {
		orders = append(orders, order)
	}))
	require.NoError(t, adp.WatchFills(context.Background(), func(fill *exchanges.Fill) {
		fills = append(fills, fill)
	}))

	report := &spotsdk.ExecutionReportEvent{
		EventTime:                1700000002,
		Symbol:                   "ETHUSDT",
		ClientOrderID:            "client-2",
		Side:                     "SELL",
		OrderType:                "LIMIT",
		ExecutionType:            "TRADE",
		OrderStatus:              "PARTIALLY_FILLED",
		OrderID:                  88,
		Quantity:                 "3",
		Price:                    "202.5",
		LastExecutedQuantity:     "1.25",
		CumulativeFilledQuantity: "1.75",
		LastExecutedPrice:        "202.75",
		CommissionAmount:         "0.02",
		CommissionAsset:          "USDT",
		TransactionTime:          1700000001,
		TradeID:                  77,
		IsMaker:                  true,
	}

	invokeLocalHandler(t, wsAccount, report)

	require.Len(t, orders, 1)
	require.Len(t, fills, 1)

	order := orders[0]
	require.Equal(t, "88", order.OrderID)
	require.Equal(t, "ETH", order.Symbol)
	require.Equal(t, exchanges.OrderSideSell, order.Side)
	require.Equal(t, exchanges.OrderTypeLimit, order.Type)
	require.True(t, order.Quantity.Equal(decimal.RequireFromString("3")))
	require.True(t, order.Price.Equal(decimal.RequireFromString("202.5")))
	require.True(t, order.OrderPrice.Equal(decimal.RequireFromString("202.5")))
	require.True(t, order.FilledQuantity.Equal(decimal.RequireFromString("1.75")))
	require.Equal(t, exchanges.OrderStatusPartiallyFilled, order.Status)
	require.EqualValues(t, 1700000001, order.Timestamp)
	require.Equal(t, "client-2", order.ClientOrderID)
	require.True(t, order.AverageFillPrice.IsZero())
	require.True(t, order.LastFillPrice.IsZero())
	require.True(t, order.LastFillQuantity.IsZero())

	fill := fills[0]
	require.Equal(t, "77", fill.TradeID)
	require.Equal(t, "88", fill.OrderID)
	require.Equal(t, "ETH", fill.Symbol)
	require.Equal(t, exchanges.OrderSideSell, fill.Side)
	require.True(t, fill.Price.Equal(decimal.RequireFromString("202.75")))
	require.True(t, fill.Quantity.Equal(decimal.RequireFromString("1.25")))
	require.True(t, fill.Fee.Equal(decimal.RequireFromString("0.02")))
	require.Equal(t, "USDT", fill.FeeAsset)
	require.True(t, fill.IsMaker)
	require.EqualValues(t, 1700000001, fill.Timestamp)
}

func TestPrivateOrderStreamsStopClearsCallbacks(t *testing.T) {
	streams := newPrivateOrderStreams(
		func(func(*int)) {},
		func(event *int) *exchanges.Order {
			return &exchanges.Order{OrderID: "1"}
		},
		func(event *int) *exchanges.Fill {
			return &exchanges.Fill{TradeID: "2"}
		},
	)

	require.NoError(t, streams.watchOrders(func() error { return nil }, func(*exchanges.Order) {}))
	require.NoError(t, streams.watchFills(func() error { return nil }, func(*exchanges.Fill) {}))

	streams.stopOrders()
	streams.stopFills()

	streams.mu.Lock()
	defer streams.mu.Unlock()
	require.Nil(t, streams.orderCallback)
	require.Nil(t, streams.fillCallback)
}

func callbackCount(target any, field string) int {
	value := reflect.ValueOf(target).Elem().FieldByName(field)
	return value.Len()
}

func invokeCallback(t *testing.T, target any, field string, index int, event any) {
	t.Helper()

	callbacks := reflect.ValueOf(target).Elem().FieldByName(field)
	callbacks = reflect.NewAt(callbacks.Type(), unsafe.Pointer(callbacks.UnsafeAddr())).Elem()
	require.Greater(t, callbacks.Len(), index)

	callbacks.Index(index).Call([]reflect.Value{reflect.ValueOf(event)})
}

func invokeLocalHandler(t *testing.T, wsAccount *spotsdk.WsAccountClient, event *spotsdk.ExecutionReportEvent) {
	t.Helper()

	payload, err := json.Marshal(event)
	require.NoError(t, err)
	wsAccount.CallSubscription("executionReport", payload)
}
