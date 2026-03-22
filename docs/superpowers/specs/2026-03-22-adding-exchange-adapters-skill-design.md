# Adding Exchange Adapters Skill Expansion Design

## Goal

Strengthen the project-local `adding-exchange-adapters` skill so it can guide a future agent through a full new-adapter implementation in this repository, including:

- package skeleton and registry wiring
- deciding when a dedicated `sdk/` layer is required
- order-interface semantics and common failure modes
- private stream and local-state integration expectations
- live `testsuite` wiring and environment-variable setup

The skill should remain focused on adapter development in this repository rather than becoming a generic exchange-integration tutorial.

## Scope

In scope:

- expand `.claude/skills/adding-exchange-adapters/SKILL.md`
- add targeted reference documents under `.claude/skills/adding-exchange-adapters/references/`
- update the skill so it complements, rather than duplicates, the broader `exchanges` skill
- document Backpack-driven lessons that generalize to future adapters

Out of scope:

- changing repository production code
- changing shared `Exchange` interfaces in this task
- creating a separate standalone `sdk` skill
- rewriting or replacing the existing global `exchanges` skills

## Problem Statement

The current `adding-exchange-adapters` skill is useful for package shape, registry wiring, and minimum test coverage, but it is not yet strong enough to reliably guide a complex adapter implementation end to end.

The missing guidance is concentrated in four areas:

1. `sdk/` boundary decisions are underspecified.
2. Order-query semantics are not explicit enough, so an agent can implement `FetchOrder` incorrectly by scanning open orders only.
3. Private WebSocket and local-state readiness criteria are too implicit.
4. Live test wiring details are missing from the definition of done.

As a result, the current skill can help start an adapter, but it does not yet prevent several high-cost integration mistakes.

## Design Principles

The strengthened skill should follow these principles:

- keep the main `SKILL.md` as the entrypoint and routing layer
- move detailed implementation guidance into focused reference files
- prefer repository-specific rules, file paths, and anti-patterns over generic exchange-adapter advice
- describe engineering decisions and invariants, not tutorials or long narrative examples
- avoid duplicating material that is already better covered by the `exchanges` skill

## Proposed Structure

### Main skill

Keep:

- overview
- peer-template selection
- minimum file set
- non-negotiable wiring
- source-of-truth routing
- test matrix

Add or strengthen:

- `Before You Write Code`
- `Architecture Decisions`
- `Order Contract Checklist`
- `Private API Readiness`
- `Live Test Readiness`
- `Do Not Ship If`

The main skill should answer:

- when to use the skill
- what files to read first
- what architectural decisions must be made early
- what conditions must be true before an adapter is considered complete

### Reference files

Add:

- `references/sdk-boundaries.md`
- `references/order-semantics.md`
- `references/private-streams-and-localstate.md`
- `references/live-test-wiring.md`

Each reference should be independently usable and narrowly scoped.

## Main Skill Content

### Before You Write Code

This section should stay short and act as an adapter-specific gate, not a second copy of the `exchanges` skill.

It should tell the agent to:

- load `exchanges` first for shared contracts and invariants
- then read one closest peer package plus the specific `testsuite` files that match the target adapter shape
- write down the target adapter's market coverage, auth model, and whether private streams are expected before choosing file layout

The point is to force early architectural decisions, not to restate every root-package file in the repository.

### Architecture Decisions

This section should define when an adapter needs a dedicated `sdk/` package.

The guidance should explicitly say that a dedicated `sdk/` layer is usually required when the new exchange needs any of these:

- exchange-specific request signing
- multiple REST surface groups or multiple WebSocket connections
- exchange-native request and response types that would otherwise leak into adapters
- non-trivial private streaming logic
- reusable low-level methods shared by spot and perp adapters

The main skill should also say what does not belong in the adapter layer:

- raw REST path building
- signing logic
- wire-format structs
- WebSocket connection lifecycle internals

It should not hard-code one `sdk/` layout. Instead, it should tell the agent to choose among repository-native patterns:

- flat `sdk/` for compact integrations with one shared client shape
- `sdk/perp` and `sdk/spot` when market-specific low-level APIs diverge materially
- `sdk/common` or shared helpers only when there is real reuse that would otherwise be duplicated

The decision rule should be based on low-level API shape and reuse, not personal preference.

### Order Contract Checklist

This section should state that order interfaces are separate contracts, not interchangeable helpers.

The skill should explicitly route to `references/order-semantics.md` before implementing:

- single-order lookup
- open-order listing
- historical order listing

The main skill should also forbid a common anti-pattern:

- implementing single-order lookup by scanning only open orders

### Private API Readiness

This section should define what an adapter must account for before claiming account or trading support:

- how REST authentication works
- whether private WebSocket exists and what it is required for
- which account/order/position state comes from REST snapshots
- which state is maintained by stream deltas
- which unsupported shared-interface methods must return explicit not-supported errors

This section also needs an explicit support matrix so future agents know when `ErrNotSupported` is acceptable and when it means the adapter is not done.

The matrix should say:

- `WatchOrders` is required if the adapter claims production-ready private trading support or intends to pass `RunLifecycleSuite` or `RunLocalStateSuite`
- `WatchPositions` is optional for spot adapters and may be optional for perp adapters only when the exchange lacks a usable position stream; in that case the adapter must return an explicit not-supported error and the skill must treat position-stream support as incomplete rather than silently successful
- `RunLifecycleSuite` is mandatory for adapters that implement private order streams and claim lifecycle correctness
- `RunLocalStateSuite` is mandatory for adapters intended to support unified local state, and its minimum prerequisites are `FetchAccount` plus a real `WatchOrders`; `WatchPositions` remains additive rather than universally required

