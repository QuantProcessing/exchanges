# Reconciliation

Reconciliation applies venue truth to local cache, execution state, and
portfolio-facing reports after restarts, private-stream gaps, delayed fills, or
external venue activity.

The core API is `execution.Reconciler`, which records `ReconciliationResult`
counters and exposes `AuditTrail()` for last result, last success, last error,
history, and unresolved discrepancies.

Operational guide:

- [Reconciliation States](./superpowers/guides/reconciliation-states.md)

Verification:

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./account ./live ./testsuite -run 'Reconciliation|MissingFill|MassStatus|ExternalOrder|Discrepancy|Audit' -v
```
