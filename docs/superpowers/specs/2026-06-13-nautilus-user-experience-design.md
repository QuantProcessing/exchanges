# Nautilus-Style User Experience Design

## Goal

Make the Go platform feel closer to NautilusTrader for strategy authors while
preserving the current platform/account/cache/adapter boundaries.

The immediate target is not to clone Nautilus' Python inheritance model. The
target is equivalent ergonomics in idiomatic Go: strategies subscribe from
`OnStart`, create orders through an order factory, receive typed callbacks, and
can be run through compact live/backtest entry points.

## Current Gaps

1. Strategy authors must handle raw `bus.Envelope` values instead of typed
   callbacks such as order-book, ticker, accepted-order, fill, account, and
   position handlers.
2. Market-data subscriptions are made outside the strategy through
   `platform.Node.SubscribeMarketData`; Nautilus strategies call subscription
   helpers inside `on_start`.
3. Order creation requires directly filling `model.SubmitOrder`, including
   account, type, time-in-force, quantity, price, and client order ID.
   Nautilus provides `order_factory.limit(...)` and similar helpers.
4. `live.Runner` requires callers to manually assemble `bus`, `cache`,
   `portfolio`, `risk`, `platform.Node`, clients, and strategies.
5. `backtest.Runner` can replay events and match orders, but the strategy API
   is still the same low-level runtime/envelope surface.
6. `platform.Node.SubscribeMarketData` currently requires instruments to be
   loaded, so a strategy calling subscribe during `OnStart` can race live node
   startup.

## Design

### Runtime Helpers

Extend `strategy.Runtime` with the operations strategy authors expect:

- `OrderFactory(accountID model.AccountID) *model.OrderFactory`
- `SubscribeMarketData(ctx, sub)`
- `UnsubscribeMarketData(ctx, sub)`
- `SubscribeTicker(ctx, instrumentID)`
- `UnsubscribeTicker(ctx, instrumentID)`
- `SubscribeOrderBookDepth(ctx, instrumentID, depth)`
- `UnsubscribeOrderBookDepth(ctx, instrumentID, depth)`

`platform.Node` and `backtest.runtime` will both implement these methods.
Live mode routes subscriptions to venue data clients. Backtest mode records the
subscription request as a no-op control surface because replay data is already
provided up front.

### Order Factory

Add `model.OrderFactory`, because order construction belongs with normalized
order models rather than with one runtime package.

The factory is account-scoped and provides:

- `Market(instrumentID, side, quantity, opts...) model.SubmitOrder`
- `Limit(instrumentID, side, quantity, price, opts...) model.SubmitOrder`

Options support client order IDs, time in force, post-only, reduce-only,
trigger price, trailing offset, and expire time. If no client order ID is
provided, the factory generates deterministic account-prefixed IDs.

### Typed Strategy Adapter

Keep the existing `strategy.Strategy` interface for compatibility, and add a
wrapper:

```go
strategy.NewTyped("id", impl)
```

The wrapped implementation may define only the callbacks it needs:

- `OnStart(context.Context, strategy.Runtime) error`
- `OnTicker(context.Context, model.Ticker) error`
- `OnOrderBook(context.Context, model.OrderBook) error`
- `OnOrderStatus(context.Context, model.OrderStatusReport) error`
- `OnOrderAccepted(context.Context, model.OrderStatusReport) error`
- `OnOrderCanceled(context.Context, model.OrderStatusReport) error`
- `OnOrderRejected(context.Context, model.OrderStatusReport) error`
- `OnOrderFilled(context.Context, model.FillReport) error`
- `OnPosition(context.Context, model.PositionStatusReport) error`
- `OnAccount(context.Context, model.AccountSnapshot) error`
- `OnStop(context.Context) error`

This gives Go users the same mental model as Nautilus typed methods without
forcing inheritance or reflection-heavy magic.

### Deferred Live Subscriptions

Allow `platform.Node.SubscribeMarketData` before instrument providers have
loaded. If the instrument is not yet in cache but a streaming data client for
the venue exists, store the request as pending. During `Node.Start`, after
instrument loading and client connect, apply pending subscriptions.

This allows a strategy to call `rt.SubscribeOrderBookDepth(...)` in `OnStart`
while preserving the existing live-runner behavior where strategies are started
before platform startup events are published.

### Live Convenience Entry

Add `live.TradingNode` as a compact assembly helper:

```go
node, err := live.NewTradingNode(live.NodeConfig{
    DataClients:      []venue.DataClient{data},
    ExecutionClients: []venue.ExecutionClient{exec},
    Strategies:       []strategy.Strategy{strategy.NewTyped("imbalance", impl)},
})
err = node.Start(ctx)
defer node.Stop(ctx)
```

It still exposes the underlying `platform.Node`, `bus`, and `cache` for advanced
users.

### Backtest Entry

Keep `backtest.NewRunner` for direct configuration, and add a Nautilus-like
facade:

```go
engine := backtest.NewEngine(backtest.EngineConfig{})
engine.AddStrategy(strategy.NewTyped("imbalance", impl))
engine.AddData(event)
result, err := engine.Run(ctx)
```

Both entry points must support the same typed strategy/runtime APIs. This lets
the same typed strategy shape run in live and backtest modes.

## Testing

Implementation must be test-first:

1. Strategy package test proving `NewTyped` dispatches typed market and
   execution callbacks and exposes runtime helpers.
2. Model package test proving `OrderFactory` creates valid market and limit
   orders with generated and explicit client order IDs.
3. Platform/live test proving `OnStart` subscriptions are deferred and then
   applied when the live node starts.
4. Backtest test proving the same typed strategy API submits an order and
   receives a fill through realistic matching.
5. Usage-comparison demo migrated to the typed API and verified through CLI and
   tests.

## Non-Goals

- Do not reimplement Nautilus' Python object hierarchy.
- Do not change SDK or exchange adapter protocol behavior.
- Do not add external dependencies.
- Do not remove the existing raw-envelope strategy interface.
