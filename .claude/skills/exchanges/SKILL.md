---
name: exchanges
description: Use when reading, debugging, extending, or generating code for the QuantProcessing exchanges Go repository, especially when choosing adapters, using the unified Exchange interfaces, wiring registry-based construction, implementing order and stream behavior, or adding and fixing exchange adapters.
---

# Exchanges

## Overview

This repository is a unified Go SDK for crypto exchanges with two layers:

- High-level adapters implementing `exchanges.Exchange`
- Low-level per-exchange SDK clients under each exchange's `sdk/` directory

The most important rule for this project:

**Trust the code in this repository over README text, older skills, or generic exchange-library assumptions.**

Several docs in this repo ecosystem have drifted before. Future agents should verify constructor names, test helpers, option fields, and supported markets from the source files listed below before answering or editing.

## Source of Truth

When answering a question or making a change, start from the smallest set of authoritative files:

| Need | Read first | Why |
|------|------------|-----|
| Unified interfaces | `exchange.go` | Real shared contract: `Exchange`, `PerpExchange`, `SpotExchange`, `Streamable`, `OrderMode` |
| Shared models/enums | `models.go` | Real fields and enum names; do not invent types |
| Error handling | `errors.go` | Sentinel errors and `ExchangeError` contract |
| Shared validation/helpers | `utils.go` | Precision formatting, `GenerateID`, slippage helpers, partial-fill helper |
| Local state / account sync | `local_state.go`, `event_bus.go` | `LocalOrderBook`, `LocalState`, `EventBus` |
| Registry behavior | `registry.go` | Supported registry keys, constructor signature |
| Real expectations for adapters | `testsuite/compliance.go`, `testsuite/order_suite.go`, `testsuite/lifecycle_suite.go`, `testsuite/helpers.go` | These are the behavioral contracts |
| Exchange-specific constructor/auth/support | `<exchange>/register.go`, `<exchange>/options.go` | Real ctor names, registry keys, auth fields, quote currency rules |
| Exchange-specific business logic | `<exchange>/perp_adapter.go`, `<exchange>/spot_adapter.go`, `<exchange>/common.go` | Real implementation paths |
| Local orderbook implementation | `<exchange>/orderbook.go` or `*_orderbook.go` | Sync logic differs per exchange |

If you only remember one workflow, remember this one:

1. Determine task type.
2. Open the corresponding source-of-truth files.
3. Only then answer or edit.

## Non-Negotiable Invariants

These are the project rules agents most often get wrong:

1. **Use base symbols only at the adapter layer.** Pass `"BTC"`, not `"BTCUSDT"` or `"BTC-USDT-SWAP"`, unless you are working directly in the exchange SDK layer.
2. **Use `decimal.Decimal` for prices, quantities, balances, and PnL.** Do not switch to `float64` for adapter-facing logic.
3. **`OrderParams.ClientID` is the input field; `Order.ClientOrderID` is the output field.** They are related but not the same identifier name.
4. **Subscribe before placing orders when you care about fills.** `WatchOrders` should usually be established before `PlaceOrder`.
5. **`GetLocalOrderBook` only makes sense after `WatchOrderBook`.** Before subscription it can return `nil`.
6. **Perp-only methods are not on `Exchange`.** Type-assert to `PerpExchange`.
7. **Spot-only methods are not on `Exchange`.** Type-assert to `SpotExchange`.
8. **Binance margin is not part of the shared root interfaces.** It is a direct concrete adapter path, not a unified `MarginExchange` contract.
9. **Do not infer position direction from signed quantity.** Use `Position.Side`.
10. **Code beats docs when they conflict.**

## Task Routing

Use this table to decide what to load first instead of reading the entire repo.

