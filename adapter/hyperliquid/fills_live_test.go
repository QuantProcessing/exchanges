package hyperliquid

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

	qty, _ := testsuite.SmartQuantity(t, adp, "HYPE")

	buyOrder, err := adp.PlaceOrder(ctx, &exchanges.OrderParams{
		Symbol:   "HYPE",
		Side:     exchanges.OrderSideBuy,
		Type:     exchanges.OrderTypeMarket,
		Quantity: qty,
	})
	require.NoError(t, err)

	buyFilled := testsuite.WaitOrderStatus(t, orderUpdates, buyOrder.OrderID, buyOrder.ClientOrderID, exchanges.OrderStatusFilled, 30*time.Second)
	buyFills := waitHLFills(t, fillCh, buyFilled.OrderID, exchanges.OrderSideBuy, buyFilled.FilledQuantity, 30*time.Second)
	require.NotEmpty(t, buyFills)

	sellOrder, err := adp.PlaceOrder(ctx, &exchanges.OrderParams{
		Symbol:     "HYPE",
		Side:       exchanges.OrderSideSell,
		Type:       exchanges.OrderTypeMarket,
		Quantity:   buyFilled.FilledQuantity,
		ReduceOnly: true,
	})
	require.NoError(t, err)

	sellFilled := testsuite.WaitOrderStatus(t, orderUpdates, sellOrder.OrderID, sellOrder.ClientOrderID, exchanges.OrderStatusFilled, 30*time.Second)
	sellFills := waitHLFills(t, fillCh, sellFilled.OrderID, exchanges.OrderSideSell, sellFilled.FilledQuantity, 30*time.Second)
	require.NotEmpty(t, sellFills)

	for i, fill := range buyFills {
		t.Logf("perp buy fill[%d]: order=%s trade=%s qty=%s price=%s fee=%s %s",
			i, fill.OrderID, fill.TradeID, fill.Quantity, fill.Price, fill.Fee, fill.FeeAsset)
	}
	for i, fill := range sellFills {
		t.Logf("perp sell fill[%d]: order=%s trade=%s qty=%s price=%s fee=%s %s",
			i, fill.OrderID, fill.TradeID, fill.Quantity, fill.Price, fill.Fee, fill.FeeAsset)
	}
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

	qty, _ := testsuite.SmartQuantity(t, adp, "HYPE")

	buyOrder, err := adp.PlaceOrder(ctx, &exchanges.OrderParams{
		Symbol:   "HYPE",
		Side:     exchanges.OrderSideBuy,
		Type:     exchanges.OrderTypeMarket,
		Quantity: qty,
	})
	require.NoError(t, err)

	buyFilled := testsuite.WaitOrderStatus(t, orderUpdates, buyOrder.OrderID, buyOrder.ClientOrderID, exchanges.OrderStatusFilled, 30*time.Second)
	buyFills := waitHLFills(t, fillCh, buyFilled.OrderID, exchanges.OrderSideBuy, buyFilled.FilledQuantity, 30*time.Second)
	require.NotEmpty(t, buyFills)

	sellOrder, err := adp.PlaceOrder(ctx, &exchanges.OrderParams{
		Symbol:   "HYPE",
		Side:     exchanges.OrderSideSell,
		Type:     exchanges.OrderTypeMarket,
		Quantity: buyFilled.FilledQuantity,
	})
	require.NoError(t, err)

	sellFilled := testsuite.WaitOrderStatus(t, orderUpdates, sellOrder.OrderID, sellOrder.ClientOrderID, exchanges.OrderStatusFilled, 30*time.Second)
	sellFills := waitHLFills(t, fillCh, sellFilled.OrderID, exchanges.OrderSideSell, sellFilled.FilledQuantity, 30*time.Second)
	require.NotEmpty(t, sellFills)

	for i, fill := range buyFills {
		t.Logf("spot buy fill[%d]: order=%s trade=%s qty=%s price=%s fee=%s %s",
			i, fill.OrderID, fill.TradeID, fill.Quantity, fill.Price, fill.Fee, fill.FeeAsset)
	}
	for i, fill := range sellFills {
		t.Logf("spot sell fill[%d]: order=%s trade=%s qty=%s price=%s fee=%s %s",
			i, fill.OrderID, fill.TradeID, fill.Quantity, fill.Price, fill.Fee, fill.FeeAsset)
	}
}

func waitHLFills(
	t *testing.T,
	ch <-chan *exchanges.Fill,
	orderID string,
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
			if fill == nil || fill.OrderID != orderID {
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
			t.Fatalf("timeout waiting for fills order=%s side=%s expectedQty=%s gotQty=%s fills=%d", orderID, side, expectedQty, total, len(fills))
		}
	}
}
