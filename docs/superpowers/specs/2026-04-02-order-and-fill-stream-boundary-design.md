# Order And Fill Stream Boundary Design

## Status

Proposed and approved for implementation.

## Problem

The current unified streaming contract exposes private order lifecycle updates only through `WatchOrders(ctx, cb)`, where the callback receives a single `Order` model.

That shape creates two related issues:

- the shared `Order` model only has one `Price` field, so adapter implementations cannot represent order price, last fill price, and average fill price without conflating them
- some exchanges expose order updates and fill updates as separate streams, while others include partial fill summaries in the order stream; forcing everything into one callback makes semantics inconsistent across adapters

This is not primarily a WebSocket limitation. The main limitation is the current unified repository model.

## Goals

- Define a clear public boundary between order lifecycle updates and execution updates.
- Keep the common path easy for SDK users who only need order status tracking.
- Preserve a unified `WatchOrders` contract for order state transitions.
- Introduce a separate execution-oriented stream for users who need real trade prices, per-fill fees, and maker/taker details.
- Avoid adapter-specific hidden fusion logic between order and fill feeds.
- Reduce ambiguity in the meaning of `Order.Price`.

## Non-Goals

- Do not force every user to subscribe to both orders and fills.
- Do not require adapters to merge fills back into order updates.
- Do not introduce a catch-all private event stream in this phase.
- Do not redesign public market trade streams such as `WatchTrades`.

## Confirmed Product Decisions

These decisions were explicitly confirmed during discussion and are fixed for this design:

- `WatchOrders` and `WatchFills` have distinct responsibilities
- adapter implementations should not be required to fuse order streams and fill streams together before invoking callbacks
- SDK users may subscribe to only `WatchOrders` for lifecycle tracking and add `WatchFills` only when they need execution detail

## Design Principles

1. One stream should represent one business concept.
2. Order lifecycle and execution detail are related but not identical.
3. The default path should stay simple for common SDK usage.
4. Richer execution detail should be available without distorting order semantics.
5. When exchange data is incomplete, adapters should surface that limitation honestly rather than synthesize a misleading value.

## Public API Design

### Stream responsibilities

`WatchOrders`

- Represents order lifecycle state.
- Emits when an order is accepted, opened, partially filled, fully filled, canceled, expired, or rejected.
- The payload is order-centric and may include aggregate fill information known at that moment.
- The payload is not required to represent each individual execution separately.

`WatchFills`

- Represents one execution event.
- Emits once per fill or per exchange-native fill aggregation unit.
- The payload is execution-centric and should preserve the actual execution price and execution quantity for that event.
- This stream is the source of truth for slippage analysis, VWAP reconstruction, execution fee tracking, and maker/taker attribution.

### Why both streams exist

An order has one lifecycle but may have many fills. A single order update can summarize current progress, but it cannot losslessly represent multiple executions at different prices. That one-to-many relationship is the reason this design separates the streams.

## Shared Model Changes

### Order model

The `Order` model should stop using a single ambiguous `Price` field as the sole price carrier.

Recommended order-facing fields:

- `OrderPrice`
- `AverageFillPrice`
- `LastFillPrice`
- `LastFillQuantity`
- `FilledQuantity`

Compatibility note:

- the legacy `Price` field may remain during migration, but its meaning must be documented as deprecated and ambiguous
- new code should read `OrderPrice` for the submitted price and use fill-specific fields for execution summaries

### Fill model

Add a dedicated `Fill` model for `WatchFills`.

Minimum recommended fields:

- `TradeID`
- `OrderID`
- `ClientOrderID`
- `Symbol`
- `Side`
- `Price`
- `Quantity`
- `Fee`
- `FeeAsset`
- `IsMaker`
- `Timestamp`

Optional fields when available:

- `LiquidityRole`
- `IsTaker`
- `QuoteQuantity`
- `RealizedPnL`

## Adapter Semantics

### WatchOrders requirements

Adapters should map exchange-native order updates into repository order states without pretending to provide execution granularity they do not actually have.

Examples:

- if an exchange order stream exposes order price and average fill price, the adapter should map both into order-facing aggregate fields
- if an exchange order stream exposes only order price plus cumulative filled quantity, the adapter should emit only those values
- if an exchange order stream does not expose fill price at all, the adapter should leave fill-price summary fields empty rather than invent one from a separate fill stream

### WatchFills requirements

Adapters should map the exchange-native fill stream or trade-history push stream into `Fill` events whenever the exchange supports such data.

If an exchange does not support private fill streaming, the adapter may return `ErrNotSupported` for `WatchFills`.

### No hidden fusion

Adapters should not be required to join order and fill streams internally in order to make `WatchOrders` appear richer than the exchange-native order stream really is.

This avoids:

- ordering races between order and fill callbacks
- adapter-local caches that attempt to reconstruct execution state heuristically
- inconsistent synthesized semantics across exchanges

## User Guidance

### Use only `WatchOrders` when:

- you need order status updates
- you need to know whether an order is open, partially filled, filled, or canceled
- you are building a simple trading bot state machine
- you are showing order lifecycle in UI

### Add `WatchFills` when:

- you need true execution prices
- you need per-fill fees
- you need maker/taker attribution
- you need execution-level analytics such as slippage, realized PnL, or VWAP
- you need to reconcile one order against multiple fills

### Combined usage

Users who subscribe to both streams should join them by `OrderID` and `ClientOrderID` when available. The repository should document that the two streams are complementary, not duplicates:

- `WatchOrders` answers "what is the current state of my order?"
- `WatchFills` answers "what executions actually happened?"

## Exchange-Specific Reality

The repository should expect three patterns:

1. Exchanges where order updates already include useful aggregate fill fields.
2. Exchanges where order updates and fills are separate streams.
3. Exchanges where private fill streaming is missing or limited.

This design supports all three without pretending they are identical.

## Backward Compatibility

Recommended migration path:

1. Add the new fields and `Fill` model.
2. Add `WatchFills` to streaming interfaces.
3. Update adapters incrementally.
4. Mark legacy `Order.Price` as deprecated in comments and documentation.
5. Keep old behavior working long enough for downstream callers to migrate.

## Testing

Implementation should add tests for:

- `Order` field mapping where order price and average fill price are both available
- `WatchOrders` behavior when fill prices are absent
- `WatchFills` mapping for exchanges with native private fill streams
- adapters returning `ErrNotSupported` when fills are unavailable
- repository docs/examples that show when one stream or both streams should be used

## Open Questions Resolved

Should adapters hide exchange differences by merging order and fill streams internally?

- No. The repository should expose the difference honestly and keep the contract explicit.

Will users need both streams for every use case?

- No. `WatchOrders` remains the default stream. `WatchFills` is only needed for execution-level detail.

Will this make the API harder to use?

- Slightly for advanced users, but significantly clearer overall because each callback now has one stable meaning.
