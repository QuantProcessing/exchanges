# Internalize TradingAccount Runtime Behind a Stable Root-Package Facade

## Summary

The repository root has become easier to scan after the recent low-risk cleanup,
but the remaining pressure is structural rather than cosmetic.

The root `exchanges` package still owns the full implementation of:

- `TradingAccount`
- `OrderFlow`
- `orderFlowRegistry`
- `EventBus`
- `Subscription`

These are not all equal in API importance.

`TradingAccount` and `OrderFlow` are now deliberate user-facing concepts.
`orderFlowRegistry` and `EventBus` are implementation mechanics that mainly
exist to support the account runtime.

The next cleanup should not be another round of file shuffling. It should move
the account-runtime implementation into an internal package while preserving the
current public root-package API shape.

Recommended direction:

- keep `exchanges.TradingAccount` and `exchanges.OrderFlow` as the public model
- move the main runtime implementation under `internal/accountruntime`
- move `EventBus` and `Subscription` out of the root package's center of
  gravity and treat them as internal runtime plumbing
- keep compatibility controlled by a thin root-package facade instead of asking
  users to import a new public subpackage

## Problem

The current root-package clutter is no longer mostly about file count. It is
about root-package responsibility.

Today the root package mixes three layers:

1. public domain model and exchange contracts
2. runtime implementation for account synchronization and order tracking
3. low-level fan-out plumbing used by that runtime

That creates three long-term issues:

1. the root package is harder to scan because implementation mechanics sit next
   to stable public concepts
2. internal account-runtime refactors remain more expensive than they need to be
   because the implementation lives directly in the public package
3. `EventBus` looks like a first-class root-package concept even though its
   real role is "best-effort in-process fan-out for runtime internals"

Another round of renaming or file merging would improve appearance again, but
it would not fix that structural mixing.

## Goals

- keep the current public `TradingAccount` and `OrderFlow` user model
- reduce the root package to stable public API surface plus thin constructors
  and wrappers
- move account-runtime implementation details into a dedicated internal package
- move `EventBus` and `Subscription` out of the root package's recommended
  public surface
- avoid introducing a new public import path for users
- preserve current runtime behavior and test expectations

## Non-Goals

- do not redesign `TradingAccount` semantics
- do not redesign `OrderFlow` semantics
- do not introduce strategy policy into the runtime
- do not create a new public package such as `exchanges/account` or
  `exchanges/runtime`
- do not remove the current root-package API in the same step as internalization

## Decision

The repository should adopt a **root facade + internal implementation** model.

### Public layer

The root `exchanges` package continues to define the user-facing account
runtime:

- `TradingAccount`
- `OrderFlow`
- `NewTradingAccount`

Users continue importing `github.com/QuantProcessing/exchanges`.

### Internal layer

A new package `internal/accountruntime` should own the main implementation of:

- trading-account state synchronization
- order-flow lifecycle handling
- order-flow registration and routing
- fan-out pub/sub infrastructure currently represented by `EventBus` and
  `Subscription`

### Compatibility stance

`TradingAccount` and `OrderFlow` remain first-class public concepts.

`EventBus` is no longer treated as a root-package concept we want to encourage.
If compatibility requires a temporary root wrapper, it should be clearly treated
as a compatibility surface, not as the preferred runtime abstraction.

## Why This Instead of a Public Subpackage

A public subpackage like `exchanges/runtime` would be clean internally, but it
would create migration cost immediately:

- downstream code would need new imports
- examples and docs would need a new package story
- the repository would be choosing two public top-level mental models at once

That is too large a scope for a cleanup motivated by root-package clarity.

The internal-package route gives most of the structural benefit without forcing
consumers to re-learn the API layout.

## Package Layout Direction

Recommended internal package:

- `internal/accountruntime`

That package should contain code equivalent to today's:

- `TradingAccount` runtime logic
- `OrderFlow`
- `orderFlowRegistry`
- `EventBus`
- `Subscription`

