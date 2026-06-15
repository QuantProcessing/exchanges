# NautilusTrader Complete Feature Matrix

This matrix is the owner map for the complete Go NautilusTrader replica program. It maps visible reference surfaces from `.omx/references/nautilus_trader/nautilus_trader` to Go packages, acceptance suites, and rollout status.

## Status Legend

| Status | Meaning |
| --- | --- |
| Contracted | A testsuite owner exists and the capability is part of the master gate. |
| Partial | Current code covers part of the surface and must be expanded by the listed epic. |
| Planned | No complete Go implementation exists yet, but the owner and acceptance test are assigned. |
| External extension | The architecture must support the surface, but this repository cannot claim venue support until an SDK module exists. |

## Core Platform Surfaces

| Nautilus reference surface | Go owner | Status | Acceptance suite | Epic | Result expectation |
| --- | --- | --- | --- | ---: | --- |
| model | model | Partial | testsuite/model_tester.go / TC-M01..TC-M06 | 1 | Identifiers, instruments, orders, reports, events, money, and data types round trip with validation. |
| model/events | model | Partial | testsuite/model_tester.go / TC-M03 | 1 | Order, fill, position, account, and lifecycle events preserve command metadata. |
| model/instruments | model | Partial | testsuite/model_tester.go / TC-M02 | 1 | Spot, perp, future-safe derivatives, and extension instruments have precise validation and decimal metadata. |
| model/orders | model | Partial | testsuite/model_tester.go / TC-M03, TC-M05 | 1 | Advanced order types, order lists, OTO/OCO/OUO/bracket semantics, and contingency metadata are modeled. |
| model/tick_scheme | model | Planned | testsuite/model_tester.go / TC-M06 | 1 | Price/size precision and tick validation are enforced before risk/execution. |
| cache-equivalent runtime state | cache | Planned | testsuite/cache_tester.go / TC-C01..TC-C07 | 2 | Orders, fills, positions, accounts, instruments, data snapshots, and deferred fills are indexed without global scans. |
| command envelope | model,bus,kernel | Planned | testsuite/command_tester.go / TC-CMD01..TC-CMD05 | 3 | Command IDs, correlation IDs, trader/strategy/client IDs, timestamps, params, and list/position IDs survive every path. |
| execution | execution,account | Partial | testsuite/exec_tester.go; testsuite/lifecycle_tester.go / TC-E*, TC-L* | 6 | Submit/modify/cancel/query/list/report flows, order manager, fills, positions, contingencies, and lifecycle transitions match reference behavior. |
| execution/reconciliation | execution,account,live | Planned | testsuite/reconciliation_tester.go / TC-REC01..TC-REC09 | 7 | Startup, periodic, reconnect, mass-status, missing-fill, external-order, and discrepancy repair paths are explicit and auditable. |
| live | live,platform | Partial | testsuite/live_node_tester.go / TC-LIVE01..TC-LIVE05 | 11 | Live node startup, reconnect, private stream readiness, reconciliation, health, and shutdown are deterministic. |
| portfolio | portfolio | Partial | testsuite/portfolio_tester.go / TC-P01..TC-P23 | 9 | Account updates, balances, positions, commissions, realized/unrealized PnL, exposure, conversion, and analyzer hooks are event driven. |
| risk engine | risk | Partial | testsuite/risk_tester.go / TC-R01..TC-R23 | 8 | Pre-trade checks, async queues, exposure, margin, reduce-only, kill switch, throttles, and risk rejection events are enforced. |
| strategy runtime | strategy | Planned | testsuite/strategy_tester.go / TC-S01..TC-S05 | 4 | One strategy implementation runs unchanged in backtest and live with typed callbacks, timers, order factory, cache, and portfolio helpers. |
| data engine/catalog | data,venue,backtest,live | Planned | testsuite/data_tester.go / TC-D01..TC-D23 | 5 | Live subscriptions and catalog-backed replay share normalized ticks, bars, books, requests, responses, and health metrics. |
| backtest | backtest | Partial | testsuite/backtest_tester.go / TC-B01..TC-B24 | 10 | Deterministic simulated venue, matching core, advanced orders, order lists, fees, slippage, latency, and byte-identical results are covered. |
| documentation/examples | docs,examples | Planned | testsuite/nautilus_master_tester.go / TC-DOC01..TC-DOC03 | 13 | Scorecard, strategy authoring, reconciliation, adapter capabilities, backtesting, live trading, and side-by-side examples are runnable. |

## Reference Adapter Surfaces

