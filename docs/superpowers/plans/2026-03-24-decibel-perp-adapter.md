# Decibel Perp Adapter Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Decibel perpetual adapter that uses authenticated REST and WebSocket reads plus Aptos-signed place/cancel transactions, and make it pass the repository compliance, order, and lifecycle suites.

## Status

Completed and merged to `main`.

Final validation:

- `go test -mod=mod ./decibel/... -count=1`
- `GOCACHE=/tmp/exchanges-gocache RUN_FULL=1 go test -mod=mod ./decibel -run "TestPerpAdapter_(Compliance|Orders|Lifecycle)$" -count=1 -v`

**Architecture:** Add a new `decibel` exchange package that keeps unified adapter semantics at the package boundary and pushes protocol details into `sdk/rest`, `sdk/ws`, and `sdk/aptos`. Load market metadata during construction, map base symbols to Decibel market addresses, route reads through REST and WebSocket, route writes through Aptos transaction builders, and make unsupported v1 behaviors return repository sentinel errors explicitly.

**Tech Stack:** Go, `github.com/aptos-labs/aptos-go-sdk`, existing `exchanges` base adapter/testsuite infrastructure, `go test`, live Decibel API key + Aptos private key + subaccount credentials

---

## File Map

### Files To Create

- `decibel/options.go`
- `decibel/options_test.go`
- `decibel/register.go`
- `decibel/constructor_test.go`
- `decibel/common.go`
- `decibel/common_test.go`
- `decibel/perp_adapter.go`
- `decibel/orderbook.go`
- `decibel/unsupported_test.go`
- `decibel/adapter_test.go`
- `decibel/sdk/rest/client.go`
- `decibel/sdk/rest/types.go`
- `decibel/sdk/rest/market.go`
- `decibel/sdk/rest/account.go`
- `decibel/sdk/rest/order.go`
- `decibel/sdk/rest/client_test.go`
- `decibel/sdk/ws/client.go`
- `decibel/sdk/ws/types.go`
- `decibel/sdk/ws/client_test.go`
- `decibel/sdk/aptos/client.go`
- `decibel/sdk/aptos/types.go`
- `decibel/sdk/aptos/client_test.go`

### Files To Modify

- `go.mod`
- `go.sum`
- `.env.example`

### Files To Reference While Implementing

- `docs/superpowers/specs/2026-03-24-decibel-perp-adapter-design.md`
- `exchange.go`
- `errors.go`
- `base_adapter.go`
- `local_state.go`
- `testsuite/compliance.go`
- `testsuite/order_suite.go`
- `testsuite/lifecycle_suite.go`
- `internal/testenv/testenv.go`
- `hyperliquid/options.go`
- `hyperliquid/perp_adapter.go`
- `hyperliquid/adapter_test.go`
- `standx/unsupported_test.go`
- `edgex/unsupported_test.go`

## Task 1: Scaffold Package, Dependencies, And Constructor/Auth Rules

**Files:**
- Create: `decibel/options.go`
- Create: `decibel/options_test.go`
- Create: `decibel/register.go`
- Create: `decibel/constructor_test.go`
- Create: `decibel/perp_adapter.go`
- Modify: `go.mod`
- Modify: `go.sum`

- [ ] **Step 1: Write the failing constructor/auth tests**

Add tests that lock in the v1 auth contract:

- `Options` rejects partial credentials with `ErrAuthFailed`
- empty credentials are rejected because Decibel bootstrap reads are authenticated
- valid `api_key + private_key + subaccount_addr` passes validation
- registry construction accepts `MarketTypePerp` when a stubbed metadata loader succeeds
- registry construction rejects `MarketTypeSpot`

- [ ] **Step 2: Run the targeted constructor tests to verify they fail**

Run: `go test ./decibel -run 'Test(DecibelOptions|DecibelRegistry)' -count=1`

Expected: FAIL because the package and constructor logic do not exist yet.

- [ ] **Step 3: Add the Aptos dependency and package skeleton**

Update `go.mod`/`go.sum` to include the selected Aptos Go SDK dependency and create the `decibel` package skeleton with `Options`, logger defaulting, credential validation, registry wiring, and a minimal `NewAdapter` constructor seam.

- [ ] **Step 4: Implement strict constructor validation**

Implement `Options.validateCredentials()` and the non-metadata option helpers so:

- only the full credential triplet is accepted
- `account_addr` is not user input
- quote selection is stored on `Options`, but exact supported-quote validation is deferred until `NewAdapter` has loaded Decibel market metadata

- [ ] **Step 5: Implement registry construction**

