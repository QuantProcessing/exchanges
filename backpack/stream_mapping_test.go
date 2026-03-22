package backpack

import (
	"testing"

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
	require.Equal(t, decimal.RequireFromString("1.5"), got.Fee)
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
