# Nautilus Core Parity Design

## Goal

Reach a Go trading platform experience that is materially equivalent to
NautilusTrader for core trading workflows: strategy lifecycle, event dispatch,
order state transitions, reconciliation, risk, portfolio, backtesting, and
adapter contract testing.

This document supersedes earlier same-day "production platform" completion
claims when those claims conflict with the stricter parity matrix below.

## Acceptance Definition

The platform is not parity-complete until all P0 suites pass:

- DataTester: instrument load, ticker, book/depth subscriptions, stream
  delivery, unsubscribe, and capability honesty.
- ExecTester: account snapshot, submit, cancel, modify, order reports, fill
  reports, position reports, private-stream resubscribe, and startup/gap
  reconciliation.
- LifecycleTester: Nautilus-style order events and legal transitions,
  including denied, emulated, released, submitted, accepted, rejected,
  triggered, pending-update, pending-cancel, updated, modify-rejected,
  cancel-rejected, canceled, expired, partially-filled, and filled.
- BacktestTester: deterministic venue loop, existing-order matching before
  strategy callbacks, same-timestamp command settling, immutable historical
  book data, liquidity consumption accounting, market-to-limit, stop, stop
  limit, and trailing stop behavior.
- PortfolioTester: balances, positions, realized/unrealized PnL, commission by
  currency, settlement-currency accounting, flips, partial closes, and
  reduce-only safety.
- AdapterParityTester: each exchange adapter can only declare a capability
  when it passes the relevant reusable tester with SDK-backed behavior or an
  explicitly scoped deterministic adapter fake.

## Architecture

`model` owns normalized commands, orders, reports, event kinds, instruments,
money, balances, positions, and PnL vocabulary.

`account` owns the order state machine and reconciliation logic. It must accept
venue reports and generate state changes without losing information.

`platform` owns live lifecycle orchestration: client connect/disconnect,
startup reconciliation, stream supervision, resubscribe, gap reconciliation,
risk checks, cache mutation, portfolio mutation, and message-bus publication.

`backtest` owns a simulated venue loop, not just a replay helper. It must model
the same order lifecycle surfaces as live mode where practical.

`testsuite` is the canonical parity gate. Feature code is not complete because
a package-level unit test passes; it is complete when the corresponding
Nautilus-style tester case is present and passing.

## Implementation Order

1. Expand `testsuite` reports and tester cases first.
2. Add missing lifecycle/order event model and state-machine tests.
3. Implement lifecycle transitions in `account` and propagate them through
   `platform`, `strategy`, and `backtest`.
4. Implement realistic backtest venue semantics and edge cases.
5. Implement risk and portfolio accounting to Nautilus-equivalent behavior.
6. Roll adapter capabilities through DataTester/ExecTester, exchange by
   exchange, using capability honesty as the default.

## Verification

Required local gate before claiming parity for a slice:

```bash
go test -count=1 ./model ./account ./cache ./risk ./portfolio ./platform ./strategy ./backtest ./testsuite
go test -count=1 ./adapter/... ./config/all
go test -run '^$' ./sdk/...
```

Race tests must run for lifecycle, platform, and backtest packages before any
production-readiness claim:

```bash
go test -race -count=1 ./account ./platform ./backtest ./testsuite
```
