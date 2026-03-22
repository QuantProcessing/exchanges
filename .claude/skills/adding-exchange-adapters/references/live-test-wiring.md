# Live Test Wiring

Live adapter integration is incomplete until `adapter_test.go` wires the shared `testsuite` coverage for the support level the adapter claims.

Backpack is the current reference pattern: [`backpack/adapter_test.go`](/home/xiguajun/Documents/GitHub/Exchanges/.worktrees/skill-adding-exchange-adapters/backpack/adapter_test.go).

## `.env` Lookup Pattern

Prefer the Backpack-style helper in `adapter_test.go`:

```go
func loadExchangeEnv() {
    for _, path := range []string{".env", "../.env", "../../.env", "../../../.env"} {
        if err := godotenv.Load(path); err == nil {
            return
        }
    }
}
```

Why this pattern:

- works from a git worktree
- works when tests are launched from different package directories
- avoids brittle `../../.env` assumptions still present in older adapters

Do not hard-code one relative path when adding a new adapter.

## `.env.example` Additions

Update the repository root [`.env.example`](/home/xiguajun/Documents/GitHub/Exchanges/.worktrees/skill-adding-exchange-adapters/.env.example) when a new adapter is added.

Add:

- required credentials
- any required account identifiers or passphrases
- `<EXCHANGE>_SPOT_TEST_SYMBOL` and/or `<EXCHANGE>_PERP_TEST_SYMBOL` when those markets are supported
- `<EXCHANGE>_QUOTE_CURRENCY` when the adapter exposes quote configuration

If the adapter only supports one market, do not add unused env vars just for symmetry.

## Environment Variable Naming

Use uppercase exchange-prefixed names that match the adapter's `Options` fields and market split:

- `BACKPACK_API_KEY`
- `BACKPACK_PRIVATE_KEY`
- `BACKPACK_PERP_TEST_SYMBOL`
- `BACKPACK_SPOT_TEST_SYMBOL`
- `BACKPACK_QUOTE_CURRENCY`

Guidance:

- keep auth names aligned with `options.go`
- distinguish spot and perp test symbols when both exist
- use one quote env var when the adapter has one shared quote setting
- avoid vague names like `TEST_SYMBOL` or `QUOTE`

## `adapter_test.go` And Shared Suite Matrix

Wire the shared suites to the adapter's declared capability level:

- `RunAdapterComplianceTests` for every adapter with live coverage
- `RunOrderSuite` for trading-capable adapters
- `RunLifecycleSuite` when `WatchOrders` is real and lifecycle correctness is claimed
- `RunLocalStateSuite` when `FetchAccount` plus `WatchOrders` make LocalState support real

Backpack shows the expected shape:

- setup helper per market
- `requireEnvSymbol` helper
- explicit shared-suite tests for perp and spot variants
- `OrderSuiteConfig` skip flags set only where the exchange behavior requires them

If these shared suites are not wired in `adapter_test.go`, the live integration is not done.

## Skip Guidance

Use explicit skips for missing live prerequisites:

- missing credentials
- missing test symbol env vars
- market variant not supported by that package

Prefer targeted skip flags in `testsuite.OrderSuiteConfig` only for real exchange limitations:

- `SkipSlippage` if the exchange or market does not support the repository's slippage path
- `SkipLimit` or `SkipMarket` only when that order type is genuinely unsupported or unstable for that market

Do not skip a suite just because the adapter implementation is incomplete. Fix the adapter or reduce the support claim.

## Stable Symbol And Quote Selection

Choose live-test defaults that are stable over time:

- use a liquid base symbol already supported by the target market
- prefer symbols with predictable availability in the exchange's default quote
- match the quote default defined in the adapter's `options.go`
- avoid ephemeral listings, promo markets, or symbols that only exist on one side of spot/perp support

When spot and perp need different symbols, name them separately and wire them separately.

## Definition Of Complete Live Wiring

Live integration is complete only when all of these are true:

- `.env.example` documents the required env vars
- `adapter_test.go` loads `.env` with the resilient helper
- setup helpers construct the real adapter using env-backed options
- the shared `testsuite` matrix in `adapter_test.go` matches the claimed capability level
- unsupported capabilities are reflected by missing suite wiring or explicit suite-level skip flags with a real reason

Anything less is partial wiring, not finished integration.
