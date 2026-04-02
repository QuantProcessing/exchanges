package backpack

import (
	"encoding/json"
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/backpack/sdk"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestMapOrderUpdate(t *testing.T) {
	t.Parallel()

	got := mapOrderUpdate(sdk.OrderUpdateEvent{
		EventType:             "orderFill",
		EventTime:             1710000000000000,
		Symbol:                "BTC_USDC_PERP",
		ClientID:              "42",
		Side:                  "Bid",
		OrderType:             "Limit",
		TimeInForce:           "GTC",
		Quantity:              "2",
		Price:                 "50000",
		OrderState:            "Filled",
		OrderID:               "abc",
		TradeID:               "1234",
		FillQuantity:          "2",
		ExecutedQuantity:      "2",
		ExecutedQuoteQuantity: "100000",
		FillPrice:             "50000",
		Fee:                   "1.5",
		FeeSymbol:             "USDC",
		EngineTimestamp:       1710000000000001,
	})

	require.Equal(t, "abc", got.OrderID)
	require.Equal(t, "42", got.ClientOrderID)
	require.Equal(t, "BTC", got.Symbol)
	require.Equal(t, decimal.RequireFromString("2"), got.FilledQuantity)
	require.Equal(t, decimal.RequireFromString("50000"), got.Price)
	require.Equal(t, decimal.RequireFromString("50000"), got.OrderPrice)
	require.True(t, got.AverageFillPrice.IsZero())
	require.True(t, got.LastFillPrice.IsZero())
	require.True(t, got.LastFillQuantity.IsZero())
	require.True(t, got.Fee.IsZero())
	require.Equal(t, int64(1710000000000), got.Timestamp)
}

func TestMapPositionUpdateShort(t *testing.T) {
	t.Parallel()

	got := mapPositionUpdate(sdk.PositionUpdateEvent{
		Symbol:          "BTC_USDC_PERP",
		EntryPrice:      "50000",
		NetQuantity:     "-1.25",
		PnlRealized:     "3.5",
		PnlUnrealized:   "1.25",
		EngineTimestamp: 1710000000000000,
	})

	require.Equal(t, "BTC", got.Symbol)
	require.Equal(t, decimal.RequireFromString("1.25"), got.Quantity)
	require.Equal(t, decimal.RequireFromString("50000"), got.EntryPrice)
	require.Equal(t, decimal.RequireFromString("1.25"), got.UnrealizedPnL)
	require.Equal(t, decimal.RequireFromString("3.5"), got.RealizedPnL)
}

func TestDispatchPrivateOrderUpdate_FansOutOverviewOrderAndDetailedFill(t *testing.T) {
	t.Parallel()

	adp := &Adapter{}

	var orderUpdate *exchanges.Order
	var fillUpdate *exchanges.Fill
	adp.privateOrderStream.orderCB = func(order *exchanges.Order) {
		orderUpdate = order
	}
	adp.privateOrderStream.fillCB = func(fill *exchanges.Fill) {
		fillUpdate = fill
	}

	payload, err := json.Marshal(sdk.OrderUpdateEvent{
		EventType:             "orderFill",
		EventTime:             1710000000000000,
		Symbol:                "BTC_USDC_PERP",
		ClientID:              "42",
		Side:                  "Bid",
		OrderType:             "Limit",
		TimeInForce:           "GTC",
		Quantity:              "2",
		Price:                 "50000",
		OrderState:            "Filled",
		OrderID:               "abc",
		TradeID:               "1234",
		FillQuantity:          "0.5",
		ExecutedQuantity:      "2",
		ExecutedQuoteQuantity: "100000",
		FillPrice:             "49900",
		Fee:                   "1.5",
		FeeSymbol:             "USDC",
		EngineTimestamp:       1710000000000001,
	})
	require.NoError(t, err)

	adp.dispatchPrivateOrderUpdate(payload, true)

	require.NotNil(t, orderUpdate)
	require.Equal(t, decimal.RequireFromString("50000"), orderUpdate.Price)
	require.Equal(t, decimal.RequireFromString("50000"), orderUpdate.OrderPrice)
	require.True(t, orderUpdate.AverageFillPrice.IsZero())
	require.True(t, orderUpdate.LastFillPrice.IsZero())
	require.True(t, orderUpdate.LastFillQuantity.IsZero())
	require.True(t, orderUpdate.Fee.IsZero())

	require.NotNil(t, fillUpdate)
	require.Equal(t, "1234", fillUpdate.TradeID)
	require.Equal(t, decimal.RequireFromString("49900"), fillUpdate.Price)
	require.Equal(t, decimal.RequireFromString("0.5"), fillUpdate.Quantity)
	require.Equal(t, decimal.RequireFromString("1.5"), fillUpdate.Fee)
	require.Equal(t, "USDC", fillUpdate.FeeAsset)
}
