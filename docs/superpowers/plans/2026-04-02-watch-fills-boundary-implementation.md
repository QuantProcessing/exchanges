# Watch Fills Boundary Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a dedicated private execution stream (`WatchFills`) and split unified order semantics so `WatchOrders` remains lifecycle-focused while fills carry execution-level detail.

**Architecture:** Extend the shared stream/model contracts first, preserving backward compatibility for existing `Order.Price` callers while introducing explicit order-price and fill-price fields plus a new `Fill` model. Then wire `WatchFills` through adapters that already expose native fill subscriptions, and make all remaining adapters compile cleanly by returning `ErrNotSupported` with explicit tests and docs.

**Tech Stack:** Go, repository root interfaces in `exchange.go`/`models.go`, per-exchange adapters, `stretchr/testify`, existing repository testsuite helpers.

---

### Task 1: Add Shared Fill Model And Streaming Contracts

**Files:**
- Modify: `exchange.go`
- Modify: `models.go`
- Modify: `convenience_order_test.go`
- Create: `streaming_contract_test.go`

- [ ] **Step 1: Write failing contract tests for the new shared types**

Create `streaming_contract_test.go` with:

```go
package exchanges_test

import (
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestOrderSupportsExplicitOrderAndFillPrices(t *testing.T) {
	order := exchanges.Order{
		OrderID:          "1",
		Price:            decimal.RequireFromString("100"),
		OrderPrice:       decimal.RequireFromString("100"),
		AverageFillPrice: decimal.RequireFromString("101"),
		LastFillPrice:    decimal.RequireFromString("102"),
		LastFillQuantity: decimal.RequireFromString("0.5"),
	}

	require.Equal(t, "100", order.OrderPrice.String())
	require.Equal(t, "101", order.AverageFillPrice.String())
	require.Equal(t, "102", order.LastFillPrice.String())
	require.Equal(t, "0.5", order.LastFillQuantity.String())
}

func TestFillCarriesExecutionDetails(t *testing.T) {
	fill := exchanges.Fill{
		TradeID:       "t-1",
		OrderID:       "o-1",
		ClientOrderID: "c-1",
		Symbol:        "BTC",
		Side:          exchanges.OrderSideBuy,
		Price:         decimal.RequireFromString("101"),
		Quantity:      decimal.RequireFromString("0.25"),
		Fee:           decimal.RequireFromString("0.01"),
		FeeAsset:      "USDT",
		IsMaker:       true,
		Timestamp:     123,
	}

	require.Equal(t, "t-1", fill.TradeID)
	require.True(t, fill.IsMaker)
}
```

- [ ] **Step 2: Run the new tests to verify they fail on missing fields/types**

Run: `env GOCACHE=/tmp/go-build go test . -run 'Test(OrderSupportsExplicitOrderAndFillPrices|FillCarriesExecutionDetails)$' -count=1`
Expected: compile failure because `OrderPrice`, `AverageFillPrice`, `LastFillPrice`, `LastFillQuantity`, and/or `Fill` do not exist yet.

- [ ] **Step 3: Add the shared fill model and stream callbacks**

Update `models.go`:

```go
type Order struct {
	OrderID          string          `json:"order_id"`
	Symbol           string          `json:"symbol"`
	Side             OrderSide       `json:"side"`
	Type             OrderType       `json:"type"`
	Quantity         decimal.Decimal `json:"quantity"`
	Price            decimal.Decimal `json:"price,omitempty"` // Deprecated: adapter-defined legacy price
	OrderPrice       decimal.Decimal `json:"order_price,omitempty"`
	AverageFillPrice decimal.Decimal `json:"average_fill_price,omitempty"`
	LastFillPrice    decimal.Decimal `json:"last_fill_price,omitempty"`
	FilledQuantity   decimal.Decimal `json:"filled_quantity"`
	LastFillQuantity decimal.Decimal `json:"last_fill_quantity,omitempty"`
	Timestamp        int64           `json:"timestamp"`
	Fee              decimal.Decimal `json:"fee,omitempty"`
	ClientOrderID    string          `json:"client_order_id,omitempty"`
	ReduceOnly       bool            `json:"reduce_only,omitempty"`
	TimeInForce      TimeInForce     `json:"time_in_force,omitempty"`
}

type Fill struct {
	TradeID       string          `json:"trade_id"`
	OrderID       string          `json:"order_id"`
	ClientOrderID string          `json:"client_order_id,omitempty"`
	Symbol        string          `json:"symbol"`
	Side          OrderSide       `json:"side"`
	Price         decimal.Decimal `json:"price"`
	Quantity      decimal.Decimal `json:"quantity"`
	Fee           decimal.Decimal `json:"fee,omitempty"`
	FeeAsset      string          `json:"fee_asset,omitempty"`
	IsMaker       bool            `json:"is_maker,omitempty"`
	Timestamp     int64           `json:"timestamp"`
}
```

