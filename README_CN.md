# exchanges

[English](./README.md) | **中文**

统一的 Go 加密货币交易所 SDK。

同时提供**底层 SDK 客户端**（REST + WebSocket）和**高层适配器**（实现统一 `Exchange` 接口）—— Go 原生的 CCXT 替代方案。

## 特性

- **统一接口** — 一套 API 对接所有交易所，切换交易所只需改一行代码
- **全市场覆盖** — 支持永续合约、现货、杠杆交易
- **双通道传输** — REST 用于查询，WebSocket 用于实时推送和低延迟下单
- **内置安全机制** — 交易所特定的请求保护、限流错误映射、订单参数校验、滑点保护
- **本地状态管理** — WebSocket 维护的深度簿、仓位/订单追踪、余额同步
- **生产可用** — 已在量化交易系统中经过实战验证，日处理数千笔订单

## 支持的交易所

| 交易所      | 永续 | 现货 | 杠杆 | 报价币种          | 默认    |
|-------------|------|------|------|------------------|---------|
| Binance     | ✅    | ✅    | ✅    | USDT, USDC       | USDT    |
| OKX         | ✅    | ✅    | —    | USDT, USDC       | USDT    |
| Aster       | ✅    | ✅    | —    | USDT, USDC       | USDT    |
| Nado        | ✅    | ✅    | —    | USDT             | USDT    |
| Lighter     | ✅    | ✅    | —    | USDC             | USDC    |
| Hyperliquid | ✅    | ✅    | —    | USDC             | USDC    |
| Bitget      | ✅    | ✅    | —    | USDT, USDC       | USDT    |
| StandX      | ✅    | —    | —    | DUSD             | DUSD    |
| GRVT        | ✅    | —    | —    | USDT             | USDT    |
| EdgeX       | ✅    | —    | —    | USDC             | USDC    |
| Decibel     | ✅    | —    | —    | USDC             | USDC    |

### 交易所说明

- Bitget 当前只支持经典账户的私有 API 面。
- `PlaceOrder`、`CancelOrder` 等无后缀写接口表示适配器的主非 WS 写路径。
- `PlaceOrderWS`、`CancelOrderWS` 等 `*WS` 接口表示显式 WebSocket 写路径。`PlaceOrderWS` 只返回 `error`，并且要求设置 `OrderParams.ClientID`，方便后续通过订单流做关联。
- Bitget 经典账户的 WebSocket 写能力仍然需要由 Bitget 为该 API key 额外开通经典交易 socket 权限。
- Decibel 在本仓库中仅支持永续。它使用带鉴权的 REST/WebSocket 读取能力，以及基于 Aptos 签名的链上交易写入，凭证为 `api_key + private_key + subaccount_addr`。

## 安装

```bash
go get github.com/QuantProcessing/exchanges
```

---

## 设计理念

### 1. 适配器模式

每个交易所都实现相同的 `Exchange` 接口。策略代码无需接触交易所特定 API：

```go
// 此函数适用于任何交易所 — Binance、OKX、Hyperliquid 等
func getSpread(ctx context.Context, adp exchanges.Exchange, symbol string) (decimal.Decimal, error) {
    ob, err := adp.FetchOrderBook(ctx, symbol, 1)
    if err != nil {
        return decimal.Zero, err
    }
    return ob.Asks[0].Price.Sub(ob.Bids[0].Price), nil
}
```

### 2. 符号约定

所有方法统一接受**基础货币符号**（如 `"BTC"`、`"ETH"`），适配器内部根据配置的报价币种自动转换为交易所特定格式：

| 你传入    | Binance (USDT)    | Binance (USDC)   | OKX (USDT)        | Hyperliquid      |
|----------|-------------------|------------------|-------------------|------------------|
| `"BTC"`  | `"BTCUSDT"`       | `"BTCUSDC"`      | `"BTC-USDT-SWAP"` | `"BTC"`          |

### 3. 双层架构

