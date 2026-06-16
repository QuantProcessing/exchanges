# 实盘交易

`live.Node` 将 data clients、execution clients、strategies、bus、cache、portfolio、
risk engine、reconnect policy 和 health monitoring 组装成一个 runtime。

## Node Assembly

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

builder 会校验 nil clients 和 nil strategies。缺失的 bus、cache、risk、portfolio
会由 `live.NewNode` 提供默认值。

可运行代码：[06_run_live_node_with_in_memory_venue.go](../../examples/06_run_live_node_with_in_memory_venue.go)。

## Startup

```go
if err := node.Start(ctx); err != nil {
    return err
}
defer node.Stop(context.Background())
```

启动过程：

1. start platform node；
2. load instruments；
3. connect data clients；
4. connect execution clients；
5. query account snapshots；
6. start strategies；
7. start health monitoring。

## Runtime Behavior

Live orders 流程：

```text
strategy.Runtime
  -> platform.Node
  -> risk.Engine
  -> execution.Engine
  -> venue.ExecutionClient
  -> adapter
  -> SDK
```

Execution events 会经过 reconciliation、cache、portfolio、bus 和 strategy callbacks
回流。

## Health And Reconnect

`node.Health()` 报告 node state 和 component health。data/execution clients 也有
`Health()` snapshots。reconnect policy 属于 node/platform 边界，不应放进策略。

更多信息：

- [Stream Health](./stream-health_CN.md)
- [Reconciliation](./reconciliation_CN.md)

## Capability Checks

```go
caps := adapter.Capabilities()
if !caps.Execution.PrivateStream || !caps.Execution.Resubscribe {
    return fmt.Errorf("strategy requires private stream and resubscribe")
}
```

可选能力包括 modify、query、order lists、fill reports、position reports、mass status、
private stream 和 resubscribe。

## Shutdown

`Node.Stop(ctx)` 会先停止 strategies，再断开 clients。生产环境 shutdown handler 应使用
带 deadline 的 context。

## Verification

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./live ./platform ./testsuite -run 'Live|Node|Runner|Health|Reconnect' -v
```
