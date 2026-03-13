# exchanges

**English** | [中文](./README_CN.md)

A unified Go SDK for interacting with multiple cryptocurrency exchanges.  

Provides both **low-level SDK clients** (REST + WebSocket) and **high-level adapters** implementing a common `Exchange` interface — a Go-native CCXT alternative.

## Features

- **Unified Interface** — One API to rule them all. Switch exchanges by changing one line.
- **Full Market Coverage** — Perpetual Futures, Spot, and Margin trading support.
- **Dual Transport** — REST for queries; WebSocket for real-time streaming and low-latency order placement.
- **Built-in Safety** — Declarative rate limiting, automatic IP ban detection/recovery, order validation, and slippage protection.
- **Local State Management** — WebSocket-maintained orderbooks, position/order tracking, and balance sync.
- **Production-Ready** — Battle-tested in quantitative trading systems handling thousands of orders daily.

## Supported Exchanges

| Exchange    | Perp | Spot | Margin | Quote Currencies | Default |
|-------------|------|------|--------|------------------|---------|
| Binance     | ✅    | ✅    | ✅      | USDT, USDC       | USDT    |
| OKX         | ✅    | ✅    | —      | USDT, USDC       | USDT    |
| Aster       | ✅    | ✅    | —      | USDT, USDC       | USDC    |
| Nado        | ✅    | ✅    | —      | USDT             | USDT    |
| Lighter     | ✅    | ✅    | —      | USDC             | USDC    |
| Hyperliquid | ✅    | ✅    | —      | USDC             | USDC    |
| StandX      | ✅    | —    | —      | DUSD             | DUSD    |
| GRVT        | ✅    | —    | —      | USDT             | USDT    |
| EdgeX       | ✅    | ✅    | —      | USDC             | USDC    |

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

- **Adapter Layer**: Implements `exchanges.Exchange`. Handles symbol mapping, order validation, slippage logic, rate limiting, and state management.
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

// One-liner limit order
order, err := exchanges.PlaceLimitOrder(ctx, adp, "BTC", exchanges.OrderSideBuy, price, qty)

// Market order with slippage
order, err := exchanges.PlaceMarketOrderWithSlippage(ctx, adp, "BTC", exchanges.OrderSideBuy, qty, slippage)
```

### WebSocket Streaming

```go
// Real-time order book (locally maintained)
err := adp.WatchOrderBook(ctx, "BTC", func(ob *exchanges.OrderBook) {
    fmt.Printf("BTC bid: %s ask: %s\n", ob.Bids[0].Price, ob.Asks[0].Price)
})

// Pull latest snapshot anytime (zero-latency, no API call)
ob := adp.GetLocalOrderBook("BTC", 5)

// Real-time ticker
adp.WatchTicker(ctx, "BTC", func(t *exchanges.Ticker) {
    fmt.Printf("Price: %s\n", t.LastPrice)
})

// Real-time order updates (fills, cancellations)
adp.WatchOrders(ctx, func(o *exchanges.Order) {
    fmt.Printf("Order %s: %s\n", o.OrderID, o.Status)
})

// Real-time position updates
adp.WatchPositions(ctx, func(p *exchanges.Position) {
    fmt.Printf("%s: %s %s @ %s\n", p.Symbol, p.Side, p.Quantity, p.EntryPrice)
})
```

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

### AccountManager (Auto-Sync State)

```go
// AccountManager subscribes to WS updates and keeps local state in sync
mgr, err := exchanges.NewAccountManager(adp.(exchanges.PerpExchange), nil)
if err != nil {
    panic(err)
}
err = mgr.Start(ctx, 1*time.Minute) // REST refresh every minute as backup

// Read state anytime (thread-safe, zero-latency)
pos, ok := mgr.GetPosition("BTC")
order, ok := mgr.GetOrder("order-123")
balance := mgr.GetLocalBalance()

// Stream updates via channels
go func() {
    for order := range mgr.GetOrderStream() {
        fmt.Printf("Order update: %s %s\n", order.OrderID, order.Status)
    }
}()
```

### Switching Exchanges

```go
// Binance — USDT market (default)
adp, _ := binance.NewAdapter(ctx, binance.Options{
    APIKey: os.Getenv("BINANCE_API_KEY"), SecretKey: os.Getenv("BINANCE_SECRET"),
})

