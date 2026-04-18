package exchanges_test

import (
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestTickerExposesExtendedStatistics(t *testing.T) {
	t.Parallel()

	tkr := exchanges.Ticker{
		Symbol:             "BTC",
		LastPrice:          decimal.RequireFromString("50000"),
		PriceChange:        decimal.RequireFromString("500"),
		PriceChangePercent: decimal.RequireFromString("1.01"),
		OpenPrice:          decimal.RequireFromString("49500"),
		WeightedAvgPrice:   decimal.RequireFromString("49750"),
		TradeCount:         12345,
	}

	require.Equal(t, "500", tkr.PriceChange.String())
	require.Equal(t, "1.01", tkr.PriceChangePercent.String())
	require.Equal(t, "49500", tkr.OpenPrice.String())
	require.Equal(t, "49750", tkr.WeightedAvgPrice.String())
	require.Equal(t, int64(12345), tkr.TradeCount)
}
