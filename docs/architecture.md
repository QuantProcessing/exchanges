# Project Architecture

This document describes how the repository is structured and how the major
modules cooperate to form a Go NautilusTrader-style trading platform.

The project is intentionally not a flat exchange wrapper. It has three
boundaries:

1. `sdk/` packages speak venue-native REST and WebSocket protocols.
2. `adapter/` and `venue/` packages expose stable, capability-honest exchange
   interfaces.
3. Runtime packages implement strategy, data, execution, risk, portfolio,
   backtest, live-node, cache, and reconciliation behavior above adapters.

## Design Goals

- Keep venue-specific protocol detail in SDK packages.
- Keep adapter interfaces small, explicit, and test-backed.
- Let strategy code target normalized model types rather than exchange payloads.
- Make backtest and live paths share command, event, risk, cache, portfolio,
  and lifecycle semantics.
- Treat `testsuite` and `docs/parity` as release-facing evidence, not as
  optional documentation.
- Return explicit unsupported errors instead of silent no-op success.

## Layer Map

```text
Strategy / Application
        |
        v
live.Node / platform.Node
        |
        +--> strategy.Engine ----> strategy.Runtime
        |
        +--> data.Engine --------> venue.DataClient --------> adapter/* --------> sdk/*
        |
        +--> risk.Engine
        |
        +--> execution.Engine ---> venue.ExecutionClient ---> adapter/* --------> sdk/*
        |
        +--> portfolio.Portfolio
        |
        +--> cache.Cache
        |
        +--> bus.Bus / kernel.Component
```

## Core Packages

### `model`

`model` is the shared vocabulary for the runtime.

It owns identifiers, instruments, account snapshots, orders, order lists,
commands, command metadata, market data, execution reports, lifecycle events,
and validation errors.

Important design rules:

- Strategy, execution, risk, portfolio, cache, and adapters should communicate
  through `model` types.
- Identifiers such as `AccountID`, `InstrumentID`, `ClientOrderID`,
  `VenueOrderID`, `OrderListID`, `PositionID`, `CommandID`, and
  `CorrelationID` must survive the full command and event path.
- Decimal trading values use `shopspring/decimal`; avoid float math in trading
  state.
- Advanced order semantics such as brackets, OTO, OCO, OUO, reduce-only,
  triggers, trailing offsets, order lists, and command metadata belong here.

### `venue`

`venue` defines the stable interfaces runtime code can depend on.

Key interfaces:

- `Adapter`: owns venue identity and exposes instruments, data, execution, and
  declared capabilities.
- `InstrumentProvider`: loads and lists normalized instruments.
- `DataClient`: fetches snapshot market data.
- `StreamingDataClient`: subscribes to live market-data events.
- `ExecutionClient`: connects account execution, queries account state, submits
  and cancels orders, generates order reports, and exposes private execution
  events.
- Optional execution interfaces: modify, query, order lists, fill reports,
  position reports, mass status, and resubscribe.

`venue.DeclaredCapabilities` is a promise. A true field must match implemented
behavior and shared contract tests.

### `sdk`

`sdk/` packages are venue-native protocol clients. They can expose endpoint
names, venue-specific request/response payloads, signing rules, account modes,
and product-specific details.

SDK packages should not depend on adapter or runtime packages. They are the
lowest layer.

### `adapter`

`adapter/` packages convert SDK behavior into stable venue interfaces.

Adapters are responsible for:

- symbol and instrument resolution;
- exchange-native to normalized `model` mapping;
- precision and order validation;
- common error mapping;
- snapshot and streaming market data;
- private execution streams;
- honest capability declarations.

Adapters should not grow the core runtime interface for a feature that only one
venue supports. Use optional interfaces and return `model.ErrNotSupported` for
unsupported behavior.

### `cache`

`cache.Cache` is the local runtime source of truth.

It indexes:

- instruments;
- account snapshots and account history;
- orders by account, order ID, client ID, venue order ID, strategy, position,
  order list, execution spawn, and open/closed state;
- fills by account, order, trade, and venue order;
- deferred fills for fill-before-order reconciliation;
- positions by account, instrument, position ID, strategy, venue position ID,
  and open/closed state;
