# Exchange Adapter Naming Convergence Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Converge repository-preferred SDK naming for Backpack query/order methods and `WsClient`-style websocket base clients without breaking existing call sites.

**Architecture:** Treat this as a compatibility-first naming pass. Backpack remains the already-landed precedent for `GetOrderBook` and `PlaceOrder`; the implementation work there is limited to regression verification and any tiny cleanup needed to keep the compatibility wrappers honest. The substantive code changes are the `WsClient` to `WSClient` renames in Aster, Binance, OKX, and StandX, implemented as concrete-type renames plus compatibility aliases and constructor wrappers. Companion docs are updated in the same rollout so the layering baseline, Backpack gap doc, and checklist all tell the same story.

**Tech Stack:** Go, repository exchange SDK packages, markdown docs under `docs/superpowers`

---

## Planned File Map

- Modify: `docs/superpowers/specs/2026-03-23-exchange-adapter-layering-design.md`
  Purpose: narrow the websocket naming deferred item to the remaining non-`WsClient` families and record Backpack naming as already-landed precedent.
- Modify: `docs/superpowers/checklists/exchange-adapter-review.md`
  Purpose: keep checklist language aligned with the updated layering baseline.
- Modify: `docs/superpowers/gaps/2026-03-23-backpack-adapter-gap.md`
  Purpose: record Backpack as the landed precedent for preferred SDK query/order naming and remove the stale WS-naming deferred wording.
- Modify: `backpack/sdk/public_rest_test.go`
- Modify: `backpack/sdk/private_rest_test.go`
- Modify: `backpack/constructor_test.go`
  Purpose: ensure regression coverage for `GetDepth -> GetOrderBook` and `ExecuteOrder -> PlaceOrder` compatibility wrappers uses the preferred names as primary entrypoints.
- Modify: `aster/sdk/perp/ws_client.go`
- Modify: `aster/sdk/perp/ws_account.go`
- Modify: `aster/sdk/perp/ws_market.go`
- Modify: `aster/sdk/spot/ws_client.go`
- Modify: `aster/sdk/spot/ws_account.go`
- Modify: `aster/sdk/spot/ws_market.go`
  Purpose: rename the concrete websocket base client type to `WSClient`, keep `WsClient` alias/wrapper compatibility, and update internal references.
- Modify: `binance/sdk/perp/ws_client.go`
- Modify: `binance/sdk/perp/ws_account.go`
- Modify: `binance/sdk/perp/ws_market.go`
- Modify: `binance/sdk/spot/ws_client.go`
- Modify: `binance/sdk/spot/ws_market.go`
  Purpose: same websocket naming convergence for Binance SDK packages.
- Modify: `okx/sdk/ws_client.go`
- Modify: `okx/sdk/ws_account.go`
- Modify: `okx/sdk/ws_market.go`
- Modify: `okx/sdk/ws_order.go`
- Modify: `okx/perp_adapter.go`
- Modify: `okx/spot_adapter.go`
  Purpose: rename the concrete websocket client type to `WSClient`, add compatibility alias/wrapper, and update adapters plus SDK references.
- Modify: `standx/sdk/ws_client.go`
- Modify: `standx/sdk/ws_market.go`
- Modify: `standx/sdk/ws_account.go`
- Modify: `standx/sdk/ws_api.go`
  Purpose: rename the concrete websocket base client type to `WSClient`, add compatibility alias/wrapper, and update internal references.

### Task 1: Align Companion Docs With The Naming-Convergence Spec

**Files:**
- Modify: `docs/superpowers/specs/2026-03-23-exchange-adapter-layering-design.md`
- Modify: `docs/superpowers/checklists/exchange-adapter-review.md`
- Modify: `docs/superpowers/gaps/2026-03-23-backpack-adapter-gap.md`

- [ ] **Step 1: Write the failing documentation review notes**

Record the specific stale statements to remove before touching the docs:

- layering spec still treats websocket naming normalization as one fully-open repository-wide decision
- Backpack gap doc still says `WSClient` naming stays deferred
- checklist does not yet mention that Backpack is the landed precedent for SDK query/order naming

- [ ] **Step 2: Update the layering baseline**

Edit `docs/superpowers/specs/2026-03-23-exchange-adapter-layering-design.md` so it:

