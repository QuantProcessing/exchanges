# exchanges

**English** | [中文](./README_CN.md)

A unified Go SDK for interacting with multiple cryptocurrency exchanges.  

Provides both **low-level SDK clients** (REST + WebSocket) and **high-level adapters** implementing a common `Exchange` interface — a Go-native CCXT alternative.

## Features

- **Unified Interface** — One API to rule them all. Switch exchanges by changing one line.
- **Full Market Coverage** — Perpetual Futures, Spot, and Margin trading support.
- **Dual Transport** — REST for queries; WebSocket for real-time streaming and low-latency order placement.
- **Built-in Safety** — Exchange-specific request protection, rate-limit error mapping, order validation, and slippage protection.
- **Local State Management** — WebSocket-maintained orderbooks, position/order tracking, and balance sync.
- **Production-Ready** — Battle-tested in quantitative trading systems handling thousands of orders daily.

## Supported Exchanges

| Exchange    | Perp | Spot | Margin | Quote Currencies | Default |
|-------------|------|------|--------|------------------|---------|
| Binance     | ✅    | ✅    | ✅      | USDT, USDC       | USDT    |
| OKX         | ✅    | ✅    | —      | USDT, USDC       | USDT    |
| Aster       | ✅    | ✅    | —      | USDT, USDC       | USDT    |
| Nado        | ✅    | ✅    | —      | USDT             | USDT    |
| Lighter     | ✅    | ✅    | —      | USDC             | USDC    |
| Hyperliquid | ✅    | ✅    | —      | USDC             | USDC    |
| Bitget      | ✅    | ✅    | —      | USDT, USDC       | USDT    |
| StandX      | ✅    | —    | —      | DUSD             | DUSD    |
| GRVT        | ✅    | —    | —      | USDT             | USDT    |
| EdgeX       | ✅    | —    | —      | USDC             | USDC    |
| Decibel     | ✅    | —    | —      | USDC             | USDC    |

### Exchange Notes

- Bitget currently supports the classic private API surface only.
- Bitget defaults to `OrderModeREST`. Explicit `OrderModeWS` is opt-in and requires Bitget to enable classic WebSocket trade access for the API key.
- Decibel is perp-only in this repository. It uses authenticated REST/WebSocket reads plus Aptos-signed on-chain trading writes, with credentials `api_key + private_key + subaccount_addr`.

## Adding New Exchanges

Repository-specific guidance for new adapters lives in `docs/contributing/adding-exchange-adapters.md`.

Treat that guide as the source of truth for package layout, capability claims, private-stream readiness, and live test wiring. It is intentionally repo-owned guidance, not a reusable global skill.

## Installation

```bash
go get github.com/QuantProcessing/exchanges
```

---

## Design Philosophy

### 1. Adapter Pattern

Every exchange implements the same `Exchange` interface. Your strategy code never touches exchange-specific APIs:

```go
// This function works with ANY exchange — Binance, OKX, Hyperliquid, etc.
func getSpread(ctx context.Context, adp exchanges.Exchange, symbol string) (decimal.Decimal, error) {
    ob, err := adp.FetchOrderBook(ctx, symbol, 1)
    if err != nil {
        return decimal.Zero, err
    }
    return ob.Asks[0].Price.Sub(ob.Bids[0].Price), nil
}
```

### 2. Symbol Convention

All methods accept a **base currency symbol** (e.g. `"BTC"`, `"ETH"`). The adapter handles conversion to exchange-specific formats internally based on the configured quote currency:

| You Pass | Binance (USDT)    | Binance (USDC)   | OKX (USDT)       | Hyperliquid      |
|----------|-------------------|------------------|------------------|------------------|
| `"BTC"`  | `"BTCUSDT"`       | `"BTCUSDC"`      | `"BTC-USDT-SWAP"`| `"BTC"`          |

### 3. Two-Layer Architecture

```
┌─────────────────────────────────────────────────────┐
│  Your Strategy / Application                        │
├─────────────────────────────────────────────────────┤
│  Adapter Layer (exchanges.Exchange interface)        │  ← Unified API
│    binance.Adapter / okx.Adapter / nado.Adapter     │
├─────────────────────────────────────────────────────┤
│  SDK Layer (low-level REST + WebSocket clients)      │  ← Exchange-specific
│    binance/sdk/ / okx/sdk/ / nado/sdk/              │
└─────────────────────────────────────────────────────┘
```

