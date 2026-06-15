# Master Scorecard

The master scorecard is the repository's release-facing quality contract. It
turns broad platform expectations into auditable test suites across model,
cache, command metadata, strategy, data, execution, reconciliation, risk,
portfolio, backtest, live node, adapters, and documentation.

## Source Of Truth

- `testsuite` defines suite requirements, point weights, case IDs, package
  owners, and report sources.
- [Complete Feature Matrix](../parity/complete-feature-matrix.md) maps platform
  surfaces to owners and acceptance suites.
- [Adapter Capability Matrix](../parity/adapter-capability-matrix.md) records
  current venue capability claims.
- [Complete Quality Gate](../parity/complete-quality-gate.json) lists release
  review and verification requirements.

The score is intentionally case-based. A suite receives credit only when the
corresponding behavior exists, is tested, and is not skipped.

## Recommended Commands

Focused metadata and scorecard checks:

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./testsuite -run 'Master|Score|Requirement|QualityGate|ReleaseNotes' -v
```

Runtime and lifecycle checks:

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./model ./cache ./account ./execution ./risk ./portfolio ./platform ./strategy ./backtest ./live ./testsuite -v
```

Adapter capability checks:

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./venue ./adapter/... ./config/all ./testsuite -run 'Adapter|Capability|Contract|PrivateStream|Resubscribe' -v
```

Hygiene checks:

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -run '^$' -count=1 ./sdk/...
go vet ./...
git diff --check
```

## Release Rule

A release can claim a capability only when:

- the relevant scorecard case is present and passing;
- adapter declarations match implementation and contract evidence;
- reconciliation unresolved discrepancy behavior is documented;
- unsupported behavior returns explicit unsupported errors;
- verification evidence and residual risks are attached to release notes.
