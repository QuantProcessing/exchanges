<!-- AUTONOMY DIRECTIVE - DO NOT REMOVE -->
YOU ARE AN AUTONOMOUS CODING AGENT. EXECUTE TASKS TO COMPLETION WITHOUT ASKING
FOR PERMISSION.
DO NOT STOP TO ASK "SHOULD I PROCEED?" - PROCEED. DO NOT WAIT FOR CONFIRMATION
ON OBVIOUS NEXT STEPS.
IF BLOCKED, TRY AN ALTERNATIVE APPROACH. ONLY ASK WHEN TRULY AMBIGUOUS,
DESTRUCTIVE, CREDENTIAL-GATED, OR EXTERNAL-PRODUCTION.
USE CODEX NATIVE SUBAGENTS FOR INDEPENDENT PARALLEL SUBTASKS WHEN THAT IMPROVES
THROUGHPUT.
<!-- END AUTONOMY DIRECTIVE -->

# Repository Agent Notes

This file is for AI agents working inside this repository. It is not marketing
copy, not a public API reference, and not a release note. It is the local
execution contract for continuing the project-wide refactor toward a complete
Go NautilusTrader-style trading platform.

Agents must treat this document as a constraint layer above ordinary code style
preferences. If a local package, test, matrix, or plan contradicts a vague
assumption, use the local artifact as the source of truth and update this file
only when the project contract itself changes.

## Mission

Build an idiomatic Go implementation that reaches behavioral parity with
NautilusTrader core trading workflows and production execution semantics while
preserving this repository's SDK, adapter, and runtime boundaries.

The project is complete only when the master parity contract is true:

> A strategy written once against the Go `strategy.Runtime` can run unchanged in
> backtest and live modes; submit bracket, list, and advanced orders; receive
> typed order, fill, position, account, and data callbacks; survive private
> stream disconnects and startup gaps; reconcile missing fills, orders, and
> positions without duplicate state; enforce risk before execution; compute
> portfolio state and PnL consistently; and pass a reusable Nautilus parity
> scorecard for every claimed adapter capability.

Do not claim the project is complete, Nautilus-compatible, production-complete,
or parity-complete unless the required scorecard and release gates pass.

Do not describe this repository through comparisons with other exchange helper
libraries. The target identity is a Go NautilusTrader-style runtime backed by
venue SDKs and capability-honest adapters.

## Source Of Truth

Read these artifacts before changing architecture, lifecycle, risk, portfolio,
adapter capabilities, or parity tests:

- `docs/plans/platform-completion-plan.md`
- `docs/parity/complete-feature-matrix.md`
- `docs/parity/adapter-capability-matrix.md`
- `docs/guides/master-scorecard.md`
- `docs/guides/adapter-capability-policy.md`
- `docs/guides/reconciliation-states.md`
- `docs/parity/adapter-live-test-policy.md`
- `docs/parity/complete-quality-gate.json`
- `testsuite/nautilus_master_tester.go`
- `scripts/verify_nautilus_parity.sh`

The local NautilusTrader reference code is under:

- `.omx/references/nautilus_trader/nautilus_trader`

Use the reference to understand behavior and workflows. Do not copy upstream
code. Implement clean-room Go behavior, then prove it with repository tests.

## Current Project State

Be transparent: this repository already has many first-pass runtime pieces, but
the feature matrix still labels many surfaces as `Partial` or `Planned`.

Current owner map:

- `model`: partial coverage for identifiers, instruments, orders, reports,
  events, command metadata, order lists, and data request types.
- `cache`: planned/expanding authoritative runtime state for orders, fills,
  positions, accounts, instruments, market data, residuals, and snapshots.
- `kernel` and `bus`: planned/expanding component lifecycle, clocks, health,
  message routing, and publish/subscribe behavior.
- `data`: planned/expanding live subscriptions, historical requests, catalogs,
  market events, aggregation, and health.
- `execution` and `account`: partial coverage for command routing, order
  lifecycle, matching/emulation, reconciliation, reports, TradingAccount, and
  order tracking.
- `risk`: partial coverage for pre-trade checks, async queue behavior, limits,
  trading state, reduce-only, throttles, and kill switch.
- `portfolio`: partial coverage for balances, positions, commissions,
  realized/unrealized PnL, exposure, conversion, and analyzer hooks.
- `strategy`: planned/expanding typed callbacks, runtime helpers, order
  factory, subscriptions, timers, cache, and portfolio access.
- `backtest`: partial coverage for deterministic simulation, matching, order
  lists, fees, slippage, latency, and reproducibility.
