# Live Trading

The live node assembles the platform runtime from data clients, execution
clients, strategies, bus, cache, portfolio, risk engine, and reconnect policy.
Use `live.NodeConfig` for direct construction or `live.NodeBuilder` for
incremental assembly.

```go
node, err := live.NewNodeBuilder().
    AddDataClient(dataClient).
    AddExecutionClient(execClient).
    AddStrategy(strategy.NewTyped("strategy-id", impl)).
    Build()
if err != nil {
    return err
}
if err := node.Start(ctx); err != nil {
    return err
}
defer node.Stop(context.Background())
```

Startup and shutdown details are documented in:

- [Live Node Configuration](./live-node-configuration.md)

Verification:

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./live ./platform ./testsuite -run 'Live|Node|Runner|NautilusMaster' -v
```
