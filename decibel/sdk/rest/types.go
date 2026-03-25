package rest

import (
	"fmt"

	"github.com/shopspring/decimal"
)

type APIError struct {
	Code         string `json:"code"`
	Message      string `json:"message"`
	ErrorMessage string `json:"error"`
}

func (e *APIError) Error() string {
	if e == nil {
		return ""
	}
	if e.Code != "" {
		return fmt.Sprintf("decibel error %s: %s", e.Code, e.Message)
	}
	if e.Message != "" {
		return e.Message
	}
	return e.ErrorMessage
}

type Market struct {
	LotSize                 decimal.Decimal `json:"lot_size"`
	MarketAddr              string          `json:"market_addr"`
	MarketName              string          `json:"market_name"`
	MaxLeverage             decimal.Decimal `json:"max_leverage"`
	MaxOpenInterest         decimal.Decimal `json:"max_open_interest"`
	MinSize                 decimal.Decimal `json:"min_size"`
	Mode                    string          `json:"mode"`
	PxDecimals              int32           `json:"px_decimals"`
	SzDecimals              int32           `json:"sz_decimals"`
	TickSize                decimal.Decimal `json:"tick_size"`
	UnrealizedPnLHaircutBps int64           `json:"unrealized_pnl_haircut_bps"`
}

type AccountOverview struct {
	Account          string          `json:"account"`
	TotalBalance     decimal.Decimal `json:"total_balance"`
	AvailableBalance decimal.Decimal `json:"available_balance"`
	UnrealizedPnL    decimal.Decimal `json:"unrealized_pnl"`
	MarginBalance    decimal.Decimal `json:"margin_balance"`
}

type AccountPosition struct {
	Market                    string          `json:"market"`
	User                      string          `json:"user"`
	Side                      string          `json:"side"`
	Size                      decimal.Decimal `json:"size"`
	EntryPrice                decimal.Decimal `json:"entry_price"`
	EstimatedLiquidationPrice decimal.Decimal `json:"estimated_liquidation_price"`
	UnrealizedFunding         decimal.Decimal `json:"unrealized_funding"`
	UserLeverage              decimal.Decimal `json:"user_leverage"`
}

type Ticker struct {
	Market    string          `json:"market"`
	LastPrice decimal.Decimal `json:"last_price"`
	MarkPrice decimal.Decimal `json:"mark_price"`
	BidPrice  decimal.Decimal `json:"bid_price"`
	AskPrice  decimal.Decimal `json:"ask_price"`
	Timestamp int64           `json:"timestamp"`
}

type OrderBookLevel struct {
	Price decimal.Decimal `json:"price"`
	Size  decimal.Decimal `json:"size"`
}

type OrderBookSnapshot struct {
	Market    string           `json:"market"`
	Bids      []OrderBookLevel `json:"bids"`
	Asks      []OrderBookLevel `json:"asks"`
	Timestamp int64            `json:"timestamp"`
}

type OpenOrdersResponse struct {
	Items      []OpenOrder `json:"items"`
	TotalCount int         `json:"total_count"`
}

type OrderResponse struct {
	Status  string    `json:"status"`
	Details string    `json:"details"`
	Order   OpenOrder `json:"order"`
}

type OpenOrder struct {
	ClientOrderID      string          `json:"client_order_id"`
	Market             string          `json:"market"`
	OrderID            string          `json:"order_id"`
	OrderType          string          `json:"order_type"`
	OrderDirection     string          `json:"order_direction"`
	Side               string          `json:"side"`
	IsBuy              bool            `json:"is_buy"`
	Status             string          `json:"status"`
	Details            string          `json:"details"`
	CancellationReason string          `json:"cancellation_reason"`
	IsReduceOnly       bool            `json:"is_reduce_only"`
	UnixMS             int64           `json:"unix_ms"`
	OrigSize           decimal.Decimal `json:"orig_size"`
	Price              decimal.Decimal `json:"price"`
	RemainingSize      decimal.Decimal `json:"remaining_size"`
	SizeDelta          decimal.Decimal `json:"size_delta"`
}
