# 完整功能矩阵

本矩阵将 platform surfaces 映射到 package owners、rollout status 和 acceptance
evidence。它是维护者文档；普通用户应先阅读 [文档索引](../README_CN.md) 中的指南。

## 状态说明

| Status | Meaning |
| --- | --- |
| Contracted | testsuite owner 已存在，能力属于 master gate。 |
| Partial | 当前代码覆盖了部分 surface，仍需 owner 扩展。 |
| Planned | 尚无完整实现，但 owner 与 acceptance path 已分配。 |
| External extension | 架构可支持，但当前仓库不声明 adapter support。 |

## Core Platform Surfaces

| Surface | Go owner | Status | Acceptance suite | Result expectation |
| --- | --- | --- | --- | --- |
| model | model | Partial | testsuite/model_tester.go / TC-M01..TC-M06 | identifiers、instruments、orders、reports、events、money、data types 通过 validation 与 round trip。 |
| model/events | model | Partial | testsuite/model_tester.go / TC-M03 | order、fill、position、account、lifecycle events 保留 command metadata。 |
| model/instruments | model | Partial | testsuite/model_tester.go / TC-M02 | spot、perpetual、future-safe derivatives 和 extension instruments 有精确 metadata validation。 |
| model/orders | model | Partial | testsuite/model_tester.go / TC-M03, TC-M05 | advanced order types、order lists、OTO/OCO/OUO/bracket semantics 与 contingency metadata 被建模。 |
| cache runtime state | cache | Planned | testsuite/cache_tester.go / TC-C01..TC-C07 | orders、fills、positions、accounts、instruments、data snapshots、deferred fills 有索引，避免全局扫描。 |
| command envelope | model,bus,kernel | Planned | testsuite/command_tester.go / TC-CMD01..TC-CMD05 | command IDs、correlation IDs、trader/strategy/client IDs、timestamps、params、list/position IDs 贯穿全路径。 |
| execution | execution,account | Partial | testsuite/exec_tester.go; testsuite/lifecycle_tester.go / TC-E*, TC-L* | submit/modify/cancel/query/list/report flows、manager state、fills、positions、contingencies 和 lifecycle transitions 显式可测。 |
| execution/reconciliation | execution,account,live | Planned | testsuite/reconciliation_tester.go / TC-REC01..TC-REC09 | startup、periodic、reconnect、mass-status、missing-fill、external-order、discrepancy repair 可审计。 |
| live | live,platform | Partial | testsuite/live_node_tester.go / TC-LIVE01..TC-LIVE09 | live node startup、reconnect、private stream readiness、reconciliation、health、shutdown deterministic。 |
| portfolio | portfolio | Partial | testsuite/portfolio_tester.go / TC-P01..TC-P23 | account updates、balances、positions、commissions、PnL、exposure、conversion、analyzer hooks event driven。 |
| risk engine | risk | Partial | testsuite/risk_tester.go / TC-R01..TC-R23 | pre-trade checks、async queues、exposure、margin、reduce-only、kill switch、throttles、rejection events enforced。 |
| strategy runtime | strategy | Planned | testsuite/strategy_tester.go / TC-S01..TC-S09 | 同一个 strategy implementation 可在 backtest/live 中使用 typed callbacks、timers、order factory、cache、portfolio helpers。 |
| data engine/catalog | data,venue,backtest,live | Planned | testsuite/data_tester.go / TC-D01..TC-D23 | live subscriptions 与 catalog replay 共享 normalized ticks、bars、books、requests、responses、health metrics。 |
| backtest | backtest | Partial | testsuite/backtest_tester.go / TC-B01..TC-B24 | deterministic simulated venue、matching core、advanced orders、order lists、fees、slippage、latency、byte-stable results。 |
| documentation/examples | docs,examples | Planned | testsuite/documentation_artifacts_test.go / TC-DOC01..TC-DOC03 | 新人文档、模块文档、使用指南、能力文档和 examples 可导航、可执行。 |

## Adapter And Extension Surfaces

