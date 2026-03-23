# Bitget WS Orders Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add explicit `OrderModeWS` support to the Bitget adapter so UTA uses WS for place/cancel/modify and Classic uses WS for place/cancel, while preserving the existing REST path.

**Architecture:** Extend `bitget/sdk/private_ws.go` from a subscription-only client into a dual-purpose private socket client that also supports request/response trade RPC with ID correlation and timeouts. Keep account-mode branching inside the existing `privateProfile` implementations, expose protocol-specific WS trade helpers in the SDK, and route adapter order methods by `OrderMode` without silent REST fallback in WS mode.

**Tech Stack:** Go, Bitget private WebSocket APIs, existing `bitget` adapter/private profile structure, repository `BaseAdapter` order mode support, `go test`

---

## File Map

### Files To Modify

- `bitget/sdk/private_ws.go`
  - add pending-request registry, response correlation, timeout cleanup, and generic request send helpers while keeping current login/subscribe behavior intact
- `bitget/sdk/types.go`
  - add WS trade request/response wire types for UTA and Classic order RPC
- `bitget/private_uta.go`
  - route `PlaceOrder`, `CancelOrder`, and `ModifyOrder` through WS when `OrderModeWS` is selected
- `bitget/private_classic.go`
  - route `PlaceOrder` and `CancelOrder` through WS when `OrderModeWS` is selected; keep `ModifyOrder` on REST
- `bitget/perp_adapter.go`
  - add `WsOrderConnected(ctx)` and any small helpers needed for order-mode routing
- `bitget/spot_adapter.go`
  - add `WsOrderConnected(ctx)` and any small helpers needed for order-mode routing

### Files To Create

- `bitget/sdk/private_ws_trade_uta.go`
  - UTA place/cancel/modify request builders and ACK decoders
- `bitget/sdk/private_ws_trade_classic.go`
  - Classic spot and Classic perp place/cancel request builders and ACK decoders
- `bitget/sdk/private_ws_test.go`
  - correlation, timeout, and response-shape unit tests
- `bitget/ws_order_mode_test.go`
  - adapter/profile order-mode routing unit tests

### Files To Reference

- `base_adapter.go`
- `exchange.go`
- `bitget/private_profile.go`
- `binance/perp_adapter.go`
- `binance/sdk/perp/ws_api.go`
- `binance/sdk/perp/ws_order.go`

## Task 1: Add Failing Tests For Private WS Correlation

**Files:**
- Create: `bitget/sdk/private_ws_test.go`
- Modify: `bitget/sdk/private_ws.go`

- [ ] **Step 1: Write a failing test for top-level response ID correlation**

Add a test that seeds a pending request, feeds a mock UTA-style response with a top-level `id`, and expects the response bytes to be delivered to the matching waiter.

- [ ] **Step 2: Run the targeted test to verify it fails**

Run: `go test ./bitget/sdk -run TestPrivateWSDispatchesTopLevelID -count=1`

Expected: FAIL because the current client has no pending-request dispatch path.

- [ ] **Step 3: Write a failing test for Classic nested-ID correlation**

Add a test that feeds a Classic-style trade ACK with the request ID nested in the response payload and expects the matching waiter to receive it.

- [ ] **Step 4: Run the targeted test to verify it fails**

Run: `go test ./bitget/sdk -run TestPrivateWSDispatchesClassicNestedID -count=1`

Expected: FAIL because the current message handler ignores trade ACK responses.

- [ ] **Step 5: Write a failing test for timeout cleanup**

Add a test that sends a request through a helper with no response and asserts the pending entry is removed after timeout.

- [ ] **Step 6: Run the timeout test to verify it fails**

Run: `go test ./bitget/sdk -run TestPrivateWSRequestTimeoutCleansPending -count=1`

Expected: FAIL because no request/response helper exists yet.

- [ ] **Step 7: Commit the red tests**

```bash
git add bitget/sdk/private_ws_test.go
git commit -m "test: add bitget private ws correlation cases"
```

## Task 2: Implement Generic Private WS Request/Response Support

**Files:**
- Modify: `bitget/sdk/private_ws.go`

- [ ] **Step 1: Add pending-request storage to `PrivateWSClient`**

Implement:

- `pendingRequests map[string]chan []byte`
- `pendingMu sync.Mutex`
- initialization in `NewPrivateWSClient`

- [ ] **Step 2: Add ID extraction helpers**

Implement helpers that can extract:

- top-level `id`
- Classic nested response IDs from the trade ACK payload shape

Do not treat unknown or non-trade messages as fatal.

- [ ] **Step 3: Add a generic request sender with timeout**

Implement a helper such as `sendRequest(id string, req any) ([]byte, error)` that:

