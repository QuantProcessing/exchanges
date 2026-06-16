# Module Guide

This guide explains what every major package does, what depends on it, and
when application code should import it.

## Layer Overview

```text
application / strategy code
        |
        v
live.Node or backtest.Runner
        |
        v
platform.Node
        |
        +--> strategy.Engine
        +--> data.Engine -----> venue.DataClient -----> adapter/* -----> sdk/*
        +--> risk.Engine
        +--> execution.Engine -> venue.ExecutionClient -> adapter/* -----> sdk/*
        +--> account.Reconciler
        +--> portfolio.Portfolio
        +--> cache.Cache
        +--> bus.Bus and kernel health/lifecycle helpers
```

The main boundary rule is simple: runtime code depends on `venue` interfaces,
adapters depend on SDKs, and SDKs do not depend on adapters or runtime
packages.

## `model`

`model` is the shared vocabulary. It owns identifiers, instruments, market
data, orders, order lists, commands, execution reports, account snapshots,
position reports, lifecycle events, and validation errors.

Use it whenever data crosses package boundaries. For example, strategies submit
`model.SubmitOrder`, adapters return `model.OrderStatusReport`, portfolio
consumes `model.FillReport`, and cache indexes `model.InstrumentID`.

Key types:

- identifiers: `Venue`, `AccountID`, `InstrumentID`, `StrategyID`,
  `ClientOrderID`, `VenueOrderID`, `OrderListID`, `PositionID`, `CommandID`,
  `CorrelationID`;
- orders: `SubmitOrder`, `ModifyOrder`, `CancelOrder`, `OrderList`,
  `OrderStatusReport`, `FillReport`;
- data: `Ticker`, `OrderBook`, `TradeTick`, `QuoteTick`, `Bar`,
  `MarketEvent`, `SubscribeMarketData`, `DataRequest`;
- account and position: `AccountSnapshot`, `Balance`, `PositionStatusReport`,
  `PositionLifecycleEvent`;
- command metadata: `CommandMetadata`.

Trading values use `shopspring/decimal`; do not convert prices, quantities,
PnL, or exposure to `float64` in trading state.

## `venue`

`venue` defines the normalized interfaces used by runtime code. It is the seam
between the trading platform and concrete exchange adapters.

Core interfaces:

- `Adapter`: returns venue identity, instruments, data client, execution
  client, and declared capabilities.
- `InstrumentProvider`: loads and lists normalized instruments.
- `DataClient`: connects, fetches ticker/order-book snapshots, reports health.
- `StreamingDataClient`: subscribes to market streams and exposes market
  events.
- `ExecutionClient`: connects account execution, queries account snapshots,
  submits and cancels orders, generates order reports, and exposes private
  execution events.

Optional interfaces describe optional capability families: order modify, order
query, order lists, fill reports, position reports, mass status, and execution
resubscribe.

Application code should check `venue.DeclaredCapabilities` before depending on
optional behavior.

## `sdk`

`sdk/` packages are venue-native protocol clients. They may expose official
REST and WebSocket endpoint names, request structs, response structs, signing,
listen-key handling, account modes, and product-specific options.

Use an SDK when you intentionally want a venue-specific API. Do not import SDKs
from strategy logic or runtime packages.

## `adapter`

`adapter/` packages convert SDK behavior into `venue` interfaces.

Adapters own:

- symbol and instrument resolution;
- exchange-native to normalized `model` mapping;
- precision and order validation;
- common error mapping;
- public market-data snapshots and streams;
- private execution streams where implemented;
- declared capability truth.

Adapters should return `model.ErrNotSupported` or a wrapped equivalent for
unsupported behavior.

## `cache`

`cache.Cache` is the local runtime state index. It stores instruments, accounts,
orders, fills, positions, market data, deferred fills, residual state, and
snapshots.

Use cache when you need to answer questions such as:

- What open orders does this account have?
- Which order has this client ID or venue order ID?
- Which fills belong to an order?
- What is the latest position for an account/instrument?
- Is there a deferred fill waiting for its order report?

