package okx

import (
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	sdkokx "github.com/QuantProcessing/exchanges/okx/sdk"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestPerpMapOrderFillReturnsExecutionDetails(t *testing.T) {
	adp := &Adapter{
		idMap: map[string]string{"BTC-USDT-SWAP": "BTC"},
		instruments: map[string]sdkokx.Instrument{
			"BTC-USDT-SWAP": {CtVal: "0.01"},
		},
	}

	fill := adp.mapOrderFill(&sdkokx.Order{
		InstId:   "BTC-USDT-SWAP",
		OrdId:    "order-1",
		ClOrdId:  "client-1",
		TradeId:  "trade-1",
		Side:     sdkokx.SideSell,
		FillPx:   "101.25",
		FillSz:   "50",
		FillTime: "1700000000000",
		Fee:      "-0.01",
		FeeCcy:   "USDT",
		ExecType: "M",
	})

	require.Equal(t, "trade-1", fill.TradeID)
	require.Equal(t, "order-1", fill.OrderID)
	require.Equal(t, "client-1", fill.ClientOrderID)
	require.Equal(t, "BTC", fill.Symbol)
	require.Equal(t, exchanges.OrderSideSell, fill.Side)
	require.True(t, fill.Price.Equal(decimal.RequireFromString("101.25")))
	require.True(t, fill.Quantity.Equal(decimal.RequireFromString("0.5")))
	require.True(t, fill.Fee.Equal(decimal.RequireFromString("-0.01")))
	require.Equal(t, "USDT", fill.FeeAsset)
	require.True(t, fill.IsMaker)
	require.EqualValues(t, 1700000000000, fill.Timestamp)
}

func TestSpotMapOrderFillReturnsExecutionDetails(t *testing.T) {
	adp := &SpotAdapter{idMap: map[string]string{"ETH-USDT": "ETH"}}

	fill := adp.mapOrderFill(&sdkokx.Order{
		InstId:   "ETH-USDT",
		OrdId:    "order-2",
		ClOrdId:  "client-2",
		TradeId:  "trade-2",
		Side:     sdkokx.SideBuy,
		FillPx:   "202.5",
		FillSz:   "1.25",
		FillTime: "1700000000123",
		Fee:      "0.02",
		FeeCcy:   "USDT",
		ExecType: "T",
	})

	require.Equal(t, "trade-2", fill.TradeID)
	require.Equal(t, "order-2", fill.OrderID)
	require.Equal(t, "client-2", fill.ClientOrderID)
	require.Equal(t, "ETH", fill.Symbol)
	require.Equal(t, exchanges.OrderSideBuy, fill.Side)
	require.True(t, fill.Price.Equal(decimal.RequireFromString("202.5")))
	require.True(t, fill.Quantity.Equal(decimal.RequireFromString("1.25")))
	require.True(t, fill.Fee.Equal(decimal.RequireFromString("0.02")))
	require.Equal(t, "USDT", fill.FeeAsset)
	require.False(t, fill.IsMaker)
	require.EqualValues(t, 1700000000123, fill.Timestamp)
}
