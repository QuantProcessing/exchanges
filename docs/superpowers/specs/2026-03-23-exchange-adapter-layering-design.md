# Exchange Adapter Layering Design

## Status

Proposed

## Problem

The repository has converged on a shared unified exchange interface, but exchange packages have started to diverge in structure, transport behavior, naming, and testing depth.

This divergence is now large enough to hurt:

- readability for maintainers moving across adapters
- confidence when extending a package or adding a new adapter
- review quality, because there is no stable standard for "correct shape"
- incremental evolution, because each new adapter can establish a new local pattern

The goal is not to force every exchange into identical files or identical internals. The goal is to define a stable layered contract so adapters are behaviorally uniform, structurally legible, and still able to express exchange-specific constraints.

## Goals

- Make adapter behavior consistent at the unified interface boundary.
- Define stable responsibilities for registry, options, adapter, helper, and SDK layers.
- Keep the main adapter files as the primary reading entrypoints.
- Allow exchange-specific protocol and account-model differences without letting them spread through the whole package.
- Define a minimum test matrix and review checklist for all adapters.
- Create a phased convergence path for existing packages.

## Non-Goals

- Force every exchange package to use the exact same number of files.
- Require immediate full rewrites of existing adapters.
- Eliminate exchange-specific behavior when the exchange protocol genuinely requires it.
- Standardize every SDK internals detail across all exchanges.

## Design Principles

1. Strongly unify external behavior.
2. Weakly unify internal organization.
3. Concentrate exchange-specific complexity into explicit seams.
4. Prefer clear entrypoints over aggressive abstraction.
5. Require documented justification for deviations.

## Layer Model

### 1. Unified Semantics Layer

This layer is a hard contract. All adapters must behave consistently here.

It covers:

- constructor semantics
- symbol semantics
- `Fetch*` / `Watch*` / `StopWatch*` behavior
- supported-capability behavior
- transport behavior
- error semantics

### 2. Adapter Orchestration Layer

This layer defines where code should live inside an exchange package.

It should keep:

- package entrypoints easy to locate
- market-specific adapter behavior easy to follow
- exchange-specific complexity isolated

### 3. SDK Implementation Layer

This layer may vary per exchange, but its responsibilities and naming should still be legible across packages.

It should keep:

- REST and WS boundaries clear
- public and private paths clear
- auth and signing separated from business requests

### 4. Testing And Evolution Layer

This layer defines:

- minimum test coverage expectations
- how deviations are reviewed
- how old packages converge without blocking all current work

## Hard Requirements

### Constructors

Every exchange package must expose:

- `NewAdapter(ctx, Options)` for perp
- `NewSpotAdapter(ctx, Options)` for spot, if spot is supported

Registry construction must remain a thin translation layer from `map[string]string` into `Options`.

`register.go` must not:

- perform network I/O
- probe exchange capabilities
- apply business-specific branching beyond market-type dispatch

Constructors may perform initial metadata loading, but constructor behavior must be explicit and stable:

- fail-fast adapters must fail consistently when essential market metadata cannot be loaded
- permissive adapters must document why degraded construction is safe

The long-term default is to prefer fail-fast construction for required market metadata.

Constructor auth policy must also be explicit. An adapter must choose one of these policies and document it:

- public-data-tolerant: constructor may succeed with empty credentials, and private calls fail later with `ErrAuthFailed`
- strict-completeness: if any private credential field is provided, all required credential fields must be present or constructor fails with `ErrAuthFailed`

Partial-credential behavior must never be accidental.

### Symbol Semantics

Adapter-facing methods should treat base symbols as the primary public contract, for example `"BTC"`.

Only adapter internals and SDK internals may use exchange-native symbols such as:

- `BTCUSDT`
- `BTC-USDT-SWAP`
- `BTC_USDC_PERP`

`FormatSymbol` and `ExtractSymbol` should be the primary explicit translation seam between unified symbols and exchange-native symbols.

Legacy adapters may still accept already formatted exchange symbols at the public boundary for compatibility. That is a tolerated exception during convergence, not the target standard for new adapters.

On cache miss:

- preferred behavior is deterministic fallback only when it is safe and obvious
- otherwise the adapter should rely on market metadata rather than ad hoc string surgery

### Error Semantics

All adapters must use shared sentinel errors for unified behavior:

- `ErrNotSupported`
- `ErrAuthFailed`
- `ErrOrderNotFound`
- `ErrSymbolNotFound`
- other applicable shared errors from `errors.go`

Exchange-specific details should be wrapped with `ExchangeError` where useful, but callers must still be able to use `errors.Is`.

Capability absence must return `ErrNotSupported`. It must not:

- silently no-op
- return `nil`
- return arbitrary string errors for stable unsupported cases

### BaseAdapter Transport Conventions

`OrderMode` is currently a `BaseAdapter` convention, not part of the public `Exchange` interface.

