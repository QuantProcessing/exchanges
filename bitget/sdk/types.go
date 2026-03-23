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

type Instrument struct {
	Symbol             string `json:"symbol"`
	Category           string `json:"category"`
	BaseCoin           string `json:"baseCoin"`
	QuoteCoin          string `json:"quoteCoin"`
	MinOrderQty        string `json:"minOrderQty"`
	MaxOrderQty        string `json:"maxOrderQty"`
	MinOrderAmount     string `json:"minOrderAmount"`
	PricePrecision     string `json:"pricePrecision"`
	QuantityPrecision  string `json:"quantityPrecision"`
	QuotePrecision     string `json:"quotePrecision"`
	PriceMultiplier    string `json:"priceMultiplier"`
	QuantityMultiplier string `json:"quantityMultiplier"`
	MakerFeeRate       string `json:"makerFeeRate"`
	TakerFeeRate       string `json:"takerFeeRate"`
	FundInterval       string `json:"fundInterval"`
	Status             string `json:"status"`
}

type PlaceOrderRequest struct {
	Category    string `json:"category"`
	Symbol      string `json:"symbol"`
	Qty         string `json:"qty"`
	Price       string `json:"price,omitempty"`
	Side        string `json:"side"`
	TradeSide   string `json:"tradeSide,omitempty"`
	OrderType   string `json:"orderType"`
	TimeInForce string `json:"timeInForce,omitempty"`
	MarginMode  string `json:"marginMode,omitempty"`
	MarginCoin  string `json:"marginCoin,omitempty"`
	ClientOID   string `json:"clientOid,omitempty"`
	ReduceOnly  string `json:"reduceOnly,omitempty"`
}

type PlaceOrderResponse struct {
	OrderID   string `json:"orderId"`
	ClientOID string `json:"clientOid"`
}

type CancelOrderRequest struct {
	Category  string `json:"category"`
	Symbol    string `json:"symbol"`
	OrderID   string `json:"orderId,omitempty"`
	ClientOID string `json:"clientOid,omitempty"`
}

type CancelOrderResponse struct {
	OrderID   string `json:"orderId"`
	ClientOID string `json:"clientOid"`
}

type CancelAllOrdersRequest struct {
	Category string `json:"category"`
	Symbol   string `json:"symbol"`
}

type ModifyOrderRequest struct {
	Category    string `json:"category"`
	Symbol      string `json:"symbol"`
	OrderID     string `json:"orderId,omitempty"`
	ClientOID   string `json:"clientOid,omitempty"`
	NewQty      string `json:"newQty,omitempty"`
	NewPrice    string `json:"newPrice,omitempty"`
	NewClientID string `json:"newClientOid,omitempty"`
}

type OrderRecord struct {
	OrderID      string      `json:"orderId"`
	ClientOID    string      `json:"clientOid"`
	Symbol       string      `json:"symbol"`
	Category     string      `json:"category"`
	Side         string      `json:"side"`
	OrderType    string      `json:"orderType"`
	TimeInForce  string      `json:"timeInForce"`
	Price        string      `json:"price"`
	Qty          string      `json:"qty"`
	Amount       string      `json:"amount"`
	BaseVolume   string      `json:"baseVolume"`
	FilledQty    string      `json:"filledQty"`
	FilledVolume string      `json:"filledVolume"`
	CumExecQty   string      `json:"cumExecQty"`
	CumExecValue string      `json:"cumExecValue"`
	OrderStatus  string      `json:"orderStatus"`
	ReduceOnly   string      `json:"reduceOnly"`
	PosSide      string      `json:"posSide"`
	HoldSide     string      `json:"holdSide"`
	HoldMode     string      `json:"holdMode"`
	TradeSide    string      `json:"tradeSide"`
	MarginMode   string      `json:"marginMode"`
	MarginCoin   string      `json:"marginCoin"`
	AvgPrice     string      `json:"avgPrice"`
	Fee          string      `json:"fee"`
	TotalProfit  string      `json:"totalProfit"`
	CreatedTime  string      `json:"cTime"`
	UpdatedTime  string      `json:"uTime"`
	DelegateType string      `json:"delegateType"`
	StpMode      string      `json:"stpMode"`
	FeeDetail    []FeeDetail `json:"feeDetail"`
}

