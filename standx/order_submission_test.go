package standx

import (
	"testing"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestSubmittedOrderUsesClientOrderIDButLeavesOrderIDEmpty(t *testing.T) {
	t.Parallel()

	now := time.Unix(0, 0)
	params := &exchanges.OrderParams{
		Symbol:   "ETH",
		Side:     exchanges.OrderSideBuy,
		Type:     exchanges.OrderTypeLimit,
		Quantity: decimal.RequireFromString("0.01"),
		Price:    decimal.RequireFromString("2000"),
	}

	order := newSubmittedOrder(params, "client-123", now)

	require.Empty(t, order.OrderID)
	require.Equal(t, "client-123", order.ClientOrderID)
	require.Equal(t, exchanges.OrderStatusNew, order.Status)
	require.Equal(t, params.Symbol, order.Symbol)
}
