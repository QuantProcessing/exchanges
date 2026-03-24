# Fetch Order Semantics Refactor Design

## Goal

Refactor the shared order-query contract so the repository uses three explicit methods:

- `FetchOrderByID`
- `FetchOrders`
- `FetchOpenOrders`

The aim is to remove the current ambiguity around `FetchOrder`, make terminal-order lookup explicit, and let adapters expose full order-list behavior without overloading open-order queries.

## Scope

In scope:

- change the shared `Exchange` interface in `exchange.go`
- replace `FetchOrder` with `FetchOrderByID` across adapters and call sites
- add `FetchOrders(ctx, symbol string) ([]Order, error)` to the shared interface
- update adapters to implement the new contract
- update shared tests and any direct call sites that rely on order-query behavior
- add or adjust regression coverage for terminal-order lookup and order-list semantics

Out of scope:

- adding pagination, `since`, `limit`, or status filters to `FetchOrders`
- changing order placement, cancellation, or WebSocket stream contracts
- introducing adapter-only history-order helpers outside the shared interface

## Problem Statement

Today the repository exposes:

- `FetchOrder(ctx, orderID, symbol string)`
- `FetchOpenOrders(ctx, symbol string)`

This creates two problems:

1. `FetchOrder` sounds generic but is often implemented as a thin wrapper over open-order queries, which loses terminal orders.
2. There is no explicit shared surface for "all visible orders for a symbol", which encourages adapters to blur single-order lookup and open-order listing.

The new API should make these responsibilities explicit:

- one method for single-order lookup by ID
- one method for all visible orders for a symbol
- one method for open orders only

## Shared Interface Design

The `Exchange` trading section will become:

```go
PlaceOrder(ctx context.Context, params *OrderParams) (*Order, error)
CancelOrder(ctx context.Context, orderID, symbol string) error
CancelAllOrders(ctx context.Context, symbol string) error
FetchOrderByID(ctx context.Context, orderID, symbol string) (*Order, error)
FetchOrders(ctx context.Context, symbol string) ([]Order, error)
FetchOpenOrders(ctx context.Context, symbol string) ([]Order, error)
```

### Method semantics

#### `FetchOrderByID`

- single-order lookup by order ID
- the adapter should return the order whenever the exchange still provides enough data to resolve it
- terminal orders are part of the intended contract
- if the order cannot be found, return `exchanges.ErrOrderNotFound`
- if the exchange cannot support terminal lookup at all with any low-level path, return `exchanges.ErrNotSupported` rather than pretending the order does not exist

#### `FetchOrders`

- returns the exchange's visible orders for the requested symbol
- minimal version only takes `symbol string`
- does not imply pagination or filters yet
- should include terminal and open orders when the exchange exposes them
- if the exchange has no low-level path beyond open orders, return `exchanges.ErrNotSupported` rather than collapsing this into `FetchOpenOrders`

#### `FetchOpenOrders`

- returns only currently open orders for the requested symbol
- remains separate from `FetchOrders`

## Symbol Semantics

Keep `symbol string` on all three methods.

Reasons:

- many exchanges naturally scope order-history queries by symbol
- it keeps the repository's base-symbol convention intact
- it avoids needing a second refactor now for optional filters

Rules:

- adapter callers continue to pass base symbols such as `"BTC"`
- adapters must convert through `FormatSymbol`
- if an exchange only exposes account-wide order lists, the adapter must filter to the requested symbol before returning

## Adapter Implementation Rules

### `FetchOrderByID`

Preferred implementation order:

1. dedicated order-detail endpoint
2. symbol-scoped order-history endpoint
3. broader authenticated order list that includes terminal states

Do not treat `FetchOpenOrders` scanning as a full implementation.

If an exchange truly has no low-level path that can resolve terminal orders:

- do not fake full support
- return `exchanges.ErrNotSupported`
- do not use open-order scanning as a "looks good enough" substitute for terminal lookup

### `FetchOrders`

Preferred implementation order:

1. symbol-scoped order-history endpoint
2. broader authenticated order list filtered by symbol

