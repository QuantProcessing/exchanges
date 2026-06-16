# Examples

The examples are a progressive API cookbook. Start at `01_...` and move down
only when you need the next layer of the trading stack.

| File | Use When You Are Building |
| --- | --- |
| [01_fetch_ticker_with_adapter.go](./01_fetch_ticker_with_adapter.go) | A market-data probe, health check, quote snapshot tool, or connector smoke test. |
| [02_build_orders_with_order_factory.go](./02_build_orders_with_order_factory.go) | A strategy that needs normalized order commands without hand-building IDs and metadata. |
| [03_validate_risk_before_execution.go](./03_validate_risk_before_execution.go) | A bot or execution service that must reject unsafe orders before venue submission. |
| [04_run_strategy_backtest.go](./04_run_strategy_backtest.go) | A research workflow that replays market events through `strategy.Runtime`. |
| [05_submit_bracket_order_backtest.go](./05_submit_bracket_order_backtest.go) | A strategy using parent/child order lists, take-profit, and stop-loss semantics. |
| [06_run_live_node_with_in_memory_venue.go](./06_run_live_node_with_in_memory_venue.go) | A paper-trading harness, integration test, or live-node assembly prototype. |
| [07_monitor_funding_rate_arbitrage.go](./07_monitor_funding_rate_arbitrage.go) | A multi-venue funding-rate monitor that creates and risk-checks a hedged arbitrage order pair. |

All examples are compiled and exercised by `examples_test.go` except the real
Binance network helper in `01_fetch_ticker_with_adapter.go`, which is provided
as a direct adapter entry point and intentionally not called by default.

Run them with:

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./examples -v
```
