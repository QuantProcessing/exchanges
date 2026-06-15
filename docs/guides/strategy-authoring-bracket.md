# Strategy Authoring With Brackets

This project follows the NautilusTrader authoring shape while using Go
composition instead of Python inheritance. A strategy receives a
`strategy.Runtime`, subscribes to data in `OnStart`, reacts through typed
callbacks, creates normalized orders with `OrderFactory`, and submits them
through the runtime so risk, execution, cache, and portfolio remain on the
same lifecycle path.

## Runnable Example

The bracket example lives in `examples/nautilus_style`:

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./examples/nautilus_style -v
```

`examples/nautilus_style/bracket_strategy.go` demonstrates the target flow:

- `OnStart` calls `SubscribeOrderBookDepth`.
- `OnOrderBook` waits for the entry price to be touched.
- `runtime.OrderFactory(accountID).Bracket(...)` builds an entry, stop-loss,
  and take-profit order list with shared metadata.
- `SubmitOrderList` routes the complete bracket into platform execution.
- `OnOrderStatus`, `OnOrderLifecycle`, and `OnOrderFilled` collect normalized
  execution events for later assertions.

## Bracket Semantics

`OrderFactory.Bracket` creates one parent entry order and two held child orders.
The platform and execution manager keep the list indexed by `OrderListID`.
When the entry fills, held children become actionable. When one OCO child fills,
the sibling is cancelled. The same order-list model is also used by backtests
so live and simulated behavior share the same command shape.

## Authoring Contract

Strategy code should not call exchange SDKs directly. Keep strategies expressed
in terms of:

- `model.InstrumentID`, `model.AccountID`, and normalized order types;
- `strategy.Runtime` subscriptions and submission methods;
- `model.OrderFactory` helpers for order creation;
- typed callbacks such as `OnOrderBook`, `OnOrderStatus`, and `OnOrderFilled`.

This mirrors NautilusTrader's strategy UX while preserving Go's explicit
interfaces and compile-time callback checks.
