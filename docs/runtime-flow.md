# Runtime Flow

This document explains how modules collaborate at runtime. It is the map to
read when you know what each package does but want to understand how data moves
through the system.

## Market Data Flow

```text
venue.StreamingDataClient or venue.DataClient
        |
        v
data.Engine
        |
        +--> cache.Cache stores instruments and latest market events
        |
        +--> bus.Bus publishes model.MarketEvent on strategy.TopicMarketData
        |
        v
strategy.Engine dispatches typed callbacks
```

1. A live node registers data clients with `data.Engine`.
2. `data.Engine` loads instruments through each client's `InstrumentProvider`.
3. Strategy code calls `SubscribeTicker`, `SubscribeOrderBookDepth`,
   `SubscribeTradeTicks`, `SubscribeQuoteTicks`, or `SubscribeBars`.
4. The platform routes the subscription to the right streaming data client.
5. Incoming venue events are mapped to `model.MarketEvent`.
6. Cache stores the event, bus publishes it, and `strategy.NewTyped` dispatches
   `OnTicker`, `OnOrderBook`, `OnTradeTick`, `OnQuoteTick`, or `OnBar`.

## Order Flow

```text
strategy callback
        |
        v
strategy.Runtime.SubmitOrder or SubmitOrderList
        |
        v
platform.Node adds metadata and runs risk.Engine
        |
        v
execution.Engine and venue.ExecutionClient
        |
        v
adapter maps model command to SDK request
        |
        v
venue API or simulated backtest matching
        |
        v
model.ExecutionEvent
        |
        +--> cache.Cache
        +--> portfolio.Portfolio
        +--> bus.Bus
        +--> strategy callbacks
```

The strategy never has to know whether a command is handled by a live adapter or
by the backtest matching path. The command shape is still the same
`model.SubmitOrder`.

The important safety property is that risk runs before execution in normal
runtime paths. A rejected order should publish a visible lifecycle result and
must not create downstream order, fill, position, or portfolio mutation.

## Order List And Bracket Flow

`model.OrderFactory.Bracket` creates one parent entry order plus two exit
children. All three orders share an `OrderListID`; children carry the parent
client order ID and reduce-only exit flags.

```text
strategy creates model.OrderList
        |
        v
SubmitOrderList
        |
        +--> parent is submitted
        +--> children are indexed and held
        |
        v
parent fill event
        |
        +--> children released
        |
        v
take-profit or stop-loss fill
        |
        +--> sibling is canceled
```

The same order-list metadata is used by live and backtest paths.

## Fill And Portfolio Flow

```text
venue fill report or simulated fill
        |
        v
model.FillReport
        |
        +--> account.Reconciler deduplicates and repairs local state
        +--> cache.Cache indexes the fill
        +--> portfolio.Portfolio updates balances, positions, fees, PnL
        +--> bus.Bus publishes execution event
        +--> strategy.OnOrderFilled receives the callback
```

Fills are identity-sensitive. Trade IDs, account IDs, instrument IDs, order IDs,
client order IDs, and venue order IDs should be preserved whenever the venue
provides them. Reconciliation must not apply the same fill twice.

## Reconciliation Flow

```text
startup, reconnect, periodic audit, or explicit request
        |
        v
execution/account reconciliation
        |
        v
venue reports: account, orders, fills, positions, mass status
        |
        v
cache and portfolio repair
        |
        v
audit trail records resolved and unresolved discrepancies
```

Reconciliation is how the runtime converges after private stream disconnects,
startup gaps, delayed reports, fill-before-order events, external venue
activity, and stale local position state.

Unresolved discrepancies are structured state with kind, identity, reason, and
timestamps. They should be visible in health, tests, and release notes.

## Backtest Flow

```text
[]backtest.Event
        |
        v
backtest.Runner
        |
        +--> advances deterministic clock
        +--> dispatches timers
        +--> records market events
        +--> matches open orders
        +--> dispatches strategy callbacks
        +--> drains same-timestamp commands
        |
        v
backtest.Result
```

Backtests are deterministic when the input events, timestamps, fill model, and
strategy code are deterministic.

## Live Flow

```text
live.Node.Start
        |
        +--> create/default bus, cache, risk, portfolio
        +--> start platform
        +--> load instruments
        +--> connect data clients
        +--> connect execution clients
        +--> query account state
        +--> start strategies
        +--> monitor health
```

`live.Node.Stop` stops strategies before disconnecting clients so strategy
`OnStop` callbacks can still inspect runtime state.
