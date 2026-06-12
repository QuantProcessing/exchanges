# Architecture And SDK Alignment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Realign the project around its intended three-layer design: native SDK coverage, stable adapter convenience abstractions, and TradingAccount lifecycle management.

**Architecture:** Keep SDK expansion behind a clarified abstraction boundary. First replace the base-symbol-only core with smaller instrument-aware capability interfaces, then tighten TradingAccount readiness contracts, then execute official SDK parity work from `docs/superpowers/plans/2026-06-10-official-api-sdk-alignment.md`.

**Tech Stack:** Go, existing exchange packages, `account/` runtime, adapter compliance suites, official API parity docs.

---

## Design Principles

The governing architecture principles are now recorded in `AGENTS.md` under `Project Architecture Contract`. Every task in this plan must comply with that contract.

Key decisions:

- SDK packages should align with official exchange APIs.
- Adapters should expose stable cross-exchange convenience, not every official endpoint.
- TradingAccount should manage lifecycle state and must not depend on funding/OI/analytics APIs.
- Funding/OI can remain adapter-accessible, but only as optional market-data capability interfaces.
- Base-symbol-only adapter APIs are legacy design. Breaking API changes are allowed; new core APIs should be quote-aware or instrument-aware.

## Phase 0: Freeze Current Behavior With Architecture Tests

**Purpose:** Before changing interfaces, make current behavior explicit so refactors are controlled.

**Files:**
- Modify: `capabilities_test.go`
- Modify: `public_contract_test.go`
- Modify: `testsuite/compliance.go`
- Create: `architecture_contract_test.go`

- [x] Add tests that capture lifecycle-critical behavior before refactoring adapters.
- [x] Add tests that verify unsupported optional surfaces return `ErrNotSupported`, not nil success.
- [x] Add tests that verify `TradingAccountReady` requires `WatchOrders`.
- [x] Add tests that expose the current single-quote limitation for Binance, OKX, Bybit, and Bitget.
- [ ] Run: `go test ./...` (attempted; blocked by sandbox DNS/listen restrictions in existing SDK/live-style tests)

**Exit criteria:** Existing behavior is covered before interface decomposition begins.

## Phase 1: Split Core Interfaces Into Optional Capability Interfaces

**Purpose:** Stop growing `Exchange`, `PerpExchange`, and `SpotExchange` as catch-all abstractions.

**Files:**
- Modify: `exchange.go`
- Modify: `capabilities.go`
- Modify: exchange `register.go` files as needed
- Modify: `docs/contributing/adding-exchange-adapters.md`

**Target shape:**

Replace the current catch-all interfaces with smaller interfaces. It is acceptable for this phase to be a breaking public API change.

```go
type MarketDataExchange interface {
	FetchTicker(...)
	FetchOrderBook(...)
	FetchTrades(...)
	FetchHistoricalTrades(...)
	FetchKlines(...)
	FetchSymbolDetails(...)
}

type OrderExecutionExchange interface {
	PlaceOrder(...)
	PlaceOrderWS(...)
	CancelOrder(...)
	CancelOrderWS(...)
	CancelAllOrders(...)
	FetchOrderByID(...)
	FetchOrders(...)
	FetchOpenOrders(...)
}

type AccountSnapshotExchange interface {
	FetchAccount(...)
	FetchBalance(...)
	FetchFeeRate(...)
}

type PerpRiskExchange interface {
	FetchPositions(...)
	SetLeverage(...)
	ModifyOrder(...)
	ModifyOrderWS(...)
}

type PerpMarketAnalytics interface {
	FetchFundingRate(...)
	FetchAllFundingRates(...)
	FetchFundingRateHistory(...)
	FetchOpenInterest(...)
}
```

- [x] Add the smaller interfaces and migrate compile-time assertions to them.
- [x] Update docs to say new adapter capabilities should prefer optional interfaces.
- [x] Update tests to type-assert existing adapters against the smaller interfaces.
- [x] Migrate repository call sites away from the old catch-all interface where practical.
- [ ] Run: `go test ./...` (blocked by sandbox DNS/listen restrictions in existing SDK/live-style tests)

**Exit criteria:** New optional interfaces exist and current adapters satisfy the expected subsets.

## Phase 2: Add Quote-Aware Instrument Routing

**Purpose:** Fix the single-quote adapter limitation by making market identity quote-aware.

**Files:**
- Modify: `models.go`
- Modify: `base_adapter.go`
- Modify: market-cache files in `binance`, `okx`, `bybit`, `bitget`, `hyperliquid`, `lighter`
- Modify: `README.md`
- Modify: `README_CN.md`

**Target model:**

```go
type MarketRef struct {
	Base        string
	Quote       string
	Settle      string
	Type        MarketType
	VenueSymbol string
}

func ParseMarketRef(symbol string, defaultQuote QuoteCurrency, marketType MarketType) MarketRef
```

