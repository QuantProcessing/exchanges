# SDK Layer

`github.com/QuantProcessing/exchanges/sdk/...` is the venue-native protocol
layer. Use it when you want direct access to an exchange API instead of the
normalized adapter abstraction.

Example import paths:

```go
import binanceperp "github.com/QuantProcessing/exchanges/sdk/binance/perp"
import okxsdk "github.com/QuantProcessing/exchanges/sdk/okx"
```

SDK packages own exchange-native REST clients, WebSocket clients, request and
response structs, signing, endpoint names, and protocol-specific options.

Allowed dependencies:

- `github.com/QuantProcessing/exchanges/sdk` for SDK-wide primitives such as
  `sdk.RequestOpts`
- `github.com/QuantProcessing/exchanges/internal/...` for private shared
  helpers
- external Go modules

Forbidden dependencies:

- the root `github.com/QuantProcessing/exchanges` package
- `github.com/QuantProcessing/exchanges/adapter/...`
- `github.com/QuantProcessing/exchanges/account`
- another exchange's SDK package

Testing expectations:

- SDK tests stay method-local and close to the official API shape.
- Public read tests may hit live official endpoints.
- Write tests must be opt-in behind exchange-specific flags and credentials.
- Shared protocol helpers belong in `internal/...`, not in another exchange's
  SDK subtree.
