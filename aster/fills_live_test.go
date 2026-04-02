package aster

import (
	"context"
	"testing"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/testsuite"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestPerpAdapter_WatchFillsLive(t *testing.T) {
	adp := setupPerpAdapter(t)
	ctx := context.Background()

	orderUpdates := testsuite.SetupOrderWatch(t, adp)

	fillCh := make(chan *exchanges.Fill, 32)
	require.NoError(t, adp.WatchFills(ctx, func(fill *exchanges.Fill) {
		select {
		case fillCh <- fill:
		default:
			t.Logf("dropping fill update for order=%s trade=%s", fill.OrderID, fill.TradeID)
		}
	}))

	time.Sleep(time.Second)

	qty, _ := testsuite.SmartQuantity(t, adp, "DOGE")

	buyOrder, err := adp.PlaceOrder(ctx, &exchanges.OrderParams{
		Symbol:   "DOGE",
		Side:     exchanges.OrderSideBuy,
		Type:     exchanges.OrderTypeMarket,
		Quantity: qty,
	})
	require.NoError(t, err)

	buyFilled := testsuite.WaitOrderStatus(t, orderUpdates, buyOrder.OrderID, buyOrder.ClientOrderID, exchanges.OrderStatusFilled, 30*time.Second)
	buyFills := waitAsterFills(t, fillCh, buyFilled.OrderID, buyFilled.ClientOrderID, exchanges.OrderSideBuy, buyFilled.FilledQuantity, 30*time.Second)
	require.NotEmpty(t, buyFills)

	sellOrder, err := adp.PlaceOrder(ctx, &exchanges.OrderParams{
		Symbol:     "DOGE",
		Side:       exchanges.OrderSideSell,
		Type:       exchanges.OrderTypeMarket,
		Quantity:   buyFilled.FilledQuantity,
		ReduceOnly: true,
	})
	require.NoError(t, err)

	sellFilled := testsuite.WaitOrderStatus(t, orderUpdates, sellOrder.OrderID, sellOrder.ClientOrderID, exchanges.OrderStatusFilled, 30*time.Second)
	sellFills := waitAsterFills(t, fillCh, sellFilled.OrderID, sellFilled.ClientOrderID, exchanges.OrderSideSell, sellFilled.FilledQuantity, 30*time.Second)
	require.NotEmpty(t, sellFills)

	logAsterFills(t, "perp buy", buyFills)
	logAsterFills(t, "perp sell", sellFills)
}

func TestSpotAdapter_WatchFillsLive(t *testing.T) {
	adp := setupSpotAdapter(t)
	ctx := context.Background()

	orderUpdates := testsuite.SetupOrderWatch(t, adp)

	fillCh := make(chan *exchanges.Fill, 32)
	require.NoError(t, adp.WatchFills(ctx, func(fill *exchanges.Fill) {
		select {
		case fillCh <- fill:
		default:
			t.Logf("dropping fill update for order=%s trade=%s", fill.OrderID, fill.TradeID)
		}
	}))

	time.Sleep(time.Second)

	qty, _ := testsuite.SmartQuantity(t, adp, "ASTER")

	buyOrder, err := adp.PlaceOrder(ctx, &exchanges.OrderParams{
		Symbol:   "ASTER",
		Side:     exchanges.OrderSideBuy,
		Type:     exchanges.OrderTypeMarket,
		Quantity: qty,
	})
	require.NoError(t, err)

	buyFilled := testsuite.WaitOrderStatus(t, orderUpdates, buyOrder.OrderID, buyOrder.ClientOrderID, exchanges.OrderStatusFilled, 30*time.Second)
	buyFills := waitAsterFills(t, fillCh, buyFilled.OrderID, buyFilled.ClientOrderID, exchanges.OrderSideBuy, buyFilled.FilledQuantity, 30*time.Second)
	require.NotEmpty(t, buyFills)

	sellOrder, err := adp.PlaceOrder(ctx, &exchanges.OrderParams{
		Symbol:   "ASTER",
		Side:     exchanges.OrderSideSell,
		Type:     exchanges.OrderTypeMarket,
		Quantity: buyFilled.FilledQuantity,
	})
	require.NoError(t, err)

	sellFilled := testsuite.WaitOrderStatus(t, orderUpdates, sellOrder.OrderID, sellOrder.ClientOrderID, exchanges.OrderStatusFilled, 30*time.Second)
	sellFills := waitAsterFills(t, fillCh, sellFilled.OrderID, sellFilled.ClientOrderID, exchanges.OrderSideSell, sellFilled.FilledQuantity, 30*time.Second)
	require.NotEmpty(t, sellFills)

	logAsterFills(t, "spot buy", buyFills)
	logAsterFills(t, "spot sell", sellFills)
}

func waitAsterFills(
	t *testing.T,
	ch <-chan *exchanges.Fill,
	orderID, clientID string,
	side exchanges.OrderSide,
	expectedQty decimal.Decimal,
	timeout time.Duration,
) []*exchanges.Fill {
	t.Helper()

	var (
		fills []*exchanges.Fill
		total = decimal.Zero
	)
	timer := time.After(timeout)
	for {
		select {
		case fill := <-ch:
			if fill == nil {
				continue
			}
			match := (orderID != "" && fill.OrderID == orderID) ||
				(clientID != "" && fill.ClientOrderID == clientID)
			if !match {
				continue
			}
			require.Equal(t, side, fill.Side)
			require.True(t, fill.Price.IsPositive())
			require.True(t, fill.Quantity.IsPositive())
			fills = append(fills, fill)
			total = total.Add(fill.Quantity)
			if total.GreaterThanOrEqual(expectedQty) {
				return fills
			}
		case <-timer:
			t.Fatalf("timeout waiting for fills order=%s clientID=%s side=%s expectedQty=%s gotQty=%s fills=%d", orderID, clientID, side, expectedQty, total, len(fills))
		}
	}
}

func logAsterFills(t *testing.T, prefix string, fills []*exchanges.Fill) {
	t.Helper()

	for i, fill := range fills {
		t.Logf("%s fill[%d]: order=%s trade=%s qty=%s price=%s fee=%s %s",
			prefix, i, fill.OrderID, fill.TradeID, fill.Quantity, fill.Price, fill.Fee, fill.FeeAsset)
	}
}
