package cache

import (
	"testing"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestCacheIndexesOrdersForRuntimeQueries(t *testing.T) {
	c := New()
	instID := model.MustInstrumentID("BTC-USDT-PERP.BINANCE")
	order := model.OrderStatusReport{
		Metadata: model.CommandMetadata{
			StrategyID:      "strategy-001",
			ExecSpawnID:     "spawn-001",
			ExecAlgorithmID: "twap-001",
		},
		AccountID:      "acct",
		InstrumentID:   instID,
		OrderListID:    "list-001",
		OrderID:        "order-001",
		VenueOrderID:   "venue-order-001",
		ClientOrderID:  "client-order-001",
		Status:         model.OrderStatusAccepted,
		PositionID:     "position-001",
		Quantity:       decimal.RequireFromString("1"),
		LeavesQuantity: decimal.RequireFromString("1"),
	}

	require.NoError(t, c.PutOrder(order))

	requireOrderIDs(t, c.OrdersByStrategy("acct", "strategy-001"), "order-001")
	requireOrderIDs(t, c.OrdersByPositionID("acct", "position-001"), "order-001")
	requireOrderIDs(t, c.OrdersByOrderListID("acct", "list-001"), "order-001")
	requireOrderIDs(t, c.OrdersByExecSpawnID("acct", "spawn-001"), "order-001")
	require.Empty(t, c.ClosedOrders("acct"))

	order.Status = model.OrderStatusFilled
	order.FilledQuantity = decimal.RequireFromString("1")
	order.LeavesQuantity = decimal.Zero
	require.NoError(t, c.PutOrder(order))
	require.Empty(t, c.OpenOrders("acct"))
	requireOrderIDs(t, c.ClosedOrders("acct"), "order-001")

	residuals := c.Residuals("acct")
	require.Zero(t, residuals.OpenOrders)
	require.Zero(t, residuals.DeferredFills)
}

func TestCacheIndexesFillsDeferredFillsAndPositions(t *testing.T) {
	c := New()
	instID := model.MustInstrumentID("BTC-USDT-PERP.BINANCE")
	fill := model.FillReport{
		AccountID:     "acct",
		InstrumentID:  instID,
		OrderID:       "order-001",
		VenueOrderID:  "venue-order-001",
		ClientOrderID: "client-order-001",
		TradeID:       "trade-001",
		Price:         decimal.RequireFromString("101"),
		Quantity:      decimal.RequireFromString("0.4"),
		Timestamp:     testTime,
	}

	stored, err := c.PutDeferredFill(fill)
	require.NoError(t, err)
	require.True(t, stored)
	stored, err = c.PutDeferredFill(fill)
	require.NoError(t, err)
	require.False(t, stored)
	require.Len(t, c.DeferredFillsForOrder("acct", "order-001"), 1)
	require.Equal(t, 1, c.Residuals("acct").DeferredFills)

	c.ClearDeferredFillsForOrder("acct", "order-001")
	require.Empty(t, c.DeferredFillsForOrder("acct", "order-001"))

	stored, err = c.PutFill(fill)
	require.NoError(t, err)
	require.True(t, stored)
	gotFill, ok := c.FillByTradeID("acct", "trade-001")
	require.True(t, ok)
	require.Equal(t, fill, gotFill)
	require.Len(t, c.FillsByVenueOrderID("acct", "venue-order-001"), 1)

	position := model.PositionStatusReport{
		Metadata:        model.CommandMetadata{StrategyID: "strategy-001"},
		AccountID:       "acct",
		InstrumentID:    instID,
		PositionID:      "position-001",
		VenuePositionID: "venue-position-001",
		Side:            model.PositionSideLong,
		Quantity:        decimal.RequireFromString("0.4"),
		EntryPrice:      decimal.RequireFromString("101"),
		Timestamp:       testTime,
	}
	require.NoError(t, c.PutPosition(position))

	gotPosition, ok := c.PositionByVenueID("acct", "venue-position-001")
	require.True(t, ok)
	require.Equal(t, position, gotPosition)
	require.Len(t, c.PositionsByStrategy("acct", "strategy-001"), 1)
	require.Len(t, c.OpenPositions("acct"), 1)
	require.Equal(t, 1, c.Residuals("acct").OpenPositions)

	position.Side = model.PositionSideFlat
	position.Quantity = decimal.Zero
	require.NoError(t, c.PutPosition(position))
	require.Empty(t, c.OpenPositions("acct"))
	require.Len(t, c.ClosedPositions("acct"), 1)
}

func TestCacheKeepsAccountSnapshotHistory(t *testing.T) {
	c := New()
	first := model.AccountSnapshot{AccountID: "acct", Venue: "BINANCE", Type: model.AccountTypeMargin, Timestamp: testTime}
	second := first
	second.Timestamp = testTime.Add(1)

	c.PutAccount(first)
	c.PutAccount(second)

	latest, ok := c.Account("acct")
	require.True(t, ok)
	require.Equal(t, second, latest)
	require.Equal(t, []model.AccountSnapshot{first, second}, c.AccountHistory("acct"))
}

func requireOrderIDs(t *testing.T, orders []model.OrderStatusReport, ids ...model.OrderID) {
	t.Helper()
	got := make([]model.OrderID, 0, len(orders))
	for _, order := range orders {
		got = append(got, order.OrderID)
	}
	require.Equal(t, ids, got)
}