- **Adapter Layer**: Implements `exchanges.Exchange`. Handles symbol mapping, order validation, slippage logic, and state management.
- **SDK Layer**: Thin REST/WebSocket clients that map 1:1 to exchange API endpoints. You can use these directly for maximum flexibility.

---

## Quick Start

### Basic Usage

```go
package main

import (
    "context"
    "fmt"

    exchanges "github.com/QuantProcessing/exchanges"
    "github.com/QuantProcessing/exchanges/binance"
    "github.com/shopspring/decimal"
)

func main() {
    ctx := context.Background()

    // Create a Binance perpetual adapter (defaults to USDT market)
    adp, err := binance.NewAdapter(ctx, binance.Options{
        APIKey:    "your-api-key",
        SecretKey: "your-secret-key",
        // QuoteCurrency: exchanges.QuoteCurrencyUSDC, // uncomment for USDC market
    })
    if err != nil {
        panic(err)
    }
    defer adp.Close()

    // Fetch ticker
    ticker, err := adp.FetchTicker(ctx, "BTC")
    if err != nil {
        panic(err)
    }
    fmt.Printf("BTC price: %s\n", ticker.LastPrice)

    // Fetch order book
    ob, err := adp.FetchOrderBook(ctx, "BTC", 5)
    if err != nil {
        panic(err)
    }
    fmt.Printf("Best bid: %s, Best ask: %s\n",
        ob.Bids[0].Price, ob.Asks[0].Price)

    // Place a limit order
    order, err := adp.PlaceOrder(ctx, &exchanges.OrderParams{
        Symbol:   "BTC",
        Side:     exchanges.OrderSideBuy,
        Type:     exchanges.OrderTypeLimit,
        Price:    ticker.Bid,
        Quantity: decimal.NewFromFloat(0.001),
    })
    if err != nil {
        panic(err)
    }
    fmt.Printf("Order placed: %s\n", order.OrderID)
}
```

### Config-Driven Bootstrap

```yaml
# exchanges.yaml
exchanges:
  - name: BINANCE
    market_type: perp
    options:
      api_key: ${BINANCE_API_KEY}
      secret_key: ${BINANCE_SECRET_KEY}
      quote_currency: USDT

  - name: OKX
    alias: okx-spot
    market_type: spot
    options:
      api_key: ${OKX_API_KEY}
      secret_key: ${OKX_SECRET_KEY}
      passphrase: ${OKX_PASSPHRASE}
      quote_currency: USDT
```

```go
package main

import (
    "context"
    "fmt"

    exconfig "github.com/QuantProcessing/exchanges/config"
    _ "github.com/QuantProcessing/exchanges/config/all"
)

func main() {
    ctx := context.Background()

    mgr, err := exconfig.LoadManager(ctx, "exchanges.yaml")
    if err != nil {
        panic(err)
    }
    defer mgr.CloseAll()

    adp, err := mgr.GetAdapter("BINANCE")
    if err != nil {
        panic(err)
    }

    ticker, err := adp.FetchTicker(ctx, "BTC")
    if err != nil {
        panic(err)
    }

    fmt.Printf("BTC price: %s\n", ticker.LastPrice)
}
```

- `config.LoadManager` supports both YAML and JSON files.
- `${ENV_VAR}` values are expanded with `os.ExpandEnv` before parsing.
- Blank-import `config/all` to auto-register every built-in exchange constructor.
- Default adapter aliases are the exchange name when configured once, and `NAME/market_type` when configured multiple times.

### Market Order with Slippage Protection

```go
// Market order with 0.5% slippage protection
// Internally converts to LIMIT IOC at (ask * 1.005) for buys
order, err := adp.PlaceOrder(ctx, &exchanges.OrderParams{
    Symbol:   "ETH",
    Side:     exchanges.OrderSideBuy,
    Type:     exchanges.OrderTypeMarket,
    Quantity: decimal.NewFromFloat(0.1),
    Slippage: decimal.NewFromFloat(0.005), // 0.5%
})
```

