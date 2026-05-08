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

## Matrix

`WS writes` means explicit WebSocket order/cancel write methods. `Private
streams` are account-side streams. `Public streams` lists WebSocket market data
streams beyond ordinary REST fetches. `Order queries` lists supported optional
query surfaces.

| Exchange | Market | WS writes | Private streams | Public streams | Order queries | Modify | TradingAccount |
|----------|--------|-----------|-----------------|----------------|---------------|--------|----------------|
| Aster | perp | - | orders, fills, positions | book, ticker, trades, klines | open | yes | yes |
| Aster | spot | - | orders, fills | book, ticker | open | - | yes |
| Backpack | perp | - | orders, fills, positions | book | open | - | yes |
| Backpack | spot | - | orders, fills | book | open | - | yes |
| Binance | perp | place, cancel | orders, fills, positions | book, ticker, trades, klines | open | yes | yes |
| Binance | spot | place, cancel | orders, fills | book, ticker, trades, klines | open | yes | yes |
| Bitget | perp | place, cancel | orders, fills, positions | book | open, history | yes | yes |
| Bitget | spot | place, cancel | orders, fills | book | open, history | - | yes |
| Bybit | perp | place, cancel | orders, fills, positions | book | open, history | yes | yes |
| Bybit | spot | place, cancel | orders, fills | book | open, history | - | yes |
| EdgeX | perp | - | orders, fills, positions | book, ticker, trades, klines | open | - | yes |
| GRVT | perp | place, cancel | orders, fills, positions | book, ticker, trades, klines | open | - | yes |
| Hyperliquid | perp | place, cancel | orders, fills, positions | book, ticker, trades | open | yes | yes |
| Hyperliquid | spot | place, cancel | orders, fills | book, ticker, trades | - | yes | yes |
| Lighter | perp | place, cancel | orders, fills, positions | book, ticker, trades | open | yes | yes |
| Lighter | spot | place, cancel | orders, fills | book, ticker, trades | open | yes | yes |
| Nado | perp | place, cancel | orders, fills, positions | book, ticker, trades, klines | open | - | yes |
| Nado | spot | place, cancel | orders, fills, positions | book, ticker, trades | open | - | yes |
| OKX | perp | place, cancel | orders, fills, positions | book, ticker | open | yes | yes |
| OKX | spot | place, cancel | orders, fills | book, ticker | open | yes | yes |
| StandX | perp | place, cancel | orders, fills, positions | book, ticker, trades | open | - | yes |

These are static support claims, not health checks. A capability can still fail
at runtime because of missing credentials, exchange-side permissions, network
conditions, or exchange account configuration.
