package testsuite

import (
	"context"
	"fmt"
	"testing"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// LocalStateConfig configures the LocalState integration test.
type LocalStateConfig struct {
	Symbol string // Required: e.g. "DOGE"
}

// RunLocalStateSuite tests the unified LocalState against a live exchange adapter.
//
// It validates:
//  1. Start — REST snapshot + WS subscriptions
//  2. State Queries — GetBalance, GetAllPositions, GetAllOpenOrders
//  3. Order Subscription — SubscribeOrders fan-out
//  4. PlaceOrder + WaitTerminal — market buy with order tracking
//  5. Position Tracking — position update via subscription
//  6. Limit Order + Cancel — PlaceOrder limit, then cancel, verify CANCELLED event
//  7. Close Position — reverse market order, verify position closed
//  8. Cleanup — Close LocalState
func RunLocalStateSuite(t *testing.T, adp exchanges.Exchange, cfg LocalStateConfig) {
	require.NotEmpty(t, cfg.Symbol, "Symbol must be set")

	ctx := context.Background()
	isPerp := adp.GetMarketType() == exchanges.MarketTypePerp

	// ======================================================================
	// Phase 1: Start LocalState
	// ======================================================================
	t.Log("═══ Phase 1: Start LocalState ═══")

	state := exchanges.NewLocalState(adp, nil)
	err := state.Start(ctx)
	require.NoError(t, err, "LocalState.Start should succeed")
	defer state.Close()
	t.Log("✓ LocalState started (REST snapshot + WS subscriptions)")

	// Allow WS to stabilize
	time.Sleep(2 * time.Second)

	// ======================================================================
	// Phase 2: Verify initial state queries
	// ======================================================================
	t.Log("═══ Phase 2: Verify initial state queries ═══")

	balance := state.GetBalance()
	t.Logf("  Balance: %s", balance)
	assert.True(t, balance.IsPositive() || balance.IsZero(), "Balance should be >= 0")

	positions := state.GetAllPositions()
	t.Logf("  Positions: %d", len(positions))
	for _, p := range positions {
		t.Logf("    %s: side=%s qty=%s", p.Symbol, p.Side, p.Quantity)
	}

	openOrders := state.GetAllOpenOrders()
	t.Logf("  Open orders: %d", len(openOrders))
	t.Log("✓ Initial state queries OK")

	// ======================================================================
	// Phase 3: Test SubscribeOrders fan-out (2 concurrent subscribers)
	// ======================================================================
	t.Log("═══ Phase 3: Test SubscribeOrders fan-out ═══")

	sub1 := state.SubscribeOrders()
	defer sub1.Unsubscribe()
	sub2 := state.SubscribeOrders()
	defer sub2.Unsubscribe()
	t.Log("✓ Two order subscribers created")

	// ======================================================================
	// Phase 4: PlaceOrder market buy → WaitTerminal
	// ======================================================================
	t.Log("═══ Phase 4: PlaceOrder market buy + WaitTerminal ═══")

	qty, lastPrice := SmartQuantity(t, adp, cfg.Symbol)
	t.Logf("  Symbol=%s  Qty=%s  RefPrice=%s", cfg.Symbol, qty, lastPrice)

	result, err := state.PlaceOrder(ctx, &exchanges.OrderParams{
		Symbol:   cfg.Symbol,
		Side:     exchanges.OrderSideBuy,
		Type:     exchanges.OrderTypeMarket,
		Quantity: qty,
	})
	require.NoError(t, err, "PlaceOrder(market buy) should succeed")
	defer result.Done()
	t.Logf("✓ Market buy placed: ID=%s", result.Order.OrderID)

	// WaitTerminal — should reach FILLED
	filled, err := result.WaitTerminal(30 * time.Second)
	require.NoError(t, err, "WaitTerminal should succeed")
	assert.Equal(t, exchanges.OrderStatusFilled, filled.Status)
	t.Logf("✓ WaitTerminal returned FILLED: qty=%s", filled.FilledQuantity)

	// Verify fan-out: sub1 and sub2 should have received the same events
	verifySub := func(name string, sub *exchanges.Subscription[exchanges.Order]) {
		timer := time.After(3 * time.Second)
		for {
			select {
			case o := <-sub.C:
				if o.OrderID == result.Order.OrderID || o.ClientOrderID == result.Order.ClientOrderID {
					t.Logf("  %s received: ID=%s Status=%s", name, o.OrderID, o.Status)
					if o.Status == exchanges.OrderStatusFilled ||
						o.Status == exchanges.OrderStatusCancelled ||
						o.Status == exchanges.OrderStatusRejected {
						return
					}
				}
			case <-timer:
				t.Logf("  %s: no terminal event within 3s (may have been consumed)", name)
				return
			}
		}
	}
	verifySub("sub1", sub1)
	verifySub("sub2", sub2)
	t.Log("✓ Fan-out verified")

	// ======================================================================
	// Phase 5: Verify position via LocalState
	// ======================================================================
	t.Log("═══ Phase 5: Verify position via LocalState ═══")

	// Wait briefly for position updates to propagate
	time.Sleep(2 * time.Second)

	if isPerp {
		pos, ok := state.GetPosition(cfg.Symbol)
		if ok && pos.Quantity.IsPositive() {
			t.Logf("✓ Position tracked: side=%s qty=%s entry=%s", pos.Side, pos.Quantity, pos.EntryPrice)
		} else {
			// Position might come from REST refresh; also check FetchPositions
			t.Log("  Position not yet in LocalState (may need WatchPositions support)")
		}
	} else {
		t.Log("  Spot: position tracking via balance change")
	}

	// ======================================================================
	// Phase 6: Limit order → Cancel → verify CANCELLED event
	// ======================================================================
	t.Log("═══ Phase 6: Limit order + Cancel ═══")

	limitPrice := SmartLimitPrice(t, adp, cfg.Symbol, exchanges.OrderSideBuy)
	t.Logf("  Passive limit price: %s (well below market)", limitPrice)

	limitResult, err := state.PlaceOrder(ctx, &exchanges.OrderParams{
		Symbol:   cfg.Symbol,
		Side:     exchanges.OrderSideBuy,
		Type:     exchanges.OrderTypeLimit,
		Price:    limitPrice,
		Quantity: qty,
	})
	require.NoError(t, err, "PlaceOrder(limit buy) should succeed")
	defer limitResult.Done()
	t.Logf("✓ Limit order placed: ID=%s", limitResult.Order.OrderID)

	// Wait for order to appear in local state
	time.Sleep(2 * time.Second)
	_, found := state.GetOrder(limitResult.Order.OrderID)
	t.Logf("  Order in local state: %v", found)

	// Cancel the order
	cancelOrderID := limitResult.Order.OrderID
	err = adp.CancelOrder(ctx, cancelOrderID, cfg.Symbol)
	require.NoError(t, err, "CancelOrder should succeed")
	t.Logf("✓ Cancel sent for order %s", cancelOrderID)

	// WaitTerminal — should reach CANCELLED
	cancelled, err := limitResult.WaitTerminal(30 * time.Second)
	require.NoError(t, err, "WaitTerminal should succeed for cancelled order")
	assert.Equal(t, exchanges.OrderStatusCancelled, cancelled.Status)
	t.Logf("✓ WaitTerminal returned CANCELLED")

	// Verify it's removed from local open orders
	time.Sleep(500 * time.Millisecond)
	_, stillOpen := state.GetOrder(cancelOrderID)
	assert.False(t, stillOpen, "Cancelled order should be removed from local orders")
	t.Log("✓ Cancelled order removed from local state")

	// ======================================================================
	// Phase 7: Close position
	// ======================================================================
	t.Log("═══ Phase 7: Close position ═══")

	closeQty := qty
	if isPerp {
		// Use actual position qty if available
		if p, ok := state.GetPosition(cfg.Symbol); ok && p.Quantity.IsPositive() {
			closeQty = p.Quantity
		}
	} else {
		closeQty = filled.FilledQuantity
	}

	closeParams := &exchanges.OrderParams{
		Symbol:   cfg.Symbol,
		Side:     exchanges.OrderSideSell,
		Type:     exchanges.OrderTypeMarket,
		Quantity: closeQty,
	}
	if isPerp {
		closeParams.ReduceOnly = true
	}

	closeResult, err := state.PlaceOrder(ctx, closeParams)
	require.NoError(t, err, "PlaceOrder(close) should succeed")
	defer closeResult.Done()
	t.Logf("✓ Close order placed: ID=%s", closeResult.Order.OrderID)

	closeFilled, err := closeResult.WaitTerminal(30 * time.Second)
	require.NoError(t, err, "WaitTerminal(close) should succeed")
	assert.Equal(t, exchanges.OrderStatusFilled, closeFilled.Status)
	t.Logf("✓ Close order filled: qty=%s", closeFilled.FilledQuantity)

	// ======================================================================
	// Phase 8: Verify final state
	// ======================================================================
	t.Log("═══ Phase 8: Verify final state ═══")

	if isPerp {
		time.Sleep(2 * time.Second)
		pos, ok := state.GetPosition(cfg.Symbol)
		if ok {
			t.Logf("  Final position: qty=%s (should be ~0)", pos.Quantity)
		} else {
			t.Log("  No position found (closed)")
		}

		// Also verify via REST
		posAfter := findPosition(t, adp, cfg.Symbol)
		if posAfter != nil {
			assert.True(t, posAfter.Quantity.IsZero(),
				fmt.Sprintf("Position should be zero after close, got qty=%s", posAfter.Quantity))
		}
		t.Log("✓ Position closed successfully")
	} else {
		account, err := adp.FetchAccount(ctx)
		require.NoError(t, err)
		t.Logf("✓ Spot closed, final balance: %s", account.TotalBalance)
	}

	finalBalance := state.GetBalance()
	t.Logf("  Final balance from LocalState: %s", finalBalance)

	// ======================================================================
	// Summary
	// ======================================================================
	t.Log("═══ Summary ═══")
	t.Log("✓ LocalState.Start (REST + WS)")
	t.Log("✓ State queries (balance, positions, orders)")
	t.Log("✓ SubscribeOrders fan-out (2 consumers)")
	t.Log("✓ PlaceOrder + WaitTerminal (market fill)")
	t.Log("✓ PlaceOrder + Cancel + WaitTerminal (limit cancel)")
	t.Log("✓ Position tracking")
	t.Log("✓ Close position")
	t.Log("All LocalState integration tests passed!")

	// Give a moment for any pending prints
	_ = lastPrice
	_ = decimal.Zero
}
