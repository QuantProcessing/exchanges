# Production Trading Platform Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement production-grade non-SDK trading platform capabilities inspired by NautilusTrader: private streams, data subscriptions, reconnect reconciliation, complete order semantics, risk and portfolio state, realistic backtest matching, and multi-exchange consistency tests.

**Architecture:** Keep SDK packages as native protocol layers. Implement production semantics in `model`, `cache`, `account`, `platform`, `strategy`, `backtest`, `risk`, `portfolio`, and `testsuite`, with canonical adapter rollout starting at Binance.

**Tech Stack:** Go 1.26, `shopspring/decimal`, repository-local contract tests, SDK-backed adapters.

---

### Task 1: Complete Order Semantics, Risk, and Portfolio

**Files:**
- Modify: `model/order.go`
- Modify: `model/account.go`
- Modify: `model/instrument.go`
- Modify: `cache/cache.go`
- Modify: `account/reconciler.go`
- Create: `risk/risk.go`
- Create: `portfolio/portfolio.go`
- Test: `model/model_test.go`
- Test: `risk/risk_test.go`
- Test: `portfolio/portfolio_test.go`
- Test: `account/reconciler_test.go`

- [x] Write failing tests for stop/trigger/reduce-only/post-only/order update fields, precision checks, risk rejection, exposure calculation, and side-aware position updates.
- [x] Implement model fields and validation.
- [x] Implement risk engine and portfolio service.
- [x] Run `go test ./model ./cache ./account ./risk ./portfolio`.

### Task 2: Data Subscription Engine

**Files:**
- Modify: `model/market_data.go`
- Modify: `venue/interfaces.go`
- Create: `platform/data_engine.go`
- Test: `platform/data_engine_test.go`
- Test: `testsuite/contracts.go`

- [x] Write failing tests for subscribe/unsubscribe and typed market event publication.
- [x] Add subscription commands and stream-capable optional interfaces.
- [x] Implement data subscription registry and event fan-out in `platform.Node`.
- [x] Implement SDK-backed Binance spot/perp market-data streams for ticker and order book.
- [x] Run `go test ./model ./venue ./platform ./testsuite`.

### Task 3: Execution Stream Supervisor and Reconciliation

**Files:**
- Modify: `venue/interfaces.go`
- Modify: `platform/node.go`
- Modify: `account/reconciler.go`
- Test: `platform/node_test.go`
- Test: `account/reconciler_test.go`

- [x] Write failing tests for reconnect, resubscribe, startup reconciliation, fill-before-order replay, and reconciliation error reporting.
- [x] Add optional resubscribe/reconcile hooks to execution clients.
- [x] Implement supervisor loop and reconciliation after stream gaps.
- [x] Implement SDK-backed Binance spot/perp private stream order/fill/account/position mapping.
- [x] Run `go test -race ./account ./platform`.

### Task 4: Realistic Backtest Matching

**Files:**
- Modify: `backtest/runner.go`
- Create: `backtest/matching.go`
- Test: `backtest/backtest_test.go`
- Test: `backtest/matching_test.go`

- [x] Write failing tests for book-walking market orders, resting limit orders, and partial fills.
- [x] Implement matching engine and deterministic venue loop.
- [x] Run `go test -race ./backtest`.
- [ ] Add stop-trigger and same-timestamp cascading settlement edge-case tests.

### Task 5: Multi-Exchange Consistency Tests

**Files:**
- Modify: `testsuite/contracts.go`
- Modify: `adapter/*/*_test.go`
- Test: `testsuite/contracts_test.go`

- [x] Write failing tests requiring honest capability declarations and lifecycle behavior per venue.
- [x] Add reusable contract suites for declared data/private stream capability evidence.
- [x] Apply first to Binance spot/perp, then keep non-ready venues marked honestly.
- [x] Run `go test ./testsuite ./adapter/...`.

## Current Capability Matrix

