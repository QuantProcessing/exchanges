package aster

import (
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	perpsdk "github.com/QuantProcessing/exchanges/aster/sdk/perp"
	spotsdk "github.com/QuantProcessing/exchanges/aster/sdk/spot"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestPerpMapOrderFillReturnsExecutionDetails(t *testing.T) {
	adp := &Adapter{quoteCurrency: "USDT"}

	event := perpsdk.OrderUpdateEvent{EventTime: 1700000003, TransactionTime: 1700000002}
	event.Order.Symbol = "BTCUSDT"
	event.Order.ClientOrderID = "client-1"
	event.Order.Side = "BUY"
	event.Order.ExecutionType = "TRADE"
	event.Order.OrderID = 66
	event.Order.LastFilledQty = "0.5"
	event.Order.LastFilledPrice = "101.25"
	event.Order.Commission = "0.01"
	event.Order.CommissionAsset = "USDT"
	event.Order.TradeTime = 1700000001
	event.Order.TradeID = 55
	event.Order.IsMaker = false

	fill := adp.mapOrderFill(&event)

	require.NotNil(t, fill)
	require.Equal(t, "55", fill.TradeID)
	require.Equal(t, "66", fill.OrderID)
	require.Equal(t, "client-1", fill.ClientOrderID)
	require.Equal(t, "BTC", fill.Symbol)
	require.Equal(t, exchanges.OrderSideBuy, fill.Side)
	require.True(t, fill.Price.Equal(decimal.RequireFromString("101.25")))
	require.True(t, fill.Quantity.Equal(decimal.RequireFromString("0.5")))
	require.True(t, fill.Fee.Equal(decimal.RequireFromString("0.01")))
	require.Equal(t, "USDT", fill.FeeAsset)
	require.False(t, fill.IsMaker)
	require.EqualValues(t, 1700000001, fill.Timestamp)
}

func TestSpotMapExecutionFillReturnsExecutionDetails(t *testing.T) {
	adp := &SpotAdapter{quoteCurrency: "USDT"}

	fill := adp.mapExecutionFill(&spotsdk.ExecutionReportEvent{
		EventTime:            1700000002,
		Symbol:               "ETHUSDT",
		ClientOrderID:        "client-2",
		Side:                 "SELL",
		ExecutionType:        "TRADE",
		OrderID:              88,
		LastExecutedQuantity: "1.25",
		LastExecutedPrice:    "202.5",
		CommissionAmount:     "0.02",
		CommissionAsset:      "USDT",
		TransactionTime:      1700000001,
		TradeID:              77,
		IsMaker:              true,
	})

	require.NotNil(t, fill)
	require.Equal(t, "77", fill.TradeID)
	require.Equal(t, "88", fill.OrderID)
	require.Equal(t, "client-2", fill.ClientOrderID)
	require.Equal(t, "ETH", fill.Symbol)
	require.Equal(t, exchanges.OrderSideSell, fill.Side)
	require.True(t, fill.Price.Equal(decimal.RequireFromString("202.5")))
	require.True(t, fill.Quantity.Equal(decimal.RequireFromString("1.25")))
	require.True(t, fill.Fee.Equal(decimal.RequireFromString("0.02")))
	require.Equal(t, "USDT", fill.FeeAsset)
	require.True(t, fill.IsMaker)
	require.EqualValues(t, 1700000001, fill.Timestamp)
}
