# Go NautilusTrader Release Notes

Release: `<version or commit>`
Date: `<YYYY-MM-DD>`

## Completed Score

- Master scorecard: `<points>/1000`
- Mandatory scope status: `<complete|blocked>`
- Golden scenarios: `<A-E pass/fail summary>`

## Verification Evidence

- `bash scripts/verify_nautilus_parity.sh`: `<pass/fail, date, operator>`
- Targeted master gate: `<command and result>`
- Full non-SDK suite: `<command and result>`
- Core race suites: `<command and result>`
- SDK compile gate: `<command and result>`
- `go vet ./...`: `<result>`
- `git diff --check`: `<result>`

## Benchmark Evidence

- `bash scripts/generate_nautilus_benchmark_report.sh`: `<pass/fail, date, operator>`
- Report path: `.omx/reports/nautilus-benchmark-report.md`
- Matching core baseline: `<ns/op, B/op, allocs/op>`
- Event dispatch baseline: `<ns/op, B/op, allocs/op>`
- Reconciliation baseline: `<ns/op, B/op, allocs/op>`
- Adapter fake contract suite status: `<pass/fail summary>`

## Known Unsupported External Adapters

List NautilusTrader providers outside this repository's SDK universe, such as Interactive Brokers, Databento, Betfair, Deribit, Kraken, dYdX, BitMEX, Polymarket, Tardis, or sandbox providers, and state whether they are unavailable, planned, or represented only by extension notes.

## Adapter Capability Changes

- Claimed capabilities added: `<venue/product/capability/evidence>`
- Claimed capabilities removed or downgraded: `<venue/product/capability/reason>`
- Live write gates: `<exchange-specific flags required>`

## Residual Risks

- Blocking risks: `<none or list critical/high issues>`
- Non-blocking risks: `<medium/low caveats>`
- Reconciliation limitations: `<none or explicit discrepancy policies>`
- Portfolio/risk limitations: `<none or explicit policy limitations>`
