package exchanges

import "github.com/shopspring/decimal"

// ============================================================================
// Enums & Constants
// ============================================================================

// OrderStatus represents the state of an order.
type OrderStatus string

const (
	OrderStatusPending         OrderStatus = "PENDING"
	OrderStatusPartiallyFilled OrderStatus = "PARTIALLY_FILLED"
	OrderStatusFilled          OrderStatus = "FILLED"
	OrderStatusCancelled       OrderStatus = "CANCELLED"
	OrderStatusRejected        OrderStatus = "REJECTED"
	OrderStatusNew             OrderStatus = "NEW"
	OrderStatusUnknown         OrderStatus = "UNKNOWN"
)

// OrderSide represents the direction of an order (buy or sell).
type OrderSide string

const (
	OrderSideBuy  OrderSide = "BUY"
	OrderSideSell OrderSide = "SELL"
)

// OrderType represents the type of an order.
type OrderType string

const (
	OrderTypeLimit            OrderType = "LIMIT"
	OrderTypeMarket           OrderType = "MARKET"
	OrderTypeStopLossLimit    OrderType = "STOP_LOSS_LIMIT"
	OrderTypeTakeProfitLimit  OrderType = "TAKE_PROFIT_LIMIT"
	OrderTypeStopLossMarket   OrderType = "STOP_LOSS_MARKET"
	OrderTypeTakeProfitMarket OrderType = "TAKE_PROFIT_MARKET"
	OrderTypePostOnly         OrderType = "POST_ONLY"
	OrderTypeUnknown          OrderType = "UNKNOWN"
)

// TimeInForce specifies how long an order remains active.
type TimeInForce string

const (
	TimeInForceGTC TimeInForce = "GTC" // Good Till Cancel
	TimeInForceIOC TimeInForce = "IOC" // Immediate Or Cancel
	TimeInForceFOK TimeInForce = "FOK" // Fill Or Kill
	TimeInForcePO  TimeInForce = "PO"  // Post Only
)

// PositionSide represents the direction of a position.
type PositionSide string

const (
	PositionSideLong  PositionSide = "LONG"
	PositionSideShort PositionSide = "SHORT"
	PositionSideBoth  PositionSide = "BOTH" // For one-way mode
)

// Interval represents the candlestick/kline time period.
type Interval string

const (
	Interval1m  Interval = "1m"
	Interval3m  Interval = "3m"
	Interval5m  Interval = "5m"
	Interval15m Interval = "15m"
	Interval30m Interval = "30m"
	Interval1h  Interval = "1h"
	Interval2h  Interval = "2h"
	Interval4h  Interval = "4h"
	Interval6h  Interval = "6h"
	Interval8h  Interval = "8h"
	Interval12h Interval = "12h"
	Interval1d  Interval = "1d"
	Interval3d  Interval = "3d"
	Interval1w  Interval = "1w"
	Interval1M  Interval = "1M"
)

// TradeSide 成交方向 (for market trades)
type TradeSide string

const (
	TradeSideBuy  TradeSide = "buy"
	TradeSideSell TradeSide = "sell"
)

// MarketType represents the type of trading market.
type MarketType string

const (
	MarketTypeSpot MarketType = "spot" // Spot trading
	MarketTypePerp MarketType = "perp" // Perpetual futures
)

// AccountType represents an account type for asset transfers.
type AccountType string

const (
	AccountTypeSpot    AccountType = "SPOT"    // Spot account
	AccountTypePerp    AccountType = "PERP"    // Perpetual futures account
	AccountTypeUnified AccountType = "UNIFIED" // Unified account
)

// QuoteCurrency represents the quote/settlement currency of a trading pair.
type QuoteCurrency string

const (
	QuoteCurrencyUSDT QuoteCurrency = "USDT"
	QuoteCurrencyUSDC QuoteCurrency = "USDC"
	QuoteCurrencyDUSD QuoteCurrency = "DUSD"
)

// ============================================================================
// Data Structs
// ============================================================================

// Order represents a trading order with its current state.
type Order struct {
	OrderID        string          `json:"order_id"`
	Symbol         string          `json:"symbol"`
	Side           OrderSide       `json:"side"`
	Type           OrderType       `json:"type"`
	Quantity       decimal.Decimal `json:"quantity"`
	Price          decimal.Decimal `json:"price,omitempty"`
	Status         OrderStatus     `json:"status"`
	FilledQuantity decimal.Decimal `json:"filled_quantity"`
	Timestamp      int64           `json:"timestamp"`
	Fee            decimal.Decimal `json:"fee,omitempty"`
	ClientOrderID  string          `json:"client_order_id,omitempty"`
	ReduceOnly     bool            `json:"reduce_only,omitempty"`
	TimeInForce    TimeInForce     `json:"time_in_force,omitempty"`
}

