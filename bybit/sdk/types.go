package sdk

import (
	"encoding/json"
	"strings"
)

type NumberString string

func (n *NumberString) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	trimmed = strings.Trim(trimmed, `"`)
	*n = NumberString(trimmed)
	return nil
}

type responseEnvelope[T any] struct {
	RetCode int    `json:"retCode"`
	RetMsg  string `json:"retMsg"`
	Result  T      `json:"result"`
	Time    int64  `json:"time"`
}

type InstrumentsResult struct {
	Category       string       `json:"category"`
	List           []Instrument `json:"list"`
	NextPageCursor string       `json:"nextPageCursor"`
}

type Instrument struct {
	Symbol        string        `json:"symbol"`
	BaseCoin      string        `json:"baseCoin"`
	QuoteCoin     string        `json:"quoteCoin"`
	Status        string        `json:"status"`
	PriceScale    string        `json:"priceScale"`
	PriceFilter   PriceFilter   `json:"priceFilter"`
	LotSizeFilter LotSizeFilter `json:"lotSizeFilter"`
}

type PriceFilter struct {
	TickSize string `json:"tickSize"`
}

type LotSizeFilter struct {
	BasePrecision    string `json:"basePrecision"`
	QtyStep          string `json:"qtyStep"`
	MinOrderQty      string `json:"minOrderQty"`
	MinOrderAmt      string `json:"minOrderAmt"`
	MinNotionalValue string `json:"minNotionalValue"`
}

type TickersResult struct {
	Category string   `json:"category"`
	List     []Ticker `json:"list"`
}

type Ticker struct {
	Symbol       string `json:"symbol"`
	LastPrice    string `json:"lastPrice"`
	Bid1Price    string `json:"bid1Price"`
	Ask1Price    string `json:"ask1Price"`
	Volume24h    string `json:"volume24h"`
	Turnover24h  string `json:"turnover24h"`
	HighPrice24h string `json:"highPrice24h"`
	LowPrice24h  string `json:"lowPrice24h"`
	IndexPrice   string `json:"indexPrice"`
	MarkPrice    string `json:"markPrice"`
	Time         string `json:"time"`
	TS           string `json:"ts"`
}

type OrderBook struct {
	Symbol string           `json:"s"`
	Bids   [][]NumberString `json:"b"`
	Asks   [][]NumberString `json:"a"`
	TS     int64            `json:"ts"`
	U      int64            `json:"u"`
}

type PublicTradesResult struct {
	Category string        `json:"category"`
	List     []PublicTrade `json:"list"`
}

type PublicTrade struct {
	ExecID string `json:"execId"`
	Symbol string `json:"symbol"`
	Price  string `json:"price"`
	Size   string `json:"size"`
	Side   string `json:"side"`
	Time   string `json:"time"`
}

type KlinesResult struct {
	Category string   `json:"category"`
	Symbol   string   `json:"symbol"`
	List     []Candle `json:"list"`
}

type Candle [7]NumberString

func (c *Candle) UnmarshalJSON(data []byte) error {
	var raw []NumberString
	if err := jsonArrayUnmarshal(data, &raw); err != nil {
		return err
	}
	for i := 0; i < len(c) && i < len(raw); i++ {
		c[i] = raw[i]
	}
	return nil
}

type WalletBalanceResult struct {
	List []WalletAccount `json:"list"`
}

type WalletAccount struct {
	AccountType           string       `json:"accountType"`
	TotalEquity           string       `json:"totalEquity"`
	TotalAvailableBalance string       `json:"totalAvailableBalance"`
	TotalPerpUPL          string       `json:"totalPerpUPL"`
	TotalWalletBalance    string       `json:"totalWalletBalance"`
	Coin                  []WalletCoin `json:"coin"`
}

type WalletCoin struct {
	Coin           string `json:"coin"`
	Equity         string `json:"equity"`
	WalletBalance  string `json:"walletBalance"`
	Locked         string `json:"locked"`
	UnrealisedPnl  string `json:"unrealisedPnl"`
	CumRealisedPnl string `json:"cumRealisedPnl"`
	UsdValue       string `json:"usdValue"`
}

type FeeRatesResult struct {
	List []FeeRateRecord `json:"list"`
}

type FeeRateRecord struct {
	Symbol       string `json:"symbol"`
	MakerFeeRate string `json:"makerFeeRate"`
	TakerFeeRate string `json:"takerFeeRate"`
	BaseCoin     string `json:"baseCoin"`
}

