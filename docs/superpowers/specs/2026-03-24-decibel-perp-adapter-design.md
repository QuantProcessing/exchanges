# Decibel Perp Adapter Design

## Status

Implemented and merged to `main`.

## Problem

The repository currently has no Decibel adapter. Decibel does not match the repository's more common exchange pattern where REST and WebSocket cover both reads and writes. Instead:

- public and private reads go through authenticated REST and authenticated WebSocket APIs
- order placement and cancellation go through Aptos on-chain transactions
- market and account resources are keyed primarily by Aptos addresses rather than by exchange-native symbol strings

That protocol split creates two risks if the adapter is added carelessly:

- adapter code becomes a single large file that mixes symbol mapping, HTTP auth, WebSocket recovery, and chain transaction building
- repository-level trading semantics drift because the adapter returns transport-specific identifiers or exposes Decibel account/address details directly at the unified interface boundary

The design goal is to add a Decibel perpetual futures adapter that behaves like the other repository adapters at the `Exchange` and `PerpExchange` boundary while isolating Decibel-specific protocol complexity into explicit SDK seams.

## Goals

- Add a new `decibel` package registered for `MarketTypePerp`.
- Support the repository's current first-class validation target for a trading adapter:
  - compliance tests
  - order suite
  - lifecycle suite
- Keep the public adapter boundary on base symbols such as `"BTC"`.
- Use Decibel market metadata to drive symbol translation, precision handling, and on-chain quantity/price encoding.
- Support authenticated private reads through REST and WebSocket.
- Support order placement and cancellation through Aptos transactions signed from a user private key.
- Preserve repository-standard sentinel error behavior and order/account semantics.
- Leave a clear package shape for a future spot implementation without implementing spot now.

## Non-Goals

- Implement spot trading in this phase.
- Implement every Decibel market-data endpoint or stream in the first release.
- Guarantee first-release support for `FetchTrades`, `FetchKlines`, `WatchTicker`, `WatchTrades`, `WatchKlines`, `FetchFundingRate`, or `FetchAllFundingRates`.
- Redesign repository-wide adapter interfaces to better fit hybrid on-chain exchanges.
- Expose Decibel-specific address concepts at the adapter-facing API boundary.

## Confirmed Product Decisions

These decisions were explicitly confirmed during brainstorming and are fixed for this design:

- scope includes Decibel REST, WebSocket, and on-chain trading together in the same adapter effort
- the adapter must be complete enough to satisfy repository live trading validation, not just readonly access
- constructor auth input is `api_key + private_key + subaccount_addr`
- `account_addr` is derived internally from the private key rather than required from callers
- package structure should be split into SDK layers rather than implemented directly inside the adapter
- first release registers only `MarketTypePerp`
- spot structure may be reserved, but spot behavior is not implemented in this phase
- first-release success is measured by `compliance + order + lifecycle` rather than maximal feature coverage
- adding Aptos-related Go dependencies is allowed where needed

## Final Implementation Notes

The merged implementation followed the approved design, with a few live-behavior clarifications that are worth recording explicitly:

- constructor auth stayed `api_key + private_key + subaccount_addr`
- `private_key` is the Decibel API wallet private key
- `subaccount_addr` is the Decibel trading account address
- the Aptos wallet address derived from `private_key` is stored internally and used for private WebSocket topics
- account, position, open-order, order-history, and single-order REST reads use the configured trading account / subaccount address
- single-order reconciliation uses `GET /api/v1/orders` and must pass exactly one of `order_id` or `client_order_id`
- private order updates use wallet-address `order_updates` plus wallet-address `user_order_history`
- `user_order_history` is treated as terminal-history support, not as the sole source of active order transitions
- `open_orders` and `order_history` pagination use `limit` / `offset` with `total_count`
- Aptos transaction hashes are not exposed as stable Decibel `OrderID` values when reconciliation times out

## Design Principles

1. Keep the unified adapter contract stable even when Decibel's protocol model is unusual.
2. Treat market metadata as the source of truth for symbol mapping and precision.
3. Separate read transport concerns from on-chain write concerns.
4. Prefer deterministic adapter behavior over permissive but ambiguous fallbacks.
5. Make first-release unsupported capability boundaries explicit with `ErrNotSupported`.

## Package Structure

The new package should follow the repository's standard adapter shape with Decibel-specific SDK internals behind it.

### Exchange package files

- `decibel/options.go`
- `decibel/register.go`
- `decibel/common.go`
- `decibel/perp_adapter.go`
- `decibel/adapter_test.go`

### SDK subpackages

