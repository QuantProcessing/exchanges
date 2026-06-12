# TradingAccount Layer

`github.com/QuantProcessing/exchanges/account` is the high-level lifecycle
runtime. Use it when you want local trading state, order/fill fusion, streams,
balances, positions, and `OrderTracker` behavior on top of an execution client.

Example import path:

```go
import "github.com/QuantProcessing/exchanges/account"
```

The account package depends on lifecycle-critical `venue.ExecutionClient`
capabilities supplied by callers. It does not import concrete adapters or SDKs.

Allowed dependencies:

- `github.com/QuantProcessing/exchanges/model`
- `github.com/QuantProcessing/exchanges/venue`
- external Go modules

Forbidden dependencies:

- `github.com/QuantProcessing/exchanges/sdk/...`
- `github.com/QuantProcessing/exchanges/adapter/...`

Testing expectations:

- Account tests should focus on lifecycle behavior: snapshots, streams,
  normalized order reports, fill reports, positions, balances, and stream health.
- Startup reconciliation is the readiness gate: account snapshot, order reports,
  fill reports, position reports, then private stream connection.
- Fill and position reports are optional when unsupported behavior is explicit
  and tested.
- Market analytics such as funding, open interest, and historical data should
  not become account-layer requirements.
