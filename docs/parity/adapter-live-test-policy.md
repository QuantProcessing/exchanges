# Adapter Live Test Policy

This document defines the live-test boundary for adapter and SDK parity work.
It prevents public read coverage from being hidden behind credentials and
prevents live write tests from mutating exchange state unless the operator has
explicitly opted in.

## Public live read tests

Public live read tests may run by default when they call official public
endpoints and do not mutate exchange state. They should use short timeouts,
validate response shape rather than market direction, and skip only transient
network failures through `internal/testenv.SkipIfTransientLiveNetworkError`.

Representative safe commands:

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./sdk/binance/spot -run 'TestClient_Get|TestMarket' -v
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./sdk/bybit -run 'TestPublic|TestClient' -v
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./sdk/bitget -run 'TestPublic|TestClient' -v
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./sdk/backpack -run 'TestPublic' -v
```

Private live read tests may require credentials, but they must use
`internal/testenv.RequireLiveCredentials` and skip clearly when credentials are
missing. They must not place, modify, cancel, transfer, or otherwise mutate
venue state.

## Live write tests

Live write tests include any test that can place, cancel, modify, transfer,
start/close private sessions with venue-side state, or otherwise mutate exchange
state. They must use `internal/testenv.RequireLiveWrite`, an exchange-specific
enable flag, and all required credentials. They must skip by default.

| Venue or SDK scope | Public read default | Live write gate | Required credentials |
| --- | --- | --- | --- |
| Binance Spot | `sdk/binance/spot` public REST and market reads | `BINANCE_ENABLE_LIVE_WRITE_TESTS` | `BINANCE_API_KEY`, `BINANCE_SECRET_KEY` |
| Binance Perp | `sdk/binance/perp` public REST and market reads | `BINANCE_PERP_ENABLE_LIVE_WRITE_TESTS` | `BINANCE_API_KEY`, `BINANCE_SECRET_KEY` |
| Binance Margin/Subaccount | read helpers must use credentials only for private reads | `BINANCE_ENABLE_LIVE_WRITE_TESTS` | `BINANCE_API_KEY`, `BINANCE_SECRET_KEY` |
| OKX | `sdk/okx` public REST and market reads | `OKX_ENABLE_LIVE_WRITE_TESTS` | `OKX_API_KEY`, `OKX_API_SECRET`, `OKX_API_PASSPHRASE` |
| Bybit | `sdk/bybit` public REST and public WS reads | `BYBIT_ENABLE_LIVE_WRITE_TESTS` | `BYBIT_API_KEY`, `BYBIT_SECRET_KEY` |
| Bitget | `sdk/bitget` public REST and public WS reads | `BITGET_ENABLE_LIVE_WRITE_TESTS` | `BITGET_API_KEY`, `BITGET_SECRET_KEY`, `BITGET_PASSPHRASE` |
| Hyperliquid Spot/Perp | `sdk/hyperliquid`, `sdk/hyperliquid/spot`, and `sdk/hyperliquid/perp` public reads | `HYPERLIQUID_ENABLE_LIVE_WRITE_TESTS` | `HYPERLIQUID_PRIVATE_KEY` plus account variables required by the package |
| Lighter | `sdk/lighter` public reads and protocol method checks | `LIGHTER_ENABLE_LIVE_WRITE_TESTS` | `LIGHTER_API_KEY`, `LIGHTER_ACCOUNT_INDEX`, `LIGHTER_PRIVATE_KEY` or package-specific signer vars |
| Backpack | `sdk/backpack` public REST reads | No live write test claimed today; add `BACKPACK_ENABLE_LIVE_WRITE_TESTS` before any mutating test. | Package-specific credentials |
| Aster | `sdk/aster/spot` and `sdk/aster/perp` public reads | No live write test claimed today; add `ASTER_ENABLE_LIVE_WRITE_TESTS` before any mutating test. | Package-specific credentials |
| Nado | `sdk/nado` public and private integration tests | No standard write gate claimed today; mutating tests must be moved behind `NADO_ENABLE_LIVE_WRITE_TESTS`. | Package-specific credentials |
| EdgeX | `sdk/edgex/perp` public reads | No live write test claimed today; add `EDGEX_ENABLE_LIVE_WRITE_TESTS` before any mutating test. | Package-specific credentials |
| GRVT | `sdk/grvt` public reads | No live write test claimed today; add `GRVT_ENABLE_LIVE_WRITE_TESTS` before any mutating test. | Package-specific credentials |
| StandX | `sdk/standx` public reads | No live write test claimed today; add `STANDX_ENABLE_LIVE_WRITE_TESTS` before any mutating test. | Package-specific credentials |

## Adapter coverage rule

Adapter capability tests may use fake clients and shared contract suites by
default. Live adapter tests are additive evidence only:

- public read paths can run by default when no state mutation occurs;
- private read paths must use `internal/testenv.RequireLiveCredentials`;
- live write paths must use `internal/testenv.RequireLiveWrite`;
- write flags must be exchange-specific and named in the package test helper;
- unsupported live coverage must remain a documented gap, not a capability
  claim.

This policy applies to Binance, OKX, Bybit, Bitget, Hyperliquid, Backpack,
Aster, Lighter, Nado, EdgeX, GRVT, and StandX until their SDK packages define a
stricter local policy.
