# Adapter Layer

`github.com/QuantProcessing/exchanges/adapter/...` is the normalized
cross-exchange convenience layer. Use it when you want stable root models and
interfaces while still choosing a concrete exchange implementation.

Example import paths:

```go
import exchanges "github.com/QuantProcessing/exchanges"
import binance "github.com/QuantProcessing/exchanges/adapter/binance"
```

Adapter packages own symbol and instrument resolution, request validation,
exchange-native to unified model mapping, common error mapping, registration,
and stable REST/WebSocket convenience methods.

Allowed dependencies:

- the root `github.com/QuantProcessing/exchanges` package
- the adapter's own SDK subtree, such as
  `github.com/QuantProcessing/exchanges/sdk/binance/...`
- `github.com/QuantProcessing/exchanges/internal/...`
- external Go modules

Forbidden dependencies:

- another exchange's SDK package
- `github.com/QuantProcessing/exchanges/account`

Testing expectations:

- Adapter tests should verify normalized behavior and capability claims.
- `options.go` and `register.go` are required entry files for every adapter.
- Lifecycle readiness requires real private order stream support, not just REST
  order methods.
- New venue-specific API surface should start in `sdk/`; expose it here only
  when it fits a stable adapter abstraction or optional capability interface.
