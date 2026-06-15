# Project Architecture

This project is a layered Go trading platform. It is not a flat exchange
wrapper. The repository separates venue-native protocol clients, normalized
venue adapters, and a strategy/runtime layer that owns data, execution, risk,
portfolio, cache, reconciliation, backtest, and live operation.

## Design Goals

- Keep exchange protocol detail in `sdk/` packages.
- Keep cross-venue contracts in `venue` and `adapter`.
- Let strategies use normalized `model` types instead of venue payloads.
- Route trading commands through risk, execution, cache, portfolio, and event
  publication.
- Make backtest and live mode use the same strategy authoring surface.
- Expose unsupported behavior explicitly instead of returning no-op success.
- Make local state repair inspectable through reconciliation results and audit
  trails.

## Layer Map

```text
Application code
        |
        v
strategy.Strategy implementations
        |
        v
strategy.Runtime
        |
        +--> backtest.Runner for deterministic simulation
        |
        +--> live.Node for live operation
                 |
                 v
             platform.Node
                 |
                 +--> data.Engine -----> venue.DataClient -----> adapter/* -----> sdk/*
                 +--> execution.Engine -> venue.ExecutionClient -> adapter/* -----> sdk/*
                 +--> account.Reconciler
                 +--> risk.Engine
                 +--> portfolio.Portfolio
                 +--> cache.Cache
                 +--> bus.Bus
                 +--> kernel health and lifecycle helpers
```

The dependency direction is one-way:

1. runtime packages depend on `venue` abstractions;
2. adapters depend on SDKs and map venue-native payloads into `model`;
3. SDKs talk to exchange protocols and do not import adapter or runtime
   packages.

## Boundary Rules

### SDK Boundary

`sdk/` packages own official REST/WebSocket endpoint names, signing, request
and response types, listen-key flows, product-specific options, and exchange
protocol details.

Use SDKs for direct exchange API access. Do not put strategy lifecycle logic,
cache mutation, risk checks, portfolio accounting, or adapter capability claims
inside SDK packages.

### Adapter Boundary

`adapter/` packages expose SDK behavior through `venue` interfaces. They own
instrument lookup, raw symbol mapping, normalized market data, order validation,
common error mapping, private execution streams where implemented, and
`venue.DeclaredCapabilities`.

Adapters should use optional interfaces for optional features. If a capability
is not implemented, callers should receive `model.ErrNotSupported` or an error
that wraps it.

### Runtime Boundary

Runtime packages own trading behavior: strategy callbacks, command metadata,
risk checks, execution lifecycle, reconciliation, cache indexes, portfolio
accounting, backtesting, live node health, timers, and event fanout.

Strategies should interact with runtime APIs, not SDKs.

## Core Runtime Modules

### `model`

`model` is the shared type system. It defines identifiers, instruments, data
events, account snapshots, positions, orders, command metadata, order lists,
execution reports, lifecycle events, and errors.

The most important invariant is identity preservation. Account IDs, strategy
IDs, command IDs, correlation IDs, client order IDs, venue order IDs, position
IDs, and order-list IDs should survive strategy, risk, execution, adapter,
report, cache, portfolio, reconciliation, and callback paths.

### `venue`

`venue` is the stable interface layer. Runtime code talks to `venue.DataClient`,
`venue.StreamingDataClient`, and `venue.ExecutionClient`. Optional capability
interfaces describe features such as modify, query, order lists, fill reports,
position reports, mass status, and resubscribe.

### `cache`

`cache.Cache` is the runtime's local state index. It stores and indexes
instruments, account snapshots, order reports, fill reports, positions, market
events, deferred fills, residuals, and snapshots.

Execution, risk, portfolio, strategy, backtest, and reconciliation should query
cache instead of scanning unrelated state.

### `data`

`data.Engine` registers data clients, loads instruments, tracks subscriptions,
forwards market data into cache and bus, supports aggregation and catalog-backed
requests, and reports data health.

### `execution`

`execution.Engine` and `execution.Manager` route commands, manage order state,
handle order lists, release contingent children, emulate trigger behavior,
produce normalized execution events, and support reconciliation.

### `account`

`account.TradingAccount`, `account.OrderTracker`, and `account.Reconciler`
coordinate startup readiness, private stream events, account/order/fill/position
reports, delayed reports, external venue activity, and unresolved discrepancy
state.

### `risk`

`risk.Engine` protects the execution boundary. It checks validity, precision,
notional, exposure, scoped limits, reduce-only constraints, trading state,
kill switch, throttles, duplicate client IDs, and queue capacity where
configured.

### `portfolio`

