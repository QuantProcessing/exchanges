# Order Transport Split Design

## Status

Proposed and approved for implementation, pending written-spec review.

## Problem

The current shared trading interface lets several adapters route the same high-level method through different transports:

- `PlaceOrder`
- `CancelOrder`
- `ModifyOrder`

In practice, those methods may use REST or WebSocket depending on `OrderMode`.

That creates a semantic mismatch at the unified interface boundary:

- REST write paths often return a concrete order identifier and other server-confirmed fields
- WebSocket write paths often return only an ACK or a submission-success signal
- callers cannot know from the method name which transport was used
- callers cannot know whether a returned `*Order` is a fully identified order or only a local submission placeholder
- adapter implementations must branch internally on transport while still pretending the public method means one thing

The result is that transport differences leak upward as return-value ambiguity instead of being expressed directly in the contract.

## Goals

- Make order-write transport explicit in the shared `Exchange` and `PerpExchange` interfaces.
- Remove `OrderMode` as a repository abstraction.
- Make REST-backed and WS-backed write paths clearly different at the method boundary.
- Keep synchronous order-return semantics only on the non-WS write path.
- Make WS write methods ACK-oriented and side-effect-only from the caller's perspective.
- Preserve a clear tracking path for WS-submitted orders through `ClientID` plus `WatchOrders`.
- Make unsupported transport capabilities explicit with `ErrNotSupported`.
- Prevent silent fallback from WS write methods to REST.

## Non-Goals

- Do not redesign `WatchOrders` / `WatchFills` semantics in this change.
- Do not redesign order-query semantics such as `FetchOrderByID` or `FetchOrders`.
- Do not require every write operation in the repository to gain a WS variant immediately.
- Do not add a WS variant of `CancelAllOrders` in this phase.
- Do not solve non-REST, non-WS write transports as a general repository abstraction in this phase.

## Confirmed Product Decisions

These decisions were explicitly confirmed during discussion and are fixed for this design:

- transport-specific write methods should be exposed directly on the shared interfaces
- `PlaceOrder` is the primary non-WS write method and remains the method that returns `*Order`
- `PlaceOrderWS` is a distinct WS submission method and returns only `error`
- `CancelOrderWS` returns only `error`
- `ModifyOrderWS` returns only `error`
- `OrderMode` should be removed rather than preserved as the main switching mechanism
- WS write methods must not silently fall back to REST
- WS order placement must rely on caller-provided `OrderParams.ClientID` for later tracking

## Current State Summary

### Shared interface ambiguity

`exchange.go` currently exposes:

```go
PlaceOrder(ctx context.Context, params *OrderParams) (*Order, error)
CancelOrder(ctx context.Context, orderID, symbol string) error
```

and `PerpExchange` currently adds:

```go
ModifyOrder(ctx context.Context, orderID, symbol string, params *ModifyOrderParams) (*Order, error)
```

Those method names do not reveal whether the adapter will use REST or WS.

### BaseAdapter switching

`BaseAdapter` currently owns `OrderMode`, `SetOrderMode`, `GetOrderMode`, and `IsRESTMode`.

That means transport selection is hidden inside adapter internals instead of expressed in the shared API.

### Real return-shape drift already exists

Current adapters already show materially different write semantics:

- REST paths often return exchange-native `OrderID`
- some WS paths return only a minimal placeholder order with `ClientOrderID`
- `LocalState` and shared tests already tolerate delayed `OrderID` discovery through `WatchOrders`

So the repository is already carrying the semantic split. It is simply encoded indirectly.

### Existing tests already protect one key rule

Current WS-order-mode tests, especially in `bitget`, already assert an important behavioral rule:

- if the caller selected the WS path, failure must not silently drop to REST

That rule should survive this refactor and become stronger under explicit WS methods.

## Considered Approaches

### Option 1: Keep one method and continue transport switching internally

This preserves the least surface churn, but it keeps the current ambiguity:

