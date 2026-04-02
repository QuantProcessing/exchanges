package hyperliquid

import (
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	sdkhyperliquid "github.com/QuantProcessing/exchanges/hyperliquid/sdk"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestPerpMapWsUserFillReturnsExecutionDetails(t *testing.T) {
	adp := &Adapter{}

	got := adp.mapWsUserFill(sdkhyperliquid.WsUserFill{
		Coin:     "BTC",
		Px:       "101",
		Sz:       "0.5",
		Side:     "B",
		Time:     1700000000,
		Tid:      11,
		Oid:      22,
		Fee:      "0.01",
		FeeToken: "USDC",
		Crossed:  false,
	})

	require.Equal(t, "11", got.TradeID)
	require.Equal(t, "22", got.OrderID)
	require.Equal(t, "BTC", got.Symbol)
	require.Equal(t, exchanges.OrderSideBuy, got.Side)
	require.True(t, got.Price.Equal(decimal.RequireFromString("101")))
	require.True(t, got.Quantity.Equal(decimal.RequireFromString("0.5")))
	require.True(t, got.Fee.Equal(decimal.RequireFromString("0.01")))
	require.Equal(t, "USDC", got.FeeAsset)
	require.True(t, got.IsMaker)
	require.EqualValues(t, 1700000000, got.Timestamp)
}

func TestSpotMapWsUserFillReturnsExecutionDetails(t *testing.T) {
	adp := &SpotAdapter{}

	got := adp.mapWsUserFill(sdkhyperliquid.WsUserFill{
		Coin:     "BTC/USDC",
		Px:       "202",
		Sz:       "1.25",
		Side:     "A",
		Time:     1700001234,
		Tid:      33,
		Oid:      44,
		Fee:      "0.02",
		FeeToken: "USDC",
		Crossed:  true,
	})

	require.Equal(t, "33", got.TradeID)
	require.Equal(t, "44", got.OrderID)
	require.Equal(t, "BTC", got.Symbol)
	require.Equal(t, exchanges.OrderSideSell, got.Side)
	require.True(t, got.Price.Equal(decimal.RequireFromString("202")))
	require.True(t, got.Quantity.Equal(decimal.RequireFromString("1.25")))
	require.True(t, got.Fee.Equal(decimal.RequireFromString("0.02")))
	require.Equal(t, "USDC", got.FeeAsset)
	require.False(t, got.IsMaker)
	require.EqualValues(t, 1700001234, got.Timestamp)
}
