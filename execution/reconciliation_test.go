package execution

import (
	"testing"
	"time"

	"github.com/QuantProcessing/exchanges/cache"
	"github.com/QuantProcessing/exchanges/model"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestReconcilerAppliesMassStatusToCacheWithAudit(t *testing.T) {
	c := cache.New()
	reconciler := NewReconciler(ReconciliationConfig{Cache: c})
	accountID := model.AccountID("acct")
	instrumentID := executionTestInstrumentID()
	order := executionTestOrderReport("venue-order-1", "client-rec-1", model.OrderStatusAccepted)
	order.AccountID = accountID
	order.InstrumentID = instrumentID
	order.Quantity = decimal.RequireFromString("2")
	order.LeavesQuantity = decimal.RequireFromString("2")
	fill := model.FillReport{
		AccountID:     accountID,
		InstrumentID:  instrumentID,
		OrderID:       order.OrderID,
		ClientOrderID: order.ClientOrderID,
		TradeID:       "trade-rec-1",
		PositionID:    "pos-rec-1",
		Side:          model.OrderSideBuy,
		Quantity:      decimal.RequireFromString("0.75"),
		Price:         decimal.RequireFromString("100"),
		Fee:           decimal.RequireFromString("0.01"),
		FeeCurrency:   "USDT",
		Timestamp:     time.Unix(10, 0),
	}
	position := model.PositionStatusReport{
		AccountID:    accountID,
		InstrumentID: instrumentID,
		PositionID:   "pos-rec-1",
		Side:         model.PositionSideLong,
		Quantity:     decimal.RequireFromString("0.75"),
		EntryPrice:   decimal.RequireFromString("100"),
	}
	account := model.AccountSnapshot{
		AccountID: accountID,
		Venue:     "BINANCE",
		Balances: []model.Balance{{
			Currency: "USDT",
			Free:     "999.99",
			Locked:   "0.01",
			Total:    "1000",
		}},
	}

	result, err := reconciler.ReconcileMassStatus(model.ExecutionMassStatus{
		AccountID: accountID,
		Venue:     "BINANCE",
		Accounts:  []model.AccountSnapshot{account},
		Orders:    []model.OrderStatusReport{order},
		Fills:     []model.FillReport{fill},
		Positions: []model.PositionStatusReport{position},
		Timestamp: time.Unix(11, 0),
	})
	require.NoError(t, err)
	require.Equal(t, "TC-REC01", result.CaseID)
	require.Equal(t, 1, result.AccountsApplied)
	require.Equal(t, 1, result.OrdersApplied)
	require.Equal(t, 1, result.FillsApplied)
	require.Equal(t, 1, result.PositionsApplied)
	require.Empty(t, result.Unresolved)
	require.Equal(t, result, reconciler.LastResult())

	gotAccount, ok := c.Account(accountID)
	require.True(t, ok)
	require.Equal(t, account.Balances[0].Free, gotAccount.Balances[0].Free)
	gotOrder, ok := c.Order(accountID, order.OrderID)
	require.True(t, ok)
	require.Equal(t, model.OrderStatusPartiallyFilled, gotOrder.Status)
	require.Equal(t, decimal.RequireFromString("0.75"), gotOrder.FilledQuantity)
	require.Equal(t, decimal.RequireFromString("1.25"), gotOrder.LeavesQuantity)
	gotFill, ok := c.FillByTradeID(accountID, fill.TradeID)
	require.True(t, ok)
	require.Equal(t, fill.OrderID, gotFill.OrderID)
	gotPosition, ok := c.Position(accountID, position.PositionID)
	require.True(t, ok)
	require.Equal(t, position.Quantity, gotPosition.Quantity)
}

func TestReconcilerAppliesOnlyMissingFillsInsideLookback(t *testing.T) {
	c := cache.New()
	reconciler := NewReconciler(ReconciliationConfig{Cache: c})
	accountID := model.AccountID("acct")
	instrumentID := executionTestInstrumentID()
	order := executionTestOrderReport("venue-order-2", "client-rec-2", model.OrderStatusAccepted)
	order.AccountID = accountID
	order.InstrumentID = instrumentID
	order.Quantity = decimal.RequireFromString("3")
	order.LeavesQuantity = decimal.RequireFromString("3")
	require.NoError(t, c.PutOrder(order))
	existingFill := model.FillReport{
		AccountID:     accountID,
		InstrumentID:  instrumentID,
		OrderID:       order.OrderID,
		ClientOrderID: order.ClientOrderID,
		TradeID:       "trade-existing",
		Side:          model.OrderSideBuy,
		Quantity:      decimal.RequireFromString("0.25"),
		Price:         decimal.RequireFromString("100"),
		Timestamp:     time.Unix(100, 0),
	}
	stored, err := c.PutFill(existingFill)
	require.NoError(t, err)
	require.True(t, stored)
	oldFill := existingFill
	oldFill.TradeID = "trade-old"
	oldFill.Quantity = decimal.RequireFromString("0.5")
	oldFill.Timestamp = time.Unix(50, 0)
	missingFill := existingFill
	missingFill.TradeID = "trade-missing"
	missingFill.Quantity = decimal.RequireFromString("0.75")
	missingFill.Timestamp = time.Unix(110, 0)

	result, err := reconciler.ReconcileMissingFills([]model.FillReport{existingFill, oldFill, missingFill}, time.Unix(90, 0))
	require.NoError(t, err)
	require.Equal(t, "TC-REC02", result.CaseID)
	require.Equal(t, 3, result.ReportsScanned)
	require.Equal(t, 1, result.FillsApplied)
	require.Equal(t, 1, result.DuplicatesSkipped)
	require.Equal(t, 1, result.LookbackSkipped)
	require.Empty(t, result.Unresolved)

	_, ok := c.FillByTradeID(accountID, missingFill.TradeID)
	require.True(t, ok)
	_, ok = c.FillByTradeID(accountID, oldFill.TradeID)
	require.False(t, ok)
	gotOrder, ok := c.Order(accountID, order.OrderID)
	require.True(t, ok)
	require.Equal(t, decimal.RequireFromString("0.75"), gotOrder.FilledQuantity)
	require.Equal(t, decimal.RequireFromString("2.25"), gotOrder.LeavesQuantity)
}

func TestReconcilerDefersFillBeforeOrderAndReplaysWhenOrderAppears(t *testing.T) {
	c := cache.New()
	reconciler := NewReconciler(ReconciliationConfig{Cache: c})
	accountID := model.AccountID("acct")
	instrumentID := executionTestInstrumentID()
	fill := model.FillReport{
		AccountID:     accountID,
		InstrumentID:  instrumentID,
		OrderID:       "venue-order-5",
		ClientOrderID: "client-rec-5",
		TradeID:       "trade-before-order",
		Side:          model.OrderSideBuy,
		Quantity:      decimal.RequireFromString("0.4"),
		Price:         decimal.RequireFromString("100"),
		Timestamp:     time.Unix(120, 0),
	}

	missing, err := reconciler.ReconcileMissingFills([]model.FillReport{fill}, time.Unix(100, 0))
	require.NoError(t, err)
	require.Equal(t, "TC-REC02", missing.CaseID)
	require.Equal(t, 1, missing.ReportsScanned)
	require.Equal(t, 0, missing.FillsApplied)
	require.Equal(t, 1, missing.FillsDeferred)
	require.Len(t, c.DeferredFillsForOrder(accountID, fill.OrderID), 1)
	_, ok := c.FillByTradeID(accountID, fill.TradeID)
	require.False(t, ok)

	order := executionTestOrderReport(fill.OrderID, fill.ClientOrderID, model.OrderStatusAccepted)
	order.AccountID = accountID
	order.InstrumentID = instrumentID
	order.Quantity = decimal.RequireFromString("1")
	order.LeavesQuantity = decimal.RequireFromString("1")
	massStatus, err := reconciler.ReconcileMassStatus(model.ExecutionMassStatus{
		AccountID: accountID,
		Venue:     "BINANCE",
		Orders:    []model.OrderStatusReport{order},
		Timestamp: time.Unix(121, 0),
	})
	require.NoError(t, err)
	require.Equal(t, "TC-REC01", massStatus.CaseID)
	require.Equal(t, 1, massStatus.OrdersApplied)
	require.Empty(t, c.DeferredFillsForOrder(accountID, fill.OrderID))
	_, ok = c.FillByTradeID(accountID, fill.TradeID)
	require.True(t, ok)
	cached, ok := c.Order(accountID, fill.OrderID)
	require.True(t, ok)
	require.Equal(t, model.OrderStatusPartiallyFilled, cached.Status)
	require.Equal(t, decimal.RequireFromString("0.4"), cached.FilledQuantity)
	require.Equal(t, decimal.RequireFromString("0.6"), cached.LeavesQuantity)
}

func TestReconcilerDetectsOrderDiscrepancyStateAndFilledQuantity(t *testing.T) {
	c := cache.New()
	reconciler := NewReconciler(ReconciliationConfig{Cache: c})
	accountID := model.AccountID("acct")
	instrumentID := executionTestInstrumentID()
	local := executionTestOrderReport("venue-order-3", "client-rec-3", model.OrderStatusAccepted)
	local.AccountID = accountID
	local.InstrumentID = instrumentID
	local.Quantity = decimal.RequireFromString("2")
	local.FilledQuantity = decimal.RequireFromString("0.25")
	local.LeavesQuantity = decimal.RequireFromString("1.75")
	require.NoError(t, c.PutOrder(local))
	venue := local
	venue.Status = model.OrderStatusFilled
	venue.FilledQuantity = decimal.RequireFromString("2")
	venue.LeavesQuantity = decimal.Zero

	result, err := reconciler.DetectOrderDiscrepancies([]model.OrderStatusReport{venue})
	require.NoError(t, err)
	require.Equal(t, "TC-REC03", result.CaseID)
	require.Equal(t, 1, result.ReportsScanned)
	require.Len(t, result.Unresolved, 2)
	require.Equal(t, "order_open_state_mismatch", result.Unresolved[0].Kind)
	require.Equal(t, local.OrderID, result.Unresolved[0].OrderID)
	require.Equal(t, "order_filled_quantity_mismatch", result.Unresolved[1].Kind)
	require.Equal(t, local.OrderID, result.Unresolved[1].OrderID)

	cached, ok := c.Order(accountID, local.OrderID)
	require.True(t, ok)
	require.Equal(t, model.OrderStatusAccepted, cached.Status)
	require.Equal(t, decimal.RequireFromString("0.25"), cached.FilledQuantity)
}

func TestReconcilerImportsOrRejectsExternalOrdersExplicitly(t *testing.T) {
	accountID := model.AccountID("acct")
	instrumentID := executionTestInstrumentID()
	external := executionTestOrderReport("external-order-8", "", model.OrderStatusAccepted)
	external.AccountID = accountID
	external.InstrumentID = instrumentID
	external.ClientOrderID = ""
	external.Metadata = model.CommandMetadata{}

	importCache := cache.New()
	importReconciler := NewReconciler(ReconciliationConfig{Cache: importCache})
	imported, err := importReconciler.ReconcileExternalOrders([]model.OrderStatusReport{external}, ExternalOrderPolicy{
		AllowImport: true,
		StrategyID:  "EXTERNAL",
	})
	require.NoError(t, err)
	require.Equal(t, "TC-REC08", imported.CaseID)
	require.Equal(t, 1, imported.ReportsScanned)
	require.Equal(t, 1, imported.OrdersApplied)
	require.Empty(t, imported.Unresolved)
	cached, ok := importCache.Order(accountID, external.OrderID)
	require.True(t, ok)
	require.Equal(t, model.StrategyID("EXTERNAL"), cached.Metadata.StrategyID)
	require.Len(t, importCache.OrdersByStrategy(accountID, "EXTERNAL"), 1)

	rejectCache := cache.New()
	rejectReconciler := NewReconciler(ReconciliationConfig{Cache: rejectCache})
	rejected, err := rejectReconciler.ReconcileExternalOrders([]model.OrderStatusReport{external}, ExternalOrderPolicy{
		AllowImport: false,
	})
	require.NoError(t, err)
	require.Equal(t, "TC-REC08", rejected.CaseID)
	require.Equal(t, 1, rejected.ReportsScanned)
	require.Equal(t, 0, rejected.OrdersApplied)
	require.Len(t, rejected.Unresolved, 1)
	require.Equal(t, "external_order_rejected", rejected.Unresolved[0].Kind)
	require.Equal(t, external.OrderID, rejected.Unresolved[0].OrderID)
	_, ok = rejectCache.Order(accountID, external.OrderID)
	require.False(t, ok)
}

func TestReconcilerAuditTrailTracksSuccessErrorAndUnresolved(t *testing.T) {
	c := cache.New()
	reconciler := NewReconciler(ReconciliationConfig{Cache: c})
	accountID := model.AccountID("acct")
	instrumentID := executionTestInstrumentID()
	order := executionTestOrderReport("audit-order", "audit-client", model.OrderStatusAccepted)
	order.AccountID = accountID
	order.InstrumentID = instrumentID
	order.Quantity = decimal.RequireFromString("1")
	order.LeavesQuantity = decimal.RequireFromString("1")

	result, err := reconciler.ReconcileMassStatus(model.ExecutionMassStatus{
		AccountID: accountID,
		Venue:     "BINANCE",
		Orders:    []model.OrderStatusReport{order},
		Timestamp: time.Unix(130, 0),
	})
	require.NoError(t, err)
	require.Equal(t, "TC-REC01", result.CaseID)

	venue := order
	venue.Status = model.OrderStatusFilled
	venue.FilledQuantity = decimal.RequireFromString("1")
	venue.LeavesQuantity = decimal.Zero
	_, err = reconciler.DetectOrderDiscrepancies([]model.OrderStatusReport{venue})
	require.NoError(t, err)

	invalidFill := model.FillReport{
		AccountID: accountID,
		TradeID:   "audit-invalid-fill",
	}
	_, err = reconciler.ReconcileMissingFills([]model.FillReport{invalidFill}, time.Time{})
	require.Error(t, err)

	audit := reconciler.AuditTrail()
	require.Len(t, audit.History, 3)
	require.Equal(t, "TC-REC01", audit.History[0].CaseID)
	require.Equal(t, "TC-REC03", audit.LastSuccess.CaseID)
	require.Equal(t, "TC-REC02", audit.LastErrorResult.CaseID)
	require.NotEmpty(t, audit.LastError)
	require.Equal(t, "TC-REC02", audit.LastResult.CaseID)
	require.Len(t, audit.Unresolved, 2)
	require.Equal(t, "order_open_state_mismatch", audit.Unresolved[0].Kind)
	require.Equal(t, "order_filled_quantity_mismatch", audit.Unresolved[1].Kind)
}
