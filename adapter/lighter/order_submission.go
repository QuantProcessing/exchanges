package lighter

import (
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
)

func newSubmittedOrder(params *exchanges.OrderParams, clientOrderID string, now time.Time) *exchanges.Order {
	return &exchanges.Order{
		Symbol:        params.Symbol,
		Side:          params.Side,
		Type:          params.Type,
		Quantity:      params.Quantity,
		Price:         params.Price,
		Status:        exchanges.OrderStatusPending,
		Timestamp:     now.UnixMilli(),
		OrderID:       "",
		ClientOrderID: clientOrderID,
	}
}
