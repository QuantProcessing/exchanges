# Adding Exchange Adapters

This document is the repository-owned guide for adding a new SDK package,
adding a new adapter package, or expanding adapter capability in this codebase.

Use it for:

- new perp or spot adapters
- support-level upgrades such as public-only to trading-capable
- private stream and TradingAccount readiness decisions
- live `testsuite` wiring and completion gates

This guide replaces the older repo-local `adding-exchange-adapters` skill as the maintained source of truth.

## Before You Write Code

1. Read `exchange.go`, `models.go`, `errors.go`, `utils.go`, `registry.go`, and the relevant `testsuite/*` files.
2. Choose the nearest existing peer by market coverage first and auth model second.
3. Borrow structure per concern instead of copying one package wholesale. One peer may be right for `options.go` and `register.go`, while another is better for orderbook sync or private streams.
4. Classify the target before choosing file layout:
   - `public-data-only`
   - `trading-capable`
   - `lifecycle-capable`
   - `trading-account-capable`

If peer selection or capability classification is missing, stop and do that first.

## Public Entry Layers

This repository exposes three user-facing layers. Keep the import paths aligned
with the layer the user is choosing:

- `github.com/QuantProcessing/exchanges` is the normalized root contract.
- `github.com/QuantProcessing/exchanges/sdk/<exchange>` is the venue-native SDK
  entry point.
- `github.com/QuantProcessing/exchanges/adapter/<exchange>` is the normalized
  adapter entry point.
- `github.com/QuantProcessing/exchanges/account` is the TradingAccount runtime.

Do not place new exchange implementation code at the repository root.

## Capability Matrix

| Capability | Shared suites to wire in `adapter_test.go` | Minimum support claim |
|------------|--------------------------------------------|-----------------------|
| `public-data-only` | `RunAdapterComplianceTests` | Private/account/trading surfaces return `exchanges.ErrNotSupported` |
| `trading-capable` | `RunAdapterComplianceTests`, `RunOrderSuite`, `RunOrderQuerySemanticsSuite` | Real trading and order-query behavior; unsupported shared surfaces return `exchanges.ErrNotSupported` |
| `lifecycle-capable` | `RunAdapterComplianceTests`, `RunOrderSuite`, `RunOrderQuerySemanticsSuite`, `RunLifecycleSuite` | Real `WatchOrders`; lifecycle claims are not valid without it |
| `trading-account-capable` | `RunAdapterComplianceTests`, `RunOrderSuite`, `RunOrderQuerySemanticsSuite`, `RunTradingAccountSuite`, and `RunLifecycleSuite` when lifecycle support is claimed | `FetchAccount` plus a real `WatchOrders`; `WatchPositions` is additive, not the gate |

`FetchOrderByID`, `FetchOrders`, and `FetchOpenOrders` are separate contracts. Preserve that distinction in both implementation and tests.

## Adapter Interface Boundaries

Adapters expose stable cross-exchange convenience, not every official exchange
API. Implement venue-specific official endpoints in `sdk/` first, then expose
them through adapters only when they fit a stable shared abstraction.

Prefer small optional interfaces over expanding `Exchange`, `PerpExchange`, or
`SpotExchange`. Current capability families include:

- `MarketDataExchange`
- `OrderExecutionExchange`
- `AccountSnapshotExchange`
- `LocalOrderBookExchange`
- `PerpRiskExchange`
- `PerpMarketAnalytics`
- `SpotBalanceExchange`
- `AssetTransferExchange`

Funding, open interest, mark/index analytics, account bills, batch orders,
trigger/algo orders, and venue-specific risk controls must remain optional
capability surfaces unless a separate design establishes them as core
cross-exchange primitives.

## Architecture And SDK Boundaries

Create or extend `sdk/<exchange>` when any of these are true:

- the exchange needs signing or auth token management
- low-level REST APIs split across multiple surface groups
- WebSocket handling needs multiple connections or non-trivial session logic
- exchange-native wire structs would otherwise leak into adapter files
- spot and perp adapters will share reusable low-level clients or mappers

