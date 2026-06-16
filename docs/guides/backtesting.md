# Backtesting

`backtest.Runner` runs strategies against timestamped events with deterministic
time and simulated execution. Use it to prove strategy behavior, order
lifecycle, risk behavior, portfolio accounting, and repeatability before wiring
live clients.

## What The Runner Does

For each event, the runner:

1. advances the runtime clock;
2. dispatches due timers;
3. expires open orders when needed;
4. records market events;
5. matches already-open orders against the latest market state;
6. dispatches the event to strategies;
7. drains same-timestamp commands created by callbacks;
8. matches newly submitted orders when marketable;
9. updates cache, portfolio, execution health, and result metadata.

## Minimal Backtest

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
    Cache: cache.New(),
    Strategies: []strategy.Strategy{
        strategy.NewTyped("research", strategyImpl),
    },
    Events:    events,
    FillModel: backtest.DefaultFillModel(),
})

result, err := runner.Run(ctx)
if err != nil {
    return err
}
orders := result.Cache.Orders("main")
exposure := result.Portfolio.Exposure("main", "USDT")
```

Runnable code: [04_run_strategy_backtest.go](../../examples/04_run_strategy_backtest.go).

## Backtest Inputs

`backtest.Event` is intentionally small:

- `At`: the event timestamp;
- `Topic`: usually `strategy.TopicMarketData`, `strategy.TopicExecution`,
  `strategy.TopicTimer`, or `strategy.TopicError`;
- `Message`: a normalized model value such as `model.MarketEvent`.

For reusable data, use a `data.Catalog` implementation and pass it through
`backtest.Config.DataCatalog`.

## Fill Model And Latency

Use `backtest.DefaultFillModel()` unless a test needs specific fee, slippage,
or fill behavior. `backtest.Config.OrderLatency` delays order eligibility,
which is useful when a strategy should not fill immediately on the same event
that caused the order.

Keep these values explicit in research tests so output comparisons stay stable.

## What To Assert

Avoid tests that only assert profit. Good backtest assertions include:

- number of processed events;
- submitted order count and order statuses;
- command metadata and client IDs;
- fill count and fill quantities;
- final positions;
- exposure and PnL;
- no duplicate fills;
- deterministic result output where used.

## Verification

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./backtest ./testsuite -run 'Backtest|Result|FillModel|Catalog' -v
```

For strategy examples:

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./examples/... -v
```

Bracket order-list behavior is covered by
[05_submit_bracket_order_backtest.go](../../examples/05_submit_bracket_order_backtest.go).