- registers the pending channel before write
- writes JSON
- waits for response or timeout
- always removes the pending entry on completion

- [ ] **Step 4: Update the read loop to dispatch responses**

Before subscription-specific decoding, attempt response-ID extraction and route trade ACKs to pending waiters.

- [ ] **Step 5: Run the targeted SDK tests to verify green**

Run: `go test ./bitget/sdk -run 'TestPrivateWSDispatchesTopLevelID|TestPrivateWSDispatchesClassicNestedID|TestPrivateWSRequestTimeoutCleansPending' -count=1`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add bitget/sdk/private_ws.go bitget/sdk/private_ws_test.go
git commit -m "feat: add bitget private ws request correlation"
```

## Task 3: Add Failing Tests For WS Order-Mode Routing

**Files:**
- Create: `bitget/ws_order_mode_test.go`
- Modify: `bitget/private_uta.go`
- Modify: `bitget/private_classic.go`
- Modify: `bitget/perp_adapter.go`
- Modify: `bitget/spot_adapter.go`

- [ ] **Step 1: Write a failing test for UTA place-order WS routing**

Create a small test seam with a stub private profile dependency or a WS-invocation seam that verifies:

- `OrderModeREST` keeps using the existing REST path
- `OrderModeWS` attempts the WS path

- [ ] **Step 2: Run the targeted test to verify it fails**

Run: `go test ./bitget -run TestUTAOrderModeWSRoutesPlaceOrderToWS -count=1`

Expected: FAIL because no WS trade route exists.

- [ ] **Step 3: Write a failing test for Classic place/cancel WS routing**

Add tests that verify:

- Classic spot `PlaceOrder` uses WS in `OrderModeWS`
- Classic perp `CancelOrder` uses WS in `OrderModeWS`

- [ ] **Step 4: Run the targeted Classic routing tests to verify they fail**

Run: `go test ./bitget -run 'TestClassicOrderModeWSRoutesPlaceOrderToWS|TestClassicOrderModeWSRoutesCancelOrderToWS' -count=1`

Expected: FAIL because Classic WS order RPC is not wired yet.

- [ ] **Step 5: Commit the red tests**

```bash
git add bitget/ws_order_mode_test.go
git commit -m "test: add bitget ws order mode routing cases"
```

## Task 4: Implement UTA WS Trade Helpers

**Files:**
- Create: `bitget/sdk/private_ws_trade_uta.go`
- Modify: `bitget/sdk/types.go`

- [ ] **Step 1: Add UTA WS trade request/response wire types**

Define exact wire structs for:

- place-order request and ACK
- cancel-order request and ACK
- modify-order request and ACK

Keep them separate from subscription payload types.

- [ ] **Step 2: Implement UTA WS place-order helper**

Create a helper that:

- builds a unique request ID
- serializes the UTA `op=trade` request shape
- calls the generic `sendRequest`
- decodes exchange-level ACK errors

- [ ] **Step 3: Implement UTA WS cancel-order helper**

Follow the same pattern for single-order cancel.

- [ ] **Step 4: Implement UTA WS modify-order helper**

Support ACK decoding only; the profile will still normalize final output via `FetchOrderByID`.

- [ ] **Step 5: Add focused decoder tests if needed**

If the generic tests do not fully exercise the UTA ACK decoder, add small unit tests in `bitget/sdk/private_ws_test.go`.

- [ ] **Step 6: Run targeted SDK tests**

Run: `go test ./bitget/sdk -run 'TestPrivateWS|TestUTAWS' -count=1`

Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add bitget/sdk/types.go bitget/sdk/private_ws_trade_uta.go bitget/sdk/private_ws_test.go
git commit -m "feat: add bitget uta ws trade helpers"
```

## Task 5: Implement Classic WS Trade Helpers

**Files:**
- Create: `bitget/sdk/private_ws_trade_classic.go`
- Modify: `bitget/sdk/types.go`

- [ ] **Step 1: Add Classic spot and perp WS trade wire types**

Define request/ACK types for:

- Classic spot place-order
- Classic spot cancel-order
- Classic perp place-order
- Classic perp cancel-order

Do not add Classic modify-order WS types in this task.

- [ ] **Step 2: Implement Classic spot WS trade helpers**

Use the documented `args[0].id`, `instType`, `instId`, `channel`, `params` shape.

- [ ] **Step 3: Implement Classic perp WS trade helpers**

Build the documented futures trade payload including margin and trade-side fields.

- [ ] **Step 4: Add Classic ACK decoder coverage**

Extend unit tests to verify that Classic ACK success and exchange-level error responses are decoded correctly.

- [ ] **Step 5: Run targeted SDK tests**

