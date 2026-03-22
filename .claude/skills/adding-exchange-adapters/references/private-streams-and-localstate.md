# Private Streams And LocalState

[`LocalState`](/home/xiguajun/Documents/GitHub/Exchanges/.worktrees/skill-adding-exchange-adapters/local_state.go) is built on adapter behavior. It is not a separate adapter interface.

The practical inputs are:

- `FetchAccount`
- `WatchOrders`
- optionally `WatchPositions`

## Hard Readiness Rules

- `WatchOrders` is mandatory for LocalState readiness.
- `WatchOrders` is also mandatory for any real lifecycle claim.
- `WatchPositions` is additive coverage, not the universal gate.
- unsupported shared stream surfaces must return [`exchanges.ErrNotSupported`](/home/xiguajun/Documents/GitHub/Exchanges/.worktrees/skill-adding-exchange-adapters/errors.go), not no-op success.

`LocalState.Start` does this today:

1. fetches a REST account snapshot
2. subscribes to `WatchOrders`
3. tries `WatchPositions`
4. starts periodic refresh

Because step 2 is mandatory in code, `WatchOrders` failure is fatal for LocalState.

## `WatchOrders`

Treat `WatchOrders` as the required private stream for unified order tracking.

It must provide real order updates that let higher layers observe:

- new order acknowledgement
- fills and partial fills
- cancellation
- rejection or other terminal states

If the exchange cannot provide a usable private order stream, do not claim lifecycle-capable or local-state-capable support.

## `WatchPositions`

`WatchPositions` improves position freshness, especially for perp adapters, but current LocalState does not require it to start successfully.

Use it when the exchange offers a real private position stream.

If the exchange does not offer one:

- return `exchanges.ErrNotSupported`
- do not fake success
- rely on `FetchAccount` snapshots and periodic refresh for baseline position visibility

Spot adapters may legitimately not support `WatchPositions`. Some perp exchanges may also lack a reliable position stream.

## REST Snapshot Versus WS Delta Responsibilities

The common repository pattern is:

1. REST snapshot establishes initial balances, positions, and currently open orders.
2. Private WS subscriptions supply deltas after startup.
3. Adapter maps those deltas into unified models.
4. LocalState or higher-level callbacks consume the mapped updates.

Keep responsibilities clear:

- `FetchAccount` owns the initial coherent snapshot
- `WatchOrders` owns ongoing order lifecycle deltas
- `WatchPositions` owns ongoing position deltas when supported
- LocalState owns caching, fan-out, and periodic reconciliation

Do not push LocalState-specific cache logic down into the adapter to compensate for missing private streams.

## Unsupported Surfaces

When a shared private stream is truly unsupported, return `exchanges.ErrNotSupported`.

Examples:

- spot adapter without position streaming: `WatchPositions` returns `exchanges.ErrNotSupported`
- public-data-only adapter: `WatchOrders` and `WatchPositions` both return `exchanges.ErrNotSupported`

Do not:

- return `nil` while never subscribing
- start a goroutine that never emits updates just to satisfy the method signature
- claim LocalState support when `WatchOrders` is absent

## Anti-Patterns

Avoid these:

- treating `WatchPositions` as required everywhere when current LocalState only treats it as additive
- treating LocalState as if it were a separate adapter interface that needs its own package contract
- using only REST polling while advertising `WatchOrders` success
- skipping `FetchAccount` snapshot and expecting private deltas to reconstruct state from nothing

Definition of done for local-state readiness: real `FetchAccount`, real `WatchOrders`, and honest `ErrNotSupported` handling for any shared private stream the exchange cannot provide.
