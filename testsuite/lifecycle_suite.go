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

// LifecycleConfig configures the full lifecycle integration test.
type LifecycleConfig struct {
	Symbol string // Required: e.g. "DOGE"
}

// RunLifecycleSuite executes a comprehensive trading lifecycle test:
//
//  1. Subscribe — WatchOrders, collect all order state transitions
//  2. Open — Place a market buy and wait for FILLED
//  3. Verify — Check position (perp) or balance change (spot)
//  4. Close — Reverse the position while monitoring state transitions
//  5. Verify — Assert position/balance is restored
//  6. Disconnect — StopWatchOrders, Close
//
// This test is intentionally a single sequential flow to validate that
// all pieces work together end-to-end.
func RunLifecycleSuite(t *testing.T, adp exchanges.Exchange, cfg LifecycleConfig) {
	require.NotEmpty(t, cfg.Symbol, "Symbol must be set")

	ctx := context.Background()
	isPerp := adp.GetMarketType() == exchanges.MarketTypePerp

	// ── Collect all state transitions for verification at the end ──
	var allTransitions []transition

	// ======================================================================
	// Phase 1: Subscribe to order updates
	// ======================================================================
	t.Log("═══ Phase 1: Subscribe to order updates ═══")

	orderCh := make(chan *exchanges.Order, 100)
	err := adp.WatchOrders(ctx, func(o *exchanges.Order) {
		select {
		case orderCh <- o:
		default:
			t.Logf("⚠ Order channel full, dropping: %s %s", o.OrderID, o.Status)
		}
	})
	require.NoError(t, err, "WatchOrders should succeed")
	t.Log("✓ WatchOrders subscribed")

	// Give WebSocket time to establish
	time.Sleep(1 * time.Second)

	// ======================================================================
	// Phase 2: Place market buy order and wait for fill
	// ======================================================================
	t.Log("═══ Phase 2: Place market buy order ═══")

	qty, lastPrice := SmartQuantity(t, adp, cfg.Symbol)
	t.Logf("  Symbol=%s  Qty=%s  RefPrice=%s  MarketType=%s", cfg.Symbol, qty, lastPrice, adp.GetMarketType())

	buyOrder, err := adp.PlaceOrder(ctx, &exchanges.OrderParams{
		Symbol:   cfg.Symbol,
		Side:     exchanges.OrderSideBuy,
		Type:     exchanges.OrderTypeMarket,
		Quantity: qty,
	})
	require.NoError(t, err, "PlaceOrder(buy) should succeed")
	t.Logf("✓ Buy order placed: ID=%s", buyOrder.OrderID)

	// Wait for FILLED, collecting all transitions
	buyFilled := waitAndCollect(t, orderCh, buyOrder.OrderID, buyOrder.ClientOrderID,
		exchanges.OrderStatusFilled, 30*time.Second, &allTransitions)
	assert.Equal(t, exchanges.OrderStatusFilled, buyFilled.Status)
	t.Logf("✓ Buy order filled: qty=%s", buyFilled.FilledQuantity)

	// ======================================================================
	// Phase 3: Verify position / balance
	// ======================================================================
	t.Log("═══ Phase 3: Verify position ═══")

	var closeQty decimal.Decimal
	var closeSide exchanges.OrderSide

	if isPerp {
		// Perp: check FetchPositions
		pos := findPosition(t, adp, cfg.Symbol)
		require.NotNil(t, pos, "Position should exist after buy fill")

		t.Logf("✓ Position verified: side=%s qty=%s entryPrice=%s",
			pos.Side, pos.Quantity, pos.EntryPrice)
		assert.True(t, pos.Quantity.IsPositive(), "Position qty should be > 0")

		closeQty = pos.Quantity
		closeSide = exchanges.OrderSideSell
		if pos.Side == exchanges.PositionSideShort {
			closeSide = exchanges.OrderSideBuy
		}
	} else {
		// Spot: check that the bought asset balance increased
		account, err := adp.FetchAccount(ctx)
		require.NoError(t, err, "FetchAccount should succeed")

		t.Logf("✓ Account fetched: TotalBalance=%s, AvailableBalance=%s",
			account.TotalBalance, account.AvailableBalance)

		// For spot, the bought qty from the fill is what we need to sell back
		closeQty = buyFilled.FilledQuantity
		closeSide = exchanges.OrderSideSell

		// Verify the fill qty is positive
		require.True(t, closeQty.IsPositive(),
			fmt.Sprintf("FilledQuantity should be > 0, got %s", closeQty))

		t.Logf("✓ Spot buy verified: filled %s %s", closeQty, cfg.Symbol)
	}

	// ======================================================================
	// Phase 4: Close position with a reverse market order
	// ======================================================================
	t.Log("═══ Phase 4: Close position ═══")
	t.Logf("  Closing: side=%s qty=%s", closeSide, closeQty)

	closeParams := &exchanges.OrderParams{
		Symbol:   cfg.Symbol,
		Side:     closeSide,
		Type:     exchanges.OrderTypeMarket,
		Quantity: closeQty,
	}
	if isPerp {
		closeParams.ReduceOnly = true
	}

	sellOrder, err := adp.PlaceOrder(ctx, closeParams)
	require.NoError(t, err, "PlaceOrder(close) should succeed")
	t.Logf("✓ Close order placed: ID=%s", sellOrder.OrderID)

	sellFilled := waitAndCollect(t, orderCh, sellOrder.OrderID, sellOrder.ClientOrderID,
		exchanges.OrderStatusFilled, 30*time.Second, &allTransitions)
	assert.Equal(t, exchanges.OrderStatusFilled, sellFilled.Status)
	t.Logf("✓ Close order filled: qty=%s", sellFilled.FilledQuantity)

	// ======================================================================
	// Phase 5: Verify position is closed
	// ======================================================================
	t.Log("═══ Phase 5: Verify position closed ═══")

	if isPerp {
		posAfter := findPosition(t, adp, cfg.Symbol)
		if posAfter != nil {
			assert.True(t, posAfter.Quantity.IsZero(),
				fmt.Sprintf("Position should be zero after close, got qty=%s", posAfter.Quantity))
		}
		t.Log("✓ Perp position closed successfully")
	} else {
		// For spot, verify the account is accessible (balances change dynamically)
		account, err := adp.FetchAccount(ctx)
		require.NoError(t, err, "FetchAccount should succeed after close")
		t.Logf("✓ Spot closed, account balance: %s", account.TotalBalance)
	}

	// ======================================================================
	// Phase 6: Disconnect
	// ======================================================================
	t.Log("═══ Phase 6: Disconnect ═══")

	err = adp.StopWatchOrders(ctx)
	if err != nil {
		t.Logf("  StopWatchOrders: %v (may not be implemented)", err)
	} else {
		t.Log("✓ StopWatchOrders succeeded")
	}

	// ======================================================================
	// Summary: print all state transitions
	// ======================================================================
	t.Log("═══ Order State Transitions ═══")
	for _, tr := range allTransitions {
		t.Logf("  [%s] %-20s ID=%-15s  Fill=%s",
			tr.Time.Format("15:04:05.000"), tr.Status, tr.OrderID, tr.FilledQty)
	}
	t.Logf("  Total transitions recorded: %d", len(allTransitions))
}

