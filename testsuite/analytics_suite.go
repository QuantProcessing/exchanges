package testsuite

import (
	"context"
	"errors"
	"testing"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RunAnalyticsComplianceTests exercises the P0 analytics surface against a
// real perp adapter. Adapters that have not yet implemented a given capability
// should return ErrNotSupported; those sub-tests will be skipped.
//
// perp is the PerpExchange under test. symbol is a base symbol known to be
// actively traded (e.g. "BTC") so statistics are populated.
func RunAnalyticsComplianceTests(t *testing.T, perp exchanges.PerpExchange, symbol string) {
	t.Run("FetchOpenInterest", func(t *testing.T) {
		TestFetchOpenInterest(t, perp, symbol)
	})

	t.Run("FetchFundingRateHistory", func(t *testing.T) {
		TestFetchFundingRateHistory(t, perp, symbol)
	})

	t.Run("FetchHistoricalTrades", func(t *testing.T) {
		TestFetchHistoricalTrades(t, perp, symbol)
	})

	t.Run("TickerExtendedStats", func(t *testing.T) {
		TestTickerExtendedStats(t, perp, symbol)
	})
}

// TestFetchOpenInterest verifies OI is positive and timestamp is recent.
func TestFetchOpenInterest(t *testing.T, perp exchanges.PerpExchange, symbol string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	oi, err := perp.FetchOpenInterest(ctx, symbol)
	if errors.Is(err, exchanges.ErrNotSupported) {
		t.Skip("adapter does not support FetchOpenInterest")
		return
	}
	require.NoError(t, err)
	require.NotNil(t, oi)

	assert.Equal(t, symbol, oi.Symbol)
	// At least one of contracts / notional must be positive.
	assert.True(t, oi.OIContracts.IsPositive() || oi.OINotional.IsPositive(),
		"expected OIContracts or OINotional to be positive, got contracts=%s notional=%s",
		oi.OIContracts, oi.OINotional)
	assert.Greater(t, oi.Timestamp, int64(0))
}

// TestFetchFundingRateHistory verifies a default call returns >=1 entry and
// respects the Limit option.
func TestFetchFundingRateHistory(t *testing.T, perp exchanges.PerpExchange, symbol string) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	rates, err := perp.FetchFundingRateHistory(ctx, symbol, nil)
	if errors.Is(err, exchanges.ErrNotSupported) {
		t.Skip("adapter does not support FetchFundingRateHistory")
		return
	}
	require.NoError(t, err)
	require.NotEmpty(t, rates, "expected at least 1 historical funding rate")

	for _, r := range rates {
		assert.Equal(t, symbol, r.Symbol)
		assert.Greater(t, r.FundingTime, int64(0))
	}

	limited, err := perp.FetchFundingRateHistory(ctx, symbol, &exchanges.FundingRateHistoryOpts{Limit: 3})
	require.NoError(t, err)
	assert.LessOrEqual(t, len(limited), 3)
}

// TestFetchHistoricalTrades verifies a default call returns trades and the
// Limit option is honored.
func TestFetchHistoricalTrades(t *testing.T, adp exchanges.Exchange, symbol string) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	trades, err := adp.FetchHistoricalTrades(ctx, symbol, &exchanges.HistoricalTradeOpts{Limit: 50})
	if errors.Is(err, exchanges.ErrNotSupported) {
		t.Skip("adapter does not support FetchHistoricalTrades")
		return
	}
	require.NoError(t, err)
	require.NotEmpty(t, trades)
	assert.LessOrEqual(t, len(trades), 50)

	for _, tr := range trades {
		assert.Equal(t, symbol, tr.Symbol)
		assert.True(t, tr.Price.IsPositive())
		assert.True(t, tr.Quantity.IsPositive())
		assert.NotEmpty(t, tr.ID)
	}
}

// TestTickerExtendedStats verifies FetchTicker populates at least the
// 24h-change / open-price / trade-count fields for active symbols.
// Adapters that genuinely cannot surface a given field can document that
// omission in their own adapter_test.
func TestTickerExtendedStats(t *testing.T, adp exchanges.Exchange, symbol string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tkr, err := adp.FetchTicker(ctx, symbol)
	require.NoError(t, err)
	require.NotNil(t, tkr)

	// At least one of the new fields must be populated for an active market.
	populated := !tkr.OpenPrice.IsZero() ||
		!tkr.PriceChange.IsZero() ||
		!tkr.PriceChangePercent.IsZero() ||
		!tkr.WeightedAvgPrice.IsZero() ||
		tkr.TradeCount > 0
	assert.True(t, populated,
		"expected at least one extended stat field to be populated; all were zero")
}
