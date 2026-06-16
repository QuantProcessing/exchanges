# 工作流示例

本指南展示常见交易工作流的 Go API 形状。每个示例都保持小而聚焦。可运行代码从
[examples cookbook](../../examples/README_CN.md) 开始看。

## Bracket Strategy

创建 entry order 加 take-profit 与 stop-loss children：

```go
list := rt.OrderFactory(accountID).Bracket(model.BracketOrderRequest{
    InstrumentID: instrumentID,
    Side:         model.OrderSideBuy,
    Quantity:     decimal.RequireFromString("1"),
    EntryPrice:   decimal.RequireFromString("100"),
    TakeProfit:   decimal.RequireFromString("104"),
    StopLoss:     decimal.RequireFromString("98"),
})
reports, err := rt.SubmitOrderList(ctx, list)
```

预期行为：

- parent entry order 先提交；
- children 在 parent fill 前保持 held；
- 一个 exit child fill 后 sibling cancel；
- final position 与 portfolio state 由 fill events 更新。

可运行代码：[05_submit_bracket_order_backtest.go](../../examples/05_submit_bracket_order_backtest.go)。

## Portfolio Query

```go
position, ok := node.Cache().PositionByInstrument(accountID, instrumentID)
exposure := node.Portfolio().Exposure(accountID, "USDT")
realized := node.Portfolio().RealizedPnLs(accountID)
unrealized := node.Portfolio().UnrealizedPnLs(accountID)
```

不要在策略里重复计算 exposure，除非这是可丢弃的 strategy-local calculation。

可运行代码：[04_run_strategy_backtest.go](../../examples/04_run_strategy_backtest.go)
和 [06_run_live_node_with_in_memory_venue.go](../../examples/06_run_live_node_with_in_memory_venue.go)。

## Risk Rejection

```go
engine := risk.NewEngine(cache.New(), risk.Config{
    MaxOrderNotional: decimal.RequireFromString("1000"),
})
if err := engine.Check(order); err != nil {
    // rejected before reaching the execution client
}
```

正常 live path 中，`platform.Node.SubmitOrder` 会在 dispatch 到 execution client 前做检查。

可运行代码：[03_validate_risk_before_execution.go](../../examples/03_validate_risk_before_execution.go)。

## Backtest Run

```go
runner := backtest.NewRunner(backtest.Config{
    Cache: cache.New(),
    Strategies: []strategy.Strategy{
        strategy.NewTyped("id", strategyImpl),
    },
    Events: events,
})
result, err := runner.Run(ctx)
```

可运行代码：[04_run_strategy_backtest.go](../../examples/04_run_strategy_backtest.go)。

## Live Node Assembly

```go
node, err := live.NewNodeBuilder().
    AddDataClient(dataClient).
    AddExecutionClient(execClient).
    AddStrategy(strategy.NewTyped("id", impl)).
    Build()
if err != nil {
    return err
}
if err := node.Start(ctx); err != nil {
    return err
}
defer node.Stop(context.Background())
```

依赖 optional execution features 前先检查 capabilities。

可运行代码：[06_run_live_node_with_in_memory_venue.go](../../examples/06_run_live_node_with_in_memory_venue.go)。
