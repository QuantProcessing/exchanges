package standx

import (
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
)

func newSubmittedOrder(params *exchanges.OrderParams, clientOrderID string, now time.Time) *exchanges.Order {
	return &exchanges.Order{
		OrderID:       "",
		Symbol:        params.Symbol,
		Side:          params.Side,
		Type:          params.Type,
		Quantity:      params.Quantity,
		Price:         params.Price,
		Status:        exchanges.OrderStatusNew,
		ClientOrderID: clientOrderID,
		Timestamp:     now.UnixMilli(),
	}
}
