package testsuite

import (
	"context"
	"testing"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

// SetupOrderWatch subscribes to order updates via WatchOrders (Streamable)
// and returns a channel that receives all order updates.
func SetupOrderWatch(t *testing.T, adp exchanges.Exchange) <-chan *exchanges.Order {
	t.Helper()

	updates := make(chan *exchanges.Order, 100)
	err := adp.WatchOrders(context.Background(), func(o *exchanges.Order) {
		select {
		case updates <- o:
		default:
			t.Logf("WARNING: Order update channel full, dropping update for %s", o.OrderID)
		}
	})
	require.NoError(t, err, "WatchOrders should not fail")

	// Give WS time to establish
	time.Sleep(1 * time.Second)
	return updates
}

// WaitOrderStatus blocks until an order with matching orderID or clientID
// reaches the target status, or times out.
func WaitOrderStatus(
	t *testing.T,
	ch <-chan *exchanges.Order,
	orderID, clientID string,
	target exchanges.OrderStatus,
	timeout time.Duration,
) *exchanges.Order {
	t.Helper()

	timer := time.After(timeout)
	for {
		select {
		case o := <-ch:
			match := (orderID != "" && o.OrderID == orderID) ||
				(clientID != "" && o.ClientOrderID == clientID) ||
				(o.Symbol != "" && orderID == "" && clientID == "")
			if match {
				t.Logf("Order Update: ID=%s ClOrdID=%s Status=%s Filled=%s",
					o.OrderID, o.ClientOrderID, o.Status, o.FilledQuantity)
				if o.Status == target {
					return o
				}
				if o.Status == exchanges.OrderStatusCancelled || o.Status == exchanges.OrderStatusRejected {
					if target != exchanges.OrderStatusCancelled {
						t.Fatalf("Order %s/%s terminal status %s (wanted %s)", orderID, clientID, o.Status, target)
					}
					return o
				}
			}
		case <-timer:
			t.Fatalf("Timeout (%s) waiting for order %s/%s to reach status %s", timeout, orderID, clientID, target)
		}
	}
}

// SmartQuantity calculates a reasonable order quantity for testing.
func SmartQuantity(t *testing.T, adp exchanges.Exchange, symbol string) (qty, price decimal.Decimal) {
	t.Helper()
	ctx := context.Background()

	details, err := adp.FetchSymbolDetails(ctx, symbol)
	require.NoError(t, err, "FetchSymbolDetails should work")

	ticker, err := adp.FetchTicker(ctx, symbol)
	require.NoError(t, err, "FetchTicker should work")

	price = ticker.LastPrice
	qty = details.MinQuantity
	if qty.IsZero() || qty.IsNegative() {
		qty = decimal.NewFromFloat(0.01)
	}

	// Ensure we meet MinNotional
	if !details.MinNotional.IsZero() && qty.Mul(price).LessThan(details.MinNotional) {
		qty = details.MinNotional.Mul(decimal.NewFromFloat(1.2)).Div(price)
	}

	// Safety buffer
	qty = qty.Mul(decimal.NewFromInt(2))
	minQty := decimal.NewFromFloat(0.001)
	if qty.LessThan(minQty) {
		qty = minQty
	}

	qty = exchanges.FloorToPrecision(qty, details.QuantityPrecision)
	t.Logf("SmartQuantity for %s: qty=%s, price=%s", symbol, qty, price)
	return qty, price
}

// SmartLimitPrice calculates a passive limit price that won't fill immediately.
func SmartLimitPrice(t *testing.T, adp exchanges.Exchange, symbol string, side exchanges.OrderSide) decimal.Decimal {
	t.Helper()
	ctx := context.Background()

	ticker, err := adp.FetchTicker(ctx, symbol)
	require.NoError(t, err)

	details, err := adp.FetchSymbolDetails(ctx, symbol)
	require.NoError(t, err)

	var price decimal.Decimal
	if side == exchanges.OrderSideBuy {
		refPrice := ticker.Bid
		if refPrice.IsZero() {
			refPrice = ticker.LastPrice
		}
		price = refPrice.Mul(decimal.NewFromFloat(0.8)) // 20% below market
	} else {
		refPrice := ticker.Ask
		if refPrice.IsZero() {
			refPrice = ticker.LastPrice
		}
		price = refPrice.Mul(decimal.NewFromFloat(1.2)) // 20% above market
	}

	price = exchanges.RoundToPrecision(price, details.PricePrecision)
	return price
}
