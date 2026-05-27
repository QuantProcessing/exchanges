# Analytics P0 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the P0 data-analysis primitives (open interest, funding rate history, extended ticker statistics, paginated trade history) to the unified exchange SDK, with real implementations for Binance perp and OKX perp and `ErrNotSupported` stubs for every other adapter.

**Architecture:**
- Extend `Ticker` with five additional 24h stat fields (non-breaking struct additions).
- Add two new methods to `PerpExchange`: `FetchOpenInterest`, `FetchFundingRateHistory`. Add `FetchHistoricalTrades` to `Exchange` (all markets).
- Deliberately NOT touching the `FetchTrades(ctx, symbol, limit)` signature. Adding `FetchHistoricalTrades(ctx, symbol, opts *HistoricalTradeOpts)` as a separate method maps cleanly to how exchanges split the endpoints (e.g. Binance `/trades` vs `/aggTrades` vs `/historicalTrades`), avoids breaking 27 call sites, and keeps "give me latest N" a cheap one-liner.
- New shared compliance tests in `testsuite/analytics_suite.go`; adapters wire them in `adapter_test.go`. Unsupported adapters must return `exchanges.ErrNotSupported` from new methods (matches repo convention per `docs/contributing/adding-exchange-adapters.md`).

**Tech Stack:** Go 1.26, `github.com/shopspring/decimal`, `github.com/stretchr/testify`. Reference: Binance Futures USD-M REST docs (`/fapi/v1/openInterest`, `/fapi/v1/fundingRate`, `/fapi/v1/ticker/24hr`, `/fapi/v1/aggTrades`); OKX v5 REST docs (`/api/v5/public/open-interest`, `/api/v5/public/funding-rate-history`, `/api/v5/market/ticker`, `/api/v5/market/history-trades`).

