# TradingAccount + OrderFlow Design

## Summary

The current `LocalState` abstraction solves a real problem, but it does not present
the right mental model to users. Its name emphasizes implementation details
("local state") instead of the user-facing job to be done: managing an exchange
account runtime and tracking order lifecycles.

This design replaces the public `LocalState` concept with two user-facing
abstractions:

- `TradingAccount`: an account-scoped runtime session
- `OrderFlow`: a handle for one order's lifecycle updates

The goal is to make strategy code express account operations in business terms
without hiding exchange semantics or embedding strategy policy in the library.

## Problem

Many applications built on top of `exchanges` need to reimplement the same
private-account runtime behavior:

- initial account snapshot loading
- `WatchOrders` and `WatchPositions` registration
- local open-order, position, and balance caches
- matching order updates by `OrderID` and `ClientOrderID`
- recovering missing order states after websocket delays or drops
- exposing per-order update streams to strategy code

`LocalState` already covers part of this behavior, but it has three product
problems:

1. its name does not tell users when they should reach for it
2. its main helper (`OrderResult` + `WaitTerminal`) is biased toward one-shot
   waiting instead of lifecycle-driven strategy logic
3. it does not give users a clean per-order runtime object that they can plug
   into execution workflows such as partial-fill hedging, cancel-and-repost, or
   timeout handling

In practice this means applications such as `cross-exchanges-arb` still carry a
large amount of order-lifecycle plumbing in strategy code even though the
library already owns most of the raw synchronization machinery.

Downstream consumer migrations, including cross-exchanges-arb, are intentionally deferred until this repository has passed full shared testing and a new release tag is published.

## Goals

- provide a public abstraction whose name matches user intent
- make account-facing code read like business logic, not stream wiring
- expose one order's lifecycle as a stable flow of `*Order` updates
- keep strategy decisions in application code, not in the library
- preserve websocket-first behavior with REST/account reconciliation fallback
- reuse the proven `LocalState` synchronization core where possible

## Non-Goals

- do not absorb market-data order book management into this abstraction
- do not provide built-in strategy policies such as repost rules, hedge rules,
  or timeout tactics
- do not replace raw adapter APIs for users who want full manual control
- do not invent a second event taxonomy on top of `OrderStatus`

## Public Model

### TradingAccount

`TradingAccount` is the public account session object for one authenticated
exchange adapter.

Responsibilities:

- start and maintain the private-account runtime
- expose cached balance, positions, and open orders
- expose account-wide order and position subscriptions
- place, track, and cancel orders through account-aware helpers
- create `OrderFlow` handles for individual orders

It is intentionally account-scoped, not strategy-scoped.

### OrderFlow

`OrderFlow` is the public lifecycle handle for one order.

It does not interpret strategy intent. It only exposes the adapter-normalized
`*Order` updates for that order and maintains the latest known snapshot.

This keeps the library responsible for routing and reconciliation, while leaving
policy decisions such as hedge-on-partial-fill or repost-on-cancel in user code.

## Public API

The minimal public API should stay small and map directly to account-facing use
cases.

### TradingAccount lifecycle

```go
func NewTradingAccount(adp Exchange, logger Logger, opts ...TradingAccountOption) *TradingAccount
func (a *TradingAccount) Start(ctx context.Context) error
func (a *TradingAccount) Close()
```

### TradingAccount state queries

```go
func (a *TradingAccount) Balance() decimal.Decimal
func (a *TradingAccount) Position(symbol string) (*Position, bool)
func (a *TradingAccount) Positions() []Position
func (a *TradingAccount) OpenOrder(orderID string) (*Order, bool)
func (a *TradingAccount) OpenOrders() []Order
```

### TradingAccount subscriptions

```go
func (a *TradingAccount) SubscribeOrders() *Subscription[Order]
func (a *TradingAccount) SubscribePositions() *Subscription[Position]
```

### TradingAccount order actions