Update `exchange.go`:

```go
type Streamable interface {
	WatchOrders(ctx context.Context, cb OrderUpdateCallback) error
	WatchFills(ctx context.Context, cb FillCallback) error
	WatchPositions(ctx context.Context, cb PositionUpdateCallback) error
	WatchTicker(ctx context.Context, symbol string, cb TickerCallback) error
	WatchTrades(ctx context.Context, symbol string, cb TradeCallback) error
	WatchKlines(ctx context.Context, symbol string, interval Interval, cb KlineCallback) error
	StopWatchOrders(ctx context.Context) error
	StopWatchFills(ctx context.Context) error
	StopWatchPositions(ctx context.Context) error
	StopWatchTicker(ctx context.Context, symbol string) error
	StopWatchTrades(ctx context.Context, symbol string) error
	StopWatchKlines(ctx context.Context, symbol string, interval Interval) error
}

type FillCallback func(*Fill)
```

Update the stub in `convenience_order_test.go`:

```go
func (s *stubExchange) WatchFills(context.Context, exchanges.FillCallback) error { return nil }
func (s *stubExchange) StopWatchFills(context.Context) error { return nil }
```

- [ ] **Step 4: Run the targeted root tests to verify the shared contract passes**

Run: `env GOCACHE=/tmp/go-build go test . -run 'Test(OrderSupportsExplicitOrderAndFillPrices|FillCarriesExecutionDetails)$' -count=1`
Expected: PASS

### Task 2: Add Repository-Level Unsupported Defaults And Tests

**Files:**
- Modify: `binance/unsupported_test.go`
- Modify: `okx/unsupported_test.go`
- Modify: `hyperliquid/unsupported_test.go`
- Modify: `lighter/unsupported_test.go`
- Modify: `standx/unsupported_test.go`
- Modify: `decibel/unsupported_test.go`
- Modify: `nado/unsupported_test.go`
- Modify: `backpack/registry_test.go`
- Modify: adapter files that currently implement `WatchOrders` but not `WatchFills`

- [ ] **Step 1: Add failing unsupported-method assertions**

Extend one representative unsupported test file, for example `binance/unsupported_test.go`:

```go
require.ErrorIs(t, spot.WatchFills(context.Background(), nil), exchanges.ErrNotSupported)
require.ErrorIs(t, spot.StopWatchFills(context.Background()), exchanges.ErrNotSupported)
require.ErrorIs(t, margin.WatchFills(context.Background(), nil), exchanges.ErrNotSupported)
require.ErrorIs(t, margin.StopWatchFills(context.Background()), exchanges.ErrNotSupported)
```

Add analogous assertions in the other existing unsupported suites.

- [ ] **Step 2: Run a narrow unsupported test command and confirm compile or runtime failures**

Run: `env GOCACHE=/tmp/go-build go test ./binance ./okx ./hyperliquid ./lighter ./standx ./decibel ./nado ./backpack -run 'Test.*Unsupported.*|TestUnsupportedMethodsReturnErrNotSupported' -count=1`
Expected: compile failures or missing method errors for `WatchFills` / `StopWatchFills`.

- [ ] **Step 3: Add minimal adapter implementations that return `ErrNotSupported`**

For each adapter package without native fill support in this first pass, add:

```go
func (a *Adapter) WatchFills(ctx context.Context, cb exchanges.FillCallback) error {
	return exchanges.ErrNotSupported
}

func (a *Adapter) StopWatchFills(ctx context.Context) error {
	return exchanges.ErrNotSupported
}
```

