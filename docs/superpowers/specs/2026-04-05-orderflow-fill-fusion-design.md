# OrderFlow Fill-Fusion Design

## Status

Proposed and approved in discussion, pending written-spec review.

## Summary

`TradingAccount` currently routes only `WatchOrders` into `OrderFlow`.

That keeps lifecycle tracking simple, but it leaves a product gap: partial-fill
and full-fill snapshots exposed through `OrderFlow.C()` cannot reliably include
the actual execution detail that strategies usually need at the moment of
fill-driven state transitions.

This design keeps the repository's raw stream boundary intact:

- `WatchOrders` remains the raw order-overview stream
- `WatchFills` remains the raw execution-detail stream

but upgrades `OrderFlow` into a per-order fused view:

- `OrderFlow.C()` emits merged `*Order` snapshots
- `OrderFlow.Fills()` emits raw `*Fill` execution events for the same order

The fused snapshot is intentionally richer than raw `WatchOrders`, while the
repository-level stream semantics stay explicit and honest.

## Problem

The repository already distinguishes private order-overview updates from private
execution updates:

- `WatchOrders` answers "what is the current order lifecycle state?"
- `WatchFills` answers "what executions actually happened?"

That boundary is correct at the adapter level, but `TradingAccount` currently
does not use `WatchFills` to enrich `OrderFlow`.

As a result:

- `OrderFlow.C()` can tell users that an order is partially filled or filled
- but it cannot reliably tell them the latest execution price or size at the
  same moment
- users who want a single per-order control flow still have to manually join
  two different subscriptions outside the runtime

The desired product experience is different:

- users should still be able to consume raw fills when they want execution-level
  detail
- but the main per-order control loop should be able to switch on merged
  `Order` snapshots and directly read fields such as:
  - `FilledQuantity`
  - `LastFillQuantity`
  - `LastFillPrice`
  - `AverageFillPrice`

## Goals

- keep `WatchOrders` and `WatchFills` as distinct raw adapter-level concepts
- upgrade `OrderFlow` into a fused per-order view built from both streams
- let `OrderFlow.C()` emit fill-enriched `*Order` snapshots
- add `OrderFlow.Fills()` so users can still consume raw `*Fill` events for the
  same order
- ensure that `PARTIALLY_FILLED` and `FILLED` snapshots exposed through
  `OrderFlow.C()` are backed by real fill data
- keep the implementation lightweight by limiting fusion state to one order flow
  at a time instead of introducing a second heavy account-wide state machine

## Non-Goals

- do not change the raw meaning of `WatchOrders`
- do not change the raw meaning of `WatchFills`
- do not synthesize fake fills from order updates when an exchange cannot expose
  private executions natively
- do not add a new repository-wide private event taxonomy
- do not require account-wide fill subscriptions to become part of the public
  `TradingAccount` API in this phase
- do not add `LatestFill()` or `WaitFill(...)` unless later usage proves they
  are necessary

## Confirmed Product Decisions

These decisions were explicitly confirmed during discussion and are fixed for
this design:

- `OrderFlow` should expose two streams:
  - `C()` for merged order snapshots
  - `Fills()` for raw fill detail
- fill arrival should be treated as a valid reason to publish a new merged order
  snapshot through `OrderFlow.C()`, even when the raw order status has not
  changed
- `OrderFlow` may derive `PARTIALLY_FILLED` and `FILLED` from fills before the
  raw order stream catches up
- `CANCELLED`, `REJECTED`, and similar non-fill terminal states remain driven by
  the raw order stream
- raw `order.FILLED` is a confirmation signal, not the direct source of the
  merged `FILLED` snapshot exposed to users
- if an adapter does not support `WatchFills`, `TradingAccount` should degrade
  honestly to the current order-only `OrderFlow` behavior instead of synthesizing
  executions

## Considered Approaches

### Option 1: Keep order and fill streams separate all the way to the user

Under this model:

- `OrderFlow.C()` would continue to reflect only `WatchOrders`
- `OrderFlow.Fills()` would expose raw fills
- users would still join both streams themselves

This is simple, but it fails the main product goal because the primary per-order
control loop still cannot switch on one enriched order snapshot.

### Option 2: Stateless fill patching

Under this model:

- a fill arrival would be applied to the latest known order snapshot
- no per-order cumulative fill state would be retained

This was rejected because correct merged snapshots require at least minimal
state:

- cumulative fill quantity
- VWAP or average fill price
- latest fill detail
- duplicate-fill suppression

Without that state, `OrderFlow.C()` would be inconsistent and fragile.

### Option 3: Per-order minimal fusion state

Under this model:

- raw `WatchOrders` and `WatchFills` stay separate
- `TradingAccount` routes both into `OrderFlow`
- each `OrderFlow` keeps only the minimal state needed to produce a merged
  snapshot for that one order