| Task | Open first | Then inspect |
|------|------------|--------------|
| Use an existing adapter in app code | `exchange.go`, target `<exchange>/options.go` | target `<exchange>/register.go`, README example if needed |
| Build adapters dynamically from config | `registry.go` | target `<exchange>/register.go`, `<exchange>/options.go` |
| Add or fix order placement | `exchange.go`, `utils.go` | target `PlaceOrder`, `FetchOrder`, `WatchOrders`, `testsuite/order_suite.go` |
| Fix order status / partial-fill behavior | `models.go`, `utils.go` | target `WatchOrders`, response mapping helpers, `testsuite/helpers.go` |
| Fix local orderbook behavior | `local_state.go` | target `WatchOrderBook`, local orderbook file, `testsuite/compliance.go` |
| Fix account sync / local state | `local_state.go`, `event_bus.go` | target WS account handlers, `FetchAccount`, `WatchOrders`, `WatchPositions` |
| Add a new exchange adapter | `registry.go`, `exchange.go`, `testsuite/*` | copy a similar exchange package structure |
| Answer "what auth/options does exchange X need?" | `<exchange>/options.go` | `<exchange>/register.go` |
| Answer "what markets does exchange X support?" | `<exchange>/register.go` | presence of `spot_adapter.go` / `perp_adapter.go` |
| Use Binance margin | `binance/margin_adapter.go` | `binance/sdk/margin/*` |

## Project Map

### Root package

| File | Purpose | Typical reason to open |
|------|---------|------------------------|
| `exchange.go` | Core interfaces and callbacks | Check what is truly unified |
| `models.go` | Shared domain models and enums | Verify field names and enum values |
| `errors.go` | Sentinel errors and `ExchangeError` | Structured error handling |
| `utils.go` | Precision, ID generation, validation helpers | Avoid re-implementing rounding or order validation |
| `base_adapter.go` | Shared adapter behavior | See how order validation, slippage, local books, and order mode are meant to work |
| `local_state.go` | `LocalOrderBook` interface + unified `LocalState` manager | Implement or debug state sync |
| `event_bus.go` | Generic `EventBus[T]` fan-out pub/sub | Understand event distribution for orders/positions |
| `registry.go` | Global adapter registry | Config-driven construction |
| `manager.go` | Runtime holder for multiple adapters | Store and retrieve already-built adapters |
| `testsuite/` | Shared behavioral tests | Understand required adapter behavior |

### Per-exchange package layout

Most exchange packages follow a stable pattern:

```text
<exchange>/
  options.go
  register.go
  common.go
  perp_adapter.go
  spot_adapter.go        # if supported
  orderbook.go or *_orderbook.go
  funding.go             # for perp exchanges
  adapter_test.go
  sdk/
```

Interpret that pattern like this:

- `options.go`: source of truth for auth fields and quote currency defaults
- `register.go`: source of truth for registry name and supported market types
- `common.go`: symbol mapping and exchange-specific shared helpers
- `perp_adapter.go` / `spot_adapter.go`: source of truth for direct constructor names and business logic
- `adapter_test.go`: best local example for how that exchange is expected to work in practice

## Real Adapter Matrix

Do not invent constructor names. In the current codebase, perp constructors are usually `NewAdapter`, not `NewPerpAdapter`.

| Exchange | Package | Registry key | Direct constructors | Markets in registry | Registry/auth keys | Default quote | Notes |
|----------|---------|--------------|---------------------|---------------------|-------------------|---------------|-------|
| Binance | `binance` | `BINANCE` | `NewAdapter`, `NewSpotAdapter`, `NewMarginAdapter` | perp, spot | `api_key`, `secret_key`, `quote_currency` | `USDT` | Margin exists only as direct construction; not registered as separate market |
| OKX | `okx` | `OKX` | `NewAdapter`, `NewSpotAdapter` | perp, spot | `api_key`, `secret_key`, `passphrase`, `quote_currency` | `USDT` | Requires passphrase |
| Aster | `aster` | `ASTER` | `NewAdapter`, `NewSpotAdapter` | perp, spot | `api_key`, `secret_key`, `quote_currency` | `USDC` | Default quote differs from Binance/OKX |
| Nado | `nado` | `NADO` | `NewAdapter`, `NewSpotAdapter` | perp, spot | `private_key`, `sub_account_name`, `quote_currency` | `USDT` | |
| Lighter | `lighter` | `LIGHTER` | `NewAdapter`, `NewSpotAdapter` | perp, spot | `private_key`, `account_index`, `key_index`, `ro_token`, `quote_currency` | `USDC` | |
| Hyperliquid | `hyperliquid` | `HYPERLIQUID` | `NewAdapter`, `NewSpotAdapter` | perp, spot | `private_key`, `account_addr`, `quote_currency` | `USDC` | |
| StandX | `standx` | `STANDX` | `NewAdapter` | perp only | `private_key`, `quote_currency` | `DUSD` | No spot adapter |
| GRVT | `grvt` | `GRVT` | `NewAdapter` | perp only | `api_key`, `sub_account_id`, `private_key`, `quote_currency` | `USDT` | No spot adapter |
| EdgeX | `edgex` | `EDGEX` | `NewAdapter` | perp only | `private_key`, `account_id`, `quote_currency` | `USDC` | No spot adapter |
| Decibel | `decibel` | `DECIBEL` | `NewAdapter` | perp only | `api_key`, `private_key`, `subaccount_addr`, `quote_currency` | `USDC` | Hybrid adapter: authenticated REST/WS reads + Aptos write path |

