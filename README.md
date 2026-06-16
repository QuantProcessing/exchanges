# exchanges

Go exchange SDKs, capability-honest adapters, and a trading runtime inspired by
NautilusTrader's strategy, execution, risk, portfolio, backtest, and live-node
workflow style.

This repository has two connected responsibilities:

- provide venue-native SDKs and normalized crypto exchange adapters;
- provide a Go trading runtime where one strategy can use the same
  `strategy.Runtime` shape in deterministic backtests and live node wiring.

The implementation is clean-room and idiomatic Go. NautilusTrader is used as
the behavioral style reference in this README only; the docs site describes this
project on its own terms.

## Start Here

If you are new to the repository, read these in order:

1. [Getting Started](./docs/getting-started.md): install the module, fetch data,
   write a strategy, run a backtest, and assemble a live node.
2. [Module Guide](./docs/module-guide.md): what every package does and when to
   import it.
3. [Runtime Flow](./docs/runtime-flow.md): how data, orders, fills, risk,
   portfolio, and reconciliation move through the system.
4. [Reliable Quant Program Guide](./docs/guides/reliable-quant-program.md):
   practical rules for writing safer trading programs.
5. [Examples](./examples/README.md): runnable API recipes from simple adapter
   market data to an in-memory live node.
6. [Documentation Index](./docs/README.md): the full docs site map.

## Current Status

The repository already contains first-pass implementations across SDKs,
adapters, model, cache, data, execution, account, risk, portfolio, strategy,
backtest, live, platform, and reusable tests. Many surfaces are still marked
`Partial` or `Planned` in the feature and capability matrices.

Do not treat a venue or runtime feature as production-complete only because a
package exists. Capability truth comes from:

- [Complete Feature Matrix](./docs/parity/complete-feature-matrix.md)
- [Adapter Capability Matrix](./docs/parity/adapter-capability-matrix.md)
- [Master Scorecard](./docs/guides/master-scorecard.md)
- [Complete Quality Gate](./docs/parity/complete-quality-gate.json)

The long-running project target is measured through a 1000 point
NautilusTrader-style parity scorecard implemented in `testsuite`. Required
release status means all mandatory cases pass, every claimed adapter capability
has evidence, and unsupported lifecycle behavior is explicit.

## Architecture At A Glance

```text
strategy code
        |
        v
strategy.Runtime
        |
        +--> backtest.Runner
        |
        +--> live.Node
                 |
                 v
             platform.Node
                 |
                 +--> data.Engine -----> venue.DataClient -----> adapter/* -----> sdk/*
                 +--> execution.Engine -> venue.ExecutionClient -> adapter/* -----> sdk/*
                 +--> account.Reconciler
                 +--> risk.Engine
                 +--> portfolio.Portfolio
                 +--> cache.Cache
                 +--> bus.Bus
```

Layer boundaries:

- `sdk/`: venue-native protocol clients.
- `adapter/` and `venue/`: normalized cross-venue surfaces and declared
  capabilities.
- runtime packages: strategy, data, execution, account, risk, portfolio, cache,
  backtest, live, platform, bus, and kernel behavior.

Read [Project Architecture](./docs/architecture.md) and
[Module Guide](./docs/module-guide.md) for the detailed version.

## Supported Venue Families

The repository currently has SDK and adapter coverage for Binance, Aster, OKX,
Bybit, Bitget, Hyperliquid, Lighter, Nado, EdgeX, GRVT, StandX, and Backpack.

Capability truth is not this overview table; it is the adapter's
`venue.DeclaredCapabilities` plus the
[Adapter Capability Matrix](./docs/parity/adapter-capability-matrix.md).
Perpetual funding-rate snapshot support is tracked there per product family:
spot adapters do not claim it, and `FundingRateStream` is still a separate
unclaimed capability.

## Installation

```bash
go get github.com/QuantProcessing/exchanges
```

The module currently targets Go 1.26.

## Quick Example: Adapter Market Data

