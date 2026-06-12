# Binance Nautilus Split Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rework Binance around separate spot and perpetual canonical adapters, with REST order submission and private WebSocket execution reports initialized from `Connect`.

**Architecture:** Remove the aggregated Binance `VenueAdapter`. Keep `venue.Adapter` as the shared Go contract, but make Binance expose account-type-specific `SpotAdapter` and `PerpAdapter` implementations selected by constructor or registry configuration. Execution clients own private stream lifecycle and emit canonical `model.ExecutionEvent` values.

**Tech Stack:** Go, current `venue`, `model`, `account`, `sdk/binance/spot`, `sdk/binance/perp`, `go test`.

---

### Task 1: Split Binance Adapter Construction

**Files:**
- Delete: `adapter/binance/venue_adapter.go`
- Create: `adapter/binance/adapter_spot.go`
- Create: `adapter/binance/adapter_perp.go`
- Modify: `adapter/binance/register.go`
- Test: `adapter/binance/canonical_api_test.go`

- [x] Replace the aggregate constructor with account-type-specific constructors.
- [x] Update venue registry to select spot/perp from `account_type`, defaulting to spot.
- [x] Verify constructors expose distinct account IDs.

### Task 2: Split Execution Clients

**Files:**
- Replace: `adapter/binance/execution_client.go`
- Create: `adapter/binance/execution_spot.go`
- Create: `adapter/binance/execution_perp.go`
- Create: `adapter/binance/execution_reports.go`
- Test: `adapter/binance/execution_client_test.go`

- [x] Move shared report mapping helpers into `execution_reports.go`.
- [x] Implement `spotExecutionClient` with only spot routes.
- [x] Implement `perpExecutionClient` with only perp routes.
- [x] Keep REST as the default submit/cancel/report transport.

### Task 3: Add Private Stream Runtime

**Files:**
- Create: `adapter/binance/private_stream.go`
- Modify: `sdk/binance/spot/ws_api.go`
- Modify: `sdk/binance/perp/ws_account.go`
- Test: `adapter/binance/private_stream_test.go`

- [x] Define minimal private stream interfaces for spot/perp execution clients.
- [x] Initialize private streams from `Connect`.
- [x] Emit order, fill, account, and position events from user stream callbacks.
- [x] Add `OnResubscribe` hook support and invoke it after reconnect/resubscribe.

### Task 4: Delete Legacy Binance Adapter Surface

**Files:**
- Delete old root-exchange Binance files that implement `exchanges.Exchange`.
- Update tests that compile against the old Binance root-exchange constructors.

- [x] Remove old order service, market-data service, local orderbook, margin, option, and legacy private stream files.
- [x] Keep SDK files unchanged except stream lifecycle hook additions.
- [x] Keep Binance capabilities scoped to account-type-specific adapters.

### Task 5: Verify

**Commands:**
- `env GOCACHE=/tmp/go-build go test ./adapter/binance ./account ./venue ./model ./testsuite`
- `env GOCACHE=/tmp/go-build go test ./sdk/binance/spot ./sdk/binance/perp`

- [x] Fix compile/test failures introduced by deleting legacy files.
- [x] Report any remaining repo-wide failures separately from targeted Binance verification.