### Convenience Functions

```go
// One-liner market order
order, err := exchanges.PlaceMarketOrder(ctx, adp, "BTC", exchanges.OrderSideBuy, qty)

// Market order with optional reference price
order, err := exchanges.PlaceMarketOrder(ctx, adp, "BTC", exchanges.OrderSideBuy, qty, refPrice)

// One-liner limit order
order, err := exchanges.PlaceLimitOrder(ctx, adp, "BTC", exchanges.OrderSideBuy, price, qty)

// Market order with slippage
order, err := exchanges.PlaceMarketOrderWithSlippage(ctx, adp, "BTC", exchanges.OrderSideBuy, qty, slippage)

// Slippage-protected market order with optional reference price
order, err := exchanges.PlaceMarketOrderWithSlippage(ctx, adp, "BTC", exchanges.OrderSideBuy, qty, slippage, refPrice)
```

Passing `refPrice` is optional. When provided, adapters can skip an extra ticker lookup in market-order/slippage flows that need a local reference price.

### WebSocket Streaming

```go
// Real-time order book (locally maintained)
err := adp.WatchOrderBook(ctx, "BTC", 20, func(ob *exchanges.OrderBook) {
    fmt.Printf("BTC bid: %s ask: %s\n", ob.Bids[0].Price, ob.Asks[0].Price)
})

// Pull latest snapshot anytime (zero-latency, no API call)
ob := adp.GetLocalOrderBook("BTC", 5)

// Use depth <= 0 to request full locally-maintained depth
full := adp.GetLocalOrderBook("BTC", 0)

// Real-time ticker
adp.WatchTicker(ctx, "BTC", func(t *exchanges.Ticker) {
    fmt.Printf("Price: %s\n", t.LastPrice)
})

// Real-time order overview updates
adp.WatchOrders(ctx, func(o *exchanges.Order) {
    fmt.Printf("Order %s: %s order=%s filled=%s\n",
        o.OrderID, o.Status, o.OrderPrice, o.FilledQuantity)
})

// Real-time private fills (one callback per fill or exchange-native fill batch)
adp.WatchFills(ctx, func(f *exchanges.Fill) {
    fmt.Printf("Fill %s: %s %s @ %s\n",
        f.TradeID, f.Side, f.Quantity, f.Price)
})

// Real-time position updates
adp.WatchPositions(ctx, func(p *exchanges.Position) {
    fmt.Printf("%s: %s %s @ %s\n", p.Symbol, p.Side, p.Quantity, p.EntryPrice)
})
```

`WatchOrders` is for order overview state. Use it for order price, quantity, filled quantity, status, IDs, and timestamp. `WatchFills` is for execution detail. Use it for execution price, execution quantity, fee, fee asset, and maker/taker. A single order may produce zero, one, or many fill callbacks, and some exchanges may batch more than one native execution into one fill update.

`WatchFills` is now supported by every private-trading adapter in this repository except Binance margin. If an adapter cannot expose private executions natively, it returns `ErrNotSupported` instead of synthesizing fills from another stream.

### Using PerpExchange Extensions

```go
// Type assert for perp-specific features
if perp, ok := adp.(exchanges.PerpExchange); ok {
    // Set leverage
    perp.SetLeverage(ctx, "BTC", 10)

    // Get positions
    positions, _ := perp.FetchPositions(ctx)
    for _, p := range positions {
        fmt.Printf("%s: %s %s\n", p.Symbol, p.Side, p.Quantity)
    }

    // Get funding rate
    fr, _ := perp.FetchFundingRate(ctx, "BTC")
    fmt.Printf("Funding rate: %s\n", fr.FundingRate)
}
```

### LocalState (Unified State Management)

