# Remove LocalState and Make TradingAccount the Only Account Runtime

## Summary

`TradingAccount + OrderFlow` is now the public model, but the implementation
still relies on `LocalState` as an internal compatibility core.

The next stage should remove `LocalState` completely instead of keeping a
second public or semi-public account-runtime abstraction alive. The repository
should converge on one account-runtime object:

- `TradingAccount` for account lifecycle, caches, subscriptions, and writes
- `OrderFlow` for one order's lifecycle updates

This design intentionally does **not** introduce a new transitional internal
type such as `accountRuntime`. Instead, it moves the proven `LocalState`
responsibilities directly into `TradingAccount`, then deletes `LocalState` and
its related compatibility concepts once the new tests are strong enough.

## Problem

Keeping `LocalState` around after `TradingAccount` is established causes three
long-term problems:

1. the repository keeps two names for the same conceptual job
2. `TradingAccount` remains structurally dependent on a type we no longer want
   users or maintainers to think in
3. future work must keep translating between old helper concepts
   (`OrderResult`, `WaitTerminal`, `RunLocalStateSuite`) and the new model

At that point `LocalState` stops being a compatibility convenience and starts
becoming maintenance debt.

## Goals

- make `TradingAccount` the only account-runtime implementation
- remove `LocalState` from production code, public API, shared suite naming, and
  release-facing docs
- preserve the existing synchronization behavior: bootstrap snapshot,
  websocket-driven updates, local caches, fan-out, and order tracking
- keep deletion risk controlled by explicit test gates
- keep `OrderFlow` as the only per-order runtime object

## Non-Goals

- do not redesign market-data order book handling
- do not add strategy policy to the library
- do not rename `TradingAccount` to `AccountState`
- do not delete archived specs or historical references under
  `docs/superpowers/archive`

## Decision

The repository should remove `LocalState` entirely and converge on one runtime
model:

- `TradingAccount` is the only account-runtime type
- `OrderFlow` is the only order-lifecycle handle
- `OrderResult`, `WaitTerminal`, and `RunLocalStateSuite` are removed

The implementation strategy is:

1. harden tests around `TradingAccount` and `OrderFlow`
2. move `LocalState` responsibilities into `TradingAccount`
3. delete `LocalState` only after all old guarantees are covered by the new
   tests

## Why Not Just Rename LocalState

Renaming `LocalState` to something like `AccountState` would improve naming, but
it would not fix the deeper mismatch.

The public entry point is no longer a passive state holder. It starts runtime
state, places orders, tracks order lifecycles, and returns `OrderFlow`
instances. That behavior is better described by `TradingAccount` than by any
`*State` name.

So the chosen direction is:

- keep `TradingAccount` as the public type
- remove `LocalState` instead of renaming it

## Architecture After Removal

After this change, `TradingAccount` should directly own the responsibilities
that currently live in `LocalState`.

### TradingAccount responsibilities

- initial `FetchAccount` bootstrap
- `WatchOrders` subscription
- `WatchPositions` subscription when supported
- cached balance, positions, and open orders
- order and position fan-out subscriptions
- periodic reconciliation refresh
- write helpers: `Place`, `PlaceWS`, `Cancel`, `CancelWS`, `Track`
- per-order routing into `OrderFlow`

### OrderFlow responsibilities

- latest order snapshot
- public stream of order updates
- waiting helpers
- lifecycle cleanup

### Removed concepts

The following compatibility concepts should disappear from active code:

- `LocalState`
- `NewLocalState`
- `OrderResult`
- `WaitTerminal`
- `RunLocalStateSuite`
- `LocalStateConfig`

## File-Level Direction

This design does not require a new named internal runtime type, but it should
still avoid creating one giant `trading_account.go`.

Recommended split:

- `trading_account.go`
  public constructor and public methods
- `trading_account_state.go`
  caches, bootstrap, refresh
- `trading_account_streams.go`
  order and position subscriptions, fan-out, update application
- `trading_account_place.go`
  `Place`, `PlaceWS`, `Track`, `Cancel`, and flow bridging

