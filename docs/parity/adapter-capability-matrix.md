# Adapter Capability Matrix

This matrix is the source of truth for adapter capability honesty. It reflects
current `venue.DeclaredCapabilities` claims and the test evidence required
before an adapter can be treated as supporting a workflow.

## Capability Key

| Mark | Meaning |
| --- | --- |
| Yes | Claimed by the adapter today and must be backed by contract tests. |
| No | Not claimed today; callers should receive explicit unsupported behavior where applicable. |
| Planned | Implementation target, not current support. |
| External | Outside the current repository SDK universe. |

## Current Repository Adapters

| Adapter | SDK package | Adapter package | Instruments | Data snapshots | Data streams | Funding snapshots | Funding stream | Account snapshot | Submit | Cancel | Modify | Query | Order reports | Fill reports | Position reports | Private stream | Resubscribe | Mass status | Order lists | Contract evidence |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Binance Spot | sdk/binance | adapter/binance | Yes | Yes | Yes | No | No | Yes | Yes | Yes | No | No | Yes | No | No | Yes | Yes | Planned | Planned | adapter/binance/spot_test.go; testsuite/contracts.go |
| Binance Perp | sdk/binance | adapter/binance | Yes | Yes | Yes | No | No | Yes | Yes | Yes | No | No | Yes | No | No | Yes | Yes | Planned | Planned | adapter/binance/perp_test.go; testsuite/contracts.go |
| Aster Spot | sdk/aster | adapter/aster | Yes | Yes | Yes | No | No | Yes | Yes | Yes | No | No | Yes | No | No | Yes | Yes | Planned | Planned | adapter/aster/aster_test.go; testsuite/contracts.go |
| Aster Perp | sdk/aster | adapter/aster | Yes | Yes | Yes | No | No | Yes | Yes | Yes | No | No | Yes | No | No | Yes | Yes | Planned | Planned | adapter/aster/aster_test.go; testsuite/contracts.go |
| OKX | sdk/okx | adapter/okx | Yes | Yes | Yes | No | No | Yes | Yes | Yes | No | No | Yes | No | No | Yes | Yes | Planned | Planned | adapter/okx/okx_test.go; testsuite/contracts.go |
| Bybit | sdk/bybit | adapter/bybit | Yes | Yes | Yes | No | No | Yes | Yes | Yes | No | No | Yes | No | No | Yes | Yes | Planned | Planned | adapter/bybit/bybit_test.go; testsuite/contracts.go |
| Bitget | sdk/bitget | adapter/bitget | Yes | Yes | Yes | No | No | Yes | Yes | Yes | No | No | Yes | No | No | Yes | Yes | Planned | Planned | adapter/bitget/bitget_test.go; testsuite/contracts.go |
| Hyperliquid Spot | sdk/hyperliquid | adapter/hyperliquid | Yes | Yes | Yes | No | No | Yes | Yes | Yes | No | No | Yes | No | No | Yes | Yes | Planned | Planned | adapter/hyperliquid/hyperliquid_test.go; testsuite/contracts.go |
| Hyperliquid Perp | sdk/hyperliquid | adapter/hyperliquid | Yes | Yes | Yes | No | No | Yes | Yes | Yes | No | No | Yes | No | No | Yes | Yes | Planned | Planned | adapter/hyperliquid/hyperliquid_test.go; testsuite/contracts.go |
| Lighter | sdk/lighter | adapter/lighter | Yes | Yes | Yes | No | No | Yes | Yes | Yes | No | No | Yes | No | No | Yes | Yes | Planned | Planned | adapter/lighter/lighter_test.go; testsuite/contracts.go |
| Nado | sdk/nado | adapter/nado | Yes | Yes | Yes | No | No | Yes | Yes | Yes | No | No | Yes | No | No | Yes | Yes | Planned | Planned | adapter/nado/nado_test.go; testsuite/contracts.go |
| EdgeX | sdk/edgex | adapter/edgex | Yes | Yes | Yes | No | No | Yes | Yes | Yes | No | No | Yes | No | No | Yes | Yes | Planned | Planned | adapter/edgex/edgex_test.go; testsuite/contracts.go |
| GRVT | sdk/grvt | adapter/grvt | Yes | Yes | Yes | No | No | Yes | Yes | Yes | No | No | Yes | No | No | Yes | Yes | Planned | Planned | adapter/grvt/grvt_test.go; testsuite/contracts.go |
| StandX | sdk/standx | adapter/standx | Yes | Yes | Yes | No | No | Yes | Yes | Yes | No | No | Yes | No | No | Yes | Yes | Planned | Planned | adapter/standx/standx_test.go; testsuite/contracts.go |
| Backpack | sdk/backpack | adapter/backpack | Yes | Yes | Yes | No | No | Yes | Yes | Yes | No | No | Yes | No | No | Yes | Yes | Planned | Planned | adapter/backpack/backpack_test.go; testsuite/contracts.go |

## Extension Targets Outside Current SDK Scope

These rows are intentionally not counted as supported repository adapters. They
remain extension targets until SDK modules, adapter packages, and contract
tests exist.

| Extension target | Current repository owner | Status | Required before support can be claimed |
| --- | --- | --- | --- |
| Betfair | adapter extension | External | SDK module, instrument model extension, data client, execution client, account reports, adapter contract tests. |
| BitMEX | adapter extension | External | SDK module, market data client, execution client, private stream, report generators, adapter contract tests. |
| Databento | data-provider extension | External | Data catalog/provider abstraction, schema mapping, replay tests, data engine contract tests. |
| Deribit | adapter extension | External | SDK module, options/futures instrument coverage, execution reports, adapter contract tests. |
| dYdX | adapter extension | External | SDK module, market data client, execution client, private stream, adapter contract tests. |
| Interactive Brokers | adapter extension | External | SDK or gateway layer, multi-asset instrument coverage, account/position reports, adapter contract tests. |
| Kraken | adapter extension | External | SDK module, spot/futures market data, execution client, private stream, adapter contract tests. |
| Polymarket | adapter extension | External | SDK module, market/instrument model extension, data client, execution client, adapter contract tests. |
| Sandbox | backtest/live fake clients | Planned | Deterministic simulated venue, fake execution reports, private-stream test harness, live node tests. |
| Tardis | data-provider extension | External | Data provider module, historical catalog mapping, replay tests, data engine contract tests. |

## Required Verification Commands

| Purpose | Command |
| --- | --- |
| Capability matrix and scorecard metadata | `go test -count=1 ./testsuite -run 'Master|Score|Requirement'` |
| Adapter contract tests | `go test -count=1 ./adapter/... ./config/all ./testsuite -run 'Adapter|Capability|Contract'` |
| SDK compile-only check | `go test -run '^$' -count=1 ./sdk/...` |
| Public SDK read tests | `go test -count=1 ./sdk/...` |
| Live write tests | Run only with the venue-specific write flag and credentials documented by that SDK package. |

## Capability Policy

- A `Yes` capability must have a passing shared contract case before it can
  count toward release evidence.
- A `No` capability must not be treated as a lifecycle requirement by callers.
- A `Planned` capability is an explicit implementation target, not current
  support.
- Private stream and resubscribe are separate claims; full lifecycle readiness
  also requires reconciliation evidence.
- Fill, position, mass-status, and order-list support must be claimed only
  after SDK-backed implementation and shared tests exist.