Run: `go test ./bitget/sdk -run 'TestPrivateWS|TestClassicWS' -count=1`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add bitget/sdk/types.go bitget/sdk/private_ws_trade_classic.go bitget/sdk/private_ws_test.go
git commit -m "feat: add bitget classic ws trade helpers"
```

## Task 6: Wire Adapter/Profile Order Methods To WS Mode

**Files:**
- Modify: `bitget/perp_adapter.go`
- Modify: `bitget/spot_adapter.go`
- Modify: `bitget/private_uta.go`
- Modify: `bitget/private_classic.go`
- Modify: `bitget/ws_order_mode_test.go`

- [ ] **Step 1: Add `WsOrderConnected(ctx)` helpers on both adapters**

These helpers should:

- require credentials
- ensure the private WS client is connected/logged in
- not subscribe to order streams

- [ ] **Step 2: Route UTA profile methods by `OrderMode`**

Implement:

- `PlaceOrder`: REST or UTA WS
- `CancelOrder`: REST or UTA WS
- `ModifyOrder`: REST or UTA WS, followed by order fetch normalization

Keep `CancelAllOrders` on REST.

- [ ] **Step 3: Route Classic profile methods by `OrderMode`**

Implement:

- `PlaceOrder`: REST or Classic WS
- `CancelOrder`: REST or Classic WS

Keep:

- `ModifyOrder` on REST
- `CancelAllOrders` on REST

- [ ] **Step 4: Make the routing tests pass**

Run: `go test ./bitget -run 'TestUTAOrderModeWSRoutesPlaceOrderToWS|TestClassicOrderModeWSRoutesPlaceOrderToWS|TestClassicOrderModeWSRoutesCancelOrderToWS' -count=1`

Expected: PASS

- [ ] **Step 5: Add one guard test for no silent fallback**

Add a test that sets `OrderModeWS`, simulates a missing/failed WS trade path, and asserts the method returns an error instead of dropping to REST.

- [ ] **Step 6: Run the guard test**

Run: `go test ./bitget -run TestBitgetWSOrderModeDoesNotSilentlyFallbackToREST -count=1`

Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add bitget/perp_adapter.go bitget/spot_adapter.go bitget/private_uta.go bitget/private_classic.go bitget/ws_order_mode_test.go
git commit -m "feat: route bitget order mode to ws transport"
```

## Task 7: Run Package Verification

**Files:**
- Modify as needed: files from earlier tasks

- [ ] **Step 1: Run formatting if needed**

Run: `gofmt -w bitget/sdk/private_ws.go bitget/sdk/private_ws_trade_uta.go bitget/sdk/private_ws_trade_classic.go bitget/sdk/types.go bitget/private_uta.go bitget/private_classic.go bitget/perp_adapter.go bitget/spot_adapter.go bitget/sdk/private_ws_test.go bitget/ws_order_mode_test.go`

- [ ] **Step 2: Run diff hygiene**

Run: `git diff --check`

Expected: no whitespace or conflict-marker issues

- [ ] **Step 3: Run Bitget package tests**

Run: `GOCACHE=/tmp/gocache-bitget go test ./bitget/...`

Expected: PASS

- [ ] **Step 4: Commit any final fixes**

```bash
git add bitget
git commit -m "test: verify bitget ws order transport"
```

## Task 8: Run Live Validation In WS Mode

**Files:**
- Modify: `bitget/adapter_test.go` only if a small helper is needed to force `OrderModeWS` in selected live tests

- [ ] **Step 1: Add a targeted live-test seam for WS order mode if needed**

If the current live tests always use the default order mode, add a narrow helper so selected order/lifecycle tests can run with `SetOrderMode(exchanges.OrderModeWS)` on the constructed adapter.

- [ ] **Step 2: Run targeted spot live validation in WS mode**

Run: `GOCACHE=/tmp/gocache-bitget go test ./bitget -run 'TestSpotAdapter_(Orders|Lifecycle)' -count=1 -v`

Expected: PASS with WS order transport active

- [ ] **Step 3: Run targeted perp live validation in WS mode**

Run: `GOCACHE=/tmp/gocache-bitget go test ./bitget -run 'TestPerpAdapter_(Orders|Lifecycle)' -count=1 -v`

Expected: PASS with WS order transport active

- [ ] **Step 4: If Bitget rejects WS trade permissions, report it explicitly**

Document the exact exchange error and leave REST behavior unchanged. Do not hide the failure by switching the tests back to REST.

- [ ] **Step 5: Commit live-test wiring only if code changed**

```bash
git add bitget/adapter_test.go
git commit -m "test: exercise bitget ws order mode live"
```