- `live` and `platform`: partial coverage for node assembly, retry, reconnect,
  shutdown, health, and runtime wiring.
- `adapter/*` and `sdk/*`: partial SDK-backed exchange coverage. Capability
  truth lives in `venue.DeclaredCapabilities` and the adapter matrix, not in
  broad README claims.
- `testsuite`: the parity oracle. Shared tests and scorecards are required for
  cross-package claims.

The current repository adapter universe is Binance, Aster, OKX, Bybit, Bitget,
Hyperliquid, Lighter, Nado, EdgeX, GRVT, StandX, and Backpack. Nautilus
reference adapters outside this SDK universe, such as Interactive Brokers,
Databento, Betfair, Deribit, Kraken, dYdX, BitMEX, Polymarket, Tardis, and
sandbox-style providers, are extension targets unless SDK modules, adapters,
and contract tests exist.

## Master Scorecard

The master scorecard totals 1000 points and is defined in
`testsuite.NautilusMasterRequirements()`.

| Domain | Points | Core expectation |
| --- | ---: | --- |
| Domain model and identifiers | 90 | Types, validation, reports, commands, events, instruments, and data round-trip. |
| Cache and state indexes | 80 | Runtime state can answer order, fill, position, account, instrument, and residual queries. |
| Command envelope and message bus | 70 | Command IDs, correlation, trader, strategy, client, timestamp, params, and position/list IDs survive all paths. |
| Strategy runtime and UX | 70 | One strategy can use typed callbacks, timers, order factory, cache, portfolio, and lifecycle hooks. |
| Data engine and catalog | 80 | Historical/replay data and live subscriptions share normalized data semantics. |
| Execution engine and lifecycle | 130 | Submit, modify, cancel, query, order lists, reports, contingencies, emulation, fills, positions, and lifecycle transitions match reference behavior. |
| Reconciliation | 90 | Startup, periodic, reconnect, mass-status, missing fill, external order, and discrepancy repair paths are explicit and auditable. |
| Risk engine | 70 | Risk rejects before execution for invalid instruments, precision, exposure, margin, trading state, reduce-only, throttles, and kill switch. |
| Portfolio/accounting | 90 | Accounts, balances, positions, commissions, PnL, exposure, conversion, snapshots, and cache invalidation are correct. |
| Backtest engine | 80 | Deterministic venue loop, matching, advanced orders, fees, slippage, latency, and reproducibility pass. |
| Live node/runtime | 60 | Config, wiring, retry, reconnect, shutdown, health, and observability are complete. |
| Adapters and SDK parity | 70 | Every claimed venue capability is SDK-backed and contract-tested. |
| Documentation and examples | 20 | Guides, examples, reports, matrices, and release notes stay executable and honest. |

Required release status is 1000/1000 for mandatory scope and zero critical
blockers. A missing, failed, or skipped required case earns no credit.

## Golden Scenarios

Every large refactor must preserve these scenarios:

- Scenario A: bracket strategy round trip. Entry fills, contingent children
  release, take-profit fills, stop-loss sibling cancels, final position is flat,
  command metadata survives, and PnL is correct after fees.
- Scenario B: reconnect with missing fill. Private stream health changes,
  reconnect and resubscribe run, missing fill is queried and applied once, and
  local state converges with venue state.
- Scenario C: position discrepancy. Stale local position is detected, recent
  activity thresholds and retry limits are respected, and unresolved state is
  explicit instead of silently accepted.
- Scenario D: risk and portfolio safety. Risk rejects before adapter
  submission, emits a typed rejection lifecycle event, and prevents order,
  position, fill, and portfolio mutation.
- Scenario E: adapter capability honesty. Optional adapter cases skip only when
  capability is false; true claims must pass matching contract tests.

Do not weaken the scenarios, loosen assertions, or reclassify required cases as
optional merely to pass local tests.

## Architecture Boundaries

The repository has three hard layers. Keep dependencies flowing downward only:

1. Runtime uses adapter and venue abstractions.
2. Adapter uses SDK and maps into runtime model types.
3. SDK talks to exchange protocols and must not import adapter/runtime packages.

### SDK Layer: Venue-Native Protocols

The exchange-local `sdk/` packages are the native protocol layer.

Allowed:

- official REST and WebSocket endpoint names;
- venue-specific request/response structs;
- venue product concepts and signing flows;
- public read tests against real official public endpoints;
- private read tests gated by credentials;
- live write tests gated by explicit write flags and credentials.

