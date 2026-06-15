# Reconciliation States

Reconciliation 将 venue truth 转换为本地 lifecycle state，并记录无法自动修复的
case。runtime 显式保留这些状态，使 live execution 能在 restart、private-stream gap
或 venue report mismatch 后暴露 unresolved discrepancy，而不是静默 drift。

## Core States

`execution.ReconciliationResult` 记录：

- `CaseID`；
- account 和 instrument identity；
- start/completion timestamps；
- accounts、orders、fills、positions、scanned reports、deferred fills、
  duplicate skips、lookback skips 等 counters；
- local 与 venue state 不一致时的 `Unresolved` discrepancies。

支持的 case IDs 包括 mass status、missing fills、order discrepancy、fill-before-order、
external orders、position repair 和 audit history。

## AuditTrail

`Reconciler.AuditTrail()` 返回：

- `LastResult`；
- `LastSuccess`；
- `LastError`；
- `LastErrorResult`；
- 最新 unresolved discrepancy list；
- 当前 reconciler instance 的 in-memory history。

这让 tests 和 live health checks 可以检查 restart、private-stream gap、venue report
mismatch 或 validation error 后的 recovery state。

## Unresolved Discrepancy Policy

unresolved discrepancy 不是 log-only warning，而是带 kind、account、instrument、
order/position identity 和 reason 的结构化状态。例如：

- `order_open_state_mismatch`
- `order_filled_quantity_mismatch`
- `external_order_rejected`
- `position_missing_from_venue`
- `position_quantity_mismatch`

调用方必须通过文档化 policy 修复，或把它带入 release 和 operations notes。release
gate 不允许忽略 reconciliation differences 后声称 runtime state 干净。

## Verification

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./account ./live ./testsuite -run 'Reconciliation|MissingFill|MassStatus|ExternalOrder|Discrepancy|VenueOrder|FillBeforeOrder|OpenOrder|RepairDelay|Position|RetryLimit|Audit' -v
```