- same method name
- different transport
- different response semantics

This option was rejected because it leaves the core API problem unsolved.

### Option 2: Keep one method but introduce a more explicit result type

This would replace ambiguous `*Order` returns with an ACK-or-order result object.

This helps with return-shape clarity, but it still leaves transport selection hidden. The caller would still not know from the method boundary whether it is asking for a REST submission or a WS submission.

This option was rejected because the user explicitly wants transport choice expressed in the shared interface.

### Option 3: Split write methods by transport at the shared interface boundary

This option makes transport explicit and lets each method own one stable semantic contract.

This is the chosen approach.

## Chosen Public Interface Design

### Exchange

The shared `Exchange` write section should become:

```go
PlaceOrder(ctx context.Context, params *OrderParams) (*Order, error)
PlaceOrderWS(ctx context.Context, params *OrderParams) error
CancelOrder(ctx context.Context, orderID, symbol string) error
CancelOrderWS(ctx context.Context, orderID, symbol string) error
CancelAllOrders(ctx context.Context, symbol string) error
FetchOrderByID(ctx context.Context, orderID, symbol string) (*Order, error)
FetchOrders(ctx context.Context, symbol string) ([]Order, error)
FetchOpenOrders(ctx context.Context, symbol string) ([]Order, error)
```

### PerpExchange

The perp-specific write section should become:

```go
ModifyOrder(ctx context.Context, orderID, symbol string, params *ModifyOrderParams) (*Order, error)
ModifyOrderWS(ctx context.Context, orderID, symbol string, params *ModifyOrderParams) error
```

### Semantic rule

Under this design:

- unsuffixed write methods are the repository's primary non-WS write methods
- `*WS` methods are explicit WebSocket submission methods

For all current dual-transport adapters in this repository, the unsuffixed write methods are the REST-backed path.

## Method Semantics

### `PlaceOrder`

`PlaceOrder` remains the method for synchronous write paths that can return structured order data immediately.

For current dual-transport adapters, this is the REST path.

Contract:

- returns `(*Order, error)`
- may return exchange-confirmed `OrderID`
- may return other normalized order fields already available from the submission response
- must not route through WS internally

### `PlaceOrderWS`

`PlaceOrderWS` is an explicit WS submission method.

Contract:

- returns only `error`
- success means the request was submitted over WS and the adapter received the exchange's success signal or equivalent ACK
- it does not return `OrderID`
- it does not return a synthetic `*Order`
- callers must use `WatchOrders` and, when possible, `FetchOrderByID` for subsequent lifecycle tracking
- it must not fall back to REST

### `CancelOrder`

`CancelOrder` is the primary non-WS cancel path.

For current dual-transport adapters, this is the REST cancel path.

Contract:

- returns only `error`
- must not route through WS internally

### `CancelOrderWS`

`CancelOrderWS` is the explicit WS cancel path.

Contract:

- returns only `error`
- success means the WS cancel request was accepted by the exchange transport layer or acknowledged according to the exchange's protocol
- it must not fall back to REST

### `ModifyOrder`

`ModifyOrder` remains the non-WS amend path that can return normalized order state.

For current dual-transport adapters, this is the REST modify path.

Contract:

- returns `(*Order, error)`
- may re-query after modify if the exchange's non-WS modify path is asynchronous but still supports normalized follow-up lookup
- must not route through WS internally

### `ModifyOrderWS`

`ModifyOrderWS` is the explicit WS amend path.

Contract:

- returns only `error`
- success means the WS amend request was accepted or acknowledged according to the exchange's protocol
- it does not return a synthetic amended order object
- it must not fall back to REST

### `CancelAllOrders`

`CancelAllOrders` remains unchanged in this rollout.

Rationale:

- broad WS batch-cancel support is not consistently available across adapters
- the immediate ambiguity problem is concentrated in single-order write flows
- a later change may split batch writes separately if repository coverage justifies it

## ClientID Rules For WS Placement

