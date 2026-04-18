package exchanges_test

import (
	"testing"
	"time"

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

func TestOpenInterestModel(t *testing.T) {
	t.Parallel()

	oi := exchanges.OpenInterest{
		Symbol:      "BTC",
		OIContracts: decimal.RequireFromString("12345.6"),
		OINotional:  decimal.RequireFromString("620000000"),
		Timestamp:   1700000000000,
	}

	require.Equal(t, "BTC", oi.Symbol)
	require.Equal(t, "12345.6", oi.OIContracts.String())
	require.Equal(t, "620000000", oi.OINotional.String())
	require.Equal(t, int64(1700000000000), oi.Timestamp)
}

func TestFundingRateHistoryOptsZeroValue(t *testing.T) {
	t.Parallel()

	var opts exchanges.FundingRateHistoryOpts
	require.Nil(t, opts.Start)
	require.Nil(t, opts.End)
	require.Zero(t, opts.Limit)
}

func TestHistoricalTradeOptsZeroValue(t *testing.T) {
	t.Parallel()

	start := time.UnixMilli(1700000000000)
	opts := exchanges.HistoricalTradeOpts{
		Start:  &start,
		FromID: "abc",
		Limit:  500,
	}
	require.Equal(t, "abc", opts.FromID)
	require.Equal(t, 500, opts.Limit)
	require.NotNil(t, opts.Start)
	require.Nil(t, opts.End)
}
