# Documentation

This directory is the public documentation home for the Go trading platform.
It is written for someone who has never seen the repository before and needs to
understand what to install, which package to import, how the modules fit
together, and how to build a reliable quantitative trading program.

## Start Here

- [Getting Started](./getting-started.md): install the module, fetch market
  data through an adapter, run a strategy in a deterministic backtest, and
  assemble a live node.
- [Module Guide](./module-guide.md): detailed responsibilities for every major
  package: `model`, `venue`, `adapter`, `sdk`, `cache`, `data`, `execution`,
  `account`, `risk`, `portfolio`, `strategy`, `backtest`, `live`, `platform`,
  `bus`, `kernel`, and `testsuite`.
- [Runtime Flow](./runtime-flow.md): how market data, orders, fills,
  portfolio updates, reconciliation, and strategy callbacks move through the
  system.
- [Project Architecture](./architecture.md): the architectural boundaries and
  design rules that keep SDKs, adapters, and runtime code separated.
- [Examples](../examples/README.md): compiled recipes that the guides link to
  when they introduce adapter, order factory, risk, backtest, bracket, and live
  node concepts.

## Build Trading Programs

- [Strategy Authoring](./guides/strategy-authoring.md): write strategies with
  typed callbacks and `strategy.Runtime`.
- [Reliable Quant Program Guide](./guides/reliable-quant-program.md): the
  practical checklist for strategy state, risk, portfolio, reconciliation,
  testing, and production rollout.
- [Backtesting](./guides/backtesting.md): run deterministic strategy research
  and simulation workflows.
- [Live Trading](./guides/live-trading.md): assemble and operate a live node.
- [Live Node Configuration](./guides/live-node-configuration.md): builder
  options, startup order, shutdown semantics, and health.
- [Strategy Authoring With Brackets](./guides/strategy-authoring-bracket.md):
  create entry, take-profit, and stop-loss order lists.
- [Workflow Recipes](./guides/workflow-recipes.md): bracket orders, portfolio
  queries, risk rejection, backtest runs, and live node assembly.
- [Quant Use Cases](./guides/quant-use-cases.md): practical strategy shapes
  including market making, portfolio monitoring, reconciliation-aware live
  trading, and cross-venue funding-rate arbitrage.

## Operate And Verify

- [Adapter Capabilities](./guides/adapter-capabilities.md): understand what a
  venue adapter currently claims to support.
- [Adapter Capability Policy](./guides/adapter-capability-policy.md): rules for
  adding or downgrading adapter capabilities.
- [Adapter Live Test Policy](./parity/adapter-live-test-policy.md): credential
  and live-write test boundaries.
- [Reconciliation](./guides/reconciliation.md): repair local state after
  startup gaps, stream disconnects, and venue discrepancies.
- [Reconciliation States](./guides/reconciliation-states.md): result counters,
  unresolved discrepancy states, and audit trail behavior.
- [Stream Health](./guides/stream-health.md): how data and execution stream
  health is surfaced.
- [Master Scorecard](./guides/master-scorecard.md): how the repository proves
  runtime, adapter, and documentation behavior with tests.

## Internal Quality Evidence

These files are meant for maintainers and release reviewers. New users should
start with the guides above.

- [Complete Feature Matrix](./parity/complete-feature-matrix.md)
- [Adapter Capability Matrix](./parity/adapter-capability-matrix.md)
- [Complete Quality Gate](./parity/complete-quality-gate.json)
- [Release Notes Template](./parity/release-notes-template.md)
- [Platform Completion Plan](./plans/platform-completion-plan.md)