### Quote currency rules

Always verify in `<exchange>/options.go` before answering or changing quote behavior.

| Exchange | Supported quotes |
|----------|------------------|
| Binance | `USDT`, `USDC` |
| OKX | `USDT`, `USDC` |
| Aster | `USDT`, `USDC` |
| Nado | `USDT` |
| Lighter | `USDC` |
| Hyperliquid | `USDC` |
| StandX | `DUSD` |
| GRVT | `USDT` |
| EdgeX | `USDC` |
| Decibel | `USDC` |

## Construction Patterns

### Direct construction

Use direct constructors when the caller already knows the exchange package at compile time.

```go
package main

import (
    "context"
    "fmt"

    exchanges "github.com/QuantProcessing/exchanges"
    "github.com/QuantProcessing/exchanges/binance"
)

func main() {
    ctx := context.Background()

    adp, err := binance.NewAdapter(ctx, binance.Options{
        APIKey:        "key",
        SecretKey:     "secret",
        QuoteCurrency: exchanges.QuoteCurrencyUSDT,
    })
    if err != nil {
        panic(err)
    }
    defer adp.Close()

    ticker, err := adp.FetchTicker(ctx, "BTC")
    if err != nil {
        panic(err)
    }

    fmt.Println(ticker.LastPrice)
}
```

### Registry-based construction

Use the registry when config chooses the exchange and market type at runtime.

```go
package main

import (
    "context"
    "fmt"

    exchanges "github.com/QuantProcessing/exchanges"

    _ "github.com/QuantProcessing/exchanges/binance"
    _ "github.com/QuantProcessing/exchanges/okx"
)

func build(ctx context.Context, name string, mt exchanges.MarketType, opts map[string]string) (exchanges.Exchange, error) {
    ctor, err := exchanges.LookupConstructor(name)
    if err != nil {
        return nil, err
    }
    return ctor(ctx, mt, opts)
}

func main() {
    ctx := context.Background()
    adp, err := build(ctx, "BINANCE", exchanges.MarketTypePerp, map[string]string{
        "api_key":        "key",
        "secret_key":     "secret",
        "quote_currency": "USDT",
    })
    if err != nil {
        panic(err)
    }
    defer adp.Close()

    fmt.Println(adp.GetExchange(), adp.GetMarketType())
}
```

### Public market-data-only access

Many public data paths work without credentials. When you only need ticker/orderbook/trades, try empty options first.

```go
ctor, err := exchanges.LookupConstructor("BINANCE")
if err != nil {
    panic(err)
}

adp, err := ctor(context.Background(), exchanges.MarketTypePerp, map[string]string{})
if err != nil {
    panic(err)
}
defer adp.Close()
```

### Switching order transport mode

`OrderMode` is defined in `exchange.go`, but `SetOrderMode` lives on `BaseAdapter`, not the `Exchange` interface. Only use it when you have a concrete adapter or a type that exposes that method.

Current code contains `IsRESTMode()` branches in these perp adapters:

- `binance/perp_adapter.go`
- `okx/perp_adapter.go`
- `lighter/perp_adapter.go`
- `nado/perp_adapter.go`
- `hyperliquid/perp_adapter.go`
- `standx/perp_adapter.go`
- `grvt/perp_adapter.go`

Do **not** assume every adapter supports both order transports.

```go
adp, err := binance.NewAdapter(ctx, binance.Options{APIKey: key, SecretKey: secret})
if err != nil {
    panic(err)
}
adp.SetOrderMode(exchanges.OrderModeREST)
```

## Interface Boundaries

Use the narrowest interface that matches the task.

### Exchange

Use `Exchange` for:

- market data
- order placement/cancel/query
- account snapshot
- symbol details
- common streams
- local orderbook access