| Surface | Go owner | Status | Acceptance suite | Result expectation |
| --- | --- | --- | --- | --- |
| adapters/binance | adapter/binance,sdk/binance | Partial | adapter/binance/*_test.go; testsuite/contracts.go | spot/perpetual capabilities SDK-backed、contract-tested、capability-honest。 |
| adapters/bybit | adapter/bybit,sdk/bybit | Partial | adapter/bybit/bybit_test.go; testsuite/contracts.go | 当前 declared capabilities SDK-backed，private stream/reconciliation claims 保持 gated。 |
| adapters/hyperliquid | adapter/hyperliquid,sdk/hyperliquid | Partial | adapter/hyperliquid/hyperliquid_test.go; testsuite/contracts.go | spot/perpetual capabilities SDK-backed、contract-tested、capability-honest。 |
| adapters/okx | adapter/okx,sdk/okx | Partial | adapter/okx/okx_test.go; testsuite/contracts.go | 当前 declared capabilities SDK-backed，optional execution reports 显式。 |
| adapter/aster | adapter/aster,sdk/aster | Partial | adapter/aster/aster_test.go; testsuite/contracts.go | spot/perpetual capabilities SDK-backed、contract-tested、capability-honest。 |
| adapter/bitget | adapter/bitget,sdk/bitget | Partial | adapter/bitget/bitget_test.go; testsuite/contracts.go | 当前 declared capabilities SDK-backed，optional reports 显式。 |
| adapter/backpack | adapter/backpack,sdk/backpack | Partial | adapter/backpack/backpack_test.go; testsuite/contracts.go | 当前 declared capabilities SDK-backed，optional reports 显式。 |
| adapter/edgex | adapter/edgex,sdk/edgex | Partial | adapter/edgex/edgex_test.go; testsuite/contracts.go | 当前 declared capabilities SDK-backed，optional reports 显式。 |
| adapter/grvt | adapter/grvt,sdk/grvt | Partial | adapter/grvt/grvt_test.go; testsuite/contracts.go | 当前 declared capabilities SDK-backed，optional reports 显式。 |
| adapter/lighter | adapter/lighter,sdk/lighter | Partial | adapter/lighter/lighter_test.go; testsuite/contracts.go | 当前 declared capabilities SDK-backed，optional reports 显式。 |
| adapter/nado | adapter/nado,sdk/nado | Partial | adapter/nado/nado_test.go; testsuite/contracts.go | 当前 declared capabilities SDK-backed，optional reports 显式。 |
| adapter/standx | adapter/standx,sdk/standx | Partial | adapter/standx/standx_test.go; testsuite/contracts.go | 当前 declared capabilities SDK-backed，optional reports 显式。 |
| adapters/interactive_brokers | adapter extension | External extension | docs/parity/adapter-capability-matrix_CN.md | SDK/gateway module 和 adapter contract tests 存在前不声明支持。 |
| adapters/databento | data-provider extension | External extension | docs/parity/adapter-capability-matrix_CN.md | 需要 catalog/provider mapping 与 data engine contract tests。 |
| adapters/betfair | adapter extension | External extension | docs/parity/adapter-capability-matrix_CN.md | SDK module 和 adapter contract tests 存在前不声明支持。 |
| adapters/deribit | adapter extension | External extension | docs/parity/adapter-capability-matrix_CN.md | SDK module 和 adapter contract tests 存在前不声明支持。 |
| adapters/kraken | adapter extension | External extension | docs/parity/adapter-capability-matrix_CN.md | SDK module 和 adapter contract tests 存在前不声明支持。 |

## Required Commands

| Purpose | Command |
| --- | --- |
| Scorecard metadata | `go test -count=1 ./testsuite -run 'Master|Score|Requirement'` |
| Existing contract suites | `go test -count=1 ./testsuite` |
| Non-SDK package compile and tests | `go test -count=1 $(go list ./... | grep -v '/sdk')` |
| SDK compile-only gate | `go test -run '^$' -count=1 ./sdk/...` |
| Runtime race gate | `go test -race -count=1 ./model ./cache ./account ./risk ./portfolio ./platform ./strategy ./backtest ./live ./testsuite` |
| Diff hygiene | `git diff --check` |
