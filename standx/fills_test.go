package standx

import (
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	sdkstandx "github.com/QuantProcessing/exchanges/standx/sdk"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestMapSDKTradeToFillReturnsExecutionDetails(t *testing.T) {
	adp := &Adapter{}

	fill := adp.mapSDKTradeToFill(&sdkstandx.Trade{
		ID:        11,
		OrderID:   22,
		Symbol:    "BTC-USD",
		Side:      "sell",
		Price:     "101.25",
		Qty:       "0.5",
		FeeQty:    "0.01",
		FeeAsset:  "USDC",
		UpdatedAt: "2026-04-02T10:11:12.000Z",
	})

	require.Equal(t, "11", fill.TradeID)
	require.Equal(t, "22", fill.OrderID)
	require.Equal(t, "BTC", fill.Symbol)
	require.Equal(t, exchanges.OrderSideSell, fill.Side)
	require.True(t, fill.Price.Equal(decimal.RequireFromString("101.25")))
	require.True(t, fill.Quantity.Equal(decimal.RequireFromString("0.5")))
	require.True(t, fill.Fee.Equal(decimal.RequireFromString("0.01")))
	require.Equal(t, "USDC", fill.FeeAsset)
	require.EqualValues(t, 1775124672000, fill.Timestamp)
}