- records Backpack as the landed precedent for `GetOrderBook` and `PlaceOrder`
- narrows websocket naming deferral to the remaining non-`WsClient` families
- keeps `WebsocketClient`, `BaseWsClient`, and `WsApiClient` explicitly deferred

- [ ] **Step 3: Update the checklist companion**

Edit `docs/superpowers/checklists/exchange-adapter-review.md` so it points reviewers at the updated baseline wording and does not imply Backpack still has unresolved query/order naming drift.

- [ ] **Step 4: Update the Backpack gap doc**

Edit `docs/superpowers/gaps/2026-03-23-backpack-adapter-gap.md` so it:

- describes Backpack as the already-landed precedent for preferred SDK query/order naming
- removes the stale `WSClient` naming deferred wording
- does not imply fresh Backpack SDK renaming work is pending

- [ ] **Step 5: Commit the companion-doc alignment**

```bash
git add docs/superpowers/specs/2026-03-23-exchange-adapter-layering-design.md docs/superpowers/checklists/exchange-adapter-review.md docs/superpowers/gaps/2026-03-23-backpack-adapter-gap.md
git commit -m "docs: align adapter naming baseline"
```

### Task 2: Lock In Backpack Naming As Regression-Covered Precedent

**Files:**
- Modify: `backpack/sdk/public_rest_test.go`
- Modify: `backpack/sdk/private_rest_test.go`
- Modify: `backpack/constructor_test.go`

- [ ] **Step 1: Write or adjust the Backpack naming regression tests**

Ensure there is deterministic coverage for both compatibility families:

```go
func TestClientGetDepthDelegatesToGetOrderBook(t *testing.T) {}
func TestClientExecuteOrderDelegatesToPlaceOrder(t *testing.T) {}
```

Also update any helper interfaces or stubs in `backpack/constructor_test.go` so the preferred names are the primary methods and the old names are not treated as canonical.

- [ ] **Step 2: Run the targeted Backpack naming tests**

Run: `go test ./backpack/... -run 'TestClient(GetDepthDelegatesToGetOrderBook|ExecuteOrderDelegatesToPlaceOrder|GetOrderBookDelegatesToDepthEndpoint|PlaceOrderDelegatesToExistingOrderExecutionPath)' -v`
Expected: PASS

- [ ] **Step 3: Commit the Backpack regression verification**

```bash
git add backpack/sdk/public_rest_test.go backpack/sdk/private_rest_test.go backpack/constructor_test.go
git commit -m "test: lock backpack naming compatibility"
```

### Task 3: Rename Aster `WsClient` To `WSClient` With Compatibility Aliases

**Files:**
- Modify: `aster/sdk/perp/ws_client.go`
- Modify: `aster/sdk/perp/ws_account.go`
- Modify: `aster/sdk/perp/ws_market.go`
- Modify: `aster/sdk/perp/ws_market_test.go`
- Create or Modify: `aster/sdk/perp/ws_client_test.go`
- Modify: `aster/sdk/spot/ws_client.go`
- Modify: `aster/sdk/spot/ws_account.go`
- Modify: `aster/sdk/spot/ws_market.go`
- Create or Modify: `aster/sdk/spot/ws_client_test.go`

- [ ] **Step 1: Write the failing compatibility tests**

Add tests in the existing websocket-client test files or new narrow tests that assert both constructor names still work:

```go
func TestNewWSClientAndNewWsClientReturnCompatibleTypes(t *testing.T) {}
```

- [ ] **Step 2: Run the targeted Aster websocket tests to verify the pre-change failure**

Run: `go test ./aster/... -run 'TestNewWSClientAndNewWsClientReturnCompatibleTypes' -v`
Expected: FAIL because `NewWSClient` and `WSClient` do not exist yet.

- [ ] **Step 3: Rename the concrete type and add compatibility wrappers**

In both `aster/sdk/perp/ws_client.go` and `aster/sdk/spot/ws_client.go`:

- rename the concrete type to `WSClient`
- rename the primary constructor to `NewWSClient`
- add:

```go
type WsClient = WSClient

func NewWsClient(ctx context.Context, url string) *WSClient {
	return NewWSClient(ctx, url)
}
```

- [ ] **Step 4: Update internal references to prefer the new names**

Change `ws_account.go`, `ws_market.go`, and any related tests to use `WSClient` and `NewWSClient` as the primary names.

