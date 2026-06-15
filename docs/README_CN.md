# 文档

这个目录是 Go 交易平台的中文文档入口。它面向第一次看到仓库的人，帮助你理解
如何安装、应该 import 哪一层、各模块职责是什么，以及如何用当前库写一个可靠
的量化交易程序。

## 从这里开始

- [快速开始](./getting-started_CN.md)：安装模块，通过 adapter 拉取行情，在确定性
  回测中运行策略，并组装 live node。
- [模块指南](./module-guide_CN.md)：详细说明 `model`、`venue`、`adapter`、
  `sdk`、`cache`、`data`、`execution`、`account`、`risk`、`portfolio`、
  `strategy`、`backtest`、`live`、`platform`、`bus`、`kernel`、`testsuite`
  的职责。
- [运行时流程](./runtime-flow_CN.md)：行情、订单、成交、组合更新、
  reconciliation 和策略回调如何在系统中流动。
- [项目架构](./architecture_CN.md)：SDK、adapter、runtime 三层边界和设计规则。

## 构建交易程序

- [策略编写](./guides/strategy-authoring_CN.md)：使用 typed callbacks 和
  `strategy.Runtime` 编写策略。
- [可靠量化程序指南](./guides/reliable-quant-program_CN.md)：策略状态、风控、
  组合、reconciliation、测试和上线的实践清单。
- [回测](./guides/backtesting_CN.md)：运行确定性的策略研究和仿真流程。
- [实盘交易](./guides/live-trading_CN.md)：组装并运行 live node。
- [Live Node 配置](./guides/live-node-configuration_CN.md)：builder 选项、启动顺序、
  关闭语义和健康状态。
- [Bracket 策略编写](./guides/strategy-authoring-bracket_CN.md)：创建 entry、
  take-profit 和 stop-loss order list。
- [工作流示例](./guides/workflow-recipes_CN.md)：bracket orders、portfolio 查询、
  risk rejection、backtest run 和 live node assembly。

## 运行与验证

- [Adapter 能力](./guides/adapter-capabilities_CN.md)：理解当前 venue adapter 声明
  支持哪些能力。
- [Adapter 能力策略](./guides/adapter-capability-policy_CN.md)：新增或降级 adapter
  capability 的规则。
- [Adapter Live Test Policy](./parity/adapter-live-test-policy_CN.md)：credential 和
  live-write 测试边界。
- [Reconciliation](./guides/reconciliation_CN.md)：在启动 gap、stream disconnect
  和 venue discrepancy 后修复本地状态。
- [Reconciliation States](./guides/reconciliation-states_CN.md)：结果计数器、
  unresolved discrepancy 和 audit trail。
- [Stream Health](./guides/stream-health_CN.md)：data/execution stream health 如何暴露。
- [主评分卡](./guides/master-scorecard_CN.md)：仓库如何用测试证明 runtime、adapter
  和文档行为。

## 内部质量证据

这些文件主要给维护者和 release reviewer 使用。新用户应先阅读上面的指南。

- [完整功能矩阵](./parity/complete-feature-matrix_CN.md)
- [Adapter 能力矩阵](./parity/adapter-capability-matrix_CN.md)
- [完整质量门](./parity/complete-quality-gate_CN.json)
- [发布说明模板](./parity/release-notes-template_CN.md)
- [平台完成计划](./plans/platform-completion-plan_CN.md)
