package account

import (
	"testing"
	"time"

	"github.com/QuantProcessing/exchanges/cache"
	"github.com/QuantProcessing/exchanges/model"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestReconcilerAppliesExecutionEventsToCache(t *testing.T) {
	c := cache.New()
	r := NewReconciler(c)
	account := model.AccountSnapshot{AccountID: "acct", Venue: "BINANCE"}
	require.NoError(t, r.Apply(model.ExecutionEvent{Account: &account}))

	order := model.OrderStatusReport{
		AccountID:    "acct",
		InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
		OrderID:      "order-1",
		Status:       model.OrderStatusAccepted,
	}
	require.NoError(t, r.Apply(model.ExecutionEvent{Order: &order}))

	_, ok := c.Account("acct")
	require.True(t, ok)
	_, ok = c.Order("acct", "order-1")
	require.True(t, ok)
}

func TestReconcilerAppliesOrderLifecycleInOrder(t *testing.T) {
	c := cache.New()
	r := NewReconciler(c)
	instID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	accepted := model.OrderStatusReport{
		AccountID:      "acct",
		InstrumentID:   instID,
		OrderID:        "order-1",
		ClientOrderID:  "client-1",
		VenueOrderID:   "venue-1",
		Status:         model.OrderStatusAccepted,
		Quantity:       decimal.RequireFromString("1"),
		LeavesQuantity: decimal.RequireFromString("1"),
	}
	require.NoError(t, r.Apply(model.ExecutionEvent{Order: &accepted}))

	fill := model.FillReport{
		AccountID:    "acct",
		InstrumentID: instID,
		OrderID:      "order-1",
		TradeID:      "trade-1",
		Price:        decimal.RequireFromString("100"),
		Quantity:     decimal.RequireFromString("0.4"),
		Timestamp:    time.Unix(100, 0),
	}
	require.NoError(t, r.Apply(model.ExecutionEvent{Fill: &fill}))

	order, ok := c.Order("acct", "order-1")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusPartiallyFilled, order.Status)
	require.True(t, decimal.RequireFromString("0.4").Equal(order.FilledQuantity))
	require.True(t, decimal.RequireFromString("0.6").Equal(order.LeavesQuantity))

	fill.TradeID = "trade-2"
	fill.Quantity = decimal.RequireFromString("0.6")
	require.NoError(t, r.Apply(model.ExecutionEvent{Fill: &fill}))

	order, ok = c.OrderByVenueID("acct", "venue-1")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusFilled, order.Status)
	require.True(t, decimal.RequireFromString("1").Equal(order.FilledQuantity))
	require.True(t, order.LeavesQuantity.IsZero())
}

func TestReconcilerDeduplicatesFillsByTradeID(t *testing.T) {
	c := cache.New()
	r := NewReconciler(c)
	instID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	order := model.OrderStatusReport{
		AccountID:      "acct",
		InstrumentID:   instID,
		OrderID:        "order-1",
		Status:         model.OrderStatusAccepted,
		Quantity:       decimal.RequireFromString("1"),
		LeavesQuantity: decimal.RequireFromString("1"),
	}
	require.NoError(t, r.Apply(model.ExecutionEvent{Order: &order}))

	fill := model.FillReport{
		AccountID:    "acct",
		InstrumentID: instID,
		OrderID:      "order-1",
		TradeID:      "trade-1",
		Price:        decimal.RequireFromString("100"),
		Quantity:     decimal.RequireFromString("0.5"),
	}
	require.NoError(t, r.Apply(model.ExecutionEvent{Fill: &fill}))
	require.NoError(t, r.Apply(model.ExecutionEvent{Fill: &fill}))

	fills := c.FillsForOrder("acct", "order-1")
	require.Len(t, fills, 1)
	gotOrder, ok := c.Order("acct", "order-1")
	require.True(t, ok)
	require.True(t, decimal.RequireFromString("0.5").Equal(gotOrder.FilledQuantity))
}

