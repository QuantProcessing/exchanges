---
name: exchanges
description: >
  Unified Go SDK for cryptocurrency exchange integration. Provides a two-layer
  architecture: low-level SDK clients (REST + WebSocket per exchange) and
  high-level adapters implementing a common Exchange interface. Supports 8+
  exchanges (Binance, OKX, Hyperliquid, GRVT, EdgeX, Lighter, Nado, StandX)
  with Spot and Perp market types. All prices/quantities use decimal.Decimal.
  Symbols use base currency format ("BTC", "ETH"). Includes real-world recipes:
  public market data, cross-exchange hedging, grid trading, funding rate scanning,
  order monitoring with fill tracking, and account risk management. Use when
  building any trading application, strategy, CLI tool, or bot that interacts
  with crypto exchanges.
---

# Exchanges SDK

A unified Go library for interacting with multiple cryptocurrency exchanges via
a common `Exchange` interface. Covers both REST and WebSocket operations.

Module: `github.com/QuantProcessing/exchanges`

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│  Application Layer (your code)                                  │
│  Uses: Exchange / PerpExchange / SpotExchange interfaces        │
├─────────────────────────────────────────────────────────────────┤
│  Root Package (exchanges/)                                      │
│  Unified interfaces, models, errors, utilities, BaseAdapter     │
├──────────┬──────────┬──────────┬──────────┬─────────────────────┤
│ binance/ │   okx/   │hyperl…/  │  grvt/   │ edgex/ lighter/ …  │
│ adapter  │ adapter  │ adapter  │ adapter  │     adapters        │
├──────────┴──────────┴──────────┴──────────┴─────────────────────┤
│  Per-Exchange SDK Layer (sdk/ subdirectories)                   │
│  Raw REST clients + WebSocket clients + auth/signing            │
└─────────────────────────────────────────────────────────────────┘
```

### Key Design Principles

1. **Two-Layer Architecture**: Each exchange has a low-level SDK (raw API) and
   a high-level adapter (unified interface). Application code uses the adapter.
2. **Base Currency Symbols**: All methods accept `"BTC"`, `"ETH"` — never
   exchange-specific formats like `"BTCUSDT"` or `"BTC-USDT-SWAP"`.
3. **decimal.Decimal Everywhere**: All prices, quantities, and monetary values
   use `github.com/shopspring/decimal` — **never** `float64`.
4. **Callback-Driven WebSocket**: `Watch*` methods accept callbacks; the library
   handles connection management, reconnection, and dispatching.
5. **Registry Pattern**: Adapters self-register via `init()` + `exchanges.Register()`.
   Application code uses `exchanges.LookupConstructor()` and blank imports.
6. **Build Tags**: Some adapters (EdgeX, GRVT) use build tags to avoid pulling
   heavy crypto dependencies unless explicitly needed.

---

## Installation

```bash
go get github.com/QuantProcessing/exchanges
```

### Importing Specific Adapters

Each adapter must be imported to register itself. Use blank imports:

```go
import (
    exchanges "github.com/QuantProcessing/exchanges"

    // Import adapters you need (registers via init())
    _ "github.com/QuantProcessing/exchanges/binance"
    _ "github.com/QuantProcessing/exchanges/hyperliquid"
    _ "github.com/QuantProcessing/exchanges/okx"

    // Build-tag gated adapters (use: go build -tags edgex,grvt)
    _ "github.com/QuantProcessing/exchanges/edgex"
    _ "github.com/QuantProcessing/exchanges/grvt"
)
```

---

## Quick Start

### Direct Adapter Construction

```go
import (
    "context"
    exchanges "github.com/QuantProcessing/exchanges"
    "github.com/QuantProcessing/exchanges/binance"
    "go.uber.org/zap"
)

func main() {
    ctx := context.Background()
    logger := zap.Must(zap.NewProduction()).Sugar()

    adp, err := binance.NewPerpAdapter(ctx, binance.Options{
        APIKey:    "your-api-key",
        SecretKey: "your-secret-key",
        Logger:    logger,
    })
    if err != nil {
        panic(err)
    }
    defer adp.Close()

    ticker, err := adp.FetchTicker(ctx, "BTC")
    if err != nil {
        panic(err)
    }
    fmt.Printf("BTC price: %s\n", ticker.LastPrice)
}
```

### Registry-Based Construction

```go
import (
    _ "github.com/QuantProcessing/exchanges/binance"
)

func createAdapter(ctx context.Context, name string, opts map[string]string) (exchanges.Exchange, error) {
    ctor, err := exchanges.LookupConstructor(name)
    if err != nil {
        return nil, err // "unsupported exchange: ..."
    }
    return ctor(ctx, exchanges.MarketTypePerp, opts)
}

