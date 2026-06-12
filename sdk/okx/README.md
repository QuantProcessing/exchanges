# OKX SDK 使用指南与核心概念

本文档旨在介绍 OKX V5 API 的核心概念，帮助开发者快速理解 `instId`、`tdMode` 等关键参数，以及如何将通用代币符号（如 BTC）转化为 OKX 可识别的交易代码。

## 1. 核心概念：产品 ID (Instrument ID - `instId`)

在 OKX V5 API 中，所有的交易标的（币对、合约）都通过 `instId` 唯一标识。这是你调用 `GetTicker`, `GetOrderBook`, `PlaceOrder` 等接口时必须传入的参数。

### 格式规则

OKX 的 `instId` 遵循特定的命名格式，取决于产品类型：

| 产品类型 (Product) | 格式规则 | 示例 (Example) | 说明 |
| :--- | :--- | :--- | :--- |
| **币币 (Spot)** | `币种-计价货币` | `BTC-USDT` | 现货交易，BTC 为币种，USDT 为计价货币。 |
| **永续合约 (Swap)** | `币种-计价货币-SWAP` | `BTC-USDT-SWAP` | USDT 本位永续合约。 |
| | `币种-USD-SWAP` | `BTC-USD-SWAP` | 币本位永续合约 (反向合约)。 |
| **交割合约 (Futures)** | `币种-计价货币-日期` | `BTC-USDT-250328` | USDT 本位交割合约，后6位为到期日(YYMMDD)。 |
| **期权 (Option)** | `币种-USD-日期-行权价-类型` | `BTC-USD-250328-100000-C` | BTC 期权，C 代表 Call (看涨)，P 代表 Put (看跌)。 |

### 常见问题：如何将 "BTC" 转化为 `instId`？

在程序化交易中，用户通常只输入 "BTC"。你需要根据**交易场景**来构建 `instId`。

**场景 1：我想买 BTC 现货**
你需要确定计价货币（通常是 USDT 或 USDC）。
- 转化公式: `Symbol` + `-` + `QuoteCurrency`
- 结果: `BTC` -> `BTC-USDT`

**场景 2：我想做多 BTC 永续合约**
你需要确定是 U 本位 (USDT Margin) 还是 币本位 (Coin Margin)。
- U 本位转化: `Symbol` + `-` + `USDT-SWAP` -> `BTC-USDT-SWAP`
- 币本位转化: `Symbol` + `-` + `USD-SWAP` -> `BTC-USD-SWAP`

> **提示**: 可以通过 `client.GetInstruments(ctx, "SWAP")` 获取所有支持的合约列表，建立本地缓存来辅助查找。

## 2. 核心概念：交易模式 (Trade Mode - `tdMode`)

下单 (`PlaceOrder`) 时，`tdMode` 决定了保证金的使用方式。

- **`cash`**: **非保证金模式 (现货默认)**。
  - 仅用于非杠杆的现货交易 (`instType=SPOT`)。
  - 账户里有多少币就买多少，无爆仓风险。
  
- **`cross`**: **全仓模式**。
  - 保证金由账户内所有资产共享。风险共担。
  - 适用于合约及杠杆交易。

- **`isolated`**: **逐仓模式**。
  - 保证金独立于该仓位，亏损仅限于该仓位分配的资金。
  - 下单时较为常用，便于风险控制。

## 3. 核心概念：持仓方向 (Position Side - `posSide`)

在合约交易（特别是 `双向持仓` 模式下）中，必须指定 `posSide`。

- **`long`**: 开多 / 平多
- **`short`**: 开空 / 平空
- **`net`**: **单向持仓模式** (默认或现货)。
  - 现货交易 (`cash`) 只有 `net` 概念（实际上 API 请求时如果不传，默认为买卖币本身）。
  - 如果你的账户配置为“单向持仓模式”，合约也使用 `net`。

## 4. 快速开始示例

### 1. 查询 BTC 价格
```go
// 确定你要查的是现货还是合约
instId := "BTC-USDT" // 现货
// instId := "BTC-USDT-SWAP" // 永续

ticker, err := client.GetTicker(ctx, instId)
fmt.Println("最新价:", ticker[0].Last)
```

### 2. 下一笔 BTC 现货买单
```go
req := &okx.OrderRequest{
    InstId:  "BTC-USDT",
    TdMode:  "cash",    // 现货非杠杆一定要用 cash
    Side:    "buy",
    OrdType: "limit",   // 限价单
    Px:      "95000",   // 价格
    Sz:      "0.1",     // 数量 (BTC)
}
resp, err := client.PlaceOrder(ctx, req)
```

### 3. 下一笔 BTC 永续合约开多单 (逐仓)
```go
req := &okx.OrderRequest{
    InstId:  "BTC-USDT-SWAP",
    TdMode:  "isolated", // 逐仓
    Side:    "buy",      // 买入
    PosSide: "long",     // 开多 (双向持仓模式必填)
    OrdType: "market",   // 市价单
    Sz:      "1",        // 数量 (张数，注意：合约通常是按"张"计算，1张=0.01 BTC 或 0.001 BTC，需查询 Instrument info)
}
resp, err := client.PlaceOrder(ctx, req)
```

## 5. 单位换算注意 (Size/Quantity)

OKX 的数量 (`Sz`) 含义随产品不同而不同：
- **现货**: 交易币种的数量（例如 `1` 代表 1 BTC）。
- **合约**: **“张” (Contracts)**。
  - 你需要通过 `GetInstruments` 接口查询 `ctVal` (合约面值) 和 `ctMult` (乘数)。
  - 例如 `BTC-USDT-SWAP`，1 张可能等于 0.01 BTC。如果你想开 1 BTC 的仓位，`Sz` 应该传 `100`。

## 6. 进阶概念：产品类型 (instType) vs 交易品种 (instFamily)

在调用 `GetTickers` 或查询公共数据时，经常会遇到这两个参数。

### 产品类型 (instType)
这是最顶层的分类，决定了你是在哪个"市场"交易。
- 常见值：`SPOT` (现货), `SWAP` (永续合约), `FUTURES` (交割合约), `OPTION` (期权), `MARGIN` (币币杠杆)。
- 当你调用接口时，`instType` 通常是必填项或核心过滤条件。
- 例子：我想看所有**永续合约**的行情 -> `instType = SWAP`

### 交易品种 (instFamily)
这是在 `instType` 之下的二级分类，通常指**标的资产 (Underlying)**。
- 常见值：`BTC-USD` (币本位及其合约), `BTC-USDT` (U本位及其合约), `ETH-USD` 等。
- `instFamily` 将同一标的下的不同到期日合约（如当周、次周、季度）归纳在一起，方便聚合查询。
- 例子：我想看所有 **BTC U本位永续合约** 的行情 -> `instType = SWAP`, `instFamily = BTC-USDT`

**总结区别**：
- **`instType`** 决定了玩法（是买现货还是玩合约）。
- **`instFamily`** 决定了标的物（是玩 BTC 还是 ETH）。
- **`instId`** 是最终具体的那个交易对/合约（例如 `BTC-USDT-SWAP`）。
