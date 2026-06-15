# Reconciliation

Reconciliation repairs local runtime state with venue truth after startup,
private stream gaps, reconnects, delayed reports, fill-before-order events,
external venue activity, and stale positions.

The goal is not to hide disagreement. The goal is to converge when the repair
is safe and record structured unresolved discrepancies when it is not.

## Where Reconciliation Runs

Reconciliation appears in two places:

- `account.Reconciler` repairs account-facing order, fill, and position state;
- execution/platform reconciliation asks `venue.ExecutionClient` for reports
  and applies the resulting events to cache and portfolio.

`platform.Node` invokes reconciliation during startup and stream recovery where
the client capabilities allow it.

## Result Shape

`execution.ReconciliationResult` records:

- case ID;
- account and instrument identity;
- start and completion timestamps;
- counters for scanned orders, fills, positions, duplicates, deferred fills,
  skipped recent activity, imported external orders, and unresolved cases;
- unresolved discrepancy details.

`Reconciler.AuditTrail()` exposes last result, last success, last error, latest
unresolved discrepancies, and in-memory history for the current reconciler.

## Common Scenarios

| Scenario | Expected behavior |
| --- | --- |
| Missing fill | Query fill reports when available, apply only missing trade IDs, skip duplicates. |
| Fill before order | Defer the fill, index it by venue order or trade identity, replay once the order report appears. |
| Missing open order | Respect recent activity thresholds before marking local state canceled or unresolved. |
| External order | Import only under an explicit policy; otherwise record an unresolved discrepancy. |
| Position mismatch | Retry under policy, repair when safe, record unresolved state when not safe. |
| Stream reconnect | Resubscribe if supported, then query reports needed to close gaps. |

## Operational Rules

- Do not turn discrepancies into log-only warnings.
- Do not apply a fill twice.
- Do not invent missing identifiers silently.
- Do not treat private stream support as the same thing as resubscribe or mass
  status support.
- Surface unresolved state in tests, health, and release notes.

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

## More Detail

- [Reconciliation States](./reconciliation-states.md)
- [Runtime Flow](../runtime-flow.md)

## Verification

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./account ./live ./testsuite -run 'Reconciliation|MissingFill|MassStatus|ExternalOrder|Discrepancy|VenueOrder|FillBeforeOrder|OpenOrder|RepairDelay|Position|RetryLimit|Audit' -v
```
