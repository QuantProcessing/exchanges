# Fetch Order Semantics Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the ambiguous shared `FetchOrder` contract with explicit `FetchOrderByID`, `FetchOrders`, and `FetchOpenOrders` semantics across the repository.

**Architecture:** Make this a deliberate breaking change in the shared `Exchange` interface, then migrate adapters and internal call sites to the new names and semantics. Add a dedicated shared suite for order-query semantics so terminal lookup support, broader order-list support, and explicit `ErrNotSupported` behavior are verified separately from the existing order-placement suite.

**Tech Stack:** Go, shared adapter interfaces in `exchange.go`, adapter packages, shared `testsuite`, live adapter integration tests

---

### Task 1: Change The Shared Interface And Test Surface

**Files:**
- Modify: `exchange.go`
- Modify: `testsuite/order_suite.go`
- Create: `testsuite/order_query_suite.go`
- Read: `errors.go`
- Read: `testsuite/helpers.go`

- [ ] **Step 1: Write the failing shared-contract test expectations**

Define the new expected shared trading methods before touching code:

- `FetchOrderByID(ctx, orderID, symbol string) (*Order, error)`
- `FetchOrders(ctx, symbol string) ([]Order, error)`
- `FetchOpenOrders(ctx, symbol string) ([]Order, error)`

Expected failure: the current `Exchange` interface still exposes `FetchOrder` and lacks `FetchOrders`.

- [ ] **Step 2: Update the shared interface in `exchange.go`**

Change the trading section to:

```go
PlaceOrder(ctx context.Context, params *OrderParams) (*Order, error)
CancelOrder(ctx context.Context, orderID, symbol string) error
CancelAllOrders(ctx context.Context, symbol string) error
FetchOrderByID(ctx context.Context, orderID, symbol string) (*Order, error)
FetchOrders(ctx context.Context, symbol string) ([]Order, error)
FetchOpenOrders(ctx context.Context, symbol string) ([]Order, error)
```

- [ ] **Step 3: Create the new shared suite skeleton**

Add `testsuite/order_query_suite.go` with:

- `OrderQueryConfig`
- `RunOrderQuerySemanticsSuite`

Config must at least carry:

- `Symbol string`
- `SupportsTerminalLookup bool`
- `SupportsOrderHistory bool`

- [ ] **Step 4: Write the semantic test flow in the new suite**

The suite should:

- place an order
- wait for terminal state
- call `FetchOrderByID`
- call `FetchOrders`
- call `FetchOpenOrders`
- assert one of two expected paths:
  - supported path: terminal lookup and broader order list work
  - honest limitation path: `errors.Is(err, exchanges.ErrNotSupported)`
- always assert that `FetchOpenOrders` does not include terminal orders
- on supported adapters, assert that `FetchOrders` is broader than the open-only set when the exchange exposes broader history

- [ ] **Step 5: Run targeted tests to verify the new suite compiles**

Run: `go test ./testsuite -run TestDoesNotExist`

Expected: package compiles; no unresolved interface names.

- [ ] **Step 6: Commit the shared-contract change**

```bash
git add exchange.go testsuite/order_suite.go testsuite/order_query_suite.go
git commit -m "refactor: add explicit order query interfaces"
```

### Task 2: Migrate Shared Call Sites And High-Risk Adapters

**Files:**
- Modify: `aster/perp_adapter.go`
- Modify: `aster/spot_adapter.go`
- Modify: `backpack/perp_adapter.go`
- Modify: `backpack/spot_adapter.go`
- Modify: `binance/perp_adapter.go`
- Modify: `backpack/perp_adapter.go`
- Modify: `backpack/spot_adapter.go`
- Modify: `binance/spot_adapter.go`
- Modify: `binance/margin_adapter.go`
- Modify: `edgex/perp_adapter.go`
- Modify: `grvt/perp_adapter.go`
- Modify: `hyperliquid/perp_adapter.go`
- Modify: `hyperliquid/spot_adapter.go`
- Modify: `lighter/perp_adapter.go`
- Modify: `lighter/spot_adapter.go`
- Modify: `nado/perp_adapter.go`
- Modify: `nado/spot_adapter.go`
- Modify: `okx/perp_adapter.go`
- Modify: `okx/spot_adapter.go`
- Modify: `standx/perp_adapter.go`
- Modify: `grvt/perp_adapter.go`
- Modify: any other adapter file containing `FetchOrder(`
- Read: `backpack/sdk/`, `grvt/sdk/`, and relevant peer SDK order endpoints

