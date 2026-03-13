# exchanges

[English](./README.md) | **中文**

统一的 Go 加密货币交易所 SDK。

同时提供**底层 SDK 客户端**（REST + WebSocket）和**高层适配器**（实现统一 `Exchange` 接口）—— Go 原生的 CCXT 替代方案。

## 特性

- **统一接口** — 一套 API 对接所有交易所，切换交易所只需改一行代码
- **全市场覆盖** — 支持永续合约、现货、杠杆交易
- **双通道传输** — REST 用于查询，WebSocket 用于实时推送和低延迟下单
- **内置安全机制** — 声明式限流、IP 封禁自动检测/恢复、订单参数校验、滑点保护
- **本地状态管理** — WebSocket 维护的深度簿、仓位/订单追踪、余额同步
- **生产可用** — 已在量化交易系统中经过实战验证，日处理数千笔订单

## 支持的交易所

| 交易所      | 永续 | 现货 | 杠杆 | 报价币种          | 默认    |
|-------------|------|------|------|------------------|---------|
| Binance     | ✅    | ✅    | ✅    | USDT, USDC       | USDT    |
| OKX         | ✅    | ✅    | —    | USDT, USDC       | USDT    |
| Aster       | ✅    | ✅    | —    | USDT, USDC       | USDC    |
| Nado        | ✅    | ✅    | —    | USDT             | USDT    |
| Lighter     | ✅    | ✅    | —    | USDC             | USDC    |
| Hyperliquid | ✅    | ✅    | —    | USDC             | USDC    |
| StandX      | ✅    | —    | —    | DUSD             | DUSD    |
| GRVT        | ✅    | —    | —    | USDT             | USDT    |
| EdgeX       | ✅    | ✅    | —    | USDC             | USDC    |

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

- **适配器层**：实现 `exchanges.Exchange` 接口。处理符号映射、订单校验、滑点逻辑、限流和状态管理。
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

// 一行限价单
order, err := exchanges.PlaceLimitOrder(ctx, adp, "BTC", exchanges.OrderSideBuy, price, qty)

// 带滑点的市价单
order, err := exchanges.PlaceMarketOrderWithSlippage(ctx, adp, "BTC", exchanges.OrderSideBuy, qty, slippage)
```

### WebSocket 实时流

```go
// 实时深度（本地维护）
err := adp.WatchOrderBook(ctx, "BTC", func(ob *exchanges.OrderBook) {
    fmt.Printf("BTC 买一: %s 卖一: %s\n", ob.Bids[0].Price, ob.Asks[0].Price)
})

// 随时拉取最新快照（零延迟，无需 API 调用）
ob := adp.GetLocalOrderBook("BTC", 5)

// 实时行情推送
adp.WatchTicker(ctx, "BTC", func(t *exchanges.Ticker) {
    fmt.Printf("价格: %s\n", t.LastPrice)
})

// 实时订单更新（成交、撤单）
adp.WatchOrders(ctx, func(o *exchanges.Order) {
    fmt.Printf("订单 %s: %s\n", o.OrderID, o.Status)
})

// 实时仓位更新
adp.WatchPositions(ctx, func(p *exchanges.Position) {
    fmt.Printf("%s: %s %s @ %s\n", p.Symbol, p.Side, p.Quantity, p.EntryPrice)
})
```

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

### AccountManager（自动状态同步）

```go
// AccountManager 订阅 WS 推送，自动维护本地状态
mgr, err := exchanges.NewAccountManager(adp.(exchanges.PerpExchange), nil)
if err != nil {
    panic(err)
}
err = mgr.Start(ctx, 1*time.Minute) // 每分钟 REST 刷新作为兜底

// 随时读取状态（线程安全，零延迟）
pos, ok := mgr.GetPosition("BTC")
order, ok := mgr.GetOrder("order-123")
balance := mgr.GetLocalBalance()

// 通过 channel 接收推送
go func() {
    for order := range mgr.GetOrderStream() {
        fmt.Printf("订单更新: %s %s\n", order.OrderID, order.Status)
    }
}()
```

### 切换交易所

```go
// Binance — USDT 市场（默认）
adp, _ := binance.NewAdapter(ctx, binance.Options{
    APIKey: os.Getenv("BINANCE_API_KEY"), SecretKey: os.Getenv("BINANCE_SECRET"),
})

