# 运行时流程

本文解释模块在运行时如何协作。读完 [模块指南](./module-guide_CN.md) 后，如果想
知道数据如何在系统里流动，就看这一页。

## 行情流程

```text
venue.StreamingDataClient or venue.DataClient
        |
        v
data.Engine
        |
        +--> cache.Cache stores instruments and latest market events
        |
        +--> bus.Bus publishes model.MarketEvent on strategy.TopicMarketData
        |
        v
strategy.Engine dispatches typed callbacks
```

策略调用 `SubscribeTicker`、`SubscribeOrderBookDepth`、`SubscribeTradeTicks`、
`SubscribeQuoteTicks` 或 `SubscribeBars` 后，platform 会把 subscription 路由到对应
streaming data client。venue event 被映射为 `model.MarketEvent`，写入 cache，
发布到 bus，并由 `strategy.NewTyped` 分发到 typed callbacks。

## 订单流程

```text
strategy callback
        |
        v
strategy.Runtime.SubmitOrder or SubmitOrderList
        |
        v
platform.Node adds metadata and runs risk.Engine
        |
        v
execution.Engine and venue.ExecutionClient
        |
        v
adapter maps model command to SDK request
        |
        v
venue API or simulated backtest matching
        |
        v
model.ExecutionEvent
        |
        +--> cache.Cache
        +--> portfolio.Portfolio
        +--> bus.Bus
        +--> strategy callbacks
```

策略不需要知道 command 最终由 live adapter 还是 backtest matching path 处理。正常
runtime path 中，risk 会在 execution 前运行；被拒绝的订单必须可见，并且不能造成
order、fill、position 或 portfolio mutation。

## Order List 与 Bracket 流程

`model.OrderFactory.Bracket` 创建一个 parent entry order 和两个 exit children。
三者共享 `OrderListID`；children 携带 parent client order ID 和 reduce-only exit
flags。

```text
strategy creates model.OrderList
        |
        v
SubmitOrderList
        |
        +--> parent is submitted
        +--> children are indexed and held
        |
        v
parent fill event
        |
        +--> children released
        |
        v
take-profit or stop-loss fill
        |
        +--> sibling is canceled
```

## 成交与组合流程

```text
venue fill report or simulated fill
        |
        v
model.FillReport
        |
        +--> account.Reconciler deduplicates and repairs local state
        +--> cache.Cache indexes the fill
        +--> portfolio.Portfolio updates balances, positions, fees, PnL
        +--> bus.Bus publishes execution event
        +--> strategy.OnOrderFilled receives the callback
```

成交对 identity 很敏感。只要 venue 提供 trade ID、account ID、instrument ID、
order ID、client order ID 或 venue order ID，runtime 都应尽量保留。reconciliation
不能重复应用同一笔成交。

## Reconciliation 流程

```text
startup, reconnect, periodic audit, or explicit request
        |
        v
execution/account reconciliation
        |
        v
venue reports: account, orders, fills, positions, mass status
        |
        v
cache and portfolio repair
        |
        v
audit trail records resolved and unresolved discrepancies
```

Reconciliation 让 runtime 在 private stream disconnect、startup gap、delayed
reports、fill-before-order、external venue activity 和 stale local position 后收敛。
无法安全修复的差异会成为结构化 unresolved discrepancy。

## Backtest 流程

```text
[]backtest.Event
        |
        v
backtest.Runner
        |
        +--> advances deterministic clock
        +--> dispatches timers
        +--> records market events
        +--> matches open orders
        +--> dispatches strategy callbacks
        +--> drains same-timestamp commands
        |
        v
backtest.Result
```

当输入 events、timestamps、fill model 和策略代码确定时，backtest 也应是确定性的。

## Live 流程

```text
live.Node.Start
        |
        +--> create/default bus, cache, risk, portfolio
        +--> start platform
        +--> load instruments
        +--> connect data clients
        +--> connect execution clients
        +--> query account state
        +--> start strategies
        +--> monitor health
```

`live.Node.Stop` 会先停止 strategies，再断开 clients，这样 `OnStop` 仍可检查 runtime
state。
