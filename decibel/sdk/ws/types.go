package ws

import (
	"strings"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/shopspring/decimal"
)

type DepthUpdateType string

const (
	DepthUpdateSnapshot DepthUpdateType = "snapshot"
	DepthUpdateDelta    DepthUpdateType = "delta"
)

type DepthLevel struct {
	Price decimal.Decimal `json:"price"`
	Size  decimal.Decimal `json:"size"`
}

type MarketDepthMessage struct {
	Topic      string          `json:"topic"`
	Market     string          `json:"market"`
	UpdateType DepthUpdateType `json:"update_type,omitempty"`
	Bids       []DepthLevel    `json:"bids"`
	Asks       []DepthLevel    `json:"asks"`
	Timestamp  int64           `json:"timestamp,omitempty"`
}

type UserOrderHistoryMessage struct {
	Topic  string             `json:"topic"`
	Market string             `json:"market,omitempty"`
	Orders []OrderHistoryItem `json:"orders,omitempty"`
}

type OrderUpdateMessage struct {
	Topic string            `json:"topic"`
	Order OrderUpdateRecord `json:"order"`
}

type OrderUpdateRecord struct {
	Status           string                `json:"status,omitempty"`
	Details          string                `json:"details,omitempty"`
	NormalizedStatus exchanges.OrderStatus `json:"-"`
	Order            OrderUpdateItem       `json:"order"`
}

type OrderUpdateItem struct {
	ClientOrderID      string          `json:"client_order_id,omitempty"`
	Market             string          `json:"market,omitempty"`
	OrderID            string          `json:"order_id,omitempty"`
	OrderType          string          `json:"order_type,omitempty"`
	OrderDirection     string          `json:"order_direction,omitempty"`
	Side               string          `json:"side,omitempty"`
	IsBuy              bool            `json:"is_buy,omitempty"`
	Status             string          `json:"status,omitempty"`
	Details            string          `json:"details,omitempty"`
	CancellationReason string          `json:"cancellation_reason,omitempty"`
	IsReduceOnly       bool            `json:"is_reduce_only,omitempty"`
	UnixMS             int64           `json:"unix_ms,omitempty"`
	OrigSize           decimal.Decimal `json:"orig_size,omitempty"`
	Price              decimal.Decimal `json:"price,omitempty"`
	RemainingSize      decimal.Decimal `json:"remaining_size,omitempty"`
	SizeDelta          decimal.Decimal `json:"size_delta,omitempty"`
}

type OrderHistoryItem struct {
	OrderID          string                `json:"order_id,omitempty"`
	ClientOrderID    string                `json:"client_order_id,omitempty"`
	Status           string                `json:"status,omitempty"`
	NormalizedStatus exchanges.OrderStatus `json:"-"`
	OrderType        string                `json:"order_type,omitempty"`
	Side             string                `json:"side,omitempty"`
	Price            decimal.Decimal       `json:"price,omitempty"`
	OrigSize         decimal.Decimal       `json:"orig_size,omitempty"`
	RemainingSize    decimal.Decimal       `json:"remaining_size,omitempty"`
	SizeDelta        decimal.Decimal       `json:"size_delta,omitempty"`
	UnixMS           int64                 `json:"unix_ms,omitempty"`
}

type UserPositionsMessage struct {
	Topic     string     `json:"topic"`
	Positions []Position `json:"positions,omitempty"`
}

type Position struct {
	Market     string          `json:"market,omitempty"`
	Size       decimal.Decimal `json:"size,omitempty"`
	EntryPrice decimal.Decimal `json:"entry_price,omitempty"`
	Side       string          `json:"side,omitempty"`
}

type subscriptionRequest struct {
	Subscribe subscriptionTopic `json:"Subscribe"`
}

type subscriptionTopic struct {
	Topic string `json:"topic"`
}

type pingRequest struct {
	Type string `json:"type"`
}

func (m MarketDepthMessage) IsDelta() bool {
	return strings.EqualFold(string(m.UpdateType), string(DepthUpdateDelta))
}

func NormalizeOrderStatus(status string) exchanges.OrderStatus {
	normalized := strings.NewReplacer("_", "", "-", "", " ", "").Replace(strings.ToUpper(strings.TrimSpace(status)))

	switch normalized {
	case "PENDING":
		return exchanges.OrderStatusPending
	case "PLACED", "OPEN", "NEW":
		return exchanges.OrderStatusNew
	case "PARTIALLYFILLED", "PARTIALFILL", "PARTIAL":
		return exchanges.OrderStatusPartiallyFilled
	case "FILLED", "EXECUTED":
		return exchanges.OrderStatusFilled
	case "CANCELLED", "CANCELED", "EXPIRED":
		return exchanges.OrderStatusCancelled
	case "REJECTED", "FAILED":
		return exchanges.OrderStatusRejected
	default:
		return exchanges.OrderStatusUnknown
	}
}
