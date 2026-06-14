# SDK-Backed Nautilus-Go Cleanroom Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rebuild the repository above `sdk/` into a clean Nautilus-style Go trading platform with SDK-backed adapters passing local contract tests.

**Architecture:** Keep SDK packages and SDK-required `internal/` helpers. Recreate `model`, `venue`, `bus`, `cache`, `account`, `platform`, `strategy`, `testsuite`, `adapter/binance`, and `config/all` from scratch around small ports and normalized events.

**Tech Stack:** Go, `github.com/shopspring/decimal`, current SDK packages, local `go test`.

---

## Task 1: Domain Model

- [x] Delete old root API and old non-SDK platform folders.
- [x] Add failing tests for identifiers, instruments, market data, execution
  reports, and account snapshots.
- [x] Implement minimal normalized model package.
- [x] Run `go test ./model -count=1`.

## Task 2: Bus, Cache, And Account Reconciliation

- [x] Add failing bus fan-out tests.
- [x] Add failing cache instrument/order/account tests.
- [x] Add failing account reconciler tests for snapshot and execution events.
- [x] Implement `bus`, `cache`, and `account`.
- [x] Run `go test ./bus ./cache ./account -count=1`.

## Task 3: Venue Ports And Registry

- [x] Add failing registry and contract tests.
- [x] Implement provider/data/execution ports, health structs, capabilities,
  and registry.
- [x] Run `go test ./venue -count=1`.

## Task 4: Platform And Strategy Runtime

- [x] Add failing platform startup-order test.
- [x] Add failing strategy event-delivery test.
- [x] Implement `platform.Node` and `strategy.Engine`.
- [x] Run `go test ./platform ./strategy -count=1`.

## Task 4.5: Backtest And Live Orchestration

- [x] Add failing strategy market-data delivery test.
- [x] Add failing backtest runner test for ordered market-event replay into
  strategies.
- [x] Add failing live runner test proving strategies start before platform
  startup execution events.
- [x] Implement `backtest.Runner` and `live.Runner`.
- [x] Run `go test ./strategy ./backtest ./live -count=1`.

## Task 5: Contract Test Suite

- [x] Add reusable provider/data/execution/platform contract suites.
- [x] Keep suites independent from any old root adapter API.
- [x] Use the venue contract suite from Binance spot/perp adapter tests.
- [x] Run `go test ./testsuite -count=1`.

## Task 6: Binance Adapter

- [x] Add failing Binance symbol/provider/data/execution tests with fake SDK
  interfaces.
- [x] Implement spot/perp instrument providers from SDK exchange info.
- [x] Implement spot/perp data clients for ticker and order book.
- [x] Implement spot/perp execution clients for account, submit, cancel, and
  startup reports.
- [x] Register Binance through `venue`.
- [x] Run `go test ./adapter/binance ./config/all -count=1`.

## Task 7: Milestone Verification

- [x] Run `go test ./model ./bus ./cache ./account ./venue ./platform ./strategy ./backtest ./live ./testsuite ./adapter/binance ./config/all -count=1`.
- [x] Run `go test ./sdk/... -run '^$'` to compile SDK packages without live
  network calls.
- [x] Run `go test ./... -run '^$'` and explain any compile-only package gaps.
- [x] Run `git diff --check`.

## Task 8: Remaining Exchange Rollout

- [x] Add SDK-backed Aster spot/perp adapter using the Binance-family contract
  pattern.
- [x] Run `go test ./adapter/aster ./config/all -count=1`.
- [x] Run `go test ./model ./bus ./cache ./account ./venue ./platform ./strategy ./backtest ./live ./testsuite ./adapter/binance ./adapter/aster ./config/all -count=1`.
- [x] Run `go test ./sdk/... -run '^$'`.
- [x] Run `go test ./... -run '^$'`.
- [x] Run `git diff --check`.
- [x] Add SDK-backed OKX spot/swap adapter using the shared venue contract
  suite.
- [x] Run `go test ./adapter/okx ./config/all -count=1`.
- [x] Run `go test ./strategy -count=20`.
- [x] Run `go test -race ./strategy -count=1`.
- [x] Run `go test ./model ./bus ./cache ./account ./venue ./platform ./strategy ./backtest ./live ./testsuite ./adapter/binance ./adapter/aster ./adapter/okx ./config/all -count=1`.
- [x] Run `go test ./sdk/... -run '^$'`.
- [x] Run `go test ./... -run '^$'`.
- [x] Run `git diff --check`.
- [x] Add SDK-backed Bybit spot/linear adapter using category-aware providers.
- [x] Run `go test ./adapter/bybit ./config/all -count=1`.
- [x] Run `go test ./model ./bus ./cache ./account ./venue ./platform ./strategy ./backtest ./live ./testsuite ./adapter/binance ./adapter/aster ./adapter/okx ./adapter/bybit ./config/all -count=1`.
- [x] Run `go test ./sdk/... -run '^$'`.
- [x] Run `go test ./... -run '^$'`.
- [x] Run `git diff --check`.
- [x] Add SDK-backed Bitget spot/perp adapter using product-type-aware
  providers.
