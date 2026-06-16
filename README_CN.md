# exchanges 中文文档

这是一个 Go 交易所 SDK、能力声明诚实的 adapter，以及受 NautilusTrader 策略、
执行、风控、组合、回测和 live node 工作流风格启发的交易运行时。

仓库有两个相互连接的职责：

- 提供交易所原生 SDK 和规范化的加密货币交易所 adapter；
- 提供 Go 交易运行时，让同一个基于 `strategy.Runtime` 的策略形状可以用于
  确定性回测和 live node 组装。

实现是 clean-room 且 Go idiomatic 的。NautilusTrader 只在 README 中作为行为
风格参考；`docs/` 站点会以本项目自身的方式解释如何使用。

## 新人从这里开始

建议按这个顺序阅读：

1. [快速开始](./docs/getting-started_CN.md)：安装、拉行情、写策略、跑回测、
   组装 live node。
2. [模块指南](./docs/module-guide_CN.md)：每个包的职责，以及什么时候该 import
   哪一层。
3. [运行时流程](./docs/runtime-flow_CN.md)：行情、订单、成交、风控、组合和
   reconciliation 如何流动。
4. [可靠量化程序指南](./docs/guides/reliable-quant-program_CN.md)：写可靠
   量化程序的实践清单。
5. [示例](./examples/README_CN.md)：从简单 adapter 行情到 in-memory live node
   的可运行 API recipes。
6. [文档索引](./docs/README_CN.md)：完整文档站地图。

## 当前状态

仓库已经有 SDK、adapter、model、cache、data、execution、account、risk、
portfolio、strategy、backtest、live、platform 和 testsuite 的第一批实现。很多
功能仍然在矩阵中标记为 `Partial` 或 `Planned`。

不要因为某个 package 存在就默认它已经 production-complete。能力真相来自：

- [完整功能矩阵](./docs/parity/complete-feature-matrix_CN.md)
- [Adapter 能力矩阵](./docs/parity/adapter-capability-matrix_CN.md)
- [主评分卡](./docs/guides/master-scorecard_CN.md)
- [完整质量门](./docs/parity/complete-quality-gate_CN.json)

长期目标通过 `testsuite` 中的 1000 分 NautilusTrader 风格 parity scorecard 追踪。
只有 mandatory cases 全部通过、adapter capability 均有证据、不支持的生命周期
行为显式返回错误时，才可以声称 release-ready。

## 架构速览

```text
strategy code
        |
        v
strategy.Runtime
        |
        +--> backtest.Runner
        |
        +--> live.Node
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

三层边界：

- `sdk/`：交易所原生协议 client；
- `adapter/` 和 `venue/`：规范化跨交易所接口和能力声明；
- runtime packages：strategy、data、execution、account、risk、portfolio、cache、
  backtest、live、platform、bus、kernel。

详见 [项目架构](./docs/architecture_CN.md) 和
[模块指南](./docs/module-guide_CN.md)。

## 支持的交易所族

当前仓库覆盖 Binance、Aster、OKX、Bybit、Bitget、Hyperliquid、Lighter、Nado、
EdgeX、GRVT、StandX 和 Backpack 的 SDK/adapter。

能力真相不是这句话，而是 adapter 的 `venue.DeclaredCapabilities` 与
[Adapter 能力矩阵](./docs/parity/adapter-capability-matrix_CN.md)。
永续合约 funding-rate snapshot 支持也按产品族记录在矩阵里：spot adapter 不声明该能力，
`FundingRateStream` 仍是单独且尚未声明的 capability。

## 安装

```bash
go get github.com/QuantProcessing/exchanges
```

当前 module 目标是 Go 1.26。

## 快速示例：Adapter 行情

```go
package main

import (
    "context"
    "fmt"

    "github.com/QuantProcessing/exchanges/adapter/binance"
    "github.com/QuantProcessing/exchanges/model"
)