```
┌─────────────────────────────────────────────────────┐
│  你的策略 / 应用                                      │
├─────────────────────────────────────────────────────┤
│  适配器层 (exchanges.Exchange 接口)                    │  ← 统一 API
│    binance.Adapter / okx.Adapter / nado.Adapter     │
├─────────────────────────────────────────────────────┤
│  SDK 层 (底层 REST + WebSocket 客户端)                │  ← 交易所特定
│    binance/sdk/ / okx/sdk/ / nado/sdk/              │
└─────────────────────────────────────────────────────┘
```

- **适配器层**：实现 `exchanges.Exchange` 接口。处理符号映射、订单校验、滑点逻辑和状态管理。
- **SDK 层**：轻量 REST/WebSocket 客户端，与交易所 API 端点一一对应。可以直接使用以获得最大灵活性。

---

## 快速开始

### 基础用法

```go
package main

import (
    "context"
    "fmt"

    exchanges "github.com/QuantProcessing/exchanges"
    "github.com/QuantProcessing/exchanges/binance"
    "github.com/shopspring/decimal"
)

func main() {
    ctx := context.Background()

    // 创建 Binance 永续适配器（默认 USDT 市场）
    adp, err := binance.NewAdapter(ctx, binance.Options{
        APIKey:    "your-api-key",
        SecretKey: "your-secret-key",
        // QuoteCurrency: exchanges.QuoteCurrencyUSDC, // 取消注释切换为 USDC 市场
    })
    if err != nil {
        panic(err)
    }
    defer adp.Close()

    // 获取行情
    ticker, err := adp.FetchTicker(ctx, "BTC")
    if err != nil {
        panic(err)
    }
    fmt.Printf("BTC 价格: %s\n", ticker.LastPrice)

    // 获取深度
    ob, err := adp.FetchOrderBook(ctx, "BTC", 5)
    if err != nil {
        panic(err)
    }
    fmt.Printf("最优买价: %s, 最优卖价: %s\n",
        ob.Bids[0].Price, ob.Asks[0].Price)

    // 下限价单
    order, err := adp.PlaceOrder(ctx, &exchanges.OrderParams{
        Symbol:   "BTC",
        Side:     exchanges.OrderSideBuy,
        Type:     exchanges.OrderTypeLimit,
        Price:    ticker.Bid,
        Quantity: decimal.NewFromFloat(0.001),
    })
    if err != nil {
        panic(err)
    }
    fmt.Printf("下单成功: %s\n", order.OrderID)
}
```

### 配置驱动启动

```yaml
# exchanges.yaml
exchanges:
  - name: BINANCE
    market_type: perp
    options:
      api_key: ${BINANCE_API_KEY}
      secret_key: ${BINANCE_SECRET_KEY}
      quote_currency: USDT

  - name: OKX
    alias: okx-spot
    market_type: spot
    options:
      api_key: ${OKX_API_KEY}
      secret_key: ${OKX_SECRET_KEY}
      passphrase: ${OKX_PASSPHRASE}
      quote_currency: USDT
```

```go
package main

import (
    "context"
    "fmt"

    exconfig "github.com/QuantProcessing/exchanges/config"
    _ "github.com/QuantProcessing/exchanges/config/all"
)

func main() {
    ctx := context.Background()

    mgr, err := exconfig.LoadManager(ctx, "exchanges.yaml")
    if err != nil {
        panic(err)
    }
    defer mgr.CloseAll()

    adp, err := mgr.GetAdapter("BINANCE")
    if err != nil {
        panic(err)
    }

    ticker, err := adp.FetchTicker(ctx, "BTC")
    if err != nil {
        panic(err)
    }

    fmt.Printf("BTC 价格: %s\n", ticker.LastPrice)
}
```

- `config.LoadManager` 同时支持 YAML 和 JSON。
- 解析前会用 `os.ExpandEnv` 展开 `${ENV_VAR}`。
- 空白导入 `config/all` 即可自动注册所有内置交易所构造器。
- 默认别名规则是：某交易所只配置一次时使用交易所名；同名配置多次时使用 `NAME/market_type`。