| Area | Status | Evidence |
| --- | --- | --- |
| Platform order lifecycle, risk, portfolio, cache | Implemented | `go test -race ./model ./cache ./account ./risk ./portfolio ./platform` |
| Data subscription engine | Implemented | `platform.Node.SubscribeMarketData`, `venue.StreamingDataClient`, cache latest ticker/book indexes |
| Execution stream recovery and reconciliation | Implemented in platform | `ExecutionResubscriber`, fill-before-order replay, reconnect/resubscribe/reconcile tests |
| Backtest order-book matching | Implemented for market/limit book walking | `go test -race ./backtest` |
| Multi-exchange capability honesty | Implemented | `testsuite.RunAdapterCapabilitySuite`, `config/all` all-adapter test |
| Binance spot/perp public streams | Implemented | SDK `WsMarketClient` mapped to normalized ticker/order book events |
| Binance spot/perp private streams | Implemented | SDK account streams mapped to order/fill/account/position events |
| Bitget spot/perp public streams | Implemented | SDK public WS mapped to normalized ticker/order book events |
| Bitget spot/perp private streams | Implemented | SDK private WS `order`/`fill`/`position` topics mapped to execution events |
| Bybit spot/linear public streams | Implemented | SDK public WS mapped to normalized ticker/order book events |
| Bybit spot/linear private streams | Implemented | SDK private WS `order`/`execution`/`position` topics mapped to execution events |
| OKX spot/swap public streams | Implemented | SDK `WSClient.SubscribeTicker`/`SubscribeOrderBook` mapped to normalized ticker/order book events |
| OKX spot/swap private streams | Implemented | SDK private `orders`/`positions` channels mapped to order/fill/position events with explicit resubscribe |
| Backpack perp public streams | Implemented | Official `bookTicker.<symbol>`/`depth.<symbol>` streams mapped through SDK `WSClient` to normalized ticker/order book events |
| Backpack perp private streams | Implemented | Official `account.orderUpdate`/`account.positionUpdate` streams mapped through SDK `WSClient` to order/fill/position events with explicit resubscribe |
| Nado perp public streams | Implemented | SDK `WsMarketClient` `best_bid_offer`/`book_depth` subscriptions mapped to normalized ticker/order book events |
| Nado perp private streams | Implemented | SDK `WsAccountClient` `order_update`/`fill`/`position_change` subscriptions mapped to execution events with explicit resubscribe |
| EdgeX perp public streams | Implemented | SDK `WsMarketClient` `ticker.<contractId>`/`depth.<contractId>.<depth>` subscriptions mapped to normalized ticker/order book events |
| EdgeX perp private streams | Implemented | SDK `WsAccountClient` order/fill/position update handlers mapped to execution events with explicit resubscribe and position report generation |
| Lighter perp public streams | Implemented | SDK `WebsocketClient` `ticker/<market>` and `order_book/<market>` subscriptions mapped to normalized ticker/order book events |
| Lighter perp private streams | Implemented | SDK `account_all_orders`/`account_all_trades`/`account_all_positions` subscriptions mapped to order/fill/position events with explicit resubscribe and position report generation |
| GRVT perp public streams | Implemented | SDK typed `SubscribeTickerSnap`/`SubscribeOrderbookSnap` streams mapped to normalized ticker/order book events |
| GRVT perp private streams | Implemented | SDK typed `SubscribeOrderUpdate`/`SubscribeFill`/`SubscribePositions` streams mapped to order/fill/position events with explicit resubscribe and account-summary position generation |
| StandX perp public streams | Implemented | SDK `WsMarketClient` `price`/`depth_book` streams mapped to normalized ticker/order book events |
| StandX perp private streams | Implemented | SDK `WsAccountClient` authenticated `order`/`trade`/`position` streams mapped to order/fill/position events with explicit resubscribe and `QueryPositions` reconciliation |
| Hyperliquid perp public streams | Implemented | SDK `SubscribeBbo`/`SubscribeL2Book` streams mapped to normalized ticker/order book events |
| Hyperliquid perp private streams | Implemented | SDK `orderUpdates`/`userFills`/`webData2` subscriptions mapped to order/fill/position events with explicit resubscribe and `GetBalance` position reconciliation |
| Aster spot public streams | Implemented | SDK `WsMarketClient` `bookTicker`/`depth` streams mapped to normalized ticker/order book events |
| Aster spot private streams | Implemented | SDK listen-key `executionReport`/`outboundAccountPosition` streams mapped to order/fill/account events with explicit resubscribe |
| Aster perp public streams | Implemented | SDK `WsMarketClient` `bookTicker`/`depth` streams mapped to normalized ticker/order book events |
| Aster perp private streams | Implemented | SDK listen-key `ORDER_TRADE_UPDATE`/`ACCOUNT_UPDATE` streams mapped to order/fill/position/account events with explicit resubscribe and account-position reconciliation |
| Remaining adapter real public/private streams | Current cleanroom adapter set implemented | Capability contracts keep `Streams`/`PrivateStream` false for any future adapter until adapter-specific mapping and tests exist |

## Nautilus Alignment Evidence

- Nautilus separates live data and execution clients (`live/data_client.py`, `live/execution_client.py`). The Go runtime mirrors this with independently constructible `venue.DataClient` and `venue.ExecutionClient` per adapter product.
- Nautilus execution reconciliation gathers `OrderStatusReport`, `FillReport`, and `PositionStatusReport` together (`live/execution_client.py`). The Go platform mirrors this in `platform.Node.reconcileInstrument` with order reports plus optional fill and position report generators.
- Nautilus concrete exchange adapters subscribe to public data and private execution topics separately (for example Bybit data/execution clients). The Go Binance, Bitget, Bybit, OKX, Backpack, Nado, EdgeX, Lighter, GRVT, StandX, Hyperliquid perp, and Aster spot/perp adapters now use SDK-backed public streams for market events and private streams for execution events, with capability claims gated by contract tests.
