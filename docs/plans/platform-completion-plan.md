# Platform Completion Plan

This maintainer plan tracks the work needed to make the repository a complete,
capability-honest Go trading platform. It is intentionally shorter than the
user-facing guides and points to tests, matrices, and quality gates as the
source of release truth.

## Completion Contract

A complete release must satisfy these conditions:

- strategy code written against `strategy.Runtime` runs in both backtest and
  live node wiring without changing its command shape;
- order, fill, position, account, data, lifecycle, and timer callbacks are
  typed and observable;
- order lists, brackets, trigger orders, reduce-only flags, and command
  metadata survive the full command/event path;
- risk checks run before normal execution submission;
- cache and portfolio state remain consistent after fills, mark updates,
  account snapshots, and reconciliation repairs;
- startup, reconnect, periodic audit, missing fills, fill-before-order, external
  orders, and position discrepancies are explicit and auditable;
- every adapter capability marked supported is backed by SDK behavior and
  contract tests;
- unsupported behavior returns explicit unsupported errors;
- documentation explains how to use the platform and how to verify claims.

## Scorecard Domains

| Domain | Points | Core expectation |
| --- | ---: | --- |
| Domain model and identifiers | 90 | Types, validation, reports, commands, events, instruments, and data round-trip. |
| Cache and state indexes | 80 | Runtime state answers order, fill, position, account, instrument, and residual queries. |
| Command envelope and message bus | 70 | Command IDs, correlation, trader, strategy, client, timestamp, params, and position/list IDs survive all paths. |
| Strategy runtime and UX | 70 | Strategies use typed callbacks, timers, order factory, cache, portfolio, and lifecycle hooks. |
| Data engine and catalog | 80 | Historical/replay data and live subscriptions share normalized data semantics. |
| Execution engine and lifecycle | 130 | Submit, modify, cancel, query, order lists, reports, contingencies, emulation, fills, positions, and lifecycle transitions are covered. |
| Reconciliation | 90 | Startup, periodic, reconnect, mass-status, missing fill, external order, and discrepancy repair paths are explicit. |
| Risk engine | 70 | Risk rejects invalid or unsafe commands before execution. |
| Portfolio/accounting | 90 | Accounts, balances, positions, commissions, PnL, exposure, conversion, snapshots, and cache invalidation are correct. |
| Backtest engine | 80 | Deterministic venue loop, matching, advanced orders, fees, slippage, latency, and reproducibility pass. |
| Live node/runtime | 60 | Config, wiring, retry, reconnect, shutdown, health, and observability are complete. |
| Adapters and SDK parity | 70 | Every claimed venue capability is SDK-backed and contract-tested. |
| Documentation and examples | 20 | Guides, examples, reports, matrices, and release notes stay executable and honest. |

## Workstreams

| Workstream | Owner packages | Current expectation |
| --- | --- | --- |
| Model and command envelope | `model`, `bus`, `kernel` | Preserve identifiers, command metadata, validation, and event round trips. |
| Runtime state | `cache`, `portfolio` | Keep cache indexes and portfolio accounting event-driven and deduplicated. |
| Strategy and data | `strategy`, `data`, `backtest`, `live` | Keep authoring APIs identical across simulation and live node wiring. |
| Execution and reconciliation | `execution`, `account`, `platform`, `live` | Preserve lifecycle state, repair gaps, and expose unresolved discrepancies. |
| Risk | `risk`, `platform`, `backtest`, `live` | Reject before normal execution and prevent downstream mutation on rejection. |
| Adapters and SDKs | `sdk/*`, `adapter/*`, `venue`, `config/all` | Keep declared capabilities backed by implementation and tests. |
| Documentation | `README.md`, `README_CN.md`, `docs/`, `examples/` | Keep newcomer docs, module docs, recipes, and quality evidence current. |

## Required Evidence

- [Complete Feature Matrix](../parity/complete-feature-matrix.md)
- [Adapter Capability Matrix](../parity/adapter-capability-matrix.md)
- [Complete Quality Gate](../parity/complete-quality-gate.json)
- [Release Notes Template](../parity/release-notes-template.md)
- `testsuite`

## Verification

Run focused tests while developing. Before a release claim, run the quality gate
commands listed in [Complete Quality Gate](../parity/complete-quality-gate.json)
and attach the output to release notes.

## Documentation Rule

README files carry project positioning and historical design context. The docs
site carries usage, architecture, module, operation, and verification guidance
for this Go platform.