```go
// LocalState wraps any Exchange adapter — auto-subscribes to WS streams,
// maintains orders/positions/balance, and provides fan-out event subscriptions.
state := exchanges.NewLocalState(adp, nil)
err := state.Start(ctx) // REST snapshot + auto WatchOrders/WatchPositions + periodic refresh

// Read state anytime (thread-safe, zero-latency)
pos, ok := state.GetPosition("BTC")
order, ok := state.GetOrder("order-123")
balance := state.GetBalance()

// Fan-out event subscriptions (multiple consumers supported)
sub := state.SubscribeOrders()
defer sub.Unsubscribe()
go func() {
    for order := range sub.C {
        fmt.Printf("Order update: %s %s\n", order.OrderID, order.Status)
    }
}()

// Place order with integrated tracking — no need for separate WatchOrders
result, err := state.PlaceOrder(ctx, &exchanges.OrderParams{
    Symbol:   "BTC",
    Side:     exchanges.OrderSideBuy,
    Type:     exchanges.OrderTypeMarket,
    Quantity: decimal.NewFromFloat(0.001),
})
defer result.Done()
filled, err := result.WaitTerminal(30 * time.Second) // blocks until FILLED/CANCELLED/REJECTED
```

### Switching Exchanges

```go
// Binance — USDT market (default)
adp, _ := binance.NewAdapter(ctx, binance.Options{
    APIKey: os.Getenv("BINANCE_API_KEY"), SecretKey: os.Getenv("BINANCE_SECRET_KEY"),
})

// Binance — USDC market
adpUSDC, _ := binance.NewAdapter(ctx, binance.Options{
    APIKey: os.Getenv("BINANCE_API_KEY"), SecretKey: os.Getenv("BINANCE_SECRET_KEY"),
    QuoteCurrency: exchanges.QuoteCurrencyUSDC,
})

// OKX — same interface, different constructor
adp, _ := okx.NewAdapter(ctx, okx.Options{
    APIKey: os.Getenv("OKX_API_KEY"), SecretKey: os.Getenv("OKX_SECRET_KEY"),
    Passphrase: os.Getenv("OKX_PASSPHRASE"),
})

// Hyperliquid — wallet-based auth (USDC only)
adp, _ := hyperliquid.NewAdapter(ctx, hyperliquid.Options{
    PrivateKey: os.Getenv("HYPERLIQUID_PRIVATE_KEY"), AccountAddr: os.Getenv("HYPERLIQUID_ACCOUNT_ADDR"),
})

// All adapters expose the exact same Exchange interface
ticker, _ := adp.FetchTicker(ctx, "BTC")
```

### Quote Currency

Each adapter supports a `QuoteCurrency` option that determines which quote currency market to connect to. If omitted, the exchange-specific default is used. Most CEX adapters default to USDT, while DEX adapters vary by venue.

```go
// Available quote currencies
exchanges.QuoteCurrencyUSDT // "USDT"
exchanges.QuoteCurrencyUSDC // "USDC"
exchanges.QuoteCurrencyDUSD // "DUSD" (StandX only)
```

Passing an unsupported quote currency returns an error at construction time:

```go
// This will fail: Hyperliquid only supports USDC
_, err := hyperliquid.NewAdapter(ctx, hyperliquid.Options{
    QuoteCurrency: exchanges.QuoteCurrencyUSDT, // error!
})
// err: "hyperliquid: unsupported quote currency "USDT", supported: [USDC]"
```

---

## API Reference

### Exchange Interface (Core)

Every adapter implements these methods:

| Category | Method | Description |
|----------|--------|-------------|
| **Identity** | `GetExchange()` | Returns exchange name (e.g. `"BINANCE"`) |
| | `GetMarketType()` | Returns `"perp"` or `"spot"` |
| | `Close()` | Closes all connections |
| **Symbol** | `FormatSymbol(symbol)` | Converts `"BTC"` → exchange format |
| | `ExtractSymbol(symbol)` | Converts exchange format → `"BTC"` |
| | `ListSymbols()` | Returns all available symbols |
| **Market Data** | `FetchTicker(ctx, symbol)` | Latest price, bid/ask, 24h volume |
| | `FetchOrderBook(ctx, symbol, limit)` | Order book snapshot (REST) |
| | `FetchTrades(ctx, symbol, limit)` | Recent trades |
| | `FetchKlines(ctx, symbol, interval, opts)` | Candlestick/OHLCV data |
| **Trading** | `PlaceOrder(ctx, params)` | Place order (market/limit/post-only) |
| | `CancelOrder(ctx, orderID, symbol)` | Cancel a single order |
| | `CancelAllOrders(ctx, symbol)` | Cancel all open orders for a symbol |
| | `FetchOrderByID(ctx, orderID, symbol)` | Get a single order by ID, including terminal orders when supported |
| | `FetchOrders(ctx, symbol)` | List all visible orders for a symbol |
| | `FetchOpenOrders(ctx, symbol)` | List all open orders |
| **Account** | `FetchAccount(ctx)` | Full account: balance + positions + orders |
| | `FetchBalance(ctx)` | Available balance only |
| | `FetchSymbolDetails(ctx, symbol)` | Precision & min quantity rules |
| | `FetchFeeRate(ctx, symbol)` | Maker/taker fee rates |
| **Orderbook** | `WatchOrderBook(ctx, symbol, depth, cb)` | Subscribe to WS orderbook (blocks until ready) |
| | `GetLocalOrderBook(symbol, depth)` | Read local WS-maintained orderbook |
| | `StopWatchOrderBook(ctx, symbol)` | Unsubscribe |
| **Streaming** | `WatchOrders(ctx, cb)` | Real-time order lifecycle updates |
| | `WatchFills(ctx, cb)` | Real-time private fills / executions |
| | `WatchPositions(ctx, cb)` | Real-time position updates |
| | `WatchTicker(ctx, symbol, cb)` | Real-time ticker |
| | `WatchTrades(ctx, symbol, cb)` | Real-time trades |
| | `WatchKlines(ctx, symbol, interval, cb)` | Real-time klines |

`FetchOrderByID`, `FetchOrders`, and `FetchOpenOrders` are intentionally separate: single-order lookup should not be implemented by scanning open orders, and `FetchOrders` is broader than `FetchOpenOrders`.

### PerpExchange Interface (extends Exchange)

| Method | Description |
|--------|-------------|
| `FetchPositions(ctx)` | Get all open positions |
| `SetLeverage(ctx, symbol, leverage)` | Set leverage for a symbol |
| `FetchFundingRate(ctx, symbol)` | Current funding rate |
| `FetchAllFundingRates(ctx)` | All funding rates |
| `ModifyOrder(ctx, orderID, symbol, params)` | Modify an open order (price/qty) |

### SpotExchange Interface (extends Exchange)

| Method | Description |
|--------|-------------|
| `FetchSpotBalances(ctx)` | Per-asset balances (free/locked) |
| `TransferAsset(ctx, params)` | Transfer between spot/futures accounts |

### OrderParams

```go
type OrderParams struct {
    Symbol      string          // Base symbol: "BTC", "ETH"
    Side        OrderSide       // OrderSideBuy or OrderSideSell
    Type        OrderType       // OrderTypeMarket, OrderTypeLimit, OrderTypePostOnly
    Quantity    decimal.Decimal // Order quantity
    Price       decimal.Decimal // Required for LIMIT orders
    TimeInForce TimeInForce     // GTC (default), IOC, FOK
    ReduceOnly  bool            // Reduce-only order
    Slippage    decimal.Decimal // If > 0, MARKET → LIMIT IOC with slippage
    ClientID    string          // Client-defined order ID
}
```

### Backpack ClientID Note

Backpack requires a numeric `clientId` in the valid `uint32` range. If you want to set `OrderParams.ClientID` yourself, do not pass UUIDs, timestamps, or any value larger than `4294967295`.

Use the package helper instead:

```go
import "github.com/QuantProcessing/exchanges/backpack"

params := &exchanges.OrderParams{
    Symbol:   "BTC",
    Side:     exchanges.OrderSideBuy,
    Type:     exchanges.OrderTypeLimit,
    Quantity: qty,
    Price:    price,
    ClientID: backpack.GenerateClientID(),
}
```

If `ClientID` is left empty, the Backpack adapter generates a safe one automatically.

### Error Handling

```go
order, err := adp.PlaceOrder(ctx, params)
if err != nil {
    // Structured error matching
    if errors.Is(err, exchanges.ErrInsufficientBalance) {
        // Handle insufficient balance
    }
    if errors.Is(err, exchanges.ErrMinQuantity) {
        // Handle below minimum quantity
    }
    if errors.Is(err, exchanges.ErrRateLimited) {
        // Handle rate limit according to your own retry/backoff policy
    }

    // Access exchange-specific details
    var exErr *exchanges.ExchangeError
    if errors.As(err, &exErr) {
        fmt.Printf("[%s] Code: %s, Message: %s\n", exErr.Exchange, exErr.Code, exErr.Message)
    }
}
```

