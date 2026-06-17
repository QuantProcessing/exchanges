# Adapter 能力矩阵

本矩阵是 adapter capability honesty 的 source of truth。它反映当前
`venue.DeclaredCapabilities` claims，以及 adapter 被视为支持某个 workflow 前所需的
test evidence。

## Capability Key

| Mark | Meaning |
| --- | --- |
| Yes | adapter 当前声明支持，必须有 contract tests。 |
| No | 当前不声明支持；调用方应收到显式 unsupported behavior。 |
| Planned | 实现目标，不是当前支持。 |
| External | 当前 repository SDK universe 之外。 |

## Current Repository Adapters

| Adapter | SDK package | Adapter package | Instruments | Data snapshots | Data streams | Funding snapshots | Funding stream | Account snapshot | Submit | Cancel | Modify | Query | Order reports | Fill reports | Position reports | Private stream | Resubscribe | Mass status | Order lists | Contract evidence |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Binance Spot | sdk/binance | adapter/binance | Yes | Yes | Yes | No | No | Yes | Yes | Yes | No | No | Yes | No | No | Yes | Yes | Planned | Planned | adapter/binance/spot_test.go; testsuite/contracts.go |
| Binance Perp | sdk/binance | adapter/binance | Yes | Yes | Yes | Yes | No | Yes | Yes | Yes | No | No | Yes | No | No | Yes | Yes | Planned | Planned | adapter/binance/perp_test.go; testsuite/contracts.go |
| Aster Spot | sdk/aster | adapter/aster | Yes | Yes | Yes | No | No | Yes | Yes | Yes | No | No | Yes | No | No | Yes | Yes | Planned | Planned | adapter/aster/aster_test.go; testsuite/contracts.go |
| Aster Perp | sdk/aster | adapter/aster | Yes | Yes | Yes | Yes | No | Yes | Yes | Yes | No | No | Yes | No | No | Yes | Yes | Planned | Planned | adapter/aster/aster_test.go; testsuite/contracts.go |
| OKX Spot | sdk/okx | adapter/okx | Yes | Yes | Yes | No | No | Yes | Yes | Yes | No | No | Yes | No | No | Yes | Yes | Planned | Planned | adapter/okx/okx_test.go; testsuite/contracts.go |
| OKX Swap | sdk/okx | adapter/okx | Yes | Yes | Yes | Yes | No | Yes | Yes | Yes | No | No | Yes | No | No | Yes | Yes | Planned | Planned | adapter/okx/okx_test.go; testsuite/contracts.go |
| Bybit Spot | sdk/bybit | adapter/bybit | Yes | Yes | Yes | No | No | Yes | Yes | Yes | No | No | Yes | No | No | Yes | Yes | Planned | Planned | adapter/bybit/bybit_test.go; testsuite/contracts.go |
| Bybit Linear | sdk/bybit | adapter/bybit | Yes | Yes | Yes | Yes | No | Yes | Yes | Yes | No | No | Yes | No | No | Yes | Yes | Planned | Planned | adapter/bybit/bybit_test.go; testsuite/contracts.go |
| Bitget Spot | sdk/bitget | adapter/bitget | Yes | Yes | Yes | No | No | Yes | Yes | Yes | No | No | Yes | No | No | Yes | Yes | Planned | Planned | adapter/bitget/bitget_test.go; testsuite/contracts.go |
| Bitget Perp | sdk/bitget | adapter/bitget | Yes | Yes | Yes | Yes | No | Yes | Yes | Yes | No | No | Yes | No | No | Yes | Yes | Planned | Planned | adapter/bitget/bitget_test.go; testsuite/contracts.go |
| Hyperliquid Spot | sdk/hyperliquid | adapter/hyperliquid | Yes | Yes | Yes | No | No | Yes | Yes | Yes | No | No | Yes | No | No | Yes | Yes | Planned | Planned | adapter/hyperliquid/hyperliquid_test.go; testsuite/contracts.go |
| Hyperliquid Perp | sdk/hyperliquid | adapter/hyperliquid | Yes | Yes | Yes | Yes | No | Yes | Yes | Yes | No | No | Yes | No | No | Yes | Yes | Planned | Planned | adapter/hyperliquid/hyperliquid_test.go; testsuite/contracts.go |
| Lighter | sdk/lighter | adapter/lighter | Yes | Yes | Yes | Yes | No | Yes | Yes | Yes | No | No | Yes | No | No | Yes | Yes | Planned | Planned | adapter/lighter/lighter_test.go; testsuite/contracts.go |
| Nado | sdk/nado | adapter/nado | Yes | Yes | Yes | Yes | No | Yes | Yes | Yes | No | No | Yes | No | No | Yes | Yes | Planned | Planned | adapter/nado/nado_test.go; testsuite/contracts.go |
| EdgeX | sdk/edgex | adapter/edgex | Yes | Yes | Yes | Yes | No | Yes | Yes | Yes | No | No | Yes | No | No | Yes | Yes | Planned | Planned | adapter/edgex/edgex_test.go; testsuite/contracts.go |
| GRVT | sdk/grvt | adapter/grvt | Yes | Yes | Yes | Yes | No | Yes | Yes | Yes | No | No | Yes | No | No | Yes | Yes | Planned | Planned | adapter/grvt/grvt_test.go; testsuite/contracts.go |
| StandX | sdk/standx | adapter/standx | Yes | Yes | Yes | Yes | No | Yes | Yes | Yes | No | No | Yes | No | No | Yes | Yes | Planned | Planned | adapter/standx/standx_test.go; testsuite/contracts.go |
| Backpack | sdk/backpack | adapter/backpack | Yes | Yes | Yes | Yes | No | Yes | Yes | Yes | No | No | Yes | No | No | Yes | Yes | Planned | Planned | adapter/backpack/backpack_test.go; testsuite/contracts.go |

