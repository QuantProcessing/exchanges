# Reliable Quant Program Guide

This guide is the practical checklist for writing a trading program with this
repository. It focuses on reliability rather than signal quality.

## 1. Keep Strategy Logic Venue-Neutral

Write strategies against normalized model types:

- `model.InstrumentID` instead of raw symbols;
- `model.OrderBook`, `model.Ticker`, `model.TradeTick`, and `model.Bar`
  instead of exchange payloads;
- `model.SubmitOrder` and `model.OrderList` instead of SDK request structs;
- `model.OrderStatusReport`, `model.FillReport`, and
  `model.PositionStatusReport` for execution state.

Venue-specific symbol parsing, signing, endpoint options, and account modes
belong in SDK or adapter code.

## 2. Store Runtime, Not Shadow Infrastructure

Use the runtime passed into `OnStart`:

```go
func (s *Strategy) OnStart(ctx context.Context, rt strategy.Runtime) error {
    s.runtime = rt
    return rt.SubscribeOrderBookDepth(ctx, s.instrumentID, 5)
}
```

Avoid creating independent caches, portfolios, order factories, or venue
clients inside a strategy. Independent state drifts from the platform state and
is hard to reconcile after reconnects.

## 3. Create Orders Through `OrderFactory`

`OrderFactory` keeps account identity, client order IDs, list IDs, parent IDs,
reduce-only flags, and command metadata consistent.

```go
order := rt.OrderFactory(accountID).Limit(
    instrumentID,
    model.OrderSideBuy,
    decimal.RequireFromString("0.25"),
    decimal.RequireFromString("100.50"),
    model.WithPostOnly(),
)
```

For bracket behavior, prefer the purpose-built helper:

```go
list := rt.OrderFactory(accountID).Bracket(model.BracketOrderRequest{
    InstrumentID: instrumentID,
    Side:         model.OrderSideBuy,
    Quantity:     decimal.RequireFromString("1"),
    EntryPrice:   decimal.RequireFromString("100"),
    TakeProfit:   decimal.RequireFromString("104"),
    StopLoss:     decimal.RequireFromString("98"),
})
_, err := rt.SubmitOrderList(ctx, list)
```

## 4. Put Safety In `risk.Engine`

Do not bury safety checks inside individual strategies. Configure risk at the
runtime boundary so every strategy and command path gets the same treatment.

```go
riskEngine := risk.NewEngine(c, risk.Config{
    MaxOrderNotional:     decimal.RequireFromString("1000"),
    MaxPositionNotional:  decimal.RequireFromString("2500"),
    MaxAccountExposure:   decimal.RequireFromString("5000"),
    ExposureCurrency:     "USDT",
    MaxOpenOrders:        20,
    MaxCommandsPerWindow: 10,
    CommandRateWindow:    time.Second,
})
```

Treat risk rejection as a normal lifecycle outcome. A strategy should be able
to observe a rejection and stop submitting repeated invalid commands.

## 5. Query Cache And Portfolio For State

Use cache for lifecycle state:

```go
order, ok := rt.Cache().OrderByClientID(accountID, clientOrderID)
fills := rt.Cache().FillsForOrder(accountID, order.OrderID)
position, ok := rt.Cache().PositionByInstrument(accountID, instrumentID)
```

Use portfolio for accounting state:

```go
exposure := rt.Portfolio().Exposure(accountID, "USDT")
realized := rt.Portfolio().RealizedPnLs(accountID)
unrealized := rt.Portfolio().UnrealizedPnLs(accountID)
```

If a strategy maintains its own signal state, keep it small and rebuildable.
Orders, fills, positions, balances, and exposure belong to the runtime.

## 6. Backtest The Lifecycle, Not Only The Signal

A useful backtest asserts more than final profit. It should also check:

- expected subscriptions were made;
- order IDs and command metadata survived;
- risk accepted or rejected as expected;
- fills were applied once;
- final position is expected;
- portfolio realized and unrealized PnL are expected;
- result output is deterministic.

Run focused tests while developing:

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./backtest ./examples/... -v
```

## 7. Check Adapter Capabilities Before Going Live

Do not assume every venue supports every lifecycle feature. Check:

- `adapter.Capabilities()` at runtime;
- [Adapter Capabilities](./adapter-capabilities.md);
- [Adapter Capability Matrix](../parity/adapter-capability-matrix.md).

If your program requires private streams, resubscribe, order query, fill
reports, position reports, mass status, or order lists, fail fast when the
adapter does not claim that capability.

## 8. Treat Reconciliation As A Required Runtime Feature

Private streams can disconnect. Venues can emit fills before order reports.
Startup can miss events. External venue activity can happen outside the
program.

Reliable programs should:

- inspect node and client health;
- surface unresolved discrepancies;
- make reconciliation idempotent;
- never silently ignore local/venue mismatches;
- include reconciliation behavior in tests and release notes.

## 9. Use Live Write Tests Deliberately

Public read tests can run by default. Private read tests need credentials.
Tests that place, cancel, modify, transfer, or otherwise mutate venue state
must be opt-in with explicit environment flags.

Read [Adapter Live Test Policy](../parity/adapter-live-test-policy.md) before
adding or running live-write coverage.

## 10. Production Rollout Checklist

Before a real deployment, verify:

- strategy has deterministic tests for its signal and lifecycle behavior;
- risk limits are configured outside the strategy;
- adapter capabilities match the strategy's required features;
- live node health is monitored;
- reconnect and reconciliation paths are exercised;
- every credential is loaded from environment or secret storage, never code;
- mutating tests are opt-in and disabled by default;
- `git diff --check` and relevant Go tests pass.
