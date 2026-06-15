# Usage Comparison Demo

This directory implements a small order-book imbalance idea in the current Go
platform and keeps a Python reference sketch beside it for maintainers.

The Go demo is the supported runnable path:

1. subscribe to order-book depth for `BTC-USDT-SPOT.BINANCE`;
2. submit a buy limit order at best ask when best-bid size is more than 2x
   best-ask size;
3. track accepted order, fill, position, and account exposure.

## Files

| File | Purpose |
| --- | --- |
| `go_demo/demo.go` | Runnable Go demo using `live.NewTradingNode`, `strategy.NewTyped`, runtime subscription helpers, `model.OrderFactory`, risk, portfolio, cache, and platform execution routing. Only the exchange network edge is in-memory. |
| `go_demo/demo_test.go` | Regression test proving the Go demo reaches a filled order, fill report, long position, exposure update, and event log. |
| `go_demo/cmd/demo/main.go` | CLI wrapper for the Go demo. |
| Python reference sketch | Non-runnable reference strategy shape kept for maintainers who compare callback ergonomics across platforms. |

## Run The Go Demo

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

## What The Go Demo Proves

- A real `platform.Node` accepts data and execution clients.
- A market-data subscription delivers an order-book event to a running strategy.
- The strategy subscribes from `OnStart` and submits a factory-created
  normalized order through the runtime.
- Risk checks run before execution submission.
- Execution reports update order state, fills, position, and portfolio
  exposure.
- The strategy observes order and fill updates through typed callbacks.
