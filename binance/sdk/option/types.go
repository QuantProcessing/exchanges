package option

import (
	"encoding/json"
	"strconv"
	"strings"
)

type APIError struct {
	Code    int    `json:"code"`
	Message string `json:"msg"`
}

func (e *APIError) Error() string {
	return "binance option api error " + strconv.Itoa(e.Code) + ": " + e.Message
}

type NumberString string

func (n *NumberString) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	trimmed = strings.Trim(trimmed, `"`)
	*n = NumberString(trimmed)
	return nil
}

type ExchangeInfoResponse struct {
	OptionSymbols []OptionSymbol `json:"optionSymbols"`
}

type OptionSymbol struct {
	Symbol        string       `json:"symbol"`
	Status        string       `json:"status"`
	BaseAsset     string       `json:"baseAsset"`
	QuoteAsset    string       `json:"quoteAsset"`
	Underlying    string       `json:"underlying"`
	SettleAsset   string       `json:"settleAsset"`
	Side          string       `json:"side"`
	StrikePrice   string       `json:"strikePrice"`
	ExpiryDate    int64        `json:"expiryDate"`
	Unit          NumberString `json:"unit"`
	PriceScale    int32        `json:"priceScale"`
	QuantityScale int32        `json:"quantityScale"`
	MinQty        string       `json:"minQty"`
}

type DepthResponse struct {
	Time int64      `json:"T"`
	Bids [][]string `json:"bids"`
	Asks [][]string `json:"asks"`
}

type TickerResponse struct {
	Symbol      string `json:"symbol"`
	LastPrice   string `json:"lastPrice"`
	BidPrice    string `json:"bidPrice"`
	AskPrice    string `json:"askPrice"`
	OpenPrice   string `json:"openPrice"`
	HighPrice   string `json:"highPrice"`
	LowPrice    string `json:"lowPrice"`
	Volume      string `json:"volume"`
	Amount      string `json:"amount"`
	CloseTime   int64  `json:"closeTime"`
	OpenTime    int64  `json:"openTime"`
	TradeCount  int64  `json:"tradeCount"`
	StrikePrice string `json:"strikePrice"`
}

type TradeResponse struct {
	ID       json.Number `json:"id"`
	TradeID  json.Number `json:"tradeId"`
	Price    string      `json:"price"`
	Quantity string      `json:"qty"`
	Side     string      `json:"side"`
	Time     int64       `json:"time"`
}

type KlineResponse []interface{}

type OrderResponse struct {
	ID            NumberString `json:"id"`
	OrderID       NumberString `json:"orderId"`
	ClientOrderID string       `json:"clientOrderId"`
	Symbol        string       `json:"symbol"`
	Price         string       `json:"price"`
	Quantity      string       `json:"quantity"`
	ExecutedQty   string       `json:"executedQty"`
	Fee           string       `json:"fee"`
	Side          string       `json:"side"`
	Type          string       `json:"type"`
	TimeInForce   string       `json:"timeInForce"`
	Status        string       `json:"status"`
	AvgPrice      string       `json:"avgPrice"`
	ReduceOnly    bool         `json:"reduceOnly"`
	CreateDate    int64        `json:"createDate"`
	UpdateTime    int64        `json:"updateTime"`
}

func (o OrderResponse) IDString() string {
	if o.ID != "" {
		return string(o.ID)
	}
	return string(o.OrderID)
}

type PlaceOrderParams struct {
	Symbol        string
	Side          string
	Type          string
	Quantity      string
	Price         string
	TimeInForce   string
	ClientOrderID string
	ReduceOnly    bool
}

type CancelOrderParams struct {
	Symbol        string
	OrderID       string
	ClientOrderID string
}

type CancelAllOrdersParams struct {
	Symbol string
}
