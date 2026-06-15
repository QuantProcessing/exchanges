# Live Node 配置

live runtime 可用 `live.NodeConfig` 或 `live.NodeBuilder` 组装。两种路径都会创建同一套
底层 stack：bus、cache、risk engine、portfolio、platform node、data clients、
execution clients 和 strategy engine。

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

`Build` 会校验 nil data clients、execution clients 和 strategies。没有显式提供的
shared components 会安全 default：bus、cache、risk engine 和 portfolio。

## Startup Order

`Node.Start(ctx)` 会委托给 live runner：

1. platform node starts data/execution clients；
2. strategy engine 以 platform node 作为 `strategy.Runtime` 启动；
3. strategy states 进入 running；
4. health monitor 监控 strategy errors 和 platform health。

如果 startup 失败，runner 会 fault，并停止已经启动的 platform path。

## Shutdown Semantics

调用 `Node.Stop(ctx)` 显式关闭。runner 会取消 monitor，先停止 strategies，再停止
platform node，并记录 stopped/faulted health state。fatal strategy errors 或 platform
health failures 也会请求同一条 graceful shutdown path。

shutdown contract 很严格：strategies 在 exchange clients disconnect 前停止，因此
`OnStop` callbacks 仍可引用 runtime state 并通过 platform unsubscribe。

## Verification

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./live ./platform ./testsuite -run 'Live|Node|Runner|Startup|Shutdown|Health' -v
```
