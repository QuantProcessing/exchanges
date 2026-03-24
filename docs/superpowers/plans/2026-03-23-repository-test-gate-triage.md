# Repository Verification Rollout Status

## Outcome

The repository no longer treats plain `go test ./...` as the canonical verification gate. The verification model is now:

- `quick verify`: `go test -short ./...`
- `exchange verify`: `scripts/verify_exchange.sh <exchange>`
- `full regression`: `bash scripts/verify_full.sh`
- `soak`: `RUN_SOAK=1 bash scripts/verify_soak.sh`

This change was made because the old default gate mixed deterministic unit coverage with live exchange behavior, private credentials, and long-running WebSocket subscriptions.

## What Landed

### 1. Shared gating helpers

`internal/testenv` now provides a single place for:

- repo-root `.env` loading
- shell-env precedence over `.env`
- legacy environment alias support
- `RequireEnv`, `RequireFull`, and `RequireSoak` test gates

This removed ad hoc `.env` parsing and inconsistent skip behavior across exchange packages.

### 2. Scripted verification entrypoints

The repository now has explicit scripts for each verification level:

- `scripts/verify_exchange.sh` for fast exchange-scoped short runs
- `scripts/verify_full.sh` for the supported live/private regression package set
- `scripts/verify_soak.sh` for the long-running subscription suite

`verify_full.sh` manages `.env` loading and `RUN_FULL=1` internally. `verify_soak.sh` requires `RUN_SOAK=1` and only covers the designated soak tests.

### 3. Live-test gating cleanup

Previously ungated or poorly gated tests were moved behind `testing.Short()`, `RequireFull`, or `RequireSoak`, including coverage in:

- `aster`
- `backpack`
- `binance`
- `bitget`
- `edgex`
- `grvt`
- `hyperliquid`
- `lighter`
- `nado`
- `okx`
- `standx`

This made the default local gate deterministic while preserving explicit live/private coverage.

### 4. Test-specific stability fixes

The rollout also included targeted fixes for issues that were exposed while building the new gate:

- `lighter/sdk/common/nonce_test.go`
  - corrected the mainnet URL
  - tolerated transient public-network errors with retry
- `lighter/sdk/ws_order_test.go`
  - fixed a hang caused by waiting indefinitely for additional order updates
- `grvt/sdk`
  - added reusable live-client and retry helpers
  - stabilized websocket connect and transient order placement behavior
  - fixed order cancellation to use the actual client order ID path
- `nado/sdk`
  - added retries for transient websocket/public-network failures
  - downgraded environment-specific order placement failures to clean skips where appropriate
- `standx/sdk/account_test.go`
  - removed a brittle assertion that rejected valid empty position responses

### 5. Soak timing redesign

The old long-running subscription checks were too expensive for routine validation. The soak suite now uses 3-minute checks rather than 30-minute waits.

For `aster` and `binance`, the prior `TestKline` coverage was replaced with long-running market-stream health checks that:

- require an initial event quickly
- require continued events during the 3-minute window

This preserved meaningful subscription coverage without retaining the old half-hour runtime model.

## Verification Evidence

The redesigned gates were exercised successfully during rollout:

- `go test ./internal/testenv -v`
- `go test -short ./...`
- `bash scripts/verify_full.sh`
- `RUN_SOAK=1 bash scripts/verify_soak.sh`

Focused reruns also passed for packages that initially exposed flaky live behavior, including `grvt/sdk`, `nado/sdk`, and `lighter/sdk/common`.

## Residual Notes

The repository still contains exchange-dependent live coverage, but it now sits behind explicit entrypoints instead of contaminating the default short gate.

The remaining deferred work is repository hygiene rather than gate correctness:

- expanding documentation where future contributors add new live tests
- deciding whether more package-specific helper logic should be consolidated into `internal/testenv`
- periodically revisiting the exact package list inside `verify_full.sh` as exchange coverage evolves

## Current Integration Policy

For normal development and CI:

- use `go test -short ./...` as the default gate
- use `scripts/verify_exchange.sh <exchange>` for exchange-scoped changes
- use `scripts/verify_full.sh` before merging significant adapter/SDK changes
- use `scripts/verify_soak.sh` only when stream durability coverage is specifically needed