func TestReconcilerReplaysFillsArrivingBeforeOrderReport(t *testing.T) {
	c := cache.New()
	r := NewReconciler(c)
	instID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	fill := model.FillReport{
		AccountID:    "acct",
		InstrumentID: instID,
		OrderID:      "order-late",
		TradeID:      "trade-early",
		Side:         model.OrderSideBuy,
		Price:        decimal.RequireFromString("100"),
		Quantity:     decimal.RequireFromString("0.4"),
		Timestamp:    time.Unix(100, 0),
	}
	require.NoError(t, r.Apply(model.ExecutionEvent{Fill: &fill}))

	order := model.OrderStatusReport{
		AccountID:       "acct",
		InstrumentID:    instID,
		OrderID:         "order-late",
		Status:          model.OrderStatusAccepted,
		Quantity:        decimal.RequireFromString("1"),
		LeavesQuantity:  decimal.RequireFromString("1"),
		LastUpdatedTime: time.Unix(99, 0),
	}
	require.NoError(t, r.Apply(model.ExecutionEvent{Order: &order}))

	gotOrder, ok := c.Order("acct", "order-late")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusPartiallyFilled, gotOrder.Status)
	require.True(t, decimal.RequireFromString("0.4").Equal(gotOrder.FilledQuantity))
	require.True(t, decimal.RequireFromString("0.6").Equal(gotOrder.LeavesQuantity))
	require.True(t, decimal.RequireFromString("100").Equal(gotOrder.AveragePrice))
}

func TestReconcilerMarksOpenOrdersMissingFromSnapshotCanceled(t *testing.T) {
	c := cache.New()
	r := NewReconciler(c)
	instID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	missing := model.OrderStatusReport{
		AccountID:      "acct",
		InstrumentID:   instID,
		OrderID:        "missing-order",
		ClientOrderID:  "missing-client",
		VenueOrderID:   "missing-venue",
		Status:         model.OrderStatusAccepted,
		Quantity:       decimal.RequireFromString("1"),
		LeavesQuantity: decimal.RequireFromString("1"),
	}
	observed := model.OrderStatusReport{
		AccountID:      "acct",
		InstrumentID:   instID,
		OrderID:        "observed-order",
		ClientOrderID:  "observed-client",
		VenueOrderID:   "observed-venue",
		Status:         model.OrderStatusAccepted,
		Quantity:       decimal.RequireFromString("1"),
		LeavesQuantity: decimal.RequireFromString("1"),
	}
	require.NoError(t, r.Apply(model.ExecutionEvent{Order: &missing}))
	require.NoError(t, r.Apply(model.ExecutionEvent{Order: &observed}))

	generated, err := r.ReconcileMissingOpenOrders("acct", instID, []model.OrderStatusReport{observed}, model.OrderStatusCanceled)
	require.NoError(t, err)
	require.Len(t, generated, 1)
	require.Equal(t, model.OrderID("missing-order"), generated[0].OrderID)
	require.Equal(t, model.OrderStatusCanceled, generated[0].Status)

	gotMissing, ok := c.Order("acct", "missing-order")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusCanceled, gotMissing.Status)
	gotObserved, ok := c.Order("acct", "observed-order")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusAccepted, gotObserved.Status)
}

func TestReconcilerSkipsRecentMissingOpenOrdersUntilRepairThreshold(t *testing.T) {
	c := cache.New()
	r := NewReconciler(c)
	instID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	now := time.Unix(200, 0)
	recent := model.OrderStatusReport{
		AccountID:       "acct",
		InstrumentID:    instID,
		OrderID:         "recent-order",
		ClientOrderID:   "recent-client",
		Status:          model.OrderStatusAccepted,
		Quantity:        decimal.RequireFromString("1"),
		LeavesQuantity:  decimal.RequireFromString("1"),
		LastUpdatedTime: now.Add(-10 * time.Second),
	}
	stale := recent
	stale.OrderID = "stale-order"
	stale.ClientOrderID = "stale-client"
	stale.LastUpdatedTime = now.Add(-2 * time.Minute)
	require.NoError(t, r.Apply(model.ExecutionEvent{Order: &recent}))
	require.NoError(t, r.Apply(model.ExecutionEvent{Order: &stale}))

	generated, err := r.ReconcileMissingOpenOrdersWithPolicy("acct", instID, nil, MissingOpenOrderRepairPolicy{
		MissingStatus:        model.OrderStatusCanceled,
		RecentActivityWindow: time.Minute,
		Now:                  now,
	})
	require.NoError(t, err)
	require.Len(t, generated, 1)
	require.Equal(t, model.OrderID("stale-order"), generated[0].OrderID)
	require.Equal(t, model.OrderStatusCanceled, generated[0].Status)

	gotRecent, ok := c.Order("acct", "recent-order")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusAccepted, gotRecent.Status)
	gotStale, ok := c.Order("acct", "stale-order")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusCanceled, gotStale.Status)
}