// Usage:
adp, err := createAdapter(ctx, "BINANCE", map[string]string{
    "api_key":    os.Getenv("BINANCE_API_KEY"),
    "secret_key": os.Getenv("BINANCE_SECRET_KEY"),
})
```

---

## API Reference

### Exchange (core interface — all adapters)

| Method | Signature | Purpose |
|--------|-----------|---------|
| **Identity** |||
| `GetExchange` | `() string` | Exchange name, e.g. `"BINANCE"` |
| `GetMarketType` | `() MarketType` | `"spot"` or `"perp"` |
| `FormatSymbol` | `(symbol) string` | Convert base → exchange format (internal) |
| `ExtractSymbol` | `(symbol) string` | Convert exchange format → base |
| `ListSymbols` | `() []string` | All supported symbols |
| `Close` | `() error` | Shutdown all connections |
| **Market Data (REST)** |||
| `FetchTicker` | `(ctx, symbol) → *Ticker` | Last price, bid, ask, mark price, 24h volume |
| `FetchOrderBook` | `(ctx, symbol, limit) → *OrderBook` | Depth snapshot: `[]Level{Price, Quantity}` |
| `FetchTrades` | `(ctx, symbol, limit) → []Trade` | Recent market trades |
| `FetchKlines` | `(ctx, symbol, interval, *KlineOpts) → []Kline` | OHLCV candlestick data |
| **Trading** |||
| `PlaceOrder` | `(ctx, *OrderParams) → *Order` | Place limit/market/post-only order |
| `CancelOrder` | `(ctx, orderID, symbol) → error` | Cancel by order ID |
| `CancelAllOrders` | `(ctx, symbol) → error` | Cancel all orders for symbol |
| `FetchOrder` | `(ctx, orderID, symbol) → *Order` | Query single order status |
| `FetchOpenOrders` | `(ctx, symbol) → []Order` | List open orders |
| **Account** |||
| `FetchAccount` | `(ctx) → *Account` | Full: balance, positions, open orders |
| `FetchBalance` | `(ctx) → decimal.Decimal` | Available balance only |
| `FetchSymbolDetails` | `(ctx, symbol) → *SymbolDetails` | Precision, min qty, min notional |
| `FetchFeeRate` | `(ctx, symbol) → *FeeRate` | Maker/taker fee rates |
| **Local OrderBook (WS-maintained)** |||
| `WatchOrderBook` | `(ctx, symbol, cb) → error` | Subscribe & maintain local orderbook; blocks until synced |
| `GetLocalOrderBook` | `(symbol, depth) → *OrderBook` | Pull current book state (requires prior `WatchOrderBook`) |
| `StopWatchOrderBook` | `(ctx, symbol) → error` | Unsubscribe |
| **WebSocket Streaming** |||
| `WatchOrders` | `(ctx, cb) → error` | Live order fills/status updates |
| `WatchPositions` | `(ctx, cb) → error` | Live position changes |
| `WatchTicker` | `(ctx, symbol, cb) → error` | Live ticker stream |
| `WatchTrades` | `(ctx, symbol, cb) → error` | Live trade stream |
| `WatchKlines` | `(ctx, symbol, interval, cb) → error` | Live kline stream |
| `StopWatch*` | mirrors above | Unsubscribe each stream |

### PerpExchange (type assert: `adp.(exchanges.PerpExchange)`)

| Method | Signature | Purpose |
|--------|-----------|---------|
| `FetchPositions` | `(ctx) → []Position` | All open positions |
| `SetLeverage` | `(ctx, symbol, leverage) → error` | Set leverage for symbol |
| `FetchFundingRate` | `(ctx, symbol) → *FundingRate` | Current funding rate |
| `FetchAllFundingRates` | `(ctx) → []FundingRate` | All symbols' funding rates |
| `ModifyOrder` | `(ctx, orderID, symbol, *ModifyOrderParams) → *Order` | Amend price/qty |

### SpotExchange (type assert: `adp.(exchanges.SpotExchange)`)

| Method | Signature | Purpose |
|--------|-----------|---------|
| `FetchSpotBalances` | `(ctx) → []SpotBalance` | Per-asset free/locked/total |
| `TransferAsset` | `(ctx, *TransferParams) → error` | Transfer between accounts |

### Convenience Functions

```go
exchanges.PlaceMarketOrder(ctx, adp, "BTC", exchanges.OrderSideBuy, qty)
exchanges.PlaceLimitOrder(ctx, adp, "BTC", exchanges.OrderSideBuy, price, qty)
exchanges.PlaceMarketOrderWithSlippage(ctx, adp, "BTC", exchanges.OrderSideBuy, qty, slippage)
exchanges.GenerateID()  // unique numeric string for ClientID
```

### Precision Utilities

```go
exchanges.RoundToPrecision(price, details.PricePrecision) // Round to N decimals
exchanges.FloorToPrecision(qty, details.QuantityPrecision) // Truncate to N decimals
exchanges.CountDecimalPlaces("0.00010")                     // → 4
exchanges.RoundToSignificantFigures(value, 4)               // → 4 sig figs
```

---

## Key Types

```go
// OrderParams — universal order input
OrderParams{
    Symbol      string
    Side        OrderSide       // OrderSideBuy / OrderSideSell
    Type        OrderType       // OrderTypeMarket / OrderTypeLimit / OrderTypePostOnly
    Quantity    decimal.Decimal
    Price       decimal.Decimal // required for LIMIT; ignored for MARKET
    TimeInForce TimeInForce     // GTC / IOC / FOK / PO
    ReduceOnly  bool
    Slippage    decimal.Decimal // >0 + MARKET → auto LIMIT+IOC
    ClientID    string
}