**Scope boundary:** This plan produces a shippable slice. Remaining perp adapters (aster/bitget/bybit/nado/lighter/hyperliquid/standx/grvt/edgex/decibel/backpack) get `ErrNotSupported` stubs so the codebase compiles; their real implementations are follow-up plans. Spot adapters get only the Ticker field additions (mostly a no-op since most don't populate these fields today) and the `FetchHistoricalTrades` `ErrNotSupported` default via `BaseAdapter`.

---

## File Structure

**Modify:**
- `models.go` — extend `Ticker`, add `OpenInterest`, `FundingRateHistoryOpts`, `HistoricalTradeOpts` types.
- `exchange.go` — add `FetchHistoricalTrades` to `Exchange`, `FetchOpenInterest` + `FetchFundingRateHistory` to `PerpExchange`.
- `base_adapter.go` — add `FetchHistoricalTrades` default returning `ErrNotSupported`.
- `public_contract_test.go` — extend `stubExchange` with new methods.
- `config/config_test.go` — extend embedded stub.
- `account/account_runtime_test.go` — extend embedded stub.
- `binance/sdk/perp/market.go` — add `GetOpenInterest`, `GetFundingRateHistory`.
- `binance/funding.go` — add `FetchFundingRateHistory`.
- `binance/perp_adapter.go` — add `FetchOpenInterest`, populate new `Ticker` fields, add `FetchHistoricalTrades`.
- `binance/adapter_test.go` — wire `RunAnalyticsComplianceTests`.
- `okx/sdk/market.go` — add `GetOpenInterest`, `GetFundingRateHistory`, `GetHistoryTrades`.
- `okx/funding.go` — add `FetchFundingRateHistory`.
- `okx/perp_adapter.go` — add `FetchOpenInterest`, populate new `Ticker` fields, add `FetchHistoricalTrades`.
- `okx/adapter_test.go` — wire `RunAnalyticsComplianceTests`.
- `aster/perp_adapter.go`, `bitget/perp_adapter.go`, `bybit/perp_adapter.go`, `nado/perp_adapter.go`, `lighter/perp_adapter.go`, `hyperliquid/perp_adapter.go`, `standx/perp_adapter.go`, `grvt/perp_adapter.go`, `edgex/perp_adapter.go`, `decibel/perp_adapter.go`, `backpack/perp_adapter.go` — add `FetchOpenInterest` + `FetchFundingRateHistory` stubs.

**Create:**
- `testsuite/analytics_suite.go` — shared compliance tests.
- `analytics_contract_test.go` (root) — unit tests pinning new types/struct fields.

---

## Phase 1: Core Types & Interface

### Task 1: Extend `Ticker` with 24h statistics fields

**Files:**
- Modify: `models.go:183-196`
- Test: `analytics_contract_test.go` (new)

- [ ] **Step 1: Write failing test**

Create `/Users/dddd/Documents/GitHub/exchanges/analytics_contract_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -run TestTickerExposesExtendedStatistics ./...`
Expected: FAIL with unknown field `PriceChange` (or similar) on `exchanges.Ticker`.

- [ ] **Step 3: Extend the struct**

Modify `models.go`. Replace the existing `Ticker` struct block (currently lines 183-196) with:

```go
// Ticker represents real-time market data for a symbol.
type Ticker struct {
	Symbol             string          `json:"symbol"`
	LastPrice          decimal.Decimal `json:"last_price"`
	IndexPrice         decimal.Decimal `json:"index_price"`
	MarkPrice          decimal.Decimal `json:"mark_price"`
	MidPrice           decimal.Decimal `json:"mid_price"`
	Bid                decimal.Decimal `json:"bid"`
	Ask                decimal.Decimal `json:"ask"`
	Volume24h          decimal.Decimal `json:"volume_24h"`
	QuoteVol           decimal.Decimal `json:"quote_vol"`
	High24h            decimal.Decimal `json:"high_24h"`
	Low24h             decimal.Decimal `json:"low_24h"`
	OpenPrice          decimal.Decimal `json:"open_price,omitempty"`
	PriceChange        decimal.Decimal `json:"price_change,omitempty"`
	PriceChangePercent decimal.Decimal `json:"price_change_percent,omitempty"`
	WeightedAvgPrice   decimal.Decimal `json:"weighted_avg_price,omitempty"`
	TradeCount         int64           `json:"trade_count,omitempty"`
	Timestamp          int64           `json:"timestamp"`
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -run TestTickerExposesExtendedStatistics ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add models.go analytics_contract_test.go
git commit -m "Extend Ticker with 24h statistics fields"
```

---

### Task 2: Add analytics model types

**Files:**
- Modify: `models.go` (append at end of struct definitions section)
- Test: `analytics_contract_test.go`

- [ ] **Step 1: Write failing tests**

Append to `analytics_contract_test.go`:

```go
import "time"

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
```

Note: the `time` import line goes into the existing `import (...)` block; do not create a duplicate block.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -run "TestOpenInterestModel|TestFundingRateHistoryOptsZeroValue|TestHistoricalTradeOptsZeroValue" ./...`
Expected: FAIL with undefined types.

- [ ] **Step 3: Add the types**

Append to `models.go` right after the `FundingRate` struct (currently ends around line 258):

```go
// OpenInterest represents the current open interest for a perpetual futures symbol.
// OIContracts is denominated in base asset (e.g. BTC). OINotional is in quote asset.
// Some exchanges only report one of the two; the unset value is zero.
type OpenInterest struct {
	Symbol      string          `json:"symbol"`
	OIContracts decimal.Decimal `json:"oi_contracts"`
	OINotional  decimal.Decimal `json:"oi_notional"`
	Timestamp   int64           `json:"timestamp"`
}

// FundingRateHistoryOpts controls FetchFundingRateHistory paging.
// Start/End are inclusive range endpoints; nil means "unbounded".
// Limit is exchange-dependent; zero means "adapter default".
type FundingRateHistoryOpts struct {
	Start *time.Time
	End   *time.Time
	Limit int
}

// HistoricalTradeOpts controls FetchHistoricalTrades paging.
// FromID is exchange-specific trade ID cursor (e.g. Binance aggTrade ID, OKX tradeId).
// If FromID is set, Start/End are ignored by exchanges that pick one or the other.
type HistoricalTradeOpts struct {
	Start  *time.Time
	End    *time.Time
	FromID string
	Limit  int
}
```

Ensure `models.go` already imports `time`. If it does not, add `"time"` to the existing import block.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -run "TestOpenInterestModel|TestFundingRateHistoryOptsZeroValue|TestHistoricalTradeOptsZeroValue" ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add models.go analytics_contract_test.go
git commit -m "Add OpenInterest, FundingRateHistoryOpts, HistoricalTradeOpts models"
```

---

### Task 3: Extend `Exchange` and `PerpExchange` interfaces

**Files:**
- Modify: `exchange.go:21-70` (Exchange), `exchange.go:78-86` (PerpExchange)
- Modify: `base_adapter.go` (append)
- Modify: `public_contract_test.go:36-124`
- Modify: `config/config_test.go`
- Modify: `account/account_runtime_test.go`

- [ ] **Step 1: Write failing test**

Append to `analytics_contract_test.go`:

```go
func TestPerpExchangeInterfaceIncludesAnalyticsMethods(t *testing.T) {
	t.Parallel()

	// Compile-time assertion: a type that implements PerpExchange must have
	// the new methods. This test fails to compile if the interface regresses.
	var _ exchanges.PerpExchange = (*analyticsInterfaceProbe)(nil)
}

type analyticsInterfaceProbe struct {
	*exchanges.BaseAdapter
}

// minimum stubs to satisfy PerpExchange; real assertions live elsewhere.
func (*analyticsInterfaceProbe) FetchTicker(context.Context, string) (*exchanges.Ticker, error) {
	return nil, nil
}
func (*analyticsInterfaceProbe) FetchOrderBook(context.Context, string, int) (*exchanges.OrderBook, error) {
	return nil, nil
}
func (*analyticsInterfaceProbe) FetchTrades(context.Context, string, int) ([]exchanges.Trade, error) {
	return nil, nil
}
func (*analyticsInterfaceProbe) FetchKlines(context.Context, string, exchanges.Interval, *exchanges.KlineOpts) ([]exchanges.Kline, error) {
	return nil, nil
}
func (*analyticsInterfaceProbe) PlaceOrder(context.Context, *exchanges.OrderParams) (*exchanges.Order, error) {
	return nil, nil
}
func (*analyticsInterfaceProbe) PlaceOrderWS(context.Context, *exchanges.OrderParams) error {
	return nil
}
func (*analyticsInterfaceProbe) CancelOrder(context.Context, string, string) error { return nil }
func (*analyticsInterfaceProbe) CancelOrderWS(context.Context, string, string) error {
	return nil
}
func (*analyticsInterfaceProbe) CancelAllOrders(context.Context, string) error { return nil }
func (*analyticsInterfaceProbe) FetchOrderByID(context.Context, string, string) (*exchanges.Order, error) {
	return nil, nil
}
func (*analyticsInterfaceProbe) FetchOrders(context.Context, string) ([]exchanges.Order, error) {
	return nil, nil
}
func (*analyticsInterfaceProbe) FetchOpenOrders(context.Context, string) ([]exchanges.Order, error) {
	return nil, nil
}
func (*analyticsInterfaceProbe) FetchAccount(context.Context) (*exchanges.Account, error) {
	return nil, nil
}
func (*analyticsInterfaceProbe) FetchBalance(context.Context) (decimal.Decimal, error) {
	return decimal.Zero, nil
}
func (*analyticsInterfaceProbe) FetchSymbolDetails(context.Context, string) (*exchanges.SymbolDetails, error) {
	return nil, nil
}
func (*analyticsInterfaceProbe) FetchFeeRate(context.Context, string) (*exchanges.FeeRate, error) {
	return nil, nil
}
func (*analyticsInterfaceProbe) FetchPositions(context.Context) ([]exchanges.Position, error) {
	return nil, nil
}
func (*analyticsInterfaceProbe) SetLeverage(context.Context, string, int) error { return nil }
func (*analyticsInterfaceProbe) FetchFundingRate(context.Context, string) (*exchanges.FundingRate, error) {
	return nil, nil
}
func (*analyticsInterfaceProbe) FetchAllFundingRates(context.Context) ([]exchanges.FundingRate, error) {
	return nil, nil
}
func (*analyticsInterfaceProbe) ModifyOrder(context.Context, string, string, *exchanges.ModifyOrderParams) (*exchanges.Order, error) {
	return nil, nil
}
func (*analyticsInterfaceProbe) ModifyOrderWS(context.Context, string, string, *exchanges.ModifyOrderParams) error {
	return nil
}
func (*analyticsInterfaceProbe) WatchOrders(context.Context, exchanges.OrderUpdateCallback) error {
	return nil
}
func (*analyticsInterfaceProbe) WatchFills(context.Context, exchanges.FillCallback) error {
	return nil
}
func (*analyticsInterfaceProbe) WatchPositions(context.Context, exchanges.PositionUpdateCallback) error {
	return nil
}
func (*analyticsInterfaceProbe) WatchTicker(context.Context, string, exchanges.TickerCallback) error {
	return nil
}
func (*analyticsInterfaceProbe) WatchTrades(context.Context, string, exchanges.TradeCallback) error {
	return nil
}
func (*analyticsInterfaceProbe) WatchKlines(context.Context, string, exchanges.Interval, exchanges.KlineCallback) error {
	return nil
}
func (*analyticsInterfaceProbe) StopWatchOrders(context.Context) error { return nil }
func (*analyticsInterfaceProbe) StopWatchFills(context.Context) error  { return nil }
func (*analyticsInterfaceProbe) StopWatchPositions(context.Context) error {
	return nil
}
func (*analyticsInterfaceProbe) StopWatchTicker(context.Context, string) error { return nil }
func (*analyticsInterfaceProbe) StopWatchTrades(context.Context, string) error { return nil }
func (*analyticsInterfaceProbe) StopWatchKlines(context.Context, string, exchanges.Interval) error {
	return nil
}
```

Add required imports to the test file: `"context"`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go build ./...`
Expected: FAIL with "does not implement PerpExchange (missing method FetchOpenInterest / FetchFundingRateHistory / FetchHistoricalTrades)" — depending on order. This proves the interface does not yet require the methods.

Actually: the test as written right now should COMPILE (since interface still does not require the new methods) and PASS. This is a positive-control test. We'll replace the test body after the interface grows to confirm the probe is still valid. Skip ahead to step 3.

- [ ] **Step 3: Extend the `Exchange` interface**

Modify `exchange.go`. In the `Exchange` interface block (starting around line 21), add the new method right after the existing `FetchTrades` line:

```go
	// FetchHistoricalTrades returns paginated historical trades.
	// opts may be nil; a nil opts means "most recent page, adapter default limit".
	// Adapters that do not support paginated history must return ErrNotSupported.
	FetchHistoricalTrades(ctx context.Context, symbol string, opts *HistoricalTradeOpts) ([]Trade, error)
```

In the `PerpExchange` interface block (line 78-86), add after `FetchAllFundingRates`:

```go
	// FetchFundingRateHistory returns historical funding rates for a symbol.
	// opts may be nil for "most recent adapter-default page".
	// Hourly normalization: returned FundingRate entries use the same
	// per-hour convention as FetchFundingRate.
	FetchFundingRateHistory(ctx context.Context, symbol string, opts *FundingRateHistoryOpts) ([]FundingRate, error)
	// FetchOpenInterest returns current open interest for a perp symbol.
	FetchOpenInterest(ctx context.Context, symbol string) (*OpenInterest, error)
```

- [ ] **Step 4: Add default `FetchHistoricalTrades` to `BaseAdapter`**

Append to `base_adapter.go`:

```go
// FetchHistoricalTrades is the default implementation for adapters that do not
// support paginated trade history. Override in the concrete adapter to provide
// real behavior.
func (b *BaseAdapter) FetchHistoricalTrades(ctx context.Context, symbol string, opts *HistoricalTradeOpts) ([]Trade, error) {
	_ = ctx
	_ = symbol
	_ = opts
	return nil, ErrNotSupported
}
```

- [ ] **Step 5: Extend stub exchanges so the tree still compiles**

Modify `public_contract_test.go`. Add these two methods to `stubExchange` right after `FetchTrades` (around line 42):

```go
func (s *stubExchange) FetchHistoricalTrades(context.Context, string, *exchanges.HistoricalTradeOpts) ([]exchanges.Trade, error) {
	return nil, nil
}
```

Modify `config/config_test.go` — find the `stubExchange` definition (line 58 has `FetchTrades`) and add the same method right after. Same for `account/account_runtime_test.go`.

- [ ] **Step 6: Run full compile**

Run: `go build ./...`
Expected: SUCCESS — all adapters will fail because they don't implement `FetchOpenInterest`/`FetchFundingRateHistory` yet. Ignore those; we fix them per-adapter in later tasks. The root package, config, and account package must build.

Expected actual output: errors like `binance.Adapter does not implement exchanges.PerpExchange`. That is fine — we resolve in subsequent tasks.

- [ ] **Step 7: Run core-package tests**

Run: `go test -run "TestTicker|TestOpenInterest|TestFundingRateHistoryOpts|TestHistoricalTradeOpts|TestPerpExchangeInterface" github.com/QuantProcessing/exchanges`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add exchange.go models.go base_adapter.go public_contract_test.go config/config_test.go account/account_runtime_test.go analytics_contract_test.go
git commit -m "Add analytics interface surface: FetchOpenInterest, FetchFundingRateHistory, FetchHistoricalTrades"
```

Note: the tree will not fully compile at this commit because adapters lack the new perp methods. That is intentional — the next tasks add them atomically per adapter. If the repo's policy forbids broken-compile commits, combine Task 3 with Task 4's stub patches into a single commit.

---

## Phase 2: Shared compliance suite

### Task 4: Write `testsuite/analytics_suite.go`

**Files:**
- Create: `testsuite/analytics_suite.go`

- [ ] **Step 1: Create the suite file**

Write `/Users/dddd/Documents/GitHub/exchanges/testsuite/analytics_suite.go`:

```go
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
```

- [ ] **Step 2: Verify the suite compiles**

Run: `go build ./testsuite/...`
Expected: SUCCESS.

- [ ] **Step 3: Commit**

```bash
git add testsuite/analytics_suite.go
git commit -m "Add shared analytics compliance suite"
```

---

## Phase 3: Binance perp reference implementation

### Task 5: Add Binance SDK types and methods

**Files:**
- Modify: `binance/sdk/perp/market.go` (append after the existing funding-rate methods)

- [ ] **Step 1: Write failing SDK test**

Open `binance/sdk/perp/market_test.go` and append:

```go
func TestGetOpenInterestParses(t *testing.T) {
	t.Parallel()

	payload := `{"symbol":"BTCUSDT","openInterest":"12345.678","time":1700000000000}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/fapi/v1/openInterest", r.URL.Path)
		require.Equal(t, "BTCUSDT", r.URL.Query().Get("symbol"))
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	c := NewClient(ClientConfig{BaseURL: srv.URL})
	oi, err := c.GetOpenInterest(context.Background(), "BTCUSDT")
	require.NoError(t, err)
	require.Equal(t, "BTCUSDT", oi.Symbol)
	require.Equal(t, "12345.678", oi.OpenInterest)
	require.Equal(t, int64(1700000000000), oi.Time)
}

