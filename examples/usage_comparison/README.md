# Usage Comparison: NautilusTrader vs Go Platform

This directory implements the same simple quant idea in both projects:

1. Subscribe to order-book depth for `BTC-USDT-SPOT.BINANCE`.
2. When best-bid size is more than 2x best-ask size, submit a buy limit order at best ask.
3. Track the accepted order, fill, position, and account exposure.

The examples are deliberately small, but they exercise the important platform
lifecycle surface: market data -> strategy -> risk -> execution -> private
execution event -> reconciler/cache -> portfolio.

## Files

| File | Purpose |
| --- | --- |
| `go_demo/demo.go` | Runnable Go demo using `live.NewTradingNode`, `strategy.NewTyped`, runtime subscription helpers, `model.OrderFactory`, risk, portfolio, cache, and platform execution routing. Only the exchange network edge is in-memory. |
| `go_demo/demo_test.go` | Regression test proving the Go demo reaches a filled order, fill report, long position, exposure update, and event log. |
| `go_demo/cmd/demo/main.go` | CLI wrapper for the Go demo. |
| `nautilus_demo.py` | NautilusTrader-side strategy written against Nautilus' native `Strategy` API. It is a usage comparison artifact; running it needs a Nautilus BacktestEngine or TradingNode environment. |

## Run the Go demo

```bash
go test ./examples/usage_comparison/go_demo
go run ./examples/usage_comparison/go_demo/cmd/demo
```

Expected CLI shape:

```text
signal_triggered=true
final_order=demo-order-1 status=filled filled=0.01 leaves=0
fills=1 first_trade=demo-trade-1
position=long qty=0.01 entry=101
usdt_exposure=1.01
events=[market:order_book execution:order:accepted execution:order:filled execution:fill]
```

## Check the Nautilus script

```bash
python3 -m py_compile examples/usage_comparison/nautilus_demo.py
```

The Nautilus script is not a standalone runner because Nautilus strategies are
normally added to a `BacktestEngine` or `TradingNode` with venue, data catalog,
and execution configuration. The script focuses on the code a Nautilus strategy
author writes.

## Usage-Level Differences

| Area | NautilusTrader usage | Current Go project usage | Interpretation |
| --- | --- | --- | --- |
| Strategy lifecycle | Inherit `Strategy`, implement `on_start`, typed data callbacks such as `on_order_book_depth`, and order callbacks such as `on_order_accepted` / `on_order_filled`. | Wrap a plain Go object with `strategy.NewTyped`; implement only callbacks such as `OnStart`, `OnOrderBook`, `OnOrderStatus`, and `OnOrderFilled`. | The strategy authoring model is now close in feel, with Go composition instead of Python inheritance. |
| Market-data subscription | Strategy calls `self.subscribe_order_book_depth(...)` directly. | Strategy calls `rt.SubscribeOrderBookDepth(...)` from `OnStart`; live mode defers the subscription until instruments are loaded. | This gap is closed for the common strategy-start subscription path. |
| Order creation | `self.order_factory.limit(...)` creates normalized, instrument-aware orders. | Strategy calls `rt.OrderFactory(accountID).Limit(...)` and passes options such as `model.WithClientOrderID(...)`. | This gap is closed for market and limit orders; advanced order helpers can grow from the same factory. |
| Risk path | `submit_order` routes through Nautilus RiskEngine before execution unless emulation/exec-algo changes the path. | `strategy.Runtime.SubmitOrder` calls `platform.Node.SubmitOrder`, which runs `risk.Engine.Check` before `ExecutionClient.SubmitOrder`. | This is now equivalent at the core lifecycle level. |
| Execution updates | Nautilus emits order/fill events into typed strategy callbacks and updates cache/portfolio internally. | Execution clients emit `model.ExecutionEvent`; `platform.Node` applies reports to `account.Reconciler`, `cache`, and `portfolio`, then `strategy.NewTyped` maps events to typed callbacks. | The callback-layer gap is closed for common order/fill/account/position events. |
| User wiring | Most platform components are hidden behind `BacktestEngine` / `TradingNode` configs. | Demo uses `live.NewTradingNode(...)` with clients, strategies, risk, cache, and portfolio. Backtests can use `backtest.NewEngine(...).AddStrategy().AddData().Run(...)`. | Go now has a higher-level entry, though Nautilus still has richer config objects and reporting surfaces. |

## What This Side-by-Side Proves

The Go demo proves the target lifecycle is not just nominal:

- A real `platform.Node` accepts data and execution clients.
- A market-data subscription delivers an order-book event to a running strategy.
- The strategy subscribes from `OnStart` and submits a factory-created normalized order through the runtime.
- Risk checks run before execution submission.
- Execution reports update order state, fills, position, and portfolio exposure.
- The strategy observes order and fill updates through typed callbacks.

So from both a functional platform perspective and a common strategy-authoring
perspective, the demo path is closed. Remaining Nautilus-level gaps are now
mostly breadth rather than the basic feel: richer config schemas, built-in
reports/analyzers, more order factory helpers for every advanced order type,
and broader live/backtest examples.
