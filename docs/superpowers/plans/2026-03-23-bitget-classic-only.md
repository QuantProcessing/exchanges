# Bitget Classic-Only Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove all Bitget UTA and account-mode support so the `bitget` package only supports the classic private API surface while preserving public market data and classic REST/WS trading.

**Architecture:** Delete the UTA profile and UTA WS helper entirely, remove account-mode branching from constructors and shared helpers, and make the adapters instantiate classic private profiles directly. Keep the public/shared SDK pieces that still serve classic mode, preserve explicit default `OrderModeREST`, and update tests so they only cover classic-only semantics.

**Tech Stack:** Go, existing `bitget` adapter and SDK, `go test`, live Bitget classic credentials

---

## File Map

### Files To Delete

- `bitget/private_uta.go`
- `bitget/sdk/private_ws_trade_uta.go`

### Files To Modify

- `bitget/options.go`
  - remove `AccountMode`
  - remove `accountMode()` helper
- `bitget/common.go`
  - remove account-mode constants and detection helpers
  - keep only shared auth/public/parsing helpers still used by classic mode
- `bitget/private_profile.go`
  - remove mode constants and mode switching
  - always build classic private profiles
- `bitget/perp_adapter.go`
  - remove `accountMode` field
  - stop calling private account-mode detection
  - instantiate classic private profile directly
- `bitget/spot_adapter.go`
  - same simplification as perp adapter
- `bitget/order_request.go`
  - remove any remaining logic that branches on `accountMode`
- `bitget/private_init_test.go`
  - rewrite to classic-only initialization expectations
- `bitget/ws_order_mode_test.go`
  - remove UTA routing tests
  - keep classic WS routing and default REST coverage
- `bitget/adapter_test.go`
  - remove wording that implies UTA support

### Files To Verify For Dead Code

- `bitget/sdk/private_rest.go`
- `bitget/sdk/types.go`
- `bitget/register.go`
- `.env.example`

## Task 1: Remove Account-Mode API Surface

**Files:**
- Modify: `bitget/options.go`
- Modify: `bitget/common.go`
- Modify: `bitget/private_profile.go`
- Test: `bitget/private_init_test.go`

- [ ] **Step 1: Write the failing tests for classic-only initialization**

Replace UTA/auto-detect initialization tests with classic-only expectations:

- constructors still allow public-only init
- partial credentials still fail with `ErrAuthFailed`
- constructors no longer expose or depend on `accountMode`

- [ ] **Step 2: Run the targeted init tests to verify they fail**

Run: `go test ./bitget -run 'TestNew(Spot|Perp)Adapter' -count=1`

Expected: FAIL because the tests still reference account-mode logic that has not yet been removed.

- [ ] **Step 3: Remove `Options.AccountMode` and account-mode helpers**

Delete `AccountMode` from `bitget/options.go` and remove its parser.

- [ ] **Step 4: Remove account-mode detection from shared helpers**

Delete the UTA/classic detection helpers from `bitget/common.go`, preserving only auth and shared parsing code still needed by classic mode.

- [ ] **Step 5: Collapse `private_profile.go` to classic-only constructors**

Remove mode constants and branching. `newPerpPrivateProfile` should always return `classicPerpProfile`; `newSpotPrivateProfile` should always return `classicSpotProfile`; `newPrivateWSClient` should always enable classic mode.

- [ ] **Step 6: Run the targeted init tests to verify green**

