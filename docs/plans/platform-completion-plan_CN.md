# 平台完成计划

这份维护者计划追踪让仓库成为完整、capability-honest Go trading platform 所需的工作。
它比用户指南更短，并把 tests、matrices 和 quality gates 作为 release truth。

## Completion Contract

完整 release 必须满足：

- 基于 `strategy.Runtime` 的 strategy code 可在 backtest 与 live node wiring 中使用，
  command shape 不变；
- order、fill、position、account、data、lifecycle、timer callbacks typed 且 observable；
- order lists、brackets、trigger orders、reduce-only flags 和 command metadata 贯穿
  command/event path；
- risk checks 在正常 execution submission 前运行；
- fills、mark updates、account snapshots 和 reconciliation repairs 后，cache 与
  portfolio state 保持一致；
- startup、reconnect、periodic audit、missing fills、fill-before-order、external orders
  和 position discrepancies 显式且可审计；
- 每个标记 supported 的 adapter capability 都由 SDK behavior 和 contract tests 支撑；
- unsupported behavior 返回 explicit unsupported errors；
- documentation 解释如何使用平台以及如何验证 claims。

## Scorecard Domains

| Domain | Points | Core expectation |
| --- | ---: | --- |
| Domain model and identifiers | 90 | Types、validation、reports、commands、events、instruments、data round-trip。 |
| Cache and state indexes | 80 | Runtime state 可回答 order、fill、position、account、instrument、residual queries。 |
| Command envelope and message bus | 70 | Command IDs、correlation、trader、strategy、client、timestamp、params、position/list IDs 贯穿全路径。 |
| Strategy runtime and UX | 70 | Strategies 使用 typed callbacks、timers、order factory、cache、portfolio、lifecycle hooks。 |
| Data engine and catalog | 80 | Historical/replay data 与 live subscriptions 共享 normalized data semantics。 |
| Execution engine and lifecycle | 130 | Submit、modify、cancel、query、order lists、reports、contingencies、emulation、fills、positions、lifecycle transitions 被覆盖。 |
| Reconciliation | 90 | Startup、periodic、reconnect、mass-status、missing fill、external order、discrepancy repair paths 显式。 |
| Risk engine | 70 | Risk 在 execution 前拒绝 invalid/unsafe commands。 |
| Portfolio/accounting | 90 | Accounts、balances、positions、commissions、PnL、exposure、conversion、snapshots、cache invalidation 正确。 |
| Backtest engine | 80 | Deterministic venue loop、matching、advanced orders、fees、slippage、latency、reproducibility。 |
| Live node/runtime | 60 | Config、wiring、retry、reconnect、shutdown、health、observability complete。 |
| Adapters and SDK parity | 70 | 每个 claimed venue capability SDK-backed 且 contract-tested。 |
| Documentation and examples | 20 | Guides、examples、reports、matrices、release notes 可执行且诚实。 |

## Workstreams

| Workstream | Owner packages | Current expectation |
| --- | --- | --- |
| Model and command envelope | `model`, `bus`, `kernel` | Preserve identifiers、command metadata、validation、event round trips。 |
| Runtime state | `cache`, `portfolio` | Cache indexes 与 portfolio accounting event-driven 且 deduplicated。 |
| Strategy and data | `strategy`, `data`, `backtest`, `live` | Authoring APIs 在 simulation/live node wiring 中一致。 |
| Execution and reconciliation | `execution`, `account`, `platform`, `live` | Preserve lifecycle state、repair gaps、expose unresolved discrepancies。 |
| Risk | `risk`, `platform`, `backtest`, `live` | Reject before normal execution，并防止 rejection 后 downstream mutation。 |
| Adapters and SDKs | `sdk/*`, `adapter/*`, `venue`, `config/all` | Declared capabilities 有 implementation 和 tests 支撑。 |
| Documentation | `README.md`, `README_CN.md`, `docs/`, `examples/` | Newcomer docs、module docs、recipes、quality evidence 保持当前。 |

## Required Evidence

- [完整功能矩阵](../parity/complete-feature-matrix_CN.md)
- [Adapter 能力矩阵](../parity/adapter-capability-matrix_CN.md)
- [完整质量门](../parity/complete-quality-gate_CN.json)
- [发布说明模板](../parity/release-notes-template_CN.md)
- `testsuite`

## Verification

开发时运行 focused tests。release claim 前运行
[完整质量门](../parity/complete-quality-gate_CN.json) 中列出的命令，并把输出附到
release notes。

## Documentation Rule

README files 承载项目定位和历史设计背景。docs site 承载这个 Go platform 的使用、架构、
模块、运行和验证指南。
