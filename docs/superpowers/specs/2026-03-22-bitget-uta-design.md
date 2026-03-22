# Bitget UTA Spot And Perp Adapter Design

## Goal

Add a new `bitget` exchange package to this repository that supports:

- UTA `SPOT`
- UTA `USDT-FUTURES`
- UTA `USDC-FUTURES`

The first implementation target is production-only mainnet support with:

- public market data
- private account and trading
- shared `testsuite` wiring
- live integration test wiring

This design explicitly excludes:

- classic Bitget account APIs
- demo trading / paper trading
- `COIN-FUTURES`

## Why Bitget Needs A Dedicated Adapter

Bitget UTA uses one unified `/api/v3` surface for spot and futures, with market selection driven by `category`. It also uses a unified authentication model and private WebSocket channels for order and position updates.

That makes Bitget a good fit for:

- one repository package: `bitget/`
- one flat low-level SDK: `bitget/sdk/`
- two adapter surfaces on top of the same low-level client:
  - `NewAdapter` for perp
  - `NewSpotAdapter` for spot

## Scope And Capability Level

This adapter should ship at `local-state-capable` level for the supported Bitget UTA products.

That means:

- public REST market data is implemented
- trading methods are implemented
- `FetchAccount` is implemented
- `WatchOrders` is real
- `RunLifecycleSuite` is expected to pass
- `RunLocalStateSuite` is expected to pass

Spot and perp do not need identical capability sets.

Planned support split:

- spot:
  - real `WatchOrders`
  - no `WatchPositions`
  - `TransferAsset` returns `exchanges.ErrNotSupported`
- perp:
  - real `WatchOrders`
  - target `WatchPositions` support in v1 when Bitget’s private position stream is strong enough, but this is not a merge gate for current shared local-state readiness
  - `SetLeverage` supported
  - `ModifyOrder` supported
- funding-rate methods may return `exchanges.ErrNotSupported` in v1 if UTA v3 support is not direct enough in the first pass

## User Constraints Confirmed

The user has already fixed these requirements:

1. Only formal production environment support is required.
2. Demo trading must not be implemented in this first version.
3. Constructors must not require credentials for public market-data access.
4. If credentials are provided, adapter initialization should validate that the account is UTA-compatible; non-UTA private usage should fail during initialization instead of failing later.
5. The first version should support spot plus USDT-settled and USDC-settled futures only.

## External API Facts This Design Depends On

The design relies on these current Bitget UTA facts:

- UTA quickstart defines one REST auth model using `ACCESS-KEY`, `ACCESS-SIGN`, `ACCESS-TIMESTAMP`, and `ACCESS-PASSPHRASE`.
- UTA mainnet REST domain is `https://api.bitget.com`.
- UTA private and public WebSocket domains are:
  - `wss://ws.bitget.com/v3/ws/public`
  - `wss://ws.bitget.com/v3/ws/private`
- market instruments are queried from `GET /api/v3/market/instruments`
- private order detail exists at `GET /api/v3/trade/order-info`
- pending order listing exists in the UTA trade group
- private order updates exist on the UTA private order channel

These should be re-verified while implementing against the official Bitget docs.

## Recommended Architecture

### Top-Level Package

Create:

```text
bitget/
  options.go
  register.go
  common.go
  perp_adapter.go
  spot_adapter.go
  orderbook.go
  adapter_test.go
  sdk/
```

Interpretation:

- `options.go`
  - adapter options and quote validation
- `register.go`
  - `BITGET` registry wiring and market dispatch
- `common.go`
  - shared category, symbol, enum, and private-validation helpers
- `perp_adapter.go`
  - UTA USDT/USDC futures shared-interface implementation
- `spot_adapter.go`
  - UTA spot shared-interface implementation
- `orderbook.go`
  - public WS book subscription plus local-book sync helpers
- `adapter_test.go`
  - live shared-suite entrypoints and env handling

### Low-Level SDK

Use a flat `bitget/sdk/` layout because UTA v3 is one low-level API family with category-driven divergence instead of separate protocol families.

Create:

```text
bitget/sdk/
  client.go
  auth.go
  types.go
  public_rest.go
  private_rest.go
  public_ws.go
  private_ws.go
```

Responsibilities:

- `client.go`
  - base URL management
  - shared HTTP client
  - request helper
- `auth.go`
  - REST signing
  - private WebSocket login signing
- `types.go`
  - instruments, ticker, trade, order, account, position, and WS payload structs
- `public_rest.go`
  - instruments, ticker, depth, trades, klines
- `private_rest.go`
  - account, account settings, balances, order placement, cancellation, modification, open orders, order lookup, leverage
- `public_ws.go`
  - public channel connection and subscriptions
