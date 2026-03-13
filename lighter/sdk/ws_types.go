package lighter

// Callback is a function that processes WebSocket messages
type Callback func([]byte)

// Subscriber manages subscriptions for a channel
type Subscriber struct {
	Channel   string
	Callbacks []Callback
}

func (s *Subscriber) Dispatch(data []byte) {
	for _, cb := range s.Callbacks {
		cb(data)
	}
}

// MsgDispatcher interface for handling different message types
type MsgDispatcher interface {
	Dispatch(subscribers map[string]*Subscriber, msg []byte) error
}

// SubscribeRequest represents a WebSocket subscription request
type SubscribeRequest struct {
	Type    string  `json:"type"`
	Channel string  `json:"channel"`
	Auth    *string `json:"auth,omitempty"`
}

// WsOrderBookEvent represents an order book update
type WsOrderBookEvent struct {
	Channel   string `json:"channel"`
	Offset    int64  `json:"offset"`
	Type      string `json:"type"`
	OrderBook struct {
		Code       int64            `json:"code"`
		Asks       []OrderBookLevel `json:"asks"`
		Bids       []OrderBookLevel `json:"bids"`
		Offset     int64            `json:"offset"`
		Nonce      int64            `json:"nonce"`
		BeginNonce int64            `json:"begin_nonce"`
		Timestamp  int64            `json:"timestamp"`
	} `json:"order_book"`
}

// WsMarketStatsEvent represents market statistics update
type WsMarketStatsEvent struct {
	Channel     string `json:"channel"`
	Type        string `json:"type"`
	MarketStats struct {
		MarketId              uint8   `json:"market_id"`
		IndexPrice            string  `json:"index_price"`
		MarkPrice             string  `json:"mark_price"`
		OpenInterest          string  `json:"open_interest"`
		LastTradePrice        string  `json:"last_trade_price"`
		CurrentFundingRate    string  `json:"current_funding_rate"`
		FundingRate           string  `json:"funding_rate"`
		FundingTimestamp      int64   `json:"funding_timestamp"`
		DailyBaseTokenVolume  float64 `json:"daily_base_token_volume"`
		DailyQuoteTokenVolume float64 `json:"daily_quote_token_volume"`
		DailyPriceLow         float64 `json:"daily_price_low"`
		DailyPriceHigh        float64 `json:"daily_price_high"`
		DailyPriceChange      float64 `json:"daily_price_change"`
	} `json:"market_stats"`
}

// WsTradeEvent represents trade updates
type WsTradeEvent struct {
	Channel string  `json:"channel"`
	Type    string  `json:"type"`
	Trades  []Trade `json:"trades"`
}

// WsHeightEvent represents blockchain height updates
type WsHeightEvent struct {
	Channel string `json:"channel"`
	Type    string `json:"type"`
	Height  int64  `json:"height"`
}

// FundingHistory represents a funding payment
type FundingHistory struct {
	Timestamp    int64  `json:"timestamp"`
	MarketId     uint8  `json:"market_id"`
	FundingId    int64  `json:"funding_id"`
	Change       string `json:"change"`
	Rate         string `json:"rate"`
	PositionSize string `json:"position_size"`
	PositionSide string `json:"position_side"`
}

// WsAccountAllEvent represents all account data
type WsAccountAllEvent struct {
	Channel            string                      `json:"channel"`
	Type               string                      `json:"type"`
	Account            int64                       `json:"account"`
	DailyTradesCount   int64                       `json:"daily_trades_count"`
	DailyVolume        float64                     `json:"daily_volume"`
	WeeklyTradesCount  int64                       `json:"weekly_trades_count"`
	WeeklyVolume       float64                     `json:"weekly_volume"`
	MonthlyTradesCount int64                       `json:"monthly_trades_count"`
	MonthlyVolume      float64                     `json:"monthly_volume"`
	TotalTradesCount   int64                       `json:"total_trades_count"`
	TotalVolume        float64                     `json:"total_volume"`
	FundingHistories   map[string][]FundingHistory `json:"funding_histories"`
	Positions          map[string]*Position        `json:"positions"`
	Shares             []Share                     `json:"shares"`
	Trades             map[string][]Trade          `json:"trades"`
}

// WsAccountMarketEvent represents account data for a specific market
type WsAccountMarketEvent struct {
	Channel  string    `json:"channel"`
	Type     string    `json:"type"`
	Account  int64     `json:"account"`
	Position *Position `json:"position"`
	Orders   []*Order  `json:"orders"`
	Trades   []Trade   `json:"trades"`
}

// AccountStats represents account statistics
type AccountStats struct {
	Collateral       string `json:"collateral"`
	PortfolioValue   string `json:"portfolio_value"`
	Leverage         string `json:"leverage"`
	AvailableBalance string `json:"available_balance"`
	MarginUsage      string `json:"margin_usage"`
	BuyingPower      string `json:"buying_power"`
}

// WsUserStatsEvent represents user statistics update
type WsUserStatsEvent struct {
	Channel string `json:"channel"`
	Type    string `json:"type"`
	Stats   struct {
		Collateral       string       `json:"collateral"`
		PortfolioValue   string       `json:"portfolio_value"`
		Leverage         string       `json:"leverage"`
		AvailableBalance string       `json:"available_balance"`
		MarginUsage      string       `json:"margin_usage"`
		BuyingPower      string       `json:"buying_power"`
		CrossStats       AccountStats `json:"cross_stats"`
		TotalStats       AccountStats `json:"total_stats"`
	} `json:"stats"`
}

// WsAccountTxEvent represents account transaction updates
type WsAccountTxEvent struct {
	Channel string `json:"channel"`
	Type    string `json:"type"`
	Txs     []Tx   `json:"txs"`
}

// WsAccountAllOrdersEvent represents all account orders
type WsAccountAllOrdersEvent struct {
	Channel string              `json:"channel"`
	Type    string              `json:"type"`
	Orders  map[string][]*Order `json:"orders"`
}

// WsAccountOrdersEvent represents account orders for a specific market
type WsAccountOrdersEvent struct {
	Channel string              `json:"channel"`
	Type    string              `json:"type"`
	Account int64               `json:"account"`
	Nonce   int64               `json:"nonce"`
	Orders  map[string][]*Order `json:"orders"`
}

// NotificationContent can be different types based on notification kind
type NotificationContent map[string]interface{}

// Notification represents a notification
type Notification struct {
	Id           string              `json:"id"`
	CreatedAt    string              `json:"created_at"`
	UpdatedAt    string              `json:"updated_at"`
	Kind         string              `json:"kind"` // "liquidation", "deleverage", "announcement"
	AccountIndex int64               `json:"account_index"`
	Content      NotificationContent `json:"content"`
	Ack          bool                `json:"ack"`
	AckedAt      *string             `json:"acked_at"`
}

// WsNotificationEvent represents notification updates
type WsNotificationEvent struct {
	Channel string         `json:"channel"`
	Type    string         `json:"type"`
	Notifs  []Notification `json:"notifs"`
}

// WsAccountAllTradesEvent represents all account trades
type WsAccountAllTradesEvent struct {
	Channel string             `json:"channel"`
	Type    string             `json:"type"`
	Trades  map[string][]Trade `json:"trades"`
}

// WsAccountAllPositionsEvent represents all account positions
type WsAccountAllPositionsEvent struct {
	Channel   string               `json:"channel"`
	Type      string               `json:"type"`
	Positions map[string]*Position `json:"positions"`
}
