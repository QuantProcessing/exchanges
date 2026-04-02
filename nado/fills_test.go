package nado

import (
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	sdknado "github.com/QuantProcessing/exchanges/nado/sdk"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestPerpMapFillReturnsExecutionDetails(t *testing.T) {
	adp := &Adapter{idMap: map[int64]string{1: "BTC"}}

	got := adp.mapFill(&sdknado.Fill{
		TradeId:   "trade-1",
		ProductId: 1,
		Price:     "101000000000000000000",
		Size:      "500000000000000000",
		Side:      "buy",
		Fee:       "10000000000000000",
		Time:      1700000000,
	})

	require.Equal(t, "trade-1", got.TradeID)
	require.Equal(t, "BTC", got.Symbol)
	require.Equal(t, exchanges.OrderSideBuy, got.Side)
	require.True(t, got.Price.Equal(decimal.RequireFromString("101")))
	require.True(t, got.Quantity.Equal(decimal.RequireFromString("0.5")))
	require.True(t, got.Fee.Equal(decimal.RequireFromString("0.01")))
	require.EqualValues(t, 1700000000, got.Timestamp)
}

func TestSpotMapFillReturnsExecutionDetails(t *testing.T) {
	adp := &SpotAdapter{idMap: map[int64]string{2: "ETH"}}

	got := adp.mapFill(&sdknado.Fill{
		TradeId:   "trade-2",
		ProductId: 2,
		Price:     "2500000000000000000000",
		Size:      "3000000000000000000",
		Side:      "sell",
		Fee:       "500000000000000000",
		Time:      1700000123,
	})

	require.Equal(t, "trade-2", got.TradeID)
	require.Equal(t, "ETH", got.Symbol)
	require.Equal(t, exchanges.OrderSideSell, got.Side)
	require.True(t, got.Price.Equal(decimal.RequireFromString("2500")))
	require.True(t, got.Quantity.Equal(decimal.RequireFromString("3")))
	require.True(t, got.Fee.Equal(decimal.RequireFromString("0.5")))
	require.EqualValues(t, 1700000123, got.Timestamp)
}