The root package should then be reduced to one of these two shapes:

### Preferred shape

The root package defines thin wrapper types whose behavior delegates to the
internal runtime package.

This keeps the root package in control of naming and compatibility, while the
real implementation lives behind a tighter boundary.

### Acceptable shape

The root package aliases selected internal-facing structures through wrapper
constructors and methods, as long as the result keeps public semantics stable
and does not leak implementation-only names.

The important rule is:

**users should keep thinking in `exchanges.TradingAccount` and
`exchanges.OrderFlow`, while maintainers implement them inside
`internal/accountruntime`.**

## EventBus Position

`EventBus` is useful, but its role is narrow.

It is:

- in-process
- non-blocking
- best-effort
- suitable for runtime fan-out

It is not:

- durable
- reliable delivery
- cross-process
- a root domain concept

Because of that, the design should stop centering it in the root package.

Recommended direction:

- move `EventBus` and `Subscription` implementation into
  `internal/accountruntime`
- stop presenting them as part of the root package's primary mental model
- if a public compatibility wrapper is temporarily needed, document it as
  compatibility-oriented rather than recommended

This keeps the repository aligned with Go practice: helper infrastructure that
exists only to support one subsystem generally belongs with that subsystem, not
in the main public package.

## Facade Boundary

The public root-package facade should stay intentionally small.

It should expose:

- constructors
- public lifecycle methods
- public query methods
- public subscription methods
- public wait and flow-consumption methods

It should not directly carry:

- registry bookkeeping
- fan-out implementation details
- background loop coordination details
- unexported helper state that only exists for runtime plumbing

That boundary is the main reason to do the refactor. If the root package still
owns those internals after the move, the cleanup did not actually solve the
problem.

## Migration Strategy

This should happen in stages.

### Stage 1: Introduce internal package

- create `internal/accountruntime`
- move `EventBus`, `Subscription`, `OrderFlow`, and `orderFlowRegistry` there
- add root wrappers or delegating types so public semantics stay the same

### Stage 2: Move TradingAccount internals

- move synchronization, refresh, routing, and flow-bridging logic into the
  internal package
- keep the root `TradingAccount` surface stable

### Stage 3: Reclassify public plumbing

- update docs and examples so `TradingAccount` and `OrderFlow` stay central
- stop treating `EventBus` as a promoted root-package concept
- decide whether a root-level `EventBus` compatibility surface should remain,
  be deprecated, or be removed in a later dedicated breaking release

## Testing Requirements

This refactor is mostly structural, so tests should protect behavior rather
than new features.

Minimum required verification:

- root package tests for `TradingAccount`
- root package tests for `OrderFlow`
- root package tests for helper order APIs
- tests covering any root-level `EventBus` compatibility story that remains
- adapter regression tests already sensitive to `TradingAccount` behavior
  (`nado`, `grvt`, and shared suite smoke)

The key success condition is that downstream-visible behavior does not change
while the implementation boundary tightens.

## Risks

Main risks:

- accidentally leaking internal-only names back through the root facade
- creating awkward double-indirection where both root and internal layers own
  meaningful logic
- introducing wrapper churn without actually reducing root-package complexity
- turning `EventBus` migration into an accidental extra breaking change

These risks are why this design prefers a deliberate facade boundary instead of
just moving files into a new folder.

## Success Criteria

This design is successful when:

- the root package is centered on stable public concepts rather than runtime
  mechanics
- `TradingAccount` and `OrderFlow` remain the user-facing API
- runtime implementation details live under `internal/accountruntime`
- `EventBus` is no longer treated as a primary root-package concept
- behavior and current tests remain stable

## Recommendation

Proceed with a dedicated refactor that introduces `internal/accountruntime`
behind a stable root-package facade.

Do **not** mix that work into the current cleanup commit series.

The root cleanup done so far should remain a low-risk preparation step. The
actual package-boundary refactor should be handled as its own spec, plan, test
pass, and implementation cycle.