### PerpExchange

Type-assert when you need:

- `FetchPositions`
- `SetLeverage`
- `FetchFundingRate`
- `FetchAllFundingRates`
- `ModifyOrder`

```go
perp, ok := adp.(exchanges.PerpExchange)
if !ok {
    return fmt.Errorf("%s is not a perp adapter", adp.GetExchange())
}
```

### SpotExchange

Type-assert when you need:

- `FetchSpotBalances`
- `TransferAsset`

### Binance margin

There is no shared margin interface in `exchange.go`.

If the task is specifically Binance margin, work with:

- `binance.NewMarginAdapter`
- `binance/margin_adapter.go`
- `binance/sdk/margin/*`

Do not claim there is a root-level `MarginExchange` contract unless one is added to the code.

## Shared Contracts That Drive Correct Implementations

### Symbols

- Adapter layer input is base symbol only: `"BTC"`, `"ETH"`, `"DOGE"`
- Outgoing exchange-specific formatting should go through `FormatSymbol`
- Incoming exchange symbols should be normalized through `ExtractSymbol`

If a bug mentions wrong symbols, inspect:

- `<exchange>/common.go`
- `<exchange>/perp_adapter.go`
- `<exchange>/spot_adapter.go`

### Precision and validation

Shared helpers already exist:

- `RoundToPrecision`
- `FloorToPrecision`
- `CountDecimalPlaces`
- `RoundToSignificantFigures`
- `ValidateAndFormatParams`

Preferred implementation pattern in adapters:

1. Apply slippage if needed
2. Fetch symbol details or use cached symbol detail
3. Validate and format params
4. Convert symbol and order type to exchange format
5. Call REST or WS client
6. Map response back to shared `Order`

### Slippage behavior

`BaseAdapter.ApplySlippage` converts:

- `MARKET` + `Slippage > 0`

into:

- `LIMIT` + IOC-style execution with slippage-adjusted price

Do not re-invent this logic unless the exchange requires a different mechanism.

### Partial-fill behavior

Some exchanges do not natively emit `PARTIALLY_FILLED` the same way. Shared helper:

```go
exchanges.DerivePartialFillStatus(order)
```

Use it when the exchange reports `NEW` but `FilledQuantity > 0`.

### Local orderbook contract

Per-exchange local orderbook implementations must satisfy `LocalOrderBook`:

```go
type LocalOrderBook interface {
    GetDepth(limit int) ([]Level, []Level)
    WaitReady(ctx context.Context, timeout time.Duration) bool
    Timestamp() int64
}
```

The shared adapter path expects:

- `WatchOrderBook` subscribes and synchronizes initial state
- `GetLocalOrderBook` reads the maintained in-memory book
- `StopWatchOrderBook` cleans up the subscription

### Local account state contract

`LocalState` is the unified local state manager that wraps any `Exchange` adapter:

- Call `NewLocalState(adp, logger)` to create
- `Start(ctx)` performs REST snapshot + auto `WatchOrders` + `WatchPositions` + periodic refresh
- `GetOrder`, `GetPosition`, `GetBalance` for zero-latency reads
- `SubscribeOrders()` / `SubscribePositions()` for fan-out event subscriptions (multiple consumers)
- `PlaceOrder` + `OrderResult.WaitTerminal` for integrated order tracking
- `Close()` to release resources

## AI Workflow For This Repository

Future agents should follow this sequence instead of answering from memory.

### If the task is "how do I use exchange X?"

1. Open `<exchange>/options.go`
2. Open `<exchange>/register.go`
3. Open the relevant adapter file
4. Confirm direct constructor name and required auth fields
5. Answer with code based on those files

### If the task is "why is order handling broken?"

1. Open `exchange.go` and `models.go`
2. Open target adapter `PlaceOrder`, `FetchOrder`, `WatchOrders`
3. Open `testsuite/order_suite.go` and `testsuite/helpers.go`
4. Check symbol conversion, status mapping, client order ID handling, and callback timing

### If the task is "add a new exchange adapter"

1. Read `exchange.go`, `utils.go`, `registry.go`, `testsuite/*`
2. Pick the closest existing exchange package as reference
3. Mirror package structure
4. Implement the smallest viable adapter first
5. Add tests by copying the nearest `adapter_test.go`

