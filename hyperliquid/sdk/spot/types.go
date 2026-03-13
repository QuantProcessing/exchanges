package spot

import (
	"github.com/QuantProcessing/exchanges/hyperliquid/sdk"
)

// Request Types

type PlaceOrderRequest struct {
	AssetID       int
	IsBuy         bool
	Price         float64
	Size          float64
	OrderType     OrderType
	ClientOrderID *string
}

type OrderType struct {
	Limit   *OrderTypeLimit
	Trigger *OrderTypeTrigger
}

type OrderTypeLimit struct {
	Tif hyperliquid.Tif
}

type OrderTypeTrigger struct {
	IsMarket  bool
	TriggerPx float64
	Tpsl      hyperliquid.Tpsl
}

type CancelOrderRequest struct {
	AssetID int
	OrderID int64
}

// Response Types (Reused from generic or defined here if specific)
// Spot responses are likely similar to Perp "statuses".

type ModifyOrderRequest struct {
	Oid   *int64
	Cloid *string
	Order PlaceOrderRequest
}
