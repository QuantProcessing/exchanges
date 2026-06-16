# 量化开发者使用场景

本指南展示量化开发者如何使用本项目。示例刻意保持小，但覆盖真实平台能力：
normalized instruments、strategy runtime callbacks、risk checks、portfolio accounting、
backtest/live symmetry、adapter capability honesty 和 reconciliation。

## Best Practices

- 用 `model.InstrumentID`、`model.AccountID`、normalized market events 和 normalized
  order commands 表达策略逻辑。
- 使用 `strategy.Runtime` 订阅数据、请求数据、提交订单；不要在策略中调用 exchange SDK。
- 通过 `model.OrderFactory` 创建订单，保留 IDs、order-list metadata 和 account identity。
- 让 `platform.Node` 或 `live.Node` 路由订单经过 risk、execution、cache 和 portfolio。
- 用 `cache.Cache` 与 `portfolio.Portfolio` 查询 runtime state。
- 依赖 optional venue behavior 前检查 `venue.DeclaredCapabilities` 与
  [Adapter 能力矩阵](../parity/adapter-capability-matrix_CN.md)。
- 这些概念的可运行版本见 [examples cookbook](../../examples/README_CN.md)。

## Use Case 1: Order Book Imbalance Strategy

目标：订阅 order-book depth，检测 bid/ask imbalance，提交 limit order，并观察 fill 与
portfolio state。

流程：

1. live node 注册 data client 与 execution client；
2. strategy 在 `OnStart` 订阅 order-book depth；
3. `OnOrderBook` 计算信号；
4. 使用 `OrderFactory` 创建 limit order；
5. runtime 经过 risk 和 execution 提交订单；
6. cache 与 portfolio 从 normalized execution events 更新。

```go
func (s *ImbalanceStrategy) OnStart(ctx context.Context, rt strategy.Runtime) error {
    s.runtime = rt
    return rt.SubscribeOrderBookDepth(ctx, s.instrumentID, 2)
}

func (s *ImbalanceStrategy) OnOrderBook(ctx context.Context, book model.OrderBook) error {
    if s.submitted || len(book.Bids) == 0 || len(book.Asks) == 0 {
        return nil
    }
    if !book.Bids[0].Size.GreaterThan(book.Asks[0].Size.Mul(decimal.NewFromInt(2))) {
        return nil
    }
    s.submitted = true
    order := s.runtime.OrderFactory(s.accountID).Limit(
        book.InstrumentID,
        model.OrderSideBuy,
        decimal.RequireFromString("0.01"),
        book.Asks[0].Price,
    )
    _, err := s.runtime.SubmitOrder(ctx, order)
    return err
}
```

可运行参考：
[06_run_live_node_with_in_memory_venue.go](../../examples/06_run_live_node_with_in_memory_venue.go)。

## Use Case 2: Bracket Entry With Take Profit And Stop Loss

```go
list := s.runtime.OrderFactory(s.accountID).Bracket(model.BracketOrderRequest{
    InstrumentID: book.InstrumentID,
    Side:         model.OrderSideBuy,
    Quantity:     decimal.RequireFromString("1"),
    EntryPrice:   decimal.RequireFromString("101"),
    TakeProfit:   decimal.RequireFromString("103"),
    StopLoss:     decimal.RequireFromString("99"),
})
_, err := s.runtime.SubmitOrderList(ctx, list)
```

预期 lifecycle：parent submit/accept，children hold，entry fill 后 release children，一个
child fill 后 sibling cancel，最终 position flat，portfolio PnL 反映 fee。

可运行参考：
[05_submit_bracket_order_backtest.go](../../examples/05_submit_bracket_order_backtest.go)。

## Use Case 3: Deterministic Backtest

```go
runner := backtest.NewRunner(backtest.Config{
    Cache:      cache.New(),
    Strategies: []strategy.Strategy{strategy.NewTyped("research", strategyImpl)},
    Events:     events,
    FillModel:  backtest.DefaultFillModel(),
})

result, err := runner.Run(ctx)
if err != nil {
    return err
}
snapshot, err := result.DeterministicJSON()
```

保持 event timestamps 与 fill model 稳定，并用 `Result.Summary` 或
`Result.DeterministicJSON` 做可重复断言。

可运行参考：
[04_run_strategy_backtest.go](../../examples/04_run_strategy_backtest.go)。

## Use Case 4: Portfolio And Exposure Guardrails

```go
riskEngine := risk.NewEngine(c, risk.Config{
    MaxOrderNotional:    decimal.RequireFromString("1000"),
    MaxAccountExposure:  decimal.RequireFromString("5000"),
    ExposureCurrency:    "USDT",
    MaxOpenOrders:       20,
    MaxCommandsPerWindow: 10,
    CommandRateWindow:   time.Second,
})
```

```go
exposure := node.Portfolio().Exposure(accountID, "USDT")
unrealized := node.Portfolio().UnrealizedPnLs(accountID)
realized := node.Portfolio().RealizedPnLs(accountID)
positions := node.Cache().Positions(accountID)
```

可运行参考：
[03_validate_risk_before_execution.go](../../examples/03_validate_risk_before_execution.go)
和 [06_run_live_node_with_in_memory_venue.go](../../examples/06_run_live_node_with_in_memory_venue.go)。

## Use Case 5: Adapter Capability-Aware Live Setup

```go
caps := adapter.Capabilities()
if !caps.Execution.Submit || !caps.Execution.Cancel || !caps.Execution.OrderReports {
    return fmt.Errorf("adapter lacks required execution lifecycle capabilities")
}
if !caps.Execution.PrivateStream {
    return fmt.Errorf("adapter lacks private stream support for live lifecycle")
}
```

## Use Case 6: Reconciliation After A Private Stream Gap

检查 node health、execution health、fill/position report capabilities 和 unresolved
discrepancies。不要在 private stream readiness 缺失时继续依赖 live lifecycle。

## Choosing A Development Path

| Goal | Start with | Why |
| --- | --- | --- |
| Research a signal | `backtest.Runner` | 确定性数据，无需 venue credentials。 |
| Build strategy logic | `strategy.NewTyped` and `strategy.Runtime` | 同一 authoring shape 可用于 backtest/live。 |
| Wire a live bot | `live.NewNodeBuilder` | 集中 data、execution、risk、portfolio、health。 |
| Add venue behavior | `sdk/` then `adapter/` | 区分 native protocol 与 stable runtime contracts。 |
| Rely on optional feature | `adapter.Capabilities()` and matrix | 防止依赖未支持 venue behavior。 |

## Verification Checklist

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./examples/...
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./backtest ./strategy ./risk ./portfolio ./testsuite
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./venue ./testsuite ./adapter/... ./config/all -run 'Adapter|Capability|Contract|PrivateStream|Resubscribe'
```
