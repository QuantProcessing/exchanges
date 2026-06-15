# Live Node Configuration

The live runtime is assembled with `live.NodeConfig` or `live.NodeBuilder`.
Both paths create the same underlying stack: bus, cache, risk engine,
portfolio, platform node, data clients, execution clients, and strategy engine.

## Builder Shape

```go
node, err := live.NewNodeBuilder().
    WithCache(cache.New()).
    WithRiskConfig(risk.Config{ /* limits */ }).
    WithPortfolio(portfolio.New(nil)).
    WithReconnectPolicy(live.RetryPolicy{ /* reconnect policy */ }).
    AddDataClient(dataClient).
    AddExecutionClient(execClient).
    AddStrategy(strategy.NewTyped("strategy-id", impl)).
    Build()
if err != nil {
    return err
}
```

`Build` validates nil data clients, execution clients, and strategies. Missing
shared components are defaulted safely: a bus, cache, risk engine, and
portfolio are created when they are not supplied.

## Startup Order

`Node.Start(ctx)` delegates to `Runner.Start(ctx)`:

1. the platform node starts data and execution clients;
2. the strategy engine starts with the platform node as `strategy.Runtime`;
3. strategy states move to running;
4. a health monitor watches strategy errors and platform health.

If startup fails, the runner faults and stops the platform path that already
started.

## Shutdown Semantics

Call `Node.Stop(ctx)` for explicit shutdown. The runner cancels the monitor,
stops strategies first, stops the platform node second, and records stopped or
faulted health state. Fatal strategy errors or platform health failures request
the same graceful shutdown path automatically.

The shutdown contract is intentionally strict: strategies stop before exchange
clients are disconnected, so `OnStop` callbacks can still reference runtime
state and unsubscribe through the platform.

## Verification

Live-node behavior is covered by:

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./live ./platform ./testsuite -run 'Live|Node|Runner|NautilusMaster' -v
```

The master gate maps this surface to the `live` suite in
`NautilusMasterRequirements`.
