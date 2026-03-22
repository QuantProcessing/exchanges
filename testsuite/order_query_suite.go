package testsuite

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// OrderQueryConfig configures the shared order-query semantics suite.
type OrderQueryConfig struct {
	Symbol                 string
	SupportsOpenOrders     bool
	SupportsTerminalLookup bool
	SupportsOrderHistory   bool
}

// RunOrderQuerySemanticsSuite verifies the semantic split between:
// - FetchOrderByID: single-order lookup
// - FetchOrders: all visible orders for a symbol
// - FetchOpenOrders: open orders only
func RunOrderQuerySemanticsSuite(t *testing.T, adp exchanges.Exchange, cfg OrderQueryConfig) {
	t.Helper()

	updates := SetupOrderWatch(t, adp)

	t.Run("TerminalLookup", func(t *testing.T) {
		testFetchOrderByIDTerminalLookup(t, adp, cfg.Symbol, cfg.SupportsTerminalLookup, updates)
	})

	t.Run("OrdersVsOpenOrders", func(t *testing.T) {
		testFetchOrdersVsOpenOrders(t, adp, cfg.Symbol, cfg.SupportsOpenOrders, cfg.SupportsOrderHistory, updates)
	})

	t.Run("Cleanup", func(t *testing.T) {
		closeAllPositions(t, adp, cfg.Symbol, updates)
	})
}

func testFetchOrderByIDTerminalLookup(
	t *testing.T,
	adp exchanges.Exchange,
	symbol string,
	supportsTerminalLookup bool,
	updates <-chan *exchanges.Order,
) {
	t.Helper()

	qty, _ := SmartQuantity(t, adp, symbol)
	ctx := context.Background()

	order, err := adp.PlaceOrder(ctx, &exchanges.OrderParams{
		Symbol:   symbol,
		Side:     exchanges.OrderSideBuy,
		Type:     exchanges.OrderTypeMarket,
		Quantity: qty,
	})
	require.NoError(t, err)

	filled := WaitOrderStatus(t, updates, order.OrderID, order.ClientOrderID, exchanges.OrderStatusFilled, 30*time.Second)
	require.NotEmpty(t, filled.OrderID)

	lookup, err := adp.FetchOrderByID(ctx, filled.OrderID, symbol)
	switch {
	case supportsTerminalLookup:
		require.NoError(t, err)
		require.NotNil(t, lookup)
		assert.Equal(t, filled.OrderID, lookup.OrderID)

		if missingID, ok := synthesizeMissingOrderID(filled.OrderID); ok {
			_, err = adp.FetchOrderByID(ctx, missingID, symbol)
			require.ErrorIs(t, err, exchanges.ErrOrderNotFound)
		}
	case errors.Is(err, exchanges.ErrNotSupported):
		return
	default:
		require.ErrorIs(t, err, exchanges.ErrNotSupported)
	}
}