Use the package’s existing receiver names and place the methods next to the other unsupported stream methods.

- [ ] **Step 4: Re-run the unsupported command**

Run: `env GOCACHE=/tmp/go-build go test ./binance ./okx ./hyperliquid ./lighter ./standx ./decibel ./nado ./backpack -run 'Test.*Unsupported.*|TestUnsupportedMethodsReturnErrNotSupported' -count=1`
Expected: PASS

### Task 3: Normalize Order Mapping With Explicit Price Fields

**Files:**
- Modify: `binance/perp_adapter.go`
- Modify: `binance/spot_adapter.go`
- Modify: `aster/perp_adapter.go`
- Modify: `aster/spot_adapter.go`
- Modify: `okx/perp_adapter.go`
- Modify: `okx/spot_adapter.go`
- Modify: `backpack/private_mapping.go`
- Modify: `bitget/private_classic.go`
- Modify: `bitget/order_request.go`
- Modify: `decibel/perp_adapter.go`
- Modify: `edgex/perp_adapter.go`
- Modify: `grvt/perp_adapter.go`
- Modify: `hyperliquid/perp_adapter.go`
- Modify: `hyperliquid/spot_adapter.go`
- Modify: `lighter/perp_adapter.go`
- Modify: `lighter/spot_adapter.go`
- Modify: `nado/perp_adapter.go`
- Modify: `nado/spot_adapter.go`
- Modify: `standx/perp_adapter.go`
- Create: `order_price_mapping_test.go`

- [ ] **Step 1: Add a root test that exercises the new order fields without changing legacy `Price` semantics**

Create `order_price_mapping_test.go` with:

```go
package exchanges_test

import (
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestExplicitOrderPriceFieldsCanCoexistWithLegacyPrice(t *testing.T) {
	order := exchanges.Order{
		Price:            decimal.RequireFromString("100"),
		OrderPrice:       decimal.RequireFromString("100"),
		AverageFillPrice: decimal.RequireFromString("101"),
		LastFillPrice:    decimal.RequireFromString("102"),
	}

	require.Equal(t, order.Price, order.OrderPrice)
	require.Equal(t, "101", order.AverageFillPrice.String())
	require.Equal(t, "102", order.LastFillPrice.String())
}
```

- [ ] **Step 2: Run the root price-field test**

Run: `env GOCACHE=/tmp/go-build go test . -run TestExplicitOrderPriceFieldsCanCoexistWithLegacyPrice -count=1`
Expected: PASS after Task 1, serving as a safety test before adapter edits.

- [ ] **Step 3: Update adapter mappings so new fields are populated where data is already available**

Use this rule:

```go
order.Price = legacyValue // preserve previous behavior
order.OrderPrice = orderPriceIfKnown
order.AverageFillPrice = avgFillPriceIfKnown
order.LastFillPrice = lastFillPriceIfKnown
order.LastFillQuantity = lastFillQtyIfKnown
```

Concrete examples:

- Binance/Aster: `OrderPrice <- p`, `AverageFillPrice <- ap`, `LastFillPrice <- L`, `LastFillQuantity <- l`
- OKX: `OrderPrice <- px`, `AverageFillPrice <- avgPx`, `LastFillPrice <- fillPx`, `LastFillQuantity <- fillSz`
- Hyperliquid/Lighter/StandX/Decibel/EdgeX/Nado current order streams: set `OrderPrice` when available; leave fill-price fields zero if unavailable
- GRVT order stream: preserve current legacy `Price` behavior, but add explicit `AverageFillPrice`; set `OrderPrice` only if the source order carries a native submitted price

- [ ] **Step 4: Run a compile-focused adapter command**

Run: `env GOCACHE=/tmp/go-build go test ./aster ./backpack ./binance ./bitget ./decibel ./edgex ./grvt ./hyperliquid ./lighter ./nado ./okx ./standx -run '^$' -count=1`
Expected: PASS

### Task 4: Implement First-Class WatchFills For Adapters With Existing Native Support