That means this standard treats transport behavior as a package-level contract for adapters that embed `BaseAdapter`, not as a universal interface guarantee.

If an adapter participates in `OrderMode`, it must document which of these patterns it follows:

- full switching: supported order operations consistently respect `OrderMode`
- REST-only: order operations always use REST and the adapter documents that choice explicitly
- hybrid: only a documented subset of order operations switch transport, including adapters that default to REST but can switch selected operations to WS

Hybrid behavior is allowed only as a transitional state and must be tested explicitly.

The long-term target is to converge on either:

- full switching for supported order operations
- or explicit REST-only behavior with no ambiguous partial abstraction

### Watch Semantics

Supported stream methods must either:

- establish the stream and maintain any promised local state
- or return a clear error

Unsupported streams must return `ErrNotSupported`.

For adapters that embed `BaseAdapter`, `WatchOrderBook` should maintain local orderbook state that is compatible with `GetLocalOrderBook` and the adapter's readiness behavior.

`WaitOrderBookReady` is also a `BaseAdapter` convention rather than a public interface requirement, so new rules in this area should be applied at the package level, not described as part of the unified `Exchange` contract.

### Order Query Semantics

`FetchOrderByID`, `FetchOrders`, and `FetchOpenOrders` must have stable, documented support boundaries for each adapter and market type.

Adapters must not drift into ambiguous behavior such as:

- returning partial history without documenting it
- silently treating unsupported terminal lookup as empty success
- returning inconsistent semantics between spot and perp without explicit reason

## Package Structure Standard

Each exchange package should use these entry files by default:

- `options.go`
- `register.go`
- `common.go`
- `perp_adapter.go`
- `spot_adapter.go` when spot is supported
- `orderbook.go` or market-specific orderbook files
- `funding.go` when perp funding logic exists

### File Responsibilities

#### `options.go`

Owns:

- `Options`
- default values
- quote validation
- logger defaulting

Must not own:

- constructors
- request mapping
- runtime network behavior

#### `register.go`

Owns:

- registry key
- translation from registry options into `Options`
- market-type switch

Must stay thin.

#### `common.go`

Owns:

- pure helpers
- shared cache types or cache-building helpers when those are reused across files
- pure mapping helpers
- time and decimal parsing helpers
- shared market metadata shaping

Must not own:

- network I/O
- adapter lifecycle orchestration

#### `perp_adapter.go` and `spot_adapter.go`

These remain the main reading entrypoints.

A maintainer should be able to open one of these files and quickly answer:

- what the adapter supports
- which unified methods are implemented
- where exchange-specific branches live
- which helpers own the heavy lifting

#### `orderbook.go`

Owns local orderbook synchronization and readiness semantics.

Split into market-specific files only if spot and perp genuinely use different synchronization models.

#### `funding.go`

Owns funding-related functionality when implemented.

Funding logic should not be scattered through `perp_adapter.go` unless the implementation is trivial and temporary.

## Allowed Auxiliary Files

Extra files are allowed only when they isolate a stable, understandable responsibility.

Examples:

- `order_request.go`
- `private_mapping.go`
- `stream_mapping.go`
- `streams.go`
- `private_profile.go`

### Rules For Auxiliary Files

- The main adapter files must still read as the package entrypoints.
- The helper file must own one clear concern.
- The helper file must not create a second hidden architecture inside the package.

`private_profile.go` is an allowed exception only when the exchange has a durable private-path split, such as distinct account modes that affect many private operations.

This is a controlled exception, not a default template.

## SDK Standard

SDK layout may vary, but the following boundaries must remain clear:

- client bootstrapping in `client.go`
- shared wire types in `types.go`
- auth and signing separated from request behavior
- REST and WS separated
- public and private behavior separated

### SDK Naming

SDK naming should converge on these verbs:

- `Get*`
- `Place*`
- `Cancel*`
- `Modify*`
- `Subscribe*`
- `Unsubscribe*`

Avoid introducing new verbs for existing concepts where a repository-standard term already exists.

For example:

- prefer `PlaceOrder` over `ExecuteOrder`
- prefer one consistent WS client naming style rather than mixing `WsClient`, `WSClient`, `PublicWSClient`, and unrelated variants without reason

### SDK Deviation Rule

SDK deviation is acceptable when required by protocol constraints, but the deviation should be localized and documented.

## Testing Standard

Every concrete market adapter must provide a minimum matrix consisting of:

- compliance tests
- order tests
- order query semantics tests
- lifecycle tests
- local state tests

In addition, adapters must add exchange-specific tests where they have unique behavior, including:

- transport routing behavior
- account-mode behavior
- request translation rules
- orderbook sequencing and gap recovery
- client-id constraints
- exchange-specific auth behavior

The intended split is:

- shared tests verify the unified contract
- local tests verify the exchange-specific edges

Neither category is sufficient on its own.

## Review Checklist

