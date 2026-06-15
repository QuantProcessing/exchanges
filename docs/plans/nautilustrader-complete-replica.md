# NautilusTrader Complete Replica Implementation Plan

> **For agentic workers:** REQUIRED EXECUTION SKILL: Use `subagent-driven-development` (recommended for parallel lanes) or `executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build an idiomatic Go implementation that reaches behavioral parity with NautilusTrader core trading workflows and production execution semantics, while retaining this repository's SDK / adapter / TradingAccount layer boundaries.

**Architecture:** Treat NautilusTrader as the reference product contract, not as code to copy. `testsuite` becomes the measurable parity oracle; `model`, `cache`, `account`, `portfolio`, `risk`, `strategy`, `backtest`, `platform`, `live`, and `adapter/*` must pass shared Nautilus-style contract gates before any capability is claimed.

**Tech Stack:** Go 1.26, `shopspring/decimal`, repository-local SDKs under `sdk/`, reusable Go tests, race tests, adapter capability suites, and clean-room references from `.omx/references/nautilus_trader`.

---

## Scope Decision

This is a program plan, not a single patch plan. "Complete NautilusTrader" spans multiple independent subsystems and must be executed as several sub-project plans. The master plan below defines the result expectation, scoring model, and full task tree. Each epic must be split into a focused implementation plan before code changes begin.

### Mandatory Scope

1. Full core-platform behavioral parity for the local NautilusTrader reference surfaces visible under `.omx/references/nautilus_trader/nautilus_trader`.
2. SDK-backed adapters for every exchange already present in this repository: Binance, Aster, OKX, Bybit, Bitget, Hyperliquid, Lighter, Nado, EdgeX, GRVT, StandX, and Backpack.
3. Live and backtest execution paths must share command, event, risk, portfolio, cache, and lifecycle semantics.
4. Capability declarations must be test-backed; unsupported surfaces return explicit `ErrNotSupported` and cannot count toward parity.

### Extended Literal-Replica Scope

NautilusTrader includes adapters and data providers not currently backed by this repository's SDK layer, such as Interactive Brokers, Databento, Betfair, Deribit, Kraken, dYdX, BitMEX, Polymarket, Tardis, and sandbox-style providers. Literal venue parity requires either adding SDK modules for those providers or documenting them as unavailable in this repository's exchange universe. The complete core architecture must support them without redesign.

## Reference Evidence

- Current Go parity acceptance defines P0 suites for data, execution, lifecycle, backtest, portfolio, risk, and adapter capability honesty through `testsuite` and the matrices in `docs/parity/`.
- Existing clean-room architecture keeps `sdk/` as venue-native protocol code and rebuilds platform layers around `model`, `venue`, `bus`, `cache`, `account`, `strategy`, `platform`, `adapter`, and `testsuite`.
- NautilusTrader reference shows `OrderList`, `SubmitOrderList`, execution mass status, order/fill/position report generation, live reconciliation, matching core, live risk engine queues, and event-driven portfolio updates.
- Current Go project already has a first-pass implementation for `TradingAccount`, command metadata, bracket order lists, backtest matching, risk checks, portfolio accounting, and parity scoreboards. This plan extends that into full product parity.

## Result Expectation

The project is complete only when the following master statement is true:

> A strategy written once against the Go `strategy.Runtime` can run unchanged in backtest and live modes; submit bracket/list/advanced orders; receive typed order/fill/position/account/data callbacks; survive private-stream disconnects and startup gaps; reconcile missing fills/orders/positions without duplicate state; enforce risk before execution; compute portfolio state and PnL consistently; and pass a reusable Nautilus parity scorecard for every claimed adapter capability.

### Master Scorecard

Total: 1000 points. Release status requires 1000/1000 for mandatory scope and zero critical blockers.

| Domain | Points | Required Result |
| --- | ---: | --- |
| Domain model and identifiers | 90 | All Nautilus-equivalent identifiers, quantities, money, order, position, account, instrument, data, command, report, and event types exist with validation and round-trip tests. |
| Cache and state indexes | 80 | Orders, fills, positions, accounts, instruments, data, snapshots, external IDs, deferred fills, and residual checks are indexed and queryable like a trading runtime source of truth. |
| Command envelope and message bus | 70 | Commands/events carry trader, strategy, client, command, correlation, timestamp, params, and position/list identifiers through live/backtest/adapter paths. |
| Strategy runtime and UX | 70 | Typed callbacks, runtime helpers, order factory, subscriptions, timers, cache/portfolio access, lifecycle hooks, and examples match Nautilus authoring ergonomics. |
| Data engine and catalog | 80 | Historical/replay data, live subscriptions, ticks, bars, books, aggregation, unsubscribe, and data request paths are normalized and tested. |
| Execution engine and lifecycle | 130 | Submit/modify/cancel/query/list commands, order manager, contingent orders, emulation, state machine, fills, positions, residuals, and private streams match Nautilus lifecycle behavior. |
| Reconciliation | 90 | Startup, periodic, reconnect, mass-status, missing-fill, external-order, fill-before-order, venue-id-only, filled-quantity mismatch, retry, and race-threshold logic are implemented. |
| Risk engine | 70 | Synchronous and asynchronous risk paths enforce instrument, precision, order, exposure, margin, trading state, reduce-only, throttling, kill switch, and queue health rules. |
| Portfolio/accounting | 90 | Account updates, balances, positions, commissions, realized/unrealized PnL, exposure, currency conversion, snapshots, analyzer hooks, and cache invalidation are correct. |
| Backtest engine | 80 | Deterministic venue loop, matching core, advanced orders, list/bracket/contingent orders, fill model, fees, slippage, latency, cascading same-timestamp commands, and reproducibility gates pass. |
| Live node/runtime | 60 | Config, node builder, data/execution/risk/portfolio wiring, retry, reconnect, shutdown, health, observability, and lifecycle tests are complete. |
| Adapters and SDK parity | 70 | Every repository exchange exposes only SDK-backed, contract-tested capability claims for data, execution, private stream, account, fill, and position support. |
| Documentation and examples | 20 | Nautilus-style examples, migration notes, capability docs, and result reports are maintained and executable. |

## Golden Scenarios

These scenarios are the acceptance backbone. Every epic must preserve them.

### Scenario A: Bracket Strategy Round Trip

Input:
- Instrument: `BTC-USDT-PERP.BINANCE`.
- Entry: buy 1 at 101.
- Take profit: sell 1 at 103, reduce-only.
- Stop loss: sell 1 at 99, reduce-only.
- Market path: entry fills, take-profit fills, stop-loss sibling is canceled.

Expected event sequence:
1. Strategy receives market data and submits one `OrderList`.
2. Entry emits submitted/accepted/filled lifecycle events.
3. Child orders are released only after parent fill.
4. Take-profit fill emits order fill plus position close.
5. Stop-loss sibling emits pending-cancel/canceled.
6. Command metadata survives from strategy to reports and lifecycle events.
7. Final position is flat.
8. Realized PnL equals expected gross PnL minus configured fees.
9. Scoreboard cases for order list, lifecycle, portfolio, risk, backtest, and execution all pass.

### Scenario B: Reconnect With Missing Fill

Input:
- Local cache has open order quantity 1.
- Private stream disconnects after venue fills 0.4.
- Venue reports include the missing fill and updated order status.

Expected result:
1. Stream health flips not-ready while closed.
2. Runtime reconnects and resubscribes private execution stream.
3. Gap reconciliation queries order, fill, and position reports.
4. Missing fill is applied once, never duplicated.
5. Order filled quantity and position quantity converge to venue state.
6. Reconciliation health records success; unresolved discrepancy records retry evidence.

### Scenario C: Position Discrepancy

Input:
- Cache has a stale long position.
- Venue position report is flat.
- Fill report query returns no missing fills.

Expected result:
1. Runtime detects cached-vs-venue position discrepancy.
2. Recent local activity threshold prevents premature repair.
3. Retry counter bounds repeated reconciliation.
4. If configured to generate missing orders, runtime creates reconciliation events; otherwise it reports an explicit unresolved discrepancy.
5. Portfolio and cache end in a consistent state or a visible blocked state, never silent success.

### Scenario D: Risk And Portfolio Safety

Input:
- Strategy tries to submit orders exceeding max notional, projected account exposure, and reduce-only constraints.

Expected result:
1. Risk rejects before adapter submission.
2. Rejection emits order denied/rejected lifecycle with command metadata.
3. No open order, position, fill, or portfolio mutation occurs after rejected risk.
4. Error is typed and can be matched with `errors.Is`.

### Scenario E: Adapter Capability Honesty

Input:
- An adapter claims private stream, fill reports, or position reports.

Expected result:
1. Shared tests require the exact corresponding cases to pass.
2. Capability false means skipped optional case; capability true means required pass.
3. No adapter can claim lifecycle readiness without a private stream and reconciliation evidence.

## Epic 0: Parity Inventory And Gating

**Purpose:** Make "complete" measurable before new implementation work.

**Files:**
- Modify: `testsuite/scoreboard.go`
- Modify: `testsuite/contracts.go`
- Create: `testsuite/nautilus_master_tester.go`
- Create: `testsuite/nautilus_master_tester_test.go`
- Create: `docs/parity/nautilustrader-complete-feature-matrix.md`
- Create: `docs/parity/adapter-capability-matrix.md`

**Tasks:**
- [x] Define a `NautilusMasterRequirements()` matrix covering all scorecard domains and scenario IDs.
- [x] Add required case groups for model, cache, command, data, execution, reconciliation, risk, portfolio, backtest, live, and adapter suites.
- [x] Add a test that fails if a required suite has no report source.
- [x] Add a feature matrix mapping every visible Nautilus reference surface to a Go package, status, and acceptance test.
- [x] Add adapter capability matrix rows for every current exchange/product surface.
- [x] Add CI-friendly command examples for required and optional live-gated tests.

**Acceptance:**
- `go test -count=1 ./testsuite -run 'TestNautilusMaster'` passes.
- `NautilusMasterRequirements()` contains no empty suites.
- Feature matrix has no "unknown owner" entries.

**Verification evidence:**
- 2026-06-14: `go test -count=1 ./testsuite -run 'TestNautilusMaster'` passed.

## Epic 1: Domain Model Completion

**Purpose:** Make Go model expressive enough for Nautilus-equivalent workflows without adapter-specific leakage.

**Files:**
- Modify: `model/identifiers.go`
- Modify: `model/instrument.go`
- Modify: `model/order.go`
- Modify: `model/order_list.go`
- Modify: `model/order_event.go`
- Modify: `model/position_event.go`
- Modify: `model/account.go`
- Modify: `model/market_data.go`
- Create: `model/command.go`
- Create: `model/execution_report.go`
- Create: `model/mass_status.go`
- Create: `model/money.go`
- Create: `model/data_request.go`

**Tasks:**
- [x] Add missing command identifiers: `PositionID`, `ExecAlgorithmID`, `ExecSpawnID`, `VenuePositionID`, `ComponentID`, and typed UUID wrapper where needed.
- [x] Add command types for submit/list/modify/cancel/cancel-all/batch-cancel/query-account/query-order/generate-order-report/generate-fill-report/generate-position-report/generate-mass-status.
- [x] Add `ExecutionMassStatus` with account, order, fill, and position report collections.
- [x] Add order-list semantics matching Nautilus: bulk, OTO, OCO, OUO, bracket classifier, uniform-instrument detection, multi-instrument same-venue validation.
- [x] Add complete instrument taxonomy for repository-supported products and a future-safe extension for futures, options, spreads, synthetic, index, equity, and betting-like instruments.
- [x] Add data request/response types for historical bars, ticks, quote ticks, trade ticks, books, and custom data.
- [x] Add serialization tests for all command/report/event types.

**Acceptance:**
- `go test -count=1 ./model -run 'Command|MassStatus|OrderList|Instrument|Serialization'` passes.
- Golden bracket scenario can represent Nautilus OUO child semantics without abusing OCO fields.
- All existing callers compile without losing current metadata behavior.

**Progress evidence:**
- 2026-06-14: `go test -count=1 ./model -run 'TestReportGenerationCommands|TestSubmitOrderList|TestExecutionMassStatus'` passed.
- 2026-06-14: `go test -count=1 ./model -run 'TestOrderListClassifies|TestOrderListAllows'` passed.
- 2026-06-14: `go test -count=1 ./model -run 'TestInstrumentTaxonomy|TestInstrumentValidateAllowsFutureSafe|TestDataRequest|TestDataResponse|TestCustomData|TestCommandReportAndDataTypesRoundTripJSON'` passed.
- 2026-06-14: `go test -count=1 ./model -run 'Command|MassStatus|OrderList|Instrument|Serialization'` passed.
- 2026-06-14: `go test -count=1 ./model` passed.

## Epic 2: Cache And Runtime State

**Purpose:** Make cache the authoritative state source required by execution, risk, portfolio, and reconciliation.

**Files:**
- Modify: `cache/cache.go`
- Modify: `cache/cache_test.go`
- Create: `cache/orders.go`
- Create: `cache/fills.go`
- Create: `cache/positions.go`
- Create: `cache/accounts.go`
- Create: `cache/instruments.go`
- Create: `cache/market.go`
- Create: `cache/indexes.go`
- Create: `cache/residuals.go`
- Create: `cache/snapshots.go`
- Create: `testsuite/cache_tester.go`
- Create: `testsuite/cache_tester_test.go`

**Tasks:**
- [x] Split cache into focused files while preserving public constructors.
- [x] Add order indexes by account/order ID, account/client ID, venue order ID, strategy ID, position ID, order list ID, exec spawn ID, and open/closed state.
- [x] Add fill indexes by account/order/trade ID and venue order ID.
- [x] Add deferred fill storage for fill-before-order scenarios.
- [x] Add position indexes by account/position ID, account/instrument, strategy, venue position ID, and open/closed state.
- [x] Add account event history and account snapshots.
- [x] Add residual checks for open orders, positions, pending deferred fills, and inconsistent ID mappings.
- [x] Add cache snapshot/purge behavior for closed orders/positions/account events.

**Acceptance:**
- `go test -count=1 ./cache ./testsuite -run 'Cache|Residual|DeferredFill|Snapshot'` passes.
- Cache can answer every query required by risk, portfolio, execution manager, and strategy runtime without scanning unrelated global state.

**Progress evidence:**
- 2026-06-14: `go test -count=1 ./cache -run 'TestCacheIndexesOrdersForRuntimeQueries|TestCacheIndexesFillsDeferredFillsAndPositions|TestCacheKeepsAccountSnapshotHistory'` passed.
- 2026-06-14: `go test -count=1 ./cache -run 'TestCacheSnapshotCapturesRuntimeState|TestCachePurgeKeepsNewestClosedStateAndClearsIndexes'` passed.
- 2026-06-14: `go test -count=1 ./testsuite -run 'TestCacheTesterReportsRuntimeStateCases'` passed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./cache ./testsuite -run 'TestCache|TestNautilusMaster|Test.*Cache' -v` passed after `cache/cache.go` was reduced to the `Cache` type and constructor, with instruments, accounts, orders, fills, positions, residuals, market data, and index helpers split into focused files.
- 2026-06-14: `go test -count=1 ./cache` passed.

## Epic 3: Component Kernel, Clock, And Message Bus

**Purpose:** Move from package-local orchestration toward Nautilus-style engines with consistent lifecycle, clock, and command/event routing.

**Files:**
- Modify: `bus/bus.go`
- Modify: `strategy/timer.go`
- Modify: `platform/node.go`
- Create: `kernel/component.go`
- Create: `kernel/clock.go`
- Create: `kernel/msgbus.go`
- Create: `kernel/health.go`
- Create: `kernel/component_test.go`

**Tasks:**
- [x] Add component states: initialized, starting, running, stopping, stopped, degraded, faulted.
- [x] Add live and test clocks with nanosecond timestamps and deterministic scheduling.
- [x] Add endpoint-style request/response and topic publish/subscribe surfaces on top of the existing bus.
- [x] Add bounded queues with observable drops/backpressure metrics.
- [x] Add lifecycle hooks for start, stop, kill, graceful shutdown, and fault propagation.
- [x] Migrate platform, live, risk, and execution engines onto the kernel interfaces incrementally.

**Acceptance:**
- `go test -count=1 ./kernel ./bus ./strategy ./platform -run 'Component|Clock|Bus|Lifecycle'` passes.
- Engine shutdown cannot leave unclosed queues or goroutines in race tests.

**Progress evidence:**
- 2026-06-14: `go test -count=1 ./kernel -run 'Component|Clock|Bus'` passed.
- 2026-06-14: `go test -count=1 ./kernel ./bus ./strategy ./platform -run 'Component|Clock|Bus|Lifecycle'` passed.
- 2026-06-15: `go test -count=1 ./platform -run 'TestNodeHealthTracksKernelLifecycleState|TestNodeHealthFaultsWhenStartupFails'` passed.
- 2026-06-15: `go test -count=1 ./kernel ./bus ./strategy ./platform -run 'Component|Clock|Bus|Lifecycle'` passed.
- 2026-06-15: `go test -count=1 ./risk -run 'TestEngineHealthTracksKernelLifecycleState'` passed.
- 2026-06-15: `go test -count=1 ./platform -run 'TestNodeDrivesConfiguredRiskEngineLifecycle'` passed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution -run '^TestEngineHealthTracksKernelLifecycleState$' -v` first failed because `execution.Health` did not expose kernel component state; it passed after `execution.Engine` was backed by `kernel.Component` hooks for client connect/disconnect, health state, and start/stop counters.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./testsuite -run 'Test(EngineHealthTracksKernelLifecycleState|ExecutionEngineTester|NautilusMaster)' -v` passed after `TC-EXENG35` made execution kernel lifecycle health part of the master execution-engine parity path.
- 2026-06-15: `go test -count=1 ./kernel ./bus ./strategy ./platform ./risk ./live -run 'Component|Clock|Bus|Lifecycle|Health|Risk'` passed.
- 2026-06-15: `go test -count=1 $(go list ./... | grep -v '/sdk')` passed.
- 2026-06-15: `go test -count=1 ./live -run 'TestRunnerHealthTracksKernelLifecycleState|TestTradingNodeHealthIncludesPlatformRiskLifecycle'` passed.
- 2026-06-15: `go test -count=1 ./kernel ./bus ./strategy ./platform ./risk ./live -run 'Component|Clock|Bus|Lifecycle|Health|Risk|Runner|TradingNode'` passed.

## Epic 4: Strategy Runtime And User Experience

**Purpose:** Let Go strategy authors use Nautilus-like ergonomics while staying idiomatic Go.

**Files:**
- Modify: `strategy/engine.go`
- Modify: `strategy/typed.go`
- Modify: `strategy/timer.go`
- Modify: `model/order_factory.go`
- Create: `strategy/config.go`
- Create: `strategy/actor.go`
- Create: `strategy/indicator.go`
- Create: `examples/nautilus_style/bracket_strategy.go`

**Tasks:**
- [x] Complete typed callbacks for data, order lifecycle, fills, positions, accounts, timers, custom data, and errors.
- [x] Add strategy config validation and immutable runtime identity.
- [x] Add runtime helpers for subscriptions, requests, order creation, portfolio, cache, clock, timers, logging, and command metadata.
- [x] Add strategy actor isolation so one strategy fault cannot silently corrupt another.
- [x] Add indicator interface and minimal built-in examples required by Nautilus demos, such as EMA/ATR.
- [x] Add a Go version of Nautilus bracket strategy example using the same live/backtest strategy implementation.

**Progress Evidence:**
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./examples/nautilus_style -v` passed after `examples/nautilus_style.BracketStrategy` ran unchanged in a backtest harness with OTO/OCO bracket completion and in a live node harness with strategy command metadata.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./strategy -run 'TestEngine(IsolatesAsyncStrategyActors|ReportsStrategyActorErrorsWithoutSkippingPeers)' -v` first failed because the global async dispatcher blocked peers behind a slow/failing strategy; it passed after async dispatch moved to per-strategy actor queues while preserving serial callbacks inside each strategy.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./strategy ./testsuite -run 'Actor|StrategyTester|NautilusParityRequirements|NautilusMaster' -v` passed after `TC-S09` required strategy actor faults to surface without skipping peer strategies.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./strategy -run '^TestEnginePassesStrategyScopedLogger$' -v` first failed because `Runtime` had no `Logger()` helper; it passed after per-strategy logger decoration added frozen `trader_id` and `strategy_id` attributes.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./strategy -run '^TestEnginePassesCommandMetadataToDataRequests$' -v` first failed because `Runtime` had no `RequestData`; it passed after data request helpers inherited command metadata.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./strategy ./backtest ./platform ./testsuite -run 'RequestData|StrategyScopedLogger|RuntimeHelpers|StrategyTester|NautilusParityRequirements|NautilusMaster' -v` passed after backtest `RequestData` read cached market data, platform `RequestData` fetched/cached ticker and book snapshots, and `TC-S08` required request metadata plus strategy-scoped logging.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./strategy ./backtest ./platform ./live ./testsuite -run 'Engine|Runtime|RequestData|StrategyTester|NautilusParityRequirements|NautilusMaster|LiveNode|NodeRequest|BacktestRequest'` passed after runtime helper coverage landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./strategy -run 'Test(ExponentialMovingAverage|AverageTrueRange|IndicatorsReject)' -v` first failed with undefined `NewExponentialMovingAverage` and `NewAverageTrueRange`; it passed after `strategy.Indicator`, EMA, ATR, period validation, and bar-based ATR updates were added.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./strategy ./testsuite -run 'Indicator|StrategyTester|NautilusParityRequirements|NautilusMaster'` passed after `TC-S07` required built-in EMA/ATR indicator initialization and updates in the Nautilus parity matrix.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./strategy ./examples/... -run 'Typed|Strategy|Timer|Indicator|Bracket|Config|Identity|CustomData|Lifecycle'` passed after built-in indicator coverage landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -race -count=1 ./strategy ./testsuite` passed after built-in indicator coverage landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 $(env GOCACHE=/private/tmp/go-build-exchanges go list ./... | rg -v '/sdk/')` passed after built-in indicator coverage landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./strategy -run 'TestTypedStrategyDispatches(CustomDataCallbacks|NautilusLifecycleEvents)' -v` first failed because `CustomData` did not dispatch to typed strategies; it passed after `OnCustomData(context.Context, model.CustomData)` dispatch was added and order lifecycle coverage was expanded to all `OrderEventKind` values through `OnOrderLifecycle`.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./strategy ./testsuite -run 'Typed|StrategyTester|NautilusParityRequirements|NautilusMaster'` passed after strategy `TC-S01` required custom data callbacks and `TC-S02` required order lifecycle callbacks.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./strategy ./examples/... -run 'Typed|Strategy|Timer|Indicator|Bracket|Config|Identity|CustomData|Lifecycle'` passed after typed callback completion landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -race -count=1 ./strategy ./testsuite` passed after typed callback completion landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 $(env GOCACHE=/private/tmp/go-build-exchanges go list ./... | rg -v '/sdk/')` passed after typed callback completion landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./strategy -run 'Test(TypedStrategyConfigValidatesIdentity|EngineFreezesRuntimeIdentityAtAdd|EngineRejectsInvalidStrategyIdentity)' -v` first failed with undefined `StrategyConfig`, `NewTypedWithConfig`, and `WithTraderID`; it passed after strategy config validation, duplicate/empty identity checks, Add-time strategy identity freezing, and trader-id command metadata defaults were added.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./strategy ./testsuite -run 'StrategyConfig|EngineFreezes|EngineRejects|StrategyTester|NautilusParityRequirements|NautilusMaster'` passed with `TC-S06` requiring config validation and immutable runtime identity.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./strategy ./examples/... -run 'Typed|Strategy|Timer|Indicator|Bracket|Config|Identity'` passed after strategy config and immutable runtime identity landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -race -count=1 ./strategy ./testsuite` passed after strategy config and immutable runtime identity landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 $(env GOCACHE=/private/tmp/go-build-exchanges go list ./... | rg -v '/sdk/')` passed after strategy config and immutable runtime identity landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./strategy -run '^TestTypedStrategyDispatchesErrorCallbacks$' -v` first failed with undefined `ErrorEvent` / `TopicError`; it passed after `strategy.ErrorEvent`, `TopicError`, and typed `OnError(context.Context, ErrorEvent)` dispatch were added.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./strategy ./testsuite -run 'Typed|StrategyTester|NautilusParityRequirements'` passed with a new `testsuite.NewStrategyTester` covering `TC-S01..TC-S05`.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./strategy ./examples/... -run 'Typed|Strategy|Timer|Indicator|Bracket'` passed after typed error callbacks and strategy parity cases landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -race -count=1 ./strategy ./testsuite` passed after typed error callbacks and strategy parity cases landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 $(env GOCACHE=/private/tmp/go-build-exchanges go list ./... | rg -v '/sdk/')` passed after typed error callbacks and strategy parity cases landed.

**Acceptance:**
- `go test -count=1 ./strategy ./examples/... -run 'Typed|Strategy|Timer|Indicator|Bracket'` passes.
- One bracket strategy implementation runs under both `backtest` and `live` test harnesses.

## Epic 5: Data Engine And Data Catalog

**Purpose:** Match Nautilus data-client and data-engine semantics for live and backtest data.

**Files:**
- Modify: `venue/interfaces.go`
- Modify: `platform/node.go`
- Create: `data/engine.go`
- Create: `data/catalog.go`
- Create: `data/subscription.go`
- Create: `data/aggregation.go`
- Create: `data/replay.go`
- Create: `testsuite/data_engine_tester.go`

**Tasks:**
- [x] Introduce `data.Engine` for normalized data clients, subscriptions, requests, and fan-out.
- [x] Add historical data catalog abstraction for bars, ticks, quotes, trades, books, and custom records.
- [x] Add bar aggregation from ticks/quotes/trades where inputs support it.
- [x] Add subscribe/unsubscribe bookkeeping with idempotency and pending-subscription replay across engine restart/reconnect lifecycle.
- [x] Add data request correlation IDs and response routing.
- [x] Add data health metrics for connected clients, subscriptions, event counts, last event time, and last errors.
- [x] Wire `platform`, `live`, and `backtest` onto shared `data.Engine` paths instead of parallel request/stream implementations.
- [x] Add explicit stale-stream threshold policy and automatic reconnect replay tests.

**Progress Evidence:**
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./data -run 'TestEngine' -v` first failed with missing data engine/catalog APIs; it passed after `data.Engine`, `MemoryCatalog`, bar aggregation, subscription replay, request correlation, fan-out, cache writes, and health counters landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./testsuite -run '^TestDataEngineTesterReportsParityCases$' -v` first failed with undefined `NewDataEngineTester` / `DataEngineTesterConfig`; it passed after `testsuite.NewDataEngineTester` added `TC-DE01..TC-DE05`.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./data ./testsuite -run 'TestEngine|TestDataEngineTester|TestNautilusParityRequirements|TestNautilusMaster' -v` passed after `data-engine` requirements were added to the parity scoreboard and the master data scorecard.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./data ./platform ./live ./backtest ./testsuite -run 'DataEngine|Catalog|Subscription|Aggregation|Request|Health|NautilusParityRequirements|NautilusMaster' -v` passed after data engine contracts were linked with adjacent runtime request/health tests.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -race -count=1 ./data ./testsuite` passed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 $(env GOCACHE=/private/tmp/go-build-exchanges go list ./... | rg -v '/sdk/')` passed.
- 2026-06-15: `git diff --check` passed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./data ./platform ./backtest -run 'ReconnectsClosedStream|HealthMarksStale|DelegatesMarketDataLifecycle|ReplaysSharedDataCatalog' -v` first failed with missing `data.RetryPolicy`, `Config.StaleAfter`, `platform.Node.DataEngine`, `Health.DataEngine`, and `backtest.EngineConfig.DataCatalog`; it passed after shared DataEngine wiring, stale health, reconnect replay, and shared data catalog replay landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./data ./platform ./backtest ./testsuite -run 'TestEngine|DataEngineTester|DelegatesMarketDataLifecycle|ReplaysSharedDataCatalog|NautilusParityRequirements|NautilusMaster' -v` passed after `TC-DE06` and `TC-DE07` expanded DataEngine parity coverage.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./data ./platform ./live ./backtest ./testsuite -run 'DataEngine|MarketData|RequestData|Health|Subscription|ReplaysSharedDataCatalog|NautilusParityRequirements|NautilusMaster' -v` passed after platform/live/backtest were migrated to the shared data engine path.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./live ./testsuite -run 'TestRunnerGracefullyStopsOnFatalPlatformStreamException|TestLiveNodeTesterReportsParityCaseResults|TestParityScoreboardAggregatesLiveNodeGate' -v` first failed because platform health did not surface `data.Engine` fatal stream errors to the live runner; it passed after `platform.Health.LastError` folded in `DataEngine.LastError`.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./data ./platform ./live ./backtest ./testsuite` passed after shared DataEngine migration and fatal stream propagation landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -race -count=1 ./data ./platform ./live ./backtest ./testsuite` passed after shared DataEngine migration and fatal stream propagation landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 $(env GOCACHE=/private/tmp/go-build-exchanges go list ./... | rg -v '/sdk/')` passed after shared DataEngine migration and fatal stream propagation landed.
- 2026-06-15: `git diff --check` passed after shared DataEngine migration and fatal stream propagation landed.

**Acceptance:**
- `go test -count=1 ./data ./platform ./testsuite -run 'DataEngine|Catalog|Subscription|Aggregation'` passes.
- Backtest can consume catalog-backed data and live can consume streaming clients through the same strategy callbacks.

## Epic 6: Execution Engine And Order Manager

**Purpose:** Replace partial lifecycle orchestration with a full Nautilus-style execution engine.

**Files:**
- Modify: `account/state_machine.go`
- Modify: `account/reconciler.go`
- Modify: `account/trading_account.go`
- Modify: `platform/node.go`
- Create: `execution/engine.go`
- Create: `execution/client.go`
- Create: `execution/manager.go`
- Create: `execution/emulator.go`
- Create: `execution/reports.go`
- Create: `execution/commands.go`
- Create: `testsuite/execution_engine_tester.go`

**Tasks:**
- [x] Add `execution.Engine` package-level command owner with execution-client routing, submit/cancel/modify/query, and cache-backed report application.
- [x] Wire `platform` submit/cancel/modify/query order command paths onto `execution.Engine`.
- [x] Wire `backtest` order command paths onto `execution.Engine`.
- [x] Add `execution.Client` interface with submit/modify/cancel/query/report client boundaries.
- [x] Extend `execution.Client` for list submit, batch cancel, cancel-all, account query, fill reports, position reports, and mass-status capabilities.
- [x] Add `execution.Manager` for legal order transitions, command history, contingent OTO release, OCO cancellation, and order-list state.
- [x] Extend `execution.Manager` for OUO quantity reduction actions.
- [x] Add durable order-list state snapshots for members, held children, cached order reports, and OUO fill progress.
- [x] Extend `execution.Manager` for full order-list state transitions beyond current OTO/OCO/OUO action snapshots.
- [x] Add position ID determination for netting and hedging modes.
- [x] Add fill handling for duplicate trades, overfill checks, and partial/final fill order updates.
- [x] Add fill-before-order deferral/replay.
- [x] Add leg fills.
- [x] Add external order claim and import flows.
- [x] Add execution algorithm routing hooks for orders managed locally before venue submission.
- [x] Add price-triggered order emulator for bid/ask and last-price market-data release into venue submission.
- [x] Publish emulated, triggered, and released lifecycle events from the execution emulator through `Engine.Events()`.
- [x] Wire emulator to data-engine subscriptions for automatic trigger-feed management.
- [x] Add price-offset trailing stop market emulation for bid/ask trigger feeds.
- [x] Transform released emulated stop/MIT/trailing market and limit orders before venue submission.
- [x] Add tick and basis-point trailing offset type support with instrument tick-size lookup.
- [x] Add trailing stop limit emulation with limit-order release and preserved trigger lifecycle reports.
- [x] Add independent trigger instrument routing for emulated orders and platform trigger-feed subscriptions.
- [x] Add bid/ask order-book trigger-feed subscriptions and top-of-book trigger evaluation.
- [x] Add synthetic-instrument cache registry/lookup for emulated trigger instruments.
- [x] Add submit-time initial matching against cached trigger market data.
- [x] Add local cancel handling for held emulated orders.
- [x] Add local modify handling and rematching for held emulated orders.
- [x] Add local cancel-all handling for held emulated orders before venue cancel-all routing.
- [x] Add full matching-core parity for the Nautilus order emulator.
- [x] Add snapshot and purge behavior for closed orders and positions.

**Progress Evidence:**
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution -run 'TestManager' -v` first failed with undefined `NewManager`, `Config`, `ErrInvalidTransition`, and `ErrOverfill`; it passed after `execution.Manager` added submit-command caching, terminal-state transition guards, fill dedupe/overfill protection, netting/hedging position IDs, and OTO/OCO order-list actions.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution -run 'Test(Engine|Manager)' -v` first failed with undefined `NewEngine`, `EngineConfig`, and `ErrClientNotFound`; it passed after `execution.Engine` added account-client routing, connect/disconnect, submit/cancel/modify/query, manager-backed report application, and missing-client errors.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./testsuite -run 'TestExecutionEngineTesterReportsParityCases' -v` passed after `testsuite.NewExecutionEngineTester` added `TC-EXENG01..TC-EXENG08`.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./testsuite -run 'Test(Engine|Manager|ExecutionEngineTester|NautilusParityRequirements|NautilusMaster)' -v` passed after `execution-engine` requirements were added to parity scoreboard and master execution scorecard.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./testsuite ./platform ./backtest` passed after the first execution package slice landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -race -count=1 ./execution ./testsuite ./platform ./backtest` passed after the first execution package slice landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 $(env GOCACHE=/private/tmp/go-build-exchanges go list ./... | rg -v '/sdk/')` passed after the first execution package slice landed.
- 2026-06-15: `git diff --check` passed after the first execution package slice landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./platform -run '^TestNodeSubmitOrderDelegatesThroughExecutionEngine$' -v` first failed because `platform.Node` did not expose or use `ExecutionEngine`; it passed after `platform.Config.ExecutionEngine`, `Node.ExecutionEngine()`, execution-client registration, and submit delegation landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./platform -run 'TestNode(SubmitOrderDelegatesThroughExecutionEngine|CancelModifyQueryDelegateThroughExecutionEngine)' -v` passed after platform modify/query/cancel paths delegated through `execution.Engine` and `execution.Engine.Health()` command counters were added.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./platform ./testsuite -run 'Test(Engine|Manager|Node.*ExecutionEngine|ExecutionEngineTester|NautilusParityRequirements|NautilusMaster)' -v` passed after `TC-EXENG09` required platform command routing through `execution.Engine`.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./platform ./live ./backtest ./testsuite` passed after platform execution-engine routing landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -race -count=1 ./execution ./platform ./live ./backtest ./testsuite` passed after platform execution-engine routing landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 $(env GOCACHE=/private/tmp/go-build-exchanges go list ./... | rg -v '/sdk/')` passed after platform execution-engine routing landed.
- 2026-06-15: `git diff --check` passed after platform execution-engine routing landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./backtest -run '^TestBacktestRoutesOrderCommandsThroughExecutionEngine$' -v` first failed because `backtest.Result` had no execution health and runtime order commands bypassed `execution.Engine`; it passed after backtest added a local execution client and routed submit/modify/query/cancel through `execution.Engine`.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./testsuite -run 'Test(ExecutionEngineTester|NautilusParityRequirements|NautilusMaster)' -v` passed after `TC-EXENG10` made backtest execution-engine routing a required parity case.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./backtest ./platform ./testsuite -run 'Test(Engine|Manager|BacktestRoutesOrderCommandsThroughExecutionEngine|Node.*ExecutionEngine|ExecutionEngineTester|NautilusParityRequirements|NautilusMaster)' -v` passed after backtest execution-engine routing landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./platform ./live ./backtest ./testsuite` passed after backtest execution-engine routing landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -race -count=1 ./execution ./platform ./live ./backtest ./testsuite` passed after backtest execution-engine routing landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 $(env GOCACHE=/private/tmp/go-build-exchanges go list ./... | rg -v '/sdk/')` passed after backtest execution-engine routing landed.
- 2026-06-15: `git diff --check` passed after backtest execution-engine routing landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution -run 'TestEngine(RoutesCompositeCommandsAndAccountQuery|GeneratesReportsAndMassStatus)' -v` first failed with missing `Engine.SubmitOrderList`, `BatchCancelOrders`, `CancelAllOrders`, `QueryAccount`, report generation, and mass-status methods; it passed after execution engine composite command routing and report-generation APIs landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./testsuite -run 'Test(Engine|Manager|ExecutionEngineTester|NautilusParityRequirements|NautilusMaster)' -v` passed after `TC-EXENG11` and `TC-EXENG12` made composite execution commands and mass-status generation required parity cases.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./platform ./live ./backtest ./testsuite` passed after execution engine composite command and mass-status APIs landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -race -count=1 ./execution ./platform ./live ./backtest ./testsuite` passed after execution engine composite command and mass-status APIs landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 $(env GOCACHE=/private/tmp/go-build-exchanges go list ./... | rg -v '/sdk/')` passed after execution engine composite command and mass-status APIs landed.
- 2026-06-15: `git diff --check` passed after execution engine composite command and mass-status APIs landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution -run '^TestManagerReducesOUOSiblingsOnPartialFill$' -v` first failed because `OrderListActions` had no modify action; it passed after `execution.Manager` added OUO partial-fill sibling resize/cancel actions with duplicate progress suppression.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./testsuite -run 'Test(Manager|ExecutionEngineTester|NautilusParityRequirements|NautilusMaster)' -v` passed after `TC-EXENG13` made OUO sibling resize a required execution-engine parity case.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./platform ./live ./backtest ./testsuite` passed after OUO manager actions landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -race -count=1 ./execution ./platform ./live ./backtest ./testsuite` passed after OUO manager actions landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 $(env GOCACHE=/private/tmp/go-build-exchanges go list ./... | rg -v '/sdk/')` passed after OUO manager actions landed.
- 2026-06-15: `git diff --check` passed after OUO manager actions landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution -run '^TestManagerDefersFillUntilOrderReportArrives$' -v` first failed because missing-order fills returned `ErrInvalidTransition`; it passed after `execution.Manager` deferred fills into cache and replayed them when the matching order report arrived.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./testsuite -run 'Test(Manager|ExecutionEngineTester|NautilusParityRequirements|NautilusMaster)' -v` passed after `TC-EXENG14` made fill-before-order replay a required execution-engine parity case.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./platform ./live ./backtest ./testsuite` passed after fill-before-order replay landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -race -count=1 ./execution ./platform ./live ./backtest ./testsuite` passed after fill-before-order replay landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 $(env GOCACHE=/private/tmp/go-build-exchanges go list ./... | rg -v '/sdk/')` passed after fill-before-order replay landed.
- 2026-06-15: `git diff --check` passed after fill-before-order replay landed.
- 2026-06-15: Nautilus reference `.omx/references/nautilus_trader/nautilus_trader/execution/engine.pyx` and `live/execution_engine.py` show `instrument_id -> strategy_id` external order claims and unclaimed `EXTERNAL` strategy assignment for venue-originated orders.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution -run '^TestEngineClaimsExternalOrderReportsByInstrument$' -v` first failed with missing external claim APIs; it passed after `execution.Engine` added external claim registration/query and report import attribution.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./testsuite -run 'Test(Engine|Manager|ExecutionEngineTester|NautilusParityRequirements|NautilusMaster)' -v` passed after `TC-EXENG15` made external order claim/import a required execution-engine parity case.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./platform ./live ./backtest ./testsuite` passed after external order claim/import landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -race -count=1 ./execution ./platform ./live ./backtest ./testsuite` passed after external order claim/import landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 $(env GOCACHE=/private/tmp/go-build-exchanges go list ./... | rg -v '/sdk/')` passed after external order claim/import landed.
- 2026-06-15: `git diff --check` passed after external order claim/import landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution -run '^TestEngineSnapshotsAndPurgesExecutionState$' -v` first failed with missing `Engine.Snapshot` and `Engine.Purge`; it passed after execution engine exposed cache-backed execution state snapshot and purge APIs.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./testsuite -run 'Test(Engine|Manager|ExecutionEngineTester|NautilusParityRequirements|NautilusMaster)' -v` passed after `TC-EXENG16` made execution snapshot/purge a required parity case.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./platform ./live ./backtest ./testsuite` passed after execution snapshot/purge landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -race -count=1 ./execution ./platform ./live ./backtest ./testsuite` passed after execution snapshot/purge landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 $(env GOCACHE=/private/tmp/go-build-exchanges go list ./... | rg -v '/sdk/')` passed after execution snapshot/purge landed.
- 2026-06-15: `git diff --check` passed after execution snapshot/purge landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution -run '^TestManagerSnapshotsOrderListState$' -v` first failed with missing `Manager.OrderListSnapshot` and `Manager.OrderListSnapshots`; it passed after durable order-list snapshots exposed list members, held children, cache-backed order reports, and OUO fill progress.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution -run 'TestManager(SnapshotsOrderListState|ClosesOrderListAndClearsTransientState|CancelsHeldChildrenWhenOTOParentEndsWithoutFill)' -v` first failed because order-list snapshots had no kind, lifecycle status, member/open/terminal/held counters, or close cleanup; it passed after `execution.Manager` derived initialized/open/closed list state, retained list kind, cleared held OTO children when a parent ended without fill, and removed transient OUO progress once every cached member was terminal.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./testsuite -run 'Test(Manager|ExecutionEngineTester|NautilusMaster)' -v` passed after `ExecutionEngineTester` asserted order-list lifecycle snapshots in the master parity path.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./testsuite -run 'Test(ExecutionEngineTester|NautilusParityRequirements|NautilusMaster)' -v` first failed because `TC-EXENG17` was required but not emitted and the execution-engine required-case count was still 16; it passed after `TC-EXENG17` required durable order-list state snapshots in the master parity gate.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./testsuite -run 'Test(Manager|ExecutionEngineTester|NautilusParityRequirements|NautilusMaster)' -v` passed after durable order-list snapshots landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./platform ./live ./backtest ./testsuite` passed after durable order-list snapshots landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -race -count=1 ./execution ./platform ./live ./backtest ./testsuite` passed after durable order-list snapshots landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 $(env GOCACHE=/private/tmp/go-build-exchanges go list ./... | rg -v '/sdk/')` passed after durable order-list snapshots landed.
- 2026-06-15: `git diff --check` passed after durable order-list snapshots landed.
- 2026-06-15: Nautilus reference `.omx/references/nautilus_trader/nautilus_trader/execution/engine.pyx` shows `_is_leg_fill` detecting `-LEG-` child fills and `_handle_leg_fill_without_order` processing leg fills without a corresponding cached order.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./portfolio -run 'Test(ManagerAppliesLegFillWithoutOrderReport|PortfolioUsesFillPositionIDForLegFills)' -v` first failed because `model.FillReport` had no `PositionID`, `IsLeg`, or `IsLegFill`; it passed after leg-fill detection, no-order manager application, and explicit portfolio position IDs landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./testsuite -run 'Test(ExecutionEngineTester|PortfolioTester|NautilusParityRequirements|NautilusMaster)' -v` first failed because `TC-EXENG18` and `TC-P16` were required but missing; it passed after leg-fill execution and portfolio parity cases landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./model ./execution ./portfolio ./testsuite -run 'Test(FillReportDetectsLegFills|ManagerAppliesLegFill|PortfolioUsesFillPositionID|ExecutionEngineTester|PortfolioTester|NautilusParityRequirements|NautilusMaster)' -v` passed after leg fills landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./model ./cache ./execution ./portfolio ./testsuite` passed after leg fills landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./platform ./live ./backtest ./portfolio ./testsuite` passed after leg fills landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -race -count=1 ./execution ./platform ./live ./backtest ./portfolio ./testsuite` passed after leg fills landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 $(env GOCACHE=/private/tmp/go-build-exchanges go list ./... | rg -v '/sdk/')` passed after leg fills landed.
- 2026-06-15: `git diff --check` passed after leg fills landed.
- 2026-06-15: Nautilus reference `.omx/references/nautilus_trader/nautilus_trader/execution/manager.pyx` shows submit commands are cached and routed to `send_algo_command` when `exec_algorithm_id` is set, before normal risk/venue routing.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution -run '^TestEngineRoutesExecAlgorithmOrdersBeforeVenueSubmission$' -v` first failed with missing `Engine.AddAlgorithm`, `Health.Algorithms`, and `ErrAlgorithmNotFound`; it passed after `execution.Engine` added an algorithm registry, algorithm-first submit routing, cache application, and explicit missing-algorithm errors.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./testsuite -run 'Test(ExecutionEngineTester|NautilusParityRequirements|NautilusMaster)' -v` first failed because `TC-EXENG19` was required but not emitted; it passed after `TC-EXENG19` required execution-algorithm orders to bypass venue submit and cache an emulated report.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./testsuite -run 'Test(EngineRoutesExecAlgorithm|Engine|ExecutionEngineTester|NautilusParityRequirements|NautilusMaster)' -v` passed after execution algorithm routing hooks landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./platform ./live ./backtest ./portfolio ./testsuite` passed after execution algorithm routing hooks landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -race -count=1 ./execution ./platform ./live ./backtest ./portfolio ./testsuite` passed after execution algorithm routing hooks landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 $(env GOCACHE=/private/tmp/go-build-exchanges go list ./... | rg -v '/sdk/')` passed after execution algorithm routing hooks landed.
- 2026-06-15: `git diff --check` passed after execution algorithm routing hooks landed.
- 2026-06-15: Nautilus reference `.omx/references/nautilus_trader/nautilus_trader/execution/emulator.pyx` shows emulated orders are cached, held against bid/ask or last-price trigger feeds, marked `OrderEmulated`, and released only when market data triggers them.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution -run '^TestEngineEmulatesTriggerOrderUntilQuoteReleasesToVenue$' -v` first failed with missing `EngineConfig.Emulator`, `NewEmulator`, `SubmitOrder.EmulationTrigger`, `TriggerTypeBidAsk`, and `Engine.ProcessMarketEvent`; it passed after `execution.Emulator` held trigger orders as emulated, released them from quote events, submitted once to the venue client, and cache replacement removed the old synthetic emulated order.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./testsuite -run 'Test(ExecutionEngineTester|NautilusParityRequirements|NautilusMaster)' -v` first failed because `TC-EXENG20` was required but not emitted; it passed after `TC-EXENG20` made price-triggered emulation a master execution-engine parity case.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./testsuite -run 'Test(EngineEmulatesTriggerOrder|Engine|ExecutionEngineTester|NautilusParityRequirements|NautilusMaster)' -v` passed after price-triggered order emulation landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./model ./cache ./execution ./platform ./live ./backtest ./portfolio ./testsuite` passed after price-triggered order emulation landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -race -count=1 ./execution ./platform ./live ./backtest ./portfolio ./testsuite` passed after price-triggered order emulation landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 $(env GOCACHE=/private/tmp/go-build-exchanges go list ./... | rg -v '/sdk/')` passed after price-triggered order emulation landed.
- 2026-06-15: `git diff --check` passed after price-triggered order emulation landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution -run '^TestEngineEmulatesTriggerOrderUntilQuoteReleasesToVenue$' -v` first failed with missing `Engine.Events`; it passed after `execution.Engine` published `OrderEmulated`, `OrderTriggered`, and `OrderReleased` lifecycle events around emulator release.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./testsuite -run 'Test(ExecutionEngineTester|NautilusParityRequirements|NautilusMaster)' -v` first failed because `TC-EXENG21` was required but not emitted; it passed after `TC-EXENG21` made emulator lifecycle event publication a master execution-engine parity case.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./testsuite -run 'Test(EngineEmulatesTriggerOrder|Engine|ExecutionEngineTester|NautilusParityRequirements|NautilusMaster)' -v` passed after emulator lifecycle events were wired into the engine and master parity gate.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./model ./cache ./execution ./platform ./live ./backtest ./portfolio ./testsuite` passed after emulator lifecycle event publication landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -race -count=1 ./execution ./platform ./live ./backtest ./portfolio ./testsuite` passed after emulator lifecycle event publication landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 $(env GOCACHE=/private/tmp/go-build-exchanges go list ./... | rg -v '/sdk/')` passed after emulator lifecycle event publication landed.
- 2026-06-15: `git diff --check` passed after emulator lifecycle event publication landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./platform -run '^TestNodeFeedsDataEngineMarketEventsIntoExecutionEmulator$' -v` first failed because `platform.Node` wrapped an emulated report as `order_accepted`; it passed after Node retained emulator quote/trade subscriptions, routed `market.data` events into `execution.Engine.ProcessMarketEvent`, forwarded engine lifecycle events, and published accepted only after release.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./testsuite -run 'Test(ExecutionEngineTester|NautilusParityRequirements|NautilusMaster)' -v` first failed because `TC-EXENG22` was required but not emitted; it passed after `TC-EXENG22` made platform data-engine-to-emulator routing a master execution-engine parity case.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./platform ./testsuite -run 'Test(EngineEmulatesTriggerOrder|NodeFeedsDataEngineMarketEventsIntoExecutionEmulator|ExecutionEngineTester|NautilusParityRequirements|NautilusMaster)' -v` passed after automatic emulator trigger-feed management landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./model ./cache ./execution ./platform ./live ./backtest ./portfolio ./testsuite` passed after automatic emulator trigger-feed management landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -race -count=1 ./execution ./platform ./live ./backtest ./portfolio ./testsuite` passed after automatic emulator trigger-feed management landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 $(env GOCACHE=/private/tmp/go-build-exchanges go list ./... | rg -v '/sdk/')` passed after automatic emulator trigger-feed management landed.
- 2026-06-15: `git diff --check` passed after automatic emulator trigger-feed management landed.
- 2026-06-15: Nautilus reference `.omx/references/nautilus_trader/nautilus_trader/execution/trailing.pyx` shows trailing stops maintain a favorable price watermark, update trigger price by offset, and trigger on adverse movement through the computed trigger.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution -run '^TestEngineEmulatesTrailingStopMarketUntilTrailingQuoteTriggers$' -v` first failed because the emulator never released a trailing stop after the high-watermark drawdown; it passed after `execution.Emulator` tracked trailing watermarks and wrote computed trigger prices into triggered/released reports.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./testsuite -run 'Test(ExecutionEngineTester|NautilusParityRequirements|NautilusMaster)' -v` first failed because `TC-EXENG23` was required but not emitted; it passed after `TC-EXENG23` made trailing stop market emulation a master execution-engine parity case.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./platform ./testsuite -run 'Test(EngineEmulatesTrailingStopMarket|EngineEmulatesTriggerOrder|NodeFeedsDataEngineMarketEventsIntoExecutionEmulator|ExecutionEngineTester|NautilusParityRequirements|NautilusMaster)' -v` passed after trailing stop market emulation landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./model ./cache ./execution ./platform ./live ./backtest ./portfolio ./testsuite` passed after trailing stop market emulation landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -race -count=1 ./execution ./platform ./live ./backtest ./portfolio ./testsuite` passed after trailing stop market emulation landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 $(env GOCACHE=/private/tmp/go-build-exchanges go list ./... | rg -v '/sdk/')` passed after trailing stop market emulation landed.
- 2026-06-15: `git diff --check` passed after trailing stop market emulation landed.
- 2026-06-15: Nautilus reference `.omx/references/nautilus_trader/nautilus_trader/execution/emulator.pyx` shows `_fill_market_order` and `_fill_limit_order` transform released emulated trigger orders to real `MarketOrder` or `LimitOrder` before sending the venue execution command.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution -run 'TestEngineTransformsReleasedStop(Market|Limit)To' -v` first failed because released emulated stop orders were still submitted as `stop_market`/`stop_limit`; it passed after `execution.Emulator` transformed released venue submissions to market/limit orders while preserving trigger lifecycle reports.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./testsuite -run 'Test(ExecutionEngineTester|NautilusParityRequirements|NautilusMaster)' -v` first failed because `TC-EXENG24` was required but not emitted; it passed after `TC-EXENG24` made release-order transform a master execution-engine parity case.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./platform ./testsuite -run 'Test(EngineTransformsReleasedStop|EngineEmulatesTrailingStopMarket|EngineEmulatesTriggerOrder|NodeFeedsDataEngineMarketEventsIntoExecutionEmulator|ExecutionEngineTester|NautilusParityRequirements|NautilusMaster)' -v` passed after release-order transform landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./model ./cache ./execution ./platform ./live ./backtest ./portfolio ./testsuite` passed after release-order transform landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -race -count=1 ./execution ./platform ./live ./backtest ./portfolio ./testsuite` passed after release-order transform landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 $(env GOCACHE=/private/tmp/go-build-exchanges go list ./... | rg -v '/sdk/')` passed after release-order transform landed.
- 2026-06-15: `git diff --check` passed after release-order transform landed.
- 2026-06-15: Nautilus reference `.omx/references/nautilus_trader/nautilus_trader/execution/trailing.pyx` shows trailing offset types convert `PRICE` directly, `BASIS_POINTS` as a price-relative bps offset, and `TICKS` through instrument price increment.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution` first failed because `model.SubmitOrder` and `OrderStatusReport` had no `TrailingOffsetType`; it passed after model/report propagation and emulator tick/bps offset calculation landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./model ./execution ./backtest ./platform` passed after trailing offset type support landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./testsuite -run 'Test(ExecutionEngineTester|NautilusParityRequirements|NautilusMaster)' -v` passed after `TC-EXENG25` made tick/basis-point trailing offset emulation a master execution-engine parity case.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./model ./cache ./execution ./platform ./live ./backtest ./portfolio ./testsuite` passed after trailing offset type support landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -race -count=1 ./execution ./platform ./live ./backtest ./portfolio ./testsuite` passed after trailing offset type support landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 $(env GOCACHE=/private/tmp/go-build-exchanges go list ./... | rg -v '/sdk/')` passed after trailing offset type support landed.
- 2026-06-15: `git diff --check` passed after trailing offset type support landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution -run '^TestEngineEmulatesTrailingStopLimitReleasesLimitOrder$' -v` passed, proving existing trailing emulator plus release transform sends triggered trailing stop limit orders to the venue as regular limit orders while lifecycle reports retain the trigger context.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./testsuite -run 'Test(EngineEmulatesTrailingStopLimit|ExecutionEngineTester|NautilusParityRequirements|NautilusMaster)' -v` passed after `TC-EXENG26` made trailing stop limit release behavior a master execution-engine parity case.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./model ./cache ./execution ./platform ./live ./backtest ./portfolio ./testsuite` passed after trailing stop limit parity gating landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -race -count=1 ./execution ./platform ./live ./backtest ./portfolio ./testsuite` passed after trailing stop limit parity gating landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 $(env GOCACHE=/private/tmp/go-build-exchanges go list ./... | rg -v '/sdk/')` passed after trailing stop limit parity gating landed.
- 2026-06-15: `git diff --check` passed after trailing stop limit parity gating landed.
- 2026-06-15: Nautilus reference `.omx/references/nautilus_trader/nautilus_trader/execution/emulator.pyx` and `.omx/references/nautilus_trader/nautilus_trader/model/events/order.pyx` show `trigger_instrument_id` defaults to `instrument_id` but can route emulation and trigger-feed subscriptions to a distinct instrument.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution -run '^TestEngineEmulatesTriggerOrderWithTriggerInstrument$' -v` first failed because `SubmitOrder` and `OrderStatusReport` had no `TriggerInstrumentID`; it passed after model/report propagation and emulator event matching moved to the canonical trigger instrument.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./model ./execution ./platform ./backtest` passed after trigger-instrument routing and platform subscription selection landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./platform ./testsuite -run 'Test(EngineEmulatesTriggerOrderWithTriggerInstrument|NodeEmulationSubscriptionsUseTriggerInstrument|ExecutionEngineTester|NautilusParityRequirements|NautilusMaster)' -v` passed after `TC-EXENG27` made trigger-instrument emulation a master execution-engine parity case.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./model ./cache ./execution ./platform ./live ./backtest ./portfolio ./testsuite` passed after trigger-instrument parity gating landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -race -count=1 ./execution ./platform ./live ./backtest ./portfolio ./testsuite` passed after trigger-instrument parity gating landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 $(env GOCACHE=/private/tmp/go-build-exchanges go list ./... | rg -v '/sdk/')` passed after trigger-instrument parity gating landed.
- 2026-06-15: `git diff --check` passed after trigger-instrument parity gating landed.
- 2026-06-15: Nautilus reference `.omx/references/nautilus_trader/nautilus_trader/execution/emulator.pyx` shows `DEFAULT`/`BID_ASK` emulation subscribes order-book deltas plus quote ticks for the trigger instrument, while `LAST_PRICE` subscribes trade ticks.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./platform -run '^TestNodeEmulationSubscriptionsUseTriggerInstrument$' -v` first failed because bid/ask emulation subscribed only quote ticks; it passed after platform trigger-feed subscriptions added depth-1 order book data for bid/ask/default triggers.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution -run '^TestEngineEmulatesBidAskTriggerFromOrderBook$' -v` first failed because the emulator ignored order-book events; it passed after bid/ask trigger-price and trailing-range evaluation accepted top-of-book data.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./platform ./testsuite -run 'Test(EngineEmulatesBidAskTriggerFromOrderBook|NodeEmulationSubscriptionsUseTriggerInstrument|ExecutionEngineTester|NautilusParityRequirements|NautilusMaster)' -v` passed after `TC-EXENG28` made order-book bid/ask emulation a master execution-engine parity case.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./model ./cache ./execution ./platform ./live ./backtest ./portfolio ./testsuite` passed after order-book bid/ask trigger parity landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -race -count=1 ./execution ./platform ./live ./backtest ./portfolio ./testsuite` passed after order-book bid/ask trigger parity landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 $(env GOCACHE=/private/tmp/go-build-exchanges go list ./... | rg -v '/sdk/')` passed after order-book bid/ask trigger parity landed.
- 2026-06-15: `git diff --check` passed after order-book bid/ask trigger parity landed.
- 2026-06-15: Nautilus reference `.omx/references/nautilus_trader/nautilus_trader/model/instruments/synthetic.pyx` and `.omx/references/nautilus_trader/nautilus_trader/execution/emulator.pyx` show synthetic trigger instruments are stored separately in cache, use venue `SYNTH`, expose `price_increment`, and are used to create trigger matching cores.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./cache ./execution -run 'Test(CacheStoresSyntheticInstrument|EngineEmulatesTrailingStopMarketWithSyntheticTriggerTickOffset)' -v` first failed because `model.SyntheticInstrument`, `Cache.PutSyntheticInstrument`, and `Cache.SyntheticInstrument` were missing; it passed after the synthetic registry and emulator synthetic tick-size lookup landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./platform -run '^TestNodeEmulationSubscriptionsSkipOrderBookForSyntheticTrigger$' -v` first failed because synthetic bid/ask triggers still subscribed order books; it passed after platform skipped order-book trigger subscriptions for `SYNTH` instruments while retaining quote ticks.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./cache ./execution ./platform ./testsuite -run 'Test(CacheStoresSyntheticInstrument|EngineEmulatesTrailingStopMarketWithSyntheticTriggerTickOffset|NodeEmulationSubscriptionsSkipOrderBookForSyntheticTrigger|ExecutionEngineTester|NautilusParityRequirements|NautilusMaster)' -v` passed after `TC-EXENG29` made synthetic trigger instruments a master execution-engine parity case.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./model ./cache ./execution ./platform ./live ./backtest ./portfolio ./testsuite` passed after synthetic trigger instrument registry/lookup landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -race -count=1 ./execution ./platform ./live ./backtest ./portfolio ./testsuite` passed after synthetic trigger instrument registry/lookup landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 $(env GOCACHE=/private/tmp/go-build-exchanges go list ./... | rg -v '/sdk/')` passed after synthetic trigger instrument registry/lookup landed.
- 2026-06-15: `git diff --check` passed after synthetic trigger instrument registry/lookup landed.
- 2026-06-15: Nautilus reference `.omx/references/nautilus_trader/nautilus_trader/execution/emulator.pyx` shows emulated submit handling calls `matching_core.match_order(order, initial=True)` before holding the order, so already-marketable emulated orders release during submit.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution -run '^TestEngineEmulatesTriggerOrderImmediatelyWithCachedMarketData$' -v` first failed because an order with already-triggering cached quote data still returned `emulated`; it passed after `execution.Emulator.SubmitOrder` returned initial releases and `execution.Engine` published triggered/released lifecycle events before venue submission.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./testsuite -run 'Test(EngineEmulatesTriggerOrderImmediatelyWithCachedMarketData|ExecutionEngineTester|NautilusParityRequirements|NautilusMaster)' -v` passed after `TC-EXENG30` made submit-time initial matching a master execution-engine parity case.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./model ./cache ./execution ./platform ./live ./backtest ./portfolio ./testsuite` passed after submit-time initial matching landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -race -count=1 ./execution ./platform ./live ./backtest ./portfolio ./testsuite` passed after submit-time initial matching landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 $(env GOCACHE=/private/tmp/go-build-exchanges go list ./... | rg -v '/sdk/')` passed after submit-time initial matching landed.
- 2026-06-15: `git diff --check` passed after submit-time initial matching landed.
- 2026-06-15: Nautilus reference `.omx/references/nautilus_trader/nautilus_trader/execution/emulator.pyx` shows `_handle_cancel_order` checks matching-core ownership and `_cancel_order` removes the local emulation trigger before sending a canceled event for held emulated orders.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution -run '^TestEngineCancelsEmulatedOrderLocally$' -v` first failed because cancel routed to the venue fake and returned `accepted-client-local-cancel`; it passed after `execution.Emulator.CancelOrder` removed held emulated orders locally and `execution.Engine.CancelOrder` published the local canceled lifecycle.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./testsuite -run 'Test(EngineCancelsEmulatedOrderLocally|ExecutionEngineTester|NautilusParityRequirements|NautilusMaster)' -v` passed after `TC-EXENG31` made local cancel for held emulated orders a master execution-engine parity case.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./model ./cache ./execution ./platform ./live ./backtest ./portfolio ./testsuite` passed after local emulated cancel landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -race -count=1 ./execution ./platform ./live ./backtest ./portfolio ./testsuite` passed after local emulated cancel landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 $(env GOCACHE=/private/tmp/go-build-exchanges go list ./... | rg -v '/sdk/')` passed after local emulated cancel landed.
- 2026-06-15: `git diff --check` passed after local emulated cancel landed.
- 2026-06-15: Nautilus reference `.omx/references/nautilus_trader/nautilus_trader/execution/emulator.pyx` shows `_handle_modify_order` emits a local `OrderUpdated` event, then calls `matching_core.match_order(order)` and re-sorts held bid/ask queues instead of routing held emulated orders directly to the venue modifier.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution -run '^TestEngineLocalEmulatedModifyRematchesTrigger$' -v` first failed because the held emulated modify still reached the venue fake as `modify:client-local-modify`; it passed after `execution.Emulator.ModifyOrder` updated the held emulated order locally, rematched against cached quote data, and released a transformed market order when the modified trigger crossed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./testsuite -run 'Test(EngineLocalEmulatedModifyRematchesTrigger|ExecutionEngineTester|NautilusParityRequirements|NautilusMaster)' -v` passed after `TC-EXENG32` made local emulated modify/rematch a master execution-engine parity case.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./model ./cache ./execution ./platform ./live ./backtest ./portfolio ./testsuite` passed after local emulated modify/rematch landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -race -count=1 ./execution ./platform ./live ./backtest ./portfolio ./testsuite` passed after local emulated modify/rematch landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 $(env GOCACHE=/private/tmp/go-build-exchanges go list ./... | rg -v '/sdk/')` passed after local emulated modify/rematch landed.
- 2026-06-15: `git diff --check` passed after local emulated modify/rematch landed.
- 2026-06-15: Nautilus reference `.omx/references/nautilus_trader/nautilus_trader/execution/emulator.pyx` shows `_handle_cancel_all_orders` iterates matching-core held orders by side and cancels them locally instead of sending held emulated orders through the venue cancel-all path.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution -run '^TestEngineCancelAllCancelsHeldEmulatedOrdersLocally$' -v` first failed because a client with venue `CancelAllOrders` returned `venue-cancel-all`; it passed after `execution.Engine` made emulated batch/cancel-all local-first and only routed remaining non-emulated cancels to venue capabilities.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./testsuite -run 'Test(EngineCancelAllCancelsHeldEmulatedOrdersLocally|ExecutionEngineTester|NautilusParityRequirements|NautilusMaster)' -v` passed after `TC-EXENG33` made local cancel-all for held emulated orders a master execution-engine parity case.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution -run '^TestEngineEmulatesLimitOrderUntilOrderBookIsMarketable$' -v` first failed because plain emulated limit orders were held forever when `TriggerPrice` was empty; it passed after emulator matching treated `Limit` and `MarketToLimit` orders as bid/ask or last-price matchable by limit price while keeping stop/MIT/trailing trigger behavior unchanged.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./testsuite -run 'Test(EngineEmulatesLimitOrderUntilOrderBookIsMarketable|ExecutionEngineTester|NautilusMaster)' -v` passed after `TC-EXENG34` made emulated limit matching-core release semantics part of the master execution-engine parity path.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./model ./cache ./execution ./platform ./live ./backtest ./portfolio ./testsuite` passed after local emulated cancel-all landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -race -count=1 ./execution ./platform ./live ./backtest ./portfolio ./testsuite` passed after local emulated cancel-all landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 $(env GOCACHE=/private/tmp/go-build-exchanges go list ./... | rg -v '/sdk/')` passed after local emulated cancel-all landed.
- 2026-06-15: `git diff --check` passed after local emulated cancel-all landed.

**Acceptance:**
- `go test -count=1 ./execution ./account ./platform ./backtest ./testsuite -run 'ExecutionEngine|OrderManager|Contingency|PositionID|Fill'` passes.
- Existing `TradingAccount` remains as a high-level facade over the new execution engine or is clearly deprecated with compatibility tests.

## Epic 7: Reconciliation Engine

**Purpose:** Reach Nautilus-grade startup, periodic, reconnect, and discrepancy repair semantics.

**Files:**
- Modify: `account/reconciler.go`
- Modify: `account/trading_account.go`
- Create: `execution/reconciliation.go`
- Create: `execution/reconciliation_test.go`
- Create: `live/reconciliation.go`
- Create: `testsuite/reconciliation_tester.go`
- Create: `docs/guides/reconciliation.md`

**Tasks:**
- [x] Add mass-status reconciliation from order, fill, and position reports.
- [x] Add missing-fill query using lookback windows and trade-ID dedupe.
- [x] Add order open-state and filled-quantity discrepancy detection.
- [x] Add venue-order-id-only mapping and query fallback.
- [x] Add fill-before-order deferral and replay once order appears.
- [x] Add missing open order repair with recent-local-activity thresholds.
- [x] Add missing/stale position repair with retry limits and explicit unresolved discrepancy records.
- [x] Add external order import or explicit external-order rejection semantics.
- [x] Add reconciliation audit trail with case IDs, counters, last success, last error, and unresolved discrepancy list.

**Acceptance:**
- `go test -count=1 ./execution ./account ./live ./testsuite -run 'Reconciliation|MissingFill|MassStatus|ExternalOrder|Discrepancy|VenueOrder|FillBeforeOrder|OpenOrder|RepairDelay|Position|RetryLimit|Audit'` passes.
- Scenario B and Scenario C pass with exact event, health, and audit expectations.

**Progress Evidence:**
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution -run '^TestReconcilerAppliesMassStatusToCacheWithAudit$' -v` first failed with missing `execution.NewReconciler` and `ReconciliationConfig`; it passed after `execution.Reconciler` applied mass-status account, order, fill, and position reports through cache plus manager-backed order/fill reconciliation and stored `TC-REC01` audit counters.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./testsuite -run '^TestReconciliationTesterReportsMassStatusCase$' -v` first failed with missing `NewReconciliationTester` and `ReconciliationTesterConfig`; it passed after `testsuite.ReconciliationTester` emitted `TC-REC01`.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./testsuite -run '^TestNautilusParityRequirementsCoverTargetSuites$' -v` first failed because `NautilusParityRequirements` had no reconciliation suite; it passed after `reconciliation:TC-REC01` became a required parity case.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./testsuite -run 'Test(ReconcilerAppliesMassStatusToCacheWithAudit|ReconciliationTester|NautilusParityRequirements|NautilusMaster)' -v` passed after `TC-REC01` landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./account ./live ./testsuite -run 'Reconciliation|MissingFill|MassStatus|ExternalOrder|Discrepancy' -v` passed after mass-status reconciliation landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./model ./cache ./execution ./account ./platform ./live ./backtest ./portfolio ./testsuite` passed after mass-status reconciliation landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -race -count=1 ./execution ./account ./platform ./live ./backtest ./portfolio ./testsuite` passed after mass-status reconciliation landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 $(env GOCACHE=/private/tmp/go-build-exchanges go list ./... | rg -v '/sdk/')` passed after mass-status reconciliation landed.
- 2026-06-15: `git diff --check` passed after mass-status reconciliation landed.
- 2026-06-15: Nautilus reference `.omx/references/nautilus_trader/nautilus_trader/live/execution_engine.py` shows position discrepancies query fill reports inside a configured lookback window and skip already reconciled trade IDs.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution -run '^TestReconcilerAppliesOnlyMissingFillsInsideLookback$' -v` first failed because `execution.Reconciler` had no `ReconcileMissingFills`; it passed after missing-fill reconciliation applied only fills inside the lookback and skipped cached trade IDs with audit counters.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./testsuite -run '^TestReconciliationTesterReportsMassStatusCase$' -v` first failed because `TC-REC02` was missing; it passed after `testsuite.ReconciliationTester` emitted the lookback/dedupe case.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./testsuite -run '^TestNautilusParityRequirementsCoverTargetSuites$' -v` first failed because `reconciliation` required only `TC-REC01`; it passed after `TC-REC02` became required.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./testsuite -run 'Test(ReconcilerAppliesOnlyMissingFillsInsideLookback|ReconciliationTester|NautilusParityRequirements|NautilusMaster)' -v` passed after `TC-REC02` landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./account ./live ./testsuite -run 'Reconciliation|MissingFill|MassStatus|ExternalOrder|Discrepancy' -v` passed after missing-fill lookback/dedupe landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./model ./cache ./execution ./account ./platform ./live ./backtest ./portfolio ./testsuite` passed after missing-fill lookback/dedupe landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -race -count=1 ./execution ./account ./platform ./live ./backtest ./portfolio ./testsuite` passed after missing-fill lookback/dedupe landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 $(env GOCACHE=/private/tmp/go-build-exchanges go list ./... | rg -v '/sdk/')` passed after missing-fill lookback/dedupe landed.
- 2026-06-15: `git diff --check` passed after missing-fill lookback/dedupe landed.
- 2026-06-15: Nautilus reference `.omx/references/nautilus_trader/nautilus_trader/live/execution_engine.py` shows reconciliation compares cached and venue order/fill/position quantities, logs persistent discrepancies, and does not silently ignore open-state or filled-quantity mismatches.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution -run '^TestReconcilerDetectsOrderDiscrepancyStateAndFilledQuantity$' -v` first failed because `execution.Reconciler` had no `DetectOrderDiscrepancies`; it passed after order discrepancy detection recorded open-state and filled-quantity mismatches without mutating cache.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./testsuite -run '^TestReconciliationTesterReportsMassStatusCase$' -v` first failed because `TC-REC03` was missing; it passed after `testsuite.ReconciliationTester` emitted the discrepancy detection case.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./testsuite -run '^TestNautilusParityRequirementsCoverTargetSuites$' -v` first failed because `reconciliation` required only `TC-REC01..TC-REC02`; it passed after `TC-REC03` became required.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./testsuite -run 'Test(ReconcilerDetectsOrderDiscrepancyStateAndFilledQuantity|ReconciliationTester|NautilusParityRequirements|NautilusMaster)' -v` passed after `TC-REC03` landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./account ./live ./testsuite -run 'Reconciliation|MissingFill|MassStatus|ExternalOrder|Discrepancy' -v` passed and covered `TestReconcilerDetectsOrderDiscrepancyStateAndFilledQuantity` after the test name was aligned with the acceptance regex.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./model ./cache ./execution ./account ./platform ./live ./backtest ./portfolio ./testsuite` passed after order discrepancy detection landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -race -count=1 ./execution ./account ./platform ./live ./backtest ./portfolio ./testsuite` passed after order discrepancy detection landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 $(env GOCACHE=/private/tmp/go-build-exchanges go list ./... | rg -v '/sdk/')` passed after order discrepancy detection landed.
- 2026-06-15: `git diff --check` passed after order discrepancy detection landed.
- 2026-06-15: Nautilus reference `.omx/references/nautilus_trader/nautilus_trader/live/execution_engine.py` and adapter execution clients show reconciliation can recover order identity from venue-originated reports when only venue IDs are available.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution -run '^TestEngineQueryOrderFallsBackToReportsByVenueOrderID$' -v` first failed with `not supported: execution client does not support query`; it passed after `execution.Engine.QueryOrder` added cache-first lookup plus `GenerateOrderStatusReports` fallback for venue-order-id-only queries.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./testsuite -run '^TestReconciliationTesterReportsMassStatusCase$' -v` first failed because `TC-REC04` was missing; it passed after `testsuite.ReconciliationTester` emitted the venue-order-id fallback case.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./testsuite -run '^TestNautilusParityRequirementsCoverTargetSuites$' -v` first failed because `reconciliation` required only `TC-REC01..TC-REC03`; it passed after `TC-REC04` became required.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./testsuite -run 'Test(EngineQueryOrderFallsBackToReportsByVenueOrderID|ReconciliationTester|NautilusParityRequirements|NautilusMaster)' -v` passed after `TC-REC04` landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./account ./live ./testsuite -run 'Reconciliation|MissingFill|MassStatus|ExternalOrder|Discrepancy|VenueOrder' -v` passed after venue-order-id query fallback landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./backtest ./testsuite -run 'Test(EngineRoutesCancelModifyAndQuery|EngineQueryOrderFallsBackToReportsByVenueOrderID|BacktestRoutesOrderCommandsThroughExecutionEngine|ExecutionEngineTester)' -v` passed after cache-first lookup was narrowed to venue-order-id-only queries.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./model ./cache ./execution ./account ./platform ./live ./backtest ./portfolio ./testsuite` passed after venue-order-id query fallback landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -race -count=1 ./execution ./account ./platform ./live ./backtest ./portfolio ./testsuite` passed after venue-order-id query fallback landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 $(env GOCACHE=/private/tmp/go-build-exchanges go list ./... | rg -v '/sdk/')` passed after venue-order-id query fallback landed.
- 2026-06-15: `git diff --check` passed after venue-order-id query fallback landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./testsuite -run 'Test(ReconcilerDefersFillBeforeOrderAndReplaysWhenOrderAppears|ReconciliationTesterReportsMassStatusCase|NautilusParityRequirementsCoverTargetSuites)' -v` first failed because `ReconciliationResult` had no `FillsDeferred`, `ReconciliationTester` did not emit `TC-REC05`, and `NautilusParityRequirements` required only `TC-REC01..TC-REC04`; it passed after fill-before-order reconciliation counted deferred fills, replayed them once the order report appeared, and made `TC-REC05` a required case.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./testsuite -run 'Test(Reconciler|ReconciliationTester|NautilusParityRequirements|NautilusMaster)' -v` passed after `TC-REC05` landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./account ./live ./testsuite -run 'Reconciliation|MissingFill|MassStatus|ExternalOrder|Discrepancy|VenueOrder|FillBeforeOrder' -v` passed after `TC-REC05` was included in the Epic 7 acceptance regex.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./model ./cache ./execution ./account ./platform ./live ./backtest ./portfolio ./testsuite` passed after fill-before-order reconciliation landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -race -count=1 ./execution ./account ./platform ./live ./backtest ./portfolio ./testsuite` passed after fill-before-order reconciliation landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 $(env GOCACHE=/private/tmp/go-build-exchanges go list ./... | rg -v '/sdk/')` passed after fill-before-order reconciliation landed.
- 2026-06-15: `git diff --check` passed after fill-before-order reconciliation landed.
- 2026-06-15: Nautilus reference `.omx/references/nautilus_trader/nautilus_trader/live/config.py` defines `open_check_threshold_ms`, and `.omx/references/nautilus_trader/nautilus_trader/live/execution_engine.py` skips missing-open-order reconciliation when cached order activity or local activity is within that threshold.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./account ./testsuite -run 'Test(ReconcilerSkipsRecentMissingOpenOrdersUntilRepairThreshold|TradingAccountReconciliationHonorsMissingOrderRepairDelay|ReconciliationTesterReportsMassStatusCase|NautilusParityRequirementsCoverTargetSuites)' -v` first failed because `account.Reconciler` had no threshold policy, `TradingAccountConfig` had no `MissingOrderRepairDelay`, `ReconciliationTester` did not emit `TC-REC06`, and `NautilusParityRequirements` required only `TC-REC01..TC-REC05`; it passed after recent local order activity skipped premature missing-open-order repair while stale missing open orders were marked canceled.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./account ./testsuite -run 'Test(Reconciler|TradingAccount|ReconciliationTester|NautilusParityRequirements|NautilusMaster)' -v` passed after `TC-REC06` landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./account ./live ./testsuite -run 'Reconciliation|MissingFill|MassStatus|ExternalOrder|Discrepancy|VenueOrder|FillBeforeOrder|OpenOrder|RepairDelay' -v` passed after `TC-REC06` was included in the Epic 7 acceptance regex.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./model ./cache ./execution ./account ./platform ./live ./backtest ./portfolio ./testsuite` first failed because implicit `LastUpdatedTime` stamping in `account.Reconciler.applyOrder` changed platform order cache equality; it passed after missing-open-order repair stopped mutating ordinary venue reports and only stamped generated repair reports.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -race -count=1 ./execution ./account ./platform ./live ./backtest ./portfolio ./testsuite` passed after missing-open-order repair threshold landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 $(env GOCACHE=/private/tmp/go-build-exchanges go list ./... | rg -v '/sdk/')` passed after missing-open-order repair threshold landed.
- 2026-06-15: `git diff --check` passed after missing-open-order repair threshold landed.
- 2026-06-15: Nautilus reference `.omx/references/nautilus_trader/nautilus_trader/live/execution_engine.py` tracks `position_check_retries`, skips recent position local activity, and logs unresolved position discrepancies after retry exhaustion.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./account ./testsuite -run 'Test(ReconcilerRepairsMissingAndStalePositionsUntilRetryLimit|ReconciliationTesterReportsMassStatusCase|NautilusParityRequirementsCoverTargetSuites)' -v` first failed because `account.Reconciler` had no `RepairPositionReports`, `PositionRepairPolicy`, or `PositionRepairDiscrepancy`, `ReconciliationTester` did not emit `TC-REC07`, and `NautilusParityRequirements` required only `TC-REC01..TC-REC06`; it passed after missing/stale position repair produced flat or venue-aligned reports until retry exhaustion and then returned explicit unresolved discrepancies.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./account ./testsuite -run 'Test(Reconciler|TradingAccount|ReconciliationTester|NautilusParityRequirements|NautilusMaster)' -v` passed after `TC-REC07` landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./account ./live ./testsuite -run 'Reconciliation|MissingFill|MassStatus|ExternalOrder|Discrepancy|VenueOrder|FillBeforeOrder|OpenOrder|RepairDelay|Position|RetryLimit' -v` passed after `TC-REC07` was included in the Epic 7 acceptance regex.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./model ./cache ./execution ./account ./platform ./live ./backtest ./portfolio ./testsuite` passed after position repair retry/unresolved records landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -race -count=1 ./execution ./account ./platform ./live ./backtest ./portfolio ./testsuite` passed after position repair retry/unresolved records landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 $(env GOCACHE=/private/tmp/go-build-exchanges go list ./... | rg -v '/sdk/')` passed after position repair retry/unresolved records landed.
- 2026-06-15: `git diff --check` passed after position repair retry/unresolved records landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./testsuite -run 'Test(ReconcilerImportsOrRejectsExternalOrdersExplicitly|ReconciliationTesterReportsMassStatusCase|NautilusParityRequirementsCoverTargetSuites)' -v` first failed because `execution.Reconciler` had no `ReconcileExternalOrders` or `ExternalOrderPolicy`, `ReconciliationTester` did not emit `TC-REC08`, and `NautilusParityRequirements` required only `TC-REC01..TC-REC07`; it passed after external orders were either imported under an explicit strategy or rejected into unresolved discrepancies without caching.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./account ./testsuite -run 'Test(Reconciler|TradingAccount|ReconciliationTester|NautilusParityRequirements|NautilusMaster)' -v` passed after `TC-REC08` landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./account ./live ./testsuite -run 'Reconciliation|MissingFill|MassStatus|ExternalOrder|Discrepancy|VenueOrder|FillBeforeOrder|OpenOrder|RepairDelay|Position|RetryLimit' -v` passed after `TC-REC08` was covered by the Epic 7 acceptance regex.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./model ./cache ./execution ./account ./platform ./live ./backtest ./portfolio ./testsuite` passed after external-order import/rejection landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -race -count=1 ./execution ./account ./platform ./live ./backtest ./portfolio ./testsuite` passed after external-order import/rejection landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 $(env GOCACHE=/private/tmp/go-build-exchanges go list ./... | rg -v '/sdk/')` passed after external-order import/rejection landed.
- 2026-06-15: `git diff --check` passed after external-order import/rejection landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./testsuite -run 'Test(ReconcilerAuditTrailTracksSuccessErrorAndUnresolved|ReconciliationTesterReportsMassStatusCase|NautilusParityRequirementsCoverTargetSuites)' -v` first failed because `execution.Reconciler` had no `AuditTrail`, `ReconciliationTester` did not emit `TC-REC09`, and `NautilusParityRequirements` required only `TC-REC01..TC-REC08`; it passed after reconciliation history, last success, last error, and unresolved discrepancy snapshots were recorded.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./account ./testsuite -run 'Test(Reconciler|TradingAccount|ReconciliationTester|NautilusParityRequirements|NautilusMaster)' -v` passed after `TC-REC09` landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./account ./live ./testsuite -run 'Reconciliation|MissingFill|MassStatus|ExternalOrder|Discrepancy|VenueOrder|FillBeforeOrder|OpenOrder|RepairDelay|Position|RetryLimit|Audit' -v` passed after `TC-REC09` completed Epic 7 acceptance coverage.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./model ./cache ./execution ./account ./platform ./live ./backtest ./portfolio ./testsuite` passed after reconciliation audit trail landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -race -count=1 ./execution ./account ./platform ./live ./backtest ./portfolio ./testsuite` passed after reconciliation audit trail landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 $(env GOCACHE=/private/tmp/go-build-exchanges go list ./... | rg -v '/sdk/')` passed after reconciliation audit trail landed.
- 2026-06-15: `git diff --check` passed after reconciliation audit trail landed.

## Epic 8: Risk Engine Completion

**Purpose:** Move risk from a synchronous order checker to a production risk subsystem.

**Files:**
- Modify: `risk/risk.go`
- Modify: `platform/node.go`
- Create: `risk/engine.go`
- Create: `risk/config.go`
- Create: `risk/queue.go`
- Create: `risk/rules.go`
- Create: `risk/events.go`
- Create: `testsuite/risk_engine_tester.go`

**Tasks:**
- [x] Keep the current synchronous `Check` API for simple callers.
- [x] Add asynchronous command/event queues with bounded capacity and health metrics.
- [x] Add pre-trade rules for trading state, instrument state, order type, price/size precision, TIF, post-only, reduce-only, max order notional, max position notional, projected account exposure, and margin.
- [x] Add account/strategy/instrument scoped limits.
- [x] Add kill switch and reducing-only state transitions.
- [x] Add throttling rules for command rate, order count, and duplicate client order IDs.
- [x] Add risk rejection events that preserve command metadata and prevent downstream venue submission.

**Acceptance:**
- `go test -count=1 ./risk ./platform ./testsuite -run 'Risk|Exposure|Margin|KillSwitch|Throttle'` passes.
- Race tests show no queue or lifecycle race.

**Progress evidence:**
- 2026-06-15: `go test -count=1 ./risk -run 'TestEngineExecute|TestEngineProcessExecutionEventsThroughEventQueue'` passed.
- 2026-06-15: `go test -count=1 ./risk` passed.
- 2026-06-15: `go test -count=1 ./platform` passed.
- 2026-06-15: `go test -count=1 ./live` passed.
- 2026-06-15: `go test -count=1 ./kernel ./bus ./strategy ./platform ./risk ./live -run 'Component|Clock|Bus|Lifecycle|Health|Risk|Runner|TradingNode|Queue'` passed.
- 2026-06-15: `go test -count=1 $(go list ./... | grep -v '/sdk')` passed.
- 2026-06-15: `go test -count=1 ./risk -run 'TestEngineRejectsDuplicateClientOrderID|TestEngineCheckExistingOrderAllowsCurrentClientOrderID'` passed.
- 2026-06-15: `go test -count=1 ./platform -run 'TestNodeModifyOrderChecksRiskBeforeVenueModification'` passed.
- 2026-06-15: `go test -count=1 ./risk ./platform ./live` passed.
- 2026-06-15: `go test -count=1 ./risk -run 'TestEngineRuntimeKillSwitchTransition|TestEngineRuntimeReducingOnlyTransition'` passed.
- 2026-06-15: `go test -count=1 ./model -run 'TestSubmitOrderRejectsInvalidOrderType'` passed.
- 2026-06-15: `go test -count=1 ./risk ./platform ./testsuite -run 'Risk|Exposure|Margin|KillSwitch|Throttle'` passed.
- 2026-06-15: `go test -race -count=1 ./risk ./platform ./live` passed.
- 2026-06-15: `go test -count=1 ./risk -run 'TestEngineAppliesAccountStrategyAndInstrumentScopedOrderLimits|TestEngineAppliesScopedPositionAndExposureLimits'` passed.
- 2026-06-15: `go test -count=1 ./risk ./platform ./testsuite -run 'Risk|Exposure|Margin|KillSwitch|Throttle|Limit'` passed.
- 2026-06-15: `go test -count=1 ./risk -run 'TestEngineRejectsWhenCommandRateLimitExceeded|TestEngineRejectsWhenOpenOrderLimitExceeded'` passed.
- 2026-06-15: `go test -race -count=1 ./risk ./platform ./live` passed.
- 2026-06-15: `go test -count=1 $(go list ./... | grep -v '/sdk')` passed.
- 2026-06-15: `go test -count=1 ./risk -run 'TestEngineExecuteReturnsDeniedLifecycleEventWithCommandMetadata'` passed.
- 2026-06-15: `go test -count=1 ./platform -run 'TestNodePublishesOrderDeniedWhenRiskRejectsSubmit'` passed.
- 2026-06-15: `go test -count=1 ./risk ./platform -run 'Denied|RiskRejects|CommandRate|OpenOrderLimit|Throttle'` passed.

## Epic 9: Portfolio And Accounting Completion

**Purpose:** Match Nautilus account-facing portfolio semantics, not just fill accounting.

**Files:**
- Modify: `portfolio/portfolio.go`
- Create: `portfolio/accounts.go`
- Create: `portfolio/positions.go`
- Create: `portfolio/pnl.go`
- Create: `portfolio/exposure.go`
- Create: `portfolio/conversion.go`
- Create: `portfolio/analyzer.go`
- Create: `testsuite/portfolio_engine_tester.go`

**Tasks:**
- [x] Add event-driven account, order, fill, and position update handlers.
- [x] Add cash and margin account accounting paths.
- [x] Add balance updates for fills, commissions, realized PnL, and settlement currency.
- [x] Add unrealized PnL calculation from selected price types and marks.
- [x] Add net position and net exposure aggregation by instrument, account, venue, and target currency.
- [x] Add conversion hooks for quote/settle/base currency conversion.
- [x] Add PnL cache invalidation on order, position, account, and market events.
- [x] Add analyzer hooks for closed-position trade records and account-currency PnL.
- [x] Apply leg fills to explicit synthetic position IDs for spread-leg accounting.

**Progress Evidence:**
- 2026-06-15: `go test -count=1 ./portfolio -run 'TestPortfolioHandlesExecutionEventsForAccountOrderFillAndPosition'` passed after the missing handler RED.
- 2026-06-15: `go test -count=1 ./testsuite -run 'TestPortfolioTesterReportsPnLAndCommissionCases'` passed with `TC-P08`.
- 2026-06-15: `go test -count=1 ./testsuite -run 'TestPortfolioTesterReportsPnLAndCommissionCases'` passed with `TC-P09` cash-vs-margin accounting paths.
- 2026-06-15: `go test -count=1 ./portfolio -run 'TestPortfolioAppliesFillBalanceDeltasForCommissionAndRealizedPnL'` passed.
- 2026-06-15: `go test -count=1 ./testsuite -run 'TestPortfolioTesterReportsPnLAndCommissionCases'` passed with `TC-P10` fill balance deltas.
- 2026-06-15: `go test -count=1 ./testsuite -run 'TestPortfolioTesterReportsPnLAndCommissionCases'` passed with `TC-P11` selected price and mark unrealized PnL.
- 2026-06-15: `go test -count=1 ./portfolio -run 'TestPortfolioAggregatesNetPositionsAndExposureByInstrumentAccountVenueAndCurrency'` passed.
- 2026-06-15: `go test -count=1 ./testsuite -run 'TestPortfolioTesterReportsPnLAndCommissionCases'` passed with `TC-P12` net aggregation.
- 2026-06-15: `go test -count=1 ./portfolio -run 'TestPortfolioUsesExplicitConversionRatesForSettleAndAccountBase'` passed.
- 2026-06-15: `go test -count=1 ./testsuite -run 'TestPortfolioTesterReportsPnLAndCommissionCases'` passed with `TC-P13` conversion hooks.
- 2026-06-15: `go test -count=1 ./portfolio -run 'TestPortfolioInvalidatesUnrealizedPnLCacheOnOrderPositionAccountAndMarketEvents'` passed.
- 2026-06-15: `go test -count=1 ./testsuite -run 'TestPortfolioTesterReportsPnLAndCommissionCases'` passed with `TC-P14` PnL cache invalidation.
- 2026-06-15: `go test -count=1 ./portfolio -run 'TestPortfolioRecordsClosedTradeWithAccountCurrencyPnL'` passed.
- 2026-06-15: `go test -count=1 ./testsuite -run 'TestPortfolioTesterReportsPnLAndCommissionCases'` passed with `TC-P15` analyzer closed-trade records.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./portfolio -run '^TestPortfolioUsesFillPositionIDForLegFills$' -v` passed after portfolio position construction started honoring fill-level `PositionID`.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./testsuite -run 'TestPortfolioTesterReportsPnLAndCommissionCases' -v` passed with `TC-P16` explicit leg-fill position IDs.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./model ./cache ./execution ./portfolio ./testsuite` passed after leg-fill portfolio accounting landed.
- 2026-06-15: `go test -count=1 ./portfolio ./testsuite -run 'Portfolio|Accounting|PnL|Exposure|Conversion|Analyzer'` passed.
- 2026-06-15: `go test -count=1 ./portfolio ./testsuite` passed.
- 2026-06-15: `go test -race -count=1 ./portfolio ./testsuite` passed.
- 2026-06-15: `go test -count=1 $(go list ./... | grep -v '/sdk')` passed.
- 2026-06-15: `git diff --check` passed.

**Acceptance:**
- `go test -count=1 ./portfolio ./testsuite -run 'Portfolio|Accounting|PnL|Exposure|Conversion|Analyzer'` passes.
- Scenario A final account/position/PnL state matches the golden expectation including fees.

## Epic 10: Backtest Engine Completion

**Purpose:** Provide a real simulated venue loop equivalent to Nautilus backtesting for core workflows.

**Files:**
- Modify: `backtest/runner.go`
- Modify: `backtest/fill_model.go`
- Modify: `testsuite/backtest_tester.go`
- Create: `backtest/engine.go`
- Create: `backtest/node.go`
- Create: `backtest/matching_core.go`
- Create: `backtest/catalog.go`
- Create: `backtest/results.go`
- Create: `testsuite/backtest_engine_tester.go`

**Tasks:**
- [x] Extract matching logic into a reusable matching core.
- [x] Enforce post-only maker-only fills and reduce-only non-opening constraints in backtest matching.
- [x] Apply OUO partial-fill quantity reductions to linked open siblings in backtest matching.
- [x] Apply Nautilus-style partial OTO child release/resizing from cumulative parent fills.
- [x] Add deterministic backtest result summary JSON for reproducibility gates.
- [x] Add multi-account result summaries and catalog-backed event loading for backtest engine runs.
- [x] Add multi-strategy result metadata coverage and a benchmark smoke gate.
- [x] Support market, limit, market-to-limit, stop-market, stop-limit, market-if-touched, limit-if-touched, trailing-stop-market, trailing-stop-limit, post-only, reduce-only, GTD expiry, and order latency.
- [x] Support order book depth walking, partial fills, volume consumption, slippage, maker/taker fees, and deterministic fill IDs.
- [x] Support OTO/OCO/OUO/bracket order lists with Nautilus-compatible semantics.
- [x] Support same-timestamp cascading command drain until stable.
- [x] Support catalog-backed runs, multi-strategy runs, multi-account runs, and result summaries.
- [x] Add benchmark and determinism tests.

**Progress Evidence:**
- 2026-06-15: `go test -count=1 ./backtest -run 'TestMatchingCore'` passed after the missing matching-core API RED.
- 2026-06-15: `go test -count=1 ./testsuite -run 'TestBacktestTesterReportsParityCaseResults|TestParityScoreboardAggregatesBacktestGate'` passed with `TC-B06`.
- 2026-06-15: `go test -count=1 ./backtest ./testsuite -run 'Backtest|Matching|Bracket|Trailing|Determinism'` passed.
- 2026-06-15: `go test -count=1 ./backtest ./testsuite` passed.
- 2026-06-15: `go test -race -count=1 ./backtest ./strategy ./portfolio ./risk` passed.
- 2026-06-15: `go test -count=1 ./backtest -run 'TestBacktest(PostOnlyLimitDoesNotTakeLiquidityOnSubmissionEvent|ReduceOnlyOrderWithoutPositionDoesNotOpenPosition)'` passed after RED failures confirmed same-event post-only taking and reduce-only opening behavior.
- 2026-06-15: `go test -count=1 ./testsuite -run 'TestBacktestTesterReportsParityCaseResults|TestParityScoreboardAggregatesBacktestGate|TestNautilusParityRequirementsCoverTargetSuites'` passed with backtest required cases expanded to `TC-B01..TC-B08`.
- 2026-06-15: `go test -count=1 ./backtest ./testsuite -run 'TestBacktestTriggers(MarketIfTouchedWhenPriceFallsToTrigger|LimitIfTouchedThenMatchesLimitPrice)|TestBacktestTesterReportsParityCaseResults|TestParityScoreboardAggregatesBacktestGate|TestNautilusParityRequirementsCoverTargetSuites'` passed with existing MIT/LIT behavior captured and backtest required cases expanded to `TC-B01..TC-B10`.
- 2026-06-15: `go test -count=1 ./backtest ./testsuite -run 'TestBacktestOuoPartialFillReducesSiblingLeavesQuantity|TestBacktestTesterReportsParityCaseResults|TestParityScoreboardAggregatesBacktestGate|TestNautilusParityRequirementsCoverTargetSuites'` passed after OUO sibling quantity reductions were added and backtest required cases expanded to `TC-B01..TC-B11`.
- 2026-06-15: `go test -count=1 ./backtest ./testsuite -run 'TestBacktestOTOChildrenReleaseAndResizeOnPartialParentFills|TestBacktestTesterReportsParityCaseResults|TestParityScoreboardAggregatesBacktestGate|TestNautilusParityRequirementsCoverTargetSuites'` passed after partial OTO child release/resizing was added and backtest required cases expanded to `TC-B01..TC-B12`.
- 2026-06-15: `go test -count=1 ./backtest ./testsuite -run 'TestBacktestResultSummaryIsDeterministic|TestBacktestTesterReportsParityCaseResults|TestParityScoreboardAggregatesBacktestGate|TestNautilusParityRequirementsCoverTargetSuites'` passed after deterministic result summary JSON was added and backtest required cases expanded to `TC-B01..TC-B13`.
- 2026-06-15: `go test -count=1 ./backtest ./testsuite -run 'TestBacktestResultSummaryDefaultsToAllTouchedAccounts|TestEngineLoadsEventsFromCatalog|TestBacktestTesterReportsParityCaseResults|TestParityScoreboardAggregatesBacktestGate|TestNautilusParityRequirementsCoverTargetSuites'` passed after multi-account auto summaries and `MemoryCatalog` event loading were added and backtest required cases expanded to `TC-B01..TC-B15`.
- 2026-06-15: `go test -count=1 ./backtest ./testsuite -run 'TestBacktestRunsMultipleStrategiesAndPreservesStrategyMetadata|TestBacktestTesterReportsParityCaseResults|TestParityScoreboardAggregatesBacktestGate|TestNautilusParityRequirementsCoverTargetSuites'` passed with multi-strategy metadata coverage and backtest required cases expanded to `TC-B01..TC-B16`.
- 2026-06-15: `go test -count=1 ./backtest -run '^$' -bench 'BenchmarkRunnerMarketOrder' -benchtime=1x` passed as the backtest benchmark smoke gate.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./backtest ./testsuite -run 'TestBacktest(OrderLatency|ExpiresGTD|MarketToLimit|TriggersStop|TriggersMarketIfTouched|TriggersLimitIfTouched|TriggersTrailing|PostOnly|ReduceOnly|AppliesInstrument.*Fee|MatchesMarketOrderAgainstOrderBookLevels|DoesNotMutateHistoricalOrderBookAfterFills|ResultSummary|RunsMultipleStrategies|Bracket|Ouo|OUO|OTO|SameTimestamp|Catalog|EngineLoadsEventsFromCatalog)|TestBacktestTesterReportsParityCaseResults|TestNautilusMaster' -v` passed, proving the advanced order, latency, GTD, fee, order-book, bracket, OUO/OTO, result-summary, and master backtest gates.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./backtest ./testsuite -run 'TestBacktest(FillModelAppliesOneTickSlippageToL1TakerFill|SettlesCascadingOrdersSubmittedFromFillCallback|EngineReplaysSharedDataCatalog|ResultPortfolioUpdatesFromMarketData|SharedPortfolioCacheDoesNotDoubleApplyFills)|TestBacktestTesterReportsParityCaseResults|TestNautilusMaster' -v` passed, proving slippage, same-timestamp cascading command drain, shared catalog replay, portfolio result updates, and shared-cache fill idempotency.

**Acceptance:**
- `go test -count=1 ./backtest ./testsuite -run 'Backtest|Matching|Bracket|Trailing|Determinism'` passes.
- `go test -race -count=1 ./backtest ./strategy ./portfolio ./risk` passes.
- Running the same backtest twice produces byte-identical event/result summaries.

## Epic 11: Live Node And Runtime Completion

**Purpose:** Make live trading assembly and lifecycle equivalent to Nautilus live node concepts.

**Files:**
- Modify: `live/runner.go`
- Modify: `platform/node.go`
- Create: `live/node.go`
- Create: `live/node_builder.go`
- Create: `live/config.go`
- Create: `live/retry.go`
- Create: `live/health.go`
- Create: `live/shutdown.go`
- Create: `testsuite/live_node_tester.go`

**Tasks:**
- [x] Add reusable live node parity tester covering assembly, lifecycle health, startup subscriptions, runtime order submission, typed market-data delivery, and startup phase order (`TC-LIVE01..TC-LIVE06`).
- [x] Add `live.Node` as the canonical assembly of data engine, execution engine, risk engine, portfolio, cache, bus, clock, and strategies.
- [x] Add `live.NodeBuilder` with typed config and validation.
- [x] Add startup phases: load instruments, connect data, connect execution, reconcile execution, apply deferred subscriptions, start strategies.
- [x] Add shutdown phases: stop strategies, cancel timers, stop data/execution/risk queues, disconnect clients, flush health.
- [x] Add reconnect/retry policies for data and execution clients.
- [x] Add health snapshots for data, execution, risk, portfolio, strategy, and node lifecycle.
- [x] Add graceful shutdown on fatal queue/engine exception.

**Progress Evidence:**
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./live -run 'TestRunnerGracefullyStopsOn(FatalPlatformStreamException|StrategyEngineException)' -v` first failed because fatal platform stream and async strategy engine exceptions did not stop the live runner; it passed after `strategy.Engine` began surfacing async handler errors and `live.Runner` began graceful fatal monitoring.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./live ./strategy ./testsuite -run 'TestRunnerGracefullyStopsOn|TestLiveNodeTesterReportsParityCaseResults|TestParityScoreboardAggregatesLiveNodeGate|TestNautilusParityRequirementsCoverTargetSuites|TestNautilusMaster'` passed with `TC-LIVE09` requiring fatal runtime exceptions to stop strategies and disconnect clients.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./live ./platform ./strategy ./testsuite -run 'LiveNode|Builder|Startup|Shutdown|Reconnect|Health|TestNodeRetries|TestRunnerGracefullyStopsOn'` passed after fatal graceful shutdown landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -race -count=1 ./live ./platform ./strategy` passed after fatal graceful shutdown landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 $(env GOCACHE=/private/tmp/go-build-exchanges go list ./... | rg -v '/sdk/')` passed after fatal graceful shutdown landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./platform -run 'TestNodeRetries(MarketData|Execution)StreamRecoveryUntilPolicySucceeds'` passed after RED tests proved data/execution stream recovery needed configurable retry attempts.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./live ./testsuite -run 'LiveNode|Builder|TestLiveNodeTesterReportsParityCaseResults|TestParityScoreboardAggregatesLiveNodeGate|TestNautilusParityRequirementsCoverTargetSuites|TestNautilusMaster'` passed with `TC-LIVE08` requiring live `ReconnectPolicy` to retry data and execution recovery.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./live ./platform ./testsuite -run 'LiveNode|Builder|Startup|Shutdown|Reconnect|Health|TestNodeRetries'` passed after live retry policy propagation and platform context-bound recovery landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -race -count=1 ./live ./platform` passed after reconnect/retry recovery landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 $(env GOCACHE=/private/tmp/go-build-exchanges go list ./... | rg -v '/sdk/')` passed after reconnect/retry recovery landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./testsuite -run 'TestLiveNodeTesterReportsParityCaseResults|TestParityScoreboardAggregatesLiveNodeGate|TestNautilusParityRequirementsCoverTargetSuites'` passed after RED build failures for missing `NewLiveNodeTester` / `LiveNodeTesterConfig`.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./live ./platform ./testsuite -run 'LiveNode|Builder|Startup|Shutdown|Reconnect|Health'` passed with live parity cases `TC-LIVE01..TC-LIVE05` added to `NautilusParityRequirements`.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -race -count=1 ./live ./platform` passed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=10 ./platform -run '^TestNodeForwardsPrivateFillAndPositionEvents$'` passed after the fixture position report was made explicit about `Side: long`, removing an order/fill position-accounting race in the assertion.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 $(env GOCACHE=/private/tmp/go-build-exchanges go list ./... | rg -v '/sdk/')` passed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./live -run 'LiveNodeBuilder|NewTradingNodeReturnsCanonicalLiveNode'` passed after RED compile failures for missing `NewNodeBuilder` and `Node`; `NewTradingNode` now returns the canonical `live.Node` alias and default builds include risk, portfolio, cache, bus, and platform wiring.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./testsuite -run 'TestLiveNodeTesterReportsParityCaseResults|TestParityScoreboardAggregatesLiveNodeGate'` passed with `TC-LIVE01` exercising `live.NewNodeBuilder`.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./live ./platform ./testsuite -run 'LiveNode|Builder|Startup|Shutdown|Reconnect|Health'` passed after canonical `live.Node` / `NodeBuilder` landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -race -count=1 ./live ./platform` passed after canonical `live.Node` / `NodeBuilder` landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 $(env GOCACHE=/private/tmp/go-build-exchanges go list ./... | rg -v '/sdk/')` passed after canonical `live.Node` / `NodeBuilder` landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./live -run '^TestRunnerStopsStrategiesBeforePlatformShutdown$' -v` passed after RED showed `[data-connect exec-connect exec-disconnect data-disconnect strategy-stop]`; `Runner.Stop` now stops strategies before platform/client shutdown.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./live ./platform ./testsuite -run 'LiveNode|Builder|Startup|Shutdown|Reconnect|Health|TestRunnerStopsStrategiesBeforePlatformShutdown'` passed after shutdown ordering was fixed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -race -count=1 ./live ./platform` passed after shutdown ordering was fixed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 $(env GOCACHE=/private/tmp/go-build-exchanges go list ./... | rg -v '/sdk/')` passed after shutdown ordering was fixed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./live -run '^TestRunnerStartsPlatformBeforeStrategies$' -v` passed after RED showed strategy `OnStart` saw `platform ready=false state=initialized`; `Runner.Start` now starts the platform node before strategy engine startup.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./testsuite -run 'TestLiveNodeTesterReportsParityCaseResults|TestParityScoreboardAggregatesLiveNodeGate|TestNautilusParityRequirementsCoverTargetSuites'` passed with `TC-LIVE06` requiring `data-load -> data-connect -> exec-connect -> exec-query-account -> strategy-start`.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./live ./platform ./testsuite -run 'LiveNode|Builder|Startup|Shutdown|Reconnect|Health|TestRunnerStartsPlatformBeforeStrategies|TestRunnerStopsStrategiesBeforePlatformShutdown'` passed after startup ordering was fixed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -race -count=1 ./live ./platform` passed after startup ordering was fixed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 $(env GOCACHE=/private/tmp/go-build-exchanges go list ./... | rg -v '/sdk/')` passed after startup ordering was fixed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./live -run '^TestLiveHealthIncludesClientAndStrategySnapshots$'` passed after RED compile failures showed missing `platform.Health.Data`, `platform.Health.Execution`, and `live.Health.Strategies`; health now snapshots data clients, execution clients, risk, platform lifecycle, and strategy lifecycle states.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./testsuite -run 'TestLiveNodeTesterReportsParityCaseResults|TestParityScoreboardAggregatesLiveNodeGate|TestNautilusParityRequirementsCoverTargetSuites'` passed with `TC-LIVE07` requiring client and strategy health snapshots.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./live ./platform ./testsuite -run 'LiveNode|Builder|Startup|Shutdown|Reconnect|Health|TestLiveHealthIncludesClientAndStrategySnapshots|TestRunnerStartsPlatformBeforeStrategies|TestRunnerStopsStrategiesBeforePlatformShutdown'` passed after health snapshots landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -race -count=1 ./live ./platform` passed after health snapshots landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 $(env GOCACHE=/private/tmp/go-build-exchanges go list ./... | rg -v '/sdk/')` passed after health snapshots landed.

**Acceptance:**
- `go test -count=1 ./live ./platform ./testsuite -run 'LiveNode|Builder|Startup|Shutdown|Reconnect|Health'` passes.
- No goroutine leaks or races under `go test -race -count=1 ./live ./platform`.

## Epic 12: Adapter And SDK Capability Rollout

**Purpose:** Make every claimed exchange capability SDK-backed and parity-tested.

**Files:**
- Modify: `venue/capabilities.go`
- Modify: `testsuite/contracts.go`
- Modify: `adapter/*/*.go`
- Modify: `adapter/*/*_test.go`
- Modify: `config/all/all_test.go`
- Modify: `docs/parity/adapter-capability-matrix.md`
- Create or update: `docs/parity/<venue>-api-parity.md`

**Tasks:**
- [x] Define granular capability claims for instruments, data snapshots, streams, account snapshots, submit, cancel, modify, query, order reports, fill reports, position reports, mass status, private stream, resubscribe, and order lists.
- [x] For each adapter, map SDK-native endpoints and streams to the capability matrix.
- [x] Add local fake-SDK tests for each claimed capability.
- [x] Add shared contract tests for each capability family.
- [x] Add live read tests where public endpoints are safe by default.
- [x] Keep live write tests gated by explicit exchange-specific environment flags.
- [x] Roll out adapters in this order: Binance, OKX, Bybit, Bitget, Hyperliquid, Backpack, Aster, Lighter, Nado, EdgeX, GRVT, StandX.
- [x] For Nautilus adapters not present in this repository, create extension notes and do not claim support until SDK modules exist.

**Acceptance:**
- `go test -count=1 ./adapter/... ./config/all ./testsuite -run 'Adapter|Capability|Contract'` passes.
- `go test -run '^$' -count=1 ./sdk/...` passes.
- Every true capability has a passing test case ID; every unsupported capability returns `ErrNotSupported`.

**Progress Evidence:**
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./venue ./testsuite ./adapter/... -run 'TestAdapterCapability|Test.*ClientsPassVenueContractSuite|Test.*PrivateStream|Test.*Resubscribe' -v` first failed while new `Snapshots`, `Resubscribe`, `MassStatus`, and `OrderLists` capability fields were absent; it passed after `venue.DeclaredCapabilities` added the granular fields, shared contract checks rejected unsupported claims, and all repository adapters explicitly claimed `Snapshots` plus `Resubscribe` only where fake-SDK private-stream tests proved support.
- 2026-06-15: `docs/parity/adapter-capability-matrix.md` maps every repository adapter/product row to data snapshots, streams, private stream, explicit resubscribe, planned mass-status/order-list support, and extension-only Nautilus adapters outside the current SDK universe.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./testsuite -run '^TestAdapterLiveTestPolicyDocumentsReadAndWriteGates$' -v` first failed because the adapter live-test policy artifact was missing; it passed after `docs/parity/adapter-live-test-policy.md` documented safe-by-default public read tests, private read credential gates, live write gates, current adapter coverage, and venue-specific write flags.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./internal/testenv ./testsuite -run 'Test(RequireLive|AdapterLiveTestPolicy)' -v` passed, proving missing private-read credentials skip through `RequireLiveCredentials`, live writes skip unless their enable flag is set through `RequireLiveWrite`, and the policy document covers current adapter live-test gates.

## Epic 13: Documentation, Examples, And Migration

**Purpose:** Make the completed platform usable and auditable.

**Files:**
- Modify: `README.md`
- Modify: `README_CN.md`
- Modify: `docs/guides/stream-health.md`
- Create: `docs/guides/reconciliation.md`
- Create: `docs/guides/strategy-authoring.md`
- Create: `docs/guides/backtesting.md`
- Create: `docs/guides/live-trading.md`
- Create: `docs/guides/adapter-capabilities.md`
- Create: `examples/nautilus_style/*`

**Tasks:**
- [x] Document the master parity scorecard and how to run it.
- [x] Document strategy authoring with a bracket example.
- [x] Document live node configuration and shutdown semantics.
- [x] Document reconciliation states, counters, and unresolved discrepancy reports.
- [x] Document adapter capability policy and live-test gates.
- [x] Provide side-by-side Nautilus and Go examples for bracket strategy, portfolio query, risk rejection, backtest run, and live node assembly.

**Acceptance:**
- `go test -count=1 ./examples/...` passes where examples are Go packages.
- README links every core workflow to runnable code and contract tests.

**Progress Evidence:**
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./testsuite -run '^TestNautilusDocumentationArtifactsCoverEpic13$' -v` first failed because `docs/guides/master-parity-scorecard.md` and the related Epic 13 guide set did not exist; it passed after master parity, bracket authoring, live node, reconciliation, adapter capability, and side-by-side Nautilus/Go guides were added.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./examples/...` passed after the README workflow links, `docs/guides/*.md` user-facing entries, and validation guides were added.

## Epic 14: Release Gate And Quality Controls

**Purpose:** Prevent partial parity claims from becoming permanent.

**Files:**
- Create: `testsuite/master_gate_test.go`
- Create: `scripts/verify_nautilus_parity.sh`
- Create: `scripts/generate_nautilus_benchmark_report.sh`
- Create: `docs/parity/nautilus-complete-quality-gate.json`
- Create: `docs/parity/nautilus-release-notes-template.md`
- Create: `.omx/ultragoal/nautilus-complete-quality-gate.json`
- Modify: `docs/parity/nautilustrader-complete-feature-matrix.md`

**Tasks:**
- [x] Add master gate that fails if any required case is missing, failed, or skipped.
- [x] Add verification script that runs targeted, full non-SDK, SDK compile, race, vet, and diff hygiene checks.
- [x] Add benchmark report generation for matching, event dispatch, reconciliation, and adapter fake contract suites.
- [x] Add final code-review and architecture-review gate requirements.
- [x] Add release notes template with completed score, known unsupported external adapters, and verification evidence.

**Acceptance:**
- `bash scripts/verify_nautilus_parity.sh` passes locally.
- `go test -count=1 $(go list ./... | grep -v '/sdk')` passes.
- `go test -race -count=1 ./model ./cache ./account ./execution ./risk ./portfolio ./platform ./strategy ./backtest ./live ./testsuite` passes.
- `go test -run '^$' -count=1 ./sdk/...` passes.
- `go vet ./...` passes.
- `git diff --check` passes.

**Progress Evidence:**
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./testsuite -run 'TestNautilusMaster(Gate|TesterReportsGateCases)' -v` first failed because `NautilusMasterGateWithRequirements` did not exist; it passed after the master gate rejected missing, failed, and skipped required cases and `TC-MASTER06` became part of the master tester report.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./testsuite` passed after the master gate landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./model ./cache ./execution ./account ./platform ./live ./backtest ./portfolio ./testsuite` passed after the master gate landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -race -count=1 ./execution ./account ./platform ./live ./backtest ./portfolio ./testsuite` passed after the master gate landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 $(env GOCACHE=/private/tmp/go-build-exchanges go list ./... | rg -v '/sdk/')` passed after the master gate landed.
- 2026-06-15: `git diff --check` passed after the master gate landed.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./testsuite -run 'TestNautilusParityVerificationScriptRunsRequiredGates|TestNautilusMaster' -v` passed after the verification script contract was added.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./adapter/backpack -run '^TestPerpClientsPassVenueContractSuite$' -v` first failed because the contract suite used the live Backpack WS client; it passed after the test injected `fakeWS` like the other adapter contract suites.
- 2026-06-15: `bash scripts/verify_nautilus_parity.sh` passed locally, covering targeted master parity, full non-SDK tests, core race suites, SDK compile, `go vet ./...`, and `git diff --check`.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./testsuite -run '^TestNautilusBenchmarkReportScriptCoversRequiredBaselines$' -v` first failed because the benchmark report script did not exist; it passed after the script contract covered matching, event dispatch, reconciliation, adapter fake contract suites, `-benchmem`, report output, and required-output checks.
- 2026-06-15: `bash scripts/generate_nautilus_benchmark_report.sh` passed and wrote `.omx/reports/nautilus-benchmark-report.md` with PASS sections for matching core, bus fanout, reconciler mass status, and adapter fake contract suites.
- 2026-06-15: `env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./testsuite -run 'TestNautilus(QualityGate|ReleaseNotes)' -v` first failed because the review gate and release notes template did not exist; it passed after `docs/parity/nautilus-complete-quality-gate.json` required code-reviewer APPROVE, architect CLEAR, parity/benchmark scripts, release sections, and blocking residual-risk tags, and the release template required score, verification, benchmark, unsupported adapter, capability, and residual-risk sections.

## Execution Sequence

1. Epic 0 must run first. It defines the scoreboard and feature matrix.
2. Epics 1 and 2 unblock all other work.
3. Epics 3 and 4 define runtime shape and strategy ergonomics.
4. Epic 5 can proceed after cache and runtime contracts exist.
5. Epics 6 and 7 are the highest-risk core parity work and should be implemented before adapter rollout.
6. Epics 8 and 9 should be developed against golden scenarios, then wired through execution/backtest/live.
7. Epic 10 should reuse execution, risk, and portfolio rather than fork semantics.
8. Epic 11 assembles the production live path after engines stabilize.
9. Epic 12 rolls capabilities exchange by exchange.
10. Epics 13 and 14 close usability and release evidence.

## Stop Conditions

Do not claim "complete Go NautilusTrader" until:

- Master scorecard is 1000/1000 for mandatory scope.
- Golden Scenarios A through E pass in testsuite.
- No claimed adapter capability is untested.
- No reconciliation discrepancy can be silently ignored.
- Live and backtest strategy APIs are shared.
- Final quality gate has independent code-review and architecture-review approval.

## Recommended Goal-Mode Execution

Use `$ultragoal` as the durable owner for this program. Split this master plan into one ultragoal story per epic, with each story producing:

- failing contract tests first;
- implementation;
- targeted tests;
- race or full-suite tests where relevant;
- feature matrix update;
- ledger checkpoint with commands and evidence.

Use `$team` for parallel lanes only after Epic 0 defines stable interfaces. Good parallel splits are:

- `model/cache`
- `strategy/data`
- `execution/reconciliation`
- `risk/portfolio`
- `backtest`
- `adapters`

Ralph-style single-owner execution is useful only for final integration or stubborn verification loops after the parallel work converges.
