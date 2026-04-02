package bitget

import (
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/bitget/sdk"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestMapClassicMixFillReturnsExecutionDetails(t *testing.T) {
	fill := mapClassicMixFill("BTC", sdk.ClassicMixFillRecord{
		OrderID:    "111",
		ClientOID:  "cli-111",
		TradeID:    "222",
		Symbol:     "BTCUSDT",
		Side:       "buy",
		OrderType:  "market",
		Price:      "51000.5",
		BaseVolume: "0.01",
		TradeScope: "taker",
		FeeDetail: []sdk.ClassicFillFeeDetail{
			{
				FeeCoin:   "USDT",
				TotalFee:  "-0.183717",
				Deduction: "no",
			},
		},
		UTime: "1703577336606",
	})

	require.NotNil(t, fill)
	require.Equal(t, "222", fill.TradeID)
	require.Equal(t, "111", fill.OrderID)
	require.Equal(t, "cli-111", fill.ClientOrderID)
	require.Equal(t, "BTC", fill.Symbol)
	require.Equal(t, exchanges.OrderSideBuy, fill.Side)
	require.True(t, decimal.RequireFromString("51000.5").Equal(fill.Price))
	require.True(t, decimal.RequireFromString("0.01").Equal(fill.Quantity))
	require.True(t, decimal.RequireFromString("-0.183717").Equal(fill.Fee))
	require.Equal(t, "USDT", fill.FeeAsset)
	require.False(t, fill.IsMaker)
	require.Equal(t, int64(1703577336606), fill.Timestamp)
}

func TestMapClassicSpotFillReturnsExecutionDetails(t *testing.T) {
	fill := mapClassicSpotFill("BTC", sdk.ClassicSpotFillRecord{
		OrderID:    "111",
		TradeID:    "222",
		Symbol:     "BTCUSDT",
		Side:       "sell",
		OrderType:  "limit",
		PriceAvg:   "42741.46",
		Size:       "0.0006",
		TradeScope: "marker",
		FeeDetail: []sdk.ClassicFillFeeDetail{
			{
				FeeCoin:           "USDT",
				TotalDeductionFee: "0.01538693",
			},
		},
		CTime: "1703580202094",
	})

	require.NotNil(t, fill)
	require.Equal(t, "222", fill.TradeID)
	require.Equal(t, "111", fill.OrderID)
	require.Equal(t, "BTC", fill.Symbol)
	require.Equal(t, exchanges.OrderSideSell, fill.Side)
	require.True(t, decimal.RequireFromString("42741.46").Equal(fill.Price))
	require.True(t, decimal.RequireFromString("0.0006").Equal(fill.Quantity))
	require.True(t, decimal.RequireFromString("0.01538693").Equal(fill.Fee))
	require.Equal(t, "USDT", fill.FeeAsset)
	require.True(t, fill.IsMaker)
	require.Equal(t, int64(1703580202094), fill.Timestamp)
}
