package backpack

import (
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/backpack/sdk"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestMapOrderFilled(t *testing.T) {
	raw := sdk.Order{
		ID:               "1",
		ClientID:         7,
		OrderType:        "Market",
		Quantity:         "2",
		ExecutedQuantity: "2",
		ReduceOnly:       true,
		Side:             "Bid",
		Status:           "Filled",
		Symbol:           "BTC_USDC_PERP",
		TimeInForce:      "IOC",
		CreatedAt:        1710000000000,
	}

	got := mapOrder(raw)
	require.Equal(t, "1", got.OrderID)
	require.Equal(t, "7", got.ClientOrderID)
	require.Equal(t, "BTC", got.Symbol)
	require.Equal(t, decimal.NewFromInt(2), got.FilledQuantity)
}

func TestMapSpotBalances(t *testing.T) {
	raw := map[string]sdk.CapitalBalance{
		"USDC": {Available: "10", Locked: "2"},
	}

	got := mapSpotBalances(raw)
	require.Len(t, got, 1)
	require.Equal(t, "USDC", got[0].Asset)
	require.Equal(t, decimal.NewFromInt(12), got[0].Total)
}

func TestMapPositionShort(t *testing.T) {
	raw := sdk.Position{
		Symbol:              "BTC_USDC_PERP",
		NetQuantity:         "-1.5",
		EntryPrice:          "50000",
		PnlUnrealized:       "12",
		EstLiquidationPrice: "60000",
	}

	got := mapPosition(raw)
	require.Equal(t, "BTC", got.Symbol)
	require.Equal(t, decimal.RequireFromString("1.5"), got.Quantity)
	require.Equal(t, decimal.RequireFromString("12"), got.UnrealizedPnL)
}

func TestMapOrderFillReturnsExecutionDetails(t *testing.T) {
	got := mapOrderFill(sdk.OrderUpdateEvent{
		Symbol:          "BTC_USDC_PERP",
		ClientID:        "7",
		Side:            "Ask",
		OrderID:         "1",
		TradeID:         "trade-1",
		FillQuantity:    "0.5",
		FillPrice:       "101.25",
		IsMaker:         true,
		Fee:             "0.01",
		FeeSymbol:       "USDC",
		EngineTimestamp: 1710000000000000,
	})

	require.Equal(t, "trade-1", got.TradeID)
	require.Equal(t, "1", got.OrderID)
	require.Equal(t, "7", got.ClientOrderID)
	require.Equal(t, "BTC", got.Symbol)
	require.Equal(t, exchanges.OrderSideSell, got.Side)
	require.True(t, got.Price.Equal(decimal.RequireFromString("101.25")))
	require.True(t, got.Quantity.Equal(decimal.RequireFromString("0.5")))
	require.True(t, got.Fee.Equal(decimal.RequireFromString("0.01")))
	require.Equal(t, "USDC", got.FeeAsset)
	require.True(t, got.IsMaker)
	require.EqualValues(t, 1710000000000, got.Timestamp)
}