- `decibel/sdk/rest`
- `decibel/sdk/ws`
- `decibel/sdk/aptos`

### Responsibility split

`decibel/options.go`

- defines `Options`
- validates auth requirements and quote currency rules
- defaults logger and any stable package-level defaults

`decibel/register.go`

- registers `DECIBEL`
- translates registry options into `Options`
- supports `MarketTypePerp`
- rejects `MarketTypeSpot` and any other market type with a constructor error

`decibel/common.go`

- stores market metadata models used by the adapter
- implements base symbol to market-address mapping
- implements market-address to base symbol reverse lookup
- centralizes precision validation, chain-unit conversion, order status normalization, and error mapping

`decibel/perp_adapter.go`

- owns the public unified adapter behavior
- coordinates REST reads, WebSocket subscriptions, and Aptos writes
- maintains local caches for market metadata and temporary order identity reconciliation

`decibel/sdk/rest`

- owns authenticated HTTP requests and Decibel REST response models
- exposes market, account, position, and order query methods used by the adapter

`decibel/sdk/ws`

- owns authenticated WebSocket connection setup
- manages ping/pong, reconnect, and subscription replay
- emits normalized internal events for market depth and private order/account updates

`decibel/sdk/aptos`

- owns Aptos transaction construction, signing, and submission
- encapsulates Move entry-function calls for place/cancel operations
- exposes typed request/response helpers suitable for adapter orchestration

## Constructor And Auth Model

### Options

The initial `Options` contract should be:

```go
type Options struct {
    APIKey         string
    PrivateKey     string
    SubaccountAddr string
    QuoteCurrency  exchanges.QuoteCurrency
    Logger         exchanges.Logger
}
```

### Access model

Decibel documents authenticated access for the REST and WebSocket surfaces used by this adapter, including market bootstrap paths. Because the adapter also depends on authenticated market metadata during initialization, the constructor should use a strict-completeness model rather than a public-only mode.

The adapter therefore supports one constructor state in v1:

- `full-trading`

`full-trading`

- requires `APIKey`, `PrivateKey`, and `SubaccountAddr` together
- enables authenticated REST, authenticated WebSocket, and on-chain writes

All partial or empty credential states are rejected in the constructor with `ErrAuthFailed`.

### Derived state

- `accountAddr` is derived from the Aptos private key and stored internally
- `subaccountAddr` remains the runtime target for account queries and trading
- `marketAddr` values come from market metadata and are never required from callers

In the merged adapter, `accountAddr` is specifically the API wallet address used for private WebSocket subscriptions, while `subaccountAddr` remains the REST and trading target.

### Quote currency policy

The adapter should validate quote currency against the actual set represented by Decibel perpetual markets. If the metadata and product model prove there is only one valid quote domain for the supported perp markets, the adapter should default to that value and reject any others. If multiple supported quote domains exist in the market list, validation should reflect that set explicitly.

The implementation must not hardcode a quote rule until it is verified from Decibel market metadata.

## Market Metadata And Symbol Semantics

### Metadata source

The adapter loads market metadata from Decibel's markets REST API during construction or early initialization and caches the fields required for trading semantics:

- market name
- market address
- price decimals
- size decimals
- tick size
- minimum size
- leverage-related limits where applicable

Constructor behavior should be fail-fast if essential market metadata cannot be loaded, because the adapter cannot safely validate symbols or build on-chain orders without it and the metadata path itself requires authenticated access.

### Public symbol contract

All adapter-facing methods continue to use repository-standard base symbols such as `"BTC"`.

### Internal lookup model

The adapter maintains:

- `base symbol -> market metadata`
- `market address -> base symbol`
- any secondary aliases needed to interpret Decibel market names deterministically

If market metadata would map one base symbol to multiple active perp markets, constructor initialization fails rather than selecting one implicitly.

### Format and extract behavior

- `FormatSymbol("BTC")` returns the internal Decibel-native identifier used for trading and subscriptions
- for this adapter, the preferred internal identifier is `market_addr`
- `ExtractSymbol` converts market addresses or recognized Decibel market-name strings back to the base symbol

The public contract remains base-symbol-first even though the internal transport contract is address-first.

### Symbol details

`FetchSymbolDetails` should use cached metadata to populate:

- `Symbol`
- `PricePrecision`
- `QuantityPrecision`
- `MinQuantity`
- `MinNotional` when it can be computed or mapped safely from metadata

If Decibel does not provide a clean `MinNotional`, the adapter should choose the most conservative deterministic mapping available and document it in code comments.

## Precision And Chain Unit Conversion

