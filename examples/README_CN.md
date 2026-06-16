# 示例

examples 是渐进式 API cookbook。建议从 `01_...` 开始，只有需要下一层交易能力时再往后看。

| 文件 | 适合参考的场景 |
| --- | --- |
| [01_fetch_ticker_with_adapter.go](./01_fetch_ticker_with_adapter.go) | 行情探针、健康检查、报价快照工具、connector smoke test。 |
| [02_build_orders_with_order_factory.go](./02_build_orders_with_order_factory.go) | 策略需要 normalized order commands，但不想手写 IDs 和 metadata。 |
| [03_validate_risk_before_execution.go](./03_validate_risk_before_execution.go) | bot 或 execution service 必须在 venue submission 前拒绝危险订单。 |
| [04_run_strategy_backtest.go](./04_run_strategy_backtest.go) | 用 `strategy.Runtime` 回放 market events 的研究流程。 |
| [05_submit_bracket_order_backtest.go](./05_submit_bracket_order_backtest.go) | 使用 parent/child order lists、take-profit、stop-loss 语义的策略。 |
| [06_run_live_node_with_in_memory_venue.go](./06_run_live_node_with_in_memory_venue.go) | paper trading harness、集成测试、live-node 组装原型。 |
| [07_monitor_funding_rate_arbitrage.go](./07_monitor_funding_rate_arbitrage.go) | 多交易所资金费率监控，生成并风控校验双腿对冲套利订单。 |

除 `01_fetch_ticker_with_adapter.go` 中真实 Binance 网络 helper 不默认调用外，所有示例都由
`examples_test.go` 编译并执行。

运行：

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./examples -v
```
