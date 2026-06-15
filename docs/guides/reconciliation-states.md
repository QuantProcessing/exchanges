# Reconciliation States

Reconciliation turns venue truth into local lifecycle state and records every
case that cannot be resolved automatically. The Go replica keeps this behavior
explicit so live execution can restart, recover missed private-stream events,
and report an unresolved discrepancy instead of silently drifting.

## Core States

`execution.ReconciliationResult` records:

- `CaseID` for the reconciliation scenario;
- account and instrument identity;
- start and completion timestamps;
- counters for accounts, orders, fills, positions, scanned reports, deferred
  fills, duplicate skips, and lookback skips;
- `Unresolved` discrepancies when local and venue state disagree.

Supported case IDs include mass status, missing fills, order discrepancy,
fill-before-order, external orders, position repair, and audit history.

## AuditTrail

`Reconciler.AuditTrail()` returns a snapshot with:

- `LastResult`;
- `LastSuccess`;
- `LastError`;
- `LastErrorResult`;
- the latest unresolved discrepancy list;
- full in-memory history for the current reconciler instance.

This gives tests and live health checks a concrete place to inspect recovery
state after a restart, private-stream gap, venue report mismatch, or validation
error.

## Unresolved Discrepancy Policy

An unresolved discrepancy is not a log-only warning. It is structured state with
kind, account, instrument, order or position identity, and reason. Examples
include:

- `order_open_state_mismatch`;
- `order_filled_quantity_mismatch`;
- `external_order_rejected`;
- `position_missing_from_venue`;
- `position_quantity_mismatch`.

Callers must either repair the discrepancy through a documented policy or carry
it into release and operations notes. The release gate forbids claiming clean
parity while reconciliation differences are being ignored.

## Verification

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./execution ./account ./live ./testsuite -run 'Reconciliation|MissingFill|MassStatus|ExternalOrder|Discrepancy|VenueOrder|FillBeforeOrder|OpenOrder|RepairDelay|Position|RetryLimit|Audit' -v
```
