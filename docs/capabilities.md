# Adapter Capabilities

Capability discovery lets SDK users ask what an adapter claims to support before
calling optional methods and handling `ErrNotSupported`.

```go
caps := exchanges.GetCapabilities(adp)
if !caps.WatchFills {
	// Fall back to order-only lifecycle handling.
}
```

For config-driven flows, capability claims can also be checked before adapter
construction:

```go
caps, ok := exchanges.LookupCapabilities("BINANCE", exchanges.MarketTypePerp)
```

`Exchange` itself remains unchanged. Adapters expose capabilities through the
optional `CapabilityProvider` interface, and `BaseAdapter` loads registered
static claims during construction.

These are static support claims, not health checks. A capability can still fail
at runtime because of missing credentials, exchange-side permissions, network
conditions, or exchange account configuration.

## Platform Runtime Claims

The `platform/` runtime uses the same declared capabilities, but interprets
them as lifecycle contracts:

- data clients must expose `venue.DataClient` lifecycle methods before a node
  can manage them;
- execution clients must expose account snapshot and report generation before
  startup reconciliation can be certified;
- `Reconciliation.Startup` requires bounded order, fill, account, and relevant
  position report sources, not only open-order queries;
- transport details such as REST versus WebSocket order entry stay inside the
  venue client and are not platform API capabilities.

The top-level `cache/` package stores normalized instruments, account states,
orders, fills, and positions for platform engines and account lifecycle logic.