Routing rules:

- Public adapter methods should accept `MarketRef` or another explicit instrument identity rather than plain base symbols.
- `"BTC/USDT"` and `"BTC/USDC"` may be accepted by parsing helpers, but plain `"BTC"` should not be the primary architecture contract.
- `VenueSymbol` is allowed for direct passthrough only when explicitly set.
- Existing `FormatSymbol(string)` can be removed or reduced to a parser helper once adapters migrate to `MarketRef`.

- [x] Add `MarketRef` and parser tests.
- [x] Add quote-aware market cache keys.
- [x] Implement Binance quote-aware spot/perp routing first.
- [x] Implement OKX, Bybit, and Bitget routing.
- [x] Keep Hyperliquid/Lighter USDC-only behavior explicit.
- [x] Add new tests showing `"BTC/USDT"` and `"BTC/USDC"` can coexist in one adapter where supported.
- [ ] Run: `go test ./binance ./okx ./bybit ./bitget ./hyperliquid ./lighter` (Binance, Bybit, Hyperliquid, and Lighter passed; OKX/Bitget full package tests are blocked by sandbox `httptest` listener restrictions, with focused quote-aware tests passing)

**Exit criteria:** Users no longer need multiple adapters just to trade multiple quote currencies on the same venue/product where the venue supports them, and new adapter code no longer relies on base-symbol-only market identity.

## Phase 3: Make Adapter Methods Instrument-Aware

**Purpose:** Make instrument-aware methods the adapter norm rather than an advanced escape hatch.

**Files:**
- Modify: `exchange.go`
- Modify: selected adapters after Phase 2
- Modify: `testsuite/compliance.go`

**Target optional interface:**

```go
type InstrumentExchange interface {
	FetchTickerFor(ctx context.Context, market MarketRef) (*Ticker, error)
	FetchOrderBookFor(ctx context.Context, market MarketRef, limit int) (*OrderBook, error)
	PlaceOrderFor(ctx context.Context, market MarketRef, params *OrderParams) (*Order, error)
	FetchOpenOrdersFor(ctx context.Context, market MarketRef) ([]Order, error)
}
```

- [x] Add interface and tests.
- [x] Implement for Binance, OKX, Bybit, and Bitget.
- [x] Remove or demote current string methods after repository call sites migrate.
- [x] Update TradingAccount order params to carry `MarketRef` or an equivalent explicit market identity.
- [ ] Run: `go test ./...` (blocked by sandbox DNS/listen restrictions in existing SDK/live-style tests; focused root/account/adapter/SDK parity tests passed)

**Exit criteria:** Quote-aware adapter usage is the standard repository path, including TradingAccount order placement.

## Phase 4: Refactor Adapter Internals Around Services

**Purpose:** Reduce adapter file growth before official SDK expansion increases surface area.

**Files:**
- Modify exchange packages incrementally
- Prefer creating files such as:
  - `market_catalog.go`
  - `market_data_service.go`
  - `order_service.go`
  - `account_service.go`
  - `stream_service.go`
  - `mappers.go`

- [x] Start with one representative exchange: Bitget.
- [x] Move instrument lookup into a `MarketCatalog`.
- [x] Move public market data methods into a service/helper file.
- [x] Move order execution and query methods into a service/helper file.
- [x] Keep public adapter method signatures unchanged.
- [x] Repeat the pattern for Binance and OKX after the first exchange is stable.
- [x] Run targeted tests after each exchange package.

**Exit criteria:** Adapter files become façades over focused internal services, not monolithic protocol/mapping/state containers.

## Phase 5: Tighten TradingAccount Readiness And Scope

**Purpose:** Make TradingAccount clearly about lifecycle runtime, not general account or market analytics.

**Files:**
- Modify: `account/base_trade_client.go`
- Modify: `account/perp_trading_account.go`
- Modify: `account/spot_trading_account.go`
- Modify: `account/stream_health.go`
- Modify: `testsuite/trading_account_suite.go`
- Modify: `docs/contributing/adding-exchange-adapters.md`

- [x] Define a minimal lifecycle dependency contract in code comments and tests.
- [x] Verify `WatchOrders` remains mandatory for TradingAccount-ready adapters.
- [x] Keep `WatchFills` optional but surface health state when unsupported.
- [x] Ensure TradingAccount does not depend on funding/OI/analytics interfaces.
- [x] Add tests for account startup behavior when optional streams are unsupported.
- [x] Add tests for quote-aware order placement once Phase 3 exists.
- [x] Run: `go test ./account ./testsuite`

**Exit criteria:** TradingAccount has an explicit readiness contract and only consumes lifecycle-critical adapter capabilities.

## Phase 6: Add Portfolio-Level Composition Above TradingAccount

