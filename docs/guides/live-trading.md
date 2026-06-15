# Live Trading

`live.Node` assembles a runtime from data clients, execution clients,
strategies, bus, cache, portfolio, risk engine, reconnect policy, and health
monitoring. Use it when a strategy should run against real venue clients or a
live-like fake client in tests.

## Node Assembly

The builder is the easiest entry point:

```go
node, err := live.NewNodeBuilder().
    WithCache(cache.New()).
    WithRiskConfig(risk.Config{
        MaxOrderNotional: decimal.RequireFromString("100"),
    }).
    AddDataClient(dataClient).
    AddExecutionClient(executionClient).
    AddStrategy(strategy.NewTyped("strategy-id", strategyImpl)).
    Build()
if err != nil {
    return err
}
```

The builder validates nil clients and nil strategies. Missing bus, cache, risk,
and portfolio components are defaulted by `live.NewNode`.

## Startup

```go
if err := node.Start(ctx); err != nil {
    return err
}
defer node.Stop(context.Background())
```

Startup does this work:

1. starts the platform node;
2. loads instruments through data clients;
3. connects data clients;
4. connects execution clients;
5. queries account snapshots;
6. starts strategy callbacks;
7. starts health monitoring.

If startup fails after a partial start, the runner faults and stops the path
that already started.

## Runtime Behavior

Live orders flow through:

```text
strategy.Runtime
  -> platform.Node
  -> risk.Engine
  -> execution.Engine
  -> venue.ExecutionClient
  -> adapter
  -> SDK
```

Execution events flow back through reconciliation, cache, portfolio, bus, and
strategy callbacks.

Market data flows through data clients, cache, bus, and typed market callbacks.

## Health And Reconnect

`node.Health()` reports node state and component health. Data and execution
clients also expose `Health()` snapshots. Reconnect policy belongs at the node
or platform boundary, not inside strategy code.

For stream details, read:

- [Stream Health](./stream-health.md)
- [Reconciliation](./reconciliation.md)

## Capability Checks

Before using optional live features, check adapter capabilities:

```go
caps := adapter.Capabilities()
if !caps.Execution.PrivateStream || !caps.Execution.Resubscribe {
    return fmt.Errorf("strategy requires private stream and resubscribe")
}
```

Optional capabilities include modify, query, order lists, fill reports,
position reports, mass status, private stream, and resubscribe.

## Shutdown

`Node.Stop(ctx)` stops strategies before disconnecting clients. This lets
`OnStop` inspect runtime state and unwind subscriptions through the platform.

Use a context with a deadline in production shutdown handlers.

## Verification

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./live ./platform ./testsuite -run 'Live|Node|Runner|Health|Reconnect' -v
```