Available sentinel errors: `ErrInsufficientBalance`, `ErrRateLimited`, `ErrInvalidPrecision`, `ErrOrderNotFound`, `ErrSymbolNotFound`, `ErrMinNotional`, `ErrMinQuantity`, `ErrAuthFailed`, `ErrNetworkTimeout`, `ErrNotSupported`.

### Rate Limit Error Handling

When an exchange returns a rate-limit error, the SDK wraps it as a structured `ExchangeError` with `ErrRateLimited` as the cause. The error flows through the entire call chain:

```
Your Code (caller)
  → adapter.PlaceOrder()     // transparent pass-through (return nil, err)
    → client.Post()          // returns exchanges.NewExchangeError(..., ErrRateLimited)
```

**Basic detection** — use `errors.Is()` to check for rate limiting:

```go
order, err := adp.PlaceOrder(ctx, params)
if errors.Is(err, exchanges.ErrRateLimited) {
    log.Warn("rate limited, backing off...")
    time.Sleep(5 * time.Second)
}
```

**Extract exchange-specific details** — use `errors.As()` for the full error context:

```go
var exErr *exchanges.ExchangeError
if errors.As(err, &exErr) && errors.Is(err, exchanges.ErrRateLimited) {
    fmt.Printf("Exchange: %s\n", exErr.Exchange) // "BINANCE", "GRVT", "LIGHTER", etc.
    fmt.Printf("Code:     %s\n", exErr.Code)     // "-1003", "1006", "429"
    fmt.Printf("Message:  %s\n", exErr.Message)  // Original error message
}
```

**Recommended retry pattern** — exponential backoff:

```go
func placeOrderWithRetry(ctx context.Context, adp exchanges.Exchange, params *exchanges.OrderParams) (*exchanges.Order, error) {
    maxRetries := 3
    for i := 0; i < maxRetries; i++ {
        order, err := adp.PlaceOrder(ctx, params)
        if err == nil {
            return order, nil
        }
        if !errors.Is(err, exchanges.ErrRateLimited) {
            return nil, err // Non-rate-limit error, fail immediately
        }
        backoff := time.Duration(1<<uint(i)) * time.Second // 1s, 2s, 4s
        log.Warnf("rate limited (attempt %d/%d), retrying in %v", i+1, maxRetries, backoff)
        select {
        case <-time.After(backoff):
        case <-ctx.Done():
            return nil, ctx.Err()
        }
    }
    return nil, fmt.Errorf("rate limited after %d retries", maxRetries)
}
```

> **Design note**: The library deliberately does not implement automatic retry or backoff. Rate-limit handling strategy (fixed delay, exponential backoff, circuit breaker, etc.) is a business-level decision that callers should own.

---

## Built-in Safety Features

### Rate Limiting

Rate-limit detection is implemented at the SDK layer for every supported exchange:

| Exchange | Detection Signal | Error Code | Details |
|----------|-----------------|------------|---------- |
| **Binance** | HTTP 429/418, code -1003/-1015, header `X-Mbx-Used-Weight` | `-1003`, `-1015` | Weight-based + order count tracking |
| **Aster** | Same as Binance (Binance-family fork) | `-1003`, `-1015` | Same X-Mbx-* header support |
| **OKX** | HTTP 429, code `50011`/`50061` | `50011`, `50061` | Per-endpoint rate limits |
| **Hyperliquid** | HTTP 429, message-based detection | `429` | Message content matching |
| **EdgeX** | HTTP 429, code/message-based detection | `429` | Custom error code/message |
| **GRVT** | HTTP 429, error code `1006` | `1006` | Per-instrument tracking |
| **Lighter** | HTTP 429 | `429` | Weight-based (60 req/min standard) |
| **Nado** | HTTP 429 | `429` | 1200 req/min per IP |
| **StandX** | HTTP 429 | `429` | Retry-After header support |

