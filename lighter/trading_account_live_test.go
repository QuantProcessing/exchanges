package lighter

import (
	"context"
	"testing"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/account"
	"github.com/QuantProcessing/exchanges/testsuite"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestPerpAdapter_TradingAccountOrderFlowFillsLive(t *testing.T) {
	adp := setupPerpAdapter(t)
	ctx := context.Background()

	acct := account.NewTradingAccount(adp, nil)
	require.NoError(t, acct.Start(ctx))
	t.Cleanup(acct.Close)

	time.Sleep(2 * time.Second)
	flattenLighterPerpPosition(t, ctx, adp, acct, "ETH")
	t.Cleanup(func() {
		flattenLighterPerpPosition(t, ctx, adp, acct, "ETH")
	})

	qty, _ := testsuite.SmartQuantity(t, adp, "ETH")

	flow, err := acct.Place(ctx, &exchanges.OrderParams{
		Symbol:   "ETH",
		Side:     exchanges.OrderSideBuy,
		Type:     exchanges.OrderTypeMarket,
		Quantity: qty,
	})
	require.NoError(t, err)
	defer flow.Close()

	merged, fills := waitLighterOrderFlowFusion(t, flow, 45*time.Second)
	require.NotNil(t, merged)
	require.NotEmpty(t, fills)
	require.Equal(t, exchanges.OrderStatusFilled, merged.Status)
	require.True(t, merged.FilledQuantity.IsPositive())

	rawTotalQty := decimal.Zero
	rawTotalQuote := decimal.Zero
	for _, fill := range fills {
		rawTotalQty = rawTotalQty.Add(fill.Quantity)
		rawTotalQuote = rawTotalQuote.Add(fill.Price.Mul(fill.Quantity))
	}
	require.True(t, merged.FilledQuantity.Equal(rawTotalQty), "merged filled quantity %s should equal raw fill total %s", merged.FilledQuantity, rawTotalQty)

	lastFill := fills[len(fills)-1]
	require.True(t, merged.LastFillQuantity.Equal(lastFill.Quantity), "merged last fill quantity %s should equal raw last fill quantity %s", merged.LastFillQuantity, lastFill.Quantity)
	require.True(t, merged.LastFillPrice.Equal(lastFill.Price), "merged last fill price %s should equal raw last fill price %s", merged.LastFillPrice, lastFill.Price)

	expectedAverage := rawTotalQuote.Div(rawTotalQty)
	require.True(t, merged.AverageFillPrice.Equal(expectedAverage), "merged average fill price %s should equal raw weighted average %s", merged.AverageFillPrice, expectedAverage)

	closeFlow, err := acct.Place(ctx, &exchanges.OrderParams{
		Symbol:     "ETH",
		Side:       exchanges.OrderSideSell,
		Type:       exchanges.OrderTypeMarket,
		Quantity:   merged.FilledQuantity,
		ReduceOnly: true,
	})
	require.NoError(t, err)
	defer closeFlow.Close()

	closeCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	_, err = closeFlow.Wait(closeCtx, func(order *exchanges.Order) bool {
		return order.Status == exchanges.OrderStatusFilled
	})
	require.NoError(t, err)
}

func waitLighterOrderFlowFusion(t *testing.T, flow *account.OrderFlow, timeout time.Duration) (*exchanges.Order, []*exchanges.Fill) {
	t.Helper()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	var (
		latest       *exchanges.Order
		fills        []*exchanges.Fill
		rawFilledQty = decimal.Zero
	)

	for {
		if latest != nil &&
			latest.Status == exchanges.OrderStatusFilled &&
			rawFilledQty.IsPositive() &&
			rawFilledQty.GreaterThanOrEqual(latest.FilledQuantity) {
			return latest, fills
		}

		select {
		case order, ok := <-flow.C():
			if !ok {
				t.Fatalf("order flow closed before merged filled snapshot arrived")
			}
			latest = order
			t.Logf("merged order update: id=%s status=%s filled=%s lastFill=%s@%s avg=%s",
				order.OrderID, order.Status, order.FilledQuantity, order.LastFillQuantity, order.LastFillPrice, order.AverageFillPrice)
			if order.Status == exchanges.OrderStatusCancelled || order.Status == exchanges.OrderStatusRejected {
				t.Fatalf("order reached unexpected terminal status %s before filled fusion completed", order.Status)
			}
		case fill, ok := <-flow.Fills():
			if !ok {
				t.Fatalf("order flow fills channel closed before raw fills arrived")
			}
			fills = append(fills, fill)
			rawFilledQty = rawFilledQty.Add(fill.Quantity)
			t.Logf("raw fill update: order=%s trade=%s qty=%s price=%s runningTotal=%s",
				fill.OrderID, fill.TradeID, fill.Quantity, fill.Price, rawFilledQty)
		case <-timer.C:
			var latestStatus exchanges.OrderStatus
			var latestFilled decimal.Decimal
			if latest != nil {
				latestStatus = latest.Status
				latestFilled = latest.FilledQuantity
			}
			t.Fatalf("timeout waiting for lighter orderflow fill fusion: latestStatus=%s latestFilled=%s rawFilled=%s rawFills=%d", latestStatus, latestFilled, rawFilledQty, len(fills))
		}
	}
}

func flattenLighterPerpPosition(t *testing.T, ctx context.Context, adp *Adapter, acct *account.TradingAccount, symbol string) {
	t.Helper()

	pos := fetchLighterPerpPosition(t, adp, symbol)
	if pos == nil || !pos.Quantity.IsPositive() {
		return
	}

	side := exchanges.OrderSideSell
	if pos.Side == exchanges.PositionSideShort {
		side = exchanges.OrderSideBuy
	}

	t.Logf("flattening existing %s position qty=%s", pos.Side, pos.Quantity)

	flow, err := acct.Place(ctx, &exchanges.OrderParams{
		Symbol:     symbol,
		Side:       side,
		Type:       exchanges.OrderTypeMarket,
		Quantity:   pos.Quantity,
		ReduceOnly: true,
	})
	require.NoError(t, err)
	defer flow.Close()

	closeCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	_, err = flow.Wait(closeCtx, func(order *exchanges.Order) bool {
		return order.Status == exchanges.OrderStatusFilled
	})
	require.NoError(t, err)

	time.Sleep(2 * time.Second)
}

func fetchLighterPerpPosition(t *testing.T, adp *Adapter, symbol string) *exchanges.Position {
	t.Helper()

	acc, err := adp.FetchAccount(context.Background())
	require.NoError(t, err)

	for i := range acc.Positions {
		if acc.Positions[i].Symbol == symbol {
			pos := acc.Positions[i]
			return &pos
		}
	}
	return nil
}