## Funding Snapshot Semantics

`Funding snapshots = Yes` 表示 adapter data client 实现了
`venue.FundingRateProvider`，并且 `FetchFundingRate` 会为永续合约返回标准化
`model.FundingRate`。这不代表已经支持资金费率 stream；只有 stream-backed
implementation 与测试存在后，`Funding stream` 才能从 `No` 改成 `Yes`。

当前 snapshot-backed providers：Binance Perp、Aster Perp、OKX Swap、Bybit
Linear、Bitget Perp、Hyperliquid Perp、Lighter、EdgeX、GRVT、StandX、Backpack。

如果 venue 提供当前全 symbol funding 或 ticker snapshot，adapter 必须使用这个批量来源，
再在本地筛选目标 instrument，不能挨个 symbol 轮询。StandX 使用
`/api/query_market_overview`；该批量响应不提供 next funding time，所以该字段保留零值。
GRVT 仍需要人工核对是否存在官方批量 current funding/ticker endpoint；当前实测
`/lite/v1/ticker` 需要传入 instrument。

`model.FundingRate` 只表示当前资费信息。adapter 不能用历史 funding rows 来满足这个
capability。mark price 和 index price 不属于 `model.FundingRate`；需要这些参考价的策略应使用普通
market data capability。

## Extension Targets Outside Current SDK Scope

这些行不计为 supported repository adapters。只有 SDK modules、adapter packages 和
contract tests 存在后，才能声明支持。

| Extension target | Current repository owner | Status | Required before support can be claimed |
| --- | --- | --- | --- |
| Betfair | adapter extension | External | SDK module、instrument model extension、data client、execution client、account reports、adapter contract tests。 |
| BitMEX | adapter extension | External | SDK module、market data client、execution client、private stream、report generators、adapter contract tests。 |
| Databento | data-provider extension | External | Data catalog/provider abstraction、schema mapping、replay tests、data engine contract tests。 |
| Deribit | adapter extension | External | SDK module、options/futures instrument coverage、execution reports、adapter contract tests。 |
| dYdX | adapter extension | External | SDK module、market data client、execution client、private stream、adapter contract tests。 |
| Interactive Brokers | adapter extension | External | SDK/gateway layer、multi-asset instrument coverage、account/position reports、adapter contract tests。 |
| Kraken | adapter extension | External | SDK module、spot/futures market data、execution client、private stream、adapter contract tests。 |
| Polymarket | adapter extension | External | SDK module、market/instrument model extension、data client、execution client、adapter contract tests。 |
| Sandbox | backtest/live fake clients | Planned | Deterministic simulated venue、fake execution reports、private-stream test harness、live node tests。 |
| Tardis | data-provider extension | External | Data provider module、historical catalog mapping、replay tests、data engine contract tests。 |

## Required Verification Commands

| Purpose | Command |
| --- | --- |
| Capability matrix and scorecard metadata | `go test -count=1 ./testsuite -run 'Master|Score|Requirement'` |
| Adapter contract tests | `go test -count=1 ./adapter/... ./config/all ./testsuite -run 'Adapter|Capability|Contract'` |
| SDK compile-only check | `go test -run '^$' -count=1 ./sdk/...` |
| Public SDK read tests | `go test -count=1 ./sdk/...` |
| Live write tests | 仅在对应 venue-specific write flag 与 credentials 存在时运行。 |

## Capability Policy

- `Yes` capability 必须有 passing shared contract case。
- `No` capability 不应被 caller 视为 lifecycle requirement。
- `Planned` 是实现目标，不是当前支持。
- private stream 与 resubscribe 是不同 claims。
- fill、position、mass-status、order-list support 必须在 SDK-backed implementation 与
  shared tests 后才能声明。
