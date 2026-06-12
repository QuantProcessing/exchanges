# Layered Public API Restructure Design

## Status

Approved direction: breaking change. The repository has not published a
stable, production-ready public version, so this redesign optimizes for the
right long-term Go API shape instead of preserving the current root-level
exchange import paths.

## Problem

The repository's product promise is that users can choose the layer that fits
their use case:

1. use a venue-native SDK directly
2. use a normalized cross-exchange adapter
3. use TradingAccount for lifecycle state, streams, and order flow

The current directory layout does not express that choice. Exchange packages
sit at the repository root, and each exchange package contains both adapter
code and an exchange-local `sdk/` subtree. This makes the repository look like
"many exchange packages" instead of a layered library with three public entry
points.

The maintenance pain is a symptom of that mismatch:

- root-level exchange packages compete visually with root public contracts
- SDK users must import through an adapter-shaped path such as
  `github.com/QuantProcessing/exchanges/binance/sdk/perp`
- adapter users and SDK users enter through the same exchange directory even
  though they want different abstractions
- SDK packages currently import the root package for shared errors and
  `SDKRequestOpts`, which couples native SDK code to unified adapter models
- cross-exchange SDK reuse exists through another exchange's SDK path, for
  example Aster importing Binance SDK common helpers

## Design Goal

Make the Go import path communicate the user's chosen abstraction layer.

The target user-facing paths are:

```go
// Shared normalized contracts and models.
import exchanges "github.com/QuantProcessing/exchanges"

// Venue-native SDK access.
import binanceperp "github.com/QuantProcessing/exchanges/sdk/binance/perp"

// Normalized adapter access.
import binance "github.com/QuantProcessing/exchanges/adapter/binance"

// Higher-level lifecycle runtime.
import "github.com/QuantProcessing/exchanges/account"
```

This is the governing rule: package path is product interface.

## Target Repository Layout

```text
/
  exchange.go
  models.go
  option_models.go
  market_ref.go
  errors.go
  registry.go
  capabilities.go
  base_adapter.go
  local_orderbook.go
  manager.go
  utils.go
  log.go
  doc.go

  sdk/
    types.go
    errors.go
    binance/
      common/
      margin/
      option/
      perp/
      portfolio/
      spot/
      subaccount/
    okx/
    bybit/
    bitget/
    aster/
      common/
      news/
      perp/
      spot/
    backpack/
    edgex/
      perp/
      starkcurve/
    grvt/
    hyperliquid/
      perp/
      spot/
    lighter/
      common/
    nado/
    standx/

  adapter/
    aster/
    backpack/
    binance/
    bitget/
    bybit/
    edgex/
    grvt/
    hyperliquid/
    lighter/
    nado/
    okx/
    standx/

  account/
  config/
  internal/
  testsuite/
  docs/
  scripts/
```

The module path remains:

```text
github.com/QuantProcessing/exchanges
```

There is no `/v2` suffix in this restructure because the repository has not
published a stable release that requires semantic import versioning.

## Package Responsibilities

### Root package: `github.com/QuantProcessing/exchanges`

The root package is the normalized public contract. It owns:

- unified interfaces such as `Exchange`, `PerpExchange`, `SpotExchange`, and
  optional capability interfaces
- unified models such as `Order`, `Fill`, `Ticker`, `Position`, `Account`,
  `MarketRef`, and option models
- unified sentinel errors used by adapter and account users
- registry and capability registry contracts
- helper functions that operate only on root contracts

The root package must not import:

- `github.com/QuantProcessing/exchanges/sdk`
- `github.com/QuantProcessing/exchanges/sdk/...`
- `github.com/QuantProcessing/exchanges/adapter/...`
- `github.com/QuantProcessing/exchanges/account`

The root package must not contain concrete exchange implementation files or
exchange-specific constructors.

### SDK layer: `github.com/QuantProcessing/exchanges/sdk/...`

The SDK layer is the venue-native protocol layer. It owns:

- exchange-native REST and WebSocket clients
- exchange-native request and response structs
- signing, auth token, endpoint, and websocket method logic
- SDK-local request options and SDK-local protocol errors

SDK packages may import:

- `github.com/QuantProcessing/exchanges/sdk` for SDK-wide primitives
- `github.com/QuantProcessing/exchanges/internal/...` for private shared
  transport, test, signing, and websocket helpers
- external dependencies

SDK packages must not import:

- `github.com/QuantProcessing/exchanges`
- `github.com/QuantProcessing/exchanges/adapter/...`
- `github.com/QuantProcessing/exchanges/account`
- another exchange's SDK package

The current root `SDKRequestOpts` moves to `sdk.RequestOpts`. SDK-level
protocol errors move to `sdk` as well. Root unified errors and SDK errors may
share private sentinels through `internal/errs`, but SDK users import `sdk`,
not the root package, when they want SDK-specific error helpers.

### Adapter layer: `github.com/QuantProcessing/exchanges/adapter/...`

The adapter layer is the normalized convenience layer. It owns:

- mapping exchange-native SDK responses into root `exchanges.*` models
- symbol and instrument resolution
- order validation and adapter policy
- stable normalized market data, order, account snapshot, and stream methods
- adapter capability registration

Adapter packages may import:

- `github.com/QuantProcessing/exchanges`
- their own SDK subtree, for example
  `github.com/QuantProcessing/exchanges/sdk/binance/perp`
- `github.com/QuantProcessing/exchanges/internal/...`
- external dependencies

Adapter packages must not import another exchange's SDK package. Shared code
that is truly exchange-neutral moves to `internal/...`, not to another
exchange's `sdk/common`.

### Account layer: `github.com/QuantProcessing/exchanges/account`

The account layer is the lifecycle runtime. It owns:

- `PerpTradingAccount`
- `SpotTradingAccount`
- `OptionTradingAccount`
- `PortfolioAccount`
- `OrderFlow`
- stream health and local runtime fan-out

The account package may import the root package. It must not import SDK or
adapter packages. TradingAccount readiness is defined only by lifecycle-critical
root interfaces: account snapshot, order placement and query, `WatchOrders`,
optional `WatchFills`, and market-specific balance or position streams.

### Config layer

`config` loads configuration and builds adapters through the root registry.

`config/all` is the only package that blank-imports every built-in adapter:

```go
package all

import (
	_ "github.com/QuantProcessing/exchanges/adapter/aster"
	_ "github.com/QuantProcessing/exchanges/adapter/backpack"
	_ "github.com/QuantProcessing/exchanges/adapter/binance"
	_ "github.com/QuantProcessing/exchanges/adapter/bitget"
	_ "github.com/QuantProcessing/exchanges/adapter/bybit"
	_ "github.com/QuantProcessing/exchanges/adapter/edgex"
	_ "github.com/QuantProcessing/exchanges/adapter/grvt"
	_ "github.com/QuantProcessing/exchanges/adapter/hyperliquid"
	_ "github.com/QuantProcessing/exchanges/adapter/lighter"
	_ "github.com/QuantProcessing/exchanges/adapter/nado"
	_ "github.com/QuantProcessing/exchanges/adapter/okx"
	_ "github.com/QuantProcessing/exchanges/adapter/standx"
)
```

## Dependency Graph

```text
external applications
  -> exchanges
  -> exchanges/sdk/...
  -> exchanges/adapter/...
  -> exchanges/account

adapter/<exchange>
  -> exchanges
  -> sdk/<exchange>
  -> internal

account
  -> exchanges

sdk/<exchange>
  -> sdk
  -> internal

config
  -> exchanges

config/all
  -> adapter/*
```

Forbidden edges:

```text
exchanges -> sdk
exchanges -> adapter
exchanges -> account
sdk/* -> exchanges
sdk/* -> adapter/*
sdk/* -> account
sdk/<exchange-a> -> sdk/<exchange-b>
account -> sdk/*
account -> adapter/*
```

## Go Best-Practice Rationale

### Keep the root package as the stable contract

Go users expect the module root to be the most stable package. Creating a
separate `core` package would add ceremony without adding clarity. The root
package should therefore remain the normalized contract surface.

### Use package paths to express abstraction level

`sdk/binance/perp` and `adapter/binance` communicate different user choices.
Putting SDK under `adapter/binance/sdk` would incorrectly present SDK access as
an implementation detail of adapters.

### Avoid broad `common` packages