- [ ] **Step 1: Write the failing migration inventory**

Run:

```bash
rg -n "FetchOrder\\(" -g'*.go'
```

Expected: adapters and internal call sites still use the old method name.

- [ ] **Step 2: Rename every shared implementation to `FetchOrderByID`**

Update every listed adapter method implementing the shared interface from:

```go
func (a *Adapter) FetchOrder(ctx context.Context, orderID, symbol string) ...
```

to:

```go
func (a *Adapter) FetchOrderByID(ctx context.Context, orderID, symbol string) ...
```

This step is not complete until every `FetchOrder(` implementation found in Step 1 has been migrated or intentionally removed from the shared surface.

- [ ] **Step 3: Add `FetchOrders` to every adapter**

For each migrated adapter:

- use a history/list endpoint when available
- return symbol-filtered results
- return `exchanges.ErrNotSupported` when the adapter only has open-order visibility

- [ ] **Step 4: Tighten `FetchOrderByID` semantics in degraded adapters**

Specifically fix adapters that currently scan open orders only, including at least:

- `backpack/perp_adapter.go`
- `backpack/spot_adapter.go`
- `grvt/perp_adapter.go`

Rule:

- if the adapter cannot resolve terminal orders with any low-level path, return `exchanges.ErrNotSupported`
- do not return `exchanges.ErrOrderNotFound` just because the order is no longer open

- [ ] **Step 5: Preserve symbol filtering and order mapping semantics**

For adapters that must filter account-wide results or normalize wire payloads:

- verify `FetchOrders` and `FetchOpenOrders` both filter by the requested base symbol
- keep adapter-local order mapping coherent between single-order and list-order paths
- add helper reuse only when it reduces divergence instead of hiding different semantics

- [ ] **Step 6: Migrate internal call sites to `FetchOrderByID`**

Update modify/recover flows such as:

- `binance/spot_adapter.go`
- `hyperliquid/perp_adapter.go`
- `hyperliquid/spot_adapter.go`
- `lighter/spot_adapter.go`

Check whether these flows require stronger behavior than an `ErrNotSupported` adapter can provide. If they do, make the limitation explicit instead of silently degrading behavior.

- [ ] **Step 7: Add modify/replace regression coverage or explicit limitation checks**

For adapters that call `FetchOrderByID` inside amend/replace logic:

- add deterministic tests where feasible to prove `ModifyOrder` still works with the new lookup semantics
- if a flow must now fail because the adapter honestly lacks terminal lookup support, add a deterministic assertion for the explicit failure path instead of leaving it implicit

- [ ] **Step 8: Include Binance margin in the interface migration**

Update `binance/margin_adapter.go` to compile against the new interface and implement the renamed method plus `FetchOrders`.

- [ ] **Step 9: Run targeted compile checks on all migrated packages**

Run:

```bash
go test ./aster ./backpack ./binance ./edgex ./grvt ./hyperliquid ./lighter ./nado ./okx ./standx -run TestDoesNotExist
```

Expected: packages compile with the new interface and renamed methods.

- [ ] **Step 10: Commit the adapter migration**

```bash
git add aster backpack binance edgex grvt hyperliquid lighter nado okx standx exchange.go
git commit -m "refactor: migrate adapters to explicit order queries"
```

### Task 3: Wire The New Shared Suite Into Adapter Tests

**Files:**
- Modify: `backpack/adapter_test.go`
- Modify: `binance/adapter_test.go`
- Modify: `okx/adapter_test.go`
- Modify: `aster/adapter_test.go`
- Modify: `nado/adapter_test.go`
- Modify: `lighter/adapter_test.go`
- Modify: `hyperliquid/adapter_test.go`
- Modify: `grvt/adapter_test.go`
- Modify: `standx/adapter_test.go`
- Modify: `edgex/adapter_test.go`

- [ ] **Step 1: Write the failing test-wiring inventory**

Run:

```bash
rg -n "RunOrderSuite|RunLifecycleSuite|RunLocalStateSuite" */adapter_test.go
```

Expected: no adapter test file wires the new order-query semantics suite yet.

- [ ] **Step 2: Add `RunOrderQuerySemanticsSuite` to live-capable adapter tests**

Wire the new suite into each relevant adapter test file.

For each adapter/market, set:

- `SupportsTerminalLookup` truthfully
- `SupportsOrderHistory` truthfully

Do not claim support just to make tests pass.

- [ ] **Step 3: Exclude Binance margin from new live-suite wiring**

Do not invent a new margin live test path in this refactor. Margin must compile and can get deterministic coverage separately, but it is outside the new shared live-suite wiring unless a margin-specific live path is added.

- [ ] **Step 4: Run a targeted compile check on adapter tests**

Run:

```bash
go test ./... -run TestDoesNotExist
```

Expected: all packages compile with the new test suite references.

- [ ] **Step 5: Commit the shared-suite wiring**

```bash
git add testsuite */adapter_test.go
git commit -m "test: add order query semantics suite"
```

### Task 4: Add Deterministic Coverage For Honest Limitation Paths

**Files:**
- Modify or create adapter-local tests near changed adapters, especially:
- `backpack/*_test.go`
- `grvt/*_test.go`
- `binance/*_test.go`
- `hyperliquid/*_test.go`
- `lighter/*_test.go`

- [ ] **Step 1: Write failing deterministic tests for limitation semantics**

Add focused tests for:

- `FetchOrderByID` limitation returns `exchanges.ErrNotSupported` when terminal lookup is unsupported
- `FetchOrders` limitation returns `exchanges.ErrNotSupported` when only open orders exist
- `errors.Is(err, exchanges.ErrOrderNotFound)` still works for real missing-order lookup where supported
- symbol filtering stays correct when low-level APIs return account-wide order lists
- adapter-local order mapping stays consistent between single-order and list-order paths where shared helpers are introduced

- [ ] **Step 2: Implement or adjust adapter behavior to satisfy those tests**

Keep changes minimal and aligned with the spec:

- capability absence => `ErrNotSupported`
- real lookup miss on a supported surface => `ErrOrderNotFound`

- [ ] **Step 3: Run the targeted deterministic tests**

Run the exact adapter-local test commands you added in Step 1.

Expected: new tests pass and prove the distinction between unsupported and not-found.

- [ ] **Step 4: Commit deterministic regression coverage**

```bash
git add backpack grvt binance hyperliquid lighter
git commit -m "test: cover order query limitation semantics"
```

### Task 5: Run End-To-End Verification

**Files:**
- Read: `docs/superpowers/specs/2026-03-22-fetch-order-semantics-design.md`
- Modify: `README.md`
- Modify: `README_CN.md`

- [ ] **Step 1: Run repository-wide compile verification**

Run:

```bash
go test ./... -run TestDoesNotExist
```

Expected: the full repository compiles against the new interface.

- [ ] **Step 2: Update shared interface docs**

Update shared-order-interface references in:

- `README.md`
- `README_CN.md`

Replace `FetchOrder` descriptions with the new three-method split where the shared interface is documented.

- [ ] **Step 3: Run the new shared suite on at least the changed live adapters that have credentials configured**

At minimum run targeted live commands for adapters you changed deeply and can verify safely in this environment.

Expected: the semantic split is exercised and either supported or honestly returns `ErrNotSupported`.

Include at least one adapter with known terminal lookup support and one adapter with an honest limitation path if both are available in the configured environment.

- [ ] **Step 4: Run final hygiene checks**

Run:

```bash
git diff --check
rg -n "FetchOrder\\(" -g'*.go'
rg -n "FetchOrderByID\\(" -g'*.go'
rg -n "FetchOrders\\(" -g'*.go'
```

Expected:

- no whitespace issues
- no remaining shared-interface `FetchOrder(` uses
- the new names are present where expected

- [ ] **Step 5: Re-read the spec and do a checklist pass**

Verify all of these are true:

- shared interface exposes all three methods
- adapters compile and migrated call sites use the new names
- `FetchOrderByID` and `FetchOrders` distinguish unsupported from not-found
- deterministic tests cover unsupported-vs-not-found, symbol filtering, and mapping-sensitive paths
- shared tests assert that `FetchOpenOrders` excludes terminal orders and that `FetchOrders` differs from open-only results where supported
- the new shared suite exists and is wired into trading adapter tests
- Binance margin is migrated but not force-fit into live shared-suite wiring
- shared docs no longer describe the old `FetchOrder` interface

- [ ] **Step 6: Commit final refinements**

```bash
git add .
git commit -m "refactor: finalize explicit order query semantics"
```
