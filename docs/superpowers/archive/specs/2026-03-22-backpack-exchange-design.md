# Backpack Exchange Integration Design

## Goal

Add Backpack exchange support to this repository with both spot and perp adapters, including:

- public market data
- authenticated account and trading flows
- registry-based construction
- shared `testsuite` coverage

This work also serves as the validation pass for the existing project skill `adding-exchange-adapters`.

## Scope

In scope:

- new `backpack/` exchange package
- new `backpack/sdk/` low-level REST and WebSocket clients
- spot and perp adapter construction
- market discovery and symbol mapping from Backpack `/markets`
- account, balances, orders, positions, funding, public market data, and orderbook streaming
- adapter integration tests wired into `testsuite`
- `.env.example` additions for Backpack credentials

Out of scope:

- margin, borrow/lend, RFQ, or strategy APIs
- generated OpenAPI client code
- a Backpack-specific skill beyond validation of `adding-exchange-adapters`

## Constraints And Source Of Truth

Repository constraints:

- adapter-layer symbols remain base symbols such as `"BTC"` rather than exchange-native symbols
- adapter-facing numeric values must use `decimal.Decimal`
- market-type behavior must match `Exchange`, `PerpExchange`, and `SpotExchange` in `exchange.go`
- new adapters should follow the existing `options.go` / `register.go` / `*_adapter.go` / `adapter_test.go` package pattern

Backpack API constraints from the official docs:

- REST base URL: `https://api.backpack.exchange/`
- WebSocket base URL: `wss://ws.backpack.exchange`
- signed requests require `X-API-Key`, `X-Signature`, `X-Timestamp`, and optional `X-Window`
- request signing uses an ED25519 keypair and an instruction-prefixed canonical string
- `/markets` returns both spot and perp market metadata by default
- public streams use `SUBSCRIBE` / `UNSUBSCRIBE`

## Architecture

### Package layout

```text
backpack/
  options.go
  register.go
  common.go
  perp_adapter.go
  spot_adapter.go
  funding.go
  orderbook.go            # or split into perp_orderbook.go / spot_orderbook.go if payloads diverge
  adapter_test.go
  sdk/
    client.go
    signer.go
    public_rest.go
    private_rest.go
    ws.go
    types.go
```

### Layer responsibilities

`backpack/sdk/` owns:

- base URL management
- ED25519 signing
- REST request encoding and decoding
- public and private WebSocket connection handling
- Backpack-native request and response types

`backpack/` owns:

- mapping Backpack-native data to shared `exchanges` models
- symbol translation between base symbols and Backpack market symbols
- unified interface behavior for spot and perp
- local orderbook and local-state integration

This keeps Backpack-specific protocol complexity out of the root adapter interface layer and matches the repository's existing two-layer design.

## Configuration

Backpack integration tests will use these environment variables:

- `BACKPACK_API_KEY`
- `BACKPACK_PRIVATE_KEY`
- `BACKPACK_SPOT_TEST_SYMBOL`
- `BACKPACK_PERP_TEST_SYMBOL`

Optional:

- `BACKPACK_QUOTE_CURRENCY`

`BACKPACK_API_KEY` and `BACKPACK_PRIVATE_KEY` will both be read from `.env`. The implementation may still derive and verify the public key from the private key internally, but the configured API key remains the source used for request headers.

Quote currency will default to `USDC` unless Backpack metadata shows a different supported default is required.

## Adapter Behavior

### Market discovery and symbol mapping

The adapter will call `/markets` during startup and build per-market maps for:

- base symbol to spot market symbol
- base symbol to perp market symbol
- symbol details such as min quantity, min notional, tick size, step size, precision, and status

This avoids hardcoding Backpack symbols and keeps spot/perp routing driven by live market metadata.

`FetchSymbolDetails` must populate every field required by `testsuite/helpers.go`, especially:

- `PricePrecision`
- `QuantityPrecision`
- `MinQuantity`
- `MinNotional`

The implementation should derive precision from Backpack `tickSize` and `stepSize` rather than guessing decimal places.

### Construction and registry

`register.go` will register `BACKPACK` and support:

- `exchanges.MarketTypeSpot` via `NewSpotAdapter`
- `exchanges.MarketTypePerp` via `NewAdapter`

Unsupported market types will fail explicitly in the registry constructor.

### Authentication

The low-level signer will build Backpack canonical signing strings by:

1. ordering body or query keys alphabetically
2. serializing them in query-string form
3. prefixing the Backpack instruction name
4. appending timestamp and window values
5. signing with the configured ED25519 private key

Authenticated REST and private WebSocket subscription flows will both go through the same signing helper.

### Public market data

Both spot and perp adapters must support:

- `FetchTicker`
- `FetchOrderBook`
- `FetchTrades`
- `FetchKlines`
- `FetchSymbolDetails`
- `FetchFeeRate` when supported by Backpack data
- `WatchOrderBook`
- `WatchTicker`
- `WatchTrades`
- `WatchKlines`
- `StopWatchOrderBook`
- `StopWatchTicker`
- `StopWatchTrades`
- `StopWatchKlines`

### Private trading and account behavior

Spot and perp adapters must support:

- `FetchAccount`
- `FetchBalance`
- `FetchOpenOrders`
- `FetchOrder`
- `PlaceOrder`
- `CancelOrder`
- `CancelAllOrders`
- `WatchOrders`
- `StopWatchOrders`

Perp adapter must additionally support:

- `FetchPositions`
- `FetchFundingRate`
- `FetchAllFundingRates`
- `WatchPositions`
- `StopWatchPositions`
- `SetLeverage`
- `ModifyOrder`

Spot adapter must additionally support:

- `FetchSpotBalances`
- `TransferAsset`

Spot-only methods and perp-only methods must remain behind the proper shared interfaces.

If Backpack does not expose a shared-interface method, the adapter must implement it with an explicit `exchanges.ErrNotSupported`-style error path rather than leaving the contract ambiguous. Likely candidates are `SetLeverage`, `ModifyOrder`, or `TransferAsset`, pending final Backpack API confirmation.

For perp cleanup and local-state correctness, `FetchAccount()` on the perp adapter must include `Account.Positions` populated consistently with `FetchPositions()`. This repository's order-suite cleanup uses `FetchAccount().Positions`, not only `PerpExchange.FetchPositions()`.

### Order status mapping

The implementation must map Backpack order and execution states into the repository's shared `OrderStatus`, `OrderType`, `OrderSide`, `TimeInForce`, and `PositionSide` values.

This mapping is a primary correctness risk because `testsuite` depends on:

- successful `WatchOrders` terminal updates
- correct `FilledQuantity`
- reliable identification by `OrderID` or `ClientOrderID`

### Orderbook handling

`WatchOrderBook` will subscribe to Backpack depth streams and maintain a local orderbook that satisfies:

- `WatchOrderBook` blocks until initial state is ready
- `GetLocalOrderBook` returns non-`nil` after subscription
- bids remain sorted descending
- asks remain sorted ascending

If Backpack depth streams require an initial REST snapshot plus deltas, that snapshot bootstrapping will live in the orderbook implementation. If spot and perp depth payloads diverge materially, the implementation should split `orderbook.go` into `spot_orderbook.go` and `perp_orderbook.go` to match peer-package patterns.

## Testing Strategy

### TDD sequence

Implementation will follow repository TDD expectations:

1. add Backpack-focused failing tests where possible for deterministic logic such as signing, symbol mapping, and status mapping
2. run those tests to confirm failure
3. implement minimal code
4. rerun targeted tests
5. wire live integration coverage in `backpack/adapter_test.go`

### Adapter test matrix

`backpack/adapter_test.go` will include:

- spot compliance
- spot orders
- spot lifecycle
- perp compliance
- perp orders
- perp lifecycle
- perp local state

Test symbols will come from:

- `BACKPACK_SPOT_TEST_SYMBOL`
- `BACKPACK_PERP_TEST_SYMBOL`

This avoids hardcoding assumptions about which Backpack markets are enabled in the test account.

The exact suite matrix is still exchange-specific:

- spot may need `OrderSuiteConfig` skips such as `SkipSlippage`
- spot `LocalState` should only be enabled if Backpack spot private streams are sufficient to satisfy the suite
- spot and perp may require different symbols and different skip flags

`adapter_test.go` should follow the style of existing packages by making these choices explicit instead of assuming a symmetric matrix.

### Verification checkpoints

Before claiming success:

- unit tests for Backpack deterministic logic pass
- `go test ./backpack/...` passes with and without credentials where appropriate
- live adapter suites pass once Backpack env vars are configured
- registry construction works for both spot and perp

## Risks

1. Backpack private WebSocket auth or subscription shape may differ from the current interpretation of the docs.
2. Spot and perp symbol conventions may not reduce cleanly to a single base symbol for every market.
3. `FetchAccount` shape and `Account.Positions` completeness may block order-suite cleanup even if `FetchPositions` works.
4. Some test-suite assumptions may require Backpack-specific skips or different symbol choices.
5. Funding or position endpoints may expose states that do not map one-to-one with existing shared models.

## Success Criteria

The integration is complete when:

- `backpack` is available through direct construction and registry construction
- spot and perp public data paths work
- authenticated account and order flows work with configured Backpack credentials
- `backpack/adapter_test.go` wires the shared suites
- the implementation passes the relevant live tests after the user adds Backpack env vars
- the `adding-exchange-adapters` skill is shown to be sufficient for guiding a real new-exchange integration in this repository