func main() {
    ctx := context.Background()

    adp, err := binance.NewSpotAdapter(ctx, binance.Options{})
    if err != nil {
        panic(err)
    }
    defer adp.Close(ctx)

    ticker, err := adp.Data().FetchTicker(ctx, model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"))
    if err != nil {
        panic(err)
    }

    fmt.Println(ticker.Last)
}
```

这个模式的可编译版本在
[01_fetch_ticker_with_adapter.go](./examples/01_fetch_ticker_with_adapter.go)。

## 快速示例：策略形状

```go
type Strategy struct {
    runtime      strategy.Runtime
    accountID    model.AccountID
    instrumentID model.InstrumentID
}

func (s *Strategy) OnStart(ctx context.Context, rt strategy.Runtime) error {
    s.runtime = rt
    return rt.SubscribeOrderBookDepth(ctx, s.instrumentID, 2)
}

func (s *Strategy) OnOrderBook(ctx context.Context, book model.OrderBook) error {
    if len(book.Asks) == 0 {
        return nil
    }
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

继续阅读：

- [策略编写](./docs/guides/strategy-authoring_CN.md)
- [回测](./docs/guides/backtesting_CN.md)
- [实盘交易](./docs/guides/live-trading_CN.md)
- [示例](./examples/README_CN.md)

## NautilusTrader 风格契约

目标行为是：

> 同一个基于 Go `strategy.Runtime` 编写的策略，可以不改代码运行在 backtest 和
> live 模式；可以提交 bracket、list 和 advanced orders；可以接收 typed order、
> fill、position、account 和 data callbacks；可以在 private stream 断线和启动
> gap 后恢复；可以 reconciliation 缺失的 fills、orders 和 positions 且不重复
> 应用状态；可以在 execution 前执行 risk；可以一致地计算 portfolio state 和
> PnL；并且每个已声明 adapter capability 都能通过可复用 NautilusTrader 风格
> scorecard。

Golden scenarios：

| ID | 场景 | 必须结果 |
| --- | --- | --- |
| A | Bracket strategy round trip | entry fill、contingent children release、sibling cancel、最终 position flat、PnL 正确 |
| B | Reconnect with missing fill | stream health 变化、gap query 只修复一次缺失 fill、状态收敛 |
| C | Position discrepancy | 检测 stale local position，重试或显式 blocked，绝不静默接受 |
| D | Risk and portfolio safety | risk 在 adapter submission 前拒绝，并阻止下游状态 mutation |
| E | Adapter capability honesty | true capability 必须有测试；unsupported surface 返回 `ErrNotSupported` |

## 验证

使用能够证明当前 claim 的最小 gate：

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./examples/...
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./strategy ./backtest ./live ./platform
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./venue ./testsuite ./adapter/... ./config/all -run 'Adapter|Capability|Contract|PrivateStream|Resubscribe'
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./testsuite -run 'TestNautilusMaster'
git diff --check
```

完整本地门禁：

```bash
bash scripts/verify_nautilus_parity.sh
bash scripts/generate_nautilus_benchmark_report.sh
```

## Live Test Policy

公开 SDK read tests 可以调用真实公开交易所 endpoint。Private read tests 需要
credentials，缺失时应 skip。Live write tests 默认绝不能执行；它们必须要求交易所
特定 enable flag 和 credentials，并使用 `internal/testenv.RequireLiveWrite`。

不支持的行为必须返回 `model.ErrNotSupported`，或返回可通过 `errors.Is` 匹配的
wrapped equivalent。

## 相关文档

- [SDK README](./sdk/README_CN.md)
- [示例](./examples/README_CN.md)
- [文档索引](./docs/README_CN.md)
- [项目架构](./docs/architecture_CN.md)
- [模块指南](./docs/module-guide_CN.md)
- [量化开发者使用场景](./docs/guides/quant-use-cases_CN.md)
- [Adapter 能力](./docs/guides/adapter-capabilities_CN.md)
- [平台完成计划](./docs/plans/platform-completion-plan_CN.md)