### 带滑点保护的市价单

```go
// 0.5% 滑点保护的市价单
// 内部自动转换为 LIMIT IOC 单，价格 = ask * 1.005（买入）
order, err := adp.PlaceOrder(ctx, &exchanges.OrderParams{
    Symbol:   "ETH",
    Side:     exchanges.OrderSideBuy,
    Type:     exchanges.OrderTypeMarket,
    Quantity: decimal.NewFromFloat(0.1),
    Slippage: decimal.NewFromFloat(0.005), // 0.5%
})
```

### 便捷函数

```go
// 一行市价单
order, err := exchanges.PlaceMarketOrder(ctx, adp, "BTC", exchanges.OrderSideBuy, qty)

// 传入可选参考价的市价单
order, err := exchanges.PlaceMarketOrder(ctx, adp, "BTC", exchanges.OrderSideBuy, qty, refPrice)

// 一行限价单
order, err := exchanges.PlaceLimitOrder(ctx, adp, "BTC", exchanges.OrderSideBuy, price, qty)

// 带滑点的市价单
order, err := exchanges.PlaceMarketOrderWithSlippage(ctx, adp, "BTC", exchanges.OrderSideBuy, qty, slippage)

// 带可选参考价的滑点保护市价单
order, err := exchanges.PlaceMarketOrderWithSlippage(ctx, adp, "BTC", exchanges.OrderSideBuy, qty, slippage, refPrice)
```

`refPrice` 是可选的。传入后，依赖本地参考价做市价单/滑点转换的适配器可以少一次额外的 ticker 请求。

### WebSocket 实时流

```go
// 实时深度（本地维护）
err := adp.WatchOrderBook(ctx, "BTC", 20, func(ob *exchanges.OrderBook) {
    fmt.Printf("BTC 买一: %s 卖一: %s\n", ob.Bids[0].Price, ob.Asks[0].Price)
})

// 随时拉取最新快照（零延迟，无需 API 调用）
ob := adp.GetLocalOrderBook("BTC", 5)

// depth <= 0 时返回本地维护的完整深度
full := adp.GetLocalOrderBook("BTC", 0)

// 实时行情推送
adp.WatchTicker(ctx, "BTC", func(t *exchanges.Ticker) {
    fmt.Printf("价格: %s\n", t.LastPrice)
})

// 实时订单概览更新
adp.WatchOrders(ctx, func(o *exchanges.Order) {
    fmt.Printf("订单 %s: %s 委托价=%s 已成交=%s\n",
        o.OrderID, o.Status, o.OrderPrice, o.FilledQuantity)
})

// 实时成交更新（每次 callback 可能是一笔成交，也可能是交易所原生聚合后的 fill 更新）
adp.WatchFills(ctx, func(f *exchanges.Fill) {
    fmt.Printf("成交 %s: %s %s @ %s\n",
        f.TradeID, f.Side, f.Quantity, f.Price)
})

// 实时仓位更新
adp.WatchPositions(ctx, func(p *exchanges.Position) {
    fmt.Printf("%s: %s %s @ %s\n", p.Symbol, p.Side, p.Quantity, p.EntryPrice)
})
```

`WatchOrders` 用来看订单概览状态。它适合查看委托价、数量、已成交数量、状态、ID 和时间戳。`WatchFills` 用来看成交执行细节。它适合查看执行价、执行数量、手续费、手续费资产和 maker/taker。同一笔订单可能对应 0 次、1 次或多次 `WatchFills` 回调，而且部分交易所会把多笔原生成交聚合成一次 fill 更新。

除 Binance margin 外，本仓库里当前所有支持私有交易的 adapter 都已经实现了 `WatchFills`。如果某个 adapter 无法原生提供私有成交流，它会明确返回 `ErrNotSupported`，而不会偷偷从别的流里合成成交事件。