- market snapshots and market events;
- residual and snapshot state for reconciliation and diagnostics.

Execution, risk, portfolio, strategy, backtest, and reconciliation should query
cache instead of scanning unrelated state.

### `bus`

`bus.Bus` is the runtime event fanout mechanism. It carries `bus.Envelope`
values on topics such as:

- `market.data`;
- `execution`;
- `timer`;
- `error`.

Strategy engines subscribe to topics and dispatch events to strategy instances.
Platform and data/execution components publish normalized events into the bus.

### `kernel`

`kernel` contains reusable infrastructure:

- component lifecycle states;
- health snapshots;
- clocks;
- message-bus primitives.

Runtime components should expose health and lifecycle state instead of hiding
reconnect, queue, or stream failures.

### `data`

`data.Engine` normalizes market data.

It owns:

- data-client registration;
- instrument loading into cache;
- live subscriptions;
- subscription replay after restart;
- market-event forwarding into cache and bus;
- bar aggregation;
- historical/catalog requests;
- health and stale-client reporting.

`data.Catalog` and `ReplayCatalog` make historical and replay data explicit.
`MemoryCatalog` is the simple in-memory implementation used by examples and
tests.

### `execution`

`execution.Engine` and `execution.Manager` own order lifecycle behavior.

Responsibilities include:

- submit, modify, cancel, query, cancel-all, and batch-cancel commands;
- order-list handling;
- contingent order release;
- emulated stop, touched, and trailing order behavior;
- normalized order, fill, position, and lifecycle events;
- matching-core behavior used by backtests;
- health reporting;
- reconciliation support.

Execution should not bypass cache, risk, portfolio, or command metadata.

### `account`

`account.TradingAccount` is the account-level lifecycle runtime.

It coordinates:

- account startup readiness;
- private execution stream connection;
- startup and gap reconciliation;
- normalized order/fill reports;
- per-order tracking through `OrderTracker`;
- stream health, unsupported stream state, event counters, and slow-subscriber
  accounting.

`account.Reconciler` turns venue reports into local lifecycle repairs and
structured unresolved discrepancies.

### `risk`

`risk.Engine` enforces pre-execution controls.

It checks:

- instrument validity and precision;
- order notional;
- position notional;
- account exposure;
- scoped limits by account, strategy, or instrument;
- reduce-only behavior;
- trading state, halt, and kill switch;
- command throttles and queue capacity;
- duplicate client-order risks where applicable.

Risk rejection happens before adapter submission in normal runtime paths. A
rejection should be typed, visible, and should not mutate orders, fills,
positions, or portfolio state.

### `portfolio`

`portfolio.Portfolio` is the event-driven accounting layer.

It consumes account, order, fill, position, position lifecycle, and market data
events. It updates:

- balances;
- positions;
- commissions;
- realized PnL;
- unrealized PnL;
- mark values;
- exposures by instrument, account, venue, and target currency;
- conversion rates;
- analyzer trade records.

Fills are deduplicated before accounting. Mark updates invalidate cached
unrealized PnL where needed.

### `strategy`

`strategy` is the developer-facing authoring layer.

A strategy receives a `strategy.Runtime`, subscribes to data in `OnStart`, uses
typed callbacks for market and execution events, creates orders with
`model.OrderFactory`, and submits commands through the runtime.

The runtime exposes:

- cache and portfolio access;
- clocks and timers;
- market-data subscriptions;
- historical data requests;
- order factory helpers;
- submit, modify, cancel, query, and account commands.

`strategy.NewTyped` lets a plain Go struct implement only the callbacks it
needs, such as `OnOrderBook`, `OnOrderFilled`, or `OnTimer`.

### `backtest`

`backtest.Runner` runs strategies against timestamped events with deterministic
state.

It owns:

- event ordering;
- deterministic clock advancement;
- timer dispatch;
- order expiration;
- same-timestamp command draining;
- matching and fill generation;
- fill model, slippage, and order latency;
- result summaries and deterministic JSON.

Backtest should share the same strategy/runtime, model, cache, portfolio, risk,
and execution semantics as live where the parity contract says behavior is
shared.

