# 可靠量化程序指南

这是一份使用本仓库编写交易程序的实践清单，重点是可靠性，而不是信号收益。

## 1. 策略逻辑保持 Venue-Neutral

策略使用 normalized model types：

- `model.InstrumentID`，而不是 raw symbols；
- `model.OrderBook`、`model.Ticker`、`model.TradeTick`、`model.Bar`；
- `model.SubmitOrder` 和 `model.OrderList`；
- `model.OrderStatusReport`、`model.FillReport`、`model.PositionStatusReport`。

venue-specific symbol parsing、signing、endpoint options 和 account modes 应放在
SDK 或 adapter 中。

代码参考：[01_fetch_ticker_with_adapter.go](../../examples/01_fetch_ticker_with_adapter.go)。

## 2. 保存 Runtime，不创建影子基础设施

```go
func (s *Strategy) OnStart(ctx context.Context, rt strategy.Runtime) error {
    s.runtime = rt
    return rt.SubscribeOrderBookDepth(ctx, s.instrumentID, 5)
}
```

不要在 strategy 内创建独立 cache、portfolio、order factory 或 venue client。独立状态
会和 platform state drift，reconnect 后很难 reconcile。

代码参考：[04_run_strategy_backtest.go](../../examples/04_run_strategy_backtest.go)。

## 3. 使用 `OrderFactory` 创建订单

```go
order := rt.OrderFactory(accountID).Limit(
    instrumentID,
    model.OrderSideBuy,
    decimal.RequireFromString("0.25"),
    decimal.RequireFromString("100.50"),
    model.WithPostOnly(),
)
```

Bracket workflow：

```go
list := rt.OrderFactory(accountID).Bracket(model.BracketOrderRequest{
    InstrumentID: instrumentID,
    Side:         model.OrderSideBuy,
    Quantity:     decimal.RequireFromString("1"),
    EntryPrice:   decimal.RequireFromString("100"),
    TakeProfit:   decimal.RequireFromString("104"),
    StopLoss:     decimal.RequireFromString("98"),
})
_, err := rt.SubmitOrderList(ctx, list)
```

代码参考：[02_build_orders_with_order_factory.go](../../examples/02_build_orders_with_order_factory.go)
和 [05_submit_bracket_order_backtest.go](../../examples/05_submit_bracket_order_backtest.go)。

## 4. 把安全规则放进 `risk.Engine`

```go
riskEngine := risk.NewEngine(c, risk.Config{
    MaxOrderNotional:     decimal.RequireFromString("1000"),
    MaxPositionNotional:  decimal.RequireFromString("2500"),
    MaxAccountExposure:   decimal.RequireFromString("5000"),
    ExposureCurrency:     "USDT",
    MaxOpenOrders:        20,
    MaxCommandsPerWindow: 10,
    CommandRateWindow:    time.Second,
})
```

risk rejection 是正常 lifecycle outcome，策略应能观察并停止重复提交无效命令。

代码参考：[03_validate_risk_before_execution.go](../../examples/03_validate_risk_before_execution.go)。

## 5. 用 Cache 和 Portfolio 查询状态

```go
order, ok := rt.Cache().OrderByClientID(accountID, clientOrderID)
fills := rt.Cache().FillsForOrder(accountID, order.OrderID)
position, ok := rt.Cache().PositionByInstrument(accountID, instrumentID)
```

```go
exposure := rt.Portfolio().Exposure(accountID, "USDT")
realized := rt.Portfolio().RealizedPnLs(accountID)
unrealized := rt.Portfolio().UnrealizedPnLs(accountID)
```

orders、fills、positions、balances 和 exposure 属于 runtime，不应由策略重复维护。

代码参考：[06_run_live_node_with_in_memory_venue.go](../../examples/06_run_live_node_with_in_memory_venue.go)。

## 6. 回测 Lifecycle，而不只是信号

有价值的 backtest 应检查 subscriptions、order IDs、command metadata、risk acceptance
或 rejection、fills 是否只应用一次、final position、portfolio PnL 和 deterministic output。

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./backtest ./examples/... -v
```

代码参考：[04_run_strategy_backtest.go](../../examples/04_run_strategy_backtest.go)。

## 7. Live 前检查 Adapter Capabilities

检查：

- `adapter.Capabilities()`；
- [Adapter 能力](./adapter-capabilities_CN.md)；
- [Adapter 能力矩阵](../parity/adapter-capability-matrix_CN.md)。

如果程序依赖 private streams、resubscribe、order query、fill reports、position reports、
mass status 或 order lists，当 adapter 不支持时应 fail fast。

## 8. 把 Reconciliation 当作必须能力

可靠程序应检查 node/client health、暴露 unresolved discrepancies、保持 reconciliation
idempotent、不静默忽略 local/venue mismatch，并在测试和 release notes 中覆盖相关行为。

## 9. 谨慎使用 Live Write Tests

public read tests 可以默认运行。private read tests 需要 credentials。任何 place、cancel、
modify、transfer 或 mutate venue state 的测试必须由显式环境变量 opt-in。

详见 [Adapter Live Test Policy](../parity/adapter-live-test-policy_CN.md)。

## 10. Production Rollout Checklist

上线前确认：

- strategy 对 signal 和 lifecycle 有 deterministic tests；
- risk limits 配置在 strategy 外；
- adapter capabilities 满足 strategy 需要；
- live node health 被监控；
- reconnect 和 reconciliation paths 被测试；
- credentials 来自环境或 secret storage；
- mutating tests 默认关闭；
- 相关 Go tests 与 `git diff --check` 通过。
