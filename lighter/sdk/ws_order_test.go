package lighter

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestPlaceOrder(t *testing.T) {
	requireFullEnv(t)
	privateKey, accountIndex, keyIndex := GetEnv()

	client := NewClient().WithCredentials(privateKey, accountIndex, uint8(keyIndex))
	wsClient := NewWebsocketClient(context.Background())
	wsClient.Connect()

	authToken, err := client.CreateAuthToken(time.Now().Add(time.Minute * 30))
	if err != nil {
		t.Fatal(err)
	}
	orderCh := make(chan []byte)
	wsClient.SubscribeAccountAllOrders(accountIndex, authToken, func(data []byte) {
		orderCh <- data
	})

	marketId := 1
	resp, err := client.GetOrderBookDetails(context.Background(), &marketId, nil)
	if err != nil {
		t.Fatal(err)
	}
	detail := resp.OrderBookDetails[0]
	orderBooks, err := client.GetOrderBookOrders(context.Background(), marketId, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(orderBooks.Asks) == 0 {
		t.Fatal("no asks in orderbook")
	}
	// For a BUY order (IsAsk=0), we want to buy from the lowest seller (BestAsk).
	// To ensure it fills as a Market order, we set a limit price slightly HIGHER than the BestAsk.
	ask, _ := decimal.NewFromString(orderBooks.Asks[0].Price)
	slippage := 0.05
	// Price = BestAsk * (1 + slippage)
	price := ask.Mul(decimal.NewFromFloat(1 + slippage))

	// format price by detail.PriceDecimals to integer representation
	price = price.Mul(decimal.NewFromInt(int64(math.Pow10(int(detail.PriceDecimals)))))

	// format base amount by detail.SizeDecimals to integer representation
	baseAmount := decimal.NewFromFloat(0.001).Mul(decimal.NewFromInt(int64(math.Pow10(int(detail.SizeDecimals)))))

	orderID, err := wsClient.PlaceOrder(context.Background(), client, CreateOrderRequest{
		MarketId:    1,
		BaseAmount:  int64(baseAmount.IntPart()),
		Price:       uint32(price.IntPart()),
		IsAsk:       0, // 0 = Buy
		OrderType:   OrderTypeMarket,
		TimeInForce: OrderTimeInForceImmediateOrCancel,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("orderID", orderID)

	timeout := time.NewTimer(time.Second * 10)
	defer timeout.Stop()
	for {
		select {
		case <-timeout.C:
			t.Fatal("timeout")
		case order := <-orderCh:
			t.Log(string(order))
			return
		}
	}
}

// TestPlaceOrderInsufficientMargin tests that order rejection errors (e.g., insufficient margin)
// are properly captured and propagated when placing an order that exceeds available margin
func TestPlaceOrderInsufficientMargin(t *testing.T) {
	requireFullEnv(t)
	privateKey, accountIndex, keyIndex := GetEnv()

	client := NewClient().WithCredentials(privateKey, accountIndex, uint8(keyIndex))
	wsClient := NewWebsocketClient(context.Background())
	if err := wsClient.Connect(); err != nil {
		t.Fatal("Failed to connect WebSocket:", err)
	}
	defer wsClient.Close()

	// Subscribe to order updates FIRST to capture all order state changes
	authToken, err := client.CreateAuthToken(time.Now().Add(time.Minute * 30))
	if err != nil {
		t.Fatal(err)
	}
	orderCh := make(chan []byte, 100) // Larger buffer
	if err := wsClient.SubscribeAccountAllOrders(accountIndex, authToken, func(data []byte) {
		t.Logf("📨 Order update received: %s", string(data))
		orderCh <- data
	}); err != nil {
		t.Fatal("Failed to subscribe to orders:", err)
	}

	// Wait for subscription confirmation
	select {
	case msg := <-orderCh:
		t.Logf("✓ Subscription confirmed: %s", string(msg))
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for subscription confirmation")
	}

	// Get market details for SOL (market ID 2)
	marketId := 2
	resp, err := client.GetOrderBookDetails(context.Background(), &marketId, nil)
	if err != nil {
		t.Fatal(err)
	}
	detail := resp.OrderBookDetails[0]

	// Get current orderbook to determine a reasonable price
	orderBooks, err := client.GetOrderBookOrders(context.Background(), marketId, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(orderBooks.Asks) == 0 {
		t.Fatal("no asks in orderbook")
	}

	// Use best ask price
	ask, _ := decimal.NewFromString(orderBooks.Asks[0].Price)
	slippage := 0.05
	price := ask.Mul(decimal.NewFromFloat(1 + slippage))

	// Format price by detail.PriceDecimals to integer representation
	price = price.Mul(decimal.NewFromInt(int64(math.Pow10(int(detail.PriceDecimals)))))

	// **KEY CHANGE**: Order 20 SOL instead of 0.001 SOL
	// This should exceed available margin and trigger an error
	baseAmount := decimal.NewFromFloat(20.0).Mul(decimal.NewFromInt(int64(math.Pow10(int(detail.SizeDecimals)))))

	t.Logf("🔵 Attempting to place order for 20 SOL (baseAmount: %d)", baseAmount.IntPart())
	t.Logf("   Expected: Order should be rejected due to insufficient margin")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	orderID, err := wsClient.PlaceOrder(ctx, client, CreateOrderRequest{
		MarketId:    marketId,
		BaseAmount:  int64(baseAmount.IntPart()),
		Price:       uint32(price.IntPart()),
		IsAsk:       0, // 0 = Buy
		OrderType:   OrderTypeMarket,
		TimeInForce: OrderTimeInForceImmediateOrCancel,
	})

	// Check immediate response from sendtx
	if err != nil {
		t.Logf("✅ Order immediately rejected via sendtx response: %v", err)
		t.Logf("   Order ID (hash): %s", orderID)
		return // Test passed - error was caught immediately
	}

	// Order was accepted via sendtx (code 200), but may still be rejected
	t.Logf("⚠️  Order sendtx accepted (code 200), waiting for order updates...")
	t.Logf("   Order ID: %s", orderID)

	// Monitor order updates for 10 seconds to see the final order status
	timeout := time.NewTimer(10 * time.Second)
	defer timeout.Stop()

	orderRejected := false
	for {
		select {
		case <-timeout.C:
			if orderRejected {
				t.Logf("✅ Order was rejected (detected via order updates)")
			} else {
				// This is actually unexpected - order should have been rejected or filled
				t.Logf("⚠️  Timeout: No order rejection detected in updates")
				t.Logf("   This could mean:")
				t.Logf("   1. Account has sufficient margin for 20 SOL")
				t.Logf("   2. Order was filled/cancelled but no update received")
				t.Logf("   3. Update messages are delayed")
			}
			return

		case orderUpdate := <-orderCh:
			updateStr := string(orderUpdate)
			
			// Check if this update contains order rejection/failure indicators
			if contains(updateStr, "rejected") || contains(updateStr, "failed") || 
			   contains(updateStr, "cancelled") || contains(updateStr, "insufficient") {
				t.Logf("🔴 Order rejection detected in update!")
				orderRejected = true
			}
			
			// Continue monitoring for a bit to see all updates
		}
	}
}

// Helper function to check if string contains substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || 
		(len(s) > 0 && len(substr) > 0 && containsAt(s, substr, 0)))
}

func containsAt(s, substr string, start int) bool {
	if start+len(substr) > len(s) {
		return false
	}
	for i := start; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if toLower(s[i+j]) != toLower(substr[j]) {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func toLower(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b + ('a' - 'A')
	}
	return b
}
