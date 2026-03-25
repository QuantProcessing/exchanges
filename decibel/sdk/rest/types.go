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
	Size                      decimal.Decimal `json:"size"`
	EntryPrice                decimal.Decimal `json:"entry_price"`
	EstimatedLiquidationPrice decimal.Decimal `json:"estimated_liquidation_price"`
	UnrealizedFunding         decimal.Decimal `json:"unrealized_funding"`
	UserLeverage              decimal.Decimal `json:"user_leverage"`
}

type OpenOrdersResponse struct {
	Items      []OpenOrder `json:"items"`
	NextCursor string      `json:"next_cursor"`
}

type OpenOrder struct {
	ClientOrderID string          `json:"client_order_id"`
	Market        string          `json:"market"`
	OrderID       string          `json:"order_id"`
	OrderType     string          `json:"order_type"`
	Status        string          `json:"status"`
	UnixMS        int64           `json:"unix_ms"`
	OrigSize      decimal.Decimal `json:"orig_size"`
	Price         decimal.Decimal `json:"price"`
	RemainingSize decimal.Decimal `json:"remaining_size"`
	SizeDelta     decimal.Decimal `json:"size_delta"`
}
