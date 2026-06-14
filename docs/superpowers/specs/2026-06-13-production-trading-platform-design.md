# Production Trading Platform Design

## Goal

Implement the non-SDK platform layer as a production-grade, SDK-backed Go trading platform in the style of NautilusTrader. The platform must support real private execution streams, market-data subscriptions, reconnect reconciliation, complete order semantics, risk and portfolio state, realistic backtest matching, and multi-exchange consistency tests.

## Nautilus Alignment

The design follows the behavior surfaces visible in the local NautilusTrader reference:

- Live data clients expose connect, disconnect, subscribe, unsubscribe, and request hooks for distinct data types.
- Execution reconciliation processes order reports, fill reports, position reports, mass status reports, duplicate trades, fill-before-order situations, external orders, venue-order-id indexing, and fill quantity mismatches.
- Live node construction wires data clients and execution clients through engines, cache, clock, and message bus routing.
- Backtests run a deterministic venue loop: process existing market state, deliver data to strategies, drain commands, settle matching engines, and repeat cascading commands for the same timestamp.

## Architecture

### Model

`model` owns normalized trading vocabulary:

- Orders: market, limit, stop-market, stop-limit, market-to-limit, trailing-stop-market, trailing-stop-limit.
- Flags: post-only, reduce-only.
- Trigger fields: trigger price, activation price, trailing offset.
- Reports: order, fill, position, mass status, and execution event envelopes.
- Market data: ticker, quote tick, trade tick, bars, order book snapshots.

### Cache and Account

`cache` stores authoritative runtime state and indexes:

- Orders by account/order id, client id, and venue id.
- Fills by account/order/trade id with idempotent deduplication.
- Positions by account/position id and account/instrument id.
- Deferred fills when a fill arrives before the order.
- Mass status reconciliation checkpoints.

`account` owns reconciliation:

- Valid state transitions.
- Fill application and inferred fill generation from order-status deltas.
- Deferred fill replay after missing order creation.
- Position/account update application.
- Reconciliation result reporting.

### Platform

`platform.Node` orchestrates clients and runtime engines:

- Data subscriptions and market event fan-out.
- Execution private stream forwarding.
- Reconnect hooks and startup reconciliation.
- Submit/cancel/modify commands routed by account.
- Risk checks before external submission.
- Portfolio updates after execution events.

### Risk and Portfolio

`risk` validates orders before they reach the venue:

- Instrument exists and is trading.
- Order quantity and price match instrument precision.
- Market exposure and max order notional limits.
- Reduce-only and position-aware checks.

`portfolio` computes account-facing state:

- Balances by account and currency.
- Positions by account/instrument.
- Realized/unrealized PnL hooks where market price is available.
- Exposure by account, instrument, and quote currency.

### Backtest

`backtest` provides a deterministic simulation venue:

- Order book snapshots feed a matching engine.
- Market orders walk available book liquidity and can partially fill.
- Limit orders rest and fill when marketable.
- Stop orders trigger before matching.
- Strategy callbacks can submit/cancel orders; the settle loop drains cascading commands in timestamp order.

### Testsuite

`testsuite` becomes the platform contract gate:

- Data subscription behavior.
- Private stream order/fill/position events.
- Reconnect resubscribe plus reconciliation.
- Submit/cancel/modify lifecycle.
- Risk rejection behavior.
- Backtest matching invariants.
- Adapter capability declarations must match implemented behavior.

## Implementation Order

1. Complete order model, cache, account reconciliation, risk, and portfolio.
2. Add data subscription interfaces and in-memory subscription tests.
3. Add execution stream supervisor and reconnect reconciliation hooks.
4. Build realistic backtest matching.
5. Expand testsuite and migrate canonical adapters.
6. Roll out adapter-specific private stream mapping exchange by exchange.

## Non-Goals For This Increment

The platform may expose extension points before every exchange has native private stream coverage. Capability declarations must remain honest: adapters must not claim private stream readiness until their private stream adapter and tests pass.

