package testsuite

import (
	"context"
	"testing"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// OrderSuiteConfig configures the order test suite.
type OrderSuiteConfig struct {
	Symbol       string          // Required: symbol to test on, e.g. "DOGE" or "ETH"
	Slippage     decimal.Decimal // Slippage for slippage test (default 0.05)
	SkipMarket   bool            // Skip market order test
	SkipSlippage bool            // Skip slippage order test
	SkipLimit    bool            // Skip limit order test
}

// RunOrderSuite runs a complete order placement test suite against any Exchange.
func RunOrderSuite(t *testing.T, adp exchanges.Exchange, cfg OrderSuiteConfig) {
	if cfg.Slippage.IsZero() {
		cfg.Slippage = decimal.NewFromFloat(0.05)
	}

	// Setup order watch for all sub-tests
	updates := SetupOrderWatch(t, adp)

	if !cfg.SkipMarket {
		t.Run("MarketOrder", func(t *testing.T) {
			testMarketOrder(t, adp, cfg.Symbol, updates)
		})
	}

	if !cfg.SkipLimit {
		t.Run("LimitOrder+Cancel", func(t *testing.T) {
			testLimitOrderAndCancel(t, adp, cfg.Symbol, updates)
		})
	}

	if !cfg.SkipSlippage {
		t.Run("MarketOrderWithSlippage", func(t *testing.T) {
			testMarketOrderSlippage(t, adp, cfg.Symbol, cfg.Slippage, updates)
		})
	}

	// Final cleanup: close any remaining position for the test symbol
	t.Run("Cleanup", func(t *testing.T) {
		closeAllPositions(t, adp, cfg.Symbol, updates)
	})
}

func testMarketOrder(t *testing.T, adp exchanges.Exchange, symbol string, updates <-chan *exchanges.Order) {
	qty, _ := SmartQuantity(t, adp, symbol)

	t.Logf("Placing Market Buy Order for %s, Qty: %s", symbol, qty)
	order, err := adp.PlaceOrder(context.Background(), &exchanges.OrderParams{
		Symbol:   symbol,
		Side:     exchanges.OrderSideBuy,
		Type:     exchanges.OrderTypeMarket,
		Quantity: qty,
	})
	require.NoError(t, err)
	t.Logf("Order Placed: %s", order.OrderID)

	filled := WaitOrderStatus(t, updates, order.OrderID, order.ClientOrderID, exchanges.OrderStatusFilled, 30*time.Second)
	assert.Equal(t, exchanges.OrderStatusFilled, filled.Status)
}

func testLimitOrderAndCancel(t *testing.T, adp exchanges.Exchange, symbol string, updates <-chan *exchanges.Order) {
	qty, _ := SmartQuantity(t, adp, symbol)
	price := SmartLimitPrice(t, adp, symbol, exchanges.OrderSideBuy)

	t.Logf("Placing Limit Buy Order for %s, Price: %s, Qty: %s", symbol, price, qty)
	order, err := adp.PlaceOrder(context.Background(), &exchanges.OrderParams{
		Symbol:      symbol,
		Side:        exchanges.OrderSideBuy,
		Type:        exchanges.OrderTypeLimit,
		Quantity:    qty,
		Price:       price,
		TimeInForce: exchanges.TimeInForceGTC,
	})
	require.NoError(t, err)
	t.Logf("Order Placed: %s", order.OrderID)

	confirmed := WaitOrderStatus(t, updates, order.OrderID, order.ClientOrderID, exchanges.OrderStatusNew, 15*time.Second)
	t.Logf("Order confirmed: %s (status=%s)", confirmed.OrderID, confirmed.Status)

	cancelID := confirmed.OrderID
	if cancelID == "" {
		cancelID = order.OrderID
	}
	err = adp.CancelOrder(context.Background(), cancelID, symbol)
	if err != nil {
		t.Logf("CancelOrder returned error (may already be filled): %v", err)
	} else {
		t.Logf("Order %s cancelled", cancelID)
	}
}

func testMarketOrderSlippage(t *testing.T, adp exchanges.Exchange, symbol string, slippage decimal.Decimal, updates <-chan *exchanges.Order) {
	qty, _ := SmartQuantity(t, adp, symbol)

	t.Logf("Placing Market Buy Order with Slippage for %s, Qty: %s, Slippage: %s", symbol, qty, slippage)
	order, err := adp.PlaceOrder(context.Background(), &exchanges.OrderParams{
		Symbol:   symbol,
		Side:     exchanges.OrderSideBuy,
		Type:     exchanges.OrderTypeMarket,
		Quantity: qty,
		Slippage: slippage,
	})
	require.NoError(t, err)
	t.Logf("Order Placed: %s", order.OrderID)

	filled := WaitOrderStatus(t, updates, order.OrderID, order.ClientOrderID, exchanges.OrderStatusFilled, 30*time.Second)
	assert.Equal(t, exchanges.OrderStatusFilled, filled.Status)
}

// closeAllPositions fetches current positions for the symbol and closes them with market orders.
func closeAllPositions(t *testing.T, adp exchanges.Exchange, symbol string, updates <-chan *exchanges.Order) {
	t.Helper()
	ctx := context.Background()

	account, err := adp.FetchAccount(ctx)
	if err != nil {
		t.Logf("FetchAccount failed, skipping cleanup: %v", err)
		return
	}

	for _, pos := range account.Positions {
		if pos.Symbol != symbol || pos.Quantity.IsZero() {
			continue
		}

		// Determine the reverse side
		closeSide := exchanges.OrderSideSell
		if pos.Side == exchanges.PositionSideShort {
			closeSide = exchanges.OrderSideBuy
		}

		t.Logf("Closing position: %s %s → %s (qty=%s)", symbol, pos.Side, closeSide, pos.Quantity)
		order, err := adp.PlaceOrder(ctx, &exchanges.OrderParams{
			Symbol:     symbol,
			Side:       closeSide,
			Type:       exchanges.OrderTypeMarket,
			Quantity:   pos.Quantity,
			ReduceOnly: true,
		})
		if err != nil {
			t.Logf("Failed to close position: %v", err)
			continue
		}

		filled := WaitOrderStatus(t, updates, order.OrderID, order.ClientOrderID, exchanges.OrderStatusFilled, 30*time.Second)
		t.Logf("Position closed: OrderID=%s, Status=%s", filled.OrderID, filled.Status)
	}

	// Verify no remaining positions
	account, err = adp.FetchAccount(ctx)
	if err != nil {
		t.Logf("FetchAccount verification failed: %v", err)
		return
	}

	for _, pos := range account.Positions {
		if pos.Symbol == symbol && !pos.Quantity.IsZero() {
			t.Errorf("Position still open after cleanup: %s %s qty=%s", symbol, pos.Side, pos.Quantity)
		}
	}

	t.Logf("All positions for %s closed successfully", symbol)
}
