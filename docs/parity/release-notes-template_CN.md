# 交易平台发布说明

Release: `<version or commit>`
Date: `<YYYY-MM-DD>`

## Completed Score

- Master scorecard: `<points>/1000`
- Mandatory scope status: `<complete|blocked>`
- Golden scenarios: `<A-E pass/fail summary>`

## Verification Evidence

- Scorecard metadata: `<command and result>`
- Full non-SDK suite: `<command and result>`
- Core race suites: `<command and result>`
- SDK compile gate: `<command and result>`
- `go vet ./...`: `<result>`
- `git diff --check`: `<result>`

## Benchmark Evidence

- Matching core baseline: `<ns/op, B/op, allocs/op>`
- Event dispatch baseline: `<ns/op, B/op, allocs/op>`
- Reconciliation baseline: `<ns/op, B/op, allocs/op>`
- Adapter fake contract suite status: `<pass/fail summary>`

## Known Unsupported Extension Adapters

列出当前 repository SDK universe 之外的 providers，并说明它们是 unavailable、planned，
还是仅有 extension notes。

## Adapter Capability Changes

- Claimed capabilities added: `<venue/product/capability/evidence>`
- Claimed capabilities removed or downgraded: `<venue/product/capability/reason>`
- Live write gates: `<exchange-specific flags required>`

## Residual Risks

- Blocking risks: `<none or list critical/high issues>`
- Non-blocking risks: `<medium/low caveats>`
- Reconciliation limitations: `<none or explicit discrepancy policies>`
- Portfolio/risk limitations: `<none or explicit policy limitations>`