```go
package main

import (
    "context"
    "fmt"

    "github.com/QuantProcessing/exchanges/adapter/binance"
    "github.com/QuantProcessing/exchanges/model"
)

func main() {
    ctx := context.Background()

    adp, err := binance.NewSpotAdapter(ctx, binance.Options{})
    if err != nil {
        panic(err)
    }
    defer adp.Close(ctx)

    ticker, err := adp.Data().FetchTicker(ctx, model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"))
    if err != nil {
        panic(err)
    }

    fmt.Println(ticker.Last)
}
```

The compiled version of this pattern lives in
[01_fetch_ticker_with_adapter.go](./examples/01_fetch_ticker_with_adapter.go).

## Quick Example: Strategy Shape

```go
type Strategy struct {
    runtime      strategy.Runtime
    accountID    model.AccountID
    instrumentID model.InstrumentID
}

func (s *Strategy) OnStart(ctx context.Context, rt strategy.Runtime) error {
    s.runtime = rt
    return rt.SubscribeOrderBookDepth(ctx, s.instrumentID, 2)
}

func (s *Strategy) OnOrderBook(ctx context.Context, book model.OrderBook) error {
    if len(book.Asks) == 0 {
        return nil
    }
    order := s.runtime.OrderFactory(s.accountID).Limit(
        book.InstrumentID,
        model.OrderSideBuy,
        decimal.RequireFromString("0.01"),
        book.Asks[0].Price,
    )
    _, err := s.runtime.SubmitOrder(ctx, order)
    return err
}
```

Continue with [Strategy Authoring](./docs/guides/strategy-authoring.md),
[Backtesting](./docs/guides/backtesting.md), and
[Live Trading](./docs/guides/live-trading.md). The matching runnable code is
organized in [examples](./examples/README.md).

## NautilusTrader-Style Contract

The target behavior is:

> A strategy written once against the Go `strategy.Runtime` can run unchanged in
> backtest and live modes; submit bracket, list, and advanced orders; receive
> typed order, fill, position, account, and data callbacks; survive private
> stream disconnects and startup gaps; reconcile missing fills, orders, and
> positions without duplicate state; enforce risk before execution; compute
> portfolio state and PnL consistently; and pass a reusable NautilusTrader-style
> scorecard for every claimed adapter capability.

Golden scenarios:

| ID | Scenario | Required result |
| --- | --- | --- |
| A | Bracket strategy round trip | entry fills, contingent children release, sibling cancel, flat final position, correct PnL |
| B | Reconnect with missing fill | stream health changes, gap query repairs missing fill once, state converges |
| C | Position discrepancy | stale local position is detected, retried or visibly blocked, never silently accepted |
| D | Risk and portfolio safety | risk rejects before adapter submission and prevents downstream mutation |
| E | Adapter capability honesty | every true capability is test-backed; unsupported surfaces return `ErrNotSupported` |

## Verification

Use the smallest gate that proves the claim being made.

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./examples/...
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./strategy ./backtest ./live ./platform
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./venue ./testsuite ./adapter/... ./config/all -run 'Adapter|Capability|Contract|PrivateStream|Resubscribe'
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./testsuite -run 'TestNautilusMaster'
git diff --check
```

Full local gates:

```bash
bash scripts/verify_nautilus_parity.sh
bash scripts/generate_nautilus_benchmark_report.sh
```

## Live Test Policy

Public SDK read tests may call real public exchange endpoints. Private read
tests require credentials and should skip when credentials are absent. Live
write tests must never execute by default; they require an exchange-specific
enable flag plus credentials and should use `internal/testenv.RequireLiveWrite`.

Unsupported behavior must return `model.ErrNotSupported` or a wrapped
equivalent that works with `errors.Is`.

## Related Docs

- [SDK README](./sdk/README.md)
- [Examples](./examples/README.md)
- [Documentation Index](./docs/README.md)
- [Project Architecture](./docs/architecture.md)
- [Module Guide](./docs/module-guide.md)
- [Quant Developer Use Cases](./docs/guides/quant-use-cases.md)
- [Adapter Capabilities](./docs/guides/adapter-capabilities.md)
- [Platform Completion Plan](./docs/plans/platform-completion-plan.md)