- [x] Run `go test ./adapter/bitget ./config/all -count=1`.
- [x] Run `go test ./model ./bus ./cache ./account ./venue ./platform ./strategy ./backtest ./live ./testsuite ./adapter/binance ./adapter/aster ./adapter/okx ./adapter/bybit ./adapter/bitget ./config/all -count=1`.
- [x] Run `go test ./sdk/... -run '^$'`.
- [x] Run `go test ./... -run '^$'`.
- [x] Run `git diff --check`.
- [x] Add SDK-backed Hyperliquid spot/perp adapter using SDK meta endpoints.
- [x] Run `go test ./adapter/hyperliquid ./config/all -count=1`.
- [x] Run `go test ./model ./bus ./cache ./account ./venue ./platform ./strategy ./backtest ./live ./testsuite ./adapter/binance ./adapter/aster ./adapter/okx ./adapter/bybit ./adapter/bitget ./adapter/hyperliquid ./config/all -count=1`.
- [x] Run `go test ./sdk/... -run '^$'`.
- [x] Run `go test ./... -run '^$'`.
- [x] Run `git diff --check`.
- [x] Add SDK-backed Lighter perp adapter using market-id-aware providers.
- [x] Run `go test ./adapter/lighter ./config/all -count=1`.
- [x] Run `go test ./model ./bus ./cache ./account ./venue ./platform ./strategy ./backtest ./live ./testsuite ./adapter/binance ./adapter/aster ./adapter/okx ./adapter/bybit ./adapter/bitget ./adapter/hyperliquid ./adapter/lighter ./config/all -count=1`.
- [x] Run `go test ./sdk/... -run '^$'`.
- [x] Run `go test ./... -run '^$'`.
- [x] Run `git diff --check`.
- [x] Add SDK-backed Nado perp adapter using contract-v2 providers.
- [x] Run `go test ./adapter/nado ./config/all -count=1`.
- [x] Run `go test ./model ./bus ./cache ./account ./venue ./platform ./strategy ./backtest ./live ./testsuite ./adapter/binance ./adapter/aster ./adapter/okx ./adapter/bybit ./adapter/bitget ./adapter/hyperliquid ./adapter/lighter ./adapter/nado ./config/all -count=1`.
- [x] Run `go test ./sdk/... -run '^$'`.
- [x] Run `go test ./... -run '^$'`.
- [x] Run `git diff --check`.
- [x] Add SDK-backed EdgeX perp adapter using contract-id-aware providers and
  StarkEx order submission metadata.
- [x] Run `go test ./adapter/edgex ./config/all -count=1`.
- [x] Run `go test ./model ./bus ./cache ./account ./venue ./platform ./strategy ./backtest ./live ./testsuite ./adapter/binance ./adapter/aster ./adapter/okx ./adapter/bybit ./adapter/bitget ./adapter/hyperliquid ./adapter/lighter ./adapter/nado ./adapter/edgex ./config/all -count=1`.
- [x] Run `go test ./sdk/... -run '^$'`.
- [x] Run `go test ./... -run '^$'`.
- [x] Run `git diff --check`.
- [x] Add SDK-backed GRVT perp adapter using instrument-hash-aware providers
  and signed single-leg order request mapping.
- [x] Run `go test ./adapter/grvt ./config/all -count=1`.
- [x] Run `go test ./model ./bus ./cache ./account ./venue ./platform ./strategy ./backtest ./live ./testsuite ./adapter/binance ./adapter/aster ./adapter/okx ./adapter/bybit ./adapter/bitget ./adapter/hyperliquid ./adapter/lighter ./adapter/nado ./adapter/edgex ./adapter/grvt ./config/all -count=1`.
- [x] Run `go test ./sdk/... -run '^$'`.
- [x] Run `go test ./... -run '^$'`.
- [x] Run `git diff --check`.
- [x] Add SDK-backed StandX perp adapter using symbol-info providers and
  client-order-id anchored REST submission reports.
- [x] Run `go test ./adapter/standx ./config/all -count=1`.
- [x] Run `go test ./model ./bus ./cache ./account ./venue ./platform ./strategy ./backtest ./live ./testsuite ./adapter/binance ./adapter/aster ./adapter/okx ./adapter/bybit ./adapter/bitget ./adapter/hyperliquid ./adapter/lighter ./adapter/nado ./adapter/edgex ./adapter/grvt ./adapter/standx ./config/all -count=1`.
- [x] Run `go test ./sdk/... -run '^$'`.
- [x] Run `go test ./... -run '^$'`.
- [x] Run `git diff --check`.
- [x] Add SDK-backed Backpack perp adapter using marketType-filtered providers
  and REST order/client-id mapping.
- [x] Run `go test ./adapter/backpack ./config/all -count=1`.
- [x] Run `go test ./model ./bus ./cache ./account ./venue ./platform ./strategy ./backtest ./live ./testsuite ./adapter/binance ./adapter/aster ./adapter/okx ./adapter/bybit ./adapter/bitget ./adapter/hyperliquid ./adapter/lighter ./adapter/nado ./adapter/edgex ./adapter/grvt ./adapter/standx ./adapter/backpack ./config/all -count=1`.
- [x] Run `go test ./sdk/... -run '^$'`.
- [x] Run `go test ./... -run '^$'`.
- [x] Run `git diff --check`.