- `private_ws.go`
  - private channel login, order channel, position channel

## Why Not Separate `sdk/spot` And `sdk/perp`

Bitget UTA v3 differs primarily by `category`:

- `SPOT`
- `USDT-FUTURES`
- `USDC-FUTURES`

The signing model, hostnames, and a large portion of REST and WS mechanics are shared.

Splitting low-level code into `sdk/spot` and `sdk/perp` would duplicate:

- auth logic
- transport lifecycle
- many wire types
- channel management

This repository should therefore treat Bitget more like Backpack’s unified low-level surface than Binance’s clearly split spot/perp surfaces.

## Options Design

Proposed `Options`:

```go
type Options struct {
    APIKey        string
    SecretKey     string
    Passphrase    string
    QuoteCurrency exchanges.QuoteCurrency
    Logger        exchanges.Logger
}
```

Quote rules for v1:

- support only `USDT` and `USDC`
- if empty, default to `USDT`

Meaning:

- spot adapter only exposes instruments quoted in the chosen quote currency
- perp adapter only exposes futures in the chosen settlement family
- do not mix USDT and USDC symbol universes inside one adapter instance

This keeps symbol selection and `SymbolDetails` deterministic.

## Constructor Behavior

### Public Construction

These constructors must work without credentials:

- `NewAdapter(ctx, Options{})`
- `NewSpotAdapter(ctx, Options{})`

They should still be able to:

- load instruments
- support public REST market data
- support public WS orderbook

### Private Validation Rules

If any private credential field is provided, the adapter must treat the instance as a private-capable adapter and validate private readiness during initialization.

Initialization should verify:

- credentials are complete enough to sign requests
- account is UTA-compatible
- account settings are compatible with the supported product families

If these checks fail, initialization returns an error.

If no credentials are provided:

- constructor succeeds
- auth-gated private methods must fail with a Bitget-scoped auth error path, ideally an `exchanges.ExchangeError` wrapping `exchanges.ErrAuthFailed`
- do not return `exchanges.ErrNotSupported` for auth-gated methods that the adapter can support once valid credentials are present

This preserves public-market-data usability while preventing a half-broken private adapter instance.

## Registry Design

Register key:

- `BITGET`

Dispatch:

- `MarketTypePerp` -> `NewAdapter`
- `MarketTypeSpot` -> `NewSpotAdapter`

Unsupported markets:

- reject anything else with a clear error

## Symbol And Category Model

The adapter boundary should continue using base symbols only:

- `BTC`
- `ETH`
- `SOL`

The adapter maps those into Bitget UTA symbols such as:

- `BTCUSDT`
- `BTCUSDC`

Category selection should be internal:

- spot adapter uses `SPOT`
- perp adapter uses:
  - `USDT-FUTURES` when quote is `USDT`
  - `USDC-FUTURES` when quote is `USDC`

Do not expose Bitget’s native `category` choice to repository callers.

## Shared Interface Coverage

### Spot Adapter

Implement:

- `FetchTicker`
- `FetchOrderBook`
- `FetchTrades`
- `FetchKlines`
- `PlaceOrder`
- `CancelOrder`
- `CancelAllOrders`
- `FetchOrderByID`
- `FetchOrders`
- `FetchOpenOrders`
- `FetchAccount`
- `FetchBalance`
- `FetchSpotBalances`
- `FetchSymbolDetails`
- `FetchFeeRate`
- `WatchOrderBook`
- `WatchOrders`
- `StopWatchOrderBook`
- `StopWatchOrders`
- `StopWatchPositions`
- `StopWatchTicker`
- `StopWatchTrades`
- `StopWatchKlines`

Unsupported in v1:

- `WatchPositions`
- `TransferAsset`
- optionally `WatchTicker`, `WatchTrades`, `WatchKlines` if those channels add too much initial complexity

Unsupported shared surfaces must return `exchanges.ErrNotSupported`.

### Perp Adapter

Implement:

- all relevant shared exchange methods above
- `FetchPositions`
- `ModifyOrder`
- `SetLeverage`
- `WatchPositions` if the UTA private position stream is stable enough; otherwise return `exchanges.ErrNotSupported` honestly in v1 while keeping local-state readiness anchored on `FetchAccount` plus `WatchOrders`

Potentially defer:

- `FetchFundingRate`
- `FetchAllFundingRates`

If funding-rate endpoints are not integrated in v1, return `exchanges.ErrNotSupported` honestly.

## Order Query Semantics

Bitget must follow the repository’s current order-query split:

- `FetchOrderByID`
  - use dedicated order-detail endpoint
  - return terminal orders when available
  - return `exchanges.ErrOrderNotFound` for true misses
