package cache

import (
	"testing"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestCacheSnapshotCapturesRuntimeState(t *testing.T) {
	c := New()
	accountID := model.AccountID("acct")
	firstAccount := model.AccountSnapshot{AccountID: accountID, Venue: "BINANCE", Type: model.AccountTypeMargin, Timestamp: time.Unix(100, 0)}
	secondAccount := firstAccount
	secondAccount.Timestamp = time.Unix(101, 0)
	c.PutAccount(firstAccount)
	c.PutAccount(secondAccount)

	openOrder := snapshotOrder("open-order", model.OrderStatusAccepted, time.Unix(102, 0))
	closedOrder := snapshotOrder("closed-order", model.OrderStatusFilled, time.Unix(103, 0))
	require.NoError(t, c.PutOrder(openOrder))
	require.NoError(t, c.PutOrder(closedOrder))
	openPosition := snapshotPosition("open-position", model.PositionSideLong, decimal.RequireFromString("0.5"), time.Unix(104, 0))
	closedPosition := snapshotPosition("closed-position", model.PositionSideFlat, decimal.Zero, time.Unix(105, 0))
	require.NoError(t, c.PutPosition(openPosition))
	require.NoError(t, c.PutPosition(closedPosition))
	_, err := c.PutDeferredFill(snapshotFill())
	require.NoError(t, err)

	snapshot := c.Snapshot(accountID)

	require.Equal(t, accountID, snapshot.AccountID)
	require.Equal(t, secondAccount, snapshot.Account)
	require.Equal(t, []model.AccountSnapshot{firstAccount, secondAccount}, snapshot.AccountHistory)
	requireOrderIDs(t, snapshot.OpenOrders, "open-order")
	requireOrderIDs(t, snapshot.ClosedOrders, "closed-order")
	require.Len(t, snapshot.OpenPositions, 1)
	require.Len(t, snapshot.ClosedPositions, 1)
	require.Equal(t, 1, snapshot.Residuals.OpenOrders)
	require.Equal(t, 1, snapshot.Residuals.OpenPositions)
	require.Equal(t, 1, snapshot.Residuals.DeferredFills)
}

func TestCachePurgeKeepsNewestClosedStateAndClearsIndexes(t *testing.T) {
	c := New()
	accountID := model.AccountID("acct")
	for i := int64(100); i <= 102; i++ {
		c.PutAccount(model.AccountSnapshot{AccountID: accountID, Venue: "BINANCE", Type: model.AccountTypeMargin, Timestamp: time.Unix(i, 0)})
	}
	oldClosedOrder := snapshotOrder("closed-old", model.OrderStatusFilled, time.Unix(100, 0))
	newClosedOrder := snapshotOrder("closed-new", model.OrderStatusFilled, time.Unix(101, 0))
	openOrder := snapshotOrder("open-order", model.OrderStatusAccepted, time.Unix(102, 0))
	require.NoError(t, c.PutOrder(oldClosedOrder))
	require.NoError(t, c.PutOrder(newClosedOrder))
	require.NoError(t, c.PutOrder(openOrder))

	oldClosedPosition := snapshotPosition("closed-old-position", model.PositionSideFlat, decimal.Zero, time.Unix(100, 0))
	newClosedPosition := snapshotPosition("closed-new-position", model.PositionSideFlat, decimal.Zero, time.Unix(101, 0))
	openPosition := snapshotPosition("open-position", model.PositionSideLong, decimal.RequireFromString("0.5"), time.Unix(102, 0))
	require.NoError(t, c.PutPosition(oldClosedPosition))
	require.NoError(t, c.PutPosition(newClosedPosition))
	require.NoError(t, c.PutPosition(openPosition))
	_, err := c.PutDeferredFill(snapshotFill())
	require.NoError(t, err)

	result := c.Purge(accountID, PurgePolicy{
		ClosedOrdersLimit:     1,
		ClosedPositionsLimit:  1,
		AccountSnapshotsLimit: 1,
	})

	require.Equal(t, 1, result.ClosedOrders)
	require.Equal(t, 1, result.ClosedPositions)
	require.Equal(t, 2, result.AccountSnapshots)
	requireOrderIDs(t, c.ClosedOrders(accountID), "closed-new")
	requireOrderIDs(t, c.OpenOrders(accountID), "open-order")
	require.Len(t, c.ClosedPositions(accountID), 1)
	require.Len(t, c.OpenPositions(accountID), 1)
	require.Len(t, c.AccountHistory(accountID), 1)
	_, ok := c.Order(accountID, "closed-old")
	require.False(t, ok)
	require.Empty(t, c.OrdersByStrategy(accountID, oldClosedOrder.Metadata.StrategyID))
	require.Equal(t, 1, c.Residuals(accountID).DeferredFills)
}

func snapshotOrder(id model.OrderID, status model.OrderStatus, at time.Time) model.OrderStatusReport {
	quantity := decimal.RequireFromString("1")
	filled := decimal.Zero
	leaves := quantity
	if !status.IsOpen() {
		filled = quantity
		leaves = decimal.Zero
	}
	return model.OrderStatusReport{
		Metadata: model.CommandMetadata{
			StrategyID:  model.StrategyID("strategy-" + string(id)),
			ExecSpawnID: model.ExecSpawnID("spawn-" + string(id)),
		},
		AccountID:       "acct",
		InstrumentID:    model.MustInstrumentID("BTC-USDT-PERP.BINANCE"),
		OrderListID:     model.OrderListID("list-" + string(id)),
		PositionID:      model.PositionID("position-" + string(id)),
		OrderID:         id,
		VenueOrderID:    model.VenueOrderID("venue-" + string(id)),
		ClientOrderID:   model.ClientOrderID("client-" + string(id)),
		Status:          status,
		Quantity:        quantity,
		FilledQuantity:  filled,
		LeavesQuantity:  leaves,
		LastUpdatedTime: at,
	}
}

func snapshotPosition(id model.PositionID, side model.PositionSide, quantity decimal.Decimal, at time.Time) model.PositionStatusReport {
	return model.PositionStatusReport{
		Metadata:        model.CommandMetadata{StrategyID: model.StrategyID("strategy-" + string(id))},
		AccountID:       "acct",
		InstrumentID:    model.MustInstrumentID("BTC-USDT-PERP.BINANCE"),
		PositionID:      id,
		VenuePositionID: model.VenuePositionID("venue-" + string(id)),
		Side:            side,
		Quantity:        quantity,
		EntryPrice:      decimal.RequireFromString("101"),
		Timestamp:       at,
	}
}

func snapshotFill() model.FillReport {
	return model.FillReport{
		AccountID:     "acct",
		InstrumentID:  model.MustInstrumentID("BTC-USDT-PERP.BINANCE"),
		OrderID:       "open-order",
		VenueOrderID:  "venue-open-order",
		ClientOrderID: "client-open-order",
		TradeID:       "trade-open-order",
		Price:         decimal.RequireFromString("101"),
		Quantity:      decimal.RequireFromString("0.1"),
		Timestamp:     time.Unix(110, 0),
	}
}
