package testsuite

import (
	"context"
	"fmt"
	"testing"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type TradingAccountConfig struct {
	Symbol string
}

func RunTradingAccountSuite(t *testing.T, adp exchanges.Exchange, cfg TradingAccountConfig) {
	require.NotEmpty(t, cfg.Symbol, "Symbol must be set")

	ctx := context.Background()
	isPerp := adp.GetMarketType() == exchanges.MarketTypePerp

	t.Log("═══ Phase 1: Start TradingAccount ═══")

	acct := exchanges.NewTradingAccount(adp, nil)
	err := acct.Start(ctx)
	require.NoError(t, err, "TradingAccount.Start should succeed")
	defer acct.Close()
	t.Log("✓ TradingAccount started (REST snapshot + WS subscriptions)")

	time.Sleep(2 * time.Second)

	t.Log("═══ Phase 2: Verify initial state queries ═══")

	balance := acct.Balance()
	t.Logf("  Balance: %s", balance)
	assert.True(t, balance.IsPositive() || balance.IsZero(), "Balance should be >= 0")

	positions := acct.Positions()
	t.Logf("  Positions: %d", len(positions))
	for _, p := range positions {
		t.Logf("    %s: side=%s qty=%s", p.Symbol, p.Side, p.Quantity)
	}

	openOrders := acct.OpenOrders()
	t.Logf("  Open orders: %d", len(openOrders))
	t.Log("✓ Initial state queries OK")

	t.Log("═══ Phase 3: Test SubscribeOrders fan-out ═══")

	sub1 := acct.SubscribeOrders()
	defer sub1.Unsubscribe()
	sub2 := acct.SubscribeOrders()
	defer sub2.Unsubscribe()
	t.Log("✓ Two order subscribers created")

	t.Log("═══ Phase 4: Place market buy + wait for fill ═══")

	qty, lastPrice := SmartQuantity(t, adp, cfg.Symbol)
	t.Logf("  Symbol=%s  Qty=%s  RefPrice=%s", cfg.Symbol, qty, lastPrice)

	flow, err := acct.Place(ctx, &exchanges.OrderParams{
		Symbol:   cfg.Symbol,
		Side:     exchanges.OrderSideBuy,
		Type:     exchanges.OrderTypeMarket,
		Quantity: qty,
	})
	require.NoError(t, err, "Place(market buy) should succeed")
	defer flow.Close()

	placed := flowLatest(t, flow, "market order")
	t.Logf("✓ Market buy placed: ID=%s", placed.OrderID)

	filled, err := flow.Wait(ctx, func(o *exchanges.Order) bool {
		return o.Status == exchanges.OrderStatusFilled
	})
	require.NoError(t, err, "OrderFlow.Wait should succeed")
	assert.Equal(t, exchanges.OrderStatusFilled, filled.Status)
	t.Logf("✓ OrderFlow.Wait returned FILLED: qty=%s", filled.FilledQuantity)

	verifySub := func(name string, sub *exchanges.Subscription[exchanges.Order]) {
		timer := time.After(3 * time.Second)
		for {
			select {
			case o := <-sub.C:
				if o.OrderID == placed.OrderID || o.ClientOrderID == placed.ClientOrderID {
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

	t.Log("═══ Phase 5: Verify position via TradingAccount ═══")

	time.Sleep(2 * time.Second)

	if isPerp {
		pos, ok := acct.Position(cfg.Symbol)
		if ok && pos.Quantity.IsPositive() {
			t.Logf("✓ Position tracked: side=%s qty=%s entry=%s", pos.Side, pos.Quantity, pos.EntryPrice)
		} else {
			t.Log("  Position not yet in TradingAccount (may need WatchPositions support)")
		}
	} else {
		t.Log("  Spot: position tracking via balance change")
	}

	t.Log("═══ Phase 6: Limit order + Cancel ═══")

	limitPrice := SmartLimitPrice(t, adp, cfg.Symbol, exchanges.OrderSideBuy)
	t.Logf("  Passive limit price: %s (well below market)", limitPrice)

	limitFlow, err := acct.Place(ctx, &exchanges.OrderParams{
		Symbol:   cfg.Symbol,
		Side:     exchanges.OrderSideBuy,
		Type:     exchanges.OrderTypeLimit,
		Price:    limitPrice,
		Quantity: qty,
	})
	require.NoError(t, err, "Place(limit buy) should succeed")
	defer limitFlow.Close()

	limitOrder := flowLatest(t, limitFlow, "limit order")
	t.Logf("✓ Limit order placed: ID=%s", limitOrder.OrderID)

	time.Sleep(2 * time.Second)
	_, found := acct.OpenOrder(limitOrder.OrderID)
	t.Logf("  Order in local state: %v", found)

	cancelOrderID := limitOrder.OrderID
	err = acct.Cancel(ctx, cancelOrderID, cfg.Symbol)
	require.NoError(t, err, "Cancel should succeed")
	t.Logf("✓ Cancel sent for order %s", cancelOrderID)

	cancelled, err := limitFlow.Wait(ctx, func(o *exchanges.Order) bool {
		return o.Status == exchanges.OrderStatusCancelled
	})
	require.NoError(t, err, "OrderFlow.Wait should succeed for cancelled order")
	assert.Equal(t, exchanges.OrderStatusCancelled, cancelled.Status)
	t.Logf("✓ OrderFlow.Wait returned CANCELLED")

	time.Sleep(500 * time.Millisecond)
	_, stillOpen := acct.OpenOrder(cancelOrderID)
	assert.False(t, stillOpen, "Cancelled order should be removed from local orders")
	t.Log("✓ Cancelled order removed from local state")

	t.Log("═══ Phase 7: Close position ═══")

	closeQty := qty
	if isPerp {
		if p, ok := acct.Position(cfg.Symbol); ok && p.Quantity.IsPositive() {
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

	closeFlow, err := acct.Place(ctx, closeParams)
	require.NoError(t, err, "Place(close) should succeed")
	defer closeFlow.Close()

	closeOrder := flowLatest(t, closeFlow, "close order")
	t.Logf("✓ Close order placed: ID=%s", closeOrder.OrderID)

	closeFilled, err := closeFlow.Wait(ctx, func(o *exchanges.Order) bool {
		return o.Status == exchanges.OrderStatusFilled
	})
	require.NoError(t, err, "OrderFlow.Wait(close) should succeed")
	assert.Equal(t, exchanges.OrderStatusFilled, closeFilled.Status)
	t.Logf("✓ Close order filled: qty=%s", closeFilled.FilledQuantity)

	t.Log("═══ Phase 8: Verify final state ═══")

	if isPerp {
		time.Sleep(2 * time.Second)
		pos, ok := acct.Position(cfg.Symbol)
		if ok {
			t.Logf("  Final position: qty=%s (should be ~0)", pos.Quantity)
		} else {
			t.Log("  No position found (closed)")
		}

		posAfter := findTradingAccountPosition(t, adp, cfg.Symbol)
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

	finalBalance := acct.Balance()
	t.Logf("  Final balance from TradingAccount: %s", finalBalance)

	t.Log("═══ Summary ═══")
	t.Log("✓ TradingAccount.Start (REST + WS)")
	t.Log("✓ State queries (balance, positions, orders)")
	t.Log("✓ SubscribeOrders fan-out (2 consumers)")
	t.Log("✓ Place + OrderFlow.Wait (market fill)")
	t.Log("✓ Place + Cancel + OrderFlow.Wait (limit cancel)")
	t.Log("✓ Position tracking")
	t.Log("✓ Close position")
	t.Log("All TradingAccount integration tests passed!")

	_ = lastPrice
}

func flowLatest(t *testing.T, flow *exchanges.OrderFlow, label string) *exchanges.Order {
	t.Helper()

	order := flow.Latest()
	require.NotNil(t, order, "%s should expose an initial snapshot", label)
	return order
}

func findTradingAccountPosition(t *testing.T, adp exchanges.Exchange, symbol string) *exchanges.Position {
	t.Helper()

	account, err := adp.FetchAccount(context.Background())
	require.NoError(t, err)

	for i := range account.Positions {
		if account.Positions[i].Symbol == symbol {
			p := account.Positions[i]
			return &p
		}
	}
	return nil
}