```go
func (a *TradingAccount) Place(ctx context.Context, params *OrderParams) (*OrderFlow, error)
func (a *TradingAccount) PlaceWS(ctx context.Context, params *OrderParams) (*OrderFlow, error)
func (a *TradingAccount) Cancel(ctx context.Context, orderID, symbol string) error
func (a *TradingAccount) CancelWS(ctx context.Context, orderID, symbol string) error
func (a *TradingAccount) Track(orderID, clientOrderID string) (*OrderFlow, error)
```

Notes:

- `Place` returns `*OrderFlow`, not a one-shot result object
- `Cancel` returns only an error; cancellation outcomes continue through the
  existing `OrderFlow`
- `PlaceWS` and `CancelWS` remain explicit transport helpers when the adapter
  supports them
- `Track` is recommended because users often need to attach to an existing order
  by `OrderID` or `ClientOrderID`

### OrderFlow API

```go
func (f *OrderFlow) C() <-chan *Order
func (f *OrderFlow) Latest() *Order
func (f *OrderFlow) Wait(ctx context.Context, predicate func(*Order) bool) (*Order, error)
func (f *OrderFlow) Close()
```

Notes:

- `C()` is the main path; it exposes adapter-normalized `*Order` updates
- `Latest()` returns the current known snapshot for polling or post-loop reads
- `Wait(...)` is a thin helper, not the primary abstraction
- users compute incremental fill deltas themselves from successive
  `FilledQuantity` values
- users use adapter timestamps directly if they want latency calculations

## Usage Shape

The intended experience should look like this:

```go
acct := exchanges.NewTradingAccount(adp, nil)
if err := acct.Start(ctx); err != nil {
    return err
}
defer acct.Close()

flow, err := acct.Place(ctx, &exchanges.OrderParams{
    Symbol:   "ETH",
    Side:     exchanges.OrderSideBuy,
    Type:     exchanges.OrderTypeLimit,
    Price:    price,
    Quantity: qty,
})
if err != nil {
    return err
}
defer flow.Close()

var lastFilled decimal.Decimal
for ord := range flow.C() {
    delta := ord.FilledQuantity.Sub(lastFilled)
    lastFilled = ord.FilledQuantity

    switch ord.Status {
    case exchanges.OrderStatusNew:
        // wait for fills
    case exchanges.OrderStatusPartiallyFilled:
        // hedge delta
    case exchanges.OrderStatusCancelled:
        // decide whether to repost
    case exchanges.OrderStatusFilled:
        // continue workflow
    }

    if delta.IsPositive() {
        // user-defined execution logic
    }
}
```

This usage keeps the library focused on synchronization and routing, while
keeping strategy behavior fully explicit in user code.

## Internal Design

The public model stays simple by separating three internal responsibilities.

### 1. Account state synchronizer

This is the existing `LocalState` core in spirit, regardless of whether the name
survives internally.

Responsibilities:

- `FetchAccount` bootstrap snapshot
- `WatchOrders` registration
- `WatchPositions` registration when supported
- local caches for balance, positions, and open orders
- periodic reconciliation refresh

This layer remains responsible for account-level truth maintenance.

### 2. Order flow registry

This internal component owns the routing table from global order updates to
individual `OrderFlow` instances.

Responsibilities:

- register flows created by `Place`, `PlaceWS`, or `Track`
- index by both `OrderID` and `ClientOrderID`
- update mappings when a flow starts with only `ClientOrderID` and later learns
  `OrderID`
- fan matching order updates into the correct flow
- retire flows when they are closed or reach terminal completion

### 3. OrderFlow state holder

Each `OrderFlow` owns only:

- the latest order snapshot
- the order-specific channel of `*Order` updates
- lightweight waiting helpers
- lifecycle cleanup

It does not run strategy logic and does not own account-wide caches.

## Reconciliation Behavior

