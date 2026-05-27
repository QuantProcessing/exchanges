package sdk

type Instrument struct {
	State                    string         `json:"state"`
	PriceIndex               string         `json:"price_index"`
	Kind                     string         `json:"kind"`
	InstrumentName           string         `json:"instrument_name"`
	InstrumentType           string         `json:"instrument_type"`
	FutureType               string         `json:"future_type"`
	SettlementCurrency       string         `json:"settlement_currency"`
	SettlementPeriod         string         `json:"settlement_period"`
	BaseCurrency             string         `json:"base_currency"`
	CounterCurrency          string         `json:"counter_currency"`
	QuoteCurrency            string         `json:"quote_currency"`
	OptionType               string         `json:"option_type"`
	MaxLeverage              int            `json:"max_leverage"`
	MakerCommission          float64        `json:"maker_commission"`
	TakerCommission          float64        `json:"taker_commission"`
	TickSize                 float64        `json:"tick_size"`
	ContractSize             float64        `json:"contract_size"`
	MinTradeAmount           float64        `json:"min_trade_amount"`
	Strike                   float64        `json:"strike"`
	ExpirationTimestamp      int64          `json:"expiration_timestamp"`
	CreationTimestamp        int64          `json:"creation_timestamp"`
	InstrumentID             int64          `json:"instrument_id"`
	IsActive                 bool           `json:"is_active"`
	TickSizeSteps            []TickSizeStep `json:"tick_size_steps"`
	BlockTradeCommission     float64        `json:"block_trade_commission"`
	BlockTradeMinTradeAmount float64        `json:"block_trade_min_trade_amount"`
	BlockTradeTickSize       float64        `json:"block_trade_tick_size"`
}

type TickSizeStep struct {
	AbovePrice float64 `json:"above_price"`
	TickSize   float64 `json:"tick_size"`
}

type Stats struct {
	High           float64 `json:"high"`
	Low            float64 `json:"low"`
	PriceChange    float64 `json:"price_change"`
	Volume         float64 `json:"volume"`
	VolumeUSD      float64 `json:"volume_usd"`
	VolumeNotional float64 `json:"volume_notional"`
}

type Ticker struct {
	InstrumentName         string  `json:"instrument_name"`
	State                  string  `json:"state"`
	BestAskAmount          float64 `json:"best_ask_amount"`
	BestAskPrice           float64 `json:"best_ask_price"`
	BestBidAmount          float64 `json:"best_bid_amount"`
	BestBidPrice           float64 `json:"best_bid_price"`
	CurrentFunding         float64 `json:"current_funding"`
	EstimatedDeliveryPrice float64 `json:"estimated_delivery_price"`
	Funding8h              float64 `json:"funding_8h"`
	IndexPrice             float64 `json:"index_price"`
	InterestValue          float64 `json:"interest_value"`
	LastPrice              float64 `json:"last_price"`
	MarkPrice              float64 `json:"mark_price"`
	MaxPrice               float64 `json:"max_price"`
	MinPrice               float64 `json:"min_price"`
	OpenInterest           float64 `json:"open_interest"`
	SettlementPrice        float64 `json:"settlement_price"`
	Timestamp              int64   `json:"timestamp"`
	Stats                  Stats   `json:"stats"`
}

type OrderBook struct {
	InstrumentName string      `json:"instrument_name"`
	Timestamp      int64       `json:"timestamp"`
	State          string      `json:"state"`
	IndexPrice     float64     `json:"index_price"`
	MarkPrice      float64     `json:"mark_price"`
	LastPrice      float64     `json:"last_price"`
	Bids           [][]float64 `json:"bids"`
	Asks           [][]float64 `json:"asks"`
	BestBidPrice   float64     `json:"best_bid_price"`
	BestBidAmount  float64     `json:"best_bid_amount"`
	BestAskPrice   float64     `json:"best_ask_price"`
	BestAskAmount  float64     `json:"best_ask_amount"`
	CurrentFunding float64     `json:"current_funding"`
	Funding8h      float64     `json:"funding_8h"`
	Stats          Stats       `json:"stats"`
}

type Trade struct {
	TradeSeq       int64   `json:"trade_seq"`
	TradeID        string  `json:"trade_id"`
	Timestamp      int64   `json:"timestamp"`
	TickDirection  int     `json:"tick_direction"`
	Price          float64 `json:"price"`
	MarkPrice      float64 `json:"mark_price"`
	IV             float64 `json:"iv"`
	InstrumentName string  `json:"instrument_name"`
	IndexPrice     float64 `json:"index_price"`
	Direction      string  `json:"direction"`
	Amount         float64 `json:"amount"`
	Contracts      float64 `json:"contracts"`
}

type TradesResult struct {
	Trades  []Trade `json:"trades"`
	HasMore bool    `json:"has_more"`
}

type TradingViewChartData struct {
	Status string    `json:"status"`
	Ticks  []int64   `json:"ticks"`
	Open   []float64 `json:"open"`
	High   []float64 `json:"high"`
	Low    []float64 `json:"low"`
	Close  []float64 `json:"close"`
	Volume []float64 `json:"volume"`
	Cost   []float64 `json:"cost"`
}

type AuthResult struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
}

type OrderRequest struct {
	InstrumentName string
	Amount         string
	Type           string
	Price          string
	TimeInForce    string
	Label          string
	ReduceOnly     bool
	PostOnly       bool
}

type OrderResult struct {
	Order OrderRecord `json:"order"`
}

type OrderRecord struct {
	OrderID        string  `json:"order_id"`
	InstrumentName string  `json:"instrument_name"`
	Direction      string  `json:"direction"`
	OrderType      string  `json:"order_type"`
	OrderState     string  `json:"order_state"`
	Amount         float64 `json:"amount"`
	FilledAmount   float64 `json:"filled_amount"`
	Price          float64 `json:"price"`
	AveragePrice   float64 `json:"average_price"`
	LastUpdateTime int64   `json:"last_update_timestamp"`
	UpdateTime     int64   `json:"-"`
	CreationTime   int64   `json:"creation_timestamp"`
	Label          string  `json:"label"`
	TimeInForce    string  `json:"time_in_force"`
	Commission     float64 `json:"commission"`
	ReduceOnly     bool    `json:"reduce_only"`
	Replaced       bool    `json:"replaced"`
	PostOnly       bool    `json:"post_only"`
}
