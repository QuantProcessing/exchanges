# Getting Started

This guide takes you from an empty Go project to the three common ways to use
the repository:

1. call a venue adapter for normalized market data;
2. write a strategy against `strategy.Runtime`;
3. run the same strategy shape in backtest and live node wiring.

The module targets Go 1.26.

## Install

```bash
go get github.com/QuantProcessing/exchanges
```

For local test and example runs, keep the Go build cache outside the
repository:

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./examples/...
```

## Choose The Right Layer

Use the lowest layer that solves your problem:

| Need | Use | Why |
| --- | --- | --- |
| Direct access to a venue endpoint | `sdk/<venue>` | You want the native request and response shape. |
| Normalized venue data or order methods | `adapter/<venue>` plus `venue` interfaces | You want a stable cross-venue surface. |
| Strategy, backtest, live trading, risk, portfolio, and reconciliation | `strategy`, `backtest`, `live`, `platform` | You want lifecycle-correct trading behavior. |

Most applications should not import an SDK from strategy code. Strategies
should use normalized model types and submit commands through the runtime so
risk, execution, cache, and portfolio remain consistent.

## Fetch Market Data Through An Adapter

Adapter methods return normalized `model` values and declare capabilities
through `venue.DeclaredCapabilities`.

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

    adp, err := binance.NewAdapter(ctx, binance.Options{})
    if err != nil {
        panic(err)
    }
    defer adp.Close(ctx)

    instrumentID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
    ticker, err := adp.Data().FetchTicker(ctx, instrumentID)
    if err != nil {
        panic(err)
    }

    fmt.Println(ticker.LastPrice)
}
```

If you need to support several venues, code against the `venue.DataClient` and
`venue.ExecutionClient` interfaces instead of a concrete adapter package.

## Write A Minimal Strategy

Strategies are ordinary Go values. `strategy.NewTyped` dispatches runtime
events into whichever typed methods your value implements.

```go
type ImbalanceStrategy struct {
    runtime      strategy.Runtime
    accountID    model.AccountID
    instrumentID model.InstrumentID
    submitted    bool
}

func (s *ImbalanceStrategy) OnStart(ctx context.Context, rt strategy.Runtime) error {
    s.runtime = rt
    return rt.SubscribeOrderBookDepth(ctx, s.instrumentID, 2)
}

func (s *ImbalanceStrategy) OnOrderBook(ctx context.Context, book model.OrderBook) error {
    if s.submitted || len(book.Bids) == 0 || len(book.Asks) == 0 {
        return nil
    }
    if !book.Bids[0].Size.GreaterThan(book.Asks[0].Size.Mul(decimal.NewFromInt(2))) {
        return nil
    }

    s.submitted = true
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

Important rules:

- Store the runtime passed to `OnStart`; do not create one yourself.
- Use `OrderFactory` so account IDs, client order IDs, list IDs, and command
  metadata are generated consistently.
- Keep venue-specific details outside the strategy.
- Query open orders, fills, positions, balances, and exposure through
  `rt.Cache()` and `rt.Portfolio()`.

## Run The Strategy In A Backtest

Backtests replay timestamped events through the same strategy callback shape.
They are the fastest way to lock down strategy behavior.

```go
events := []backtest.Event{
    {
        At:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
        Topic: strategy.TopicMarketData,
        Message: model.MarketEvent{OrderBook: &model.OrderBook{
            InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
            Bids: []model.OrderBookLevel{{
                Price: decimal.RequireFromString("100"),
                Size:  decimal.RequireFromString("3"),
            }},
            Asks: []model.OrderBookLevel{{
                Price: decimal.RequireFromString("101"),
                Size:  decimal.RequireFromString("1"),
            }},
        }},
    },
}

runner := backtest.NewRunner(backtest.Config{
    Strategies: []strategy.Strategy{
        strategy.NewTyped("imbalance", &ImbalanceStrategy{
            accountID:    "main",
            instrumentID: model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
        }),
    },
    Events: events,
})

result, err := runner.Run(context.Background())
if err != nil {
    panic(err)
}
fmt.Println(result.EventsProcessed)
```

## Assemble A Live Node

A live node wires data clients, execution clients, strategies, risk, cache,
portfolio, bus, reconnect policy, and health reporting into one runtime.

```go
node, err := live.NewNodeBuilder().
    WithCache(cache.New()).
    WithRiskConfig(risk.Config{
        MaxOrderNotional: decimal.RequireFromString("100"),
    }).
    AddDataClient(dataClient).
    AddExecutionClient(executionClient).
    AddStrategy(strategy.NewTyped("imbalance", strategyImpl)).
    Build()
if err != nil {
    return err
}

if err := node.Start(ctx); err != nil {
    return err
}
defer node.Stop(context.Background())
```

The live node startup path loads instruments, connects market data, connects
execution, queries account state, starts strategies, forwards stream events,
and records health. Keep `Node.Stop` in a defer or shutdown handler.

## Next Reading

- [Module Guide](./module-guide.md)
- [Runtime Flow](./runtime-flow.md)
- [Reliable Quant Program Guide](./guides/reliable-quant-program.md)
- [Strategy Authoring](./guides/strategy-authoring.md)
- [Backtesting](./guides/backtesting.md)
- [Live Trading](./guides/live-trading.md)