The important rule is not the exact filenames. The important rule is that
`TradingAccount` becomes self-sufficient and no longer wraps `LocalState`.

## Required Test Gates Before Deletion

`LocalState` should not be deleted just because the new API exists. It should be
deleted only after the new model reproduces the old guarantees.

### Gate 1: OrderFlow tests

`OrderFlow` must already cover:

- immediate match on current latest snapshot
- future update match
- no historical replay
- close wakes waiting callers
- multiple waiters do not interfere
- `Latest()` always reflects the newest snapshot
- `C()` and `Wait()` can coexist safely
- race-test coverage

### Gate 2: TradingAccount unit tests

`TradingAccount` must cover the core behavior formerly protected by
`LocalState`:

- startup success path
- startup failure on `FetchAccount`
- startup failure on `WatchOrders`
- non-fatal `WatchPositions` unsupported path
- `Start()` idempotence
- `Close()` closes active flows
- synchronous first order update is not lost
- `PlaceWS()` first update is not lost
- routing by `ClientOrderID` and later `OrderID`
- multiple concurrent tracked orders remain isolated
- open-order cache updates and terminal removal
- balance and position cache updates
- order and position fan-out subscriptions
- cancel result continues through the same `OrderFlow`

### Gate 3: Shared suite migration

Before deletion:

- adapters should call `RunTradingAccountSuite` directly
- `RunLocalStateSuite` should no longer be the active suite entry point
- the shared suite should still cover start, query, fan-out, market fill,
  limit cancel, and position close flows

### Gate 4: Repository verification

Deletion should be blocked until both of these are green:

- repository-focused package matrix
- live/gated adapter matrix under `RUN_FULL=1` with pass or clean skip

## Implementation Sequence

The removal should happen in two stages, not one large destructive commit.

### Stage A: Make TradingAccount fully independent

1. strengthen `OrderFlow` and `TradingAccount` tests
2. move cache, stream, and refresh behavior out of `LocalState`
3. make `TradingAccount` stop holding a `*LocalState`
4. migrate shared suite names and adapter tests to `RunTradingAccountSuite`
5. update docs and capability terminology away from `LocalState`

After Stage A, `LocalState` may still exist temporarily, but it should no longer
be required by active code paths.

### Stage B: Delete compatibility code

1. remove `LocalState` and `NewLocalState`
2. remove `OrderResult` and `WaitTerminal`
3. remove `RunLocalStateSuite` and `LocalStateConfig`
4. remove or rename policy/doc wording such as `local-state-capable`
5. rerun the full verification matrix

This split keeps regressions easier to diagnose:

- Stage A failures mean the new implementation is incomplete
- Stage B failures mean compatibility cleanup missed a live dependency

## Deletion Readiness Check

Right before deletion, repository search should show no live usage of the old
concepts outside historical/archive material:

- `LocalState`
- `NewLocalState`
- `RunLocalStateSuite`
- `OrderResult`
- `WaitTerminal`

If active production code, tests, or docs still rely on any of these, deletion
should stop until the references are migrated.

## Risks

The main risks are:

- silently dropping old synchronization guarantees while moving logic
- deleting compatibility names before adapter tests and shared suites are
  fully migrated
- moving too much logic into one large file and recreating the same structural
  problem under a new name

These risks are why the test gates and two-stage sequence are part of the
design, not optional cleanup advice.

## Success Criteria

This work is complete when all of the following are true:

- `TradingAccount` no longer depends on `LocalState`
- `OrderFlow` and `TradingAccount` carry all account-runtime behavior
- shared and live tests run through `TradingAccount`
- active code has no remaining dependency on `LocalState` concepts
- `LocalState` and its helper types are deleted

## Migration Order

1. strengthen tests first
2. migrate implementation into `TradingAccount`
3. migrate suite and docs terminology
4. delete `LocalState` and old helpers
5. publish only after repository verification passes

Downstream consumer migrations, including `cross-exchanges-arb`, should remain
deferred until the repository passes full shared testing and the new release tag
is published.