Choose the `sdk/` shape from the nearest peer:

- flat `sdk/<exchange>` when one compact client shape covers the exchange
- `sdk/<exchange>/perp` plus `sdk/<exchange>/spot` when low-level APIs diverge materially by market
- shared helpers only when there is real reuse

Adapter-layer responsibilities:

- symbol mapping and validation
- picking the low-level method that satisfies the shared interface
- mapping validated SDK responses into unified `exchanges.*` models
- adapter-side policy such as slippage handling and `ErrNotSupported`

Do not keep these in adapter files:

- raw REST path building
- signing logic
- wire-format request or response structs
- WebSocket connection lifecycle internals

Rule of thumb: `sdk/<exchange>` speaks exchange-native protocol,
`adapter/<exchange>` speaks the shared repository contract.

## Official API Parity

Before adding SDK endpoints or claiming official spot/perp coverage, update the
matching file under `docs/superpowers/gaps/official-api-parity-*.md`.

Rules:

- every official spot/perp REST endpoint, WebSocket channel, or WebSocket API operation needs a row
- use `implemented-sdk` for typed SDK methods
- use `implemented-raw` only when a deliberate raw SDK fallback exists
- use `implemented-adapter` only when the endpoint is also exposed through a stable adapter capability
- do not leave `missing-sdk` or `missing-adapter` rows when declaring a parity slice complete
- do not add adapter interfaces for venue-specific endpoints without a design note like `docs/superpowers/gaps/adapter-exposure-policy.md`

Review the updated parity matrix during code review after matrix changes.

## SDK Test Layout

Keep SDK tests direct and Go-idiomatic.

Rules:

- every SDK implementation file gets a corresponding `_test.go` file
- every public SDK API method gets a directly named test method
- method test names should follow `TestClient_MethodName` or
  `TestWSClient_MethodName`
- tests should sit beside the code they cover
- read-method tests should call the real official exchange endpoint by default
- public read tests should not require a feature flag
- private read tests should skip only when required credentials are missing
- write-method tests must require an exchange-specific enable flag such as
  `BINANCE_ENABLE_LIVE_WRITE_TESTS=1` plus required credentials before they
  execute against the real exchange
- avoid fake transports, fake WebSocket connections, and local
  `httptest.Server` listeners for SDK API-method tests unless the code under
  test is a pure parser, pure signing helper, or local dispatcher rather than
  an exchange API call

Example:

```text
sdk/binance/spot/order.go
sdk/binance/spot/order_test.go
```

```go
func TestClient_PlaceOrder(t *testing.T) {}
func TestClient_CancelOrder(t *testing.T) {}
func TestClient_GetOrder(t *testing.T) {}
func TestClient_GetOpenOrders(t *testing.T) {}
```

Each read API method test should cover the useful minimum:

- successful response parsing
- enough request inputs to prove the SDK method is wired to the intended
  official API behavior

Each write API method test should cover the useful minimum:

- clear skip message when the enable flag or credentials are missing
- tiny, explicitly parameterized request values loaded from environment
- response/error parsing from the real official endpoint

## Order Contract Rules

The current shared order-query surfaces are:

- `FetchOrderByID`
- `FetchOrders`
- `FetchOpenOrders`

Required behavior:

- use base symbols at the adapter boundary
- filter by symbol when the low-level API returns broader results
- return unified `exchanges.Order` values
- return `exchanges.ErrOrderNotFound` when lookup is exhausted

Hard rules:

- do not implement `FetchOrderByID` by scanning only open orders and pretend that is complete behavior
- do not collapse `FetchOrders` into `FetchOpenOrders` unless the exchange truly exposes only open orders and that limitation is explicit in code and tests
- do not invent adapter-only history APIs beyond the shared interface
- do not return `nil, nil` for missing orders

Preferred lookup order for `FetchOrderByID`:

1. dedicated order-detail endpoint
2. authenticated order history or broader order-list endpoint that includes terminal states
3. symbol-scoped history/list endpoint that can still resolve the order

## Position.InstrumentType Is Mandatory

