# 策略编写

策略是普通 Go 值，由 `strategy.NewTyped` 包装。策略在 `OnStart` 中接收
`strategy.Runtime`，订阅数据，通过 typed callbacks 响应事件，使用
`model.OrderFactory` 创建订单，并通过 runtime 提交命令。

## 心智模型

```text
OnStart
  -> 保存 strategy.Runtime
  -> 订阅数据

market callback
  -> 计算信号
  -> 创建 model.SubmitOrder 或 model.OrderList
  -> SubmitOrder / SubmitOrderList

execution callback
  -> 观察订单、成交、仓位生命周期
  -> 只更新策略本地信号状态
```

策略不要直接调用 venue SDK。通过 runtime 提交命令，risk、execution、cache、
portfolio、event callbacks 和 reconciliation 才会走同一条路径。

## 最小 Typed Strategy

```go
type ImbalanceStrategy struct {
    runtime      strategy.Runtime
    accountID    model.AccountID
    instrumentID model.InstrumentID
    submitted    bool
}

func (s *ImbalanceStrategy) OnStart(ctx context.Context, rt strategy.Runtime) error {
    s.runtime = rt
    return rt.SubscribeOrderBookDepth(ctx, s.instrumentID, 2)
}

func (s *ImbalanceStrategy) OnOrderBook(ctx context.Context, book model.OrderBook) error {
    if book.InstrumentID != s.instrumentID || s.submitted {
        return nil
    }
    if len(book.Bids) == 0 || len(book.Asks) == 0 {
        return nil
    }

    bidSize := book.Bids[0].Size
    askSize := book.Asks[0].Size
    if !bidSize.GreaterThan(askSize.Mul(decimal.NewFromInt(2))) {
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

注册到 backtest runner 或 live node：

```go
strat := strategy.NewTyped("imbalance", &ImbalanceStrategy{
    accountID:    "main",
    instrumentID: model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
})
```

可运行代码：[04_run_strategy_backtest.go](../../examples/04_run_strategy_backtest.go)
和 [06_run_live_node_with_in_memory_venue.go](../../examples/06_run_live_node_with_in_memory_venue.go)。

## 支持的 Typed Callbacks

Market data callbacks：

- `OnTicker(context.Context, model.Ticker) error`
- `OnOrderBook(context.Context, model.OrderBook) error`
- `OnTradeTick(context.Context, model.TradeTick) error`
- `OnQuoteTick(context.Context, model.QuoteTick) error`
- `OnBar(context.Context, model.Bar) error`
- `OnCustomData(context.Context, model.CustomData) error`

Execution callbacks：

- `OnAccount(context.Context, model.AccountSnapshot) error`
- `OnOrderStatus(context.Context, model.OrderStatusReport) error`
- `OnOrderSubmitted`、`OnOrderAccepted`、`OnOrderPartiallyFilled`、
  `OnOrderCanceled`、`OnOrderRejected`
- `OnOrderLifecycle(context.Context, model.OrderLifecycleEvent) error`
- `OnOrderFilled(context.Context, model.FillReport) error`
- `OnPosition(context.Context, model.PositionStatusReport) error`
- `OnPositionLifecycle`、`OnPositionOpened`、`OnPositionChanged`、
  `OnPositionClosed`

Runtime callbacks：

- `OnTimer(context.Context, strategy.TimerEvent) error`
- `OnError(context.Context, strategy.ErrorEvent) error`
- `OnEvent(context.Context, bus.Envelope) error`
- `OnStop(context.Context) error`

## Runtime API 清单

使用 `strategy.Runtime` 完成：

- `Cache()` 与 `Portfolio()` 状态查询；
- `Clock()` runtime time；
- `SetTimer` 与 `CancelTimer`；
- market-data subscriptions；
- historical/catalog-backed data requests；
- `OrderFactory(accountID)`；
- submit、modify、cancel、batch-cancel、cancel-all、query commands；
- account queries。

## 策略状态规则

适合放在策略本地的状态：signal flags、rolling windows、indicator values、debouncing、
策略主动持有的 order IDs、上次信号触发时间。

应该从 runtime 查询的状态：open orders、fills、positions、account balances、exposure、
realized/unrealized PnL、venue stream health。

## Brackets 与 Order Lists

entry + take-profit/stop-loss 工作流见：

- [Bracket 策略编写](./strategy-authoring-bracket_CN.md)
- [05_submit_bracket_order_backtest.go](../../examples/05_submit_bracket_order_backtest.go)

## Verification

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./strategy ./examples/... -v
```