**Files:**
- Modify: `grvt/perp_adapter.go`
- Modify: `hyperliquid/perp_adapter.go`
- Modify: `hyperliquid/spot_adapter.go`
- Modify: `nado/perp_adapter.go`
- Modify: `nado/spot_adapter.go`
- Create: `grvt/fills_test.go`
- Create: `hyperliquid/fills_test.go`
- Create: `nado/fills_test.go`

- [ ] **Step 1: Add failing adapter tests around native fill mapping**

Representative test shape:

```go
func TestMapGrvtFillUsesExecutionPrice(t *testing.T) {
	// construct sdk fill
	// call mapGrvtFill
	// assert Fill.Price is the execution price and Fill.Quantity is the execution size
}
```

For Hyperliquid and Nado, write equivalent tests around their existing user-fill SDK payload types.

- [ ] **Step 2: Run the targeted fill tests and confirm failures**

Run: `env GOCACHE=/tmp/go-build go test ./grvt ./hyperliquid ./nado -run 'Test(MapGrvtFillUsesExecutionPrice|MapHyperliquidFillUsesExecutionPrice|MapNadoFillUsesExecutionPrice)' -count=1`
Expected: compile failure because `Fill` mapping helpers or `WatchFills` methods do not exist yet.

- [ ] **Step 3: Implement native fill streaming for the adapters that already expose it**

Wire:

- `grvt`: use `wsAccount.SubscribeFill("all", ...)` and convert `WsFill` into `*exchanges.Fill`
- `hyperliquid` perp/spot: subscribe to existing `userFills` account stream and map each fill to `*exchanges.Fill`
- `nado` perp/spot: use `WsAccountClient.SubscribeFills(...)` and map execution `price`/`filled_qty` into `*exchanges.Fill`

Implementation shape:

```go
func (a *Adapter) WatchFills(ctx context.Context, cb exchanges.FillCallback) error {
	// ensure ws/account connection
	// subscribe to native fill stream
	// callback(mapFill(...))
}

func (a *Adapter) StopWatchFills(ctx context.Context) error {
	// if the SDK has no explicit unsubscribe, return nil when the connection is shared
	return nil
}
```

- [ ] **Step 4: Run the targeted fill tests**

Run: `env GOCACHE=/tmp/go-build go test ./grvt ./hyperliquid ./nado -run 'Test(MapGrvtFillUsesExecutionPrice|MapHyperliquidFillUsesExecutionPrice|MapNadoFillUsesExecutionPrice)' -count=1`
Expected: PASS

### Task 5: Update Docs, Helpers, And End-To-End Verification

**Files:**
- Modify: `README.md`
- Modify: `README_CN.md`
- Modify: `testsuite/helpers.go`
- Modify: `testsuite/lifecycle_suite.go`
- Modify: `testsuite/order_suite.go`

- [ ] **Step 1: Add docs/examples that explain when to use orders vs fills**

Document:

```md
- Use `WatchOrders` for lifecycle state.
- Use `WatchFills` for per-execution price, fee, and maker/taker details.
- Most strategies only need `WatchOrders`; execution analytics should subscribe to both.
```

- [ ] **Step 2: Add testsuite logging support for fills without making fills mandatory**

Update helpers and lifecycle logs so any future fill-aware suite code can print fills, but do not require all adapters to support them.

- [ ] **Step 3: Run focused repository verification**

Run:

`env GOCACHE=/tmp/go-build go test . ./grvt ./hyperliquid ./nado ./binance ./okx ./aster ./backpack ./bitget ./decibel ./edgex ./lighter ./standx -run 'Test(OrderSupportsExplicitOrderAndFillPrices|FillCarriesExecutionDetails|ExplicitOrderPriceFieldsCanCoexistWithLegacyPrice|MapGrvtFillUsesExecutionPrice|MapHyperliquidFillUsesExecutionPrice|MapNadoFillUsesExecutionPrice|.*Unsupported.*|TestUnsupportedMethodsReturnErrNotSupported)' -count=1`

Expected: PASS

- [ ] **Step 4: Run full compile verification across the repository**

Run: `env GOCACHE=/tmp/go-build go test ./... -run '^$' -count=1`
Expected: PASS
