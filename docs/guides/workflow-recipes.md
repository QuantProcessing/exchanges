# Workflow Recipes

This guide shows common trading workflows using the repository's Go APIs. Each
recipe is intentionally small and focuses on the shape of the platform path.

## Bracket Strategy

Create one entry order plus take-profit and stop-loss children:

```go
list := rt.OrderFactory(accountID).Bracket(model.BracketOrderRequest{
    InstrumentID: instrumentID,
    Side:         model.OrderSideBuy,
    Quantity:     decimal.RequireFromString("1"),
    EntryPrice:   decimal.RequireFromString("100"),
    TakeProfit:   decimal.RequireFromString("104"),
    StopLoss:     decimal.RequireFromString("98"),
})
reports, err := rt.SubmitOrderList(ctx, list)
```

Expected behavior:

- parent entry order is submitted first;
- children remain held until the parent fills;
- one exit child filling cancels the sibling;
- final position and portfolio state are updated from fill events.

## Portfolio Query

Read lifecycle state from cache and accounting state from portfolio:

```go
position, ok := node.Cache().PositionByInstrument(accountID, instrumentID)
exposure := node.Portfolio().Exposure(accountID, "USDT")
realized := node.Portfolio().RealizedPnLs(accountID)
unrealized := node.Portfolio().UnrealizedPnLs(accountID)
```

Do not recompute exposure in a strategy unless the calculation is deliberately
strategy-local and disposable.

## Risk Rejection

Configure limits outside the strategy:

```go
engine := risk.NewEngine(cache.New(), risk.Config{
    MaxOrderNotional: decimal.RequireFromString("1000"),
})
if err := engine.Check(order); err != nil {
    // rejected before reaching the execution client
}
```

In normal live paths, `platform.Node.SubmitOrder` performs this check before
dispatching to the execution client.

## Backtest Run

```go
runner := backtest.NewRunner(backtest.Config{
    Cache: cache.New(),
    Strategies: []strategy.Strategy{
        strategy.NewTyped("id", strategyImpl),
    },
    Events: events,
})
result, err := runner.Run(ctx)
```

The runner replays timestamped events, dispatches timers, drains commands,
matches orders, updates cache and portfolio, and returns a result summary.

## Live Node Assembly

```go
node, err := live.NewNodeBuilder().
    AddDataClient(dataClient).
    AddExecutionClient(execClient).
    AddStrategy(strategy.NewTyped("id", strategyImpl)).
    Build()
if err != nil {
    return err
}
if err := node.Start(ctx); err != nil {
    return err
}
defer node.Stop(context.Background())
```

Use capability checks before depending on optional execution features.
