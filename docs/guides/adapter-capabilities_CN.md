# Adapter 能力

Adapter capabilities 是产品承诺。它告诉应用代码哪些 venue 行为已经实现、测试过，
可以安全依赖。

能力真相来自两处：

1. runtime 中的 `adapter.Capabilities()`；
2. [Adapter 能力矩阵](../parity/adapter-capability-matrix_CN.md)。

market-data adapter 用法见
[01_fetch_ticker_with_adapter.go](../../examples/01_fetch_ticker_with_adapter.go)。

## Capability Families

`venue.DeclaredCapabilities` 按 instruments、market data、execution 和 account 分组。

Market data capabilities：

- snapshots、ticker、order book；
- ticker stream、order-book stream；
- trade ticks、quote ticks、bars；
- funding-rate snapshots、funding-rate stream；
- general stream support。

Execution capabilities：

- submit、cancel、modify、query；
- order reports、fill reports、position reports；
- private stream、resubscribe、mass status、order lists。

Account capabilities：

- account snapshot。

## Runtime Check

```go
caps := adp.Capabilities()
if !caps.MarketData.OrderBookStream {
    return fmt.Errorf("%s does not support order-book streaming", adp.Venue())
}
if !caps.MarketData.FundingRates {
    return fmt.Errorf("%s does not expose normalized funding-rate snapshots", adp.Venue())
}
if !caps.Execution.Submit || !caps.Execution.Cancel {
    return fmt.Errorf("%s cannot run this strategy safely", adp.Venue())
}
```

对 optional interfaces，同时检查 capability 与 type assertion：

```go
if caps.MarketData.FundingRates {
    provider, ok := adp.Data().(venue.FundingRateProvider)
    if !ok {
        return fmt.Errorf("funding-rate snapshot claimed but interface missing")
    }
    funding, err := provider.FetchFundingRate(ctx, instrumentID)
    _ = funding
    return err
}

if caps.Execution.Query {
    querier, ok := adp.Execution().(venue.OrderQuerier)
    if !ok {
        return fmt.Errorf("query claimed but interface missing")
    }
    _, err := querier.QueryOrder(ctx, query)
    return err
}
```

## Funding-Rate Providers

当前声明 `caps.MarketData.FundingRates` 并实现 `venue.FundingRateProvider` 的
adapter：

- 当前 venue snapshot：Binance Perp、Aster Perp、OKX Swap、Bybit Linear、
  Bitget Perp、Hyperliquid Perp、Lighter、EdgeX、GRVT、StandX、Backpack、Nado。

标准化 payload 始终是 `model.FundingRate`，并且只表达当前 funding 本体：rate、interval、
next funding time、timestamp 和 init time。它不承载 mark price 或 index price。需要参考价的策略必须把 funding
snapshot 和普通 market-data capability 组合起来，例如 ticker、mark-price 或 index-price feed。
funding snapshot claim 不代表 `FundingRateStream`；stream support 有单独 capability flag。

live arbitrage 形态见
[07_monitor_funding_rate_arbitrage.go](../../examples/07_monitor_funding_rate_arbitrage.go)。

## Unsupported Behavior

Unsupported behavior 必须返回 `model.ErrNotSupported` 或 wrapped equivalent：

```go
if errors.Is(err, model.ErrNotSupported) {
    // 降级 workflow，或给 operator 一个清晰的 fail-fast 错误
}
```

如果策略正确性依赖 modify、query、fill reports、position reports、mass status 或
order-list support，不要把这些缺失能力当成 optional。

## Policy And Matrix

- [Adapter 能力策略](./adapter-capability-policy_CN.md)
- [Adapter 能力矩阵](../parity/adapter-capability-matrix_CN.md)
- [Adapter Live Test Policy](../parity/adapter-live-test-policy_CN.md)

## Verification

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./venue ./testsuite ./adapter/... ./config/all -run 'Adapter|Capability|Contract|PrivateStream|Resubscribe' -v
```
