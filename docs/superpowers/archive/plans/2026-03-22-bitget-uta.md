# Bitget UTA Adapter Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a new `bitget` exchange package that supports Bitget UTA spot plus USDT/USDC futures with public market data, private trading/account access, shared `testsuite` wiring, and live test setup.

**Architecture:** Implement Bitget as one top-level exchange package with a flat `bitget/sdk/` because UTA v3 is one low-level API family with category-based divergence. Build public REST and WS first, then private REST, then private streams, then adapter wiring, then live tests. Keep auth and wire types in `sdk/`, and keep symbol/category mapping plus unified-model translation in adapter files.

**Tech Stack:** Go, repository shared `Exchange` interfaces, Bitget UTA v3 REST/WS, `shopspring/decimal`, `godotenv`, repository `testsuite`

---

## File Map

### New files

- `bitget/options.go`
  - adapter options, quote validation, logger fallback
- `bitget/register.go`
  - `BITGET` registry wiring and market dispatch
- `bitget/common.go`
  - shared symbol/category/status/side helpers and auth-required helpers
- `bitget/perp_adapter.go`
  - perp shared-interface implementation for `USDT-FUTURES` and `USDC-FUTURES`
- `bitget/spot_adapter.go`
  - spot shared-interface implementation for `SPOT`
- `bitget/orderbook.go`
  - local orderbook sync and WS book subscription helpers
- `bitget/adapter_test.go`
  - live shared `testsuite` wiring and env helpers
- `bitget/sdk/client.go`
  - HTTP client, URLs, request helpers, shared request execution
- `bitget/sdk/auth.go`
  - REST signing, WS login signing
- `bitget/sdk/types.go`
  - REST/WS wire types and shared enums
- `bitget/sdk/public_rest.go`
  - instruments, ticker, depth, trades, klines
- `bitget/sdk/private_rest.go`
  - account config, balances, positions, order placement, cancel, modify, order detail, open orders
- `bitget/sdk/public_ws.go`
  - public WS connection and subscriptions
- `bitget/sdk/private_ws.go`
  - private WS login and private order/position subscriptions

### Existing files to modify

- `.env.example`
  - add Bitget credentials and test symbol env vars
- `README.md`
  - document Bitget support and any special notes if needed
- `README_CN.md`
  - document Bitget support and any special notes if needed

### Existing files to reference while implementing

- `exchange.go`
- `errors.go`
- `base_adapter.go`
- `local_state.go`
- `registry.go`
- `testsuite/compliance.go`
- `testsuite/order_suite.go`
- `testsuite/order_query_suite.go`
- `testsuite/lifecycle_suite.go`
- `testsuite/localstate_suite.go`
- `okx/options.go`
- `okx/register.go`
- `backpack/adapter_test.go`
- `backpack/sdk/*`

## Task 1: Scaffold Package And SDK Skeleton

**Files:**
- Create: `bitget/options.go`
- Create: `bitget/register.go`
- Create: `bitget/common.go`
- Create: `bitget/perp_adapter.go`
- Create: `bitget/spot_adapter.go`
- Create: `bitget/sdk/client.go`
- Create: `bitget/sdk/auth.go`
- Create: `bitget/sdk/types.go`
- Create: `bitget/sdk/public_rest.go`
- Create: `bitget/sdk/private_rest.go`
- Create: `bitget/sdk/public_ws.go`
- Create: `bitget/sdk/private_ws.go`
- Test: `go test ./bitget/... -run TestDoesNotExist`

- [ ] **Step 1: Create the package, adapter, and SDK skeleton files**

Add the new files with package declarations, placeholder structs, placeholder constructors, and the minimum exported types needed for the later tasks to compile.

- [ ] **Step 2: Implement `Options` and registry wiring**

Use `okx/options.go` and `okx/register.go` as the nearest pattern.

Requirements:

- `Options` must include `APIKey`, `SecretKey`, `Passphrase`, `QuoteCurrency`, `Logger`
- `QuoteCurrency` supports only `USDT` and `USDC`
- default quote is `USDT`
- `register.go` must register `BITGET`
- `MarketTypePerp` dispatches to `NewAdapter`
- `MarketTypeSpot` dispatches to `NewSpotAdapter`
- `perp_adapter.go` and `spot_adapter.go` must exist in this task with placeholder `NewAdapter` / `NewSpotAdapter`
- include enough placeholder `Exchange` / `Streamable` methods, including `Close()` and stop methods, to let the package compile before real behavior exists

- [ ] **Step 3: Define SDK client and auth helpers**

Implement:

- base URLs for REST and WS
- shared HTTP client
- public request execution
- signed private request execution
- WS login signing payload helper

- [ ] **Step 4: Run package compile verification**

Run: `GOCACHE=/tmp/gocache-bitget go test ./bitget/... -run TestDoesNotExist`

Expected: package compiles, even if many adapter methods still return placeholders.

- [ ] **Step 5: Commit**

```bash
git add bitget
git commit -m "feat: scaffold bitget UTA package"
```

## Task 2: Implement Public REST Market Data

**Files:**
- Modify: `bitget/sdk/types.go`
- Modify: `bitget/sdk/public_rest.go`
- Modify: `bitget/common.go`
- Modify: `bitget/perp_adapter.go`
- Modify: `bitget/spot_adapter.go`
- Test: `go test ./bitget/... -run TestDoesNotExist`

- [ ] **Step 1: Add instrument and market-data wire types**

Define the wire structs needed for:

- instruments
- ticker
- orderbook snapshot
- trades
- klines

- [ ] **Step 2: Implement public REST SDK methods**

Implement low-level methods for:

- list instruments by category
- fetch ticker
- fetch orderbook snapshot
- fetch trades
- fetch klines

- [ ] **Step 3: Implement symbol universe loading**

In adapter constructors, load and map instruments into:

- symbol details
- symbol list
- category-specific universe filtered by quote currency

Use base symbols at the adapter boundary and Bitget native symbols internally.

- [ ] **Step 4: Implement public adapter methods**

Implement:

- `ListSymbols`
- `FormatSymbol`
- `ExtractSymbol`
- `FetchTicker`
- `FetchOrderBook`
- `FetchTrades`
- `FetchKlines`
- `FetchSymbolDetails`

- [ ] **Step 5: Verify compile**

Run: `GOCACHE=/tmp/gocache-bitget go test ./bitget/... -run TestDoesNotExist`

Expected: compile passes with public methods wired.

- [ ] **Step 6: Commit**

```bash
git add bitget
git commit -m "feat: add bitget public market data"
```

## Task 3: Implement Public Orderbook WS And Local Book

**Files:**
- Create: `bitget/orderbook.go`
- Modify: `bitget/sdk/public_ws.go`
- Modify: `bitget/perp_adapter.go`
- Modify: `bitget/spot_adapter.go`
- Test: `go test ./bitget/... -run TestDoesNotExist`

- [ ] **Step 1: Define public orderbook WS payload types and subscription helpers**

Add public WS support for Bitget orderbook channels in `sdk/public_ws.go`.

- [ ] **Step 2: Implement local orderbook sync helper**

Build the local-book synchronization logic in `bitget/orderbook.go`.

Requirements:

- `WatchOrderBook` blocks until the local book is ready
- `GetLocalOrderBook` returns non-`nil` after successful sync
- `StopWatchOrderBook` unsubscribes and clears local state

- [ ] **Step 3: Wire `WatchOrderBook` into spot and perp adapters**

Both adapter types should use the same public WS helper and local-book implementation.

- [ ] **Step 4: Stub unsupported public live streams honestly**

If `WatchTicker`, `WatchTrades`, or `WatchKlines` are not implemented in v1, return `exchanges.ErrNotSupported`.

- [ ] **Step 5: Verify compile**