func TestGetFundingRateHistoryParses(t *testing.T) {
	t.Parallel()

	payload := `[
		{"symbol":"BTCUSDT","fundingRate":"0.0001","fundingTime":1700000000000,"markPrice":"50000"},
		{"symbol":"BTCUSDT","fundingRate":"0.00012","fundingTime":1700028800000,"markPrice":"50100"}
	]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/fapi/v1/fundingRate", r.URL.Path)
		require.Equal(t, "BTCUSDT", r.URL.Query().Get("symbol"))
		require.Equal(t, "5", r.URL.Query().Get("limit"))
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	c := NewClient(ClientConfig{BaseURL: srv.URL})
	hist, err := c.GetFundingRateHistory(context.Background(), "BTCUSDT", 0, 0, 5)
	require.NoError(t, err)
	require.Len(t, hist, 2)
	require.Equal(t, "0.0001", hist[0].FundingRate)
	require.Equal(t, int64(1700028800000), hist[1].FundingTime)
}
```

Required imports at the top of `market_test.go` (add if not already present): `"context"`, `"net/http"`, `"net/http/httptest"`, `"testing"`, `"github.com/stretchr/testify/require"`.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -run "TestGetOpenInterestParses|TestGetFundingRateHistoryParses" ./binance/sdk/perp`
Expected: FAIL with undefined `GetOpenInterest` / `GetFundingRateHistory`.

- [ ] **Step 3: Implement the SDK methods**

Append to `binance/sdk/perp/market.go` after the `GetAllFundingRates` function (currently ends around line 280):

```go
// OpenInterestResponse matches /fapi/v1/openInterest.
type OpenInterestResponse struct {
	Symbol       string `json:"symbol"`
	OpenInterest string `json:"openInterest"` // in base asset (contracts)
	Time         int64  `json:"time"`
}

// GetOpenInterest retrieves current open interest for a perp symbol.
// Docs: https://binance-docs.github.io/apidocs/futures/en/#open-interest
func (c *Client) GetOpenInterest(ctx context.Context, symbol string) (*OpenInterestResponse, error) {
	params := map[string]interface{}{"symbol": symbol}
	var res OpenInterestResponse
	if err := c.Get(ctx, "/fapi/v1/openInterest", params, false, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

// FundingRateHistoryEntry matches one element of /fapi/v1/fundingRate.
type FundingRateHistoryEntry struct {
	Symbol      string `json:"symbol"`
	FundingRate string `json:"fundingRate"`
	FundingTime int64  `json:"fundingTime"`
	MarkPrice   string `json:"markPrice"`
}

// GetFundingRateHistory retrieves historical funding rate entries for a symbol.
// startMillis/endMillis are optional; pass 0 to omit. limit <= 0 uses exchange default (100).
// Docs: https://binance-docs.github.io/apidocs/futures/en/#get-funding-rate-history
func (c *Client) GetFundingRateHistory(ctx context.Context, symbol string, startMillis, endMillis int64, limit int) ([]FundingRateHistoryEntry, error) {
	params := map[string]interface{}{"symbol": symbol}
	if startMillis > 0 {
		params["startTime"] = startMillis
	}
	if endMillis > 0 {
		params["endTime"] = endMillis
	}
	if limit > 0 {
		params["limit"] = limit
	}
	var res []FundingRateHistoryEntry
	if err := c.Get(ctx, "/fapi/v1/fundingRate", params, false, &res); err != nil {
		return nil, err
	}
	return res, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -run "TestGetOpenInterestParses|TestGetFundingRateHistoryParses" ./binance/sdk/perp`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add binance/sdk/perp/market.go binance/sdk/perp/market_test.go
git commit -m "Binance SDK: add GetOpenInterest and GetFundingRateHistory"
```

---

### Task 6: Wire Binance adapter methods

**Files:**
- Modify: `binance/funding.go`
- Modify: `binance/perp_adapter.go:463-494` (FetchTicker) and `:568-592` (FetchTrades region)

- [ ] **Step 1: Add `FetchFundingRateHistory` to the adapter**

Append to `binance/funding.go`:

```go
// FetchFundingRateHistory retrieves historical funding rates for a symbol.
// Rates are normalized to per-hour by Binance's GetFundingRate helper — the
// raw fundingRate endpoint emits per-period values, so we divide by the symbol's
// funding interval hours before returning.
func (a *Adapter) FetchFundingRateHistory(ctx context.Context, symbol string, opts *exchanges.FundingRateHistoryOpts) ([]exchanges.FundingRate, error) {
	formattedSymbol := a.FormatSymbol(symbol)

	var startMs, endMs int64
	var limit int
	if opts != nil {
		if opts.Start != nil {
			startMs = opts.Start.UnixMilli()
		}
		if opts.End != nil {
			endMs = opts.End.UnixMilli()
		}
		limit = opts.Limit
	}

	raw, err := a.client.GetFundingRateHistory(ctx, formattedSymbol, startMs, endMs, limit)
	if err != nil {
		return nil, err
	}

	intervalHours, err := a.client.GetFundingIntervalHours(ctx, formattedSymbol)
	if err != nil {
		return nil, err
	}
	if intervalHours <= 0 {
		intervalHours = 8
	}

	out := make([]exchanges.FundingRate, 0, len(raw))
	for _, r := range raw {
		rate := parseDecimal(r.FundingRate)
		if intervalHours != 1 {
			rate = rate.Div(decimal.NewFromInt(intervalHours))
		}
		out = append(out, exchanges.FundingRate{
			Symbol:               symbol,
			FundingRate:          rate,
			FundingIntervalHours: intervalHours,
			FundingTime:          r.FundingTime,
			UpdateTime:           time.Now().Unix(),
		})
	}
	return out, nil
}
```

Check that `binance/funding.go` imports `"github.com/shopspring/decimal"`. If not, add it.

Also check that `binance/sdk/perp/market.go` exposes a `GetFundingIntervalHours(ctx, symbol)` helper — search with `grep -n 'GetFundingIntervalHours\|FundingIntervalHours' binance/sdk/perp/market.go`. If it does NOT exist, add this helper near `GetFundingRate`:

```go
// GetFundingIntervalHours returns the hourly interval for a symbol's funding,
// derived from /fapi/v1/fundingInfo. Defaults to 8 when the symbol is absent.
func (c *Client) GetFundingIntervalHours(ctx context.Context, symbol string) (int64, error) {
	infos, err := c.GetFundingInfo(ctx)
	if err != nil {
		return 0, err
	}
	for _, fi := range infos {
		if fi.Symbol == symbol {
			return fi.FundingIntervalHours, nil
		}
	}
	return 8, nil
}
```

- [ ] **Step 2: Add `FetchOpenInterest` to the adapter**

Append to `binance/funding.go` (or create `binance/open_interest.go` if the team prefers one file per capability; the existing repo mixes styles, so either is acceptable):

```go
// FetchOpenInterest retrieves current open interest for a perp symbol.
// Binance returns OI only in contracts (base asset); notional requires a
// separate mark-price lookup and is left zero here — callers needing notional
// can multiply by FetchTicker().MarkPrice or use the analytics history endpoint.
func (a *Adapter) FetchOpenInterest(ctx context.Context, symbol string) (*exchanges.OpenInterest, error) {
	formattedSymbol := a.FormatSymbol(symbol)
	res, err := a.client.GetOpenInterest(ctx, formattedSymbol)
	if err != nil {
		return nil, err
	}
	return &exchanges.OpenInterest{
		Symbol:      symbol,
		OIContracts: parseDecimal(res.OpenInterest),
		Timestamp:   res.Time,
	}, nil
}
```

- [ ] **Step 3: Populate extended Ticker fields in `FetchTicker`**

In `binance/perp_adapter.go` (around line 476-484), replace the `exchanges.Ticker{...}` literal with:

```go
	ticker := &exchanges.Ticker{
		Symbol:             symbol,
		LastPrice:          parseDecimal(t.LastPrice),
		High24h:            parseDecimal(t.HighPrice),
		Low24h:             parseDecimal(t.LowPrice),
		Volume24h:          parseDecimal(t.Volume),
		QuoteVol:           parseDecimal(t.QuoteVolume),
		OpenPrice:          parseDecimal(t.OpenPrice),
		PriceChange:        parseDecimal(t.PriceChange),
		PriceChangePercent: parseDecimal(t.PriceChangePercent),
		WeightedAvgPrice:   parseDecimal(t.WeightedAvgPrice),
		TradeCount:         t.Count,
		Timestamp:          t.CloseTime,
	}
```

The `TickerResponse` already carries these fields (see `binance/sdk/perp/market.go:90-107`); no SDK change needed.

- [ ] **Step 4: Add `FetchHistoricalTrades` to the adapter**

Append to `binance/perp_adapter.go` (anywhere after `FetchTrades`, near line 592):

```go
// FetchHistoricalTrades returns paginated historical trades via aggTrades.
// Binance's aggTrades endpoint accepts symbol + (fromId | startTime/endTime) + limit.
// fromId takes precedence over the time range when set.
func (a *Adapter) FetchHistoricalTrades(ctx context.Context, symbol string, opts *exchanges.HistoricalTradeOpts) ([]exchanges.Trade, error) {
	formattedSymbol := a.FormatSymbol(symbol)

	params := map[string]interface{}{"symbol": formattedSymbol}
	limit := 500
	if opts != nil {
		if opts.FromID != "" {
			id, err := strconv.ParseInt(opts.FromID, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid FromID: %w", err)
			}
			params["fromId"] = id
		} else {
			if opts.Start != nil {
				params["startTime"] = opts.Start.UnixMilli()
			}
			if opts.End != nil {
				params["endTime"] = opts.End.UnixMilli()
			}
		}
		if opts.Limit > 0 {
			limit = opts.Limit
		}
	}
	params["limit"] = limit

	var raw []perp.AggTrade
	if err := a.client.Raw().Get(ctx, "/fapi/v1/aggTrades", params, false, &raw); err != nil {
		return nil, err
	}

	trades := make([]exchanges.Trade, 0, len(raw))
	for _, r := range raw {
		side := exchanges.TradeSideBuy
		if r.IsBuyerMaker {
			side = exchanges.TradeSideSell
		}
		trades = append(trades, exchanges.Trade{
			ID:        strconv.FormatInt(r.ID, 10),
			Symbol:    symbol,
			Price:     parseDecimal(r.Price),
			Quantity:  parseDecimal(r.Quantity),
			Side:      side,
			Timestamp: r.Timestamp,
		})
	}
	return trades, nil
}
```

`a.client.Raw()` may or may not exist — check with `grep -n 'func.*Raw' binance/sdk/perp/client.go`. If it does not, replace the raw call with reusing the existing `GetAggTrades` helper and extend it to accept options:

Alternative implementation using the existing helper: replace the body above with:

```go
	// Binance's aggTrades endpoint lets us page via fromId OR startTime/endTime.
	// The existing GetAggTrades only takes (symbol, limit); use a direct call here
	// to forward the full param set.
	raw, err := a.client.GetAggTradesPaged(ctx, perp.AggTradesQuery{
		Symbol:    formattedSymbol,
		FromID:    fromID,           // pointer, nil when unset
		StartTime: startMs,
		EndTime:   endMs,
		Limit:     limit,
	})
	if err != nil {
		return nil, err
	}
```

If you add `GetAggTradesPaged`, put it in `binance/sdk/perp/market.go` next to the existing `GetAggTrades`:

```go
// AggTradesQuery is the full parameter set for /fapi/v1/aggTrades.
type AggTradesQuery struct {
	Symbol    string
	FromID    *int64
	StartTime int64
	EndTime   int64
	Limit     int
}

func (c *Client) GetAggTradesPaged(ctx context.Context, q AggTradesQuery) ([]AggTrade, error) {
	params := map[string]interface{}{"symbol": q.Symbol}
	if q.FromID != nil {
		params["fromId"] = *q.FromID
	}
	if q.StartTime > 0 {
		params["startTime"] = q.StartTime
	}
	if q.EndTime > 0 {
		params["endTime"] = q.EndTime
	}
	if q.Limit > 0 {
		params["limit"] = q.Limit
	}
	var res []AggTrade
	if err := c.Get(ctx, "/fapi/v1/aggTrades", params, false, &res); err != nil {
		return nil, err
	}
	return res, nil
}
```

Pick one approach. The `GetAggTradesPaged` helper is cleaner; use that. Adapter code becomes:

```go
func (a *Adapter) FetchHistoricalTrades(ctx context.Context, symbol string, opts *exchanges.HistoricalTradeOpts) ([]exchanges.Trade, error) {
	formattedSymbol := a.FormatSymbol(symbol)

	q := perp.AggTradesQuery{Symbol: formattedSymbol, Limit: 500}
	if opts != nil {
		if opts.Limit > 0 {
			q.Limit = opts.Limit
		}
		if opts.FromID != "" {
			id, err := strconv.ParseInt(opts.FromID, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid FromID: %w", err)
			}
			q.FromID = &id
		} else {
			if opts.Start != nil {
				q.StartTime = opts.Start.UnixMilli()
			}
			if opts.End != nil {
				q.EndTime = opts.End.UnixMilli()
			}
		}
	}

	raw, err := a.client.GetAggTradesPaged(ctx, q)
	if err != nil {
		return nil, err
	}

	trades := make([]exchanges.Trade, 0, len(raw))
	for _, r := range raw {
		side := exchanges.TradeSideBuy
		if r.IsBuyerMaker {
			side = exchanges.TradeSideSell
		}
		trades = append(trades, exchanges.Trade{
			ID:        strconv.FormatInt(r.ID, 10),
			Symbol:    symbol,
			Price:     parseDecimal(r.Price),
			Quantity:  parseDecimal(r.Quantity),
			Side:      side,
			Timestamp: r.Timestamp,
		})
	}
	return trades, nil
}
```

- [ ] **Step 5: Run adapter-level compile check**

Run: `go build ./binance/...`
Expected: SUCCESS.

- [ ] **Step 6: Commit**

```bash
git add binance/funding.go binance/perp_adapter.go binance/sdk/perp/market.go
git commit -m "Binance perp adapter: implement FetchOpenInterest, FetchFundingRateHistory, FetchHistoricalTrades, extended Ticker fields"
```

---

### Task 7: Wire Binance live test

**Files:**
- Modify: `binance/adapter_test.go`

- [ ] **Step 1: Locate the existing test harness**

Run: `grep -n 'RunAdapterComplianceTests\|RunOrderSuite' binance/adapter_test.go`
Expected: finds calls like `testsuite.RunAdapterComplianceTests(t, adp, symbol)`.

- [ ] **Step 2: Add analytics suite wiring**

In `binance/adapter_test.go`, in the test function that currently calls `RunAdapterComplianceTests` for perp, add right after it:

```go
	testsuite.RunAnalyticsComplianceTests(t, adp.(exchanges.PerpExchange), symbol)
```

(If the local variable is already typed as `*Adapter`, the type assertion is safe — `*Adapter` implements `PerpExchange`. If the existing variable is already of interface type `exchanges.PerpExchange`, drop the assertion.)

- [ ] **Step 3: Run short compliance tests**

Run: `scripts/verify_exchange.sh binance`
Expected: new tests execute. They may skip if `BINANCE_PERP_TEST_SYMBOL` is unset in `.env`; verify the skip message is clean.

- [ ] **Step 4: Run full live tests if credentials available**

Run: `RUN_FULL=1 scripts/verify_exchange.sh binance` (only if `.env` has Binance keys)
Expected: `TestFetchOpenInterest`, `TestFetchFundingRateHistory`, `TestFetchHistoricalTrades`, `TestTickerExtendedStats` all PASS.

- [ ] **Step 5: Commit**

```bash
git add binance/adapter_test.go
git commit -m "Wire Binance perp to analytics compliance suite"
```

---

## Phase 4: OKX perp reference implementation

### Task 8: Add OKX SDK methods

**Files:**
- Modify: `okx/sdk/market.go`

- [ ] **Step 1: Write failing SDK test**

Append to `okx/sdk/market_test.go`:

```go
func TestGetOpenInterestParses(t *testing.T) {
	t.Parallel()

	payload := `{"code":"0","msg":"","data":[{"instId":"BTC-USDT-SWAP","instType":"SWAP","oi":"12345.6","oiCcy":"123.456","ts":"1700000000000"}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v5/public/open-interest", r.URL.Path)
		require.Equal(t, "BTC-USDT-SWAP", r.URL.Query().Get("instId"))
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	c := NewClient(ClientConfig{BaseURL: srv.URL})
	oi, err := c.GetOpenInterest(context.Background(), "BTC-USDT-SWAP")
	require.NoError(t, err)
	require.Equal(t, "BTC-USDT-SWAP", oi.InstId)
	require.Equal(t, "12345.6", oi.OI)
	require.Equal(t, "123.456", oi.OICcy)
}

func TestGetFundingRateHistoryParses(t *testing.T) {
	t.Parallel()

	payload := `{"code":"0","msg":"","data":[
		{"instId":"BTC-USDT-SWAP","fundingRate":"0.0001","realizedRate":"0.0001","fundingTime":"1700000000000","method":"current_period"},
		{"instId":"BTC-USDT-SWAP","fundingRate":"0.00012","realizedRate":"0.00012","fundingTime":"1700028800000","method":"current_period"}
	]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v5/public/funding-rate-history", r.URL.Path)
		require.Equal(t, "BTC-USDT-SWAP", r.URL.Query().Get("instId"))
		require.Equal(t, "5", r.URL.Query().Get("limit"))
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	c := NewClient(ClientConfig{BaseURL: srv.URL})
	hist, err := c.GetFundingRateHistory(context.Background(), "BTC-USDT-SWAP", 0, 0, 5)
	require.NoError(t, err)
	require.Len(t, hist, 2)
	require.Equal(t, "0.0001", hist[0].FundingRate)
	require.Equal(t, "1700028800000", hist[1].FundingTime)
}
```

Add imports as needed (`"context"`, `"net/http"`, `"net/http/httptest"`, `"testing"`, `"github.com/stretchr/testify/require"`).

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -run "TestGetOpenInterestParses|TestGetFundingRateHistoryParses" ./okx/sdk`
Expected: FAIL with undefined methods.

- [ ] **Step 3: Implement the SDK methods**

Append to `okx/sdk/market.go`:

```go
// OpenInterest matches one element of /api/v5/public/open-interest's data array.
type OpenInterest struct {
	InstType string `json:"instType"`
	InstId   string `json:"instId"`
	OI       string `json:"oi"`    // open interest in contracts
	OICcy    string `json:"oiCcy"` // OI in coin (base asset)
	OIUsd    string `json:"oiUsd,omitempty"`
	Ts       string `json:"ts"`
}

// GetOpenInterest returns the current open interest for a single instrument.
// Docs: https://www.okx.com/docs-v5/en/#public-data-rest-api-get-open-interest
func (c *Client) GetOpenInterest(ctx context.Context, instId string) (*OpenInterest, error) {
	// OKX requires instType; we derive it from the instId suffix.
	instType := "SWAP"
	path := fmt.Sprintf("/api/v5/public/open-interest?instType=%s&instId=%s", instType, instId)
	resp, err := Request[OpenInterest](c, ctx, MethodGet, path, nil, false)
	if err != nil {
		return nil, err
	}
	if len(resp) == 0 {
		return nil, fmt.Errorf("okx: empty open-interest response for %s", instId)
	}
	return &resp[0], nil
}

// FundingRateHistoryEntry matches one element of /api/v5/public/funding-rate-history.
type FundingRateHistoryEntry struct {
	InstType     string `json:"instType"`
	InstId       string `json:"instId"`
	FundingRate  string `json:"fundingRate"`
	RealizedRate string `json:"realizedRate"`
	FundingTime  string `json:"fundingTime"`
	Method       string `json:"method"`
}

// GetFundingRateHistory retrieves historical funding rates.
// beforeMillis / afterMillis are optional OKX timestamp cursors (pass 0 to omit).
// OKX `before` = newer than timestamp; `after` = older than timestamp.
// limit is capped at 100 by OKX; zero means exchange default.
// Docs: https://www.okx.com/docs-v5/en/#public-data-rest-api-get-funding-rate-history
func (c *Client) GetFundingRateHistory(ctx context.Context, instId string, beforeMillis, afterMillis int64, limit int) ([]FundingRateHistoryEntry, error) {
	u := url.Values{}
	u.Set("instId", instId)
	if beforeMillis > 0 {
		u.Set("before", strconv.FormatInt(beforeMillis, 10))
	}
	if afterMillis > 0 {
		u.Set("after", strconv.FormatInt(afterMillis, 10))
	}
	if limit > 0 {
		u.Set("limit", strconv.Itoa(limit))
	}
	path := "/api/v5/public/funding-rate-history?" + u.Encode()
	return Request[FundingRateHistoryEntry](c, ctx, MethodGet, path, nil, false)
}

// HistoryTrade matches one element of /api/v5/market/history-trades.
type HistoryTrade struct {
	InstId  string `json:"instId"`
	TradeId string `json:"tradeId"`
	Px      string `json:"px"`
	Sz      string `json:"sz"`
	Side    string `json:"side"`
	Ts      string `json:"ts"`
}

// GetHistoryTrades returns paginated historical public trades.
// `typ` is OKX's pagination mode: 1=by tradeId (before/after refer to tradeId),
// 2=by timestamp (before/after refer to ts). Pass 1 for id-cursor pagination.
// before/after are cursors; either may be empty.
// Docs: https://www.okx.com/docs-v5/en/#public-data-rest-api-get-trades-history
func (c *Client) GetHistoryTrades(ctx context.Context, instId string, typ int, before, after string, limit int) ([]HistoryTrade, error) {
	u := url.Values{}
	u.Set("instId", instId)
	if typ > 0 {
		u.Set("type", strconv.Itoa(typ))
	}
	if before != "" {
		u.Set("before", before)
	}
	if after != "" {
		u.Set("after", after)
	}
	if limit > 0 {
		u.Set("limit", strconv.Itoa(limit))
	}
	path := "/api/v5/market/history-trades?" + u.Encode()
	return Request[HistoryTrade](c, ctx, MethodGet, path, nil, false)
}
```

Ensure `okx/sdk/market.go` imports `"net/url"`, `"strconv"`, `"fmt"` (most likely already present; check and add missing ones).

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -run "TestGetOpenInterestParses|TestGetFundingRateHistoryParses" ./okx/sdk`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add okx/sdk/market.go okx/sdk/market_test.go
git commit -m "OKX SDK: add GetOpenInterest, GetFundingRateHistory, GetHistoryTrades"
```

---

### Task 9: Wire OKX adapter methods

**Files:**
- Modify: `okx/funding.go`
- Modify: `okx/perp_adapter.go` (FetchTicker region, add FetchOpenInterest + FetchHistoricalTrades)

- [ ] **Step 1: Add `FetchFundingRateHistory` and `FetchOpenInterest` to `okx/funding.go`**

Open `okx/funding.go`. Append:

```go
// FetchFundingRateHistory retrieves historical funding rates for a symbol.
// OKX emits per-period rates; convert to per-hour using the instrument's
// funding interval (same rule used by FetchFundingRate).
func (a *Adapter) FetchFundingRateHistory(ctx context.Context, symbol string, opts *exchanges.FundingRateHistoryOpts) ([]exchanges.FundingRate, error) {
	instId := a.FormatSymbol(symbol)

	var before, after int64
	var limit int
	if opts != nil {
		if opts.End != nil {
			before = opts.End.UnixMilli() // newer than
		}
		if opts.Start != nil {
			after = opts.Start.UnixMilli() // older than
		}
		limit = opts.Limit
	}

	raw, err := a.client.GetFundingRateHistory(ctx, instId, before, after, limit)
	if err != nil {
		return nil, err
	}

	intervalHours, err := a.fundingIntervalHours(ctx, instId)
	if err != nil {
		return nil, err
	}
	if intervalHours <= 0 {
		intervalHours = 8
	}

	out := make([]exchanges.FundingRate, 0, len(raw))
	for _, r := range raw {
		rate := parseString(r.FundingRate)
		if intervalHours != 1 {
			rate = rate.Div(decimal.NewFromInt(intervalHours))
		}
		ft, _ := strconv.ParseInt(r.FundingTime, 10, 64)
		out = append(out, exchanges.FundingRate{
			Symbol:               symbol,
			FundingRate:          rate,
			FundingIntervalHours: intervalHours,
			FundingTime:          ft,
			UpdateTime:           time.Now().Unix(),
		})
	}
	return out, nil
}

// FetchOpenInterest retrieves current open interest for a perp symbol.
func (a *Adapter) FetchOpenInterest(ctx context.Context, symbol string) (*exchanges.OpenInterest, error) {
	instId := a.FormatSymbol(symbol)
	res, err := a.client.GetOpenInterest(ctx, instId)
	if err != nil {
		return nil, err
	}
	ts, _ := strconv.ParseInt(res.Ts, 10, 64)
	return &exchanges.OpenInterest{
		Symbol:      symbol,
		OIContracts: parseString(res.OI),
		OINotional:  parseString(res.OIUsd),
		Timestamp:   ts,
	}, nil
}
```

Check the existing `okx/funding.go` for the funding-interval helper (likely `fundingIntervalHours` or similar). If it doesn't exist, promote the logic used by `FetchFundingRate` into a package-level helper:

```go
// fundingIntervalHours returns the funding interval (hours) for a given instId.
// Derived from the instrument metadata's fundingInterval field (unit: ms).
func (a *Adapter) fundingIntervalHours(ctx context.Context, instId string) (int64, error) {
	// Existing funding code already looks up interval; reuse its helper.
	// If no such helper is exported, mirror its implementation here.
	meta, err := a.client.GetFundingRate(ctx, instId)
	if err != nil {
		return 0, err
	}
	return meta.FundingIntervalHours, nil
}
```

Verify imports: add `"strconv"`, `"time"`, `"github.com/shopspring/decimal"` if missing.

- [ ] **Step 2: Populate extended Ticker fields in OKX's FetchTicker**

Find the OKX `FetchTicker` implementation: `grep -n 'func (a \*Adapter) FetchTicker' okx/perp_adapter.go`. Read the surrounding block.

Replace the existing Ticker construction so it sets (at minimum): `OpenPrice`, `PriceChange`, `PriceChangePercent`, `WeightedAvgPrice`, `TradeCount`. OKX's `/api/v5/market/ticker` response fields are: `last`, `lastSz`, `askPx`, `askSz`, `bidPx`, `bidSz`, `open24h`, `high24h`, `low24h`, `volCcy24h`, `vol24h`, `sodUtc0`, `sodUtc8`, `ts`. Price change / percent must be computed:

```go
	open := parseString(t.Open24h)
	last := parseString(t.Last)
	priceChange := decimal.Zero
	priceChangePct := decimal.Zero
	if open.IsPositive() {
		priceChange = last.Sub(open)
		priceChangePct = priceChange.Div(open).Mul(decimal.NewFromInt(100))
	}

	ticker := &exchanges.Ticker{
		Symbol:             symbol,
		LastPrice:          last,
		Bid:                parseString(t.BidPx),
		Ask:                parseString(t.AskPx),
		High24h:            parseString(t.High24h),
		Low24h:             parseString(t.Low24h),
		Volume24h:          parseString(t.Vol24h),
		QuoteVol:           parseString(t.VolCcy24h),
		OpenPrice:          open,
		PriceChange:        priceChange,
		PriceChangePercent: priceChangePct,
		// WeightedAvgPrice is not provided by OKX; leave zero.
		// TradeCount is not provided; leave zero.
		Timestamp:          parseTime(t.Ts),
	}
```

Keep existing logic for `MarkPrice`, `IndexPrice`, `MidPrice` intact.

- [ ] **Step 3: Add `FetchHistoricalTrades`**

Append to `okx/perp_adapter.go` (near the existing `FetchTrades` function, line ~940):

```go
// FetchHistoricalTrades returns paginated historical public trades.
// Uses OKX's /api/v5/market/history-trades endpoint in tradeId-cursor mode
// (type=1) when FromID is set; otherwise timestamp-cursor mode (type=2).
func (a *Adapter) FetchHistoricalTrades(ctx context.Context, symbol string, opts *exchanges.HistoricalTradeOpts) ([]exchanges.Trade, error) {
	instId := a.FormatSymbol(symbol)

	typ := 1
	var before, after string
	limit := 100
	if opts != nil {
		if opts.Limit > 0 {
			limit = opts.Limit
		}
		if opts.FromID != "" {
			typ = 1
			after = opts.FromID // older than this tradeId
		} else if opts.Start != nil || opts.End != nil {
			typ = 2
			if opts.End != nil {
				before = strconv.FormatInt(opts.End.UnixMilli(), 10)
			}
			if opts.Start != nil {
				after = strconv.FormatInt(opts.Start.UnixMilli(), 10)
			}
		}
	}

	raw, err := a.client.GetHistoryTrades(ctx, instId, typ, before, after, limit)
	if err != nil {
		return nil, err
	}

	out := make([]exchanges.Trade, 0, len(raw))
	for _, r := range raw {
		side := exchanges.TradeSideBuy
		if r.Side == "sell" {
			side = exchanges.TradeSideSell
		}
		out = append(out, exchanges.Trade{
			ID:        r.TradeId,
			Symbol:    symbol,
			Price:     parseString(r.Px),
			Quantity:  parseString(r.Sz),
			Side:      side,
			Timestamp: parseTime(r.Ts),
		})
	}
	return out, nil
}
```

- [ ] **Step 4: Run adapter-level compile check**

Run: `go build ./okx/...`
Expected: SUCCESS.

- [ ] **Step 5: Commit**

```bash
git add okx/funding.go okx/perp_adapter.go
git commit -m "OKX perp adapter: implement FetchOpenInterest, FetchFundingRateHistory, FetchHistoricalTrades, extended Ticker fields"
```

---

### Task 10: Wire OKX live test

**Files:**
- Modify: `okx/adapter_test.go`

- [ ] **Step 1: Locate existing wiring**

Run: `grep -n 'RunAdapterComplianceTests' okx/adapter_test.go`

- [ ] **Step 2: Add analytics suite call**

Right after the existing `testsuite.RunAdapterComplianceTests(t, adp, symbol)` call, add:

```go
	testsuite.RunAnalyticsComplianceTests(t, adp.(exchanges.PerpExchange), symbol)
```

- [ ] **Step 3: Run short compliance**

Run: `scripts/verify_exchange.sh okx`
Expected: tests pass or cleanly skip on missing creds.

- [ ] **Step 4: Run full live tests**

Run: `RUN_FULL=1 scripts/verify_exchange.sh okx` (needs `.env`)
Expected: all analytics sub-tests PASS.

- [ ] **Step 5: Commit**

```bash
git add okx/adapter_test.go
git commit -m "Wire OKX perp to analytics compliance suite"
```

---

## Phase 5: `ErrNotSupported` stubs for remaining perp adapters

The `PerpExchange` interface now requires `FetchOpenInterest` and `FetchFundingRateHistory`. Every perp adapter that doesn't have a real impl must provide a stub. The `FetchHistoricalTrades` default already comes from `BaseAdapter`, so no stubbing is needed for it.

### Task 11: Add stubs to Aster and Bitget

**Files:**
- Modify: `aster/perp_adapter.go`, `bitget/perp_adapter.go`

- [ ] **Step 1: Append stub to Aster**

Append at the end of `aster/perp_adapter.go`:

```go
// FetchOpenInterest is not yet implemented for Aster.
func (a *Adapter) FetchOpenInterest(ctx context.Context, symbol string) (*exchanges.OpenInterest, error) {
	_ = ctx
	_ = symbol
	return nil, exchanges.ErrNotSupported
}

// FetchFundingRateHistory is not yet implemented for Aster.
func (a *Adapter) FetchFundingRateHistory(ctx context.Context, symbol string, opts *exchanges.FundingRateHistoryOpts) ([]exchanges.FundingRate, error) {
	_ = ctx
	_ = symbol
	_ = opts
	return nil, exchanges.ErrNotSupported
}
```

- [ ] **Step 2: Append stub to Bitget**

Append at the end of `bitget/perp_adapter.go` (use the same code as above, with only package-local struct method receivers — `func (a *Adapter) ...`).

- [ ] **Step 3: Compile check**

Run: `go build ./aster/... ./bitget/...`
Expected: SUCCESS.

- [ ] **Step 4: Commit**

```bash
git add aster/perp_adapter.go bitget/perp_adapter.go
git commit -m "Aster + Bitget: ErrNotSupported stubs for new PerpExchange analytics methods"
```

---

### Task 12: Add stubs to Bybit and Nado

**Files:**
- Modify: `bybit/perp_adapter.go`, `nado/perp_adapter.go`

- [ ] **Step 1: Append the same two-method stub block to each file.**

The stub bodies are identical to Task 11. Use `(a *Adapter)` as the receiver in both files.

- [ ] **Step 2: Compile check**

Run: `go build ./bybit/... ./nado/...`

- [ ] **Step 3: Commit**

```bash
git add bybit/perp_adapter.go nado/perp_adapter.go
git commit -m "Bybit + Nado: ErrNotSupported stubs for new PerpExchange analytics methods"
```

---

### Task 13: Add stubs to Lighter, Hyperliquid, StandX

**Files:**
- Modify: `lighter/perp_adapter.go`, `hyperliquid/perp_adapter.go`, `standx/perp_adapter.go`

- [ ] **Step 1: Append the same two-method stub block to each file.**

- [ ] **Step 2: Compile check**

Run: `go build ./lighter/... ./hyperliquid/... ./standx/...`

- [ ] **Step 3: Commit**

```bash
git add lighter/perp_adapter.go hyperliquid/perp_adapter.go standx/perp_adapter.go
git commit -m "Lighter + Hyperliquid + StandX: ErrNotSupported stubs for new PerpExchange analytics methods"
```

---

### Task 14: Add stubs to GRVT, EdgeX, Decibel, Backpack

**Files:**
- Modify: `grvt/perp_adapter.go`, `edgex/perp_adapter.go`, `decibel/perp_adapter.go`, `backpack/perp_adapter.go`

- [ ] **Step 1: Append the same two-method stub block to each file.**

Note: GRVT and EdgeX are behind build tags. Make sure the stub is added inside the build-tag-guarded file (top of the file will have `//go:build grvt` / `//go:build edgex`).

- [ ] **Step 2: Compile check — default tags**

Run: `go build ./decibel/... ./backpack/...`
Expected: SUCCESS.

- [ ] **Step 3: Compile check — behind build tags**

Run: `go build -tags grvt,edgex ./grvt/... ./edgex/...`
Expected: SUCCESS.

- [ ] **Step 4: Full compile of entire repo**

Run: `go build -tags grvt,edgex ./...`
Expected: SUCCESS (everything compiles).

- [ ] **Step 5: Commit**

```bash
git add grvt/perp_adapter.go edgex/perp_adapter.go decibel/perp_adapter.go backpack/perp_adapter.go
git commit -m "GRVT + EdgeX + Decibel + Backpack: ErrNotSupported stubs for new PerpExchange analytics methods"
```

---

## Phase 6: Repo-wide verification

### Task 15: Run the standard verification gates

- [ ] **Step 1: Unit tests — default tags**

Run: `go test -short ./...`
Expected: SUCCESS. Any new sub-test that hits a non-Binance / non-OKX adapter must skip cleanly on `ErrNotSupported`.

- [ ] **Step 2: Unit tests — with build-tag adapters**

Run: `go test -short -tags grvt,edgex ./grvt/... ./edgex/...`
Expected: SUCCESS.

- [ ] **Step 3: Focused live verification for reference exchanges**

If `.env` has credentials:
- `scripts/verify_exchange.sh binance`
- `scripts/verify_exchange.sh okx`

Expected: SUCCESS for both; analytics sub-tests PASS.

- [ ] **Step 4: Commit any fallout**

If any test needed a touch-up during verification, commit with a descriptive message. Otherwise skip this step.

---

## Phase 7: Documentation

### Task 16: Update README and CLAUDE.md

**Files:**
- Modify: `README.md` — extend the API reference table.
- Modify: `CLAUDE.md` — mention analytics suite as an optional compliance tier.

- [ ] **Step 1: Extend README API Reference section**

In `README.md` find the `### PerpExchange Interface (extends Exchange)` table (around line 474) and add rows:

```markdown
| `FetchFundingRateHistory(ctx, symbol, opts)` | Historical funding rates (per-hour normalized) |
| `FetchOpenInterest(ctx, symbol)` | Current open interest |
```

In the core Exchange table (around line 441), under "Market Data (REST)", add:

```markdown
| | `FetchHistoricalTrades(ctx, symbol, opts)` | Paginated historical trades |
```

Also update the `Ticker` struct reference in the README (if it lists fields) to note the new 24h-change statistics.

- [ ] **Step 2: Extend CLAUDE.md**

In `CLAUDE.md` under the "Shared compliance suites" section, add a row to the capability table:

```markdown
| `analytics-capable` | + `RunAnalyticsComplianceTests` (requires `FetchOpenInterest`, `FetchFundingRateHistory` on PerpExchange) |
```

- [ ] **Step 3: Commit**

```bash
git add README.md CLAUDE.md
git commit -m "Document analytics-capable tier and new Exchange/PerpExchange methods"
```

---

## Self-Review (checklist, not execution)

- **Spec coverage**
  - P0 item 1 (extend `Ticker`): Task 1 + Task 6 step 3 (Binance) + Task 9 step 2 (OKX). ✓
  - P0 item 2 (`FetchOpenInterest`): Task 2 (model), Task 3 (interface), Task 5 (Binance SDK), Task 6 (Binance adapter), Task 8 (OKX SDK), Task 9 (OKX adapter), Tasks 11–14 (stubs). ✓
  - P0 item 3 (`FetchFundingRateHistory`): Task 2 (opts), Task 3 (interface), Task 5/8 (SDK), Task 6/9 (adapter), Tasks 11–14 (stubs). ✓
  - P0 item 4 (`FetchTrades` pagination): satisfied via new `FetchHistoricalTrades` method — design rationale captured in Architecture. Task 2 (opts), Task 3 (interface + BaseAdapter default), Task 6/9 (Binance + OKX impls). Every other adapter inherits the `ErrNotSupported` default from `BaseAdapter`. ✓

- **Placeholders**: none of "TBD", "implement later", "handle edge cases", or "similar to Task N" appear.

- **Type consistency**
  - `OpenInterest` fields: `OIContracts`, `OINotional`, `Timestamp`, `Symbol` — consistent across Tasks 2, 5, 6, 8, 9, 11–14.
  - `FundingRateHistoryOpts` fields: `Start`, `End`, `Limit` — consistent.
  - `HistoricalTradeOpts` fields: `Start`, `End`, `FromID`, `Limit` — consistent.
  - `Ticker` extension fields: `PriceChange`, `PriceChangePercent`, `OpenPrice`, `WeightedAvgPrice`, `TradeCount` — consistent across Task 1, Binance/OKX adapter impls, and the compliance test.
  - SDK method names: `GetOpenInterest`, `GetFundingRateHistory` in both `binance/sdk/perp` and `okx/sdk`; `GetHistoryTrades` (OKX) and `GetAggTradesPaged` (Binance) — consistent per-exchange.
  - Interface methods: `FetchOpenInterest`, `FetchFundingRateHistory`, `FetchHistoricalTrades` — consistent in all adapter stubs and real impls.
