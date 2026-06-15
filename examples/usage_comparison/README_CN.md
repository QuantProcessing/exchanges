# 使用对比 Demo

这个目录实现了一个小型 order-book imbalance 策略示例：Go 版本展示当前平台的
可运行路径，旁边保留 Python 参考草图，方便维护者比较 callback ergonomics。

Go demo 是受支持的可运行路径：

1. 订阅 `BTC-USDT-SPOT.BINANCE` 的 order-book depth；
2. 当 best-bid size 超过 best-ask size 的 2 倍时，在 best ask 提交买入限价单；
3. 跟踪 accepted order、fill、position 和 account exposure。

## 文件

| 文件 | 作用 |
| --- | --- |
| `go_demo/demo.go` | 使用 `live.NewTradingNode`、`strategy.NewTyped`、runtime subscription helpers、`model.OrderFactory`、risk、portfolio、cache 和 platform execution routing 的可运行 Go demo。只有交易所网络边界是 in-memory。 |
| `go_demo/demo_test.go` | 回归测试，证明 Go demo 可以产生 filled order、fill report、long position、exposure update 和 event log。 |
| `go_demo/cmd/demo/main.go` | Go demo 的 CLI wrapper。 |
| Python reference sketch | 不可运行的参考策略形状，供维护者比较不同平台的 callback 写法。 |

## 运行 Go Demo

```bash
go test ./examples/usage_comparison/go_demo
go run ./examples/usage_comparison/go_demo/cmd/demo
```

期望 CLI 形状：

```text
signal_triggered=true
final_order=demo-order-1 status=filled filled=0.01 leaves=0
fills=1 first_trade=demo-trade-1
position=long qty=0.01 entry=101
usdt_exposure=1.01
events=[market:order_book execution:order:accepted execution:order:filled execution:fill]
```

## Go Demo 证明了什么

- 真实 `platform.Node` 可以接收 data 和 execution clients。
- market-data subscription 会把 order-book event 送到运行中的 strategy。
- strategy 在 `OnStart` 订阅，并通过 runtime 提交 factory-created normalized order。
- risk checks 会在 execution submission 前执行。
- execution reports 会更新 order state、fills、position 和 portfolio exposure。
- strategy 可以通过 typed callbacks 观察 order 和 fill updates。