### 永续合约扩展接口

```go
// 类型断言获取永续合约特有功能
if perp, ok := adp.(exchanges.PerpExchange); ok {
    // 设置杠杆
    perp.SetLeverage(ctx, "BTC", 10)

    // 获取持仓
    positions, _ := perp.FetchPositions(ctx)
    for _, p := range positions {
        fmt.Printf("%s: %s %s\n", p.Symbol, p.Side, p.Quantity)
    }

    // 获取资金费率
    fr, _ := perp.FetchFundingRate(ctx, "BTC")
    fmt.Printf("资金费率: %s\n", fr.FundingRate)
}
```

### TradingAccount + OrderFlow（统一状态管理）

```go
import "github.com/QuantProcessing/exchanges/account"

// TradingAccount 现在位于公开的 account 子包。
// Place 会返回一个 OrderFlow，方便你查看融合快照和单笔订单成交流。
acct := account.NewTradingAccount(adp, nil)
if err := acct.Start(ctx); err != nil {
    panic(err)
}
defer acct.Close()

flow, err := acct.Place(ctx, &exchanges.OrderParams{
    Symbol:   "BTC",
    Side:     exchanges.OrderSideBuy,
    Type:     exchanges.OrderTypeMarket,
    Quantity: decimal.NewFromFloat(0.001),
})
if err != nil {
    panic(err)
}
defer flow.Close()

for {
    select {
    case order, ok := <-flow.C():
        if !ok {
            return
        }
        fmt.Printf("订单=%s 状态=%s 已成交=%s 最近成交=%s@%s\n",
            order.OrderID, order.Status, order.FilledQuantity,
            order.LastFillQuantity, order.LastFillPrice)
    case fill, ok := <-flow.Fills():
        if !ok {
            return
        }
        fmt.Printf("成交=%s 数量=%s 价格=%s\n", fill.TradeID, fill.Quantity, fill.Price)
    }
}

latest := flow.Latest()
fmt.Printf("最新快照: %s %s\n", latest.OrderID, latest.Status)
```

`OrderFlow.C()` 现在返回单笔订单的融合快照。订单生命周期字段来自 `WatchOrders`，当交易所支持私有成交流时，还会把 `WatchFills` 推导出的 `FilledQuantity`、`LastFillQuantity`、`LastFillPrice`、`AverageFillPrice` 一并带上。

`OrderFlow.Fills()` 仍然提供同一笔订单的原始成交明细流。要逐笔执行细节时读 `Fills()`；要在策略控制流里消费统一快照时读 `C()`。

Downstream consumer migrations, including cross-exchanges-arb, are intentionally deferred until this repository has passed full shared testing and a new release tag is published.

> TradingAccount 是当前发布面向外部的账户运行时入口，位于 `github.com/QuantProcessing/exchanges/account`。新的集成应基于 `TradingAccount + OrderFlow`。

## Migration Order

1. 升级到包含 `TradingAccount + OrderFlow` 的发布版本。
2. 重新运行你现有的 adapter 集成测试。
3. 只有在新 tag 发布后，才迁移下游应用。

### 切换交易所

```go
// Binance — USDT 市场（默认）
adp, _ := binance.NewAdapter(ctx, binance.Options{
    APIKey: os.Getenv("BINANCE_API_KEY"), SecretKey: os.Getenv("BINANCE_SECRET_KEY"),
})

// Binance — USDC 市场
adpUSDC, _ := binance.NewAdapter(ctx, binance.Options{
    APIKey: os.Getenv("BINANCE_API_KEY"), SecretKey: os.Getenv("BINANCE_SECRET_KEY"),
    QuoteCurrency: exchanges.QuoteCurrencyUSDC,
})

// OKX — 相同接口，不同构造器
adp, _ := okx.NewAdapter(ctx, okx.Options{
    APIKey: os.Getenv("OKX_API_KEY"), SecretKey: os.Getenv("OKX_SECRET_KEY"),
    Passphrase: os.Getenv("OKX_PASSPHRASE"),
})

// Hyperliquid — 钱包签名认证（仅支持 USDC）
adp, _ := hyperliquid.NewAdapter(ctx, hyperliquid.Options{
    PrivateKey: os.Getenv("HYPERLIQUID_PRIVATE_KEY"), AccountAddr: os.Getenv("HYPERLIQUID_ACCOUNT_ADDR"),
})

// 所有适配器暴露完全相同的 Exchange 接口
ticker, _ := adp.FetchTicker(ctx, "BTC")
```

