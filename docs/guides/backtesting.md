# Backtesting

The backtest runner replays timestamped market events through the same strategy
runtime shape used by live trading. It supports order latency, deterministic
fill IDs, order-list semantics, advanced order triggers, market-data catalogs,
shared cache and portfolio state, and result summaries.

Minimal runner shape:

```go
runner := backtest.NewRunner(backtest.Config{
    Cache:      cache.New(),
    Strategies: []strategy.Strategy{strategy.NewTyped("strategy-id", impl)},
    Events:     events,
})
result, err := runner.Run(ctx)
```

Core behavior is covered by:

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./backtest ./testsuite -run 'TestBacktest|TestNautilusMaster' -v
```

The side-by-side workflow guide includes a Nautilus-style mapping:

- [Side-By-Side Nautilus And Go Examples](./side-by-side-nautilus-go-examples.md)