- `FetchOrders`
  - in the first implementation pass, default to `exchanges.ErrNotSupported` unless implementation confirms a direct Bitget UTA history surface that is broad enough to satisfy the repository contract
- `FetchOpenOrders`
  - return only open orders

Bitget must not implement `FetchOrderByID` by scanning only open orders.

Planned `RunOrderQuerySemanticsSuite` capability flags for v1:

- spot:
  - `SupportsOpenOrders: true`
  - `SupportsTerminalLookup: true`
  - `SupportsOrderHistory: false`
- perp:
  - `SupportsOpenOrders: true`
  - `SupportsTerminalLookup: true`
  - `SupportsOrderHistory: false`

Only raise `SupportsOrderHistory` to `true` after implementation proves Bitget UTA has a sufficiently broad symbol-scoped order-history path for the shared contract.

## Private Streams And Local State

To meet `local-state-capable`, Bitget must provide:

- `FetchAccount`
- `WatchOrders`

Perp should additionally provide `WatchPositions` if Bitget’s private stream is usable, but this is an additive improvement rather than a hard gate for current shared local-state readiness.

Spot may legitimately return `exchanges.ErrNotSupported` from `WatchPositions`, and perp may also return `exchanges.ErrNotSupported` in v1 if the position stream is too weak to support clean shared semantics.

The intended local-state pattern is:

1. fetch initial account snapshot
2. start private order stream
3. start private position stream where supported
4. let shared local-state infrastructure maintain live state

Returning success from stream methods without a real subscription is forbidden.

## Orderbook Strategy

Use public WebSocket orderbook subscription plus a synchronized local orderbook.

The repository requirement remains:

- `WatchOrderBook` should block until the local orderbook is ready
- `GetLocalOrderBook` should return non-`nil` after successful subscription and sync

Implementation should mirror the nearest reliable peer in this repository rather than inventing a custom event model.

## Test Strategy

`bitget/adapter_test.go` should wire:

- `RunAdapterComplianceTests`
- `RunOrderSuite`
- `RunOrderQuerySemanticsSuite`
- `RunLifecycleSuite`
- `RunLocalStateSuite`

The first implementation should wire `RunOrderQuerySemanticsSuite` with:

- spot: `SupportsOpenOrders=true`, `SupportsTerminalLookup=true`, `SupportsOrderHistory=false`
- perp: `SupportsOpenOrders=true`, `SupportsTerminalLookup=true`, `SupportsOrderHistory=false`

Environment handling should follow the Backpack pattern:

- search `.env`
- `../.env`
- `../../.env`
- `../../../.env`

Expected env vars:

- `BITGET_API_KEY`
- `BITGET_SECRET_KEY`
- `BITGET_PASSPHRASE`
- `BITGET_PERP_TEST_SYMBOL`
- `BITGET_SPOT_TEST_SYMBOL`
- `BITGET_QUOTE_CURRENCY`

`BITGET_QUOTE_CURRENCY` should be either `USDT` or `USDC`.

`.env.example` must be updated accordingly.

## Recommended Peer Packages

Use different peers by concern:

- `okx`
  - registry and options shape
  - spot+perp centralized exchange structure
- `backpack`
  - resilient `.env` lookup
  - modern live-test wiring
  - unified low-level surface shape
- `binance`
  - centralized exchange market-data/trading patterns where useful

Do not copy any one package wholesale.

## Risks

### 1. Over-claiming history-order support

Bitget’s exact history-order coverage must be confirmed before claiming `FetchOrders` support.

Mitigation:

- use dedicated order-detail endpoint for `FetchOrderByID`
- only claim `FetchOrders` if the UTA API really exposes sufficient history semantics
- otherwise return `exchanges.ErrNotSupported`

### 2. Mixing quote universes in one adapter instance

Supporting both USDT and USDC inside a single adapter instance would complicate symbol resolution and tests.

Mitigation:

- bind one adapter instance to one quote currency

### 3. Private initialization ambiguity

Allowing incomplete credentials could create inconsistent private behavior.

Mitigation:

- no credentials: public-only behavior is fine
- partial or invalid credentials: initialization should fail once private mode is implied

### 4. Stream support drift

Claiming local-state readiness without real private order streams would be a false completion signal.

Mitigation:

- treat `WatchOrders` as mandatory
- treat `WatchPositions` as optional for spot and additive for perp in v1

## Success Criteria

This design is successful when implementation yields:

- a new `bitget` package registered as `BITGET`
- public market data without credentials
- private spot and perp trading with UTA-only validation when credentials are present
- support for spot plus USDT/USDC futures only
- shared `testsuite` live wiring
- honest `ErrNotSupported` behavior for any deferred surface
