# Backpack Exchange Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Backpack spot and perp adapters with public market data, authenticated account/order flows, registry wiring, and shared `testsuite` validation.

**Architecture:** Implement Backpack with the repository's standard two-layer design: a low-level `backpack/sdk` package for REST, WebSocket, signing, and native payloads; and a `backpack` adapter package that maps Backpack behavior onto the shared `exchanges` interfaces. Build deterministic pieces first with unit tests, then layer public adapter behavior, then private trading and streaming behavior, and finally wire live integration tests and environment variables.

**Tech Stack:** Go, `shopspring/decimal`, existing repository adapter patterns, Backpack REST/WS API with ED25519 signatures.

---

### Task 1: Scaffold Backpack package and deterministic signing tests

**Files:**
- Create: `backpack/options.go`
- Create: `backpack/options_test.go`
- Create: `backpack/register.go`
- Create: `backpack/sdk/client.go`
- Create: `backpack/sdk/signer.go`
- Create: `backpack/sdk/types.go`
- Create: `backpack/sdk/signer_test.go`

- [ ] **Step 1: Write the failing signing and options tests**

```go
func TestSignerBuildsCanonicalString(t *testing.T) {
	params := map[string]string{
		"symbol":    "BTC_USDC",
		"orderType": "Limit",
		"price":     "100",
	}

	got := buildSigningPayload("orderExecute", params, 123456, 5000)

	require.Equal(t,
		"instruction=orderExecute&orderType=Limit&price=100&symbol=BTC_USDC&timestamp=123456&window=5000",
		got,
	)
}
```

```go
func TestOptionsQuoteCurrencyDefaultsToUSDC(t *testing.T) {
	q, err := (Options{}).quoteCurrency()
	require.NoError(t, err)
	require.Equal(t, exchanges.QuoteCurrencyUSDC, q)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./backpack/sdk -run TestSignerBuildsCanonicalString`
Expected: FAIL because signer helpers do not exist yet.

Run: `go test ./backpack -run TestOptionsQuoteCurrencyDefaultsToUSDC`
Expected: FAIL because Backpack options do not exist yet.

- [ ] **Step 3: Write minimal implementation**

```go
type Options struct {
	APIKey        string
	PrivateKey    string
	QuoteCurrency exchanges.QuoteCurrency
	Logger        exchanges.Logger
}

func buildSigningPayload(instruction string, params map[string]string, ts, window int64) string {
	// Sort keys, prepend instruction, append timestamp/window.
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./backpack/sdk -run TestSignerBuildsCanonicalString`
Expected: PASS

Run: `go test ./backpack -run TestOptionsQuoteCurrencyDefaultsToUSDC`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backpack/options.go backpack/options_test.go backpack/register.go backpack/sdk/client.go backpack/sdk/signer.go backpack/sdk/types.go backpack/sdk/signer_test.go
git commit -m "feat: scaffold backpack sdk signing"
```

### Task 2: Add market metadata discovery and symbol mapping

**Files:**
- Create: `backpack/common.go`
- Create: `backpack/common_test.go`
- Modify: `backpack/sdk/client.go`
- Modify: `backpack/sdk/types.go`

- [ ] **Step 1: Write the failing metadata and symbol-mapping tests**

```go
func TestBuildMarketMapsSeparatesSpotAndPerp(t *testing.T) {
	markets := []sdk.Market{
		{Symbol: "BTC_USDC", BaseSymbol: "BTC", QuoteSymbol: "USDC", MarketType: "SPOT"},
		{Symbol: "BTC_USDC_PERP", BaseSymbol: "BTC", QuoteSymbol: "USDC", MarketType: "PERP"},
	}

	cache := newMarketCache(markets, exchanges.QuoteCurrencyUSDC)

	require.Equal(t, "BTC_USDC", cache.spotByBase["BTC"].Symbol)
	require.Equal(t, "BTC_USDC_PERP", cache.perpByBase["BTC"].Symbol)
}