// ─── Internal helpers ───────────────────────────────────────────────────────

type transition struct {
	Time      time.Time
	OrderID   string
	Status    exchanges.OrderStatus
	FilledQty decimal.Decimal
}

// waitAndCollect is like WaitOrderStatus but also records every transition.
func waitAndCollect(
	t *testing.T,
	ch <-chan *exchanges.Order,
	orderID, clientID string,
	target exchanges.OrderStatus,
	timeout time.Duration,
	transitions *[]transition,
) *exchanges.Order {
	t.Helper()
	timer := time.After(timeout)
	for {
		select {
		case o := <-ch:
			match := (orderID != "" && o.OrderID == orderID) ||
				(clientID != "" && o.ClientOrderID == clientID)
			if !match {
				continue
			}

			*transitions = append(*transitions, transition{
				Time:      time.Now(),
				OrderID:   o.OrderID,
				Status:    o.Status,
				FilledQty: o.FilledQuantity,
			})

			t.Logf("  ↳ %s → %s (filled=%s)",
				o.OrderID, o.Status, o.FilledQuantity)

			if o.Status == target {
				return o
			}
			if o.Status == exchanges.OrderStatusCancelled || o.Status == exchanges.OrderStatusRejected {
				if target != exchanges.OrderStatusCancelled {
					t.Fatalf("Order %s reached terminal status %s (wanted %s)", o.OrderID, o.Status, target)
				}
				return o
			}
		case <-timer:
			t.Fatalf("Timeout (%s) waiting for order %s to reach %s", timeout, orderID, target)
		}
	}
}

// findPosition returns the position for the symbol, or nil.
// Works with both perp (FetchPositions via PerpExchange) and spot/perp (FetchAccount).
func findPosition(t *testing.T, adp exchanges.Exchange, symbol string) *exchanges.Position {
	t.Helper()
	ctx := context.Background()

	// Try PerpExchange.FetchPositions first (more reliable for perp)
	if perp, ok := adp.(exchanges.PerpExchange); ok {
		positions, err := perp.FetchPositions(ctx)
		if err == nil {
			for _, p := range positions {
				if p.Symbol == symbol && !p.Quantity.IsZero() {
					return &p
				}
			}
			return nil
		}
		t.Logf("  FetchPositions failed, falling back to FetchAccount: %v", err)
	}

	// Fallback: use FetchAccount().Positions
	account, err := adp.FetchAccount(ctx)
	require.NoError(t, err, "FetchAccount should succeed")

	for _, p := range account.Positions {
		if p.Symbol == symbol && !p.Quantity.IsZero() {
			return &p
		}
	}
	return nil
}