// Position represents an open position in a perpetual futures market.
type Position struct {
	Symbol            string          `json:"symbol"`
	Side              PositionSide    `json:"side"`
	Quantity          decimal.Decimal `json:"quantity"`
	EntryPrice        decimal.Decimal `json:"entry_price"`
	UnrealizedPnL     decimal.Decimal `json:"unrealized_pnl"`
	RealizedPnL       decimal.Decimal `json:"realized_pnl"`
	LiquidationPrice  decimal.Decimal `json:"liquidation_price,omitempty"`
	Leverage          decimal.Decimal `json:"leverage,omitempty"`
	MaintenanceMargin decimal.Decimal `json:"maintenance_margin,omitempty"`
	MarginType        string          `json:"margin_type,omitempty"` // ISOLATED or CROSSED
}

// Account represents a trading account summary.
type Account struct {
	TotalBalance     decimal.Decimal `json:"total_balance"`
	AvailableBalance decimal.Decimal `json:"available_balance"`
	Positions        []Position      `json:"positions"`
	Orders           []Order         `json:"orders"` // Open orders
	UnrealizedPnL    decimal.Decimal `json:"unrealized_pnl"`
	RealizedPnL      decimal.Decimal `json:"realized_pnl"`
}

// Ticker represents real-time market data for a symbol.
type Ticker struct {
	Symbol     string          `json:"symbol"`
	LastPrice  decimal.Decimal `json:"last_price"`
	IndexPrice decimal.Decimal `json:"index_price"`
	MarkPrice  decimal.Decimal `json:"mark_price"`
	MidPrice   decimal.Decimal `json:"mid_price"`
	Bid        decimal.Decimal `json:"bid"` // Best Bid
	Ask        decimal.Decimal `json:"ask"` // Best Ask
	Volume24h  decimal.Decimal `json:"volume_24h"`
	QuoteVol   decimal.Decimal `json:"quote_vol"` // Quote Volume
	High24h    decimal.Decimal `json:"high_24h"`
	Low24h     decimal.Decimal `json:"low_24h"`
	Timestamp  int64           `json:"timestamp"`
}

// Level represents a single price level in the order book.
type Level struct {
	Price    decimal.Decimal `json:"price"`
	Quantity decimal.Decimal `json:"quantity"`
}

// OrderBook represents a snapshot of the order book.
type OrderBook struct {
	Symbol    string  `json:"symbol"`
	Bids      []Level `json:"bids"` // Sorted descending by price
	Asks      []Level `json:"asks"` // Sorted ascending by price
	Timestamp int64   `json:"timestamp"`
}

// Kline represents a single candlestick/OHLCV bar.
type Kline struct {
	Symbol    string          `json:"symbol"`
	Interval  Interval        `json:"interval"`
	Open      decimal.Decimal `json:"open"`
	High      decimal.Decimal `json:"high"`
	Low       decimal.Decimal `json:"low"`
	Close     decimal.Decimal `json:"close"`
	Volume    decimal.Decimal `json:"volume"`    // Base currency volume
	QuoteVol  decimal.Decimal `json:"quote_vol"` // Quote currency volume
	Timestamp int64           `json:"timestamp"` // Open time in milliseconds
}

// Trade represents a single market trade.
type Trade struct {
	ID        string          `json:"id"`
	Symbol    string          `json:"symbol"`
	Price     decimal.Decimal `json:"price"`
	Quantity  decimal.Decimal `json:"quantity"`
	Side      TradeSide       `json:"side"`
	Timestamp int64           `json:"timestamp"` // Milliseconds
}

// SymbolDetails provides trading rules and precision for a symbol.
type SymbolDetails struct {
	Symbol            string          `json:"symbol"`
	PricePrecision    int32           `json:"price_precision"`    // Decimal places for price
	QuantityPrecision int32           `json:"quantity_precision"` // Decimal places for quantity
	MinQuantity       decimal.Decimal `json:"min_quantity"`       // Minimum order quantity
	MinNotional       decimal.Decimal `json:"min_notional"`       // Minimum notional value (price * qty)
}

// FeeRate represents maker/taker fee rates for a symbol.
type FeeRate struct {
	Maker decimal.Decimal `json:"maker"`
	Taker decimal.Decimal `json:"taker"`
}

