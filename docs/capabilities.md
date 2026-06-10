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
