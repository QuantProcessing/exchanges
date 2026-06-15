# Reconciliation

Reconciliation 在 startup、private stream gaps、reconnects、delayed reports、
fill-before-order events、external venue activity 和 stale positions 之后，用 venue
truth 修复本地 runtime state。

目标不是隐藏差异，而是在安全时收敛，并在不能安全修复时记录结构化 unresolved
discrepancies。

## 运行位置

- `account.Reconciler` 修复 account-facing order、fill、position state；
- execution/platform reconciliation 通过 `venue.ExecutionClient` 查询 reports，并将
  events 应用到 cache 和 portfolio。

`platform.Node` 会在 startup 与 stream recovery 中根据 client capabilities 调用
reconciliation。

## Result Shape

`execution.ReconciliationResult` 记录 case ID、account/instrument identity、开始和
完成时间、orders/fills/positions/duplicates/deferred fills/skipped activity/imported
external orders/unresolved counters，以及 unresolved discrepancy details。

`Reconciler.AuditTrail()` 暴露 last result、last success、last error、latest unresolved
discrepancies 和当前 reconciler 的 in-memory history。

## 常见场景

| 场景 | 期望行为 |
| --- | --- |
| Missing fill | 查询 fill reports，只应用缺失 trade IDs，跳过 duplicates。 |
| Fill before order | defer fill，按 venue order/trade identity 索引，order 出现后 replay。 |
| Missing open order | 尊重 recent activity thresholds，再决定 cancel 或 unresolved。 |
| External order | 仅在显式 policy 下 import，否则记录 unresolved discrepancy。 |
| Position mismatch | 按 policy retry，安全时 repair，不安全时记录 unresolved state。 |
| Stream reconnect | 支持时 resubscribe，然后查询 reports 关闭 gaps。 |

## Operational Rules

- 不要把 discrepancies 变成 log-only warnings。
- 不要重复应用 fill。
- 不要静默发明缺失 identifiers。
- private stream、resubscribe、mass status 是不同能力。
- unresolved state 应出现在 tests、health 和 release notes 中。

## Example Inspection

```go
trail := reconciler.AuditTrail()
if len(trail.Unresolved) > 0 {
    for _, d := range trail.Unresolved {
        logger.Warn("unresolved reconciliation discrepancy",
            "kind", d.Kind,
            "account", d.AccountID,
            "instrument", d.InstrumentID,
            "reason", d.Reason,
        )
    }
}
```

更多信息：

- [Reconciliation States](./reconciliation-states_CN.md)
- [运行时流程](../runtime-flow_CN.md)

## Verification

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./account ./live ./testsuite -run 'Reconciliation|MissingFill|MassStatus|ExternalOrder|Discrepancy|VenueOrder|FillBeforeOrder|OpenOrder|RepairDelay|Position|RetryLimit|Audit' -v
```
