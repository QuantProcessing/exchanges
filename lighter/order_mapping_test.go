package lighter

import (
	exchanges "github.com/QuantProcessing/exchanges"
	sdklighter "github.com/QuantProcessing/exchanges/lighter/sdk"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPerpMapOrderHandlesExtendedTerminalStatuses(t *testing.T) {
	adp := &Adapter{
		idToSymbol: map[int]string{0: "ETH"},
	}

	cancelled := adp.mapOrder(&sdklighter.Order{
		MarketIndex:       0,
		OrderId:           "1",
		ClientOrderId:     "10",
		Status:            sdklighter.OrderStatusCanceledInvalidBalance,
		OrderType:         sdklighter.OrderTypeRespLimit,
		Price:             "100",
		InitialBaseAmount: "0.01",
		FilledBaseAmount:  "0",
	})
	require.Equal(t, exchanges.OrderStatusCancelled, cancelled.Status)

	pending := adp.mapOrder(&sdklighter.Order{
		MarketIndex:       0,
		OrderId:           "2",
		ClientOrderId:     "20",
		Status:            sdklighter.OrderStatusInProgress,
		OrderType:         sdklighter.OrderTypeRespMarket,
		Price:             "100",
		InitialBaseAmount: "0.01",
		FilledBaseAmount:  "0",
	})
	require.Equal(t, exchanges.OrderStatusPending, pending.Status)
}

func TestSpotMapOrderHandlesExtendedTerminalStatuses(t *testing.T) {
	adp := &SpotAdapter{
		idToSymbol: map[int]string{2048: "ETH"},
	}

	cancelled := adp.mapOrder(&sdklighter.Order{
		MarketIndex:       2048,
		OrderId:           "3",
		ClientOrderId:     "30",
		Status:            sdklighter.OrderStatusCanceledInvalidBalance,
		OrderType:         sdklighter.OrderTypeRespLimit,
		Price:             "100",
		InitialBaseAmount: "0.01",
		FilledBaseAmount:  "0",
	})
	require.Equal(t, exchanges.OrderStatusCancelled, cancelled.Status)

	pending := adp.mapOrder(&sdklighter.Order{
		MarketIndex:       2048,
		OrderId:           "4",
		ClientOrderId:     "40",
		Status:            sdklighter.OrderStatusInProgress,
		OrderType:         sdklighter.OrderTypeRespMarket,
		Price:             "100",
		InitialBaseAmount: "0.01",
		FilledBaseAmount:  "0",
	})
	require.Equal(t, exchanges.OrderStatusPending, pending.Status)
}

func TestSubmittedOrderUsesClientOrderIDButLeavesOrderIDEmpty(t *testing.T) {
	now := time.Unix(0, 0)
	params := &exchanges.OrderParams{
		Symbol:   "ETH",
		Side:     exchanges.OrderSideBuy,
		Type:     exchanges.OrderTypeMarket,
		Quantity: parseString("0.01"),
		Price:    parseString("2000"),
	}

	order := newSubmittedOrder(params, "12345", now)
	require.Empty(t, order.OrderID)
	require.Equal(t, "12345", order.ClientOrderID)
	require.Equal(t, exchanges.OrderStatusPending, order.Status)
	require.Equal(t, params.Symbol, order.Symbol)
}