### Live Test Readiness

This section should add live test wiring to the adapter definition of done.

It should require:

- `.env.example` updates for exchange-specific credentials and test symbols
- `adapter_test.go` wiring to shared `testsuite`
- clear skip conditions when credentials or symbols are missing
- exchange-specific test-symbol and quote-currency configuration where needed

It should also choose a repository convention for `.env` lookup in new adapters:

- prefer a small helper in `adapter_test.go` that searches the worktree-local `.env` first and then parent directories up to the repository root

This avoids hard-coded relative paths that break when tests run from worktrees.

### Do Not Ship If

This section should list concrete stop conditions such as:

- registry advertises a market type with no adapter behind it
- `WatchOrders` is missing but order lifecycle is claimed to work
- local orderbook is declared supported but never reaches a non-`nil` synced state
- order-query methods silently collapse distinct semantics
- spot or perp stream methods return success while doing nothing
- live test prerequisites are undocumented

## Reference Content

### references/sdk-boundaries.md

This reference should define the adapter/SDK split in repository terms.

It should cover:

- what belongs in `sdk/`
- what belongs in adapter files
- the decision tree for choosing flat `sdk/`, `sdk/perp` plus `sdk/spot`, or mixed shared subpackages
- representative file layouts already present in this repository, such as flat `sdk/` packages and market-scoped SDK subpackages
- how mapping helpers should avoid leaking wire types into the unified layer
- when spot and perp should share SDK code versus split

It should include concrete anti-patterns:

- REST requests constructed inline in `perp_adapter.go`
- WebSocket auth code duplicated across spot and perp adapters
- unified `exchanges.Order` populated directly from unvalidated wire JSON

### references/order-semantics.md

This reference should define the intended meaning of each order-query surface for future adapter work.

It should cover:

- single-order lookup by order ID
- open-order list semantics
- history-order list semantics
- symbol filtering expectations
- acceptable fallback behavior when an exchange lacks a direct history endpoint
- when to return `ErrOrderNotFound`

It should include a warning that terminal-order lookup is part of the contract for single-order queries and that scanning open orders alone is not a valid implementation.

### references/private-streams-and-localstate.md

This reference should define readiness rules for:

- `WatchOrders`
- `WatchPositions`
- `FetchAccount`
- the adapter surfaces required by `LocalState`

It should explain that `LocalState` is not an adapter interface. It is built on top of adapter behavior, especially `FetchAccount`, `WatchOrders`, and optionally `WatchPositions`.

It should then explain the common repository pattern:

- initial REST snapshot
- private WS subscriptions
- stream-to-model mapping
- state fan-out to callbacks or local managers

It should also say that if a stream surface is unsupported, the adapter must return an explicit not-supported error instead of a no-op success.

### references/live-test-wiring.md

This reference should define how a new adapter is wired into shared live tests.

It should cover:

- `.env.example` additions
- expected environment-variable naming
- the preferred `.env` lookup helper pattern inside `adapter_test.go`
- which `testsuite` entries to wire for spot and perp
- when to use skip flags like slippage-related skips
- how to choose stable test symbols and quote defaults

It should explicitly say that a new adapter is not fully integrated until `adapter_test.go` exists and exercises the intended shared suites.

## Relationship To Existing Skills

The strengthened `adding-exchange-adapters` skill should not re-document the entire repository.

Division of responsibility:

- `exchanges` remains the source for repository-wide interfaces, types, invariants, and construction primitives
- `adding-exchange-adapters` becomes the source for new-adapter architecture, completion criteria, support-level decisions, and adapter-specific decision routing

The main skill should therefore point back to `exchanges` for:

- shared interfaces
- shared models and errors
- root package invariants

and keep its own focus on:

- new package creation
- exchange-specific low-level layering decisions
- support-matrix decisions for private streams and local state
- adapter completion and validation

## Validation Strategy

The updated skill should be validated in two ways:

1. direct review against the Backpack implementation work that exposed the gaps
2. pressure-testing whether a future agent could answer these questions from the skill:
   - when should a new adapter create `sdk/`?
   - how should single-order lookup differ from open-order listing?
   - what makes private stream support real rather than superficial?
   - what files and environment variables are required to wire live `testsuite` coverage?

For this task, validation will be document-level rather than code-level. Baseline repository tests in this environment are known to include network-dependent cases that fail under sandbox DNS restrictions, so verification for the skill work should focus on file correctness and review quality.

## Risks

- the main `SKILL.md` could become too large and less searchable if too much detail stays inline
- the references could duplicate the `exchanges` skill unless boundaries are stated clearly
- order-semantics guidance could drift from future code if it is written too specifically to current method names

## Mitigations

- keep detailed mechanics in the `references/` directory
- add explicit “complements `exchanges`” language in the main skill
- write order semantics in terms of contracts and failure modes, not only current implementation names

## Success Criteria

This task is successful when:

- `adding-exchange-adapters` is clearly capable of guiding complex new adapter work in this repository
- the main skill remains concise enough to route an agent effectively
- the four new references cover `sdk/`, order semantics, private streams/local state, and live test wiring
- the updated skill reads as repository-specific engineering guidance rather than generic tutorial prose