// Binance — USDC market
adpUSDC, _ := binance.NewAdapter(ctx, binance.Options{
    APIKey: os.Getenv("BINANCE_API_KEY"), SecretKey: os.Getenv("BINANCE_SECRET"),
    QuoteCurrency: exchanges.QuoteCurrencyUSDC,
})

// OKX — same interface, different constructor
adp, _ := okx.NewAdapter(ctx, okx.Options{
    APIKey: os.Getenv("OKX_API_KEY"), SecretKey: os.Getenv("OKX_SECRET"),
    Passphrase: os.Getenv("OKX_PASSPHRASE"),
})

// Hyperliquid — wallet-based auth (USDC only)
adp, _ := hyperliquid.NewAdapter(ctx, hyperliquid.Options{
    PrivateKey: os.Getenv("HL_PRIVATE_KEY"), AccountAddr: os.Getenv("HL_ADDR"),
})

// All adapters expose the exact same Exchange interface
ticker, _ := adp.FetchTicker(ctx, "BTC")
```

### Quote Currency

Each adapter supports a `QuoteCurrency` option that determines which quote currency market to connect to. If omitted, the exchange-specific default is used (CEX → USDT, DEX → USDC).

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
| | `FetchOrder(ctx, orderID, symbol)` | Get order status |
| | `FetchOpenOrders(ctx, symbol)` | List all open orders |
| **Account** | `FetchAccount(ctx)` | Full account: balance + positions + orders |
| | `FetchBalance(ctx)` | Available balance only |
| | `FetchSymbolDetails(ctx, symbol)` | Precision & min quantity rules |
| | `FetchFeeRate(ctx, symbol)` | Maker/taker fee rates |
| **Orderbook** | `WatchOrderBook(ctx, symbol, cb)` | Subscribe to WS orderbook (blocks until ready) |
| | `GetLocalOrderBook(symbol, depth)` | Read local WS-maintained orderbook |
| | `StopWatchOrderBook(ctx, symbol)` | Unsubscribe |
| **Streaming** | `WatchOrders(ctx, cb)` | Real-time order updates |
| | `WatchPositions(ctx, cb)` | Real-time position updates |
| | `WatchTicker(ctx, symbol, cb)` | Real-time ticker |
| | `WatchTrades(ctx, symbol, cb)` | Real-time trades |
| | `WatchKlines(ctx, symbol, interval, cb)` | Real-time klines |

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
        // Handle rate limit (auto-handled by built-in limiter)
    }

    // Access exchange-specific details
    var exErr *exchanges.ExchangeError
    if errors.As(err, &exErr) {
        fmt.Printf("[%s] Code: %s, Message: %s\n", exErr.Exchange, exErr.Code, exErr.Message)
    }
}
```

Available sentinel errors: `ErrInsufficientBalance`, `ErrRateLimited`, `ErrInvalidPrecision`, `ErrOrderNotFound`, `ErrSymbolNotFound`, `ErrMinNotional`, `ErrMinQuantity`, `ErrAuthFailed`, `ErrNetworkTimeout`, `ErrNotSupported`.

---

## Built-in Safety Features

### Rate Limiting

Each adapter comes with exchange-specific rate limiters pre-configured. The sliding-window rate limiter supports:
- Multiple time windows (e.g., 1200 req/min + 10 req/sec)
- Category-based weights (different endpoints consume different quotas)
- Automatic blocking until quota is available

### IP Ban Detection & Recovery

If an exchange returns a ban error (HTTP 418/429 with retry-after), the adapter:
1. Automatically parses the ban duration
2. Blocks all subsequent requests until the ban expires
3. Resumes normal operation — no manual intervention needed

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
├── base_adapter.go         Shared adapter logic (rate limit, ban, orderbook, validation)
├── local_state.go          LocalOrderBook interface + AccountManager
├── log.go                  Logger interface + NopLogger
├── ratelimit/              Declarative sliding-window rate limiter
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

Copy the example environment file and fill in your credentials:
```bash
cp .env.example .env
```

Run unit tests (no API keys needed):
```bash
go test ./ratelimit/ -v
go test . -run TestBan -v
go test -run "Test(Options|Format|Extract)" ./binance/ ./okx/ ./aster/ ./grvt/ -v  # QuoteCurrency tests
```

Run integration tests (requires API keys in `.env`):
```bash
go test ./binance/ -v      # Tests skip automatically if keys are missing
go test ./grvt/ -v
go test ./edgex/ -v
```

## License

MIT