### 报价币种

每个适配器支持 `QuoteCurrency` 选项，用于指定连接哪个报价币种市场。省略时使用各交易所自己的默认值。大多数 CEX 默认是 USDT，DEX 则因交易所而异。

```go
// 可用报价币种
exchanges.QuoteCurrencyUSDT // "USDT"
exchanges.QuoteCurrencyUSDC // "USDC"
exchanges.QuoteCurrencyDUSD // "DUSD"（仅 StandX）
```

传入不支持的报价币种会在构造时返回错误：

```go
// 失败：Hyperliquid 仅支持 USDC
_, err := hyperliquid.NewAdapter(ctx, hyperliquid.Options{
    QuoteCurrency: exchanges.QuoteCurrencyUSDT, // 报错！
})
// err: "hyperliquid: unsupported quote currency "USDT", supported: [USDC]"
```

---

## API 参考

### Exchange 接口（核心）

每个适配器均实现以下方法：

| 分类 | 方法 | 说明 |
|------|------|------|
| **标识** | `GetExchange()` | 返回交易所名称（如 `"BINANCE"`） |
| | `GetMarketType()` | 返回 `"perp"` 或 `"spot"` |
| | `Close()` | 关闭所有连接 |
| **符号** | `FormatSymbol(symbol)` | 将 `"BTC"` 转为交易所格式 |
| | `ExtractSymbol(symbol)` | 将交易所格式转回 `"BTC"` |
| | `ListSymbols()` | 返回所有可用交易对 |
| **行情** | `FetchTicker(ctx, symbol)` | 最新价、买卖价、24h 成交量 |
| | `FetchOrderBook(ctx, symbol, limit)` | 深度快照（REST） |
| | `FetchTrades(ctx, symbol, limit)` | 最近成交 |
| | `FetchKlines(ctx, symbol, interval, opts)` | K 线 / OHLCV 数据 |
| **交易** | `PlaceOrder(ctx, params)` | 下单（市价/限价/PostOnly） |
| | `CancelOrder(ctx, orderID, symbol)` | 撤销单个订单 |
| | `CancelAllOrders(ctx, symbol)` | 撤销某交易对全部订单 |
| | `FetchOrderByID(ctx, orderID, symbol)` | 按订单 ID 查询单笔订单；若交易所支持，应可返回终态订单 |
| | `FetchOrders(ctx, symbol)` | 查询某个 symbol 下所有可见订单 |
| | `FetchOpenOrders(ctx, symbol)` | 查询所有挂单 |
| **账户** | `FetchAccount(ctx)` | 完整账户：余额 + 持仓 + 挂单 |
| | `FetchBalance(ctx)` | 仅查可用余额 |
| | `FetchSymbolDetails(ctx, symbol)` | 精度和最小数量规则 |
| | `FetchFeeRate(ctx, symbol)` | Maker/Taker 费率 |
| **深度簿** | `WatchOrderBook(ctx, symbol, depth, cb)` | 订阅 WS 深度（阻塞至就绪） |
| | `GetLocalOrderBook(symbol, depth)` | 读取本地维护的深度簿 |
| | `StopWatchOrderBook(ctx, symbol)` | 取消订阅 |
| **实时流** | `WatchOrders(ctx, cb)` | 实时订单生命周期更新 |
| | `WatchFills(ctx, cb)` | 实时私有成交 / execution |
| | `WatchPositions(ctx, cb)` | 实时仓位更新 |
| | `WatchTicker(ctx, symbol, cb)` | 实时行情 |
| | `WatchTrades(ctx, symbol, cb)` | 实时成交 |
| | `WatchKlines(ctx, symbol, interval, cb)` | 实时 K 线 |

