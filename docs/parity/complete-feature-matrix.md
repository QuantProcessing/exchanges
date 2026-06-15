# Complete Feature Matrix

This matrix maps platform surfaces to package owners, rollout status, and
acceptance evidence. It is a maintainer document; users should start with the
guides under `docs/`.

## Status Legend

| Status | Meaning |
| --- | --- |
| Contracted | A testsuite owner exists and the capability is part of the master gate. |
| Partial | Current code covers part of the surface and must be expanded by the listed owner. |
| Planned | No complete implementation exists yet, but the owner and acceptance path are assigned. |
| External extension | The architecture can support this later, but no repository adapter support is claimed. |

## Core Platform Surfaces

| Surface | Go owner | Status | Acceptance suite | Result expectation |
| --- | --- | --- | --- | --- |
| model | model | Partial | testsuite/model_tester.go / TC-M01..TC-M06 | Identifiers, instruments, orders, reports, events, money, and data types round trip with validation. |
| model/events | model | Partial | testsuite/model_tester.go / TC-M03 | Order, fill, position, account, and lifecycle events preserve command metadata. |
| model/instruments | model | Partial | testsuite/model_tester.go / TC-M02 | Spot, perpetual, future-safe derivatives, and extension instruments validate precise metadata. |
| model/orders | model | Partial | testsuite/model_tester.go / TC-M03, TC-M05 | Advanced order types, order lists, OTO/OCO/OUO/bracket semantics, and contingency metadata are modeled. |
| cache runtime state | cache | Planned | testsuite/cache_tester.go / TC-C01..TC-C07 | Orders, fills, positions, accounts, instruments, data snapshots, and deferred fills are indexed without global scans. |
| command envelope | model,bus,kernel | Planned | testsuite/command_tester.go / TC-CMD01..TC-CMD05 | Command IDs, correlation IDs, trader/strategy/client IDs, timestamps, params, and list/position IDs survive every path. |
| execution | execution,account | Partial | testsuite/exec_tester.go; testsuite/lifecycle_tester.go / TC-E*, TC-L* | Submit/modify/cancel/query/list/report flows, manager state, fills, positions, contingencies, and lifecycle transitions are explicit. |
| execution/reconciliation | execution,account,live | Planned | testsuite/reconciliation_tester.go / TC-REC01..TC-REC09 | Startup, periodic, reconnect, mass-status, missing-fill, external-order, and discrepancy repair paths are auditable. |
| live | live,platform | Partial | testsuite/live_node_tester.go / TC-LIVE01..TC-LIVE09 | Live node startup, reconnect, private stream readiness, reconciliation, health, and shutdown are deterministic. |
| portfolio | portfolio | Partial | testsuite/portfolio_tester.go / TC-P01..TC-P23 | Account updates, balances, positions, commissions, PnL, exposure, conversion, and analyzer hooks are event driven. |
| risk engine | risk | Partial | testsuite/risk_tester.go / TC-R01..TC-R23 | Pre-trade checks, async queues, exposure, margin, reduce-only, kill switch, throttles, and rejection events are enforced. |
| strategy runtime | strategy | Planned | testsuite/strategy_tester.go / TC-S01..TC-S09 | One strategy implementation runs in backtest and live with typed callbacks, timers, order factory, cache, and portfolio helpers. |
| data engine/catalog | data,venue,backtest,live | Planned | testsuite/data_tester.go / TC-D01..TC-D23 | Live subscriptions and catalog replay share normalized ticks, bars, books, requests, responses, and health metrics. |
| backtest | backtest | Partial | testsuite/backtest_tester.go / TC-B01..TC-B24 | Deterministic simulated venue, matching core, advanced orders, order lists, fees, slippage, latency, and byte-stable results are covered. |
| documentation/examples | docs,examples | Planned | testsuite/documentation_artifacts_test.go / TC-DOC01..TC-DOC03 | Newcomer docs, module docs, usage guides, capability docs, and examples are navigable and executable. |

## Adapter And Extension Surfaces