func TestSymbolDetailsFromMarketUsesTickAndStepPrecision(t *testing.T) {
	m := sdk.Market{
		Symbol: "BTC_USDC",
		Filters: sdk.MarketFilters{
			Price:    sdk.PriceFilter{TickSize: "0.10"},
			Quantity: sdk.QuantityFilter{StepSize: "0.001", MinQuantity: "0.001"},
		},
	}

	details, err := symbolDetailsFromMarket(m)
	require.NoError(t, err)
	require.Equal(t, 1, details.PricePrecision)
	require.Equal(t, 3, details.QuantityPrecision)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./backpack -run 'TestBuildMarketMaps|TestSymbolDetails'`
Expected: FAIL because cache and conversion helpers do not exist.

- [ ] **Step 3: Write minimal implementation**

```go
type marketCache struct {
	spotByBase map[string]sdk.Market
	perpByBase map[string]sdk.Market
	bySymbol   map[string]sdk.Market
}

func symbolDetailsFromMarket(m sdk.Market) (*exchanges.SymbolDetails, error) {
	// Derive MinQuantity, MinNotional, PricePrecision, QuantityPrecision from filters.
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./backpack -run 'TestBuildMarketMaps|TestSymbolDetails'`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backpack/common.go backpack/common_test.go backpack/sdk/client.go backpack/sdk/types.go
git commit -m "feat: add backpack market metadata mapping"
```

### Task 3: Implement public REST client and public adapter methods

**Files:**
- Create: `backpack/perp_adapter.go`
- Create: `backpack/spot_adapter.go`
- Create: `backpack/public_adapter_test.go`
- Modify: `backpack/sdk/client.go`
- Create: `backpack/sdk/public_rest.go`
- Modify: `backpack/sdk/types.go`

- [ ] **Step 1: Write the failing public adapter tests**

```go
func TestFormatSymbolUsesMarketCache(t *testing.T) {
	adp := &Adapter{markets: marketCache{
		perpByBase: map[string]sdk.Market{"BTC": {Symbol: "BTC_USDC_PERP"}},
	}}

	require.Equal(t, "BTC_USDC_PERP", adp.FormatSymbol("BTC"))
}

func TestExtractSymbolReturnsBaseSymbol(t *testing.T) {
	require.Equal(t, "BTC", extractBaseSymbol("BTC_USDC_PERP"))
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./backpack -run 'TestFormatSymbol|TestExtractSymbol'`
Expected: FAIL because adapters and symbol helpers are incomplete.

- [ ] **Step 3: Write minimal implementation**

```go
func (a *Adapter) FetchTicker(ctx context.Context, symbol string) (*exchanges.Ticker, error) {
	market := a.lookupMarket(symbol)
	raw, err := a.client.GetTicker(ctx, market.Symbol)
	if err != nil {
		return nil, err
	}
	return toTicker(symbol, raw), nil
}
```

Implement at minimum:

- direct constructors `NewAdapter` and `NewSpotAdapter`
- `FormatSymbol`, `ExtractSymbol`, `ListSymbols`
- `FetchTicker`
- `FetchOrderBook`
- `FetchTrades`
- `FetchKlines`
- `FetchSymbolDetails`
- `FetchFeeRate`

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./backpack -run 'TestFormatSymbol|TestExtractSymbol'`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backpack/perp_adapter.go backpack/spot_adapter.go backpack/public_adapter_test.go backpack/sdk/public_rest.go backpack/sdk/client.go backpack/sdk/types.go
git commit -m "feat: add backpack public adapter methods"
```

### Task 4: Implement authenticated REST account and order flows

**Files:**
- Create: `backpack/private_adapter_test.go`
- Modify: `backpack/perp_adapter.go`
- Modify: `backpack/spot_adapter.go`
- Create: `backpack/sdk/private_rest.go`
- Modify: `backpack/sdk/client.go`
- Modify: `backpack/sdk/types.go`

- [ ] **Step 1: Write the failing deterministic mapping tests**

```go
func TestMapOrderStatusFilled(t *testing.T) {
	raw := sdk.Order{ID: "1", Status: "Filled", ExecutedQuantity: "2"}
	got := mapOrder(raw, "BTC")
	require.Equal(t, exchanges.OrderStatusFilled, got.Status)
	require.True(t, got.FilledQuantity.Equal(decimal.NewFromInt(2)))
}

func TestMapAccountIncludesPerpPositions(t *testing.T) {
	raw := sdk.AccountResponse{
		Balances: []sdk.Balance{{Symbol: "USDC", AvailableQuantity: "10"}},
		Positions: []sdk.Position{{Symbol: "BTC_USDC_PERP", Quantity: "1", Side: "Long"}},
	}
	acc := mapAccount(raw, exchanges.MarketTypePerp)
	require.Len(t, acc.Positions, 1)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./backpack -run 'TestMapOrderStatus|TestMapAccountIncludesPerpPositions'`
Expected: FAIL because private mappings do not exist.

- [ ] **Step 3: Write minimal implementation**

```go
func (a *Adapter) PlaceOrder(ctx context.Context, params *exchanges.OrderParams) (*exchanges.Order, error) {
	req := toPlaceOrderRequest(a.lookupMarket(params.Symbol), params)
	raw, err := a.client.PlaceOrder(ctx, req)
	if err != nil {
		return nil, err
	}
	return mapOrder(raw, params.Symbol), nil
}
```

Implement at minimum:

- `FetchAccount`
- `FetchBalance`
- `FetchOpenOrders`
- `FetchOrder`
- `PlaceOrder`
- `CancelOrder`
- `CancelAllOrders`
- perp `FetchPositions`
- perp account mapping that fills `Account.Positions`
- spot `FetchSpotBalances`

For unsupported shared methods, return explicit `exchanges.ErrNotSupported` wrappers instead of stubs with silent zero values.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./backpack -run 'TestMapOrderStatus|TestMapAccountIncludesPerpPositions'`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backpack/private_adapter_test.go backpack/perp_adapter.go backpack/spot_adapter.go backpack/sdk/private_rest.go backpack/sdk/client.go backpack/sdk/types.go
git commit -m "feat: add backpack private trading flows"
```

### Task 5: Implement perp funding methods

**Files:**
- Create: `backpack/funding.go`
- Create: `backpack/funding_test.go`
- Modify: `backpack/perp_adapter.go`
- Modify: `backpack/sdk/public_rest.go`
- Modify: `backpack/sdk/types.go`

- [ ] **Step 1: Write the failing funding tests**

```go
func TestMapFundingRate(t *testing.T) {
	raw := sdk.FundingRate{
		Symbol:        "BTC_USDC_PERP",
		EstimatedRate: "0.0001",
		NextFundingAt: 1710000000000,
		MarkPrice:     "50000",
		IndexPrice:    "49990",
	}

	got := mapFundingRate(raw)
	require.Equal(t, "BTC", got.Symbol)
	require.True(t, got.Rate.Equal(decimal.RequireFromString("0.0001")))
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./backpack -run TestMapFundingRate`
Expected: FAIL because funding mapping and methods do not exist.

- [ ] **Step 3: Write minimal implementation**

```go
func (a *Adapter) FetchFundingRate(ctx context.Context, symbol string) (*exchanges.FundingRate, error) {
	raw, err := a.client.GetFundingIntervalRate(ctx, a.lookupMarket(symbol).Symbol)
	if err != nil {
		return nil, err
	}
	return mapFundingRate(raw), nil
}
```

Implement:

- `FetchFundingRate`
- `FetchAllFundingRates`
- Backpack funding-rate response mapping in `backpack/funding.go`

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./backpack -run TestMapFundingRate`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backpack/funding.go backpack/funding_test.go backpack/perp_adapter.go backpack/sdk/public_rest.go backpack/sdk/types.go
git commit -m "feat: add backpack funding support"
```

### Task 6: Implement public WebSocket streams and orderbook synchronization

**Files:**
- Create: `backpack/orderbook_test.go`
- Create: `backpack/orderbook.go`
- Create: `backpack/sdk/ws.go`
- Modify: `backpack/sdk/types.go`
- Modify: `backpack/perp_adapter.go`
- Modify: `backpack/spot_adapter.go`

- [ ] **Step 1: Write the failing orderbook tests**

```go
func TestApplyDepthUpdateMaintainsSortedBook(t *testing.T) {
	book := newLocalBook("BTC")
	book.applySnapshot(
		[]level{{Price: "100", Quantity: "1"}},
		[]level{{Price: "101", Quantity: "1"}},
	)
	book.applyDelta(
		[]level{{Price: "99", Quantity: "2"}},
		[]level{{Price: "102", Quantity: "2"}},
	)

	ob := book.snapshot(10)
	require.Equal(t, "100", ob.Bids[0].Price.String())
	require.Equal(t, "101", ob.Asks[0].Price.String())
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./backpack -run TestApplyDepthUpdateMaintainsSortedBook`
Expected: FAIL because orderbook code does not exist.

- [ ] **Step 3: Write minimal implementation**

```go
func (a *Adapter) WatchOrderBook(ctx context.Context, symbol string, cb exchanges.OrderBookCallback) error {
	// Subscribe WS, bootstrap initial snapshot if needed, apply deltas, publish callback.
}
```

Implement at minimum:

- `WatchOrderBook`
- `GetLocalOrderBook`
- `StopWatchOrderBook`
- `WatchTicker`
- `WatchTrades`
- `WatchKlines`
- `StopWatchTicker`
- `StopWatchTrades`
- `StopWatchKlines`

If spot and perp depth payloads diverge materially, split `backpack/orderbook.go` into `backpack/spot_orderbook.go` and `backpack/perp_orderbook.go` before proceeding.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./backpack -run TestApplyDepthUpdateMaintainsSortedBook`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backpack/orderbook_test.go backpack/orderbook.go backpack/sdk/ws.go backpack/sdk/types.go backpack/perp_adapter.go backpack/spot_adapter.go
git commit -m "feat: add backpack websocket market data"
```

### Task 7: Implement private WebSocket order and position streams

**Files:**
- Create: `backpack/stream_mapping_test.go`
- Modify: `backpack/sdk/ws.go`
- Modify: `backpack/sdk/signer.go`
- Modify: `backpack/sdk/types.go`
- Modify: `backpack/perp_adapter.go`
- Modify: `backpack/spot_adapter.go`

- [ ] **Step 1: Write the failing private-stream tests**

```go
func TestMapOrderUpdateTerminalStatus(t *testing.T) {
	raw := sdk.OrderUpdate{OrderID: "1", Status: "Cancelled", Symbol: "BTC_USDC"}
	got := mapOrderUpdate(raw)
	require.Equal(t, exchanges.OrderStatusCancelled, got.Status)
}

func TestBuildPrivateSubscribePayloadUsesSignedInstruction(t *testing.T) {
	payload := buildPrivateSubscribePayload("account.orderUpdate", "api", "sig", 123, 5000)
	require.Contains(t, payload, "\"signature\":\"sig\"")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./backpack -run 'TestMapOrderUpdateTerminalStatus|TestBuildPrivateSubscribePayload'`
Expected: FAIL because private WS helpers do not exist.

- [ ] **Step 3: Write minimal implementation**

```go
func (a *Adapter) WatchOrders(ctx context.Context, cb exchanges.OrderUpdateCallback) error {
	return a.privateWS.SubscribeOrders(ctx, func(raw sdk.OrderUpdate) {
		cb(mapOrderUpdate(raw))
	})
}
```

Implement at minimum:

- `WatchOrders`
- `StopWatchOrders`
- perp `WatchPositions`
- `StopWatchPositions`
- private WS auth/subscription signing
- order update mapping that preserves `OrderID`, `ClientOrderID`, `FilledQuantity`, and terminal statuses

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./backpack -run 'TestMapOrderUpdateTerminalStatus|TestBuildPrivateSubscribePayload'`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backpack/stream_mapping_test.go backpack/sdk/ws.go backpack/sdk/signer.go backpack/sdk/types.go backpack/perp_adapter.go backpack/spot_adapter.go
git commit -m "feat: add backpack private websocket streams"
```

### Task 8: Implement remaining shared-interface methods and registry wiring

**Files:**
- Modify: `backpack/register.go`
- Modify: `backpack/perp_adapter.go`
- Modify: `backpack/spot_adapter.go`
- Create: `backpack/registry_test.go`

- [ ] **Step 1: Write the failing registry and unsupported-method tests**

```go
func TestLookupConstructorBuildsBackpackSpotAndPerp(t *testing.T) {
	ctor, err := exchanges.LookupConstructor("BACKPACK")
	require.NoError(t, err)
	require.NotNil(t, ctor)
}

func TestUnsupportedMethodsReturnErrNotSupported(t *testing.T) {
	adp := &SpotAdapter{}
	err := adp.TransferAsset(context.Background(), &exchanges.TransferParams{})
	require.ErrorIs(t, err, exchanges.ErrNotSupported)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./backpack -run 'TestLookupConstructorBuildsBackpackSpotAndPerp|TestUnsupportedMethodsReturnErrNotSupported'`
Expected: FAIL because constructor wiring or unsupported-method behavior is incomplete.

- [ ] **Step 3: Write minimal implementation**

```go
func init() {
	exchanges.Register("BACKPACK", func(ctx context.Context, mt exchanges.MarketType, opts map[string]string) (exchanges.Exchange, error) {
		o := Options{
			APIKey:        opts["api_key"],
			PrivateKey:    opts["private_key"],
			QuoteCurrency: exchanges.QuoteCurrency(opts["quote_currency"]),
		}
		switch mt {
		case exchanges.MarketTypePerp:
			return NewAdapter(ctx, o)
		case exchanges.MarketTypeSpot:
			return NewSpotAdapter(ctx, o)
		default:
			return nil, fmt.Errorf("backpack: unsupported market type %q", mt)
		}
	})
}
```

Cover explicitly:

- `SetLeverage`
- `ModifyOrder`
- `TransferAsset`
- `StopWatch*` methods
- error behavior for unsupported Backpack operations

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./backpack -run 'TestLookupConstructorBuildsBackpackSpotAndPerp|TestUnsupportedMethodsReturnErrNotSupported'`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backpack/register.go backpack/perp_adapter.go backpack/spot_adapter.go backpack/registry_test.go
git commit -m "feat: wire backpack registry and shared interface coverage"
```

### Task 9: Wire integration tests and environment variables

**Files:**
- Create: `backpack/adapter_test.go`
- Modify: `.env.example`
- Modify: `README.md`
- Modify: `README_CN.md`

- [ ] **Step 1: Write the failing Backpack adapter test wiring**

```go
func TestPerpAdapter_Compliance(t *testing.T) {
	adp := setupPerpAdapter(t)
	testsuite.RunAdapterComplianceTests(t, adp, os.Getenv("BACKPACK_PERP_TEST_SYMBOL"))
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./backpack -run TestPerpAdapter_Compliance`
Expected: FAIL because setup helpers and env wiring do not exist yet.

- [ ] **Step 3: Write minimal implementation**

```go
func setupPerpAdapter(t *testing.T) *Adapter {
	t.Helper()
	_ = godotenv.Load("../../.env")
	if os.Getenv("BACKPACK_API_KEY") == "" || os.Getenv("BACKPACK_PRIVATE_KEY") == "" {
		t.Skip("Skipping: BACKPACK credentials not set")
	}
	adp, err := NewAdapter(context.Background(), Options{
		APIKey:     os.Getenv("BACKPACK_API_KEY"),
		PrivateKey: os.Getenv("BACKPACK_PRIVATE_KEY"),
	})
	if err != nil {
		t.Fatalf("NewAdapter failed: %v", err)
	}
	return adp
}
```

Add to `.env.example`:

```dotenv
BACKPACK_API_KEY=
BACKPACK_PRIVATE_KEY=
BACKPACK_SPOT_TEST_SYMBOL=
BACKPACK_PERP_TEST_SYMBOL=
BACKPACK_QUOTE_CURRENCY=
```

Use exchange-specific suite config:

- spot `RunOrderSuite` may need `SkipSlippage: true`
- spot `RunLocalStateSuite` should only be enabled if private Backpack spot streams satisfy it

- [ ] **Step 4: Run tests to verify they pass structurally**

Run: `go test ./backpack -run 'TestPerpAdapter_Compliance|TestSpotAdapter_Compliance'`
Expected: PASS when credentials are absent via `t.Skip`, or PASS with live credentials if configured.

- [ ] **Step 5: Commit**

```bash
git add backpack/adapter_test.go .env.example README.md README_CN.md
git commit -m "test: wire backpack integration suites"
```

### Task 10: Full verification and live execution

**Files:**
- Modify: any Backpack files needed to fix verification failures

- [ ] **Step 1: Run deterministic Backpack tests**

Run: `go test ./backpack/...`
Expected: PASS for unit tests and structural tests. Live integration tests may skip if Backpack env vars are not configured.

- [ ] **Step 2: Add Backpack credentials and symbols to `.env`**

Required:

```dotenv
BACKPACK_API_KEY=...
BACKPACK_PRIVATE_KEY=...
BACKPACK_SPOT_TEST_SYMBOL=...
BACKPACK_PERP_TEST_SYMBOL=...
```

Optional:

```dotenv
BACKPACK_QUOTE_CURRENCY=USDC
```

- [ ] **Step 3: Run targeted live Backpack suites**

Run: `go test ./backpack -run 'TestPerpAdapter_(Compliance|Orders|Lifecycle|LocalState)|TestSpotAdapter_(Compliance|Orders|Lifecycle)' -v`
Expected: PASS for the default spot/perp matrix, with spot skip flags adjusted in code if Backpack spot does not support slippage.

If `backpack/adapter_test.go` wires `TestSpotAdapter_LocalState`, also run:

Run: `go test ./backpack -run TestSpotAdapter_LocalState -v`
Expected: PASS

- [ ] **Step 4: Run full repository verification**

Run: `go test ./...`
Expected: PASS, or non-Backpack unrelated skips only.

- [ ] **Step 5: Commit final fixes**

```bash
git add backpack .env.example README.md README_CN.md
git commit -m "feat: add backpack exchange adapter"
```