Run: `go test ./bitget -run 'TestNew(Spot|Perp)Adapter' -count=1`

Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add bitget/options.go bitget/common.go bitget/private_profile.go bitget/private_init_test.go
git commit -m "refactor: remove bitget account mode selection"
```

## Task 2: Remove UTA Adapter/Profile Code

**Files:**
- Delete: `bitget/private_uta.go`
- Modify: `bitget/perp_adapter.go`
- Modify: `bitget/spot_adapter.go`
- Modify: `bitget/order_request.go`
- Test: `bitget/ws_order_mode_test.go`

- [ ] **Step 1: Write the failing tests for classic-only constructors and routing**

Update `bitget/ws_order_mode_test.go` so it asserts only:

- constructors default to REST
- classic spot WS place order routes to WS in `OrderModeWS`
- classic perp WS cancel order routes to WS in `OrderModeWS`
- WS mode does not silently fallback to REST

Remove UTA routing tests so the file reflects the desired classic-only contract.

- [ ] **Step 2: Run the routing tests to verify they fail**

Run: `go test ./bitget -run 'Test(Classic|BitgetConstructorsDefaultToRESTOrderMode|BitgetWSOrderModeDoesNotSilentlyFallbackToREST)' -count=1`

Expected: FAIL while UTA-specific seams and account-mode fields still exist.

- [ ] **Step 3: Delete the UTA profile file**

Delete `bitget/private_uta.go`.

- [ ] **Step 4: Simplify perp and spot constructors**

Remove the `accountMode` field from both adapters and stop calling account-mode detection. Instantiate classic private profiles directly while preserving `OrderModeREST` defaulting.

- [ ] **Step 5: Remove remaining account-mode-dependent order-request logic**

Clean up `bitget/order_request.go` so no path depends on `accountMode`.

- [ ] **Step 6: Run the routing tests to verify green**

Run: `go test ./bitget -run 'Test(Classic|BitgetConstructorsDefaultToRESTOrderMode|BitgetWSOrderModeDoesNotSilentlyFallbackToREST)' -count=1`

Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add bitget/perp_adapter.go bitget/spot_adapter.go bitget/order_request.go bitget/ws_order_mode_test.go
git rm bitget/private_uta.go
git commit -m "refactor: make bitget private adapters classic only"
```

## Task 3: Remove UTA SDK Helpers And Dead References

**Files:**
- Delete: `bitget/sdk/private_ws_trade_uta.go`
- Modify: `bitget/sdk/private_ws.go`
- Modify: `bitget/sdk/types.go`
- Test: `bitget/sdk/private_ws_test.go`

- [ ] **Step 1: Write the failing SDK tests for classic-only WS trade support**

Update `bitget/sdk/private_ws_test.go` so it only depends on the generic correlation machinery plus classic trade helpers. Remove any UTA-specific helper expectations that should disappear with the deletion.

- [ ] **Step 2: Run the targeted SDK tests to verify they fail**

Run: `go test ./bitget/sdk -run 'TestPrivateWS|TestPlaceClassicPerpOrderWSOmitsFalseReduceOnly' -count=1`

Expected: FAIL while UTA-specific helper references still exist.

- [ ] **Step 3: Delete the UTA WS helper file**

Delete `bitget/sdk/private_ws_trade_uta.go`.

- [ ] **Step 4: Remove dead UTA references from SDK types or helpers**

Strip any now-dead UTA WS helper references while keeping the generic request-correlation logic and classic WS trading intact.

- [ ] **Step 5: Run the targeted SDK tests to verify green**

Run: `go test ./bitget/sdk -run 'TestPrivateWS|TestPlaceClassicPerpOrderWSOmitsFalseReduceOnly' -count=1`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add bitget/sdk/private_ws.go bitget/sdk/types.go bitget/sdk/private_ws_test.go
git rm bitget/sdk/private_ws_trade_uta.go
git commit -m "refactor: drop bitget uta ws helpers"
```

## Task 4: Update Docs And Live Test Wiring

**Files:**
- Modify: `bitget/adapter_test.go`
- Modify: `.env.example`
- Verify: `docs/superpowers/specs/2026-03-23-bitget-classic-only-design.md`

- [ ] **Step 1: Remove UTA-specific language from live tests**

Keep the classic live tests and optional WS test gating, but remove any wording that implies UTA support.

- [ ] **Step 2: Verify `.env.example` still matches the supported classic path**

Keep only the env vars needed for classic testing. Do not add account-mode toggles back.

- [ ] **Step 3: Run package tests**

Run: `go test ./bitget/...`

Expected: PASS

- [ ] **Step 4: Run live classic REST verification**

Run: `GOCACHE=/tmp/gocache-bitget-classic go test ./bitget -run 'Test(Perp|Spot)Adapter_(Compliance|Orders|OrderQuerySemantics|Lifecycle|LocalState)' -count=1 -v`

Expected: PASS against the user's classic-enabled Bitget setup.

- [ ] **Step 5: Run optional classic WS verification**

Run: `BITGET_ENABLE_WS_ORDER_TESTS=1 GOCACHE=/tmp/gocache-bitget-classic go test ./bitget -run 'Test(PerpAdapter_(Orders_WS|Lifecycle_WS)|SpotAdapter_(Orders_WS|Lifecycle_WS))$' -count=1 -v`

Expected: PASS if Bitget has classic WS trade channel enabled for the account; otherwise capture the exact external failure.

- [ ] **Step 6: Commit**

```bash
git add bitget/adapter_test.go .env.example
git commit -m "docs: align bitget tests with classic-only support"
```