Exchange-neutral shared helpers belong in focused internal packages, for
example `internal/wsdispatch`, `internal/testenv`, or `internal/mbx`. Exchange
SDK packages should not import `sdk/binance/common` just to reuse generic
websocket helpers.

### Preserve package-local names

The final import aliases stay natural:

```go
binance "github.com/QuantProcessing/exchanges/adapter/binance"
binanceperp "github.com/QuantProcessing/exchanges/sdk/binance/perp"
```

Inside `adapter/binance`, the package name remains `binance`. Inside
`sdk/binance/perp`, the package name remains `perp`.

## Breaking Changes

The following import paths are removed:

```text
github.com/QuantProcessing/exchanges/binance
github.com/QuantProcessing/exchanges/binance/sdk/...
github.com/QuantProcessing/exchanges/okx
github.com/QuantProcessing/exchanges/okx/sdk
github.com/QuantProcessing/exchanges/bybit
github.com/QuantProcessing/exchanges/bybit/sdk
```

The replacement pattern is:

```text
github.com/QuantProcessing/exchanges/<exchange>
  -> github.com/QuantProcessing/exchanges/adapter/<exchange>

github.com/QuantProcessing/exchanges/<exchange>/sdk[/...]
  -> github.com/QuantProcessing/exchanges/sdk/<exchange>[/...]
```

Because this is a deliberate pre-stable breaking change, the repository does
not keep root-level compatibility wrappers.

## Adapter Package File Standard

Each `adapter/<exchange>` package should use this default file shape:

```text
options.go
register.go
common.go
perp_adapter.go
spot_adapter.go       # when supported
option_adapter.go     # when supported
market_data_service.go
order_service.go
orderbook.go
private_streams.go
funding.go            # when perp analytics exist
```

The file count may differ when a venue has real constraints, but every
deviation must preserve the same responsibility split:

- `options.go`: options, defaults, quote validation, logger handling
- `register.go`: registry and capability registration only
- adapter files: public constructor and normalized method façade
- service files: focused normalized behavior
- mapper/helper files: SDK-native to root model translation
- stream files: private/public websocket orchestration

## SDK Package Standard

Each `sdk/<exchange>` package should mirror the venue's official API
structure. Product-specific subpackages are allowed when official APIs diverge:

```text
sdk/binance/spot
sdk/binance/perp
sdk/binance/margin
sdk/binance/option
sdk/edgex/perp
sdk/hyperliquid/perp
sdk/hyperliquid/spot
```

Flat SDKs remain flat when the exchange has one compact API surface:

```text
sdk/backpack
sdk/bitget
sdk/bybit
sdk/grvt
sdk/nado
sdk/okx
sdk/standx
```

SDK tests remain beside the SDK files they test.

## Architecture Tests

The restructure must add architecture tests that enforce the dependency graph
instead of relying on documentation alone:

- root package does not import `sdk`, `adapter`, or `account`
- SDK packages do not import root, adapter, account, or another exchange SDK
- adapter packages import only root, internal, external packages, and their own
  SDK subtree
- account imports root but not SDK or adapter
- `config/all` imports canonical `adapter/*` packages
- no root-level exchange implementation directories remain
- every adapter package has `options.go` and `register.go`

## Verification Strategy

The official verification path after the restructure is:

```bash
go test -short .
go test -short ./sdk/...
go test -short ./adapter/...
go test -short ./account
go test -short ./config/...
go test -short ./testsuite
go test -short ./...
```

Exchange-focused verification becomes:

```bash
go test -short ./sdk/binance/...
go test -short ./adapter/binance/...
```

The existing full/live verification model remains valid after path updates:

```bash
scripts/verify_full.sh
RUN_SOAK=1 scripts/verify_soak.sh
```

## Acceptance Criteria

- root-level exchange directories are gone
- `sdk/<exchange>` is the only SDK entry path
- `adapter/<exchange>` is the only adapter entry path
- root package has no dependency on SDK, adapter, or account
- SDK packages no longer import root
- account package does not import SDK or adapter
- adapter packages do not import another exchange's SDK
- `config/all` blank-imports `adapter/*`
- README examples show all three intended user choices
- `go test -short ./...` passes in the normal quick-gate environment
