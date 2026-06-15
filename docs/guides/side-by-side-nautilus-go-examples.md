# Side-By-Side Nautilus And Go Examples

This guide maps common NautilusTrader workflows to the Go replica. The goal is
not line-for-line API cloning; it is equivalent lifecycle behavior with Go
interfaces, normalized models, and testable state transitions.

## Bracket Strategy

Nautilus shape:

```python
order_list = self.order_factory.bracket(...)
self.submit_order_list(order_list)
```

Go shape:

```go
list := rt.OrderFactory(accountID).Bracket(model.BracketOrderRequest{
    InstrumentID: instrumentID,
    Side:         model.OrderSideBuy,
    Quantity:     qty,
    EntryPrice:   entry,
    TakeProfit:   target,
    StopLoss:     stop,
})
reports, err := rt.SubmitOrderList(ctx, list)
```

Runnable Go package: `examples/nautilus_style`.

## Portfolio Query

Nautilus shape:

```python
position = self.cache.position(instrument_id)
balances = self.cache.account(account_id).balances
```

Go shape:

```go
position, ok := node.Cache().PositionByInstrument(accountID, instrumentID)
exposure := node.Portfolio().Exposure(accountID, "USDT")
realized := node.Portfolio().RealizedPnLs(accountID)
unrealized := node.Portfolio().UnrealizedPnLs(accountID)
```

The Go portfolio keeps realized PnL, unrealized PnL, net positions, net
exposures by account, and venue-level exposure queries over the shared cache.

## Risk Rejection

Nautilus shape:

```python
self.submit_order(order)
# RiskEngine rejects before execution when limits fail.
```

Go shape:

```go
engine := risk.NewEngine(cache.New(), risk.Config{
    MaxOrderNotional: decimal.RequireFromString("1000"),
})
if err := engine.Check(order); err != nil {
    // rejected before reaching the execution client
}
```

The platform node uses the same risk engine before dispatching orders to
execution clients.

## Backtest Run

Nautilus shape:

```python
engine.add_strategy(strategy)
engine.add_data(data)
engine.run()
```

Go shape:

```go
runner := backtest.NewRunner(backtest.Config{
    Cache:      cache.New(),
    Strategies: []strategy.Strategy{strategy.NewTyped("id", impl)},
    Events:     events,
})
result, err := runner.Run(ctx)
```

The Go runner replays timestamped events, drains same-timestamp strategy
commands, applies latency and fill models, updates cache and portfolio, and
returns a result summary.

## Live Node Assembly

Nautilus shape:

```python
node = TradingNode(config=...)
node.start()
node.stop()
```

Go shape:

```go
node, err := live.NewNodeBuilder().
    AddDataClient(dataClient).
    AddExecutionClient(execClient).
    AddStrategy(strategy.NewTyped("id", impl)).
    Build()
if err != nil {
    return err
}
if err := node.Start(ctx); err != nil {
    return err
}
defer node.Stop(context.Background())
```

For a runnable market-data to execution comparison, see
`examples/usage_comparison`, which includes both
`examples/usage_comparison/nautilus_demo.py` and the tested Go demo under
`examples/usage_comparison/go_demo`.