Every new adapter or major adapter refactor should be reviewed against this checklist:

1. Are constructor semantics explicit and stable?
2. Are base-symbol semantics preserved at the adapter boundary, or is any compatibility exception explicit?
3. Are shared sentinel errors used consistently?
4. Is `OrderMode` classification explicit and accurate: full switching, REST-only, or approved hybrid?
5. Are adapter, helper, and SDK boundaries clear?
6. Is exchange-specific complexity concentrated into a small number of files?
7. Do the main adapter files remain the primary reading entrypoints?
8. Does SDK naming align with repository conventions?
9. Does the adapter satisfy the minimum test matrix?
10. Are deviations from this standard explicitly justified?

## Deviation Policy

A deviation is acceptable only when at least one of the following is true:

1. The exchange protocol has a hard constraint that the standard shape cannot reasonably express.
2. The exchange has a durable market/account model split that affects many private operations.
3. The alternative structure produces materially better readability while preserving the layered boundaries in this document.

Deviations must be:

- small in scope
- documented in the package design/spec
- visible in review

## Current Package Assessment

### Binance

Strengths:

- close to the historical repository shape
- clear adapter entrypoints
- real transport switching in perp
- broad stream support

Gaps against this standard:

- constructor behavior is stricter than some packages and looser than others, so the repository lacks a consistent constructor policy
- spot order transport behavior does not presently participate in a repository-wide `OrderMode` contract
- SDK/package naming is not yet the single repository standard

Role in convergence:

- use as one of the baseline references, not as the final standard by itself

### OKX

Strengths:

- close to the historical repository shape
- strong symbol mapping via loaded instruments
- clear flat SDK organization

Gaps against this standard:

- constructor failure policy is permissive where other packages are fail-fast
- some credential behavior is inconsistent with other spot adapters
- several unsupported or missing paths still return free-form errors instead of shared sentinel errors
- naming and stream coverage still differ from Binance

Role in convergence:

- baseline reference, especially for flat SDK structure and explicit instrument mapping

### Bitget

Strengths:

- good test depth
- explicit handling of exchange-specific account-mode realities
- strong request and orderbook unit coverage

Gaps against this standard:

- introduces a private-profile/classic-only local architecture that should be treated as a controlled exception, not a new package default
- uses an approved hybrid transport shape today: REST-default with optional WS switching for a documented subset of order operations
- funding and private-path placement should be normalized relative to the package standard

Role in convergence:

- acceptable package with targeted cleanup and explicit exception classification

### Backpack

Strengths:

- focused mapping and request tests
- clear market-cache-driven metadata load
- exchange-specific client-id handling is well localized

Gaps against this standard:

- currently behaves as a de facto REST-only adapter and should declare that explicitly unless full transport switching is added later
- stream organization has drifted into a separate local pattern
- SDK naming is farther from the repository norm
- some stable failures still use free-form errors rather than shared sentinel errors
- package shape is more fragmented than needed for this repository

Role in convergence:

- first package to prioritize for structural convergence

## Phased Convergence Plan

### Phase 1: Standardize The Rulebook

- adopt this document as the baseline adapter standard
- use it for all new adapters immediately
- require reviewers to check major adapter work against this document

### Phase 2: Bring Current Outliers Under Control

Priority order:

1. `backpack`
2. `bitget`
3. `binance`
4. `okx`

#### Backpack Phase-2 Targets

- decide and document whether it is REST-only or a real `OrderMode` participant
- normalize SDK naming where practical
- reduce unnecessary structural fragmentation while keeping clear stream/orderbook boundaries
- expand SDK-level tests where runtime coverage is thin

#### Bitget Phase-2 Targets

- classify classic/private-profile layering as an approved exception with tighter boundaries
- decide whether REST default is a temporary compatibility choice or the permanent adapter contract
- align file placement for funding/private transport responsibilities

#### Binance And OKX Phase-2 Targets

- align constructor failure policy
- align naming and minor transport/documentation behavior
- reduce historical inconsistencies without large churn

### Phase 3: Enforce For All New Work

After the first convergence pass:

- new adapters must satisfy this standard by default
- deviations must be justified in their design/spec
- review should reject unmotivated structural drift

## Recommended Immediate Follow-Up

1. Resolve the open decisions below and then publish this document as the repository baseline standard.
2. Produce per-package gap lists for `backpack` and `bitget`.
3. Turn the review checklist into an actionable PR or review template.
4. Use the standard as the starting point for future adapter-generation workflows.

## Open Decisions

These should be resolved before implementation planning:

1. Should repository-standard constructor behavior be fail-fast for required market metadata in all adapters?
2. Should repository-standard `OrderMode` default remain inherited from `BaseAdapter`, or should adapters be required to declare their transport default explicitly?
3. Should stream logic remain in the main adapter files by default, with split files treated as exceptions?
4. Should SDK WS client naming be normalized to a single style across the repository?
