# Adapter Capabilities

Adapter capabilities are product promises. They tell application code which
venue behaviors are implemented, tested, and safe to depend on.

Capability truth comes from two places:

1. `adapter.Capabilities()` at runtime;
2. [Adapter Capability Matrix](../parity/adapter-capability-matrix.md) for
   repository documentation and release review.

Market-data adapter usage is shown in
[01_fetch_ticker_with_adapter.go](../../examples/01_fetch_ticker_with_adapter.go).

## Capability Families

`venue.DeclaredCapabilities` is grouped by instruments, market data, execution,
and account support.

Market data capabilities:

- snapshots;
- ticker;
- order book;
- ticker stream;
- order-book stream;
- trade ticks;
- quote ticks;
- bars;
- funding-rate snapshots;
- funding-rate stream;
- general stream support.

Execution capabilities:

- submit;
- cancel;
- modify;
- query;
- order reports;
- fill reports;
- position reports;
- private stream;
- resubscribe;
- mass status;
- order lists.

Account capabilities:

- account snapshot.

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

For optional interfaces, check capabilities and type assertions together:

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
    report, err := querier.QueryOrder(ctx, query)
    _ = report
    return err
}
```

## Unsupported Behavior

Unsupported behavior must return `model.ErrNotSupported` or a wrapped
equivalent:

```go
if errors.Is(err, model.ErrNotSupported) {
    // downgrade the workflow or fail fast with a clear operator message
}
```

Do not treat missing modify, query, fill reports, position reports, mass
status, or order-list support as optional if your strategy requires them for
correctness.

## Policy And Matrix

- [Adapter Capability Policy](./adapter-capability-policy.md)
- [Adapter Capability Matrix](../parity/adapter-capability-matrix.md)
- [Adapter Live Test Policy](../parity/adapter-live-test-policy.md)

## Verification

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./venue ./testsuite ./adapter/... ./config/all -run 'Adapter|Capability|Contract|PrivateStream|Resubscribe' -v
```