### `live`

`live.Node` wraps `platform.Node` into a live trading node.

It wires:

- data clients;
- execution clients;
- strategies;
- risk engine;
- portfolio;
- cache;
- bus;
- reconnect policy;
- health reporting;
- start and stop lifecycle.

Use `live.NewNodeBuilder()` when assembling a live node from application code.

### `platform`

`platform.Node` is the orchestration layer. It wires data, execution, risk,
portfolio, cache, strategy runtime, bus, timers, subscriptions, and reconcilers
into one runtime facade.

Strategy runtime methods are implemented here for live mode:

- market data subscriptions route to `data.Engine`;
- orders route through `risk.Engine` before `execution.Engine`;
- execution events update cache and portfolio and publish to the bus;
- reconciliation runs against registered execution clients;
- timers publish strategy timer events.

### `testsuite`

`testsuite` is the parity oracle.

It contains reusable contract testers for:

- adapter capability honesty;
- model/cache/data/execution/risk/portfolio/backtest/live behavior;
- reconciliation;
- master scorecard metadata;
- release-gate documentation artifacts;
- benchmark report scripts.

When a behavior crosses packages, prefer adding a reusable `testsuite` case in
addition to package-local tests.

## Runtime Flows

### Market Data Flow

```text
venue.DataClient / StreamingDataClient
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
subscriptions call `SubscribeMarketData`, forward venue events into
`model.MarketEvent`, update cache, run bar aggregators, and publish to the bus.

### Strategy Order Flow

```text
strategy callback
        |
        v
strategy.Runtime.SubmitOrder / SubmitOrderList
        |
        v
platform.Node adds command metadata and checks risk
        |
        v
execution.Engine sends command to venue.ExecutionClient
        |
        v
adapter maps model command to SDK request
        |
        v
sdk talks to venue API
        |
        v
order/fill/position reports return as model.ExecutionEvent
        |
        +--> cache.Cache
        +--> portfolio.Portfolio
        +--> bus.Bus -> strategy callbacks
```

The important invariant is that strategy code does not call SDKs directly.
Commands go through runtime so risk, cache, portfolio, lifecycle, and metadata
stay consistent.

### Bracket Order Flow

`model.OrderFactory.Bracket` creates an order list with:

- one parent entry order;
- one take-profit child;
- one stop-loss child;
- shared `OrderListID`;
- parent-child IDs;
- reduce-only exit children;
- OTO/OCO contingency metadata.

The platform and execution manager hold children until the parent fills. When
one OCO child fills, the sibling is cancelled. This flow is shared by live and
backtest tests.

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

Unresolved discrepancies are structured state. They are not log-only warnings.
Missing fills must be applied once, and duplicate repair must be idempotent.

## Backtest vs Live

Backtest and live share the same strategy authoring surface. The difference is
the source of truth:

- backtest uses deterministic market events and a simulated execution path;
- live uses registered venue data and execution clients;
- both should preserve model types, command metadata, risk semantics, cache
  updates, portfolio accounting, and lifecycle events.

Use backtest to prove strategy behavior and lifecycle expectations. Use live
node tests to prove wiring, stream health, reconnect, and adapter capability
claims.

## Failure And Unsupported Behavior

Failure should be explicit:

- unsupported adapter surfaces return `model.ErrNotSupported` or a wrapped
  equivalent;
- risk rejection returns typed `risk.ErrRiskRejected` behavior and emits a
  visible rejection event where appropriate;
- reconciliation discrepancies are recorded in audit state;
- health snapshots expose stale streams, queue pressure, and last errors.

Silent success is a bug when a caller asked for trading lifecycle behavior.

## Documentation And Evidence

Long-lived docs live in:

- `docs/architecture.md` for this architecture overview;
- `docs/guides/` for usage guides;
- `docs/parity/` for scorecards, matrices, quality gates, and release evidence;
- `docs/plans/` for the active master plan.

Release-facing evidence should be backed by:

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./testsuite -run 'TestNautilusMaster'
bash scripts/verify_nautilus_parity.sh
bash scripts/generate_nautilus_benchmark_report.sh
```