Precision handling must be centralized and metadata-driven.

### Adapter-side validation

Before any order write:

- validate symbol existence
- validate quantity precision against market size decimals
- validate limit price precision against market price decimals and tick size
- validate minimum quantity and any directly enforceable market minima

Violations map to repository sentinel errors such as `ErrInvalidPrecision`, `ErrMinQuantity`, or `ErrMinNotional`.

### Chain encoding

Decibel write requests require Aptos-compatible encoded numeric values derived from market precision fields. The implementation must:

- quantize `decimal.Decimal` values first
- convert them into the exact integer or fixed-point representation expected by the Decibel Move entry functions
- keep that encoding logic inside `decibel/common.go` or `decibel/sdk/aptos`, not spread across adapter methods

The adapter must never use floating-point math for price or quantity conversion.

## Trading Write Path

### Supported first-release order surface

First release only needs the order behaviors required by current repository validation:

- market orders
- limit orders
- cancel order
- reduce-only close flow for perp lifecycle tests

If Decibel's protocol requires market orders to be expressed as aggressive limits or another protected-order variant, that translation happens inside the adapter or Aptos SDK layer without changing the unified interface.

### PlaceOrder flow

`PlaceOrder` should:

1. require full trading credentials
2. validate and normalize `OrderParams`
3. resolve base symbol to market metadata
4. quantize price and size and encode them for chain submission
5. build and sign the Aptos transaction for Decibel place-order entrypoints
6. submit the transaction
7. reconcile the resulting chain submission to a Decibel order identity usable by repository tests
8. return an `exchanges.Order` object suitable for subsequent status tracking

### Order identity strategy

This adapter cannot assume that the synchronous chain submission response already contains the final Decibel order identifier. To keep repository semantics stable:

- `PlaceOrder` should try to resolve a real Decibel order identifier before returning when that is feasible within a short bounded wait
- reconciliation may use transaction result data, private order streams, and short REST polling
- `ClientOrderID` should preserve the incoming `OrderParams.ClientID` when the protocol supports it directly
- if direct protocol support is missing, the adapter should still preserve the client ID in adapter-local correlation state where possible

The target behavior is that `PlaceOrder` returns an order object with a stable `OrderID` whenever practical, because the repository order and lifecycle suites expect returned orders to be trackable immediately.

### CancelOrder flow

`CancelOrder` should:

- require full trading credentials
- interpret the incoming `orderID` as the Decibel order identifier
- translate any temporary identity used during reconciliation into the real order identifier before submission if needed
- build and sign the Aptos cancel-order transaction against the correct subaccount and market context
- treat already-completed or already-cancelled orders with repository-standard semantics rather than leaking raw Decibel protocol errors directly

## Read Path Design

### FetchTicker

`FetchTicker` should use the most direct Decibel market-data read that provides:

- last trade or last mark/last price equivalent
- bid/ask when available
- timestamp

Only the fields needed by current repository expectations need to be guaranteed in the first release.

### FetchOrderBook

`FetchOrderBook` should use the REST orderbook snapshot endpoint and normalize prices and quantities to repository models using cached market metadata.

### WatchOrderBook

`WatchOrderBook` should:

- establish the Decibel market-depth stream for the resolved market address
- obtain or synthesize an initial local-book snapshot if the stream itself is not snapshot-complete
- apply incremental updates into the `BaseAdapter` local orderbook
- return only after the initial local orderbook is ready

This behavior is required to align with repository compliance expectations for `GetLocalOrderBook`.

### FetchAccount

`FetchAccount` should combine private account overview, positions, and open-order reads into a repository `Account`.

The adapter should:

- populate balance fields from the most appropriate overview resource
- map positions from Decibel position resources without inferring side from signed quantity alone when explicit side data exists
- populate current open orders using the Decibel open-orders read path

### Order query methods

First-release support priority:

- `FetchOrderByID`
- `FetchOpenOrders`
- `FetchOrders` only to the degree Decibel provides a stable and useful query surface for current adapter tests

If a broader order-history contract cannot be implemented cleanly in the first release, that limitation should be explicit in tests and documentation rather than approximated silently.

## PerpExchange Surface In v1

The first-release status for `PerpExchange` methods is:

- `FetchPositions`: implemented, using the same Decibel account/position sources that back `FetchAccount`
- `SetLeverage`: `ErrNotSupported` in v1 unless Decibel exposes a clean supported write path that can be implemented without expanding the requested scope
- `ModifyOrder`: `ErrNotSupported` in v1
- `FetchFundingRate`: `ErrNotSupported` in v1
- `FetchAllFundingRates`: `ErrNotSupported` in v1

