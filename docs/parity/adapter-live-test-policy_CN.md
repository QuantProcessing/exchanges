# Adapter Live Test Policy

本文定义 adapter 与 SDK live-test 边界：public read coverage 不能被 credentials 隐藏；
live write tests 不能在 operator 未显式 opt-in 时 mutate exchange state。

## Public live read tests

当测试调用官方 public endpoints 且不 mutate exchange state 时，可以默认运行。测试应使用
短 timeout，验证 response shape，而不是市场方向。临时网络问题可通过
`internal/testenv.SkipIfTransientLiveNetworkError` skip。

代表性安全命令：

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./sdk/binance/spot -run 'TestClient_Get|TestMarket' -v
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./sdk/bybit -run 'TestPublic|TestClient' -v
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./sdk/bitget -run 'TestPublic|TestClient' -v
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./sdk/backpack -run 'TestPublic' -v
```

private live read tests 可能需要 credentials，但必须使用
`internal/testenv.RequireLiveCredentials`，缺失时清晰 skip。它们不能 place、modify、
cancel、transfer 或 mutate venue state。

## Live write tests

live write tests 包括任何可能 place、cancel、modify、transfer、start/close private
sessions with venue-side state，或 mutate exchange state 的测试。它们必须使用
`internal/testenv.RequireLiveWrite`、exchange-specific enable flag 和全部 required
credentials，默认必须 skip。

| Venue or SDK scope | Public read default | Live write gate | Required credentials |
| --- | --- | --- | --- |
| Binance Spot | `sdk/binance/spot` public REST and market reads | `BINANCE_ENABLE_LIVE_WRITE_TESTS` | `BINANCE_API_KEY`, `BINANCE_SECRET_KEY` |
| Binance Perp | `sdk/binance/perp` public REST and market reads | `BINANCE_PERP_ENABLE_LIVE_WRITE_TESTS` | `BINANCE_API_KEY`, `BINANCE_SECRET_KEY` |
| OKX | `sdk/okx` public REST and market reads | `OKX_ENABLE_LIVE_WRITE_TESTS` | `OKX_API_KEY`, `OKX_API_SECRET`, `OKX_API_PASSPHRASE` |
| Bybit | `sdk/bybit` public REST and public WS reads | `BYBIT_ENABLE_LIVE_WRITE_TESTS` | `BYBIT_API_KEY`, `BYBIT_SECRET_KEY` |
| Bitget | `sdk/bitget` public REST and public WS reads | `BITGET_ENABLE_LIVE_WRITE_TESTS` | `BITGET_API_KEY`, `BITGET_SECRET_KEY`, `BITGET_PASSPHRASE` |
| Hyperliquid Spot/Perp | `sdk/hyperliquid` public reads | `HYPERLIQUID_ENABLE_LIVE_WRITE_TESTS` | `HYPERLIQUID_PRIVATE_KEY` plus package-specific vars |
| Lighter | `sdk/lighter` public reads | `LIGHTER_ENABLE_LIVE_WRITE_TESTS` | `LIGHTER_API_KEY`, `LIGHTER_ACCOUNT_INDEX`, signer vars |
| Backpack | `sdk/backpack` public REST reads | add `BACKPACK_ENABLE_LIVE_WRITE_TESTS` before mutating tests | package-specific credentials |
| Aster | `sdk/aster/spot`, `sdk/aster/perp` public reads | add `ASTER_ENABLE_LIVE_WRITE_TESTS` before mutating tests | package-specific credentials |
| Nado | public/private integration tests | move mutating tests behind `NADO_ENABLE_LIVE_WRITE_TESTS` | package-specific credentials |
| EdgeX | `sdk/edgex/perp` public reads | add `EDGEX_ENABLE_LIVE_WRITE_TESTS` before mutating tests | package-specific credentials |
| GRVT | `sdk/grvt` public reads | add `GRVT_ENABLE_LIVE_WRITE_TESTS` before mutating tests | package-specific credentials |
| StandX | `sdk/standx` public reads | add `STANDX_ENABLE_LIVE_WRITE_TESTS` before mutating tests | package-specific credentials |

## Adapter coverage rule

adapter capability tests 默认可使用 fake clients 和 shared contract suites。live adapter
tests 只是额外 evidence：

- public read paths 在不 mutate state 时可默认运行；
- private read paths 必须使用 `internal/testenv.RequireLiveCredentials`；
- live write paths 必须使用 `internal/testenv.RequireLiveWrite`；
- write flags 必须是 exchange-specific；
- unsupported live coverage 必须保持 documented gap，而不是 capability claim。
