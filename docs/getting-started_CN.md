# 快速开始

本指南带你完成三种最常见的使用方式：

1. 通过 venue adapter 获取规范化行情；
2. 基于 `strategy.Runtime` 编写策略；
3. 用同一种策略形状运行 backtest 和 live node。

当前 module 目标是 Go 1.26。

## 安装

```bash
go get github.com/QuantProcessing/exchanges
```

本地测试和示例建议把 Go build cache 放到仓库外：

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./examples/...
```

## 选择正确层级

| 需求 | 使用 | 原因 |
| --- | --- | --- |
| 直接访问交易所 endpoint | `sdk/<venue>` | 需要 venue-native request/response。 |
| 规范化行情或订单方法 | `adapter/<venue>` 与 `venue` interfaces | 需要稳定跨交易所接口。 |
| 策略、回测、实盘、风控、组合、reconciliation | `strategy`、`backtest`、`live`、`platform` | 需要生命周期正确的交易行为。 |

大多数应用不应该在策略代码中 import SDK。策略应使用 normalized model types，并通过
runtime 提交命令，这样 risk、execution、cache、portfolio 才能保持一致。

## 通过 Adapter 获取行情

```go
ctx := context.Background()

adp, err := binance.NewSpotAdapter(ctx, binance.Options{})
if err != nil {
    panic(err)
}
defer adp.Close(ctx)

instrumentID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
ticker, err := adp.Data().FetchTicker(ctx, instrumentID)
if err != nil {
    panic(err)
}
fmt.Println(ticker.Last)
```

可编译示例：[01_fetch_ticker_with_adapter.go](../examples/01_fetch_ticker_with_adapter.go)。

如果要支持多个 venue，优先面向 `venue.DataClient` 和 `venue.ExecutionClient`
接口编程，而不是直接依赖具体 adapter。

## 编写最小策略

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

关键规则：

- 保存 `OnStart` 中传入的 runtime，不要自己创建 runtime。
- 使用 `OrderFactory` 生成 account ID、client order ID、list ID 和 command metadata。
- venue-specific 细节应放在 data/execution client 或 adapter 中。
- open orders、fills、positions、balances 和 exposure 从 `rt.Cache()` 与
  `rt.Portfolio()` 查询。

可编译示例：[02_build_orders_with_order_factory.go](../examples/02_build_orders_with_order_factory.go)
和 [04_run_strategy_backtest.go](../examples/04_run_strategy_backtest.go)。

## 在 Backtest 中运行策略

```go
events := []backtest.Event{
    {
        At:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
        Topic: strategy.TopicMarketData,
        Message: model.MarketEvent{OrderBook: &model.OrderBook{
            InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
            Bids: []model.OrderBookLevel{{Price: decimal.RequireFromString("100"), Size: decimal.RequireFromString("3")}},
            Asks: []model.OrderBookLevel{{Price: decimal.RequireFromString("101"), Size: decimal.RequireFromString("1")}},
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

可编译示例：[04_run_strategy_backtest.go](../examples/04_run_strategy_backtest.go)。

## 组装 Live Node

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

live node 启动路径会加载 instruments、连接 market data、连接 execution、查询 account
state、启动 strategies、转发 stream events 并记录 health。

可编译示例：[06_run_live_node_with_in_memory_venue.go](../examples/06_run_live_node_with_in_memory_venue.go)。

## 下一步阅读

- [模块指南](./module-guide_CN.md)
- [运行时流程](./runtime-flow_CN.md)
- [可靠量化程序指南](./guides/reliable-quant-program_CN.md)
- [策略编写](./guides/strategy-authoring_CN.md)
- [回测](./guides/backtesting_CN.md)
- [实盘交易](./guides/live-trading_CN.md)
- [示例](../examples/README_CN.md)
