# SDK 层

`github.com/QuantProcessing/exchanges/sdk/...` 是交易所原生协议层。当你需要直接
访问某个交易所 API，而不是使用规范化 adapter 抽象时，应该使用这一层。

示例 import 路径：

```go
import binanceperp "github.com/QuantProcessing/exchanges/sdk/binance/perp"
import okxsdk "github.com/QuantProcessing/exchanges/sdk/okx"
```

SDK package 负责交易所原生 REST client、WebSocket client、请求/响应结构、
签名、endpoint 名称，以及协议特定选项。

## 允许的依赖

- `github.com/QuantProcessing/exchanges/sdk`：SDK 通用 primitive，例如
  `sdk.RequestOpts`。
- `github.com/QuantProcessing/exchanges/internal/...`：仓库私有共享 helper。
- 外部 Go module。

## 禁止的依赖

- 根 package `github.com/QuantProcessing/exchanges`。
- `github.com/QuantProcessing/exchanges/adapter/...`。
- `github.com/QuantProcessing/exchanges/account`。
- 其他交易所的 SDK package。

## 测试期望

- SDK 测试应靠近 method 和官方 API shape。
- 公共 read tests 可以调用真实官方公开 endpoint。
- Write tests 必须通过交易所专属 flag 和 credentials 显式开启。
- 共享协议 helper 应放在 `internal/...`，不要放进其他交易所 SDK 子树。
