package testsuite

import (
	"context"
	"testing"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RunAdapterComplianceTests runs a standard set of compliance tests against any exchanges.
func RunAdapterComplianceTests(t *testing.T, adp exchanges.Exchange, symbol string) {
	t.Run("FetchTicker", func(t *testing.T) {
		TestFetchTicker(t, adp, symbol)
	})

	t.Run("FetchOrderBook", func(t *testing.T) {
		TestFetchOrderBook(t, adp, symbol)
	})

	t.Run("WatchOrderBook", func(t *testing.T) {
		TestWatchOrderBook(t, adp, symbol)
	})

	t.Run("FetchAccount", func(t *testing.T) {
		TestFetchAccount(t, adp)
	})

	t.Run("FetchSymbolDetails", func(t *testing.T) {
		TestFetchSymbolDetails(t, adp, symbol)
	})

	t.Run("FetchFeeRate", func(t *testing.T) {
		TestFetchFeeRate(t, adp, symbol)
	})
}

// TestFetchTicker verifies the FetchTicker implementation
func TestFetchTicker(t *testing.T, adp exchanges.Exchange, symbol string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ticker, err := adp.FetchTicker(ctx, symbol)
	require.NoError(t, err, "FetchTicker should not return an error")
	require.NotNil(t, ticker, "Ticker should not be nil")

	assert.Equal(t, symbol, ticker.Symbol)
	assert.True(t, ticker.LastPrice.IsPositive(), "LastPrice should be > 0")
	assert.Greater(t, ticker.Timestamp, int64(0), "Timestamp should be > 0")
}

// TestFetchOrderBook verifies the FetchOrderBook implementation
func TestFetchOrderBook(t *testing.T, adp exchanges.Exchange, symbol string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ob, err := adp.FetchOrderBook(ctx, symbol, 20)
	require.NoError(t, err, "FetchOrderBook should not return an error")
	require.NotNil(t, ob, "OrderBook should not be nil")

	assert.Equal(t, symbol, ob.Symbol)
	if len(ob.Bids) > 0 && len(ob.Asks) > 0 {
		assert.True(t, ob.Bids[0].Price.IsPositive(), "Bid price should be > 0")
		assert.True(t, ob.Asks[0].Price.IsPositive(), "Ask price should be > 0")
		assert.True(t, ob.Bids[0].Price.LessThan(ob.Asks[0].Price), "Top bid should be less than top ask")
	}
}

// TestWatchOrderBook verifies the WebSocket OrderBook subscription and local syncing
func TestWatchOrderBook(t *testing.T, adp exchanges.Exchange, symbol string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := adp.WatchOrderBook(ctx, symbol, nil)
	require.NoError(t, err, "WatchOrderBook should not return error")

	// Give it time to sync
	time.Sleep(3 * time.Second)

	// Get the local orderbook
	ob := adp.GetLocalOrderBook(symbol, 10)
	require.NotNil(t, ob, "Local OrderBook should not be nil after watch")

	assert.Equal(t, symbol, ob.Symbol)

	t.Logf("OrderBook Synced: Bids=%d, Asks=%d", len(ob.Bids), len(ob.Asks))

	if len(ob.Bids) > 0 {
		assert.True(t, ob.Bids[0].Price.IsPositive(), "Top bid price should be > 0")
	}
	if len(ob.Asks) > 0 {
		assert.True(t, ob.Asks[0].Price.IsPositive(), "Top ask price should be > 0")
	}

	// Assert order sorting
	if len(ob.Bids) >= 2 {
		assert.True(t, ob.Bids[0].Price.GreaterThanOrEqual(ob.Bids[1].Price), "Bids should be sorted descending")
	}
	if len(ob.Asks) >= 2 {
		assert.True(t, ob.Asks[0].Price.LessThanOrEqual(ob.Asks[1].Price), "Asks should be sorted ascending")
	}

	// Cleanup
	err = adp.StopWatchOrderBook(context.Background(), symbol)
	if err != nil {
		t.Logf("StopWatchOrderBook returned error (may not be implemented): %v", err)
	}
}

// TestFetchAccount verifies the FetchAccount implementation
func TestFetchAccount(t *testing.T, adp exchanges.Exchange) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	acc, err := adp.FetchAccount(ctx)
	if err == nil && acc != nil {
		t.Logf("FetchAccount succeeded: TotalBalance=%s", acc.TotalBalance)
	} else {
		t.Logf("FetchAccount test skipped/failed (possibly missing API keys): %v", err)
	}
}

// TestFetchSymbolDetails verifies the FetchSymbolDetails implementation
func TestFetchSymbolDetails(t *testing.T, adp exchanges.Exchange, symbol string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	details, err := adp.FetchSymbolDetails(ctx, symbol)
	require.NoError(t, err, "FetchSymbolDetails should not return error")
	require.NotNil(t, details, "SymbolDetails should not be nil")

	t.Logf("SymbolDetails: Symbol=%s PricePrecision=%d QtyPrecision=%d MinQty=%s MinNotional=%s",
		details.Symbol, details.PricePrecision, details.QuantityPrecision, details.MinQuantity, details.MinNotional)
}

// TestFetchFeeRate verifies the FetchFeeRate implementation
func TestFetchFeeRate(t *testing.T, adp exchanges.Exchange, symbol string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	feeRate, err := adp.FetchFeeRate(ctx, symbol)
	if err == nil && feeRate != nil {
		t.Logf("FetchFeeRate: Maker=%s, Taker=%s", feeRate.Maker, feeRate.Taker)
	} else {
		t.Logf("FetchFeeRate skipped (may not be implemented): %v", err)
	}
}