`portfolio.Portfolio` consumes account, order, fill, position, lifecycle, and
market data events. It updates balances, positions, commissions, realized PnL,
unrealized PnL, marks, conversion rates, exposure, and analyzer records.

### `strategy`

`strategy` is the user-facing authoring layer. Strategies receive
`strategy.Runtime`, subscribe in `OnStart`, react to typed callbacks, create
orders with `model.OrderFactory`, and submit commands through the runtime.

### `backtest`

`backtest.Runner` replays timestamped events through the strategy runtime. It
controls deterministic time, timer dispatch, order expiration, same-timestamp
command draining, matching, fill modeling, slippage, latency, and result
summaries.

### `live`

`live.Node` wraps `platform.Node` with start/stop lifecycle, reconnect policy,
health monitoring, data client registration, execution client registration, and
strategy startup.

### `platform`

`platform.Node` is the live runtime facade and implements `strategy.Runtime`.
It wires data, execution, risk, portfolio, cache, bus, timers, subscriptions,
and reconciliation into one command and event path.

### `testsuite`

`testsuite` contains cross-package contract tests and scorecards. Use
package-local tests for narrow behavior and testsuite cases when a claim crosses
module boundaries.

## Runtime Flows

### Market Data Flow

```text
venue.DataClient / venue.StreamingDataClient
        |
        v
data.Engine
        |
        +--> cache.Cache stores instruments and market events
        |
        +--> bus.Bus publishes model.MarketEvent on market.data
        |
        v
strategy.Engine dispatches typed callbacks
```

Snapshot requests call `FetchTicker` or `FetchOrderBook`. Streaming
subscriptions call `SubscribeMarketData`, normalize venue events into
`model.MarketEvent`, update cache, run aggregators, and publish to strategy
callbacks.

### Strategy Order Flow

```text
strategy callback
        |
        v
strategy.Runtime.SubmitOrder / SubmitOrderList
        |
        v
platform.Node fills command metadata and checks risk
        |
        v
execution.Engine sends command to venue.ExecutionClient
        |
        v
adapter maps model command to SDK request
        |
        v
venue API or simulated execution path
        |
        v
order/fill/position reports return as model.ExecutionEvent
        |
        +--> cache.Cache
        +--> portfolio.Portfolio
        +--> bus.Bus -> strategy callbacks
```

The invariant is that strategy code does not call SDKs directly. Commands go
through the runtime so risk, cache, portfolio, lifecycle, and metadata stay
consistent.

### Bracket Order Flow

`model.OrderFactory.Bracket` creates one parent entry order and two reduce-only
exit children. The parent carries OTO metadata; the children carry OCO metadata
and share the same `OrderListID`.

```text
SubmitOrderList
        |
        +--> submit parent
        +--> hold children by parent client order ID
        |
        v
parent fill
        |
        +--> release take-profit and stop-loss children
        |
        v
one child fills
        |
        +--> cancel the sibling
```

### Reconciliation Flow

```text
startup, reconnect, periodic audit, or explicit request
        |
        v
account.Reconciler / execution reconciliation
        |
        v
venue.ExecutionClient reports:
  - account snapshot
  - order status reports
  - optional fill reports
  - optional position reports
  - optional mass status
        |
        v
cache and portfolio repair
        |
        v
audit trail records resolved and unresolved discrepancies
```

Unresolved discrepancies are structured state. Missing fills must be applied
once. Fill-before-order reports should be deferred and replayed when the order
appears. External orders should be imported only under an explicit policy.

## Backtest And Live Equivalence

Backtest and live share `strategy.Runtime`, normalized model types, order
factory helpers, typed callbacks, cache queries, portfolio queries, risk
checks, lifecycle events, and command metadata.

The source of truth changes:

- backtest uses deterministic events and simulated matching;
- live uses registered venue data and execution clients;
- both should make state transitions and failures visible.

Use backtests to prove strategy behavior. Use live-node and adapter tests to
prove wiring, health, reconnect, stream, and venue capability behavior.

## Failure And Unsupported Behavior

Failure should be explicit:

- unsupported adapter surfaces return `model.ErrNotSupported` or a wrapped
  equivalent;
- risk rejection returns typed behavior and should publish a visible lifecycle
  event in runtime paths;
- reconciliation discrepancies are recorded in audit state;
- health snapshots expose stale streams, queue pressure, and last errors;
- live write tests are opt-in and must not run by default.

Silent success is a bug when a caller asked for lifecycle behavior the system
does not support.

## Verification

Run the smallest command that proves your claim:

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./strategy ./backtest ./live ./platform
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./account ./risk ./portfolio ./testsuite
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./venue ./adapter/... ./config/all ./testsuite -run 'Adapter|Capability|Contract|PrivateStream|Resubscribe'
git diff --check
```