`TradingAccount` should remain websocket-first, with reconciliation used as a
reliability tool instead of the primary data path.

Default behavior:

1. subscribe first
2. place or track the order
3. route matching websocket updates into the flow
4. if the flow is waiting for progress and websocket updates stall, attempt
   reconciliation with `FetchOrderByID`
5. when useful, refresh account-level caches from `FetchAccount`

The library should continue handling late `OrderID` discovery through
`ClientOrderID` matching so users do not need to write that plumbing
themselves.

## Relationship To Existing LocalState

`LocalState` is solving the right internal problem but presenting the wrong
public product shape.

Recommended direction:

- stop treating `LocalState` as the primary user-facing concept
- reuse its synchronization core behind `TradingAccount`
- replace `OrderResult` as the primary order helper with `OrderFlow`
- preserve compatibility temporarily if needed, but make the new public
  documentation and examples center on `TradingAccount`

This is a product-level rename and reshape, not a rejection of the existing
synchronization logic.

## Impact On cross-exchanges-arb

This design is a strong fit for `cross-exchanges-arb`, but only for the
execution-runtime half of the application.

Expected simplifications:

- remove manual `WatchOrders` setup from trader startup
- replace shared maker/taker order channels with per-order `OrderFlow`
- remove repeated `OrderID`/`ClientOrderID` matching logic from strategy code
- centralize websocket-first plus REST-reconciliation behavior in the library
- simplify open, hedge, and close confirmation paths
- read balance and position state from account runtime instead of repeated ad hoc
  fetch logic where appropriate

Expected non-simplifications:

- the arbitrage state machine still belongs in application code
- hedge, repost, timeout, and manual-intervention policies still belong in
  application code
- market-data order book handling should continue through the existing
  order-book pipeline instead of being folded into `TradingAccount`

The value proposition is not "abstract the whole strategy." The value
proposition is "remove reusable execution plumbing from the strategy."

Downstream consumer migrations, including cross-exchanges-arb, are intentionally deferred until this repository has passed full shared testing and a new release tag is published.

## Naming Guidance

Public names should describe the user's job, not the implementation:

- prefer `TradingAccount` over `LocalState`
- prefer `OrderFlow` over `OrderResult`

This makes the abstraction easier to discover and easier to explain:

- "start a trading account"
- "place an order and get an order flow"
- "track an existing order"

These phrases are easier for users to reason about than "create local state."

## Testing Strategy

Implementation should preserve and extend the existing coverage model.

Required coverage:

- unit tests for order-flow routing by `OrderID` and `ClientOrderID`
- unit tests for late `OrderID` backfill after placement
- unit tests for `Track(...)`
- unit tests for `OrderFlow.Wait(...)`
- unit tests for flow cleanup on close and terminal states
- integration coverage reusing the current local-state/shared-private-stream
  suite expectations
- one application-level proof point by adapting the key execution path in
  `cross-exchanges-arb`

The existing `LocalState` tests are not throwaway work. They should be migrated
or repointed so the proven synchronization guarantees remain intact.

## Migration Strategy

Recommended implementation order:

1. introduce `TradingAccount` on top of the current synchronization core
2. introduce `OrderFlow` and route placements to it
3. keep `LocalState` compatibility temporarily if needed
4. update README examples to use `TradingAccount`
5. validate the abstraction by simplifying `cross-exchanges-arb`
6. decide whether `LocalState` becomes deprecated or remains as a lower-level
   compatibility wrapper

This sequence keeps risk controlled while moving users toward the better public
model.

Downstream consumer migrations, including cross-exchanges-arb, are intentionally deferred until this repository has passed full shared testing and a new release tag is published.

## Decision

The library should evolve from a user-facing `LocalState` abstraction to a
user-facing `TradingAccount + OrderFlow` model.

`TradingAccount` should own account-runtime synchronization.
`OrderFlow` should expose one order's raw normalized updates.
Strategy code should continue owning execution policy.