Run: `GOCACHE=/tmp/gocache-bitget go test ./bitget/... -run TestDoesNotExist`

- [ ] **Step 6: Commit**

```bash
git add bitget
git commit -m "feat: add bitget public orderbook streaming"
```

## Task 4: Implement Private Initialization And Auth-Gated Error Paths

**Files:**
- Modify: `bitget/common.go`
- Modify: `bitget/sdk/private_rest.go`
- Modify: `bitget/perp_adapter.go`
- Modify: `bitget/spot_adapter.go`
- Create: `bitget/private_init_test.go`
- Test: `go test ./bitget/... -run TestDoesNotExist`

- [ ] **Step 1: Add credential completeness helper**

Implement helpers that classify:

- no credentials present
- incomplete credentials
- complete credentials

- [ ] **Step 2: Add private initialization validation**

When complete credentials are provided, constructors must validate:

- request signing works
- account is UTA-compatible
- account config is compatible with the market family

If validation fails, constructor returns an error.

- [ ] **Step 3: Add constructor-behavior tests before implementation**

Add focused tests covering:

- no credentials: public constructor path is allowed
- partial credentials: constructor fails before any private network use
- auth-gated helper methods return an `ExchangeError` wrapping `exchanges.ErrAuthFailed`

If non-UTA rejection cannot be unit tested without live credentials, document it as a live-only validation path in `bitget/adapter_test.go` and structure the constructor validation logic so partial-credential behavior is still unit tested.

- [ ] **Step 4: Define one auth-gated error path**

Implement a shared helper that returns an `exchanges.ExchangeError` wrapping `exchanges.ErrAuthFailed` for auth-gated methods invoked without usable credentials.

Apply this consistently to private surfaces such as:

- `FetchAccount`
- `FetchBalance`
- `FetchOpenOrders`
- `PlaceOrder`
- `WatchOrders`
- `WatchPositions`

- [ ] **Step 5: Verify compile**

Run: `GOCACHE=/tmp/gocache-bitget go test ./bitget/... -run TestDoesNotExist`

- [ ] **Step 6: Commit**

```bash
git add bitget
git commit -m "feat: validate bitget private initialization"
```

## Task 5: Implement Private REST Trading And Account Surfaces

**Files:**
- Modify: `bitget/sdk/types.go`
- Modify: `bitget/sdk/private_rest.go`
- Modify: `bitget/perp_adapter.go`
- Modify: `bitget/spot_adapter.go`
- Test: `go test ./bitget/... -run TestDoesNotExist`

- [ ] **Step 1: Add private REST wire types**

Define request/response wire types for:

- balances
- account config
- positions
- order placement
- cancel
- cancel all
- modify
- order detail
- open orders
- fee rate if directly available

- [ ] **Step 2: Implement low-level private REST methods**

Implement SDK methods for:

- account config lookup
- balance/account snapshot
- positions
- place order
- cancel order
- cancel all orders
- modify order
- order detail
- open orders

- [ ] **Step 3: Implement spot private adapter methods**

Implement:

- `PlaceOrder`
- `CancelOrder`
- `CancelAllOrders`
- `FetchOrderByID`
- `FetchOrders` returning `exchanges.ErrNotSupported` in v1 unless direct history support is verified
- `FetchOpenOrders`
- `FetchAccount`
- `FetchBalance`
- `FetchSpotBalances`
- `FetchFeeRate` if directly supported, else `exchanges.ErrNotSupported`
- `TransferAsset` returning `exchanges.ErrNotSupported`
- decide the spot slippage strategy now:
  - either support market-with-slippage through `BaseAdapter.ApplySlippage`
  - or plan to set `SkipSlippage: true` in the spot `RunOrderSuite`

- [ ] **Step 4: Implement perp private adapter methods**

Implement:

- all relevant methods above
- `FetchPositions`
- `ModifyOrder`
- `SetLeverage`
- `FetchFundingRate` and `FetchAllFundingRates` only if directly supportable in v1; otherwise return `exchanges.ErrNotSupported`
- decide the perp slippage strategy now:
  - either support market-with-slippage through `BaseAdapter.ApplySlippage`
  - or plan to set `SkipSlippage: true` in the perp `RunOrderSuite`

- [ ] **Step 5: Enforce order-query semantics**

Requirements:

- `FetchOrderByID` uses direct order-detail API
- `FetchOrderByID` returns terminal orders when available
- true misses map to `exchanges.ErrOrderNotFound`
- `FetchOpenOrders` returns only open orders
- `FetchOrders` stays `exchanges.ErrNotSupported` unless the Bitget UTA history surface is explicitly confirmed broad enough

- [ ] **Step 6: Verify compile**

Run: `GOCACHE=/tmp/gocache-bitget go test ./bitget/... -run TestDoesNotExist`

- [ ] **Step 7: Commit**

```bash
git add bitget
git commit -m "feat: add bitget private trading and account APIs"
```

## Task 6: Implement Private WS For Orders And Positions

**Files:**
- Modify: `bitget/sdk/private_ws.go`
- Modify: `bitget/perp_adapter.go`
- Modify: `bitget/spot_adapter.go`
- Test: `go test ./bitget/... -run TestDoesNotExist`

- [ ] **Step 1: Add private WS login and subscription support**

Implement:

- connection establishment
- login
- order channel subscribe
- position channel subscribe
- reconnect behavior

- [ ] **Step 2: Implement unified order update mapping**

Map Bitget private order events into `exchanges.Order` updates suitable for:

- `RunLifecycleSuite`
- `LocalState`

- [ ] **Step 3: Implement spot `WatchOrders` and stop methods**

Implement:

- `WatchOrders`
- `StopWatchOrders`
- `Close`
- `StopWatchTicker`
- `StopWatchTrades`
- `StopWatchKlines`

`WatchPositions` and `StopWatchPositions` should return `exchanges.ErrNotSupported` for spot if no meaningful position stream applies.

- [ ] **Step 4: Implement perp `WatchOrders`, `Close`, and stop methods**

Perp must have a real private order stream to satisfy lifecycle and local-state readiness.

Implement at minimum:

- `WatchOrders`
- `StopWatchOrders`
- `StopWatchPositions`
- `Close`

- [ ] **Step 5: Implement perp `WatchPositions` only if the stream is clean enough**

If Bitget’s private position channel maps cleanly to the shared model, implement it.

Otherwise:

- return `exchanges.ErrNotSupported`
- keep `WatchOrders` working
- do not block the rest of v1 on this path

- [ ] **Step 6: Verify compile**

Run: `GOCACHE=/tmp/gocache-bitget go test ./bitget/... -run TestDoesNotExist`

- [ ] **Step 7: Commit**

```bash
git add bitget
git commit -m "feat: add bitget private order and position streams"
```

## Task 7: Wire Live Tests And Environment Setup

**Files:**
- Create: `bitget/adapter_test.go`
- Modify: `.env.example`
- Modify: `README.md`
- Modify: `README_CN.md`
- Test: `go test ./bitget/... -run TestDoesNotExist`

- [ ] **Step 1: Add Bitget env vars to `.env.example`**

Add:

- `BITGET_API_KEY`
- `BITGET_SECRET_KEY`
- `BITGET_PASSPHRASE`
- `BITGET_PERP_TEST_SYMBOL`
- `BITGET_SPOT_TEST_SYMBOL`
- `BITGET_QUOTE_CURRENCY`

- [ ] **Step 2: Implement `adapter_test.go` with resilient `.env` loading**

Follow the Backpack pattern and load:

- `.env`
- `../.env`
- `../../.env`
- `../../../.env`

- [ ] **Step 3: Add targeted constructor/live init checks**

Add focused tests or subtests for:

- no-credential public construction
- partial credential rejection
- non-UTA private credential rejection when a live validation path is available

- [ ] **Step 4: Wire shared suites for spot**

