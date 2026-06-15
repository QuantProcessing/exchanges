# exchanges

[English](./README.md) | 中文

Go 交易平台与交易所 SDK，正在按可量化的 NautilusTrader 风格运行时目标重建。

这个仓库有两个相互连接的职责：

- 提供交易所原生 SDK，以及稳定的加密货币交易所 adapter。
- 提供 Go 交易运行时；其生命周期、风险、组合、策略、回测、live node 和
  reconciliation 行为都可以对照本地 NautilusTrader 参考实现进行评分。

目标是完整、可测试、由 SDK 和 adapter 支撑的 Go 版 NautilusTrader 核心交易
工作流复刻。实现方式应保持 Go idiomatic，并以本仓库的测试和能力矩阵为准。

## 当前状态

项目通过 1000 分 Nautilus parity scorecard 跟踪完成度。必须满足以下条件，
才可以把 mandatory scope 声称为 release-ready：

- 所有 required scorecard case 通过；
- 每个 adapter capability 声明都有测试证据；
- 不支持的生命周期行为不能静默成功。

核心目标文档：

- [完整复刻主计划](./docs/plans/nautilustrader-complete-replica.md)
- [项目架构](./docs/architecture.md)
- [Master parity scorecard 指南](./docs/guides/master-parity-scorecard.md)
- [完整功能矩阵](./docs/parity/nautilustrader-complete-feature-matrix.md)
- [Adapter capability 矩阵](./docs/parity/adapter-capability-matrix.md)
- [质量门禁定义](./docs/parity/nautilus-complete-quality-gate.json)
- [发布说明模板](./docs/parity/nautilus-release-notes-template.md)

可运行门禁：

```bash
bash scripts/verify_nautilus_parity.sh
bash scripts/generate_nautilus_benchmark_report.sh
```

本地运行 Go 命令时，建议把 Go build cache 放到仓库外：

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./testsuite -run 'TestNautilusMaster'
```

## 架构

仓库围绕三个边界组织。

### SDK 层

交易所本地 `sdk/` 包是 venue-native protocol client。它们尽量贴近官方 REST
和 WebSocket API，可以暴露交易所特定 endpoint、请求字段、响应结构、签名规则
和产品概念。

当你需要直接访问官方交易所 API 时，使用这一层。

### Adapter 层

`adapter/` 和 `venue/` 包提供稳定的跨交易所便利接口，用于常见交易工作流。

Adapter 负责：

- instrument 和 market 解析；
- 交易所原生模型到统一 `model` 的映射；
- 订单校验和通用错误映射；
- REST 和 WebSocket 便利方法；
- 诚实的 `venue.DeclaredCapabilities` 声明。

Adapter 不镜像每一个 SDK endpoint。新的能力族应先通过小型可选接口表达，
再考虑进入核心跨交易所契约。

### Runtime 层

Nautilus 风格运行时构建在 SDK 和 adapter 之上：

| Package | 职责 |
| --- | --- |
| `model` | identifiers、instruments、commands、orders、reports、events、data types |
| `cache` | 权威运行时状态和索引 |
| `kernel` | component lifecycle、clock、health、message bus primitives |
| `bus` | event dispatch 和 fanout |
| `data` | data catalog、historical requests、live subscriptions、aggregation |
| `execution` | command routing、matching、emulation、lifecycle、reconciliation |
| `account` | trading-account readiness、order tracking、stream reconciliation |
| `risk` | pre-execution checks、limits、kill switch、reduce-only、throttling |
| `portfolio` | balances、positions、commissions、exposure、realized/unrealized PnL |
| `strategy` | strategy runtime、typed callbacks、order factory、timers |
| `backtest` | deterministic venue loop、matching、fees、slippage、latency |
| `live` | live node wiring、retry、reconnect、shutdown、health |
| `platform` | data、execution、risk、portfolio 之上的高层 node facade |
| `testsuite` | 可复用 parity、adapter、runtime、benchmark、release gates |

## Nautilus Parity Contract

NautilusTrader 是行为参考契约。Go 实现是 clean-room 且 idiomatic 的，但在支持
范围内，工作流语义应与参考实现保持一致。

核心验收语句：

> 同一个基于 Go `strategy.Runtime` 编写的策略，可以不改代码运行在 backtest
> 和 live 模式；可以提交 bracket、list 和 advanced orders；可以接收 typed
> order、fill、position、account 和 data callbacks；可以在 private stream
> 断线和启动 gap 后恢复；可以 reconciliation 缺失的 fills、orders 和 positions
> 且不重复应用状态；可以在 execution 前执行 risk；可以一致地计算 portfolio
> state 和 PnL；并且每个已声明 adapter capability 都能通过可复用 Nautilus
> parity scorecard。

Master scorecard 定义在 `testsuite.NautilusMasterRequirements()`，并由
`scripts/verify_nautilus_parity.sh` 执行。

Golden scenarios：

| ID | 场景 | 必须结果 |
| --- | --- | --- |
| A | Bracket strategy round trip | entry fill、contingent children release、sibling cancel、最终 position flat、PnL 正确 |
| B | Reconnect with missing fill | stream health 变化、gap query 只修复一次缺失 fill、状态收敛 |
| C | Position discrepancy | 检测 stale local position，重试或显式 blocked，绝不静默接受 |
| D | Risk and portfolio safety | risk 在 adapter submission 前拒绝，并阻止下游状态 mutation |
| E | Adapter capability honesty | true capability 必须有测试；unsupported surface 返回 `ErrNotSupported` |

## 支持的交易所

下表描述仓库覆盖范围。能力真相以 adapter 的 `venue.DeclaredCapabilities` 和
adapter matrix 为准，而不是这张概览表。

| Venue | Perp | Spot | Margin | 常用报价币种 |
| --- | --- | --- | --- | --- |
| Binance | 是 | 是 | 是 | USDT, USDC |
| OKX | 是 | 是 | 否 | USDT, USDC |
| Aster | 是 | 是 | 否 | USDT, USDC |
| Nado | 是 | 是 | 否 | USDT |
| Lighter | 是 | 是 | 否 | USDC |
| Hyperliquid | 是 | 是 | 否 | USDC |
| Backpack | 是 | 是 | 否 | USDC |
| Bitget | 是 | 是 | 否 | USDT, USDC |
| Bybit | 是 | 是 | 否 | USDT, USDC |
| StandX | 是 | 否 | 否 | DUSD |
| GRVT | 是 | 否 | 否 | USDT |
| EdgeX | 是 | 否 | 否 | USDC |

相关参考：

- [Adapter capabilities](./docs/guides/adapter-capabilities.md)
- [Adapter capability policy](./docs/guides/adapter-capability-policy.md)
- [Adapter live test policy](./docs/parity/adapter-live-test-policy.md)

## 安装

```bash
go get github.com/QuantProcessing/exchanges
```

当前 module 目标是 Go 1.26。

## 使用

### Adapter 级行情数据

```go
package main

