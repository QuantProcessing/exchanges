# exchanges

Go trading platform and exchange SDK, rebuilt toward a measurable
NautilusTrader-style runtime.

This repository has two connected responsibilities:

- Provide venue-native SDKs and stable exchange adapters for crypto venues.
- Provide a Go trading runtime whose lifecycle, risk, portfolio, strategy,
  backtest, live-node, and reconciliation behavior can be scored against the
  local NautilusTrader reference.

The target is a complete, test-backed Go replica of NautilusTrader's core
trading workflows, implemented idiomatically in Go and backed by this
repository's SDK and adapter layers.

## Status

The project tracks completion through a 1000 point Nautilus parity scorecard.
Mandatory release status requires all required scorecard cases to pass, every
claimed adapter capability to be backed by tests, and no silent success for
unsupported lifecycle behavior.

Primary target artifacts:

- [Complete replica plan](./docs/plans/nautilustrader-complete-replica.md)
- [Project architecture](./docs/architecture.md)
- [Master parity scorecard guide](./docs/guides/master-parity-scorecard.md)
- [Complete feature matrix](./docs/parity/nautilustrader-complete-feature-matrix.md)
- [Adapter capability matrix](./docs/parity/adapter-capability-matrix.md)
- [Quality gate definition](./docs/parity/nautilus-complete-quality-gate.json)
- [Release notes template](./docs/parity/nautilus-release-notes-template.md)

Runnable gates:

```bash
bash scripts/verify_nautilus_parity.sh
bash scripts/generate_nautilus_benchmark_report.sh
```

For local Go command runs, prefer a writable cache outside the repository:

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./testsuite -run 'TestNautilusMaster'
```

## Architecture

The repository is organized around three boundaries.

### SDK layer

The exchange-local `sdk/` packages are venue-native protocol clients. They
track official REST and WebSocket APIs as closely as practical and may expose
venue-specific endpoints, request fields, response shapes, signing rules, and
product concepts.

Use this layer when you need direct access to an official exchange API.

### Adapter layer

The `adapter/` and `venue/` packages expose stable cross-exchange convenience
interfaces for common trading workflows.

Adapters own:

- instrument and market resolution;
- exchange-native to unified model mapping;
- order validation and common error mapping;
- REST and WebSocket convenience methods;
- honest `venue.DeclaredCapabilities` reporting.

Adapters do not mirror every SDK endpoint. New capability families should use
small optional interfaces before becoming core cross-exchange contracts.

### Runtime layer

The Nautilus-style runtime is built above SDKs and adapters:

| Package | Responsibility |
| --- | --- |
| `model` | identifiers, instruments, commands, orders, reports, events, data types |
| `cache` | authoritative runtime state and indexes |
| `kernel` | component lifecycle, clocks, health, message bus primitives |
| `bus` | event dispatch and fanout |
| `data` | data catalog, historical requests, live subscriptions, aggregation |
| `execution` | command routing, matching, emulation, lifecycle, reconciliation |
| `account` | trading-account readiness, order tracking, stream reconciliation |
| `risk` | pre-execution checks, limits, kill switch, reduce-only, throttling |
| `portfolio` | balances, positions, commissions, exposure, realized and unrealized PnL |
| `strategy` | strategy runtime, typed callbacks, order factory, timers |
| `backtest` | deterministic venue loop, matching, fees, slippage, latency |
| `live` | live node wiring, retry, reconnect, shutdown, health |
| `platform` | high-level node facade across data, execution, risk, and portfolio |
| `testsuite` | reusable parity, adapter, runtime, benchmark, and release gates |

## Nautilus Parity Contract

NautilusTrader is the behavioral reference contract. The Go implementation is
clean-room and idiomatic, but the workflows are expected to match the reference
semantics for the supported scope.

The core acceptance statement is:

> A strategy written once against the Go `strategy.Runtime` can run unchanged in
> backtest and live modes; submit bracket, list, and advanced orders; receive
> typed order, fill, position, account, and data callbacks; survive private
> stream disconnects and startup gaps; reconcile missing fills, orders, and
> positions without duplicate state; enforce risk before execution; compute
> portfolio state and PnL consistently; and pass a reusable Nautilus parity
> scorecard for every claimed adapter capability.

The master scorecard is defined in `testsuite.NautilusMasterRequirements()` and
is exercised by `scripts/verify_nautilus_parity.sh`.

Golden scenarios:

| ID | Scenario | Required result |
| --- | --- | --- |
| A | Bracket strategy round trip | entry fills, contingent children release, sibling cancel, flat final position, correct PnL |
| B | Reconnect with missing fill | stream health changes, gap query repairs missing fill once, state converges |
| C | Position discrepancy | stale local position is detected, retried or visibly blocked, never silently accepted |
| D | Risk and portfolio safety | risk rejects before adapter submission and prevents downstream mutation |
| E | Adapter capability honesty | every true capability is test-backed; unsupported surfaces return `ErrNotSupported` |

## Supported Venues

The table below describes repository coverage. Capability truth is the
adapter's `venue.DeclaredCapabilities` plus the adapter matrix, not this table.

| Venue | Perp | Spot | Margin | Common quote currencies |
| --- | --- | --- | --- | --- |
| Binance | yes | yes | yes | USDT, USDC |
| OKX | yes | yes | no | USDT, USDC |
| Aster | yes | yes | no | USDT, USDC |
| Nado | yes | yes | no | USDT |
| Lighter | yes | yes | no | USDC |
| Hyperliquid | yes | yes | no | USDC |
| Backpack | yes | yes | no | USDC |
| Bitget | yes | yes | no | USDT, USDC |
| Bybit | yes | yes | no | USDT, USDC |
| StandX | yes | no | no | DUSD |
| GRVT | yes | no | no | USDT |
| EdgeX | yes | no | no | USDC |

Useful references:

- [Adapter capabilities](./docs/guides/adapter-capabilities.md)
- [Adapter capability policy](./docs/guides/adapter-capability-policy.md)
- [Adapter live test policy](./docs/parity/adapter-live-test-policy.md)

## Installation

```bash
go get github.com/QuantProcessing/exchanges
```

The module currently targets Go 1.26.

## Usage

### Adapter-level market data

```go
package main