type OrderList struct {
	List  []OrderRecord `json:"list"`
	EndID string        `json:"endId"`
}

type AccountAsset struct {
	Coin      string `json:"coin"`
	Available string `json:"available"`
	Frozen    string `json:"frozen"`
	Locked    string `json:"locked"`
	Equity    string `json:"equity"`
	USDTValue string `json:"usdtValue"`
	Bonus     string `json:"bonus"`
}

type AccountAssets struct {
	AccountEquity    string         `json:"accountEquity"`
	UsdtEquity       string         `json:"usdtEquity"`
	Available        string         `json:"available"`
	UnrealizedPL     string         `json:"unrealizedPL"`
	Coupon           string         `json:"coupon"`
	UnionTotalMargin string         `json:"unionTotalMargin"`
	Assets           []AccountAsset `json:"assets"`
}

type PositionRecord struct {
	Symbol           string `json:"symbol"`
	Category         string `json:"category"`
	PosSide          string `json:"posSide"`
	HoldSide         string `json:"holdSide"`
	Qty              string `json:"qty"`
	Total            string `json:"total"`
	Size             string `json:"size"`
	Available        string `json:"available"`
	Frozen           string `json:"frozen"`
	AverageOpenPrice string `json:"averageOpenPrice"`
	OpenPriceAvg     string `json:"openPriceAvg"`
	AvgPrice         string `json:"avgPrice"`
	MarkPrice        string `json:"markPrice"`
	LiquidationPrice string `json:"liquidationPrice"`
	LiqPrice         string `json:"liqPrice"`
	Leverage         string `json:"leverage"`
	MarginMode       string `json:"marginMode"`
	UnrealizedPL     string `json:"unrealizedPL"`
	AchievedProfits  string `json:"achievedProfits"`
	CurRealisedPnl   string `json:"curRealisedPnl"`
	PositionStatus   string `json:"positionStatus"`
	CreatedTime      string `json:"createdTime"`
	UpdatedTime      string `json:"updatedTime"`
}

type PositionList struct {
	List []PositionRecord `json:"list"`
}

type SetLeverageRequest struct {
	Symbol   string `json:"symbol"`
	Category string `json:"category"`
	Leverage string `json:"leverage"`
}

type FeeDetail struct {
	FeeCoin string `json:"feeCoin"`
	Fee     string `json:"fee"`
}

type Ticker struct {
	Category     string `json:"category"`
	Symbol       string `json:"symbol"`
	Timestamp    string `json:"ts"`
	LastPrice    string `json:"lastPrice"`
	OpenPrice24h string `json:"openPrice24h"`
	HighPrice24h string `json:"highPrice24h"`
	LowPrice24h  string `json:"lowPrice24h"`
	Ask1Price    string `json:"ask1Price"`
	Bid1Price    string `json:"bid1Price"`
	Ask1Size     string `json:"ask1Size"`
	Bid1Size     string `json:"bid1Size"`
	Volume24h    string `json:"volume24h"`
	Turnover24h  string `json:"turnover24h"`
	IndexPrice   string `json:"indexPrice"`
	MarkPrice    string `json:"markPrice"`
	FundingRate  string `json:"fundingRate"`
}

type OrderBook struct {
	Asks [][]NumberString `json:"a"`
	Bids [][]NumberString `json:"b"`
	TS   string           `json:"ts"`
}

type PublicFill struct {
	ExecID     string `json:"execId"`
	ExecLinkID string `json:"execLinkId"`
	Price      string `json:"price"`
	Size       string `json:"size"`
	Side       string `json:"side"`
	Timestamp  string `json:"ts"`
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

func jsonArrayUnmarshal(data []byte, out any) error {
	return json.Unmarshal(data, out)
}
