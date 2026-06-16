# 回测

`backtest.Runner` 用 timestamped events、deterministic time 和 simulated execution
运行策略。它适合在接入真实 venue clients 前验证策略行为、订单生命周期、风控、
组合会计和可重复性。

## Runner 做什么

每个 event 会经历：

1. 推进 runtime clock；
2. dispatch due timers；
3. 必要时 expire open orders；
4. 记录 market events；
5. 用最新 market state match open orders；
6. dispatch event 到 strategies；
7. drain callbacks 中创建的 same-timestamp commands；
8. match 新提交且 marketable 的 orders；
9. 更新 cache、portfolio、execution health 和 result metadata。

## 最小 Backtest

```go
runner := backtest.NewRunner(backtest.Config{
    Cache: cache.New(),
    Strategies: []strategy.Strategy{
        strategy.NewTyped("research", strategyImpl),
    },
    Events:    events,
    FillModel: backtest.DefaultFillModel(),
})

result, err := runner.Run(ctx)
if err != nil {
    return err
}
orders := result.Cache.Orders("main")
exposure := result.Portfolio.Exposure("main", "USDT")
```

可运行代码：[04_run_strategy_backtest.go](../../examples/04_run_strategy_backtest.go)。

## Backtest Inputs

`backtest.Event` 包含：

- `At`：event timestamp；
- `Topic`：通常是 `strategy.TopicMarketData`、`strategy.TopicExecution`、
  `strategy.TopicTimer` 或 `strategy.TopicError`；
- `Message`：如 `model.MarketEvent` 的 normalized model value。

可复用数据可通过 `data.Catalog` 实现，并传给 `backtest.Config.DataCatalog`。

## Fill Model 与 Latency

默认使用 `backtest.DefaultFillModel()`。需要特定 fee、slippage 或 fill behavior 时再显式
配置。`backtest.Config.OrderLatency` 可延迟订单 eligibility，避免订单在触发它的同一个
event 上立即成交。

## 应该断言什么

不要只断言最终收益。更好的断言包括 processed events、order count/status、command
metadata、fill count/quantity、final positions、exposure/PnL、no duplicate fills 和
deterministic result output。

## Verification

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./backtest ./testsuite -run 'Backtest|Result|FillModel|Catalog' -v
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./examples/... -v
```

Bracket order-list 行为见
[05_submit_bracket_order_backtest.go](../../examples/05_submit_bracket_order_backtest.go)。