| Surface | Go owner | Status | Acceptance suite | Result expectation |
| --- | --- | --- | --- | --- |
| adapters/binance | adapter/binance,sdk/binance | Partial | adapter/binance/*_test.go; testsuite/contracts.go | Spot and perpetual capabilities are SDK-backed, contract-tested, and capability-honest. |
| adapters/bybit | adapter/bybit,sdk/bybit | Partial | adapter/bybit/bybit_test.go; testsuite/contracts.go | Current declared capabilities are SDK-backed and private stream/reconciliation claims remain gated. |
| adapters/hyperliquid | adapter/hyperliquid,sdk/hyperliquid | Partial | adapter/hyperliquid/hyperliquid_test.go; testsuite/contracts.go | Spot and perpetual capabilities are SDK-backed, contract-tested, and capability-honest. |
| adapters/okx | adapter/okx,sdk/okx | Partial | adapter/okx/okx_test.go; testsuite/contracts.go | Current declared capabilities are SDK-backed and optional execution reports are explicit. |
| adapter/aster | adapter/aster,sdk/aster | Partial | adapter/aster/aster_test.go; testsuite/contracts.go | Spot and perpetual capabilities are SDK-backed, contract-tested, and capability-honest. |
| adapter/bitget | adapter/bitget,sdk/bitget | Partial | adapter/bitget/bitget_test.go; testsuite/contracts.go | Current declared capabilities are SDK-backed and optional reports remain explicit. |
| adapter/backpack | adapter/backpack,sdk/backpack | Partial | adapter/backpack/backpack_test.go; testsuite/contracts.go | Current declared capabilities are SDK-backed and optional reports remain explicit. |
| adapter/edgex | adapter/edgex,sdk/edgex | Partial | adapter/edgex/edgex_test.go; testsuite/contracts.go | Current declared capabilities are SDK-backed and optional reports remain explicit. |
| adapter/grvt | adapter/grvt,sdk/grvt | Partial | adapter/grvt/grvt_test.go; testsuite/contracts.go | Current declared capabilities are SDK-backed and optional reports remain explicit. |
| adapter/lighter | adapter/lighter,sdk/lighter | Partial | adapter/lighter/lighter_test.go; testsuite/contracts.go | Current declared capabilities are SDK-backed and optional reports remain explicit. |
| adapter/nado | adapter/nado,sdk/nado | Partial | adapter/nado/nado_test.go; testsuite/contracts.go | Current declared capabilities are SDK-backed and optional reports remain explicit. |
| adapter/standx | adapter/standx,sdk/standx | Partial | adapter/standx/standx_test.go; testsuite/contracts.go | Current declared capabilities are SDK-backed and optional reports remain explicit. |
| adapters/interactive_brokers | adapter extension | External extension | docs/parity/adapter-capability-matrix.md | No support is claimed until an SDK or gateway module and adapter contract tests exist. |
| adapters/databento | data-provider extension | External extension | docs/parity/adapter-capability-matrix.md | Data-provider support requires catalog/provider mapping and data engine contract tests. |
| adapters/betfair | adapter extension | External extension | docs/parity/adapter-capability-matrix.md | No support is claimed until an SDK module and adapter contract tests exist. |
| adapters/deribit | adapter extension | External extension | docs/parity/adapter-capability-matrix.md | No support is claimed until an SDK module and adapter contract tests exist. |
| adapters/kraken | adapter extension | External extension | docs/parity/adapter-capability-matrix.md | No support is claimed until an SDK module and adapter contract tests exist. |

## Required Commands

| Purpose | Command |
| --- | --- |
| Scorecard metadata | `go test -count=1 ./testsuite -run 'Master|Score|Requirement'` |
| Existing contract suites | `go test -count=1 ./testsuite` |
| Non-SDK package compile and tests | `go test -count=1 $(go list ./... | grep -v '/sdk')` |
| SDK compile-only gate | `go test -run '^$' -count=1 ./sdk/...` |
| Runtime race gate | `go test -race -count=1 ./model ./cache ./account ./risk ./portfolio ./platform ./strategy ./backtest ./live ./testsuite` |
| Diff hygiene | `git diff --check` |