func TestReconcilerGeneratesFlatPositionReportsMissingFromSnapshot(t *testing.T) {
	c := cache.New()
	r := NewReconciler(c)
	instID := model.MustInstrumentID("BTC-USDT-PERP.BINANCE")
	position := model.PositionStatusReport{
		AccountID:    "acct",
		InstrumentID: instID,
		PositionID:   "pos-1",
		Side:         model.PositionSideLong,
		Quantity:     decimal.RequireFromString("1"),
		EntryPrice:   decimal.RequireFromString("100"),
	}
	require.NoError(t, r.Apply(model.ExecutionEvent{Position: &position}))

	generated, err := r.MissingPositionReports("acct", instID, nil)
	require.NoError(t, err)
	require.Len(t, generated, 1)
	require.Equal(t, model.PositionID("pos-1"), generated[0].PositionID)
	require.Equal(t, model.PositionSideFlat, generated[0].Side)
	require.True(t, generated[0].Quantity.IsZero())

	require.NoError(t, r.Apply(model.ExecutionEvent{Position: &generated[0]}))
	got, ok := c.PositionByInstrument("acct", instID)
	require.True(t, ok)
	require.Equal(t, model.PositionSideFlat, got.Side)
	require.True(t, got.Quantity.IsZero())
}

func TestReconcilerRepairsMissingAndStalePositionsUntilRetryLimit(t *testing.T) {
	c := cache.New()
	r := NewReconciler(c)
	instID := model.MustInstrumentID("BTC-USDT-PERP.BINANCE")
	missing := model.PositionStatusReport{
		AccountID:    "acct",
		InstrumentID: instID,
		PositionID:   "missing-position",
		Side:         model.PositionSideLong,
		Quantity:     decimal.RequireFromString("1"),
		EntryPrice:   decimal.RequireFromString("100"),
	}
	stale := model.PositionStatusReport{
		AccountID:    "acct",
		InstrumentID: instID,
		PositionID:   "stale-position",
		Side:         model.PositionSideLong,
		Quantity:     decimal.RequireFromString("2"),
		EntryPrice:   decimal.RequireFromString("100"),
	}
	venueStale := stale
	venueStale.Quantity = decimal.RequireFromString("1")
	venueStale.EntryPrice = decimal.RequireFromString("101")
	require.NoError(t, r.Apply(model.ExecutionEvent{Position: &missing}))
	require.NoError(t, r.Apply(model.ExecutionEvent{Position: &stale}))

	first, err := r.RepairPositionReports("acct", instID, []model.PositionStatusReport{venueStale}, PositionRepairPolicy{MaxAttempts: 2})
	require.NoError(t, err)
	require.Len(t, first.Generated, 2)
	require.Empty(t, first.Unresolved)
	requirePositionRepair(t, first.Generated, "missing-position", model.PositionSideFlat, "0")
	requirePositionRepair(t, first.Generated, "stale-position", model.PositionSideLong, "1")

	second, err := r.RepairPositionReports("acct", instID, []model.PositionStatusReport{venueStale}, PositionRepairPolicy{MaxAttempts: 2})
	require.NoError(t, err)
	require.Len(t, second.Generated, 2)
	require.Empty(t, second.Unresolved)

	third, err := r.RepairPositionReports("acct", instID, []model.PositionStatusReport{venueStale}, PositionRepairPolicy{MaxAttempts: 2})
	require.NoError(t, err)
	require.Empty(t, third.Generated)
	require.Len(t, third.Unresolved, 2)
	requirePositionDiscrepancy(t, third.Unresolved, "missing-position", "position_missing_from_venue", 2)
	requirePositionDiscrepancy(t, third.Unresolved, "stale-position", "position_quantity_mismatch", 2)
}