| Nautilus reference surface | Go owner | Status | Acceptance suite | Epic | Result expectation |
| --- | --- | --- | --- | ---: | --- |
| adapters/binance | adapter/binance,sdk/binance | Partial | adapter/binance/*_test.go; testsuite/contracts.go | 12 | Spot and perp capabilities are SDK-backed, contract-tested, and capability-honest. |
| adapters/bybit | adapter/bybit,sdk/bybit | Partial | adapter/bybit/bybit_test.go; testsuite/contracts.go | 12 | Current declared capabilities are SDK-backed and private-stream/reconciliation claims remain gated. |
| adapters/hyperliquid | adapter/hyperliquid,sdk/hyperliquid | Partial | adapter/hyperliquid/hyperliquid_test.go; testsuite/contracts.go | 12 | Spot and perp capabilities are SDK-backed, contract-tested, and capability-honest. |
| adapters/okx | adapter/okx,sdk/okx | Partial | adapter/okx/okx_test.go; testsuite/contracts.go | 12 | Current declared capabilities are SDK-backed and optional execution reports are explicit. |
| adapters/betfair | adapter extension | External extension | docs/parity/adapter-capability-matrix.md | 12 | No support is claimed until an SDK module and adapter contract tests exist. |
| adapters/bitmex | adapter extension | External extension | docs/parity/adapter-capability-matrix.md | 12 | No support is claimed until an SDK module and adapter contract tests exist. |
| adapters/databento | adapter extension | External extension | docs/parity/adapter-capability-matrix.md | 12 | Data-provider shape must be supported architecturally before support can be claimed. |
| adapters/deribit | adapter extension | External extension | docs/parity/adapter-capability-matrix.md | 12 | No support is claimed until an SDK module and adapter contract tests exist. |
| adapters/dydx | adapter extension | External extension | docs/parity/adapter-capability-matrix.md | 12 | No support is claimed until an SDK module and adapter contract tests exist. |
| adapters/interactive_brokers | adapter extension | External extension | docs/parity/adapter-capability-matrix.md | 12 | No support is claimed until an SDK module and adapter contract tests exist. |
| adapters/interactive_brokers_pyo3 | adapter extension | External extension | docs/parity/adapter-capability-matrix.md | 12 | The Go platform may expose an equivalent adapter later, but no Python bridge support is claimed here. |
| adapters/kraken | adapter extension | External extension | docs/parity/adapter-capability-matrix.md | 12 | No support is claimed until an SDK module and adapter contract tests exist. |
| adapters/polymarket | adapter extension | External extension | docs/parity/adapter-capability-matrix.md | 12 | No support is claimed until an SDK module and adapter contract tests exist. |
| adapters/sandbox | adapter extension | Planned | testsuite/backtest_tester.go; testsuite/live_node_tester.go | 10 | Sandbox semantics are covered by backtest/live fake clients before any venue claim. |
| adapters/tardis | adapter extension | External extension | docs/parity/adapter-capability-matrix.md | 12 | Data-provider shape must be supported architecturally before support can be claimed. |

## Repository-Only Adapter Surfaces

| Repository surface | Go owner | Status | Acceptance suite | Epic | Result expectation |
| --- | --- | --- | --- | ---: | --- |
| adapter/aster | adapter/aster,sdk/aster | Partial | adapter/aster/aster_test.go; testsuite/contracts.go | 12 | Spot and perp capabilities are SDK-backed, contract-tested, and capability-honest. |
| adapter/bitget | adapter/bitget,sdk/bitget | Partial | adapter/bitget/bitget_test.go; testsuite/contracts.go | 12 | Current declared capabilities are SDK-backed and optional reports remain explicit. |
| adapter/backpack | adapter/backpack,sdk/backpack | Partial | adapter/backpack/backpack_test.go; testsuite/contracts.go | 12 | Current declared capabilities are SDK-backed and optional reports remain explicit. |
| adapter/edgex | adapter/edgex,sdk/edgex | Partial | adapter/edgex/edgex_test.go; testsuite/contracts.go | 12 | Current declared capabilities are SDK-backed and optional reports remain explicit. |
| adapter/grvt | adapter/grvt,sdk/grvt | Partial | adapter/grvt/grvt_test.go; testsuite/contracts.go | 12 | Current declared capabilities are SDK-backed and optional reports remain explicit. |
| adapter/lighter | adapter/lighter,sdk/lighter | Partial | adapter/lighter/lighter_test.go; testsuite/contracts.go | 12 | Current declared capabilities are SDK-backed and optional reports remain explicit. |
| adapter/nado | adapter/nado,sdk/nado | Partial | adapter/nado/nado_test.go; testsuite/contracts.go | 12 | Current declared capabilities are SDK-backed and optional reports remain explicit. |
| adapter/standx | adapter/standx,sdk/standx | Partial | adapter/standx/standx_test.go; testsuite/contracts.go | 12 | Current declared capabilities are SDK-backed and optional reports remain explicit. |

## Required Commands

| Purpose | Command |
| --- | --- |
| Master gate metadata | `go test -count=1 ./testsuite -run 'TestNautilusMaster'` |
| Existing contract suites | `go test -count=1 ./testsuite` |
| Non-SDK package compile and tests | `go test -count=1 $(go list ./... | grep -v '/sdk')` |
| SDK compile-only gate | `go test -run '^$' -count=1 ./sdk/...` |
| Race gate for runtime packages | `go test -race -count=1 ./model ./cache ./account ./risk ./portfolio ./platform ./strategy ./backtest ./live ./testsuite` |
| Diff hygiene | `git diff --check` |
