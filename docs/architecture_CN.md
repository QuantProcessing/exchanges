# 项目架构

本项目是分层的 Go 交易平台，不是扁平的 exchange wrapper。仓库把 venue-native
protocol clients、normalized venue adapters，以及拥有 data、execution、risk、
portfolio、cache、reconciliation、backtest 和 live operation 的 runtime 层分开。

## 设计目标

- 将交易所协议细节留在 `sdk/` packages。
- 将跨 venue 契约放在 `venue` 与 `adapter`。
- 让策略使用 normalized `model` types，而不是 venue payloads。
- 交易命令必须经过 risk、execution、cache、portfolio 和 event publication。
- backtest 与 live mode 使用同一套 strategy authoring surface。
- 不支持的行为必须显式返回错误，而不是 no-op success。
- 本地状态修复必须可通过 reconciliation results 和 audit trails 检查。

## 分层图

```text
Application code
        |
        v
strategy.Strategy implementations
        |
        v
strategy.Runtime
        |
        +--> backtest.Runner for deterministic simulation
        |
        +--> live.Node for live operation
                 |
                 v
             platform.Node
                 |
                 +--> data.Engine -----> venue.DataClient -----> adapter/* -----> sdk/*
                 +--> execution.Engine -> venue.ExecutionClient -> adapter/* -----> sdk/*
                 +--> account.Reconciler
                 +--> risk.Engine
                 +--> portfolio.Portfolio
                 +--> cache.Cache
                 +--> bus.Bus
```

依赖方向：

1. runtime packages 依赖 `venue` abstractions；
2. adapters 依赖 SDK，并把 venue-native payloads 映射到 `model`；
3. SDK 只处理交易所协议，不 import adapter 或 runtime packages。

## 三个边界

### SDK 边界

`sdk/` packages 负责官方 REST/WebSocket endpoint names、签名、request/response
types、listen-key flows、account modes 和 product-specific options。

### Adapter 边界

`adapter/` packages 通过 `venue` interfaces 暴露 SDK 行为，负责 instrument lookup、
raw symbol mapping、normalized market data、order validation、error mapping、
private execution streams 和 `venue.DeclaredCapabilities`。

### Runtime 边界

Runtime packages 负责 trading behavior：strategy callbacks、command metadata、
risk checks、execution lifecycle、reconciliation、cache indexes、portfolio
accounting、backtesting、live node health、timers 和 event fanout。

## 核心模块

- `model`：共享类型系统和 identity preservation。
- `venue`：runtime 可依赖的稳定 interfaces 和 optional capability interfaces。
- `cache`：本地 runtime state index。
- `data`：data clients、subscriptions、cache/bus forwarding、aggregation 和 catalog。
- `execution`：command routing、order lifecycle、order lists、emulated triggers。
- `account`：account readiness、private streams、reports、unresolved discrepancies。
- `risk`：execution boundary 前的 pre-trade checks。
- `portfolio`：balances、positions、fees、PnL、marks、exposure。
- `strategy`：开发者编写策略的 typed callback surface。
- `backtest`：timestamped event replay 和 deterministic simulation。
- `live`：live node startup/shutdown、reconnect、health。
- `platform`：live runtime facade，实现 `strategy.Runtime`。
- `testsuite`：跨 package contract tests 和 scorecards。

## Strategy Order Flow

```text
strategy callback
        |
        v
strategy.Runtime.SubmitOrder / SubmitOrderList
        |
        v
platform.Node fills command metadata and checks risk
        |
        v
execution.Engine sends command to venue.ExecutionClient
        |
        v
adapter maps model command to SDK request
        |
        v
venue API or simulated execution path
        |
        v
order/fill/position reports return as model.ExecutionEvent
        |
        +--> cache.Cache
        +--> portfolio.Portfolio
        +--> bus.Bus -> strategy callbacks
```

核心 invariant：策略代码不直接调用 SDK。命令通过 runtime，才能保证 risk、cache、
portfolio、lifecycle 和 metadata 保持一致。

## Reconciliation Flow

```text
startup, reconnect, periodic audit, or explicit request
        |
        v
account.Reconciler / execution reconciliation
        |
        v
venue.ExecutionClient reports
        |
        v
cache and portfolio repair
        |
        v
audit trail records resolved and unresolved discrepancies
```

Unresolved discrepancies 是结构化状态。missing fills 必须只应用一次；
fill-before-order reports 应被 defer，并在 order 出现后 replay。

## Backtest 与 Live 等价面

Backtest 和 live 共享 `strategy.Runtime`、normalized model types、order factory、
typed callbacks、cache queries、portfolio queries、risk checks、lifecycle events
和 command metadata。

不同之处在于 source of truth：backtest 使用 deterministic events 和 simulated
matching；live 使用 registered venue data/execution clients。

## Verification

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./strategy ./backtest ./live ./platform
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./account ./risk ./portfolio ./testsuite
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./venue ./adapter/... ./config/all ./testsuite -run 'Adapter|Capability|Contract|PrivateStream|Resubscribe'
git diff --check
```