Wire:

- `RunAdapterComplianceTests`
- `RunOrderSuite`
- `RunOrderQuerySemanticsSuite`
- `RunLifecycleSuite`
- `RunLocalStateSuite`

Decide slippage explicitly:

- if spot `PlaceOrder` supports `BaseAdapter.ApplySlippage`, keep the default slippage subtest
- otherwise set `SkipSlippage: true`

Set order-query config:

- `SupportsOpenOrders: true`
- `SupportsTerminalLookup: true`
- `SupportsOrderHistory: false`

- [ ] **Step 5: Wire shared suites for perp**

Wire the same shared suites with:

- `SupportsOpenOrders: true`
- `SupportsTerminalLookup: true`
- `SupportsOrderHistory: false`

Decide slippage explicitly for perp as well:

- keep default slippage test only if implementation supports it
- otherwise set `SkipSlippage: true`

- [ ] **Step 6: Document any honest v1 limitations in README files**

If funding or position streaming is deferred, document it briefly and precisely.

- [ ] **Step 7: Verify compile**

Run: `GOCACHE=/tmp/gocache-bitget go test ./bitget/... -run TestDoesNotExist`

- [ ] **Step 8: Commit**

```bash
git add .env.example README.md README_CN.md bitget/adapter_test.go
git commit -m "test: wire bitget live adapter suites"
```

## Task 8: Run Live Validation And Tighten Capability Claims

**Files:**
- Modify: `bitget/perp_adapter.go`
- Modify: `bitget/spot_adapter.go`
- Modify: `bitget/adapter_test.go`
- Modify: `README.md`
- Modify: `README_CN.md`

- [ ] **Step 1: Run package compile baseline**

Run: `GOCACHE=/tmp/gocache-bitget go test ./bitget/... -run TestDoesNotExist`

- [ ] **Step 2: Run Bitget spot live suites**

Run after env vars are configured:

```bash
GOCACHE=/tmp/gocache-bitget go test ./bitget -run 'TestSpotAdapter_(Compliance|Orders|OrderQuerySemantics|Lifecycle|LocalState)' -count=1 -v
```

- [ ] **Step 3: Run Bitget perp live suites**

Run after env vars are configured:

```bash
GOCACHE=/tmp/gocache-bitget go test ./bitget -run 'TestPerpAdapter_(Compliance|Orders|OrderQuerySemantics|Lifecycle|LocalState)' -count=1 -v
```

- [ ] **Step 4: Tighten capability claims based on real results**

If any of these were left provisional, set them definitively from live results:

- `FetchFeeRate`
- `FetchFundingRate`
- `FetchAllFundingRates`
- perp `WatchPositions`

If a surface is not cleanly supportable, return `exchanges.ErrNotSupported` and document it instead of carrying a brittle partial implementation.

- [ ] **Step 5: Re-run compile and any affected live suites**

Run:

```bash
GOCACHE=/tmp/gocache-bitget go test ./bitget/... -run TestDoesNotExist
```

and rerun the specific live suites impacted by the last changes.

- [ ] **Step 6: Commit**

```bash
git add bitget README.md README_CN.md
git commit -m "fix: finalize bitget capability claims"
```

## Final Verification

- [ ] **Step 1: Run repository compile verification**

Run:

```bash
GOCACHE=/tmp/gocache-bitget go test ./... -run TestDoesNotExist
```

Expected: full repository compile passes.

- [ ] **Step 2: Run Bitget package live verification**

Run:

```bash
GOCACHE=/tmp/gocache-bitget go test ./bitget -count=1 -v
```

Expected: Bitget live suites pass or skip only for missing env vars / unsupported-but-honestly-declared surfaces.

- [ ] **Step 3: Run diff hygiene checks**

Run:

```bash
git diff --check
```

Expected: no whitespace or patch-format issues.

- [ ] **Step 4: Commit any final fixups**

```bash
git add .
git commit -m "chore: finish bitget UTA adapter"
```
