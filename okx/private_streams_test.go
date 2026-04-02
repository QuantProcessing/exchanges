package okx

import (
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	sdkokx "github.com/QuantProcessing/exchanges/okx/sdk"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestDispatchPrivateOrder_PerpUsesOverviewOrderAndDetailedFill(t *testing.T) {
	t.Parallel()

	adp := &Adapter{
		symbolMap: map[string]string{"BTC": "BTC-USDT-SWAP"},
		idMap:     map[string]string{"BTC-USDT-SWAP": "BTC"},
		instruments: map[string]sdkokx.Instrument{
			"BTC-USDT-SWAP": {CtVal: "0.01"},
		},
	}

	var orderUpdate *exchanges.Order
	var fillUpdate *exchanges.Fill
	adp.privateOrderStream.orderCB = func(order *exchanges.Order) {
		orderUpdate = order
	}
	adp.privateOrderStream.fillCB = func(fill *exchanges.Fill) {
		fillUpdate = fill
	}

	adp.dispatchPrivateOrder(&sdkokx.Order{
		OrdId:     "order-1",
		ClOrdId:   "client-1",
		InstId:    "BTC-USDT-SWAP",
		Side:      sdkokx.SideBuy,
		OrdType:   sdkokx.OrderTypeLimit,
		State:     sdkokx.OrderStatusPartiallyFilled,
		Sz:        "300",
		AccFillSz: "200",
		Px:        "50000",
		AvgPx:     "49950",
		FillPx:    "49900",
		FillSz:    "100",
		Fee:       "1.5",
		FeeCcy:    "USDT",
		TradeId:   "trade-1",
		FillTime:  "1710000000456",
		UTime:     "1710000000123",
		CTime:     "1710000000000",
	})

	require.NotNil(t, orderUpdate)
	require.Equal(t, "order-1", orderUpdate.OrderID)
	require.Equal(t, "client-1", orderUpdate.ClientOrderID)
	require.Equal(t, "BTC", orderUpdate.Symbol)
	require.True(t, orderUpdate.Quantity.Equal(decimal.RequireFromString("3")))
	require.True(t, orderUpdate.FilledQuantity.Equal(decimal.RequireFromString("2")))
	require.Equal(t, decimal.RequireFromString("50000"), orderUpdate.OrderPrice)
	require.Equal(t, decimal.RequireFromString("50000"), orderUpdate.Price)
	require.True(t, orderUpdate.AverageFillPrice.IsZero())
	require.True(t, orderUpdate.LastFillPrice.IsZero())
	require.True(t, orderUpdate.LastFillQuantity.IsZero())
	require.True(t, orderUpdate.Fee.IsZero())

	require.NotNil(t, fillUpdate)
	require.Equal(t, "trade-1", fillUpdate.TradeID)
	require.Equal(t, "order-1", fillUpdate.OrderID)
	require.Equal(t, "client-1", fillUpdate.ClientOrderID)
	require.Equal(t, decimal.RequireFromString("49900"), fillUpdate.Price)
	require.True(t, fillUpdate.Quantity.Equal(decimal.RequireFromString("1")))
	require.Equal(t, decimal.RequireFromString("1.5"), fillUpdate.Fee)
	require.Equal(t, "USDT", fillUpdate.FeeAsset)
	require.Equal(t, int64(1710000000456), fillUpdate.Timestamp)
}

func TestDispatchPrivateOrder_SpotUsesOverviewOrderAndDetailedFill(t *testing.T) {
	t.Parallel()

	adp := &SpotAdapter{
		idMap: map[string]string{"BTC-USDT": "BTC"},
	}

	var orderUpdate *exchanges.Order
	var fillUpdate *exchanges.Fill
	adp.privateOrderStream.orderCB = func(order *exchanges.Order) {
		orderUpdate = order
	}
	adp.privateOrderStream.fillCB = func(fill *exchanges.Fill) {
		fillUpdate = fill
	}

	adp.dispatchPrivateOrder(&sdkokx.Order{
		OrdId:     "order-2",
		ClOrdId:   "client-2",
		InstId:    "BTC-USDT",
		Side:      sdkokx.SideSell,
		OrdType:   sdkokx.OrderTypeLimit,
		State:     sdkokx.OrderStatusPartiallyFilled,
		Sz:        "2",
		AccFillSz: "1",
		Px:        "50000",
		AvgPx:     "49950",
		FillPx:    "49900",
		FillSz:    "0.25",
		Fee:       "0.5",
		FeeCcy:    "USDT",
		TradeId:   "trade-2",
		FillTime:  "1710000001456",
		CTime:     "1710000001000",
	})

	require.NotNil(t, orderUpdate)
	require.Equal(t, "order-2", orderUpdate.OrderID)
	require.Equal(t, "client-2", orderUpdate.ClientOrderID)
	require.Equal(t, "BTC", orderUpdate.Symbol)
	require.True(t, orderUpdate.Quantity.Equal(decimal.RequireFromString("2")))
	require.True(t, orderUpdate.FilledQuantity.Equal(decimal.RequireFromString("1")))
	require.Equal(t, decimal.RequireFromString("50000"), orderUpdate.OrderPrice)
	require.Equal(t, decimal.RequireFromString("50000"), orderUpdate.Price)
	require.True(t, orderUpdate.AverageFillPrice.IsZero())
	require.True(t, orderUpdate.LastFillPrice.IsZero())
	require.True(t, orderUpdate.LastFillQuantity.IsZero())
	require.True(t, orderUpdate.Fee.IsZero())

	require.NotNil(t, fillUpdate)
	require.Equal(t, decimal.RequireFromString("49900"), fillUpdate.Price)
	require.Equal(t, decimal.RequireFromString("0.25"), fillUpdate.Quantity)
	require.Equal(t, decimal.RequireFromString("0.5"), fillUpdate.Fee)
	require.Equal(t, "USDT", fillUpdate.FeeAsset)
	require.Equal(t, int64(1710000001456), fillUpdate.Timestamp)
}