import (
    "context"
    "fmt"

    "github.com/QuantProcessing/exchanges/adapter/binance"
)

func main() {
    ctx := context.Background()

    adp, err := binance.NewAdapter(ctx, binance.Options{})
    if err != nil {
        panic(err)
    }
    defer adp.Close()

    ticker, err := adp.FetchTicker(ctx, "BTC/USDT")
    if err != nil {
        panic(err)
    }

    fmt.Println(ticker.LastPrice)
}
```

### 配置驱动 adapter bootstrap

```yaml
exchanges:
  - name: BINANCE
    market_type: perp
    options:
      api_key: ${BINANCE_API_KEY}
      secret_key: ${BINANCE_SECRET_KEY}
      quote_currency: USDT

  - name: OKX
    alias: okx-spot
    market_type: spot
    options:
      api_key: ${OKX_API_KEY}
      secret_key: ${OKX_API_SECRET}
      passphrase: ${OKX_API_PASSPHRASE}
      quote_currency: USDT
```

```go
package main

import (
    "context"
    "fmt"

    exconfig "github.com/QuantProcessing/exchanges/config"
    _ "github.com/QuantProcessing/exchanges/config/all"
)

func main() {
    ctx := context.Background()

    mgr, err := exconfig.LoadManager(ctx, "exchanges.yaml")
    if err != nil {
        panic(err)
    }
    defer mgr.CloseAll()

    adp, err := mgr.GetAdapter("BINANCE")
    if err != nil {
        panic(err)
    }

    ticker, err := adp.FetchTicker(ctx, "BTC/USDT")
    if err != nil {
        panic(err)
    }

    fmt.Println(ticker.LastPrice)
}
```

### Nautilus 风格运行时示例

- [Nautilus 风格 bracket strategy](./examples/nautilus_style)
- [Go vs Nautilus 用法对照](./examples/usage_comparison)
- [量化开发者使用案例](./docs/guides/quant-use-cases.md)
- [Strategy authoring guide](./docs/guides/strategy-authoring.md)
- [Backtesting guide](./docs/guides/backtesting.md)
- [Live trading guide](./docs/guides/live-trading.md)
- [Reconciliation guide](./docs/guides/reconciliation.md)
- [Stream health guide](./docs/guides/stream-health.md)

## 验证

使用能够证明当前 claim 的最小 gate。

Targeted master scorecard：

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./testsuite -run 'TestNautilusMaster'
```

完整 Nautilus parity gate：

```bash
bash scripts/verify_nautilus_parity.sh
```

Benchmark 和 adapter-contract report：

```bash
bash scripts/generate_nautilus_benchmark_report.sh
```

示例级 smoke tests：

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./examples/...
```

## Live Test Policy

公开 SDK read tests 可以调用真实公开交易所 endpoint。Private read tests 需要
credentials，缺失时应 skip。Live write tests 默认绝不能执行；它们必须要求交易所
特定 enable flag 和 credentials，并使用 `internal/testenv.RequireLiveWrite`。

不支持的行为必须返回 `model.ErrNotSupported`，或返回可通过 `errors.Is` 匹配的
wrapped equivalent。

## 开发规则

- 保持 SDK、adapter、runtime 边界清晰。
- 优先使用 quote-aware 或 instrument-aware routing，而不是 base-symbol-only API。
- 没有 private stream 和 reconciliation 证据，不得声明 adapter lifecycle readiness。
- 修改 lifecycle、risk、portfolio、reconciliation 或 adapter capability 行为前先加测试。
- capability claim 变化时，同步更新 parity scorecard、capability matrix 和 runnable gates。

## 相关文档

- [SDK README](./sdk/README.md)
- [文档索引](./docs/README.md)
- [项目架构](./docs/architecture.md)
- [量化开发者使用案例](./docs/guides/quant-use-cases.md)
- [Adapter capabilities](./docs/guides/adapter-capabilities.md)
- [Complete feature matrix](./docs/parity/nautilustrader-complete-feature-matrix.md)
- [Adapter capability matrix](./docs/parity/adapter-capability-matrix.md)
- [Side-by-side Nautilus and Go examples](./docs/guides/side-by-side-nautilus-go-examples.md)