// Order — returned from PlaceOrder and WatchOrders callbacks
Order{
    OrderID        string
    Symbol         string
    Side           OrderSide
    Type           OrderType
    Status         OrderStatus // NEW / PENDING / PARTIALLY_FILLED / FILLED / CANCELLED / REJECTED
    Quantity       decimal.Decimal
    Price          decimal.Decimal
    FilledQuantity decimal.Decimal
    Fee            decimal.Decimal
    Timestamp      int64           // milliseconds
    ClientOrderID  string
    ReduceOnly     bool
    TimeInForce    TimeInForce
}

// Level — single orderbook price level
Level{ Price decimal.Decimal, Quantity decimal.Decimal }

// OrderBook — Bids descending, Asks ascending
OrderBook{ Symbol string, Bids []Level, Asks []Level, Timestamp int64 }

// Ticker
Ticker{
    Symbol, LastPrice, IndexPrice, MarkPrice, MidPrice,
    Bid, Ask, Volume24h, QuoteVol, High24h, Low24h decimal.Decimal
    Timestamp int64
}

// Position (perp only)
Position{
    Symbol            string
    Side              PositionSide    // LONG / SHORT / BOTH
    Quantity          decimal.Decimal
    EntryPrice        decimal.Decimal
    UnrealizedPnL     decimal.Decimal
    RealizedPnL       decimal.Decimal
    LiquidationPrice  decimal.Decimal
    Leverage          decimal.Decimal
    MaintenanceMargin decimal.Decimal
    MarginType        string          // "ISOLATED" or "CROSSED"
}

// SymbolDetails — trading precision rules
SymbolDetails{
    Symbol            string
    PricePrecision    int32           // Decimal places for price
    QuantityPrecision int32           // Decimal places for quantity
    MinQuantity       decimal.Decimal
    MinNotional       decimal.Decimal // price * qty minimum
}

// FundingRate
FundingRate{
    Symbol               string
    FundingRate          decimal.Decimal
    FundingIntervalHours int64
    FundingTime          int64
    NextFundingTime      int64
}

// Enums
MarketType:  MarketTypeSpot ("spot") / MarketTypePerp ("perp")
OrderMode:   OrderModeWS ("ws") / OrderModeREST ("rest")
Interval:    Interval1m / Interval5m / Interval15m / Interval1h / Interval4h / Interval1d / ...
```

---

## Error Handling

### Sentinel Errors

```go
import "errors"