Add `decibel/register.go` with `DECIBEL` registration that:

- translates `map[string]string` into `Options`
- dispatches `MarketTypePerp` to a minimal `NewAdapter` that validates credentials and calls an injectable metadata bootstrap seam
- rejects other market types with a constructor error

- [ ] **Step 6: Implement the minimal constructor skeleton**

Add the first version of `decibel/perp_adapter.go` containing:

- `Adapter` type definition
- `NewAdapter`
- constructor-time metadata bootstrap hook that can be stubbed in tests now and implemented fully in later tasks
- enough adapter identity methods to let registry construction compile without inventing a temporary code path
- [ ] **Step 7: Run the targeted constructor tests to verify green**

Run: `go test ./decibel -run 'Test(DecibelOptions|DecibelRegistry)' -count=1`

Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add go.mod go.sum decibel/options.go decibel/options_test.go decibel/register.go decibel/constructor_test.go decibel/perp_adapter.go
git commit -m "feat: scaffold decibel adapter auth and registry"
```

## Task 2: Build Market Metadata, Symbol Mapping, And REST Read SDK

**Files:**
- Create: `decibel/common.go`
- Create: `decibel/common_test.go`
- Create: `decibel/sdk/rest/client.go`
- Create: `decibel/sdk/rest/types.go`
- Create: `decibel/sdk/rest/market.go`
- Create: `decibel/sdk/rest/account.go`
- Create: `decibel/sdk/rest/order.go`
- Create: `decibel/sdk/rest/client_test.go`

- [ ] **Step 1: Write the failing metadata and REST SDK tests**

Add tests that verify:

- REST requests send `Authorization: Bearer <api_key>`
- market metadata parsing produces a deterministic `base symbol -> market address` map
- duplicate base-symbol collisions fail loudly
- price/size quantization and chain-unit conversion use `decimal.Decimal`
- Decibel REST errors map cleanly to repository sentinel errors

- [ ] **Step 2: Run the targeted metadata/REST tests to verify they fail**

Run: `go test ./decibel ./decibel/sdk/rest -run 'Test(DecibelMetadata|DecibelREST)' -count=1`

Expected: FAIL because the metadata helpers and REST client are not implemented yet.

- [ ] **Step 3: Implement REST client transport and response types**

Create the authenticated REST client with:

- base URL and HTTP client configuration
- request helper that injects Bearer auth
- typed market/account/order response models needed by the adapter

- [ ] **Step 4: Implement market metadata loading and symbol lookup**

Add metadata helpers in `decibel/common.go` that:

- parse the markets response
- extract a single base symbol per perp market
- cache `symbol -> market metadata`
- cache `market address -> symbol`
- expose precision and minimum-size helpers

- [ ] **Step 5: Implement REST error normalization**

Map Decibel REST failures into `ExchangeError` + shared sentinels for auth, rate limits, symbol misses, precision failures, and order misses.

- [ ] **Step 6: Run the targeted metadata/REST tests to verify green**

Run: `go test ./decibel ./decibel/sdk/rest -run 'Test(DecibelMetadata|DecibelREST)' -count=1`

Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add decibel/common.go decibel/common_test.go decibel/sdk/rest/client.go decibel/sdk/rest/types.go decibel/sdk/rest/market.go decibel/sdk/rest/account.go decibel/sdk/rest/order.go decibel/sdk/rest/client_test.go
git commit -m "feat: add decibel metadata and rest sdk"
```

## Task 3: Build Aptos Place/Cancel SDK And Precision Encoding

**Files:**
- Create: `decibel/sdk/aptos/client.go`
- Create: `decibel/sdk/aptos/types.go`
- Modify: `decibel/common.go`
- Modify: `decibel/common_test.go`
- Create: `decibel/sdk/aptos/client_test.go`

- [ ] **Step 1: Write the failing Aptos SDK tests**

Add tests that verify:

- the Aptos client derives the account address from the provided private key
- place-order requests encode market, side, price, size, reduce-only, and client-id/correlation fields correctly
- cancel-order requests encode the Decibel order identifier correctly
- invalid precision and below-minimum quantities are rejected before transaction build

- [ ] **Step 2: Run the targeted Aptos tests to verify they fail**

Run: `go test ./decibel ./decibel/sdk/aptos -run 'Test(DecibelAptos|DecibelPrecision)' -count=1`

Expected: FAIL because the Aptos builder and encoding paths do not exist yet.

- [ ] **Step 3: Implement Aptos client and request types**

Add a focused `sdk/aptos` client that:

- loads the Aptos private key
- derives the main account address
- builds the exact Decibel place/cancel entry-function payloads
- signs and submits transactions through the chosen SDK