All rate-limit errors are wrapped as `ExchangeError` with `ErrRateLimited` as the unwrappable cause. Use `errors.Is(err, exchanges.ErrRateLimited)` for detection — see [Rate Limit Error Handling](#rate-limit-error-handling) for detailed usage.

### IP Ban Detection & Recovery

If an exchange returns explicit ban or throttle errors (for example HTTP 418/429), the SDK surfaces them as structured exchange errors so callers can decide whether to retry, back off, or pause.

### Order Validation

Before sending orders, adapters automatically:
- Round price to symbol's price precision
- Truncate quantity to symbol's quantity precision
- Validate minimum quantity and notional constraints

---

## Logger

All adapters accept an optional `Logger` for structured logging:

```go
// Compatible with *zap.SugaredLogger
logger := zap.NewProduction().Sugar()
adp, _ := binance.NewAdapter(ctx, binance.Options{
    APIKey: "...", SecretKey: "...",
    Logger: logger,
})
```

If no logger is provided, `NopLogger` is used. The interface:

```go
type Logger interface {
    Debugw(msg string, keysAndValues ...any)
    Infow(msg string, keysAndValues ...any)
    Warnw(msg string, keysAndValues ...any)
    Errorw(msg string, keysAndValues ...any)
}
```

---

## Project Structure

```
exchanges/                  Root package — interfaces, models, errors, utilities
├── exchange.go             Core Exchange / PerpExchange / SpotExchange interfaces
├── models.go               Unified data types (Order, Position, Ticker, etc.)
├── errors.go               Sentinel errors + ExchangeError type
├── base_adapter.go         Shared adapter logic (orderbook, validation, common helpers)
├── local_state.go          LocalOrderBook interface + unified LocalState manager
├── event_bus.go            Generic EventBus[T] for fan-out pub/sub
├── log.go                  Logger interface + NopLogger
├── testsuite/              Adapter compliance test suite
├── binance/                Binance adapter + SDK
│   ├── options.go          Options{APIKey, SecretKey, QuoteCurrency, Logger}
│   ├── perp_adapter.go     Perp adapter (Exchange + PerpExchange)
│   ├── spot_adapter.go     Spot adapter (Exchange + SpotExchange)
│   └── sdk/                Low-level REST & WebSocket clients
├── okx/                    OKX (same structure)
├── aster/                  Aster
├── nado/                   Nado
├── lighter/                Lighter
├── hyperliquid/            Hyperliquid
├── standx/                 StandX
├── grvt/                   GRVT (build tag: grvt)
└── edgex/                  EdgeX (build tag: edgex)
```

## Testing

The repository uses a layered verification model. Plain `go test ./...` is not the canonical gate because some packages include live exchange coverage that depends on credentials or longer-running WebSocket sessions.

Copy the example environment file and fill in your credentials when you need live/private verification:
```bash
cp .env.example .env
```

Run the default quick gate:
```bash
go test -short ./...
```

Run a focused short verification for one exchange:
```bash
scripts/verify_exchange.sh backpack
scripts/verify_exchange.sh okx
scripts/verify_exchange.sh hyperliquid
```

Run the full regression suite:
```bash
GOCACHE=/tmp/exchanges-gocache bash scripts/verify_full.sh
```

`verify_full.sh` loads `.env` from the repository root, preserves already-exported shell variables, and manages `RUN_FULL=1` internally. It also accepts these legacy aliases:

- `EDGEX_PRIVATE_KEY -> EDGEX_STARK_PRIVATE_KEY`
- `NADO_SUB_ACCOUNT_NAME -> NADO_SUBACCOUNT_NAME`
- `OKX_SECRET_KEY -> OKX_API_SECRET`
- `OKX_PASSPHRASE -> OKX_API_PASSPHRASE`

Run the soak suite for longer-lived subscription checks:
```bash
GOCACHE=/tmp/exchanges-gocache RUN_SOAK=1 bash scripts/verify_soak.sh
```

The current soak suite runs 3-minute stream checks for the designated packages. `RUN_FULL` and `RUN_SOAK` are script-managed toggles for the dedicated verification entrypoints above; they are not the default repository gate.

## License

MIT