order, err := adp.PlaceOrder(ctx, params)
if err != nil {
    switch {
    case errors.Is(err, exchanges.ErrInsufficientBalance):
        // Not enough funds
    case errors.Is(err, exchanges.ErrMinQuantity):
        // Below minimum quantity
    case errors.Is(err, exchanges.ErrMinNotional):
        // Below minimum notional value (price * qty)
    case errors.Is(err, exchanges.ErrRateLimited):
        // Rate limit hit — adapter auto-handles via AcquireRate
    case errors.Is(err, exchanges.ErrOrderNotFound):
        // Order doesn't exist
    case errors.Is(err, exchanges.ErrAuthFailed):
        // Bad API credentials
    case errors.Is(err, exchanges.ErrNotSupported):
        // Method not supported by this exchange
    default:
        // Exchange-specific or network error
    }
}
```

### ExchangeError (typed)

```go
var exErr *exchanges.ExchangeError
if errors.As(err, &exErr) {
    fmt.Printf("Exchange: %s, Code: %s, Message: %s\n",
        exErr.Exchange, exErr.Code, exErr.Message)
}
```

### Available Sentinel Errors

| Error | When |
|-------|------|
| `ErrInsufficientBalance` | Not enough funds |
| `ErrRateLimited` | Rate limit exceeded |
| `ErrInvalidPrecision` | Bad precision values |
| `ErrOrderNotFound` | Order doesn't exist |
| `ErrSymbolNotFound` | Symbol not available |
| `ErrMinNotional` | Below min notional (price × qty) |
| `ErrMinQuantity` | Below min order quantity |
| `ErrAuthFailed` | Authentication failure |
| `ErrNetworkTimeout` | Network timeout |
| `ErrNotSupported` | Feature not supported |

---

## Logger Interface

All adapters accept an optional `exchanges.Logger` interface (compatible with
`*zap.SugaredLogger`):

```go
type Logger interface {
    Debugw(msg string, keysAndValues ...any)
    Infow(msg string, keysAndValues ...any)
    Warnw(msg string, keysAndValues ...any)
    Errorw(msg string, keysAndValues ...any)
}
```

If no logger is provided, `exchanges.NopLogger` is used (all output discarded).

---

## Rate Limiting & Ban Detection

Rate limiting and IP ban detection are **built into** every adapter. You do not
need to implement any rate limiting in application code.

- All REST methods automatically call `AcquireRate()` before execution
- If the exchange returns an IP ban error, the adapter auto-detects it via
  `BanState.ParseAndSetBan()` and blocks subsequent requests until the ban expires
- Use `context.WithTimeout` to control maximum wait time

```go
// Context timeout controls max wait for rate limit clearance
ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
defer cancel()
ticker, err := adp.FetchTicker(ctx, "BTC") // auto-rate-limited
```

---

## Scenario 0: Public Market Data (No Credentials)

**Goal**: Monitor orderbooks or fetch tickers without API keys.

```go
func monitorPublicBBO(ctx context.Context) {
    // Use registry with empty credentials for public-only access
    ctor, _ := exchanges.LookupConstructor("BINANCE")
    adp, err := ctor(ctx, exchanges.MarketTypePerp, map[string]string{})
    if err != nil {
        log.Fatal(err)
    }
    defer adp.Close()

    // Market data works without authentication
    ticker, _ := adp.FetchTicker(ctx, "BTC")
    fmt.Printf("BTC: bid=%s ask=%s\n", ticker.Bid, ticker.Ask)

    // WS orderbook also works without credentials
    adp.WatchOrderBook(ctx, "BTC", func(ob *exchanges.OrderBook) {
        if len(ob.Bids) > 0 && len(ob.Asks) > 0 {
            fmt.Printf("BBO: %s / %s\n", ob.Bids[0].Price, ob.Asks[0].Price)
        }
    })
}
```

---

## Scenario 1: Cross-Exchange BBO Monitoring & Spread Analysis

**Goal**: Monitor best bid/ask from two exchanges, compute spread in real-time.

```go
func monitorSpread(ctx context.Context, maker, taker exchanges.Exchange, symbol string, thresholdBps float64) {
    var mu sync.Mutex
    var makerBid, makerAsk, takerBid, takerAsk decimal.Decimal

    check := func() {
        mu.Lock()
        defer mu.Unlock()
        if makerAsk.IsZero() || takerBid.IsZero() { return }

        spread := takerBid.Sub(makerAsk).Div(makerAsk).Mul(decimal.NewFromInt(10000))
        if spreadF, _ := spread.Float64(); spreadF > thresholdBps {
            log.Printf("🟢 SIGNAL: spread=%.1f bps, maker_ask=%s, taker_bid=%s",
                spreadF, makerAsk, takerBid)
        }
    }

    // Subscribe both — callback-driven, no polling
    maker.WatchOrderBook(ctx, symbol, func(ob *exchanges.OrderBook) {
        if len(ob.Bids) == 0 || len(ob.Asks) == 0 { return }
        mu.Lock()
        makerBid = ob.Bids[0].Price
        makerAsk = ob.Asks[0].Price
        mu.Unlock()
        check()
    })
    taker.WatchOrderBook(ctx, symbol, func(ob *exchanges.OrderBook) {
        if len(ob.Bids) == 0 || len(ob.Asks) == 0 { return }
        mu.Lock()
        takerBid = ob.Bids[0].Price
        takerAsk = ob.Asks[0].Price
        mu.Unlock()
        check()
    })
}
```

**Key patterns**: `WatchOrderBook` callback for event-driven updates; `sync.Mutex`
for safe concurrent state; `decimal` arithmetic for BPS computation.

---

## Scenario 2: Cross-Exchange Hedging (Maker-Taker Pattern)

**Goal**: Place a maker limit order on exchange A, monitor fills via WS, immediately
hedge each fill with a market order on exchange B.

```go
func hedgeCycle(ctx context.Context, maker, taker exchanges.Exchange, symbol string, qty decimal.Decimal) {
    // 1. Subscribe to order updates BEFORE placing any orders
    makerCh := make(chan *exchanges.Order, 100)
    maker.WatchOrders(ctx, func(o *exchanges.Order) {
        select {
        case makerCh <- o:
        case <-ctx.Done():
        }
    })

    // 2. Get BBO from local orderbook for pricing
    ob := maker.GetLocalOrderBook(symbol, 1)
    if ob == nil || len(ob.Bids) == 0 {
        return
    }

    // 3. Fetch symbol details for precision
    details, _ := maker.FetchSymbolDetails(ctx, symbol)

    // 4. Place maker BUY limit below best bid
    limitPrice := ob.Bids[0].Price.Sub(decimal.NewFromFloat(0.5))
    limitPrice = exchanges.RoundToPrecision(limitPrice, details.PricePrecision)

    makerOrder, err := maker.PlaceOrder(ctx, &exchanges.OrderParams{
        Symbol:      symbol,
        Side:        exchanges.OrderSideBuy,
        Type:        exchanges.OrderTypeLimit,
        Price:       limitPrice,
        Quantity:    qty,
        TimeInForce: exchanges.TimeInForcePO,
    })
    if err != nil { return }

    // 5. Monitor fills — hedge each fill to taker immediately
    lastFilled := makerOrder.FilledQuantity
    for {
        select {
        case <-ctx.Done():
            return
        case update := <-makerCh:
            // Match by OrderID or ClientOrderID
            isMatch := update.OrderID == makerOrder.OrderID ||
                (makerOrder.ClientOrderID != "" && update.ClientOrderID == makerOrder.ClientOrderID)
            if !isMatch { continue }

            diff := update.FilledQuantity.Sub(lastFilled)
            if diff.GreaterThan(decimal.Zero) {
                // Hedge: SELL same quantity on taker
                exchanges.PlaceMarketOrder(ctx, taker, symbol, exchanges.OrderSideSell, diff)
                lastFilled = update.FilledQuantity
            }

            if update.Status == exchanges.OrderStatusFilled ||
                update.Status == exchanges.OrderStatusCancelled {
                return
            }
        }
    }
}
```

**Key patterns**: `WatchOrders` BEFORE `PlaceOrder`; order matching by both
`OrderID` and `ClientOrderID`; incremental fill detection via `FilledQuantity`
delta; `PlaceMarketOrder` convenience for hedging.

---

## Scenario 3: Grid Trading with Precision & Fill-Driven Replacement

**Goal**: Place a grid of limit orders, track fills via WS, replace each filled
order with the opposite side.

```go
func runGrid(ctx context.Context, adp exchanges.PerpExchange, symbol string,
    midPrice decimal.Decimal, gridSize int, stepBps float64, qtyPerLevel float64,
) {
    // 1. MUST fetch precision before any calculation
    details, _ := adp.FetchSymbolDetails(ctx, symbol)
    step := decimal.NewFromFloat(stepBps / 10000)
    qty := exchanges.FloorToPrecision(
        decimal.NewFromFloat(qtyPerLevel), details.QuantityPrecision,
    )

    // 2. Calculate grid prices & place orders
    type GridOrder struct {
        Price decimal.Decimal
        Side  exchanges.OrderSide
        CID   string
    }
    var orders []GridOrder

    for i := 1; i <= gridSize; i++ {
        offset := step.Mul(decimal.NewFromInt(int64(i)))
        buyPrice := exchanges.RoundToPrecision(
            midPrice.Mul(decimal.NewFromInt(1).Sub(offset)), details.PricePrecision)
        sellPrice := exchanges.RoundToPrecision(
            midPrice.Mul(decimal.NewFromInt(1).Add(offset)), details.PricePrecision)

        buyCID := exchanges.GenerateID()
        sellCID := exchanges.GenerateID()

        adp.PlaceOrder(ctx, &exchanges.OrderParams{
            Symbol: symbol, Side: exchanges.OrderSideBuy, Type: exchanges.OrderTypeLimit,
            Price: buyPrice, Quantity: qty, TimeInForce: exchanges.TimeInForceGTC,
            ClientID: buyCID,
        })
        adp.PlaceOrder(ctx, &exchanges.OrderParams{
            Symbol: symbol, Side: exchanges.OrderSideSell, Type: exchanges.OrderTypeLimit,
            Price: sellPrice, Quantity: qty, TimeInForce: exchanges.TimeInForceGTC,
            ClientID: sellCID,
        })

        orders = append(orders,
            GridOrder{Price: buyPrice, Side: exchanges.OrderSideBuy, CID: buyCID},
            GridOrder{Price: sellPrice, Side: exchanges.OrderSideSell, CID: sellCID},
        )
    }

    // 3. Monitor fills — replace with opposite side
    adp.WatchOrders(ctx, func(o *exchanges.Order) {
        if o.Status != exchanges.OrderStatusFilled { return }
        for _, g := range orders {
            if o.ClientOrderID == g.CID {
                oppSide := exchanges.OrderSideSell
                if g.Side == exchanges.OrderSideSell {
                    oppSide = exchanges.OrderSideBuy
                }
                // Replace in goroutine to avoid blocking WS readLoop
                go adp.PlaceOrder(ctx, &exchanges.OrderParams{
                    Symbol: symbol, Side: oppSide, Type: exchanges.OrderTypeLimit,
                    Price: g.Price, Quantity: qty, TimeInForce: exchanges.TimeInForceGTC,
                    ClientID: exchanges.GenerateID(),
                })
                break
            }
        }
    })
}
```

**Key patterns**: `FetchSymbolDetails` for precision BEFORE computing prices;
`RoundToPrecision` for price, `FloorToPrecision` for quantity; `GenerateID()` for
unique client IDs; `WatchOrders` for fill-driven order replacement; `go` prefix
for order placement in callbacks to avoid blocking the WS read loop.

---

## Scenario 4: Post-Only Maker with Timeout & Cancel

**Goal**: Place a POST-ONLY limit order, wait for fill or cancel on timeout.

```go
func postOnlyWithTimeout(ctx context.Context, adp exchanges.Exchange, symbol string,
    side exchanges.OrderSide, price, qty decimal.Decimal, timeout time.Duration,
) (decimal.Decimal, error) {
    // 1. Listen for updates BEFORE placing
    fillCh := make(chan decimal.Decimal, 1)
    cid := exchanges.GenerateID()

    adp.WatchOrders(ctx, func(o *exchanges.Order) {
        if o.ClientOrderID != cid { return }
        switch o.Status {
        case exchanges.OrderStatusFilled, exchanges.OrderStatusCancelled, exchanges.OrderStatusRejected:
            fillCh <- o.FilledQuantity
        }
    })

    // 2. Place POST-ONLY order (TimeInForce = PO)
    order, err := adp.PlaceOrder(ctx, &exchanges.OrderParams{
        Symbol:      symbol,
        Side:        side,
        Type:        exchanges.OrderTypePostOnly,
        Price:       price,
        Quantity:    qty,
        TimeInForce: exchanges.TimeInForcePO,
        ClientID:    cid,
    })
    if err != nil { return decimal.Zero, err }

    // 3. Wait for fill or timeout
    select {
    case filled := <-fillCh:
        return filled, nil
    case <-time.After(timeout):
        cancelCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
        defer cancel()
        adp.CancelOrder(cancelCtx, order.OrderID, symbol)
        select {
        case filled := <-fillCh:
            return filled, nil
        case <-time.After(2 * time.Second):
            return decimal.Zero, nil
        }
    }
}
```

**Key patterns**: `WatchOrders` + `ClientID` matching; `GenerateID()` for unique
tracking; terminal status checks (`Filled`/`Cancelled`/`Rejected`); timeout-driven
cancel with separate context.

---

## Scenario 5: Market Order with Slippage Protection

**Goal**: Execute a market buy with configurable slippage cap.

```go
func safeBuy(ctx context.Context, adp exchanges.Exchange, symbol string, qty decimal.Decimal) error {
    // Slippage=0.002 → willing to pay up to 0.2% above current ask
    // Internally: fetches ticker → computes limit price → places LIMIT IOC
    order, err := exchanges.PlaceMarketOrderWithSlippage(
        ctx, adp, symbol, exchanges.OrderSideBuy, qty, decimal.NewFromFloat(0.002),
    )
    if err != nil { return err }

    // Verify fill
    time.Sleep(time.Second)
    result, _ := adp.FetchOrder(ctx, order.OrderID, symbol)
    if result.Status != exchanges.OrderStatusFilled {
        log.Printf("⚠️ Partial fill: %s/%s", result.FilledQuantity, qty)
    }
    return nil
}
```

---

## Scenario 6: Funding Rate Arbitrage Scanner

**Goal**: Scan all exchanges for funding rate differences.

```go
func scanFundingArb(ctx context.Context, adapters []exchanges.Exchange) {
    type Rate struct {
        Exchange string
        Rate     decimal.Decimal
        Symbol   string
    }
    var allRates []Rate

    for _, adp := range adapters {
        perp, ok := adp.(exchanges.PerpExchange) // ← type assertion
        if !ok { continue }

        rates, err := perp.FetchAllFundingRates(ctx)
        if err != nil { continue }

        for _, r := range rates {
            allRates = append(allRates, Rate{
                Exchange: adp.GetExchange(),
                Rate:     r.FundingRate,
                Symbol:   r.Symbol,
            })
        }
    }

    // Group by symbol, find largest spread
    grouped := make(map[string][]Rate)
    for _, r := range allRates {
        grouped[r.Symbol] = append(grouped[r.Symbol], r)
    }

    for symbol, rates := range grouped {
        if len(rates) < 2 { continue }
        maxRate, minRate := rates[0], rates[0]
        for _, r := range rates {
            if r.Rate.GreaterThan(maxRate.Rate) { maxRate = r }
            if r.Rate.LessThan(minRate.Rate) { minRate = r }
        }
        diff := maxRate.Rate.Sub(minRate.Rate).Abs()
        if diff.GreaterThan(decimal.NewFromFloat(0.001)) {
            log.Printf("💰 %s: long %s (%s), short %s (%s), spread=%s",
                symbol, minRate.Exchange, minRate.Rate, maxRate.Exchange, maxRate.Rate, diff)
        }
    }
}
```

**Key patterns**: `PerpExchange` type assertion; `FetchAllFundingRates`; cross-exchange
comparison with `GetExchange()`.

---

## Scenario 7: Account Risk Monitor with Auto-Close

**Goal**: Live dashboard that monitors position PnL and triggers emergency close.

```go
func riskMonitor(ctx context.Context, adp exchanges.Exchange, maxLoss decimal.Decimal) {
    perp, ok := adp.(exchanges.PerpExchange)
    if !ok { panic("need perp exchange") }

    // 1. Initial snapshot
    acc, _ := adp.FetchAccount(ctx)
    log.Printf("Balance: %s, UnrealizedPnL: %s, Positions: %d",
        acc.TotalBalance, acc.UnrealizedPnL, len(acc.Positions))

    // 2. Live position monitoring with risk check
    adp.WatchPositions(ctx, func(pos *exchanges.Position) {
        if pos.UnrealizedPnL.IsNegative() && pos.UnrealizedPnL.Abs().GreaterThan(maxLoss) {
            log.Printf("🚨 MAX LOSS on %s: PnL=%s — closing!", pos.Symbol, pos.UnrealizedPnL)

            side := exchanges.OrderSideSell
            qty := pos.Quantity
            if pos.Quantity.IsNegative() {
                side = exchanges.OrderSideBuy
                qty = qty.Abs()
            }
            exchanges.PlaceMarketOrder(ctx, adp, pos.Symbol, side, qty)
        }
    })

    // 3. Periodic balance refresh
    ticker := time.NewTicker(30 * time.Second)
    for range ticker.C {
        positions, _ := perp.FetchPositions(ctx)
        balance, _ := adp.FetchBalance(ctx)
        totalPnL := decimal.Zero
        for _, p := range positions {
            totalPnL = totalPnL.Add(p.UnrealizedPnL)
        }
        log.Printf("📊 Balance=%s PnL=%s Positions=%d", balance, totalPnL, len(positions))
    }
}
```

---

## Scenario 8: AccountManager for Auto-Synced State

**Goal**: Use the built-in `AccountManager` for automatic position/order/balance
tracking instead of manual WatchOrders/WatchPositions management.

```go
func useAccountManager(ctx context.Context, adp exchanges.PerpExchange, logger exchanges.Logger) {
    mgr, err := exchanges.NewAccountManager(adp, logger)
    if err != nil { panic(err) }
    defer mgr.Close()

    // Start syncs initial state + subscribes to WS updates + starts periodic refresh
    if err := mgr.Start(ctx, 1*time.Minute); err != nil {
        panic(err)
    }

    // Read-only queries — always up-to-date
    balance := mgr.GetLocalBalance()
    positions := mgr.GetAllPositions()
    order, found := mgr.GetOrder("some-order-id")

    // Stream channels for event-driven processing
    for o := range mgr.GetOrderStream() {
        fmt.Printf("Order update: %s status=%s\n", o.OrderID, o.Status)
    }
}
```

---

## Order Validation (Auto)

Order validation is **automatic** in all adapters. When you call `PlaceOrder`:

1. `BaseAdapter.ValidateOrder` auto-rounds price and truncates quantity using
   cached `SymbolDetails`
2. If `Slippage > 0` and `Type == MARKET`, `BaseAdapter.ApplySlippage` converts
   it to a LIMIT IOC with slippage-adjusted price
3. Errors like `ErrMinQuantity` and `ErrMinNotional` are returned before the
   order reaches the exchange

You should still call `FetchSymbolDetails` when doing **your own** price/qty
calculations (grid levels, tick offsets, etc.).

---

## Adding a New Exchange Adapter

When implementing a new exchange adapter, follow these steps:

### 1. Register via init()

```go
// In <exchange>/perp_adapter.go
func init() {
    exchanges.Register("MYEXCHANGE", func(ctx context.Context, mt exchanges.MarketType, opts map[string]string) (exchanges.Exchange, error) {
        return NewPerpAdapter(ctx, Options{
            APIKey:    opts["api_key"],
            SecretKey: opts["secret_key"],
        })
    })
}
```

### 2. Embed BaseAdapter

```go
type PerpAdapter struct {
    *exchanges.BaseAdapter
    // ... exchange-specific fields
}

