# 模块指南

本指南说明每个主要 package 的职责、依赖关系，以及应用代码什么时候应该 import 它。

## 分层概览

```text
application / strategy code
        |
        v
live.Node or backtest.Runner
        |
        v
platform.Node
        |
        +--> strategy.Engine
        +--> data.Engine -----> venue.DataClient -----> adapter/* -----> sdk/*
        +--> risk.Engine
        +--> execution.Engine -> venue.ExecutionClient -> adapter/* -----> sdk/*
        +--> account.Reconciler
        +--> portfolio.Portfolio
        +--> cache.Cache
        +--> bus.Bus and kernel health/lifecycle helpers
```

核心边界规则：runtime 依赖 `venue` interfaces，adapter 依赖 SDK，SDK 不依赖
adapter 或 runtime packages。

## `model`

`model` 是共享类型系统，定义 identifiers、instruments、market data、orders、
order lists、commands、execution reports、account snapshots、position reports、
lifecycle events 和 validation errors。

常见类型包括：

- identifiers：`Venue`、`AccountID`、`InstrumentID`、`StrategyID`、
  `ClientOrderID`、`VenueOrderID`、`OrderListID`、`PositionID`、`CommandID`；
- orders：`SubmitOrder`、`ModifyOrder`、`CancelOrder`、`OrderList`、
  `OrderStatusReport`、`FillReport`；
- data：`Ticker`、`OrderBook`、`TradeTick`、`QuoteTick`、`Bar`、
  `MarketEvent`；
- account/position：`AccountSnapshot`、`Balance`、`PositionStatusReport`；
- command metadata：`CommandMetadata`。

交易数值使用 `shopspring/decimal`，不要用 `float64` 保存交易状态。

## `venue`

`venue` 定义 runtime 使用的规范化 interfaces。核心接口包括 `Adapter`、
`InstrumentProvider`、`DataClient`、`StreamingDataClient` 和 `ExecutionClient`。
可选接口描述 modify、query、order lists、fill reports、position reports、
mass status 和 resubscribe 等能力。

依赖可选行为前，应检查 `venue.DeclaredCapabilities`。

## `sdk`

`sdk/` 是 venue-native 协议层，负责 REST/WebSocket endpoint、request/response、
签名、listen-key、account mode 和 product-specific options。策略代码不应直接
import SDK。

## `adapter`

`adapter/` 把 SDK 行为转换为 `venue` interfaces，负责 symbol/instrument 解析、
normalized `model` 映射、precision/order validation、error mapping、market data、
private execution streams 和 capability declaration。

不支持的行为应返回 `model.ErrNotSupported` 或 wrapped equivalent。

## `cache`

`cache.Cache` 是本地 runtime 状态索引，存储 instruments、accounts、orders、fills、
positions、market data、deferred fills、residuals 和 snapshots。

策略应通过 `rt.Cache()` 查询 runtime 状态，而不是维护重复状态。

## `bus` 与 `kernel`

`bus.Bus` 负责 topic-based event fanout，常见 topic 包括 `market.data`、
`execution`、`timer` 和 `error`。`kernel` 提供 component lifecycle、clock、
health snapshots 和 message-bus primitives。

## `data`

`data.Engine` 注册 data clients、加载 instruments、管理 subscriptions、把 market
events 写入 cache/bus、支持 bar aggregation、data requests 和 health reporting。
标准 market events 包括 tickers、books、trades、quotes、bars、永续合约 funding rates
以及 custom extension data。
`data.Catalog`、`ReplayCatalog`、`MemoryCatalog` 用于历史或 replay 数据。

## `execution`

`execution.Engine` 和 `execution.Manager` 负责 command routing 与 order lifecycle：
submit、modify、cancel、query、order lists、contingent release、emulated triggers、
normalized execution events、matching-core 和 reconciliation support。

## `account`

`account.TradingAccount`、`account.OrderTracker` 和 `account.Reconciler` 负责 account
readiness、private stream events、account/order/fill/position reports、delayed
reports、external venue activity 和 unresolved discrepancy state。

## `risk`

`risk.Engine` 在正常 execution submission 前运行，检查 instrument、precision、
notional、exposure、scoped limits、reduce-only、trading state、kill switch、
throttles、duplicate client IDs 和 queue capacity。

## `portfolio`

`portfolio.Portfolio` 是 event-driven accounting 层，更新 balances、positions、
commissions、realized/unrealized PnL、marks、conversion rates、exposure 和 analyzer
records。

## `strategy`

`strategy` 是策略编写层。策略接收 `strategy.Runtime`，在 `OnStart` 订阅数据，
通过 typed callbacks 响应事件，使用 `model.OrderFactory` 创建订单，并通过 runtime
提交命令。

## `backtest`

`backtest.Runner` 用 timestamped events 回放策略，控制 deterministic clock、timers、
event ordering、command draining、matching、fill model、slippage、latency 和 result
summary。

## `live`

`live.Node` 把 `platform.Node` 包装成 live trading node，组装 data clients、
execution clients、strategies、risk、portfolio、cache、bus、reconnect policy 和
health monitoring。

## `platform`

`platform.Node` 是 live runtime orchestrator，并实现 `strategy.Runtime`。它协调 data、
execution、risk、portfolio、cache、bus、timers、subscriptions 和 reconciliation。

## `testsuite`

`testsuite` 包含 reusable contract tests 和 scorecards。窄范围行为用 package-local
tests，跨模块 claim 用 testsuite cases。
