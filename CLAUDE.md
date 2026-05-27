# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository

Go SDK (`github.com/QuantProcessing/exchanges`, Go 1.26) that gives one unified `Exchange` interface across many crypto venues: Binance, OKX, Aster, Nado, Lighter, Hyperliquid, Bitget, Bybit, StandX, GRVT, EdgeX, Decibel, Backpack. Designed as a Go-native CCXT alternative for a quantitative trading stack.

## Architecture

Two strict layers, with a non-negotiable boundary:

- **Adapter layer** (`<exchange>/perp_adapter.go`, `<exchange>/spot_adapter.go`) — implements `Exchange` / `PerpExchange` / `SpotExchange`. Owns symbol mapping, validation, slippage policy, `ErrNotSupported` decisions, and mapping SDK structs to unified `exchanges.*` models.
- **SDK layer** (`<exchange>/sdk/...`) — exchange-native REST + WebSocket. Owns signing, wire structs, connection lifecycle. Never surface exchange-native structs from adapter methods. See `docs/contributing/adding-exchange-adapters.md` for the full rule set.

Cross-cutting files in the root package:

- `exchange.go` — core `Exchange`, `PerpExchange`, `SpotExchange`, `Streamable` interfaces. Method naming is load-bearing: `Fetch*` = REST, `Watch*` = WebSocket subscription, `*WS` suffix = explicit WebSocket write (returns only transport/ACK errors; requires `OrderParams.ClientID` so later updates correlate).
- `models.go` — unified types (`Order`, `Fill`, `Position`, `Ticker`, `OrderBook`, `OrderParams`, …).
- `errors.go` — sentinel errors + structured `ExchangeError`. Rate-limit, balance, precision, etc. always wrap these so `errors.Is`/`errors.As` works across adapters.
- `base_adapter.go`, `local_orderbook.go` — shared orderbook cache + sync helpers used by most adapters.
- `registry.go` + `manager.go` — `AdapterConstructor` registry. Each `<exchange>/register.go` calls `Register("NAME", ctor)` from `init()`. `config/` consumes this for YAML/JSON-driven bootstrap; blank-import `config/all` to pull in every constructor.

Separate-but-related runtime:

- `account/` — `TradingAccount` + `OrderFlow`. `TradingAccount.Place` returns an `OrderFlow` that fuses `WatchOrders` (lifecycle) and `WatchFills` (execution detail) into per-order merged snapshots (`flow.C()`) plus a raw fills stream (`flow.Fills()`). Adapters that can't expose native fills must return `ErrNotSupported` — never synthesize fills from another stream.

`FetchOrderByID`, `FetchOrders`, `FetchOpenOrders` are three distinct contracts. Never implement the first by scanning the third.

## Build tags

- `grvt/` uses build tag `grvt`.
- `edgex/` uses build tag `edgex`.

Default `go build ./...` excludes these; pass `-tags grvt,edgex` (or run the verify scripts, which handle this) to include them.

## Testing

Plain `go test ./...` is NOT the canonical gate — several suites need live credentials or longer WebSocket sessions. Use the layered model:

```bash
# Quick default gate (unit + short adapter tests)
go test -short ./...

# Focused short verification for one exchange
scripts/verify_exchange.sh <backpack|aster|binance|bitget|edgex|grvt|hyperliquid|lighter|nado|okx|standx>

# Full regression — needs .env with live creds; script sets RUN_FULL=1
GOCACHE=/tmp/exchanges-gocache bash scripts/verify_full.sh

# Long-lived stream soak (3-minute per package)
GOCACHE=/tmp/exchanges-gocache RUN_SOAK=1 bash scripts/verify_soak.sh

# Single test
go test -run TestName ./binance/...
```

`scripts/verify_full.sh` loads `.env` from repo root (copy `.env.example` first), preserves already-exported vars, and remaps legacy aliases (`EDGEX_PRIVATE_KEY→EDGEX_STARK_PRIVATE_KEY`, `NADO_SUB_ACCOUNT_NAME→NADO_SUBACCOUNT_NAME`, `OKX_SECRET_KEY→OKX_API_SECRET`, `OKX_PASSPHRASE→OKX_API_PASSPHRASE`). `RUN_FULL` / `RUN_SOAK` are managed by those scripts; do not export them for the default gate.

Env loading inside tests uses `internal/testenv`. Test symbols are configured per-market via `<EXCHANGE>_PERP_TEST_SYMBOL` / `<EXCHANGE>_SPOT_TEST_SYMBOL` (only where that market is supported). Bybit trade-WS order tests are gated behind `BYBIT_ENABLE_WS_ORDER_TESTS=1`.

## Shared compliance suites

New adapters wire shared suites from `testsuite/` in `<exchange>/adapter_test.go` based on claimed capability:

| Claim | Required suites |
|-------|-----------------|
| `public-data-only` | `RunAdapterComplianceTests` |
| `trading-capable` | + `RunOrderSuite`, `RunOrderQuerySemanticsSuite` |
| `lifecycle-capable` | + `RunLifecycleSuite` (requires real `WatchOrders`) |
| `trading-account-capable` | + `RunTradingAccountSuite` (requires `FetchAccount` + real `WatchOrders`) |
| `analytics-capable` | + `RunAnalyticsComplianceTests` (requires `FetchOpenInterest` + `FetchFundingRateHistory` on PerpExchange) |

Surfaces that are genuinely unsupported must return `exchanges.ErrNotSupported` — never a silent no-op. `FetchOrderByID` must not be implemented by scanning open orders only.

## Adding / modifying adapters

`AGENTS.md` designates `docs/contributing/adding-exchange-adapters.md` as repository policy. Read it before any task that adds a venue, changes adapter capability claims, or touches private-stream/TradingAccount wiring. Key invariants it enforces:

- Pick the nearest peer by market coverage first, auth model second; borrow per concern rather than cloning one package.
- Adapter files must not contain signing, raw REST path building, wire-format structs, or WebSocket connection lifecycle — those belong in `sdk/`.
- `WatchOrders` is mandatory for any lifecycle or TradingAccount claim; `WatchPositions` is additive, not the gate.
- Live-test wiring (`.env.example`, `internal/testenv` gating, shared suite matrix) must exist before calling a capability-level change done.

## Conventions worth preserving

- Symbol convention: all public methods take a base symbol (`"BTC"`). `FormatSymbol` / `ExtractSymbol` handle the venue-specific form based on the adapter's configured `QuoteCurrency`.
- `QuoteCurrency` is per-adapter; passing an unsupported one (e.g. USDT into Hyperliquid) must error at construction time, not silently work.
- The library deliberately does NOT implement retry/backoff. Callers own rate-limit strategy; the SDK just surfaces `ErrRateLimited` wrapped in `ExchangeError` so `errors.Is` works.
- Backpack `OrderParams.ClientID` must be a numeric `uint32`-range value; prefer `backpack.GenerateClientID()` or leave empty to let the adapter generate one.