import (
    "context"
    "fmt"

    "github.com/QuantProcessing/exchanges/adapter/binance"
)

func main() {
    ctx := context.Background()

    adp, err := binance.NewAdapter(ctx, binance.Options{})
    if err != nil {
        panic(err)
    }
    defer adp.Close()

    ticker, err := adp.FetchTicker(ctx, "BTC/USDT")
    if err != nil {
        panic(err)
    }

    fmt.Println(ticker.LastPrice)
}
```

### Config-driven adapter bootstrap

```yaml
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
      secret_key: ${OKX_API_SECRET}
      passphrase: ${OKX_API_PASSPHRASE}
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

    ticker, err := adp.FetchTicker(ctx, "BTC/USDT")
    if err != nil {
        panic(err)
    }

    fmt.Println(ticker.LastPrice)
}
```

### Nautilus-style runtime examples

- [Nautilus-style bracket strategy](./examples/nautilus_style)
- [Go vs Nautilus usage comparison](./examples/usage_comparison)
- [Quant developer use cases](./docs/guides/quant-use-cases.md)
- [Strategy authoring guide](./docs/guides/strategy-authoring.md)
- [Backtesting guide](./docs/guides/backtesting.md)
- [Live trading guide](./docs/guides/live-trading.md)
- [Reconciliation guide](./docs/guides/reconciliation.md)
- [Stream health guide](./docs/guides/stream-health.md)

## Verification

Use the smallest gate that proves the claim being made.

Targeted master scorecard:

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./testsuite -run 'TestNautilusMaster'
```

Full Nautilus parity gate:

```bash
bash scripts/verify_nautilus_parity.sh
```

Benchmark and adapter-contract report:

```bash
bash scripts/generate_nautilus_benchmark_report.sh
```

Example-level smoke tests:

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./examples/...
```

## Live Test Policy

Public SDK read tests may call real public exchange endpoints. Private read
tests require credentials and should skip when credentials are absent. Live
write tests must never execute by default; they require an exchange-specific
enable flag plus credentials and should use `internal/testenv.RequireLiveWrite`.

Unsupported behavior must return `model.ErrNotSupported` or a wrapped equivalent
that works with `errors.Is`.

## Development Rules

- Keep SDK, adapter, and runtime boundaries explicit.
- Prefer quote-aware or instrument-aware routing over base-symbol-only APIs.
- Do not claim adapter lifecycle readiness without private stream and
  reconciliation evidence.
- Add tests before changing lifecycle, risk, portfolio, reconciliation, or
  adapter capability behavior.
- Update the parity scorecard, capability matrix, and runnable gates whenever a
  capability claim changes.

## Related Docs

- [SDK README](./sdk/README.md)
- [Project architecture](./docs/architecture.md)
- [Quant developer use cases](./docs/guides/quant-use-cases.md)
- [Adapter capabilities](./docs/guides/adapter-capabilities.md)
- [Complete feature matrix](./docs/parity/nautilustrader-complete-feature-matrix.md)
- [Adapter capability matrix](./docs/parity/adapter-capability-matrix.md)
- [Side-by-side Nautilus and Go examples](./docs/guides/side-by-side-nautilus-go-examples.md)
