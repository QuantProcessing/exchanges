# WatchOrders Overview-Only Design

## Status

Proposed and approved for implementation.

## Problem

The repository already separates private order lifecycle updates (`WatchOrders`) from private execution updates (`WatchFills`), but the runtime behavior is still uneven across adapters.

Two inconsistencies matter:

- Some adapters populate execution-oriented fields such as `AverageFillPrice`, `LastFillPrice`, and `LastFillQuantity` in `WatchOrders`, while others cannot.
- Some adapters subscribe to the same native private order topic more than once when the caller enables both `WatchOrders` and `WatchFills`, even though the native payload can drive both callbacks.

This makes streaming behavior feel different across exchanges even when the public interfaces are identical.

## Goals

- Make `WatchOrders` consistently represent an order-overview stream.
- Make `WatchFills` the only execution-detail stream.
- Ensure one native private topic is subscribed at most once per adapter instance.
- Support fan-out from one subscribed native topic into multiple public callbacks when the exchange payload contains both order-lifecycle and execution information.
- Preserve a clean user mental model:
  - `WatchOrders` answers "what is the current state of my order?"
  - `WatchFills` answers "what executions actually happened?"

## Non-Goals

- Do not force adapters with separate native order and fill topics to merge them.
- Do not remove execution-oriented fields from the shared `Order` struct in this phase.
- Do not redesign REST order-fetching semantics.
- Do not change public `WatchFills` payload shape.

## Confirmed Product Decisions

These decisions were explicitly confirmed during discussion and are fixed for this design:

- `WatchOrders` should expose only order-overview semantics.
- `OrderPrice`, `Quantity`, `FilledQuantity`, `Status`, `OrderID`, `ClientOrderID`, and `Timestamp` remain valid `WatchOrders` fields.
- `AverageFillPrice`, `LastFillPrice`, and `LastFillQuantity` should not be relied on in `WatchOrders`.
- Users who need execution price or per-fill quantity must subscribe to `WatchFills`.
- If one native private topic can power both public callbacks, the adapter should subscribe to that topic only once and fan out internally.

## Public Semantics

### WatchOrders

`WatchOrders` is an order-overview stream.

It should be used for:

- order acceptance
- open / new state
- partial fill state
- filled state
- cancel / expire / reject state
- aggregate filled quantity progress

It should not be treated as an execution-detail stream.

For streaming semantics, adapters should populate:

- `OrderID`
- `ClientOrderID`
- `Symbol`
- `Side`
- `Type` when known from the native order stream
- `OrderPrice`
- `Quantity`
- `FilledQuantity`
- `Status`
- `Timestamp`

For streaming semantics, adapters should not promise:

- `AverageFillPrice`
- `LastFillPrice`
- `LastFillQuantity`

Those fields may continue to exist on `Order` for compatibility and non-streaming usage, but `WatchOrders` should not rely on them or document them as part of the stable lifecycle contract.

### WatchFills

`WatchFills` remains the execution-detail stream.

It is the source of truth for:

- execution price
- execution quantity
- fee and fee asset
- maker/taker attribution
- one-order-to-many-fills reconciliation

If a strategy needs exact execution details, it must subscribe to `WatchFills`.

## Adapter Behavior Rules

### Rule 1: One native topic, one subscription

If `WatchOrders` and `WatchFills` are both driven by the same native private topic or native event family, the adapter must subscribe to that native topic only once per adapter instance.

The adapter should then fan out internally:

- order-lifecycle projection to `OrderUpdateCallback`
- execution projection to `FillCallback`

This avoids duplicate topic subscriptions and keeps callback behavior consistent.

### Rule 2: Multi-callback fan-out is supported

Adapters should support one subscribed native topic dispatching into multiple public callback channels.

The adapter should maintain callback registration state so that:

- enabling `WatchOrders` first and `WatchFills` later does not create a duplicate native subscription
- disabling one callback stream does not tear down the native topic while the other public stream is still active
- the native topic is unsubscribed only when no public consumers remain

### Rule 3: Separate native topics stay separate

If an exchange exposes distinct native topics for order lifecycle and executions, the adapter should continue subscribing to them separately.

This design does not require hidden fusion for exchanges such as:

- Bitget
- Decibel
- EdgeX
- GRVT
- Hyperliquid
- Lighter
- Nado
- StandX

### Rule 4: Partial-fill state remains on WatchOrders

`WatchOrders` is still the place where partial-fill state is expressed.

Adapters should continue to emit `OrderStatusPartiallyFilled` when they can determine:

- `0 < FilledQuantity < Quantity`

This remains true even when the exchange-native status string does not literally use the phrase `PARTIALLY_FILLED`.

## Exchange Pattern Classification

Current repository behavior falls into two broad classes:

### Same native order topic can feed both public streams

These adapters should use single-subscription fan-out:

- Binance
- Aster
- OKX
- Backpack

### Native order and native fill topics are already separate

These adapters should keep separate native subscriptions:

- Bitget
- Decibel
- EdgeX
- GRVT
- Hyperliquid
- Lighter
- Nado
- StandX

Important clarification:

- sharing one authenticated WebSocket connection is not the same as sharing one native topic
- this design cares about duplicate subscriptions to the same native order topic, not about the number of TCP/WebSocket connections

## Compatibility Strategy

The shared `Order` model can keep `AverageFillPrice`, `LastFillPrice`, and `LastFillQuantity` for backward compatibility.

However:

- `WatchOrders` documentation should stop presenting them as part of the stable streaming contract
- adapters should avoid populating them in `WatchOrders` when aligning to the overview-only model
- downstream users should treat `WatchFills` as the supported source of execution detail

This keeps the public structs compatible while making stream semantics consistent.

## Documentation Changes

Repository docs should explicitly say:

- `WatchOrders` is an order-overview stream
- `WatchFills` is an execution-detail stream
- if an order is canceled, rely on `WatchOrders`
- if an order is partially filled and then canceled, use `WatchOrders` for lifecycle state and `WatchFills` for the actual executions
- some exchanges use one native order topic for both public streams, but the adapter hides that behind one subscription and internal fan-out

## Testing Requirements

Implementation should add or update tests for:

- adapters with shared native order topics subscribe once even when both `WatchOrders` and `WatchFills` are enabled
- stopping one public stream does not unsubscribe the shared native topic while the other stream is still active
- adapters with shared native topics still emit both order and fill callbacks from one native message
- `WatchOrders` no longer relies on `AverageFillPrice`, `LastFillPrice`, or `LastFillQuantity` as part of the documented streaming contract
- docs/examples describe `WatchOrders` as overview-only and `WatchFills` as execution-only

## Implementation Direction

Recommended rollout order:

1. Update documentation and spec language first.
2. Add shared or per-adapter tests for single-subscription fan-out behavior.
3. Refactor shared-topic adapters:
   - Binance
   - Aster
   - OKX
   - Backpack
4. Normalize `WatchOrders` mappings so they focus on order-overview fields.
5. Re-run live verification for at least one shared-topic exchange and one separate-topic exchange.

## Open Questions Resolved

Should `WatchOrders` keep surfacing recent execution detail when some exchanges can provide it?

- No. For consistent behavior, `WatchOrders` should remain an order-overview stream and `WatchFills` should carry execution detail.

Should adapters subscribe twice when one native topic can drive both public streams?

- No. One native topic should be subscribed once and fan out internally to multiple public callbacks.