Every `exchanges.Position` value an adapter returns — whether from
`FetchPositions`, `WatchPositions`, or as part of `FetchAccount` — MUST set
`InstrumentType`. Use `exchanges.InstrumentTypePerp` for perp adapters,
`InstrumentTypeSpot` for synthesized spot "positions", and
`InstrumentTypeOption` for option adapters (where `Position.Option` is also
required). The zero value is reserved as a programming error and rejected
by the compliance suites.

## Private Streams And TradingAccount

`TradingAccount` depends on adapter behavior. It is not a separate adapter interface.

Practical readiness rules:

- `WatchOrders` is mandatory for lifecycle claims
- `WatchOrders` is mandatory for TradingAccount readiness
- `WatchFills` is optional; return `ErrNotSupported` so runtime health can report it
- `WatchPositions` is additive coverage, not the universal gate
- funding, open interest, and market analytics are optional adapter capabilities, never TradingAccount dependencies
- unsupported shared private surfaces must return `exchanges.ErrNotSupported`, not no-op success

Responsibility split:

1. `FetchAccount` establishes the initial coherent snapshot
2. `WatchOrders` supplies order lifecycle deltas
3. `WatchFills` enriches execution detail when supported
4. `WatchPositions` supplies position deltas when supported
5. `TradingAccount` owns caching, fan-out, tracked `OrderFlow` updates, stream health, and periodic reconciliation

If the exchange lacks a usable private order stream, do not claim `lifecycle-capable` or `trading-account-capable`.

## Live Test Wiring

Live adapter integration is incomplete until `adapter_test.go` wires the shared `testsuite` coverage for the claimed support level.

Required wiring:

- update the repository-root `.env.example`
- use `internal/testenv` for env loading; SDK write tests use
  exchange-specific enable flags, while adapter/full regression suites may
  continue using the script-managed `RUN_FULL` / `RUN_SOAK` gates
- construct the real adapter from env-backed options
- wire the shared suite matrix that matches the adapter's capability level
- use clear skips only for missing credentials, missing symbols, or genuine exchange limitations

Environment variable guidance:

- use uppercase exchange-prefixed names
- keep auth names aligned with `options.go`
- add `*_PERP_TEST_SYMBOL` and `*_SPOT_TEST_SYMBOL` only for supported markets
- add `*_QUOTE_CURRENCY` only when the adapter exposes quote configuration

Stable defaults matter:

- use liquid, durable symbols
- match the adapter's default quote from `options.go`
- avoid ephemeral listings or promo-only markets

## Do Not Ship If

Stop immediately if any of these are true:

- peer selection was skipped or one package was copied wholesale without re-evaluating each concern
- the adapter capability level is undefined or the wired `testsuite` coverage does not match it
- the registry advertises a market type with no real adapter behind it
- `WatchOrders` is missing but lifecycle or TradingAccount support is claimed
- unsupported shared surfaces return no-op success instead of `exchanges.ErrNotSupported`
- `FetchOrderByID` is implemented by scanning only open orders
- adapter files own signing, raw REST construction, wire structs, or WebSocket lifecycle internals that belong in `sdk/`
- stream methods report success while doing nothing
- local orderbook is claimed as supported but never reaches a non-`nil` synced state
- live test prerequisites are undocumented or `adapter_test.go` is missing

## Reference Files

Start from the smallest authoritative set:

- `exchange.go`
- `models.go`
- `errors.go`
- `utils.go`
- `registry.go`
- `trading_account.go`
- `trading_account_state.go`
- `order_flow.go`
- `testsuite/compliance.go`
- `testsuite/order_suite.go`
- `testsuite/lifecycle_suite.go`
- `testsuite/trading_account_suite.go`
- `testsuite/helpers.go`
- `adapter/<exchange>/options.go`
- `adapter/<exchange>/register.go`
- `adapter/<exchange>/perp_adapter.go`
- `adapter/<exchange>/spot_adapter.go`
- `adapter/<exchange>/adapter_test.go`

For final review before merge, also use `docs/superpowers/checklists/exchange-adapter-review.md`.
