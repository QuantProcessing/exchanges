# Order Semantics

This reference covers only the current shared order-query surfaces from [`exchange.go`](/home/xiguajun/Documents/GitHub/Exchanges/.worktrees/skill-adding-exchange-adapters/exchange.go):

- `FetchOrder(ctx, orderID, symbol string) (*Order, error)`
- `FetchOpenOrders(ctx, symbol string) ([]Order, error)`

Do not invent adapter-level history-order APIs, closed-order APIs, or other order-query helpers that are not on the shared interface today.

## `FetchOrder` Contract

`FetchOrder` is single-order lookup by ID within the current shared interface. It is not "find an open order if convenient".

Required behavior:

- accept the shared base symbol, not the exchange-native symbol
- return the matching unified `exchanges.Order` when the exchange can still resolve that order
- support terminal-order lookup when the exchange provides enough low-level data to do so
- return [`exchanges.ErrOrderNotFound`](/home/xiguajun/Documents/GitHub/Exchanges/.worktrees/skill-adding-exchange-adapters/errors.go) when the order truly cannot be found

If the exchange has a dedicated order-detail endpoint, use it first. That is the clean path.

## `FetchOpenOrders` Contract

`FetchOpenOrders` returns the current open orders for the requested symbol.

Required behavior:

- apply symbol filtering to the requested base symbol
- return only open orders
- return unified `exchanges.Order` values, not wire types

If the exchange only exposes "all open orders", the adapter must filter the result to the requested symbol before returning it.

## Symbol Filtering Expectations

The shared boundary uses base symbols such as `"BTC"`.

Rules:

- convert the requested base symbol through `FormatSymbol` before calling low-level APIs
- if the exchange endpoint already filters by symbol, still ensure the mapped result is consistent
- if the endpoint returns all open orders, filter after mapping or during mapping so callers only receive the requested symbol
- if `symbol == ""` is meaningfully supported by the low-level API for "all symbols", document that choice in the adapter; otherwise follow the nearest peer behavior and keep filtering explicit

Do not mix exchange-native symbols like `BTCUSDT` or `BTC-USD-SWAP` into the adapter contract.

## Acceptable `FetchOrder` Fallbacks

If the exchange lacks a direct order-detail endpoint, acceptable fallbacks are limited:

1. Query a broader authenticated order list that includes terminal states, then scan for the target ID.
2. Query a symbol-scoped user-order history endpoint, then scan for the target ID.
3. Query a low-level "all orders for symbol" surface, then map the matching record.

Last-resort fallback:

- if the exchange truly exposes no historical or terminal order lookup at all, do not pretend full `FetchOrder` semantics exist
- document the limitation in code comments and return `exchanges.ErrOrderNotFound` when the order is no longer open

Open-order scanning alone is a degraded limitation path, not adapter-complete behavior and not the preferred implementation.

## `ErrOrderNotFound`

When lookup is exhausted and no matching order exists, return `exchanges.ErrOrderNotFound`.

Do this:

- `return nil, exchanges.ErrOrderNotFound`
- or wrap it with exchange context so `errors.Is(err, exchanges.ErrOrderNotFound)` still works

Do not do this:

- `fmt.Errorf("order not found")`
- exchange-specific string errors that drop the sentinel

## Anti-Patterns

Avoid these:

- implementing `FetchOrder` by scanning only `FetchOpenOrders` and treating that as complete behavior
- returning an open-order result for the wrong symbol because the adapter skipped filtering
- adding adapter-only "fetch order history" methods outside the shared interface
- returning `nil, nil` when an order is missing
- using exchange-native symbols at the adapter boundary

Current repo contract: `FetchOrder` and `FetchOpenOrders` are separate semantics. Preserve that distinction.