`FetchOrderByID`、`FetchOrders`、`FetchOpenOrders` 是刻意拆开的语义：单笔查单不能用扫描挂单来伪实现，`FetchOrders` 的范围也应大于 `FetchOpenOrders`。

### PerpExchange 接口（继承 Exchange）

| 方法 | 说明 |
|------|------|
| `FetchPositions(ctx)` | 查询所有持仓 |
| `SetLeverage(ctx, symbol, leverage)` | 设置杠杆倍数 |
| `FetchFundingRate(ctx, symbol)` | 查询当前资金费率 |
| `FetchAllFundingRates(ctx)` | 查询所有资金费率 |
| `ModifyOrder(ctx, orderID, symbol, params)` | 修改挂单（价格/数量） |

### SpotExchange 接口（继承 Exchange）

| 方法 | 说明 |
|------|------|
| `FetchSpotBalances(ctx)` | 查询各币种余额（可用/冻结） |
| `TransferAsset(ctx, params)` | 现货与合约间划转 |

### OrderParams 下单参数

```go
type OrderParams struct {
    Symbol      string          // 基础符号: "BTC", "ETH"
    Side        OrderSide       // OrderSideBuy 或 OrderSideSell
    Type        OrderType       // OrderTypeMarket, OrderTypeLimit, OrderTypePostOnly
    Quantity    decimal.Decimal // 下单数量
    Price       decimal.Decimal // 限价单必填
    TimeInForce TimeInForce     // GTC（默认）, IOC, FOK
    ReduceOnly  bool            // 只减仓
    Slippage    decimal.Decimal // 若 > 0，MARKET 转为 LIMIT IOC + 滑点
    ClientID    string          // 自定义订单 ID
}
```

### Backpack 的 ClientID 说明

Backpack 要求 `clientId` 必须是 `uint32` 范围内的数字。若你要自己传 `OrderParams.ClientID`，不要传 UUID、毫秒时间戳，或任何大于 `4294967295` 的值。

建议直接用包内 helper：

```go
import "github.com/QuantProcessing/exchanges/backpack"

params := &exchanges.OrderParams{
    Symbol:   "BTC",
    Side:     exchanges.OrderSideBuy,
    Type:     exchanges.OrderTypeLimit,
    Quantity: qty,
    Price:    price,
    ClientID: backpack.GenerateClientID(),
}
```

如果不传 `ClientID`，Backpack adapter 也会自动生成一个安全可用的值。

### 错误处理

```go
order, err := adp.PlaceOrder(ctx, params)
if err != nil {
    // 结构化错误匹配
    if errors.Is(err, exchanges.ErrInsufficientBalance) {
        // 余额不足
    }
    if errors.Is(err, exchanges.ErrMinQuantity) {
        // 低于最小数量
    }
    if errors.Is(err, exchanges.ErrRateLimited) {
        // 触发限流 — 按自己的策略重试/退避
    }

    // 获取交易所原始错误信息
    var exErr *exchanges.ExchangeError
    if errors.As(err, &exErr) {
        fmt.Printf("[%s] Code: %s, Message: %s\n", exErr.Exchange, exErr.Code, exErr.Message)
    }
}
```

可用哨兵错误：`ErrInsufficientBalance`、`ErrRateLimited`、`ErrInvalidPrecision`、`ErrOrderNotFound`、`ErrSymbolNotFound`、`ErrMinNotional`、`ErrMinQuantity`、`ErrAuthFailed`、`ErrNetworkTimeout`、`ErrNotSupported`。

### 限流错误处理