### If the task is "what does the unified SDK guarantee?"

Start with:

- `exchange.go`
- `models.go`
- `errors.go`
- `local_state.go`
- `event_bus.go`

Not the README.

## Order Flow Recipe

This is the safest default recipe for placing and tracking an order.

```go
func placeAndTrack(ctx context.Context, adp exchanges.Exchange, symbol string, qty decimal.Decimal) error {
    updates := make(chan *exchanges.Order, 100)
    cid := exchanges.GenerateID()

    if err := adp.WatchOrders(ctx, func(o *exchanges.Order) {
        if o.ClientOrderID != cid {
            return
        }
        select {
        case updates <- o:
        default:
        }
    }); err != nil {
        return err
    }

    order, err := adp.PlaceOrder(ctx, &exchanges.OrderParams{
        Symbol:   symbol,
        Side:     exchanges.OrderSideBuy,
        Type:     exchanges.OrderTypeMarket,
        Quantity: qty,
        ClientID: cid,
    })
    if err != nil {
        return err
    }

    timeout := time.After(30 * time.Second)
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-timeout:
            return fmt.Errorf("timeout waiting for order %s", order.OrderID)
        case update := <-updates:
            switch update.Status {
            case exchanges.OrderStatusFilled:
                return nil
            case exchanges.OrderStatusCancelled, exchanges.OrderStatusRejected:
                return fmt.Errorf("terminal order status: %s", update.Status)
            }
        }
    }
}
```

Why this is the default:

- watch comes before place
- `ClientID` provides a stable tracking key
- the code handles terminal states
- it avoids polling loops until necessary

> **Note:** For simpler use, prefer `LocalState.PlaceOrder()` which handles WatchOrders subscription, fan-out, and filtering automatically. The manual recipe above is for cases where you don't want `LocalState` overhead.

## LocalState Recipe

Use `LocalState` when the caller wants a local synchronized view of orders, positions, and balance, with fan-out event subscriptions and integrated order tracking.

```go
func startLocalState(ctx context.Context, adp exchanges.Exchange) (*exchanges.LocalState, error) {
    state := exchanges.NewLocalState(adp, nil)
    if err := state.Start(ctx); err != nil {
        return nil, err
    }
    return state, nil
}
```

Remember:

- it wraps any `Exchange` (not just `PerpExchange`)
- it performs an initial `FetchAccount`
- it subscribes to order and position streams automatically
- `SubscribeOrders()` returns a `*Subscription[Order]` with a `C` channel for fan-out
- `PlaceOrder` returns `*OrderResult` with `WaitTerminal(timeout)` for blocking until terminal state
- `GetAllOpenOrders()` returns `[]Order`
- `GetBalance()` returns the last synced balance
- Always call `state.Close()` when done

## Adding Or Extending An Exchange Adapter

When building or modifying an adapter, follow the existing project shape instead of inventing new abstractions.

### Minimum file set for a new exchange package

Create or mirror:

- `options.go`
- `register.go`
- `common.go`
- `perp_adapter.go`
- `spot_adapter.go` if supported
- `orderbook.go` or `*_orderbook.go` if local books are supported
- `adapter_test.go`
- `sdk/` client files

### Constructor checklist

1. Validate quote currency in `options.go`
2. Provide default quote currency there
3. Register the exchange in `register.go`
4. Support only real market types in the registry switch
5. Keep registry option keys aligned with `opts map[string]string`

### Adapter checklist

In adapter constructors:

1. Embed `*exchanges.BaseAdapter`
2. Set correct exchange name and `MarketType`
3. Load symbol details into cache
4. Initialize clients and WS connections
5. Return the concrete adapter

In order paths:

1. Apply shared slippage helper where appropriate
2. Validate and format params using symbol details
3. Convert symbol with `FormatSymbol`
4. Map exchange order types and time-in-force values explicitly
5. Preserve `ClientID` / `ClientOrderID`
6. Normalize statuses to shared enums
7. Use `DerivePartialFillStatus` if exchange semantics require it

In stream paths:

1. Convert incoming symbols back to base symbols
2. Update local order/position/balance state through shared helpers
3. Avoid blocking the WS read loop with slow user work
4. Expose stop/unsubscribe behavior

For local orderbooks:

1. Implement `LocalOrderBook`
2. Register it with `SetLocalOrderBook`
3. Wait for readiness before returning success from `WatchOrderBook`
4. Remove it on unsubscribe or cleanup

