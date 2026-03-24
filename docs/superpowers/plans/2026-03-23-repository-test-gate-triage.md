# Repository Test Gate Triage

## Purpose

This note captures why `go test ./...` is still not a clean integration gate after the adapter naming-convergence rollout, and what should happen next.

The important conclusion is that the current failures are not a single branch regression. They fall into three buckets:

- ungated live/integration tests that should not run in the default local gate
- tests that assume credentials exist and fail badly when they do not
- a smaller set of tests that likely reflect actual SDK drift and should be investigated separately once the gate is narrowed

## Current Failure Groups

### 1. Ungated live websocket or exchange-network tests

These tests make real network calls or wait for exchange-side events, but they do not currently opt out of the default `go test ./...` path.

- `aster/sdk/perp/ws_market_test.go`
  Evidence: [TestKline](/Users/dylan/Code/exchanges/.worktrees/adapter-naming-convergence/aster/sdk/perp/ws_market_test.go#L186) opens a real websocket and waits 10 seconds for a Binance kline event.
- `binance/sdk/perp/ws_market_test.go`
  Evidence: [TestKline](/Users/dylan/Code/exchanges/.worktrees/adapter-naming-convergence/binance/sdk/perp/ws_market_test.go#L186) does the same.
- `grvt/sdk/ws_market_test.go`
  Evidence: [TestSubscribeOrderbookDelta](/Users/dylan/Code/exchanges/.worktrees/adapter-naming-convergence/grvt/sdk/ws_market_test.go#L10) connects to the live websocket and waits on a timer.
- `okx/sdk/ws_order_test.go`
  Evidence: [TestCancelOrderWs](/Users/dylan/Code/exchanges/.worktrees/adapter-naming-convergence/okx/sdk/ws_order_test.go#L74) connects to the live private websocket and places/cancels orders.
- `hyperliquid/sdk/perp/ws_account_test.go`
  Evidence: [TestSubscribeOrderUpdates](/Users/dylan/Code/exchanges/.worktrees/adapter-naming-convergence/hyperliquid/sdk/perp/ws_account_test.go#L22) waits for 30 minutes.
- `lighter/sdk/common/nonce_test.go`
  Evidence: [TestNonceManager_Fetch](/Users/dylan/Code/exchanges/.worktrees/adapter-naming-convergence/lighter/sdk/common/nonce_test.go#L8) fetches from `https://mainnet.zkelliot.ai` directly.

These should not be part of the default branch integration gate.

### 2. Tests that implicitly require credentials but do not guard on env presence

These tests read `.env` or raw env vars and then proceed unconditionally. On a machine without credentials they fail immediately, and in some cases they fail with a poor error mode.

- `edgex/sdk/perp/account_test.go`
  Evidence: [GetEnv](/Users/dylan/Code/exchanges/.worktrees/adapter-naming-convergence/edgex/sdk/perp/account_test.go#L14) loads env vars and [TestGetAccountAsset](/Users/dylan/Code/exchanges/.worktrees/adapter-naming-convergence/edgex/sdk/perp/account_test.go#L19) runs regardless of whether credentials exist.
- `nado/sdk/account_test.go` and `nado/sdk/market_test.go`
  Evidence: [GetEnv](/Users/dylan/Code/exchanges/.worktrees/adapter-naming-convergence/nado/sdk/market_test.go#L12) loads env vars and multiple tests call `WithCredentials(...)` without any skip gate.
- `grvt/sdk/account_test.go`
  Evidence: [GetEnv](/Users/dylan/Code/exchanges/.worktrees/adapter-naming-convergence/grvt/sdk/account_test.go#L13) loads env vars and [TestGetFundingAccountSummary](/Users/dylan/Code/exchanges/.worktrees/adapter-naming-convergence/grvt/sdk/account_test.go#L19) runs immediately.
- `hyperliquid/sdk/client_test.go`
  Evidence: [GetEnv](/Users/dylan/Code/exchanges/.worktrees/adapter-naming-convergence/hyperliquid/sdk/client_test.go#L13) loads env vars and [TestGetUserFees](/Users/dylan/Code/exchanges/.worktrees/adapter-naming-convergence/hyperliquid/sdk/client_test.go#L17) always hits the live API.
- `hyperliquid/sdk/perp/ws_account_test.go`
  Evidence: [GetEnv](/Users/dylan/Code/exchanges/.worktrees/adapter-naming-convergence/hyperliquid/sdk/perp/ws_account_test.go#L15) loads env vars but the websocket tests do not skip when credentials are missing.

These should be moved behind explicit environment guards, build tags, or `testing.Short()` policy.

### 3. Failures that indicate real code or contract issues once the gate is narrowed

These deserve separate debugging after the default gate is fixed.

- `lighter/sdk/order.go`
  Evidence: [PlaceOrder](/Users/dylan/Code/exchanges/.worktrees/adapter-naming-convergence/lighter/sdk/order.go#L42) dereferences `c.KeyManager` without protecting against an invalid or missing credential setup. This surfaced as a nil panic in `TestOrder_PlaceMarketOrder`.
- `hyperliquid/sdk/client.go`
  Evidence: [GetUserFees](/Users/dylan/Code/exchanges/.worktrees/adapter-naming-convergence/hyperliquid/sdk/client.go#L174) blindly unmarshals the `/info` response into `UserFees`. The reported failure was a JSON deserialization mismatch, which suggests API drift or a model mismatch rather than a naming-convergence regression.

The `grvt/sdk` streaming failure from the broad `go test ./...` run should be treated as unclassified until reproduced with a narrower command. The current evidence only shows that a live websocket/integration test failed, not whether the cause is network, credentials, or SDK logic.

## Recommended Next Steps

### Phase 1: Fix the default test gate

Make `go test ./...` represent a deterministic local/CI-safe gate.

Recommended actions:

1. Add explicit skip guards for tests that require private credentials.
2. Mark long-running websocket/live tests as integration tests, or gate them behind dedicated env vars.
3. Keep public-network smoke tests out of the default `go test ./...` path unless they are rewritten to use mocks or fixtures.

The immediate goal is not to delete live coverage. It is to stop mixing live exchange behavior with the repository's default merge gate.

### Phase 2: Fix poor failure modes exposed by missing credentials

After gating, clean up the most obvious sharp edges:

1. `lighter/sdk` should fail with a normal credential/setup error instead of panicking when `KeyManager` is unavailable.
2. Any SDK test helper that loads env vars should skip cleanly when required credentials are absent instead of attempting a live request.

### Phase 3: Investigate likely real SDK drift

Once Phases 1 and 2 are done, reproduce and debug the remaining genuine failures:

1. `hyperliquid/sdk` `TestGetUserFees`
2. any remaining `grvt/sdk` websocket/orderbook failures after live-test gating is corrected

## Recommended Integration Policy For The Naming-Convergence Branch

Until the broader repository gate is cleaned up, this branch should be evaluated on:

- focused naming-convergence tests
- `go build ./...`
- targeted package reviews

It should not be blocked on the current repository-wide mix of live websocket and credential-dependent tests.