**Purpose:** Support multi-adapter, multi-quote, and multi-exchange lifecycle management without overloading a single adapter.

**Files:**
- Create: `account/portfolio_account.go`
- Create: `account/portfolio_account_test.go`

**Target concept:**

```go
type PortfolioAccount struct {
	// owns multiple SpotTradingAccount / PerpTradingAccount instances
}
```

- [x] Define a small registry keyed by exchange, market type, and quote/instrument key.
- [x] Add read-only aggregate views for balances, positions, open orders, and health.
- [x] Do not add smart order routing in this phase.
- [x] Do not place orders through PortfolioAccount until routing policy is designed.
- [x] Run: `go test ./account`

**Exit criteria:** Users have a natural place to compose multiple TradingAccounts without pushing that responsibility into adapters.

## Phase 7: Execute Official SDK Parity Work

**Purpose:** Fill missing official API coverage after the architecture can absorb it cleanly.

**Files:**
- Continue from: `docs/superpowers/plans/2026-06-10-official-api-sdk-alignment.md`
- Create/modify parity docs under `docs/superpowers/gaps/`
- Modify exchange `sdk/` packages

- [x] Build or update the official API parity matrix framework.
- [x] Classify official spot/perp endpoints as `missing-sdk`, `implemented-sdk`, `implemented-raw`, `intentionally-unsupported`, `blocked-by-official-api`, or `deprecated-official`.
- [x] Implement SDK endpoints one exchange/product slice at a time.
- [x] Add typed tests for each SDK endpoint.
- [x] Keep venue-specific endpoints in SDK unless a Phase 1 optional interface explicitly covers them.
- [x] Run exchange SDK tests after each slice.

**Exit criteria:** In-scope official spot/perp API rows have no remaining `missing-sdk` status.

## Phase 8: Selective Adapter Exposure For New SDK Capabilities

**Purpose:** Expose only stable convenience capabilities after SDK parity exists.

**Files:**
- Modify: `exchange.go`
- Modify: `capabilities.go`
- Modify adapters and tests only for approved optional interfaces

Candidate optional interfaces:

- `PerpMarketAnalytics` for funding/OI.
- `BatchOrderExchange` for venues with compatible batch semantics.
- `AlgoOrderExchange` only if a stable cross-exchange abstraction is designed.
- `AccountBillsExchange` for normalized account ledger reads.
- `TransferExchange` only after account-type semantics are consistent enough.

- [x] For each candidate, write a short design note before adding an interface.
- [x] Add interface and tests. (No new adapter interface was added; design note rejects current candidates.)
- [x] Wire only exchanges with semantically compatible behavior. (No additional adapter wiring in this pass.)
- [x] Keep incompatible official endpoints SDK-only.
- [ ] Run: `go test ./...` (blocked by sandbox DNS/listen restrictions in existing SDK/live-style tests; focused verification passed)

**Exit criteria:** Adapter convenience grows by deliberate capability families, not by endpoint accretion.

## Phase 9: Documentation And Migration

**Purpose:** Make the new model understandable for users and future agents.

**Files:**
- Modify: `README.md`
- Modify: `README_CN.md`
- Modify: `docs/contributing/adding-exchange-adapters.md`
- Modify: `docs/superpowers/gaps/*.md`

- [x] Document the three-layer architecture.
- [x] Document the breaking change from base-symbol-only APIs to quote-aware/instrument-aware APIs.
- [x] Document when to use SDK vs adapter vs TradingAccount.
- [x] Document optional capability interfaces.
- [x] Document official API parity workflow.
- [ ] Run: `go test ./...` (blocked by sandbox DNS/listen restrictions in existing SDK/live-style tests; focused verification passed)

**Exit criteria:** Public docs match the architecture contract in `AGENTS.md`.

## Recommended Order

1. Phase 0: freeze behavior.
2. Phase 1: split capability interfaces.
3. Phase 2: quote-aware routing.
4. Phase 3: instrument-aware optional methods.
5. Phase 4: adapter internals.
6. Phase 5: TradingAccount contract.
7. Phase 6: PortfolioAccount composition.
8. Phase 7: SDK parity.
9. Phase 8: selective adapter exposure.
10. Phase 9: documentation and migration.

## Verification Gates

- Every phase must pass `go test ./...` unless the phase only changes docs.
- SDK endpoint additions require endpoint-level tests.
- Adapter capability claims require adapter-level tests.
- TradingAccount-ready claims require lifecycle suite coverage.
- Live write tests must remain opt-in behind `RUN_FULL=1` and exchange-specific flags.

## Relationship To SDK Parity Plan

`docs/superpowers/plans/2026-06-10-official-api-sdk-alignment.md` remains the detailed SDK endpoint parity plan. This plan supersedes it on execution order: do the architecture phases first, then execute SDK parity with the clarified boundaries.