This is the chosen approach.

It gives users the desired fused view without creating a second heavy account
runtime.

## Public API

`TradingAccount` public placement, tracking, and account-state APIs stay
unchanged in this phase.

`OrderFlow` grows one new method:

```go
func (f *OrderFlow) C() <-chan *Order
func (f *OrderFlow) Fills() <-chan *Fill
func (f *OrderFlow) Latest() *Order
func (f *OrderFlow) Wait(ctx context.Context, predicate func(*Order) bool) (*Order, error)
func (f *OrderFlow) Close()
```

Semantic notes:

- `C()` is no longer a raw order-overview mirror; it is the merged per-order
  view
- `Fills()` exposes the raw normalized execution events routed to that flow
- `Latest()` returns the latest merged order snapshot
- `Wait(...)` remains order-snapshot oriented and observes the merged order
  stream, not the raw fill stream

No `LatestFill()` or `WaitFill(...)` method is introduced in this phase.

`TradingAccount.SubscribeOrders()` should keep its current raw order-overview
semantics in this phase. The new fusion behavior is scoped to `OrderFlow`.

## Public Semantics

### Raw stream meanings stay unchanged

This design does not redefine repository-level raw streams.

They remain:

- `WatchOrders`: raw order-overview lifecycle stream
- `WatchFills`: raw execution-detail stream

The new behavior is specifically a property of `OrderFlow`, not a rewrite of the
adapter-level contracts.

### `OrderFlow.C()` becomes a fused per-order view

`OrderFlow.C()` should emit merged `*Order` snapshots that combine:

- lifecycle fields from raw order updates
- execution-detail fields derived from raw fills

The resulting snapshot is intended to be directly usable in strategy control
flow.

If a user receives:

- `PARTIALLY_FILLED`
- `FILLED`

from `OrderFlow.C()`, they should be able to inspect real fill-derived values
such as:

- `FilledQuantity`
- `LastFillQuantity`
- `LastFillPrice`
- `AverageFillPrice`

### `OrderFlow.Fills()` remains the raw execution stream

`OrderFlow.Fills()` should expose the same normalized `*Fill` events that the
runtime receives from `WatchFills`, routed per order.

This gives users two complementary views of the same order:

- the merged control-flow snapshot
- the raw execution-detail stream

## Source Of Truth Rules

### Raw order stream owns lifecycle and non-fill metadata

The raw order stream remains the source of truth for:

- `OrderID`
- `ClientOrderID`
- `Symbol`
- `Side`
- `Type`
- `OrderPrice`
- `Quantity`
- `PENDING`
- `NEW`
- `CANCELLED`
- `REJECTED`
- other future non-fill terminal or non-fill lifecycle states

### Raw fill stream owns execution detail

The raw fill stream is the source of truth for:

- `LastFillQuantity`
- `LastFillPrice`
- `AverageFillPrice`
- fill-driven progression toward `PARTIALLY_FILLED`
- fill-driven progression toward `FILLED`

### Merged `FilledQuantity`

`FilledQuantity` in the merged snapshot should be:

```text
max(raw order filled quantity, cumulative fill quantity)
```

This keeps the merged snapshot monotonic even when fills arrive before the raw
order stream catches up.

## Status Derivation Rules

### Fill-driven states

If at least one fill has been observed and the flow is not already in a raw
non-fill terminal state:

- derive `PARTIALLY_FILLED` when cumulative fill quantity is positive but has
  not yet reached known order quantity
- derive `FILLED` when cumulative fill quantity has reached or exceeded known
  order quantity

### Raw `order.FILLED`

Raw `order.FILLED` should not immediately produce a merged `FILLED` snapshot.

Instead it acts as a confirmation flag:

- record that the raw order stream says the order is filled
- keep waiting for fill data to drive the merged terminal snapshot

This avoids emitting `FILLED` to users without actual fill-derived execution
detail.

If known order quantity is still unavailable when the raw `order.FILLED` arrives,
the next fill-driven recomputation may move the merged snapshot to `FILLED`
based on the confirmation flag even without a complete quantity field.

### Raw non-fill terminals

The raw order stream remains the only source that may directly move the merged
snapshot into:

- `CANCELLED`
- `REJECTED`
- other future non-fill terminal states such as `EXPIRED`

Fill data must not synthesize those states.

## Publish Rules

### On raw fill arrival

When a matching fill arrives:

1. update the flow's cumulative fill state
2. emit the raw `*Fill` through `OrderFlow.Fills()`
3. recompute the merged `*Order` snapshot
4. emit the recomputed merged snapshot through `OrderFlow.C()`

This publish should happen even if the raw order status has not changed.

### On raw order arrival

When a matching raw order update arrives:

1. update the flow's base order snapshot
2. recompute the merged order using current cumulative fill state
3. emit the recomputed merged snapshot through `OrderFlow.C()`

No attempt is made to wait for a paired order/fill bundle. The first stream to
arrive may immediately drive output.

## Internal Design

### TradingAccount subscriptions

`TradingAccount.Start()` should subscribe to:

- `WatchOrders`
- `WatchFills` when supported
- `WatchPositions` when supported

Account-wide balance, position, and open-order caches remain order-oriented.

This change does not require the public `TradingAccount` surface to expose an
account-wide `SubscribeFills()` API.

### Registry routing

`orderFlowRegistry` should route two event types:

- raw orders
- raw fills

Recommended internal shape:

```go
func (r *orderFlowRegistry) RouteOrder(update *exchanges.Order)
func (r *orderFlowRegistry) RouteFill(fill *exchanges.Fill)
```

Routing still keys off:

- `OrderID`
- `ClientOrderID`

### Pending-fill buffering

The registry should keep a small best-effort pending-fill buffer for fills that
arrive before the flow learns the identifier needed for routing.

Typical example:

- a WS placement creates a flow indexed by `ClientOrderID`
- an early fill arrives carrying only `OrderID`
- the later raw order update binds `OrderID` to the existing flow
- the pending fills are then replayed into that flow in arrival order

This buffer should be bounded and cleared on account close. It exists to smooth
early cross-stream races, not to become a durable replay system.

Recommended bound:

- keep at most 32 pending fills per unresolved routing key
- drop the oldest pending fill when the cap is exceeded

### Per-flow serialized fusion

Each `OrderFlow` should process routed raw order and raw fill events through a
single serialized inbox.

The goal is to avoid merge races between concurrent callbacks while keeping the
state local to one order.

Recommended minimal internal state per flow:

- latest raw/base order snapshot
- latest merged order snapshot
- cumulative fill quantity
- cumulative fill quote value
- latest fill snapshot
- raw `order.FILLED` confirmation flag
- duplicate-fill tracking set

## Duplicate Fill Handling

`OrderFlow` should suppress duplicate raw fills before they affect cumulative
state or public output.

Preferred key:

- `TradeID`

Fallback when `TradeID` is absent or unstable:

- a deterministic fingerprint built from fields such as `OrderID`,
  `ClientOrderID`, `Timestamp`, `Price`, `Quantity`, and `Fee`

## Unsupported `WatchFills`

If the adapter returns `ErrNotSupported` for `WatchFills`:

- `TradingAccount.Start()` should continue successfully
- `OrderFlow.C()` should degrade to the current order-only behavior
- `OrderFlow.Fills()` should produce no events
- no synthetic fills should be created from order updates

This preserves existing compatibility while keeping the capability boundary
honest.

## Track Behavior

`Track(orderID, clientOrderID)` may create a flow before a full order snapshot is
available.

That flow should still be able to receive and publish fills first.

In that case the merged snapshot may initially contain only:

- routing identifiers
- fill-derived quantity and price fields
- a fill-driven status such as `PARTIALLY_FILLED`

When a later raw order snapshot arrives, the flow should enrich the merged view
with missing metadata such as:

- `Quantity`
- `OrderPrice`
- `Side`
- `Type`
- non-fill lifecycle states

## Flow Termination

The flow should become terminal when the merged per-order view becomes terminal.

That means:

- `CANCELLED` and `REJECTED` terminate immediately when confirmed by raw order
  updates
- `FILLED` terminates when the merged snapshot reaches `FILLED` through fill
  data

As a consequence, a raw `order.FILLED` may arrive before the flow is considered
terminal if the necessary fill detail has not yet been observed.

This is an intentional trade-off: the runtime prefers a truthful final merged
snapshot over a faster but incomplete terminal transition.

## Testing Requirements

Implementation should add coverage for at least:

- fill arrives before raw order progress update
- raw order `FILLED` arrives before the last fill
- raw order `CANCELLED` after one or more fills
- duplicate fills do not double-count quantity or quote value
- `Track(...)` with only one identifier available initially
- pending-fill replay once the routing identifier becomes known
- `WatchFills` unsupported path degrades without synthetic execution data
- `Wait(...)` sees fill-driven merged snapshots

## Open Questions Resolved

Should `OrderFlow` expose one combined event stream instead of separate order
and fill streams?

- No. Two streams are still clearer:
  - `C()` for the merged order view
  - `Fills()` for raw execution detail

Should raw `order.FILLED` be treated as the direct source of the merged
`FILLED` snapshot?

- No. It is a confirmation signal only. The merged `FILLED` snapshot should be
  backed by fill data.

Should `TradingAccount` fail to start when `WatchFills` is unsupported?

- No. It should degrade honestly to order-only behavior rather than breaking
  existing adapter compatibility.