Rules:

- return all visible orders for the symbol that the exchange exposes
- do not silently collapse this into open orders only
- if the exchange only exposes open orders, return `exchanges.ErrNotSupported`

### `FetchOpenOrders`

- continue to return only open orders
- if the low-level API returns account-wide open orders, filter by symbol

## Migration Strategy

This will be a deliberate breaking change.

Repository changes required:

1. Update `exchange.go` to rename `FetchOrder` to `FetchOrderByID` and add `FetchOrders`.
2. Update every adapter implementation.
3. Update all internal call sites in adapters that currently call `FetchOrder`.
4. Update any shared helpers or tests that use the old name.
5. Update README snippets or docs only where they describe the shared order-query interface.
6. Review modify/replace flows that currently call `FetchOrder` to recover live order state before amendment.
7. Include `binance/margin_adapter.go` in the migration even though it is not registry-wired like the main spot/perp adapters.

There will be no compatibility shim in the shared interface. The point is to make the semantic split explicit immediately.

## Testing Strategy

### Shared behavior to verify

Add or update tests so the repository verifies:

- `FetchOrderByID` can retrieve a just-filled or just-cancelled order when the adapter claims terminal lookup support
- `FetchOrderByID` returns `exchanges.ErrNotSupported` when the adapter honestly lacks terminal lookup support
- `FetchOrders` includes more than the current open set when the adapter claims broader order-list support
- `FetchOrders` returns `exchanges.ErrNotSupported` when the adapter only has open-order visibility
- `FetchOpenOrders` excludes terminal orders
- `errors.Is(err, exchanges.ErrOrderNotFound)` works for real missing-order lookup

### Test layers

#### Deterministic tests

Where adapters or SDKs have deterministic mapping/helpers, add focused unit tests for:

- `ErrOrderNotFound` propagation
- symbol filtering behavior
- order-list mapping if there is adapter-local normalization logic

#### Live shared-suite tests

The shared `testsuite` should add a dedicated suite, for example `RunOrderQuerySemanticsSuite`, that exercises the semantic split.

Its config should let each adapter declare the capabilities being claimed, for example:

- `SupportsTerminalLookup`
- `SupportsOrderHistory`

The suite should:

- place an order
- observe terminal state
- query `FetchOrderByID`
- query `FetchOrders`
- query `FetchOpenOrders`
- assert the semantic differences or the expected `ErrNotSupported` paths

Every trading-capable adapter test file should wire this suite explicitly, just like the existing shared suites.

Binance margin is the exception for live verification scope in this refactor:

- it still needs the interface migration and semantic implementation updates
- it must compile against the new shared interface
- it should receive deterministic coverage where practical
- it does not need new live shared-suite wiring in this refactor unless the repository first establishes a margin-specific live test path

This can extend `testsuite/order_suite.go` or add a dedicated shared suite if that keeps the existing order suite from becoming unclear.

## Testsuite Design Direction

Preferred direction:

- keep existing order-placement coverage in `testsuite/order_suite.go`
- add a focused companion suite for order-query semantics rather than overloading the existing order suite

The test should not assume every exchange has infinite history. It should only enforce the repository contract that the adapter honestly represents what the exchange can resolve.

## Adapter Risk Areas

Adapters most likely to need careful changes:

- Backpack and GRVT, because they currently implement single-order lookup by scanning open orders
- adapters with internal call sites that use `FetchOrder` as part of modify/cancel flows
- spot/perp pairs where one side has stronger history support than the other
- Binance margin, because it still implements the shared order-query surface outside the main registry-wired spot/perp matrix and therefore needs migration without the normal live-suite path

## Success Criteria

This refactor is successful when:

- the shared interface exposes `FetchOrderByID`, `FetchOrders`, and `FetchOpenOrders`
- all adapters compile against the new interface
- current internal call sites are migrated to the new method names
- shared tests verify the intended semantic split and the explicit `ErrNotSupported` paths
- no adapter still relies on the old `FetchOrder` name