Forbidden:

- importing adapter or runtime packages;
- hiding unsupported endpoint behavior behind no-op success;
- adding fake HTTP or fake WebSocket tests for API-method coverage unless the
  method is a pure parser, signer, local dispatcher, or otherwise has no
  exchange API side effect;
- running mutating live tests by default.

SDK additions do not imply adapter support. Add the SDK first, then separately
decide whether a stable adapter abstraction exists.

### Adapter Layer: Stable Convenience Surface

The `adapter/` and `venue/` packages are stable facades over SDK clients.

Adapters own:

- instrument and market resolution;
- quote-aware and instrument-aware routing;
- exchange-native to `model` mapping;
- order validation and error mapping;
- REST and WebSocket convenience methods;
- honest `venue.DeclaredCapabilities`;
- explicit unsupported errors for unimplemented behavior.

Adapters must not become mirrors of every official API endpoint. Use optional
capability interfaces for capability families such as modify, batch orders,
order lists, mass status, fill reports, position reports, resubscribe, account
bills, transfers, or venue-specific risk controls.

Base-symbol-only methods are legacy convenience. New architecture work should
prefer `model.InstrumentID`, quote-aware symbols, or explicit market references.

### Runtime Layer: Trading Lifecycle

Runtime packages implement Nautilus-style trading behavior:

- `model`: strongly typed identifiers, instruments, commands, data requests,
  account snapshots, orders, order lists, lifecycle events, fills, positions,
  and reports.
- `cache`: authoritative local state and indexes for instruments, accounts,
  orders, fills, deferred fills, positions, market data, snapshots, and
  residual checks.
- `kernel`: component states, health, clocks, and lifecycle semantics.
- `bus`: event fanout and command/event delivery.
- `data`: normalized market data requests, subscriptions, catalog/replay,
  aggregation, and data health.
- `execution`: command routing, order manager, matching, emulation, contingent
  orders, reports, lifecycle events, and reconciliation.
- `account`: TradingAccount readiness, order tracking, private stream handling,
  normalized order/fill reports, and stream health.
- `risk`: pre-execution controls and rejection events.
- `portfolio`: account, position, balance, PnL, exposure, conversion, and
  analyzer state.
- `strategy`: author-facing runtime, typed callbacks, order factory, timers,
  subscriptions, cache, portfolio, and lifecycle hooks.
- `backtest`: deterministic simulated execution and replay.
- `live`: live node configuration, reconnect, retry, shutdown, and health.
- `platform`: high-level node wiring across data, execution, risk, portfolio,
  strategy, and adapters.
- `testsuite`: reusable test contracts and release scorecards.

Runtime behavior should be shared between backtest and live wherever the parity
contract says semantics are shared. Do not let live-only shortcuts bypass risk,
portfolio, cache, lifecycle, or command metadata.

## Core Invariants

### Identity And Commands

- Preserve `TraderID`, `StrategyID`, `AccountID`, `ClientOrderID`, `VenueOrderID`,
  `PositionID`, order-list IDs, correlation IDs, command IDs, timestamps, and
  arbitrary params through strategy, risk, execution, adapter, report, event,
  cache, and portfolio paths.
- Never synthesize or drop identifiers silently. If a venue lacks an identifier,
  record the missing mapping explicitly.
- Command metadata must survive rejections, emulated order release, order-list
  child release, reconciliation repair, and backtest matching.

### Lifecycle And Reconciliation

- Order state transitions must follow explicit state-machine rules.
- Startup readiness requires snapshot/reconciliation over the execution client,
  not merely an adapter object existing.
- Private stream disconnects must affect health and trigger replay or
  reconciliation according to the owning component.
- Missing fills, fill-before-order, external orders, venue-ID-only reports, open
  state mismatches, filled quantity mismatches, and stale positions must be
  structured reconciliation outcomes.
- An unresolved discrepancy is product state, not a log-only warning.
- Repair logic must be idempotent. Applying a fill twice is a critical bug.

### Risk

- Risk checks run before adapter submission unless a documented test harness
  explicitly bypasses execution for unit isolation.
- Rejected risk must emit a typed lifecycle/rejection result with command
  metadata.
- Risk rejection must not mutate open orders, positions, fills, or portfolio
  state.
- Risk must consider instrument validity, precision, order type, notional,
  exposure, position limits, account/strategy/instrument scoped limits, margin,
  trading state, reduce-only constraints, duplicate client IDs, throttles, queue
  capacity, and kill switch behavior where the relevant case is in scope.