- [ ] **Step 5: Run the targeted Aster package tests**

Run: `go test ./aster/... -run 'Test(NewWSClientAndNewWsClientReturnCompatibleTypes|TestOptions_|TestPerpPrivatePathsWithoutCredentialsReturnAuthSentinel|TestSpotPrivatePathsWithoutCredentialsReturnAuthSentinel)' -v`
Expected: PASS

- [ ] **Step 6: Commit the Aster websocket naming change**

```bash
git add aster/sdk/perp/ws_client.go aster/sdk/perp/ws_account.go aster/sdk/perp/ws_market.go aster/sdk/perp/ws_market_test.go aster/sdk/perp/ws_client_test.go aster/sdk/spot/ws_client.go aster/sdk/spot/ws_account.go aster/sdk/spot/ws_market.go aster/sdk/spot/ws_client_test.go
git commit -m "refactor: normalize aster ws client naming"
```

### Task 4: Rename Binance `WsClient` To `WSClient` With Compatibility Aliases

**Files:**
- Modify: `binance/sdk/perp/ws_client.go`
- Modify: `binance/sdk/perp/ws_account.go`
- Modify: `binance/sdk/perp/ws_market.go`
- Modify: `binance/sdk/perp/ws_market_test.go`
- Modify: `binance/sdk/spot/ws_client.go`
- Modify: `binance/sdk/spot/ws_market.go`
- Modify: `binance/sdk/spot/ws_client_test.go`

- [ ] **Step 1: Write the failing compatibility tests**

Add compatibility tests mirroring the Aster pattern:

```go
func TestPerpNewWSClientAndNewWsClientReturnCompatibleTypes(t *testing.T) {}
func TestSpotNewWSClientAndNewWsClientReturnCompatibleTypes(t *testing.T) {}
```

- [ ] **Step 2: Run the targeted Binance websocket tests to verify the pre-change failure**

Run: `go test ./binance/... -run 'Test(Perp|Spot)NewWSClientAndNewWsClientReturnCompatibleTypes' -v`
Expected: FAIL because `NewWSClient` and `WSClient` do not exist yet.

- [ ] **Step 3: Rename the concrete type and add compatibility wrappers**

Apply the same pattern as Aster in both `binance/sdk/perp/ws_client.go` and `binance/sdk/spot/ws_client.go`.

- [ ] **Step 4: Update internal references**

Switch Binance SDK references to prefer `WSClient` and `NewWSClient`.

- [ ] **Step 5: Run the targeted Binance package tests**

Run: `go test ./binance/... -run 'Test(Perp|Spot)NewWSClientAndNewWsClientReturnCompatibleTypes' -v`
Expected: PASS

- [ ] **Step 6: Commit the Binance websocket naming change**

```bash
git add binance/sdk/perp/ws_client.go binance/sdk/perp/ws_account.go binance/sdk/perp/ws_market.go binance/sdk/perp/ws_market_test.go binance/sdk/spot/ws_client.go binance/sdk/spot/ws_market.go binance/sdk/spot/ws_client_test.go
git commit -m "refactor: normalize binance ws client naming"
```

### Task 5: Rename OKX `WsClient` To `WSClient` With Compatibility Aliases

**Files:**
- Modify: `okx/sdk/ws_client.go`
- Modify: `okx/sdk/ws_account.go`
- Modify: `okx/sdk/ws_market.go`
- Modify: `okx/sdk/ws_order.go`
- Modify: `okx/sdk/ws_market_test.go`
- Modify: `okx/sdk/ws_order_test.go`
- Modify: `okx/perp_adapter.go`
- Modify: `okx/spot_adapter.go`

- [ ] **Step 1: Write the failing compatibility test**

Add:

```go
func TestNewWSClientAndNewWsClientReturnCompatibleTypes(t *testing.T) {}
```

- [ ] **Step 2: Run the targeted OKX websocket test to verify the pre-change failure**

Run: `go test ./okx/... -run 'TestNewWSClientAndNewWsClientReturnCompatibleTypes' -v`
Expected: FAIL because `NewWSClient` and `WSClient` do not exist yet.

- [ ] **Step 3: Rename the concrete type and add compatibility wrappers**

In `okx/sdk/ws_client.go`:

- rename the concrete type to `WSClient`
- rename the primary constructor to `NewWSClient`
- add:

```go
type WsClient = WSClient

func NewWsClient(ctx context.Context) *WSClient {
	return NewWSClient(ctx)
}
```

- [ ] **Step 4: Update adapters and SDK references**

Change `okx/perp_adapter.go`, `okx/spot_adapter.go`, `okx/sdk/ws_account.go`, `okx/sdk/ws_market.go`, `okx/sdk/ws_order.go`, and tests to prefer `WSClient`.

- [ ] **Step 5: Run the targeted OKX package tests**

Run: `go test ./okx/... -run 'TestNewWSClientAndNewWsClientReturnCompatibleTypes' -v`
Expected: PASS

- [ ] **Step 6: Commit the OKX websocket naming change**

```bash
git add okx/sdk/ws_client.go okx/sdk/ws_account.go okx/sdk/ws_market.go okx/sdk/ws_order.go okx/sdk/ws_market_test.go okx/sdk/ws_order_test.go okx/perp_adapter.go okx/spot_adapter.go
git commit -m "refactor: normalize okx ws client naming"
```

### Task 6: Rename StandX `WsClient` To `WSClient` With Compatibility Aliases

**Files:**
- Modify: `standx/sdk/ws_client.go`
- Modify: `standx/sdk/ws_market.go`
- Modify: `standx/sdk/ws_account.go`
- Modify: `standx/sdk/ws_api.go`
- Create or Modify: `standx/sdk/ws_client_test.go`

- [ ] **Step 1: Write the failing compatibility test**

Add:

```go
func TestNewWSClientAndNewWsClientReturnCompatibleTypes(t *testing.T) {}
```

- [ ] **Step 2: Run the targeted StandX websocket test to verify the pre-change failure**

Run: `go test ./standx/... -run 'TestNewWSClientAndNewWsClientReturnCompatibleTypes' -v`
Expected: FAIL because `NewWSClient` and `WSClient` do not exist yet.

- [ ] **Step 3: Rename the concrete type and add compatibility wrappers**

In `standx/sdk/ws_client.go`:

- rename the concrete type to `WSClient`
- rename the primary constructor to `NewWSClient`
- add:

```go
type WsClient = WSClient

func NewWsClient(ctx context.Context, url string, logger *zap.SugaredLogger) *WSClient {
	return NewWSClient(ctx, url, logger)
}
```

- [ ] **Step 4: Update internal references**

Change `standx/sdk/ws_market.go`, `standx/sdk/ws_account.go`, and `standx/sdk/ws_api.go` to prefer `WSClient`.

- [ ] **Step 5: Run the targeted StandX package tests**

Run: `go test ./standx/... -run 'TestNewWSClientAndNewWsClientReturnCompatibleTypes' -v`
Expected: PASS

- [ ] **Step 6: Commit the StandX websocket naming change**

```bash
git add standx/sdk/ws_client.go standx/sdk/ws_market.go standx/sdk/ws_account.go standx/sdk/ws_api.go standx/sdk/ws_client_test.go
git commit -m "refactor: normalize standx ws client naming"
```

### Task 7: Final Verification And Rollout Status Update

**Files:**
- Modify: `docs/superpowers/specs/2026-03-23-exchange-adapter-layering-design.md`
- Modify: `docs/superpowers/checklists/exchange-adapter-review.md`
- Modify: `docs/superpowers/gaps/2026-03-23-backpack-adapter-gap.md`

- [ ] **Step 1: Run the focused naming verification**

Run: `go test ./backpack/... ./aster/sdk/... ./binance/sdk/... ./okx/sdk ./standx/sdk -v`
Expected: PASS

- [ ] **Step 2: Run repository compile verification**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 3: Re-read the companion docs and ensure they match the landed state**

Confirm:

- Backpack is described as precedent rather than pending work
- `WsClient`-family naming convergence is recorded as landed for the touched packages
- remaining non-`WsClient` naming families remain explicitly deferred

- [ ] **Step 4: Commit the final rollout state**

```bash
git add docs/superpowers/specs/2026-03-23-exchange-adapter-layering-design.md docs/superpowers/checklists/exchange-adapter-review.md docs/superpowers/gaps/2026-03-23-backpack-adapter-gap.md
git commit -m "docs: finalize adapter naming convergence rollout"
```
