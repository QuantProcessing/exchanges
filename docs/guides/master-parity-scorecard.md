# Master Parity Scorecard

The master parity scorecard is the release-facing contract for the Go
NautilusTrader replica. It turns the broad target into 1,000 auditable points
across model, cache, command, strategy, data, execution, reconciliation, risk,
portfolio, backtest, live, adapter, and documentation suites.

## Source Of Truth

- `testsuite.NautilusMasterRequirements` defines every suite, point weight,
  required case ID, golden scenario mapping, Go package, and report source.
- `testsuite.NewNautilusMasterTester().Run(t)` checks that the requirement
  table remains complete and internally consistent.
- `testsuite.NautilusMasterGate(...)` fails if any required case is missing,
  failed, or skipped.
- `docs/plans/nautilustrader-complete-replica.md`
  records implementation evidence and remaining work.

The score is intentionally binary at the case level. A suite cannot receive
credit for a required behavior until the corresponding contract case is present
and passing.

## Required Commands

Run the full local gate before claiming parity progress:

```bash
bash scripts/verify_nautilus_parity.sh
```

The script runs targeted master parity tests, full non-SDK tests, race-sensitive
core suites, SDK compile-only checks, `go vet ./...`, and `git diff --check`.

For focused development, run the master metadata tests directly:

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./testsuite -run 'TestNautilusMaster' -v
```

Benchmark evidence is generated separately:

```bash
bash scripts/generate_nautilus_benchmark_report.sh
```

The benchmark script writes `.omx/reports/nautilus-benchmark-report.md` and
must include matching-core, bus fanout, reconciler mass-status, and adapter
fake-contract evidence.

## Release Rule

A release note may call the project Nautilus-compatible only when:

- every `NautilusMasterRequirements` required case is passing;
- the adapter capability matrix does not over-claim implementation;
- reconciliation unresolved discrepancy behavior is documented;
- benchmark and quality-gate evidence are attached;
- unsupported external Nautilus adapters are listed as planned or external,
  not implied as supported.