### Portfolio

- Portfolio is event-driven and cache-aware. It should not invent state that
  execution, account, or cache cannot justify.
- Fills must update positions, commissions, realized PnL, balances, and
  invalidated unrealized PnL consistently.
- Unrealized PnL depends on current marks and conversion rates; stale marks must
  not be presented as fresh certainty.
- Exposure should be computable by instrument, account, venue, and target
  currency when required by the scorecard.
- Risk and portfolio must agree on signed position and exposure semantics.

### Backtest And Live Equivalence

- Backtest and live should share command, event, risk, portfolio, cache, and
  lifecycle semantics.
- Backtest may use simulated matching, fees, slippage, latency, and deterministic
  clocks, but it must not use a simplified lifecycle that strategies cannot see
  in live mode.
- Live mode may add retries, reconnects, private stream health, and venue
  reconciliation, but it must not skip core risk and portfolio flows.

## Capability Honesty

Capability declarations are product promises.

- `venue.DeclaredCapabilities` must match implemented and tested behavior.
- `Yes` in the adapter capability matrix means required contract tests must
  pass.
- `No` means callers receive explicit unsupported behavior where applicable.
- `Planned` means implementation target, not current support.
- `External` means outside the current repository SDK universe.
- Private stream and resubscribe are separate claims.
- Full lifecycle readiness also requires reconciliation evidence.
- Fill reports, position reports, mass status, order lists, query, modify, and
  batch support must be SDK-backed before they can be claimed.
- No adapter can claim lifecycle readiness without private execution stream
  support and reconciliation evidence.

If implementation and matrix disagree, fix the code, tests, and matrix together.

## Development Workflow For Agents

### Before Editing

- Read the relevant package code, package tests, and the corresponding
  `testsuite` owner.
- Read the matching section of the complete replica plan and feature matrix.
- For adapter work, read the adapter capability matrix, capability policy, and
  live test policy.
- For lifecycle work, read reconciliation states and existing state-machine
  tests.
- For risk or portfolio work, read both packages because their exposure and
  position semantics interact.
- Prefer `rg` and `rg --files` for search.
- Identify the smallest test that can prove the intended behavior.

### While Editing

- Keep changes scoped to the owning package and required contracts.
- Prefer existing patterns, structs, helper APIs, and test style.
- Add abstractions only when they remove real duplication or encode a stable
  cross-package contract.
- Keep SDK, adapter, and runtime responsibilities separated.
- Use `apply_patch` for manual edits.
- Do not revert user changes unless explicitly asked.
- Do not run destructive git commands unless the user explicitly requested that
  destructive operation.
- Do not add dependencies unless they are necessary and fit the architecture.
- Do not preserve stale links or stale policy references. Repair them.

### After Editing

- Run targeted package tests first.
- Run the relevant shared `testsuite` cases when behavior crosses packages.
- Run `git diff --check`.
- For broad lifecycle, risk, portfolio, adapter, or release-gate work, run or
  explicitly report the gap for `bash scripts/verify_nautilus_parity.sh`.
- Update docs, matrices, scorecards, and examples when capability claims or user
  workflows change.
- Report what was verified and what was not verified.

## Change Protocols By Area

### Model And Cache

- Add tests for validation, serialization, and indexes.
- Preserve decimal precision and avoid float-based trading math.
- Cache queries needed by execution, risk, portfolio, strategy, and
  reconciliation must be indexed or otherwise justified.
- Deferred fills and residual state must remain inspectable.

### Execution And Account

- Preserve lifecycle state-machine legality.
- Keep command metadata through submit, modify, cancel, query, emulation,
  order-list release, venue reports, and reconciliation.
- Execution reports should normalize venue behavior without hiding uncertainty.
- TradingAccount readiness must be based on lifecycle-critical capabilities:
  account snapshot, order placement/cancel/query, order reports, optional fill
  reports, and market-specific balance/position reports.
- Funding, open interest, historical analytics, or venue admin APIs must not
  become TradingAccount startup requirements.

### Reconciliation

- Reconciliation results must include case ID, counters, timestamps, unresolved
  discrepancies, and audit trail state.
- Do not turn discrepancy handling into logs only.
- Retry limits and recent activity thresholds must be explicit and tested.
- Missing fills and external orders must be idempotent.

### Risk

- Tests must cover both success and rejection paths.
- Rejection must be typed and matchable with `errors.Is` where appropriate.
- Async queue behavior, kill switch, reduce-only, trading state, throttle, and
  scoped limit changes require tests.