func NewPerpAdapter(ctx context.Context, opts Options) (*PerpAdapter, error) {
    a := &PerpAdapter{
        BaseAdapter: exchanges.NewBaseAdapter("MYEXCHANGE", exchanges.MarketTypePerp, opts.Logger),
    }
    // Load symbol details into cache
    details := loadSymbolDetails()
    a.SetSymbolDetails(details)
    // Configure rate limiting
    a.WithRateLimiter(rateLimitRules, rateLimitWeights)
    return a, nil
}
```

### 3. Implement Exchange Interface

Every method should:
- Call `a.AcquireRate(ctx, "MethodName")` at the start (REST methods)
- Call `a.RecordBan(err)` on errors (REST methods)
- Call `a.ValidateOrder(params)` + `a.ApplySlippage(ctx, params, a.FetchTicker)` in `PlaceOrder`
- Use `a.FormatSymbol(symbol)` for outgoing requests
- Use `a.ExtractSymbol(exchangeSymbol)` for incoming responses

### 4. Implement LocalOrderBook

Create `orderbook.go` implementing the `exchanges.LocalOrderBook` interface:

```go
type OrderBook struct {
    // exchange-specific sync logic
}
func (ob *OrderBook) GetDepth(limit int) ([]exchanges.Level, []exchanges.Level) { ... }
func (ob *OrderBook) WaitReady(ctx context.Context, timeout time.Duration) bool { ... }
func (ob *OrderBook) Timestamp() int64 { ... }
```

### 5. Run Compliance Tests

```go
// In <exchange>/adapter_test.go
func TestCompliance(t *testing.T) {
    adp := createTestAdapter(t)
    testsuite.RunComplianceSuite(t, adp)
}
```

---

## Common Pitfalls

| ❌ Don't | ✅ Do |
|----------|------|
| `float64` for price/qty | `decimal.Decimal` everywhere |
| `ob.Bids[0][0]` | `ob.Bids[0].Price` (Level is a struct) |
| `fmt.Sprintf("%f", price)` | `price.StringFixed(precision)` or `price.String()` |
| Poll with `FetchTicker` in a loop | Use `WatchTicker` or `WatchOrderBook` callback |
| Place order then subscribe | Subscribe `WatchOrders` **before** `PlaceOrder` |
| `adp.FetchPositions(ctx)` on `Exchange` | Type-assert to `PerpExchange` first |
| Ignore `FetchSymbolDetails` | Always fetch precision before computing prices |
| `panic` on errors | Return errors, log, or handle gracefully |
| Map orders by `OrderID` from `PlaceOrder` | **Use `ClientOrderID` as map key** — `PlaceOrder` may return ClientID as OrderID; `WatchOrders` returns exchange-assigned OrderID. `ClientOrderID` is the only stable key. |
| Access shared state in callbacks without lock | **Always `sync.Mutex`** — callbacks may run concurrently |
| Place orders directly in WS callbacks | **Use `go` prefix** — callbacks run in WS read loop; blocking delays all message processing |
| Manual rate limiting in app code | Rate limiting is built into adapters via `AcquireRate` |
| `exchanges.GenerateID(16)` | `exchanges.GenerateID()` — no argument needed (returns unique numeric string) |
| Ignoring `PartiallyFilled` status | Handle `OrderStatusPartiallyFilled` as non-terminal — keep monitoring |
| Using `ReduceOnly: true` for opening positions | `ReduceOnly` is for closing only — set `false` for new positions |
| Forget `defer adp.Close()` | Always close adapters to release WS connections and goroutines |

---

## Supported Exchanges

| Exchange | Package | Perp | Spot | Build Tag | Order Mode |
|----------|---------|------|------|-----------|------------|
| Binance | `binance/` | ✅ | ✅ | — | WS / REST |
| OKX | `okx/` | ✅ | ✅ | — | WS / REST |
| Hyperliquid | `hyperliquid/` | ✅ | ✅ | — | REST |
| GRVT | `grvt/` | ✅ | — | `grvt` | WS |
| EdgeX | `edgex/` | ✅ | — | `edgex` | WS |
| Lighter | `lighter/` | ✅ | — | — | WS |
| Nado | `nado/` | ✅ | — | — | WS |
| StandX | `standx/` | ✅ | — | — | WS |
| Aster | `aster/` | ✅ | — | — | REST |

### Build Tags

```bash
# Include EdgeX and GRVT adapters
go build -tags "edgex,grvt" ./...

# Run tests with all adapters
go test -tags "edgex,grvt" ./...
```

---

## Testing

### Compliance Test Suite

The `testsuite/` package provides a standardized compliance test suite:

```go
import "github.com/QuantProcessing/exchanges/testsuite"

// Full lifecycle test (requires live credentials)
testsuite.RunLifecycleSuite(t, adp, "BTC")

// Order flow test
testsuite.RunOrderSuite(t, adp, "BTC")

// Compliance check (interface satisfaction)
testsuite.RunComplianceSuite(t, adp)
```

### Environment Variables

All adapters load credentials from environment variables via `.env` files:

```bash
# .env
BINANCE_API_KEY=xxx
BINANCE_SECRET_KEY=xxx
HYPERLIQUID_API_KEY=xxx
HYPERLIQUID_SECRET_KEY=xxx
# ... etc
```