// FundingRate represents the funding rate for a perpetual futures symbol.
type FundingRate struct {
	Symbol               string          `json:"symbol"`
	FundingRate          decimal.Decimal `json:"funding_rate"`
	FundingIntervalHours int64           `json:"funding_interval_hours"`
	FundingTime          int64           `json:"funding_time"`
	NextFundingTime      int64           `json:"next_funding_time"`
	UpdateTime           int64           `json:"update_time"`
}

// SpotBalance represents the balance of a single asset in a spot account.
type SpotBalance struct {
	Asset  string          `json:"asset"`  // Currency symbol, e.g. "BTC", "USDT"
	Free   decimal.Decimal `json:"free"`   // Available balance
	Locked decimal.Decimal `json:"locked"` // Frozen/locked balance
	Total  decimal.Decimal `json:"total"`  // Total balance (Free + Locked)
}

type TransferParams struct {
	Asset       string          // Currency symbol
	Amount      decimal.Decimal // Transfer amount
	FromAccount AccountType     // Source account type
	ToAccount   AccountType     // Destination account type
}

// MarginAsset represents a single asset in a margin account.
type MarginAsset struct {
	Asset    string          `json:"asset"`
	Borrowed decimal.Decimal `json:"borrowed"`
	Free     decimal.Decimal `json:"free"`
	Interest decimal.Decimal `json:"interest"`
	Locked   decimal.Decimal `json:"locked"`
	NetAsset decimal.Decimal `json:"net_asset"`
}

// MarginAccount represents a cross-margin account summary.
type MarginAccount struct {
	MarginLevel       decimal.Decimal `json:"margin_level"`
	TotalAssetBTC     decimal.Decimal `json:"total_asset_btc"`
	TotalLiabilityBTC decimal.Decimal `json:"total_liability_btc"`
	TotalNetAssetBTC  decimal.Decimal `json:"total_net_asset_btc"`
	UserAssets        []MarginAsset   `json:"user_assets"`
}

// IsolatedMarginAsset represents a single asset in an isolated margin account.
type IsolatedMarginAsset struct {
	Asset         string          `json:"asset"`
	BorrowEnabled bool            `json:"borrow_enabled"`
	Borrowed      decimal.Decimal `json:"borrowed"`
	Free          decimal.Decimal `json:"free"`
	Interest      decimal.Decimal `json:"interest"`
	Locked        decimal.Decimal `json:"locked"`
	NetAsset      decimal.Decimal `json:"net_asset"`
	TotalAsset    decimal.Decimal `json:"total_asset"`
}

// IsolatedMarginSymbol represents an isolated margin trading pair.
type IsolatedMarginSymbol struct {
	Symbol         string              `json:"symbol"`
	BaseAsset      IsolatedMarginAsset `json:"base_asset"`
	QuoteAsset     IsolatedMarginAsset `json:"quote_asset"`
	MarginLevel    decimal.Decimal     `json:"margin_level"`
	MarginRatio    decimal.Decimal     `json:"margin_ratio"`
	IndexPrice     decimal.Decimal     `json:"index_price"`
	LiquidatePrice decimal.Decimal     `json:"liquidate_price"`
	LiquidateRate  decimal.Decimal     `json:"liquidate_rate"`
	Enabled        bool                `json:"enabled"`
}

// IsolatedMarginAccount represents an isolated margin account summary.
type IsolatedMarginAccount struct {
	Assets            []IsolatedMarginSymbol `json:"assets"`
	TotalAssetBTC     decimal.Decimal        `json:"total_asset_btc"`
	TotalLiabilityBTC decimal.Decimal        `json:"total_liability_btc"`
	TotalNetAssetBTC  decimal.Decimal        `json:"total_net_asset_btc"`
}

// ============================================================================
// Request Parameters
// ============================================================================

// OrderParams is the unified parameter struct for PlaceOrder.
type OrderParams struct {
	Symbol      string
	Side        OrderSide
	Type        OrderType // MARKET or LIMIT
	Quantity    decimal.Decimal
	Price       decimal.Decimal // Required for LIMIT; optional reference price for MARKET and slippage conversion
	TimeInForce TimeInForce     // Default: GTC for LIMIT
	ReduceOnly  bool
	Slippage    decimal.Decimal // If > 0 and Type == MARKET, auto-applies slippage logic
	ClientID    string          // Client-defined order ID
}

// ModifyOrderParams specifies parameters for modifying an existing order.
type ModifyOrderParams struct {
	Quantity decimal.Decimal
	Price    decimal.Decimal
}

type MarginType string

const (
	MarginTypeCrossed  MarginType = "CROSSED"
	MarginTypeIsolated MarginType = "ISOLATED"
	MarginTypeCash     MarginType = "CASH"
)