type PositionsResult struct {
	NextPageCursor string           `json:"nextPageCursor"`
	List           []PositionRecord `json:"list"`
}

type PositionRecord struct {
	Symbol         string `json:"symbol"`
	Side           string `json:"side"`
	Size           string `json:"size"`
	AvgPrice       string `json:"avgPrice"`
	Leverage       string `json:"leverage"`
	UnrealisedPnl  string `json:"unrealisedPnl"`
	CumRealisedPnl string `json:"cumRealisedPnl"`
	LiqPrice       string `json:"liqPrice"`
}

type SetLeverageRequest struct {
	Category     string `json:"category"`
	Symbol       string `json:"symbol"`
	BuyLeverage  string `json:"buyLeverage"`
	SellLeverage string `json:"sellLeverage"`
}

type PlaceOrderRequest struct {
	Category              string `json:"category"`
	Symbol                string `json:"symbol"`
	Side                  string `json:"side"`
	OrderType             string `json:"orderType"`
	Qty                   string `json:"qty"`
	Price                 string `json:"price,omitempty"`
	TimeInForce           string `json:"timeInForce,omitempty"`
	ReduceOnly            bool   `json:"reduceOnly,omitempty"`
	OrderLinkID           string `json:"orderLinkId,omitempty"`
	MarketUnit            string `json:"marketUnit,omitempty"`
	SlippageToleranceType string `json:"slippageToleranceType,omitempty"`
	SlippageTolerance     string `json:"slippageTolerance,omitempty"`
}

type CancelOrderRequest struct {
	Category    string `json:"category"`
	Symbol      string `json:"symbol"`
	OrderID     string `json:"orderId,omitempty"`
	OrderLinkID string `json:"orderLinkId,omitempty"`
}

type CancelAllOrdersRequest struct {
	Category   string `json:"category"`
	Symbol     string `json:"symbol,omitempty"`
	BaseCoin   string `json:"baseCoin,omitempty"`
	SettleCoin string `json:"settleCoin,omitempty"`
}

type AmendOrderRequest struct {
	Category    string `json:"category"`
	Symbol      string `json:"symbol"`
	OrderID     string `json:"orderId,omitempty"`
	OrderLinkID string `json:"orderLinkId,omitempty"`
	Qty         string `json:"qty,omitempty"`
	Price       string `json:"price,omitempty"`
}

type OrderActionResponse struct {
	OrderID     string `json:"orderId"`
	OrderLinkID string `json:"orderLinkId"`
}

type OrdersResult struct {
	List           []OrderRecord `json:"list"`
	NextPageCursor string        `json:"nextPageCursor"`
}

type OrderRecord struct {
	OrderID            string `json:"orderId"`
	OrderLinkID        string `json:"orderLinkId"`
	Symbol             string `json:"symbol"`
	Side               string `json:"side"`
	OrderType          string `json:"orderType"`
	TimeInForce        string `json:"timeInForce"`
	Price              string `json:"price"`
	Qty                string `json:"qty"`
	CumExecQty         string `json:"cumExecQty"`
	AvgPrice           string `json:"avgPrice"`
	OrderStatus        string `json:"orderStatus"`
	ReduceOnly         bool   `json:"reduceOnly"`
	CreatedTime        string `json:"createdTime"`
	UpdatedTime        string `json:"updatedTime"`
	CumExecFee         string `json:"cumExecFee"`
	ClosedPnl          string `json:"closedPnl"`
	TriggerPrice       string `json:"triggerPrice"`
	LastPriceOnCreated string `json:"lastPriceOnCreated"`
}

type ExecutionRecord struct {
	ExecID      string `json:"execId"`
	OrderID     string `json:"orderId"`
	OrderLinkID string `json:"orderLinkId"`
	Symbol      string `json:"symbol"`
	Side        string `json:"side"`
	ExecPrice   string `json:"execPrice"`
	ExecQty     string `json:"execQty"`
	ExecFee     string `json:"execFee"`
	FeeCurrency string `json:"feeCurrency"`
	IsMaker     bool   `json:"isMaker"`
	ExecTime    string `json:"execTime"`
}

func jsonArrayUnmarshal(data []byte, out any) error {
	return json.Unmarshal(data, out)
}