当交易所返回限流错误时，SDK 会将其包装为结构化 `ExchangeError`，以 `ErrRateLimited` 作为底层错误。该错误会透传整个调用链：

```
你的代码 (caller)
  → adapter.PlaceOrder()     // 透明传递 (return nil, err)
    → client.Post()          // 返回 exchanges.NewExchangeError(..., ErrRateLimited)
```

**基本检测** — 使用 `errors.Is()` 判断是否限流：

```go
order, err := adp.PlaceOrder(ctx, params)
if errors.Is(err, exchanges.ErrRateLimited) {
    log.Warn("触发限流，等待重试...")
    time.Sleep(5 * time.Second)
}
```

**提取交易所信息** — 使用 `errors.As()` 获取完整错误上下文：

```go
var exErr *exchanges.ExchangeError
if errors.As(err, &exErr) && errors.Is(err, exchanges.ErrRateLimited) {
    fmt.Printf("交易所: %s\n", exErr.Exchange) // "BINANCE", "GRVT", "LIGHTER" 等
    fmt.Printf("错误码: %s\n", exErr.Code)     // "-1003", "1006", "429"
    fmt.Printf("消息:   %s\n", exErr.Message)  // 原始错误消息
}
```

**推荐重试模式** — 指数退避：

```go
func placeOrderWithRetry(ctx context.Context, adp exchanges.Exchange, params *exchanges.OrderParams) (*exchanges.Order, error) {
    maxRetries := 3
    for i := 0; i < maxRetries; i++ {
        order, err := adp.PlaceOrder(ctx, params)
        if err == nil {
            return order, nil
        }
        if !errors.Is(err, exchanges.ErrRateLimited) {
            return nil, err // 非限流错误，直接失败
        }
        backoff := time.Duration(1<<uint(i)) * time.Second // 1s, 2s, 4s
        log.Warnf("触发限流 (第 %d/%d 次)，%v 后重试", i+1, maxRetries, backoff)
        select {
        case <-time.After(backoff):
        case <-ctx.Done():
            return nil, ctx.Err()
        }
    }
    return nil, fmt.Errorf("限流重试 %d 次后仍失败", maxRetries)
}
```

> **设计说明**：本库不实现自动重试或退避。限流处理策略（固定延迟、指数退避、熔断器等）属于业务层决策，应由调用方自行控制。

---

## 内置安全机制

### 限流

限流检测已在 SDK 层为所有支持的交易所逐一实现：

| 交易所 | 检测信号 | 错误码 | 说明 |
|--------|---------|--------|------|
| **Binance** | HTTP 429/418, code -1003/-1015, `X-Mbx-Used-Weight` 头 | `-1003`, `-1015` | 权重追踪 + 订单计数 |
| **Aster** | 同 Binance（Binance 系分叉） | `-1003`, `-1015` | 同样支持 X-Mbx-* 头 |
| **OKX** | HTTP 429, code `50011`/`50061` | `50011`, `50061` | 按端点独立限流 |
| **Hyperliquid** | HTTP 429, 消息文本匹配 | `429` | 响应消息关键词检测 |
| **EdgeX** | HTTP 429, code/消息匹配 | `429` | 自定义错误码/消息 |
| **GRVT** | HTTP 429, 错误码 `1006` | `1006` | 按交易对独立追踪 |
| **Lighter** | HTTP 429 | `429` | 权重限制（标准 60 req/min） |
| **Nado** | HTTP 429 | `429` | 1200 req/min per IP |
| **StandX** | HTTP 429 | `429` | 支持 Retry-After 头 |