func TestReconcilerUpdatesPositionsAndRejectsInvalidTransitions(t *testing.T) {
	c := cache.New()
	r := NewReconciler(c)
	instID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	filled := model.OrderStatusReport{
		AccountID:    "acct",
		InstrumentID: instID,
		OrderID:      "order-1",
		Status:       model.OrderStatusFilled,
		Quantity:     decimal.RequireFromString("1"),
	}
	require.NoError(t, r.Apply(model.ExecutionEvent{Order: &filled}))

	accepted := filled
	accepted.Status = model.OrderStatusAccepted
	require.Error(t, r.Apply(model.ExecutionEvent{Order: &accepted}))

	position := model.PositionStatusReport{
		AccountID:    "acct",
		InstrumentID: instID,
		PositionID:   "BTC-USDT-SPOT.BINANCE",
		Quantity:     decimal.RequireFromString("1"),
		EntryPrice:   decimal.RequireFromString("100"),
	}
	require.NoError(t, r.Apply(model.ExecutionEvent{Position: &position}))

	gotPosition, ok := c.PositionByInstrument("acct", instID)
	require.True(t, ok)
	require.Equal(t, position, gotPosition)
}

func requirePositionRepair(t *testing.T, reports []model.PositionStatusReport, positionID model.PositionID, side model.PositionSide, quantity string) {
	t.Helper()
	for _, report := range reports {
		if report.PositionID != positionID {
			continue
		}
		require.Equal(t, side, report.Side)
		require.True(t, decimal.RequireFromString(quantity).Equal(report.Quantity), "position %s quantity %s != %s", positionID, report.Quantity, quantity)
		return
	}
	require.Failf(t, "missing position repair", "position %s not found in %+v", positionID, reports)
}

func requirePositionDiscrepancy(t *testing.T, discrepancies []PositionRepairDiscrepancy, positionID model.PositionID, kind string, attempts int) {
	t.Helper()
	for _, discrepancy := range discrepancies {
		if discrepancy.PositionID != positionID {
			continue
		}
		require.Equal(t, kind, discrepancy.Kind)
		require.Equal(t, attempts, discrepancy.Attempts)
		return
	}
	require.Failf(t, "missing position discrepancy", "position %s not found in %+v", positionID, discrepancies)
}

func TestReconcilerAcceptsVenueFillsWhileCancelPending(t *testing.T) {
	c := cache.New()
	r := NewReconciler(c)
	instID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	order := model.OrderStatusReport{
		AccountID:      "acct",
		InstrumentID:   instID,
		OrderID:        "order-pending-cancel-fill",
		ClientOrderID:  "client-pending-cancel-fill",
		Status:         model.OrderStatusAccepted,
		Quantity:       decimal.RequireFromString("1"),
		LeavesQuantity: decimal.RequireFromString("1"),
	}
	require.NoError(t, r.Apply(model.ExecutionEvent{Order: &order}))
	order.Status = model.OrderStatusPendingCancel
	require.NoError(t, r.Apply(model.ExecutionEvent{Order: &order}))

	order.Status = model.OrderStatusPartiallyFilled
	order.FilledQuantity = decimal.RequireFromString("0.4")
	order.LeavesQuantity = decimal.RequireFromString("0.6")
	require.NoError(t, r.Apply(model.ExecutionEvent{Order: &order}))

	got, ok := c.Order("acct", "order-pending-cancel-fill")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusPartiallyFilled, got.Status)
	require.True(t, decimal.RequireFromString("0.4").Equal(got.FilledQuantity))
}

func TestReconcilerKeepsPendingUpdateOnSubmittedEcho(t *testing.T) {
	c := cache.New()
	r := NewReconciler(c)
	instID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	order := model.OrderStatusReport{
		AccountID:      "acct",
		InstrumentID:   instID,
		OrderID:        "order-pending-update-echo",
		ClientOrderID:  "client-pending-update-echo",
		Status:         model.OrderStatusAccepted,
		Quantity:       decimal.RequireFromString("1"),
		LeavesQuantity: decimal.RequireFromString("1"),
	}
	require.NoError(t, r.Apply(model.ExecutionEvent{Order: &order}))
	order.Status = model.OrderStatusPendingUpdate
	require.NoError(t, r.Apply(model.ExecutionEvent{Order: &order}))

	order.Status = model.OrderStatusSubmitted
	require.NoError(t, r.Apply(model.ExecutionEvent{Order: &order}))

	got, ok := c.Order("acct", "order-pending-update-echo")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusPendingUpdate, got.Status)
}