Strategies should query `rt.Cache()` instead of maintaining duplicate state
unless the state is purely strategy-local.

## `bus`

`bus.Bus` publishes and subscribes to topic-based runtime events. Common topics
include:

- `market.data`;
- `execution`;
- `timer`;
- `error`.

Data, execution, platform, backtest, and strategy components use the bus to
fan out normalized events without tying every package to every consumer.

## `kernel`

`kernel` contains reusable runtime infrastructure: component lifecycle states,
clock abstractions, health snapshots, and message-bus primitives. Packages use
these concepts to expose state instead of hiding startup, reconnect, and
shutdown failures.

## `data`

`data.Engine` normalizes market data. It registers venue data clients, loads
instruments into cache, manages subscriptions, forwards market events into
cache and bus, supports bar aggregation, serves data requests, and reports
health. Standard market events include tickers, books, trades, quotes, bars,
funding rates for perpetual instruments, and custom extension data.

`data.Catalog`, `ReplayCatalog`, and `MemoryCatalog` represent historical or
replay data sources used by backtests and examples.

## `execution`

`execution.Engine` and `execution.Manager` own command routing and order
lifecycle behavior.

They handle:

- submit, modify, cancel, query, cancel-all, and batch-cancel commands;
- order lists and contingent order release;
- emulated stop/touched/trailing order behavior;
- normalized order, fill, position, and lifecycle events;
- matching-core behavior for backtests;
- health and reconciliation support.

Execution should not bypass risk, cache, portfolio, or command metadata.

## `account`

`account.TradingAccount` and `account.Reconciler` manage account-level
readiness and repair.

They coordinate private execution stream connection, startup snapshots,
order/fill/position reports, per-order tracking, stream health, and structured
unresolved discrepancies. Reconciliation output is product state, not only log
text.

## `risk`

`risk.Engine` runs before normal execution submission. It validates instrument
state, precision, notional limits, exposure limits, scoped limits, reduce-only
rules, trading state, kill switch, throttles, duplicate client IDs, and queue
capacity where configured.

Risk rejection should be visible and typed, and it must not mutate orders,
fills, positions, or portfolio state.

## `portfolio`

`portfolio.Portfolio` is event-driven accounting over cache-backed state. It
updates balances, positions, commissions, realized PnL, unrealized PnL, marks,
conversion rates, exposure, and analyzer records from normalized events.

Use portfolio for risk-facing and strategy-facing questions such as account
exposure, realized PnL, unrealized PnL, and position value.

## `strategy`

`strategy` is the author-facing layer. A strategy receives a
`strategy.Runtime`, subscribes to data in `OnStart`, receives typed callbacks,
creates orders through `model.OrderFactory`, and submits commands through the
runtime.

`strategy.NewTyped` lets a plain Go value implement only the callbacks it
needs, such as `OnOrderBook`, `OnTicker`, `OnOrderStatus`, `OnOrderFilled`,
`OnPosition`, `OnTimer`, and `OnError`.

## `backtest`

`backtest.Runner` replays timestamped market events through the strategy
runtime. It owns deterministic clock advancement, timer dispatch, market event
ordering, command draining, order expiration, matching, fill model, slippage,
latency, and result summaries.

Use backtests to prove strategy behavior before connecting real venue clients.

## `live`

`live.Node` wraps `platform.Node` into an operational live trading node. It
wires data clients, execution clients, strategies, risk, portfolio, cache, bus,
reconnect policy, health monitoring, startup, and shutdown.

Use `live.NewNodeBuilder()` for application assembly.

## `platform`

`platform.Node` is the runtime orchestrator. It implements `strategy.Runtime`
for live mode and coordinates data, execution, risk, portfolio, cache, bus,
timers, subscriptions, and reconciliation.

Most strategy commands eventually pass through `platform.Node`.

## `testsuite`

`testsuite` contains reusable contract tests and scorecards. Package-local
tests prove local behavior; testsuite cases prove cross-package behavior and
adapter capability truth.

Run focused tests while developing and broader tests before release.