// Binance — USDC 市场
adpUSDC, _ := binance.NewAdapter(ctx, binance.Options{
    APIKey: os.Getenv("BINANCE_API_KEY"), SecretKey: os.Getenv("BINANCE_SECRET"),
    QuoteCurrency: exchanges.QuoteCurrencyUSDC,
})

// OKX — 相同接口，不同构造器
adp, _ := okx.NewAdapter(ctx, okx.Options{
    APIKey: os.Getenv("OKX_API_KEY"), SecretKey: os.Getenv("OKX_SECRET"),
    Passphrase: os.Getenv("OKX_PASSPHRASE"),
})

// Hyperliquid — 钱包签名认证（仅支持 USDC）
adp, _ := hyperliquid.NewAdapter(ctx, hyperliquid.Options{
    PrivateKey: os.Getenv("HL_PRIVATE_KEY"), AccountAddr: os.Getenv("HL_ADDR"),
})

// 所有适配器暴露完全相同的 Exchange 接口
ticker, _ := adp.FetchTicker(ctx, "BTC")
```

### 报价币种

每个适配器支持 `QuoteCurrency` 选项，用于指定连接哪个报价币种市场。省略时使用交易所默认值（CEX → USDT，DEX → USDC）。

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
| | `FetchOrder(ctx, orderID, symbol)` | 查询订单状态 |
| | `FetchOpenOrders(ctx, symbol)` | 查询所有挂单 |
| **账户** | `FetchAccount(ctx)` | 完整账户：余额 + 持仓 + 挂单 |
| | `FetchBalance(ctx)` | 仅查可用余额 |
| | `FetchSymbolDetails(ctx, symbol)` | 精度和最小数量规则 |
| | `FetchFeeRate(ctx, symbol)` | Maker/Taker 费率 |
| **深度簿** | `WatchOrderBook(ctx, symbol, cb)` | 订阅 WS 深度（阻塞至就绪） |
| | `GetLocalOrderBook(symbol, depth)` | 读取本地维护的深度簿 |
| | `StopWatchOrderBook(ctx, symbol)` | 取消订阅 |
| **实时流** | `WatchOrders(ctx, cb)` | 实时订单更新 |
| | `WatchPositions(ctx, cb)` | 实时仓位更新 |
| | `WatchTicker(ctx, symbol, cb)` | 实时行情 |
| | `WatchTrades(ctx, symbol, cb)` | 实时成交 |
| | `WatchKlines(ctx, symbol, interval, cb)` | 实时 K 线 |

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

    // 获取交易所原始错误信息
    var exErr *exchanges.ExchangeError
    if errors.As(err, &exErr) {
        fmt.Printf("[%s] Code: %s, Message: %s\n", exErr.Exchange, exErr.Code, exErr.Message)
    }
}
```

可用哨兵错误：`ErrInsufficientBalance`、`ErrRateLimited`、`ErrInvalidPrecision`、`ErrOrderNotFound`、`ErrSymbolNotFound`、`ErrMinNotional`、`ErrMinQuantity`、`ErrAuthFailed`、`ErrNetworkTimeout`、`ErrNotSupported`。

---

## 内置安全机制

### 限流

每个适配器预配置了交易所特定的限流规则。滑动窗口限流器支持：
- 多时间窗口（如 1200 次/分 + 10 次/秒）
- 分类权重（不同端点消耗不同配额）
- 自动阻塞直到配额可用

### IP 封禁检测与自动恢复

当交易所返回封禁错误（HTTP 418/429 + retry-after）时，适配器会：
1. 自动解析封禁时长
2. 阻塞所有后续请求直到封禁到期
3. 自动恢复正常运行 — 无需人工干预

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
├── base_adapter.go         共享适配器逻辑（限流、封禁、深度簿、校验）
├── local_state.go          LocalOrderBook 接口 + AccountManager
├── log.go                  Logger 接口 + NopLogger
├── ratelimit/              声明式滑动窗口限流器
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

复制环境变量模板并填入你的 API 凭证：
```bash
cp .env.example .env
```

运行单元测试（无需 API Key）：
```bash
go test ./ratelimit/ -v
go test . -run TestBan -v
go test -run "Test(Options|Format|Extract)" ./binance/ ./okx/ ./aster/ ./grvt/ -v  # 报价币种测试
```

运行集成测试（需要 `.env` 中的 API Key）：
```bash
go test ./binance/ -v      # 若未配置 Key 会自动跳过
go test ./grvt/ -v
go test ./edgex/ -v
```

## 许可证

MIT
