---
name: adding-exchange-adapters
description: Use when adding a new exchange package or wiring a new exchange into the QuantProcessing exchanges Go repository, especially when deciding package skeleton, registry behavior, market-type support, and the minimum tests required before considering the adapter complete.
---

# Adding Exchange Adapters

## Overview

Use this skill for brand-new exchange packages. It complements `exchanges`: use `exchanges` for repo-wide interfaces and invariants, then use this skill for the package skeleton, registry wiring, and done criteria of a new adapter.

The common failure modes are predictable:

- missing or incorrect `register.go` market routing
- inventing constructor names instead of matching repo conventions
- forgetting `options.go` quote validation and defaults
- shipping an adapter without `testsuite` coverage
- guessing package layout from README text instead of peer packages

## Pick the Right Template

Choose a peer package by market shape and auth style, then read that package's `options.go`, `register.go`, `adapter_test.go`, and orderbook file before adding code.

| Need | Start from |
|------|------------|
| Perp + spot with API key / secret | `binance`, `okx`, `aster` |
| Perp + spot with private-key auth | `nado`, `lighter`, `hyperliquid` |
| Perp only | `standx`, `grvt`, `edgex` |

Prefer a template with the same market coverage first, then the same auth pattern.

## Minimum File Set

Most new exchange packages should include:

```text
<exchange>/
  options.go
  register.go
  common.go
  perp_adapter.go
  spot_adapter.go        # only if supported
  funding.go             # perp only
  orderbook.go or *_orderbook.go
  adapter_test.go
  sdk/                   # only when this repo owns the low-level client
```

Interpret the files like this:

- `options.go`: auth fields, logger fallback, supported quote currencies, default quote
- `register.go`: registry key, runtime market-type dispatch, options-map key names
- `common.go`: symbol conversion and exchange-specific shared helpers
- `perp_adapter.go` / `spot_adapter.go`: unified interface implementation and constructors
- `funding.go`: `PerpExchange` funding methods
- `orderbook.go` or `*_orderbook.go`: snapshot + delta sync for local books
- `adapter_test.go`: live integration entrypoints into `testsuite`

## Non-Negotiable Wiring

1. `register.go` must call `exchanges.Register("<NAME>", ...)` from `init()`.
2. The registry constructor must switch on `exchanges.MarketType` and reject unsupported markets with a clear error.
3. Direct constructors should match repository conventions: perp is usually `NewAdapter`, spot is usually `NewSpotAdapter`.
4. `options.go` is the source of truth for required auth keys and quote-currency defaults. The same option names must be used in `register.go`.
5. New adapter code must implement the root `exchanges.Exchange` contract before adding exchange-specific conveniences.
6. Perp-only behavior belongs behind `exchanges.PerpExchange`; do not add perp methods to spot adapters.
7. Symbols at the adapter layer stay in base form like `"BTC"`; exchange-specific symbol formatting belongs inside the adapter.

## Source-of-Truth Routing

| Task | Read first | Then inspect |
|------|------------|--------------|
| Confirm unified methods to implement | `exchange.go` | `models.go`, `errors.go`, `utils.go` |
| Mirror registry behavior | `registry.go` | peer `<exchange>/register.go` |
| Define auth fields and quote rules | peer `<exchange>/options.go` | target `register.go` |
| Implement order behavior | `testsuite/order_suite.go` | peer adapter `PlaceOrder`, `FetchOrder`, `WatchOrders` |
| Implement order lifecycle correctness | `testsuite/lifecycle_suite.go` | peer `WatchOrders`, `FetchAccount`, position handling |
| Implement local state / subscriptions | `testsuite/localstate_suite.go` | `local_state.go`, peer orderbook file |
| Decide package skeleton | peer package tree | `testsuite/compliance.go` |

## Definition of Done

Do not consider a new adapter complete until all of these are true:

1. Direct construction works with the package's real constructor names.
2. Registry-based construction works through `LookupConstructor`.
3. Unsupported market types fail explicitly in `register.go`.
4. `Options` validates quote currency and applies the intended default.
5. The adapter passes the relevant `testsuite` entrypoints in `adapter_test.go`.
6. Symbol formatting round-trips correctly through `FormatSymbol` and `ExtractSymbol`.
7. Local orderbook subscription returns a non-`nil` book after `WatchOrderBook`.

## Test Matrix

`adapter_test.go` should wire the shared suites instead of inventing custom coverage first.

| Adapter shape | Minimum suites |
|---------------|----------------|
| Any adapter | `RunAdapterComplianceTests` |
| Trading-capable adapter | `RunOrderSuite` |
| Adapter with order-stream correctness expectations | `RunLifecycleSuite` |
| Adapter intended to support unified local state | `RunLocalStateSuite` |

Use peer exchange tests to choose symbols and skip flags. Spot adapters may need `SkipSlippage`; perp-only exchanges should not expose spot tests at all.

## Common Mistakes

- Copying README examples instead of peer package code when choosing constructors or option keys.
- Returning support for a market in docs or registry while omitting `spot_adapter.go` or `perp_adapter.go`.
- Using exchange-native symbols like `"BTCUSDT"` at the unified adapter layer.
- Skipping `WatchOrders`-driven validation and assuming REST-only order state is enough.
- Adding a package without `adapter_test.go`, which leaves `testsuite` contracts unexercised.
- Forgetting that `GetLocalOrderBook` only makes sense after `WatchOrderBook`.

## Fast Start

When adding a new exchange package, read files in this order:

1. `exchange.go`
2. `registry.go`
3. one peer package: `options.go`, `register.go`, `adapter_test.go`
4. `testsuite/compliance.go`
5. `testsuite/order_suite.go`
6. `testsuite/lifecycle_suite.go`
7. `testsuite/localstate_suite.go`

Then create the new package by copying the closest peer shape and changing behavior one surface at a time.