## Testing Contract

The test suite names in this repository are:

- `testsuite.RunAdapterComplianceTests`
- `testsuite.RunOrderSuite`
- `testsuite.RunLifecycleSuite`
- `testsuite.RunLocalStateSuite`

Do not invent names like `RunComplianceSuite`.

### Standard usage

```go
func TestMyExchangeCompliance(t *testing.T) {
    adp := createTestAdapter(t)
    testsuite.RunAdapterComplianceTests(t, adp, "BTC")
}

func TestMyExchangeOrders(t *testing.T) {
    adp := createTestAdapter(t)
    testsuite.RunOrderSuite(t, adp, testsuite.OrderSuiteConfig{
        Symbol: "DOGE",
    })
}

func TestMyExchangeLifecycle(t *testing.T) {
    adp := createTestAdapter(t)
    testsuite.RunLifecycleSuite(t, adp, testsuite.LifecycleConfig{
        Symbol: "DOGE",
    })
}

func TestMyExchange_LocalState(t *testing.T) {
    adp := createTestAdapter(t)
    testsuite.RunLocalStateSuite(t, adp, testsuite.LocalStateConfig{
        Symbol: "DOGE",
    })
}
```

### Test helpers worth reusing

From `testsuite/helpers.go`:

- `SetupOrderWatch`
- `WaitOrderStatus`
- `SmartQuantity`
- `SmartLimitPrice`

These helpers encode the current project assumptions for matching order updates and picking viable test quantities.

### What the tests imply

If an adapter is considered correct, it should satisfy at least:

- `FetchTicker` returns positive price fields
- `FetchOrderBook` returns normalized base-symbol book data
- `WatchOrderBook` produces a synchronized local book
- `FetchSymbolDetails` works for tradable symbols
- `PlaceOrder` integrates with live order updates
- `FetchAccount` is consistent enough for cleanup and lifecycle checks

## Common Mistakes

These are the recurring mistakes to prevent explicitly.

| Mistake | Correct behavior |
|---------|------------------|
| Inventing `NewPerpAdapter` | Use actual constructors from source, usually `NewAdapter` |
| Inventing `RunComplianceSuite` | Use `RunAdapterComplianceTests` |
| Passing pair-form symbols into adapters | Pass base symbols like `"BTC"` |
| Using `float64` in adapter logic | Use `decimal.Decimal` |
| Matching order updates only by `OrderID` | Prefer `ClientOrderID` when you control it |
| Calling `GetLocalOrderBook` before subscribing | Call `WatchOrderBook` first |
| Inferring perp side from quantity sign | Use `Position.Side` |
| Blocking inside WS callbacks | Keep callbacks fast; hand work off if needed |
| Assuming all adapters support spot and perp | Verify in `register.go` and package files |
| Assuming all adapters expose REST and WS order placement | Verify actual adapter code |
| Assuming build tags because a README says so | Verify source files; current package files may not enforce that assumption |
| Treating Binance margin as part of unified interfaces | Use `binance/margin_adapter.go` directly |
| Re-implementing precision logic ad hoc | Reuse `ValidateAndFormatParams`, `RoundToPrecision`, `FloorToPrecision` |
| Ignoring `OrderStatusPartiallyFilled` | Treat it as non-terminal |
| Forgetting `defer adp.Close()` | Always close adapters to release resources |

## Red Flags

If you catch yourself thinking any of these, stop and re-open the source files:

- "I know the constructor name already."
- "This exchange probably uses the same auth fields as the others."
- "The README says X, so I can assume X."
- "I'll just use pair symbols here."
- "There must be a shared margin interface."
- "I can answer from the old skill without reading code."
- "This test helper is probably called RunComplianceSuite."

## The Bottom Line

This skill exists to help future agents use the repository correctly, not to replace the repository.

For this project, reliable behavior comes from following this rule set:

1. Start from code, not memory.
2. Verify constructor names in `register.go` and `options.go`.
3. Verify shared contracts in `exchange.go`, `models.go`, `errors.go`, and `local_state.go`.
4. Let `testsuite/` define expected behavior.
5. Reuse shared helpers instead of writing fresh exchange glue unless necessary.

If you do that, you will avoid almost every costly mistake people make in this codebase.