- [ ] **Step 4: Centralize precision and chain-unit helpers**

Extend `decibel/common.go` so all price/size validation and chain encoding is shared between the adapter and Aptos client rather than duplicated.

- [ ] **Step 5: Run the targeted Aptos tests to verify green**

Run: `go test ./decibel ./decibel/sdk/aptos -run 'Test(DecibelAptos|DecibelPrecision)' -count=1`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add decibel/common.go decibel/common_test.go decibel/sdk/aptos/client.go decibel/sdk/aptos/types.go decibel/sdk/aptos/client_test.go
git commit -m "feat: add decibel aptos trading sdk"
```

## Task 4: Build WebSocket SDK, Orderbook Sync, And Private Event Handling

**Files:**
- Create: `decibel/sdk/ws/client.go`
- Create: `decibel/sdk/ws/types.go`
- Create: `decibel/sdk/ws/client_test.go`
- Create: `decibel/orderbook.go`
- Modify: `decibel/common.go`

- [ ] **Step 1: Write the failing WebSocket and orderbook tests**

Add tests that verify:

- WebSocket auth uses Decibel's required protocol/header setup
- the client replays subscriptions after reconnect
- market-depth events can seed and update a local orderbook
- private order-update events normalize into repository order statuses

- [ ] **Step 2: Run the targeted WebSocket tests to verify they fail**

Run: `go test ./decibel ./decibel/sdk/ws -run 'Test(DecibelWS|DecibelOrderBook)' -count=1`

Expected: FAIL because the WebSocket client and local orderbook sync code do not exist yet.

- [ ] **Step 3: Implement the WebSocket client**

Create `sdk/ws` with:

- authenticated dial logic
- ping/pong handling
- bounded reconnect and subscription replay
- typed dispatchers for market depth and private account/order events

- [ ] **Step 4: Implement orderbook sync helpers**

Add `decibel/orderbook.go` to convert Decibel depth snapshots/deltas into the repository local orderbook model used by `BaseAdapter`.

- [ ] **Step 5: Implement order status normalization helpers**

Extend `decibel/common.go` with the stable state mapping needed by `WatchOrders` and `PlaceOrder` reconciliation.

- [ ] **Step 6: Run the targeted WebSocket tests to verify green**

Run: `go test ./decibel ./decibel/sdk/ws -run 'Test(DecibelWS|DecibelOrderBook)' -count=1`

Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add decibel/sdk/ws/client.go decibel/sdk/ws/types.go decibel/sdk/ws/client_test.go decibel/orderbook.go decibel/common.go
git commit -m "feat: add decibel websocket and orderbook support"
```

## Task 5: Integrate The Perp Adapter And Lock Unsupported Boundaries

**Files:**
- Modify: `decibel/perp_adapter.go`
- Create: `decibel/unsupported_test.go`
- Modify: `decibel/common.go`
- Modify: `decibel/sdk/rest/account.go`
- Modify: `decibel/sdk/rest/order.go`

- [ ] **Step 1: Write the failing adapter behavior tests**

Add focused tests that lock in:

- `FetchPositions` is implemented from the same account sources as `FetchAccount`
- `FetchFeeRate` is implemented with a stable Decibel-backed mapping or a documented deterministic fallback acceptable to the compliance suite
- `SetLeverage`, `ModifyOrder`, `FetchFundingRate`, and `FetchAllFundingRates` return `ErrNotSupported`
- `FetchTicker`, `FetchOrderBook`, `FetchAccount`, `FetchOpenOrders`, and `FetchOrderByID` call the expected SDK seams
- `WatchOrderBook` waits for initial sync before returning
- `WatchOrders` can reconcile a newly placed order into a stable `OrderID`

- [ ] **Step 2: Run the targeted adapter tests to verify they fail**

Run: `go test ./decibel -run 'Test(DecibelAdapter|DecibelUnsupported)' -count=1`

Expected: FAIL because the adapter orchestration and unsupported-path handling are not implemented yet.

- [ ] **Step 3: Implement `NewAdapter` and metadata bootstrap**

Implement `NewAdapter` in `decibel/perp_adapter.go` so it:

- validates credentials
- creates REST, WebSocket, and Aptos clients
- loads market metadata at construction time
- caches symbol details on the base adapter

- [ ] **Step 4: Implement read methods and private watches**

Implement:

- `GetExchange`
- `GetMarketType`
- `FormatSymbol`
- `ExtractSymbol`
- `ListSymbols`
- `Close`
- `FetchTicker`
- `FetchOrderBook`
- `FetchFeeRate`
- `FetchAccount`
- `FetchBalance`
- `FetchPositions`
- `FetchSymbolDetails`
- `FetchOrderByID`
- `FetchOpenOrders`
- `FetchOrders` as far as Decibel provides a stable testable read path
- `WatchOrderBook`
- `WatchOrders`
- `StopWatchOrderBook`
- `StopWatchOrders`

- [ ] **Step 5: Implement place/cancel orchestration**

Implement:

- `PlaceOrder` using metadata lookup, precision validation, Aptos submission, and bounded order-ID reconciliation
- `CancelOrder`
- `CancelAllOrders` only if Decibel exposes a safe first-release path; otherwise return `ErrNotSupported`

- [ ] **Step 6: Implement the remaining interface surface explicitly**

Implement the remaining `Exchange`, `PerpExchange`, and `Streamable` methods explicitly so the adapter boundary is complete and deterministic:

- return `ErrNotSupported` for the agreed v1 unsupported surface such as `SetLeverage`, `ModifyOrder`, `FetchTrades`, `FetchKlines`, `FetchFundingRate`, `FetchAllFundingRates`, `WatchTicker`, `WatchTrades`, `WatchKlines`, and any matching stop-watch methods
- implement `WatchPositions` only if Decibel's private account stream is ready in the same phase; otherwise return `ErrNotSupported`
- make stop methods for supported watches idempotent and safe when no subscription is active
- cover the unsupported and stop-method boundaries in `unsupported_test.go`

- [ ] **Step 7: Run the targeted adapter tests to verify green**

Run: `go test ./decibel -run 'Test(DecibelAdapter|DecibelUnsupported)' -count=1`

Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add decibel/perp_adapter.go decibel/unsupported_test.go decibel/common.go decibel/sdk/rest/account.go decibel/sdk/rest/order.go
git commit -m "feat: integrate decibel perp adapter"
```

## Task 6: Add Live Tests, Env Wiring, And Repository Verification

**Files:**
- Create: `decibel/adapter_test.go`
- Modify: `.env.example`
- Verify: `docs/superpowers/specs/2026-03-24-decibel-perp-adapter-design.md`

- [ ] **Step 1: Write the failing live-test wiring**

Add `decibel/adapter_test.go` following repository conventions:

- `setupPerpAdapter` uses `testenv.RequireFull`
- required env vars are `DECIBEL_API_KEY`, `DECIBEL_PRIVATE_KEY`, `DECIBEL_SUBACCOUNT_ADDR`, `DECIBEL_PERP_TEST_SYMBOL`
- tests run `RunAdapterComplianceTests`, `RunOrderSuite`, and `RunLifecycleSuite`

- [ ] **Step 2: Run the targeted adapter test build to verify it fails**

Run: `go test ./decibel -run 'TestPerpAdapter_(Compliance|Orders|Lifecycle)$' -count=1`

Expected: the package builds; the live tests either fail before the final harness is complete or skip when `RUN_FULL` and Decibel env vars are not set.

- [ ] **Step 3: Document the new environment variables**

Update `.env.example` with the Decibel credentials and test symbol required by the live suite.

- [ ] **Step 4: Run the package unit/integration tests**

Run: `go test ./decibel/... -count=1`

Expected: PASS

- [ ] **Step 5: Run repository-level targeted verification**

Run: `go test ./decibel -run 'Test(DecibelOptions|DecibelRegistry|DecibelMetadata|DecibelREST|DecibelAptos|DecibelPrecision|DecibelWS|DecibelOrderBook|DecibelAdapter|DecibelUnsupported)' -count=1`

Expected: PASS

- [ ] **Step 6: Run live Decibel verification**

Run: `GOCACHE=/tmp/exchanges-gocache RUN_FULL=1 go test ./decibel -run 'TestPerpAdapter_(Compliance|Orders|Lifecycle)$' -count=1 -v`

Expected: PASS against a Decibel account configured with the required API key, Aptos private key, and subaccount.

- [ ] **Step 7: Commit**

```bash
git add decibel/adapter_test.go .env.example
git commit -m "test: add decibel live adapter coverage"
```

## Outcome

All six planned tasks were completed. The final implementation also incorporated follow-up review fixes:

- `FetchOrders` now uses order history rather than aliasing open orders
- private order streaming uses wallet-address `order_updates` plus `user_order_history`
- REST `open_orders` and `order_history` use `limit` / `offset` pagination
- single-order reconciliation uses `GET /api/v1/orders`
- timeout fallback no longer returns the Aptos transaction hash as a public stable order identifier