This keeps the first release aligned with the repository validation target while making unsupported perp capabilities explicit.

## Private WebSocket Design

### Authentication

The WebSocket layer must implement Decibel's authenticated connection contract, including its required protocol/header behavior and session lifecycle limits.

### Connection responsibilities

The SDK WebSocket client should manage:

- authenticated dial
- ping/pong handling
- reconnect after disconnect or session expiry
- subscription replay after reconnect

### Order and account state

`WatchOrders` should primarily rely on private order-update streams.

To support repository trading flows reliably:

- order status normalization must cover the states required by the order and lifecycle suites
- the adapter may use short REST polling as a bounded fallback when the private stream lags behind chain submission or reconnect recovery

`WatchPositions` should subscribe to the corresponding account stream if Decibel provides one. If the stream is unavailable or insufficient for first-release requirements, the adapter may return `ErrNotSupported` as long as that does not violate the selected validation target.

## Error Mapping

Decibel-specific protocol errors should be wrapped into repository sentinel errors whenever there is a stable semantic match.

Required mappings include:

- authentication and authorization failures -> `ErrAuthFailed`
- rate limit failures -> `ErrRateLimited`
- unknown symbol or missing market mapping -> `ErrSymbolNotFound`
- order lookup misses or terminal-state cancel misses -> `ErrOrderNotFound`
- precision and tick-size validation failures -> `ErrInvalidPrecision`
- minimum quantity failures -> `ErrMinQuantity`
- minimum notional failures -> `ErrMinNotional`

Raw protocol details should still be preserved through `ExchangeError` where useful for debugging.

## Capability Boundaries For First Release

The first release targets repository validation success, not maximum feature coverage.

The adapter may return `ErrNotSupported` for capabilities outside that target, including:

- `FetchTrades`
- `FetchKlines`
- `WatchTicker`
- `WatchTrades`
- `WatchKlines`
- `FetchFundingRate`
- `FetchAllFundingRates`

These boundaries must be implemented deliberately and tested where stable unsupported behavior matters.

## Testing Strategy

### Unit tests

Add offline unit tests for:

- options validation
- partial-credential rejection
- market-name and market-address mapping
- precision validation
- chain-unit conversion
- order status mapping
- error mapping

### SDK protocol tests

Add focused tests for:

- REST auth header behavior
- WebSocket auth setup
- WebSocket subscription replay behavior
- Aptos transaction-build parameter encoding for place/cancel paths

### Live adapter tests

Add `decibel/adapter_test.go` following repository conventions with at least:

- compliance suite
- order suite
- lifecycle suite

The expected live test environment is:

- `DECIBEL_API_KEY`
- `DECIBEL_PRIVATE_KEY`
- `DECIBEL_SUBACCOUNT_ADDR`
- `DECIBEL_PERP_TEST_SYMBOL`

If additional live configuration is needed for Aptos network selection or Decibel environment selection, that should be surfaced through explicit environment variables rather than hidden defaults.

## Risks And Mitigations

### Risk: order ID is not available immediately after chain submission

Mitigation:

- keep a bounded reconciliation step inside `PlaceOrder`
- use private order updates and short REST polling to resolve the true order identity before giving up

### Risk: market metadata fields do not map cleanly to repository `SymbolDetails`

Mitigation:

- treat market metadata as constructor-critical
- centralize translation and document any conservative fallback chosen for missing fields such as `MinNotional`

### Risk: Decibel WebSocket sessions require aggressive reconnect handling

Mitigation:

- keep reconnect and subscription replay inside the SDK layer
- keep adapter methods dependent on a narrow internal event API rather than on raw WS frames

### Risk: Aptos dependency selection adds avoidable implementation churn

Mitigation:

- choose one Aptos dependency path and isolate it under `decibel/sdk/aptos`
- do not let Aptos-specific transaction building leak into adapter orchestration methods

## Planning Readiness

This design covers one implementation target: a Decibel perpetual adapter with split REST, WebSocket, and Aptos SDK layers. It does not combine unrelated subsystems. The implementation can therefore be planned as a single focused workstream with phased tasks for package scaffolding, metadata and auth handling, read paths, write paths, and live-test enablement.

## Final Verification

The merged implementation was verified with:

- `go test -mod=mod ./decibel/... -count=1`
- `GOCACHE=/tmp/exchanges-gocache RUN_FULL=1 go test -mod=mod ./decibel -run "TestPerpAdapter_(Compliance|Orders|Lifecycle)$" -count=1 -v`