### Required caller input

`PlaceOrderWS` must require a non-empty `OrderParams.ClientID`.

If `ClientID` is empty:

- the adapter returns a validation error
- it does not auto-generate a hidden client ID
- it does not submit the order

### Why this is required

Because `PlaceOrderWS` returns only `error`, the caller needs a stable correlation key to identify the resulting order in `WatchOrders`.

That stable correlation key is `ClientID`.

Without a caller-supplied `ClientID`, the repository would be forcing the caller to use a fire-and-forget write path that cannot be tracked reliably through the shared streaming API.

### Adapter-specific encoding constraints

Some exchanges impose tighter `ClientID` rules, for example:

- numeric-only values
- length limits
- character-set limits

Those constraints remain adapter-specific and should still be validated by each adapter. If a provided `ClientID` cannot be encoded for that exchange, the adapter should return a validation error instead of rewriting it into an unrelated hidden identifier.

## Tracking Model For WS Orders

The intended lifecycle for WS placement becomes:

1. caller chooses `ClientID`
2. caller subscribes to `WatchOrders` before placing
3. caller calls `PlaceOrderWS`
4. caller tracks the order by `ClientOrderID`
5. when the exchange later emits a real `OrderID`, the caller or helper layer can reconcile to that identifier

This matches the repository's existing LocalState and shared-test pattern more honestly than returning a transport-shaped placeholder `*Order`.

## LocalState Design Changes

`LocalState.PlaceOrder` should remain the helper for the primary non-WS write path and keep calling `adp.PlaceOrder`.

This design adds:

```go
PlaceOrderWS(ctx context.Context, params *OrderParams) (*OrderResult, error)
```

Behavior:

1. subscribe to order updates before submitting
2. require `params.ClientID`
3. call `adp.PlaceOrderWS(ctx, params)`
4. create an `OrderResult` seeded with:
   - `ClientOrderID`
   - symbol
   - side
   - type
   - quantity
   - price
   - initial pending/new status if needed
5. reconcile real `OrderID` and later status updates from `WatchOrders`

This gives the repository a first-class helper for WS submission tracking without polluting the shared `Exchange` write return contract.

## OrderMode Removal

`OrderMode` should be removed from shared adapter infrastructure.

That includes:

- `OrderMode` type and constants in `exchange.go`
- `orderMode` state in `BaseAdapter`
- `SetOrderMode`
- `GetOrderMode`
- `IsRESTMode`
- any constructor defaults or tests that only exist to preserve `OrderMode`

### Why removal is preferred

Keeping `OrderMode` after explicit transport methods are added would preserve two competing ways to express the same decision:

- call `PlaceOrderWS`
- or call `PlaceOrder` after mutating adapter transport state

That would reintroduce ambiguity immediately. This design therefore removes the switch entirely.

## Adapter Migration Strategy

### Dual-transport adapters

Adapters that currently support both REST and WS writes should split their logic into explicit methods:

- REST logic stays in `PlaceOrder`, `CancelOrder`, `ModifyOrder`
- WS logic moves to `PlaceOrderWS`, `CancelOrderWS`, `ModifyOrderWS`

No method should branch on transport internally after the refactor.

### REST-only adapters

REST-only adapters should:

- keep existing unsuffixed write methods
- implement `*WS` methods with `ErrNotSupported`

Examples likely include:

- Backpack
- any adapter that deliberately supports only non-WS writes

### WS-partial adapters

Adapters that support only a subset of WS writes should expose that subset honestly:

- supported WS write methods perform real WS writes
- unsupported ones return `ErrNotSupported`

No method may silently downgrade to REST.

### Non-REST, non-WS write transports

This repository already has at least one important exception:

- `decibel` writes through Aptos transaction submission rather than REST or WS

This design does not try to force a fake REST/WS classification onto that adapter.

For this rollout, the repository should explicitly document Decibel as a controlled exception:

- `PlaceOrder`, `CancelOrder`, and any related non-WS write methods remain its primary write surface
- `*WS` methods return `ErrNotSupported`
- the adapter documentation must state that its primary write path is chain-backed rather than REST-backed

This exception is preferable to lying about the transport semantics.

## Shared Helper And Documentation Changes

The repository should update:

- `exchange.go` comments to describe unsuffixed methods as non-WS writes and `*WS` methods as WS writes
- `README.md`
- `README_CN.md`
- examples that currently imply one write method can transparently choose transport
- any docs that currently recommend `OrderMode`

Convenience helpers such as `PlaceMarketOrder` and `PlaceLimitOrder` remain non-WS helpers because they call `PlaceOrder`.

If the repository later wants ergonomic WS helpers, it can add parallel helpers explicitly rather than reviving implicit transport switching.

## Testing Strategy

### Shared compile and contract migration

Update all stubs, mocks, and test doubles that satisfy `Exchange` or `PerpExchange` so they implement the new `*WS` methods.

### REST-path verification

Existing order-placement and lifecycle suites should continue to validate the unsuffixed write methods as the primary synchronous write path.

That means current suites such as:

- `RunOrderSuite`
- `RunLifecycleSuite`
- `RunOrderQuerySemanticsSuite`

become explicit verification of the non-WS write path unless a given test is extended intentionally.

### New WS-path verification

Add dedicated WS write tests that verify:

- `PlaceOrderWS` requires `ClientID`
- `PlaceOrderWS` does not return order data
- `PlaceOrderWS` does not fall back to REST when WS submission fails
- `CancelOrderWS` does not fall back to REST
- `ModifyOrderWS` does not fall back to REST
- `WatchOrders` can reconcile a WS-submitted order by `ClientOrderID`
- `LocalState.PlaceOrderWS` can backfill `OrderID` from later order updates
- unsupported WS write methods return `ErrNotSupported`

### Existing transport-switch tests

Current tests that prove explicit WS behavior without fallback, especially in `bitget`, should be retained but rewritten around the new explicit method names instead of `SetOrderMode`.

## Migration Risks

### Interface breakage

This is an intentional shared-interface breaking change.

That is acceptable because:

- the current contract is semantically ambiguous
- the repository already performed similar shared-interface migrations for order-query semantics

### Caller behavior changes

Some callers may currently depend on:

- `PlaceOrder` returning a placeholder order even when the adapter used WS
- `SetOrderMode` as a mutable runtime toggle

Those callers must migrate to:

- `PlaceOrderWS` plus `WatchOrders`
- or `LocalState.PlaceOrderWS`

### Exchange-specific client ID requirements

Requiring caller-supplied `ClientID` for `PlaceOrderWS` shifts some responsibility to the caller. That is intentional, but docs and adapter-specific tests must make the requirement obvious.

## Implementation Direction

Recommended rollout order:

1. Update shared interfaces in `exchange.go`.
2. Remove `OrderMode` from `BaseAdapter` and shared docs.
3. Update root helpers, mocks, and test doubles to compile with the new interfaces.
4. Add `LocalState.PlaceOrderWS`.
5. Migrate dual-transport adapters to explicit `*WS` methods.
6. Migrate REST-only adapters to return `ErrNotSupported` for `*WS`.
7. Keep Decibel as a documented exception for non-WS primary writes.
8. Add targeted WS write-path tests.
9. Re-run focused repository tests and selected adapter package tests.

## Open Questions Resolved

Should transport selection stay hidden behind one write method?

- No. The method boundary should express transport directly.

Should WS write methods return `*Order` with partial information?

- No. They should return only `error` and rely on `ClientID` plus `WatchOrders` for tracking.

Should `OrderMode` remain as a compatibility switch?

- No. Keeping it would preserve ambiguous dual control.

Should `ModifyOrder` follow the same split as placement and cancel?

- Yes. `ModifyOrderWS` should be added and should return only `error`.