- Risk must run before adapter submission in normal strategy/runtime paths.

### Portfolio

- Tests must cover long, short, closing, flipping, commissions, balance deltas,
  mark updates, conversion, cache invalidation, and aggregation scopes when the
  change touches those behaviors.
- Do not compute exposure from stale or missing instrument metadata without an
  explicit unsupported or unavailable result.
- Keep realized and unrealized PnL semantics explicit.

### Strategy

- Strategy-facing APIs should be ergonomic but not magical.
- Typed callbacks must preserve event order assumptions required by tests.
- Runtime helpers should route through platform/risk/execution rather than
  bypassing the lifecycle.
- Examples must remain runnable when public docs mention them.

### Data, Backtest, Live, And Platform

- Data subscriptions and historical requests must use normalized model types.
- Backtest determinism matters. Preserve deterministic clocks and stable replay
  order.
- Live reconnect and retry behavior must surface health, not hide failures.
- Platform nodes must wire data, execution, risk, portfolio, cache, and strategy
  consistently.

### SDK And Adapter Work

- Start with SDK protocol correctness.
- Add adapter exposure only when there is a stable cross-exchange abstraction.
- Update `docs/parity/adapter-capability-matrix.md` for capability
  changes.
- Add or update adapter contract tests for every true capability.
- Keep mutating live tests behind exchange-specific enable flags.
- Return `model.ErrNotSupported` or a wrapped equivalent for unsupported
  adapter behavior.

### Documentation And Examples

- Documentation must be evidence-backed and current.
- Do not claim unsupported adapters or incomplete lifecycle semantics.
- Examples should compile or have a clearly documented reason if they are
  illustrative only.
- Public-facing docs should point readers to gates, matrices, and examples.
- Agent-facing docs should state constraints and current gaps plainly.

## Testing Policy

Default tests should be practical Go tests: colocated, method-named, and easy
to review.

SDK tests:

- Every SDK source file should have a corresponding `_test.go` file when it
  exposes public behavior.
- Every public SDK API method should have a directly named test where practical.
- Public read-method tests may call real official public endpoints by default.
- Private read tests must use credential gates and skip clearly when credentials
  are missing.
- Live write tests must never execute by default. They must use
  `internal/testenv.RequireLiveWrite`, exchange-specific enable flags, and
  required credentials.

Runtime and adapter tests:

- Add regression tests before changing lifecycle, reconciliation, risk,
  portfolio, order-state, command metadata, or capability behavior.
- Prefer package-local tests first, then shared `testsuite` cases.
- Use race tests for concurrency-sensitive cache, bus, execution, live,
  account, reconciliation, and stream-health work.

Useful commands:

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./testsuite -run 'TestNautilusMaster'
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./examples/...
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./venue ./testsuite ./adapter/... ./config/all -run 'Adapter|Capability|Contract|PrivateStream|Resubscribe'
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./account ./live ./testsuite -run 'Reconciliation|MissingFill|MassStatus|ExternalOrder|Discrepancy|VenueOrder|FillBeforeOrder|OpenOrder|RepairDelay|Position|RetryLimit|Audit'
bash scripts/verify_nautilus_parity.sh
bash scripts/generate_nautilus_benchmark_report.sh
```

If a required verification cannot run, state why and run the next-best targeted
check. Do not claim passing evidence you did not collect.

## File And Git Safety

- You may be in a dirty worktree. Treat unknown changes as user changes.
- Never revert unrelated changes.
- Never use destructive commands such as `git reset --hard` or `git checkout --`
  unless explicitly requested.
- Keep generated or benchmark output out of commits unless the task explicitly
  asks for it or the artifact is part of the release gate.
- Use concise, reviewable diffs.
- Stage only intended files.

## Commit Protocol

Every commit message must follow the Lore protocol: a concise decision record
using git-native trailers.

Format:

```text
<intent line: why the change was made, not what changed>

<optional concise body: constraints and approach rationale>

Constraint: <external constraint that shaped the decision>
Rejected: <alternative considered> | <reason for rejection>
Confidence: <low|medium|high>
Scope-risk: <narrow|moderate|broad>
Directive: <forward-looking warning for future modifiers>
Tested: <what was verified>
Not-tested: <known gaps in verification>
```

Rules:

- Intent line first; describe why the change exists.
- Use trailers only when they add decision context.
- Use `Rejected:` for alternatives future agents should not re-explore.
- Use `Directive:` for warnings future modifiers must respect.
- Use `Not-tested:` for known verification gaps.