所有限流错误均被包装为 `ExchangeError`，底层错误为 `ErrRateLimited`。使用 `errors.Is(err, exchanges.ErrRateLimited)` 即可统一检测 — 详细用法见 [限流错误处理](#限流错误处理)。

### IP 封禁检测与自动恢复

当交易所返回明确的封禁或限流错误（例如 HTTP 418/429）时，SDK 会把它们包装成结构化错误，由调用方决定是否重试、退避或暂停。

### 订单校验

下单前适配器自动执行：
- 按交易对精度四舍五入价格
- 按交易对精度截断数量
- 校验最小数量和最小名义价值

---

## 日志

所有适配器支持可选的 `Logger` 注入实现结构化日志：

```go
// 兼容 *zap.SugaredLogger
logger := zap.NewProduction().Sugar()
adp, _ := binance.NewAdapter(ctx, binance.Options{
    APIKey: "...", SecretKey: "...",
    Logger: logger,
})
```

不提供 Logger 时默认使用 `NopLogger`（静默）。接口定义：

```go
type Logger interface {
    Debugw(msg string, keysAndValues ...any)
    Infow(msg string, keysAndValues ...any)
    Warnw(msg string, keysAndValues ...any)
    Errorw(msg string, keysAndValues ...any)
}
```

---

## 项目结构

```
exchanges/                  根包 — 接口、模型、错误、工具函数
├── exchange.go             核心 Exchange / PerpExchange / SpotExchange 接口
├── models.go               统一数据类型（Order, Position, Ticker 等）
├── errors.go               哨兵错误 + ExchangeError 类型
├── base_adapter.go         共享适配器逻辑（深度簿、校验、通用辅助）
├── local_orderbook.go      本地订单簿缓存与同步辅助
├── log.go                  Logger 接口 + NopLogger
├── account/                公开的账户运行时子包
│   ├── trading_account.go  TradingAccount 运行时入口
│   ├── order_flow.go       OrderFlow 生命周期流 + 最新快照辅助
│   └── ...                 内部运行时同步辅助
├── testsuite/              适配器一致性测试套件
├── binance/                Binance 适配器 + SDK
│   ├── options.go          Options{APIKey, SecretKey, QuoteCurrency, Logger}
│   ├── perp_adapter.go     永续适配器 (Exchange + PerpExchange)
│   ├── spot_adapter.go     现货适配器 (Exchange + SpotExchange)
│   └── sdk/                底层 REST & WebSocket 客户端
├── okx/                    OKX（相同结构）
├── aster/                  Aster
├── nado/                   Nado
├── lighter/                Lighter
├── hyperliquid/            Hyperliquid
├── standx/                 StandX
├── grvt/                   GRVT (build tag: grvt)
└── edgex/                  EdgeX (build tag: edgex)
```

## 测试

仓库现在采用分层验证模型。普通 `go test ./...` 不再是规范化验证入口，因为部分包包含依赖凭证或较长时间 WebSocket 观察的实盘测试。

当你需要实盘/私有接口验证时，先复制环境变量模板并填入凭证：
```bash
cp .env.example .env
```

运行默认快速验证：
```bash
go test -short ./...
```

针对单个交易所运行短验证：
```bash
scripts/verify_exchange.sh backpack
scripts/verify_exchange.sh okx
scripts/verify_exchange.sh hyperliquid
```

运行完整回归验证：
```bash
GOCACHE=/tmp/exchanges-gocache bash scripts/verify_full.sh
```

`verify_full.sh` 会从仓库根目录加载 `.env`，保留已导出的 shell 环境变量，并自动管理 `RUN_FULL=1`。它同时兼容以下历史变量别名：

- `EDGEX_PRIVATE_KEY -> EDGEX_STARK_PRIVATE_KEY`
- `NADO_SUB_ACCOUNT_NAME -> NADO_SUBACCOUNT_NAME`
- `OKX_SECRET_KEY -> OKX_API_SECRET`
- `OKX_PASSPHRASE -> OKX_API_PASSPHRASE`

运行长时间订阅检查：
```bash
GOCACHE=/tmp/exchanges-gocache RUN_SOAK=1 bash scripts/verify_soak.sh
```

当前 soak 套件运行指定包的 3 分钟流稳定性检查。`RUN_FULL` 和 `RUN_SOAK` 是这些专用验证脚本的控制开关，不是默认测试入口。

## 许可证

MIT
