# Nautilus-Style User Experience Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement Nautilus-style strategy authoring ergonomics on top of the existing Go trading platform.

**Architecture:** Keep the platform kernel unchanged: `platform.Node` still owns live data/execution routing, account reconciliation, cache, risk, and portfolio updates. Add a strategy-facing layer: model order factory, runtime subscription helpers, typed strategy adapter, deferred live subscriptions, and compact live node assembly.

**Tech Stack:** Go 1.26, `shopspring/decimal`, existing `bus/cache/model/platform/strategy/live/backtest` packages.

---

### Task 1: Order Factory

**Files:**
- Create: `model/order_factory.go`
- Test: `model/order_factory_test.go`

- [x] Write failing tests for generated client order IDs, explicit client order IDs, default limit TIF, market orders, and advanced options.
- [x] Implement `model.OrderFactory`, `NewOrderFactory`, `Market`, `Limit`, and order options.
- [x] Run `go test ./model`.

### Task 2: Runtime Subscription Helpers

**Files:**
- Modify: `strategy/engine.go`
- Modify: `platform/node.go`
- Modify: `backtest/runner.go`
- Test: `strategy/strategy_test.go`
- Test: `platform/node_test.go`
- Test: `backtest/backtest_test.go`

- [x] Write failing compile/runtime tests using `rt.SubscribeOrderBookDepth(...)`, `rt.SubscribeTicker(...)`, and `rt.OrderFactory(accountID)`.
- [x] Extend `strategy.Runtime`.
- [x] Implement helpers in `platform.Node`.
- [x] Implement no-op recorded helpers in `backtest.runtime`.
- [x] Run `go test ./strategy ./platform ./backtest`.

### Task 3: Typed Strategy Adapter

**Files:**
- Create: `strategy/typed.go`
- Test: `strategy/typed_test.go`

- [x] Write failing test for `strategy.NewTyped("id", impl)` where `impl` has no raw `OnEvent`.
- [x] Dispatch ticker/order-book/order-status/accepted/canceled/rejected/fill/account/position callbacks.
- [x] Preserve raw `OnEvent` escape hatch when implemented by the wrapped object.
- [x] Run `go test ./strategy`.

### Task 4: Live OnStart Subscriptions and Convenience Node

**Files:**
- Modify: `platform/node.go`
- Modify: `live/runner.go`
- Test: `live/live_test.go`

- [x] Write failing test where a typed strategy calls `SubscribeOrderBookDepth` from `OnStart` before platform startup completes.
- [x] Add pending subscription storage in `platform.Node`.
- [x] Apply pending subscriptions after instrument provider load and data-client connect.
- [x] Add `live.NodeConfig`, `live.TradingNode`, `NewTradingNode`, `Start`, `Stop`, `Platform`, `Cache`, and `Bus`.
- [x] Run `go test ./platform ./live`.

### Task 5: Backtest Typed Strategy Closure

**Files:**
- Modify: `backtest/runner.go`
- Test: `backtest/backtest_test.go`

- [x] Write failing test for a typed backtest strategy subscribing in `OnStart`, placing a factory-created order on order-book data, and receiving `OnOrderFilled`.
- [x] Use existing matching engine and new runtime helpers to pass.
- [x] Run `go test ./backtest`.

### Task 6: Usage Demo Migration and Documentation

**Files:**
- Modify: `examples/usage_comparison/go_demo/demo.go`
- Modify: `examples/usage_comparison/go_demo/demo_test.go`
- Modify: `examples/usage_comparison/README.md`

- [x] Replace raw-envelope demo strategy with `strategy.NewTyped`.
- [x] Move market-data subscription into strategy `OnStart`.
- [x] Replace manual `model.SubmitOrder` construction with `rt.OrderFactory(accountID).Limit(...)`.
- [x] Update comparison docs so the remaining gap list reflects the new API.
- [x] Run `go test ./examples/usage_comparison/go_demo`.
- [x] Run `go run ./examples/usage_comparison/go_demo/cmd/demo`.

### Task 7: Full Verification

**Files:**
- All touched files.

- [x] Run `gofmt` on touched Go files.
- [x] Run `go test -count=1 $(go list ./... | grep -v '/sdk/')`.
- [x] Run Python compile check for `examples/usage_comparison/nautilus_demo.py`.
- [x] Run `git diff --check`.
- [x] Report exact evidence and remaining limitations.
