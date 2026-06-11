# TradingAccount Layer

`github.com/QuantProcessing/exchanges/account` is the high-level lifecycle
runtime. Use it when you want local trading state, order/fill fusion, streams,
balances, positions, and `OrderFlow` behavior on top of an adapter.

Example import path:

```go
import "github.com/QuantProcessing/exchanges/account"
```

The account package depends on root interfaces and lifecycle-critical adapter
capabilities supplied by callers. It does not import concrete adapters or SDKs.

Allowed dependencies:

- the root `github.com/QuantProcessing/exchanges` package
- `github.com/QuantProcessing/exchanges/internal/...`
- external Go modules

Forbidden dependencies:

- `github.com/QuantProcessing/exchanges/sdk/...`
- `github.com/QuantProcessing/exchanges/adapter/...`

Testing expectations:

- Account tests should focus on lifecycle behavior: snapshots, streams, local
  order state, balance/position updates, fills, and order flow.
- `WatchOrders` is the readiness gate for lifecycle-capable adapters.
- `WatchFills` is optional when unsupported behavior is explicit and tested.
- Market analytics such as funding, open interest, and historical data should
  not become account-layer requirements.