func testFetchOrdersVsOpenOrders(
	t *testing.T,
	adp exchanges.Exchange,
	symbol string,
	supportsOpenOrders bool,
	supportsOrderHistory bool,
	updates <-chan *exchanges.Order,
) {
	t.Helper()

	ctx := context.Background()

	if !supportsOpenOrders {
		_, err := adp.FetchOpenOrders(ctx, symbol)
		require.ErrorIs(t, err, exchanges.ErrNotSupported)

		if !supportsOrderHistory {
			_, err = adp.FetchOrders(ctx, symbol)
			require.ErrorIs(t, err, exchanges.ErrNotSupported)
			return
		}

		qty, _ := SmartQuantity(t, adp, symbol)
		price := SmartLimitPrice(t, adp, symbol, exchanges.OrderSideBuy)

		limit, err := adp.PlaceOrder(ctx, &exchanges.OrderParams{
			Symbol:      symbol,
			Side:        exchanges.OrderSideBuy,
			Type:        exchanges.OrderTypeLimit,
			Quantity:    qty,
			Price:       price,
			TimeInForce: exchanges.TimeInForceGTC,
		})
		require.NoError(t, err)

		confirmed := WaitOrderStatus(t, updates, limit.OrderID, limit.ClientOrderID, exchanges.OrderStatusNew, 15*time.Second)
		require.NotEmpty(t, confirmed.OrderID)

		err = adp.CancelOrder(ctx, confirmed.OrderID, symbol)
		require.NoError(t, err)

		cancelled := WaitOrderStatus(t, updates, confirmed.OrderID, confirmed.ClientOrderID, exchanges.OrderStatusCancelled, 15*time.Second)
		require.NotEmpty(t, cancelled.OrderID)

		allOrders, err := adp.FetchOrders(ctx, symbol)
		require.NoError(t, err)
		requireOrderPresent(t, allOrders, cancelled.OrderID)
		return
	}

	qty, _ := SmartQuantity(t, adp, symbol)
	price := SmartLimitPrice(t, adp, symbol, exchanges.OrderSideBuy)

	limit, err := adp.PlaceOrder(ctx, &exchanges.OrderParams{
		Symbol:      symbol,
		Side:        exchanges.OrderSideBuy,
		Type:        exchanges.OrderTypeLimit,
		Quantity:    qty,
		Price:       price,
		TimeInForce: exchanges.TimeInForceGTC,
	})
	require.NoError(t, err)

	confirmed := WaitOrderStatus(t, updates, limit.OrderID, limit.ClientOrderID, exchanges.OrderStatusNew, 15*time.Second)
	require.NotEmpty(t, confirmed.OrderID)

	openOrders, err := adp.FetchOpenOrders(ctx, symbol)
	require.NoError(t, err)
	requireOrderPresent(t, openOrders, confirmed.OrderID)

	allOrders, err := adp.FetchOrders(ctx, symbol)
	switch {
	case supportsOrderHistory:
		require.NoError(t, err)
		requireOrderPresent(t, allOrders, confirmed.OrderID)
	case errors.Is(err, exchanges.ErrNotSupported):
		err = adp.CancelOrder(ctx, confirmed.OrderID, symbol)
		require.NoError(t, err)
		cancelled := WaitOrderStatus(t, updates, confirmed.OrderID, confirmed.ClientOrderID, exchanges.OrderStatusCancelled, 15*time.Second)

		openOrders, err = adp.FetchOpenOrders(ctx, symbol)
		require.NoError(t, err)
		requireOrderAbsent(t, openOrders, cancelled.OrderID)
		return
	default:
		require.ErrorIs(t, err, exchanges.ErrNotSupported)
	}

	err = adp.CancelOrder(ctx, confirmed.OrderID, symbol)
	require.NoError(t, err)

	cancelled := WaitOrderStatus(t, updates, confirmed.OrderID, confirmed.ClientOrderID, exchanges.OrderStatusCancelled, 15*time.Second)
	require.NotEmpty(t, cancelled.OrderID)

	require.Eventually(t, func() bool {
		orders, err := adp.FetchOpenOrders(ctx, symbol)
		if err != nil {
			return false
		}
		return !containsOrderID(orders, cancelled.OrderID)
	}, 15*time.Second, 500*time.Millisecond)

	allOrders, err = adp.FetchOrders(ctx, symbol)
	require.NoError(t, err)
	requireOrderPresent(t, allOrders, cancelled.OrderID)
}

func requireOrderPresent(t *testing.T, orders []exchanges.Order, orderID string) {
	t.Helper()
	require.Truef(t, containsOrderID(orders, orderID), "expected order %s in result set", orderID)
}

func requireOrderAbsent(t *testing.T, orders []exchanges.Order, orderID string) {
	t.Helper()
	require.Falsef(t, containsOrderID(orders, orderID), "expected order %s to be absent from result set", orderID)
}

func containsOrderID(orders []exchanges.Order, orderID string) bool {
	for _, order := range orders {
		if order.OrderID == orderID {
			return true
		}
	}
	return false
}

func synthesizeMissingOrderID(orderID string) (string, bool) {
	if orderID == "" {
		return "", false
	}

	if n, err := strconv.ParseUint(orderID, 10, 64); err == nil {
		return strconv.FormatUint(n+1_000_000_000, 10), true
	}

	b := []byte(orderID)
	for i := len(b) - 1; i >= 0; i-- {
		switch {
		case b[i] >= '0' && b[i] <= '8':
			b[i]++
			return string(b), true
		case b[i] == '9':
			b[i] = '0'
			return string(b), true
		case b[i] >= 'a' && b[i] <= 'y':
			b[i]++
			return string(b), true
		case b[i] == 'z':
			b[i] = 'a'
			return string(b), true
		case b[i] >= 'A' && b[i] <= 'Y':
			b[i]++
			return string(b), true
		case b[i] == 'Z':
			b[i] = 'A'
			return string(b), true
		}
	}

	return "", false
}
