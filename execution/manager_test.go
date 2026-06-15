package execution

import (
	"testing"
	"time"

	"github.com/QuantProcessing/exchanges/cache"
	"github.com/QuantProcessing/exchanges/model"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestManagerCachesAndPopsSubmitCommands(t *testing.T) {
	manager := NewManager(Config{Cache: cache.New()})
	order := executionTestSubmit("client-1", model.OrderSideBuy, decimal.RequireFromString("1"))

	require.NoError(t, manager.CacheSubmitCommand(order))
	got, ok := manager.SubmitCommand("client-1")
	require.True(t, ok)
	require.Equal(t, order, got)

	popped, ok := manager.PopSubmitCommand("client-1")
	require.True(t, ok)
	require.Equal(t, order, popped)
	_, ok = manager.SubmitCommand("client-1")
	require.False(t, ok)
}

func TestManagerRejectsClosedOrderRegression(t *testing.T) {
	c := cache.New()
	manager := NewManager(Config{Cache: c})
	filled := executionTestOrderReport("order-1", "client-1", model.OrderStatusFilled)
	filled.Quantity = decimal.RequireFromString("1")
	filled.FilledQuantity = decimal.RequireFromString("1")
	filled.LeavesQuantity = decimal.Zero
	require.NoError(t, manager.ApplyOrderReport(filled))

	regressed := filled
	regressed.Status = model.OrderStatusAccepted
	regressed.FilledQuantity = decimal.Zero
	regressed.LeavesQuantity = decimal.RequireFromString("1")
	require.ErrorIs(t, manager.ApplyOrderReport(regressed), ErrInvalidTransition)

	cached, ok := c.Order("acct", "order-1")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusFilled, cached.Status)
}

func TestManagerDeduplicatesFillsAndRejectsOverfill(t *testing.T) {
	c := cache.New()
	manager := NewManager(Config{Cache: c})
	order := executionTestOrderReport("order-1", "client-1", model.OrderStatusAccepted)
	order.Quantity = decimal.RequireFromString("1")
	order.LeavesQuantity = decimal.RequireFromString("1")
	require.NoError(t, manager.ApplyOrderReport(order))

	applied, err := manager.ApplyFill(executionTestFill("trade-1", "order-1", "client-1", "0.4", "100"))
	require.NoError(t, err)
	require.True(t, applied)
	applied, err = manager.ApplyFill(executionTestFill("trade-1", "order-1", "client-1", "0.4", "100"))
	require.NoError(t, err)
	require.False(t, applied)

	_, err = manager.ApplyFill(executionTestFill("trade-2", "order-1", "client-1", "0.7", "101"))
	require.ErrorIs(t, err, ErrOverfill)
	cached, ok := c.Order("acct", "order-1")
	require.True(t, ok)
	require.Equal(t, "0.4", cached.FilledQuantity.String())
	require.Equal(t, "0.6", cached.LeavesQuantity.String())
	require.Equal(t, model.OrderStatusPartiallyFilled, cached.Status)
}

func TestManagerDefersFillUntilOrderReportArrives(t *testing.T) {
	c := cache.New()
	manager := NewManager(Config{Cache: c})
	fill := executionTestFill("trade-deferred", "order-deferred", "client-deferred", "0.4", "100")

	applied, err := manager.ApplyFill(fill)
	require.NoError(t, err)
	require.False(t, applied)
	require.Len(t, c.DeferredFillsForOrder("acct", "order-deferred"), 1)
	_, ok := c.FillByTradeID("acct", "trade-deferred")
	require.False(t, ok)

	order := executionTestOrderReport("order-deferred", "client-deferred", model.OrderStatusAccepted)
	order.Quantity = decimal.RequireFromString("1")
	order.LeavesQuantity = decimal.RequireFromString("1")
	require.NoError(t, manager.ApplyOrderReport(order))

	require.Empty(t, c.DeferredFillsForOrder("acct", "order-deferred"))
	_, ok = c.FillByTradeID("acct", "trade-deferred")
	require.True(t, ok)
	cached, ok := c.Order("acct", "order-deferred")
	require.True(t, ok)
	require.Equal(t, model.OrderStatusPartiallyFilled, cached.Status)
	require.True(t, decimal.RequireFromString("0.4").Equal(cached.FilledQuantity))
	require.True(t, decimal.RequireFromString("0.6").Equal(cached.LeavesQuantity))
}

func TestManagerAppliesLegFillWithoutOrderReport(t *testing.T) {
	c := cache.New()
	manager := NewManager(Config{Cache: c})
	fill := executionTestFill("trade-leg-1", "", "spread-LEG-1", "0.25", "101")
	fill.VenueOrderID = "venue-LEG-1"
	fill.PositionID = "leg-position-1"
	fill.IsLeg = true

	applied, err := manager.ApplyFill(fill)
	require.NoError(t, err)
	require.True(t, applied)
	cached, ok := c.FillByTradeID("acct", "trade-leg-1")
	require.True(t, ok)
	require.True(t, cached.IsLegFill())
	require.Equal(t, model.PositionID("leg-position-1"), cached.PositionID)
	require.Empty(t, c.DeferredFillsForOrder("acct", ""))

	applied, err = manager.ApplyFill(fill)
	require.NoError(t, err)
	require.False(t, applied)
}

func TestManagerDeterminesNettingAndHedgingPositionIDs(t *testing.T) {
	instID := executionTestInstrumentID()
	netting := NewManager(Config{PositionIDMode: PositionIDModeNetting})
	require.Equal(t, model.PositionID(instID.String()), netting.DeterminePositionID(model.AccountID("acct"), instID, "strategy-a"))
	require.Equal(t, model.PositionID(instID.String()), netting.DeterminePositionID(model.AccountID("acct"), instID, "strategy-b"))

	hedging := NewManager(Config{PositionIDMode: PositionIDModeHedging})
	first := hedging.DeterminePositionID(model.AccountID("acct"), instID, "strategy-a")
	second := hedging.DeterminePositionID(model.AccountID("acct"), instID, "strategy-a")
	otherStrategy := hedging.DeterminePositionID(model.AccountID("acct"), instID, "strategy-b")
	require.Equal(t, first, second)
	require.NotEqual(t, first, otherStrategy)
	require.Contains(t, string(otherStrategy), "strategy-b")
}

func TestManagerReleasesOTOChildrenAndCancelsOCOSiblings(t *testing.T) {
	c := cache.New()
	manager := NewManager(Config{Cache: c})
	instID := executionTestInstrumentID()
	list := model.OrderList{
		ID: "bracket-list",
		Orders: []model.SubmitOrder{
			executionTestSubmit("entry", model.OrderSideBuy, decimal.RequireFromString("1")),
			executionTestSubmit("stop", model.OrderSideSell, decimal.RequireFromString("1")),
			executionTestSubmit("target", model.OrderSideSell, decimal.RequireFromString("1")),
		},
	}
	list.Orders[0].InstrumentID = instID
	list.Orders[0].OrderListID = list.ID
	list.Orders[0].Contingency = model.ContingencyTypeOTO
	list.Orders[1].InstrumentID = instID
	list.Orders[1].OrderListID = list.ID
	list.Orders[1].ParentClientOrderID = "entry"
	list.Orders[1].Contingency = model.ContingencyTypeOCO
	list.Orders[2].InstrumentID = instID
	list.Orders[2].OrderListID = list.ID
	list.Orders[2].ParentClientOrderID = "entry"
	list.Orders[2].Contingency = model.ContingencyTypeOCO
	require.NoError(t, manager.IndexOrderList(list))

	entryFilled := executionTestOrderReport("order-entry", "entry", model.OrderStatusFilled)
	entryFilled.OrderListID = list.ID
	entryFilled.Contingency = model.ContingencyTypeOTO
	entryFilled.FilledQuantity = decimal.RequireFromString("1")
	entryFilled.LeavesQuantity = decimal.Zero
	actions, err := manager.HandleOrderListProgress(entryFilled)
	require.NoError(t, err)
	require.Len(t, actions.Submit, 2)
	require.Empty(t, actions.Cancel)
	require.Equal(t, model.ClientOrderID("stop"), actions.Submit[0].ClientOrderID)
	require.Equal(t, model.ClientOrderID("target"), actions.Submit[1].ClientOrderID)

	stop := executionTestOrderReport("order-stop", "stop", model.OrderStatusAccepted)
	stop.OrderListID = list.ID
	stop.Contingency = model.ContingencyTypeOCO
	stop.Quantity = decimal.RequireFromString("1")
	stop.LeavesQuantity = decimal.RequireFromString("1")
	target := executionTestOrderReport("order-target", "target", model.OrderStatusAccepted)
	target.OrderListID = list.ID
	target.Contingency = model.ContingencyTypeOCO
	target.Quantity = decimal.RequireFromString("1")
	target.LeavesQuantity = decimal.RequireFromString("1")
	require.NoError(t, manager.ApplyOrderReport(stop))
	require.NoError(t, manager.ApplyOrderReport(target))

	stopFilled := stop
	stopFilled.Status = model.OrderStatusFilled
	stopFilled.FilledQuantity = decimal.RequireFromString("1")
	stopFilled.LeavesQuantity = decimal.Zero
	actions, err = manager.HandleOrderListProgress(stopFilled)
	require.NoError(t, err)
	require.Empty(t, actions.Submit)
	require.Len(t, actions.Cancel, 1)
	require.Equal(t, model.ClientOrderID("target"), actions.Cancel[0].ClientOrderID)
}

func TestManagerReducesOUOSiblingsOnPartialFill(t *testing.T) {
	c := cache.New()
	manager := NewManager(Config{Cache: c})
	list := model.OrderList{
		ID: "ouo-list",
		Orders: []model.SubmitOrder{
			executionTestSubmit("ouo-a", model.OrderSideSell, decimal.RequireFromString("1")),
			executionTestSubmit("ouo-b", model.OrderSideSell, decimal.RequireFromString("1")),
			executionTestSubmit("ouo-c", model.OrderSideSell, decimal.RequireFromString("1")),
		},
	}
	for i := range list.Orders {
		list.Orders[i].OrderListID = list.ID
		list.Orders[i].Contingency = model.ContingencyTypeOUO
	}
	require.NoError(t, manager.IndexOrderList(list))

	a := executionTestOrderReport("order-ouo-a", "ouo-a", model.OrderStatusAccepted)
	a.OrderListID = list.ID
	a.Contingency = model.ContingencyTypeOUO
	a.Side = model.OrderSideSell
	b := executionTestOrderReport("order-ouo-b", "ouo-b", model.OrderStatusPartiallyFilled)
	b.OrderListID = list.ID
	b.Contingency = model.ContingencyTypeOUO
	b.Side = model.OrderSideSell
	b.FilledQuantity = decimal.RequireFromString("0.2")
	b.LeavesQuantity = decimal.RequireFromString("0.8")
	cSibling := executionTestOrderReport("order-ouo-c", "ouo-c", model.OrderStatusPartiallyFilled)
	cSibling.OrderListID = list.ID
	cSibling.Contingency = model.ContingencyTypeOUO
	cSibling.Side = model.OrderSideSell
	cSibling.FilledQuantity = decimal.RequireFromString("0.8")
	cSibling.LeavesQuantity = decimal.RequireFromString("0.2")
	require.NoError(t, manager.ApplyOrderReport(a))
	require.NoError(t, manager.ApplyOrderReport(b))
	require.NoError(t, manager.ApplyOrderReport(cSibling))

	a.Status = model.OrderStatusPartiallyFilled
	a.FilledQuantity = decimal.RequireFromString("0.4")
	a.LeavesQuantity = decimal.RequireFromString("0.6")
	require.NoError(t, manager.ApplyOrderReport(a))
	actions, err := manager.HandleOrderListProgress(a)
	require.NoError(t, err)
	require.Empty(t, actions.Submit)
	require.Len(t, actions.Modify, 1)
	require.Equal(t, model.ClientOrderID("ouo-b"), actions.Modify[0].ClientOrderID)
	require.True(t, decimal.RequireFromString("0.6").Equal(actions.Modify[0].Quantity))
	require.Len(t, actions.Cancel, 1)
	require.Equal(t, model.ClientOrderID("ouo-c"), actions.Cancel[0].ClientOrderID)

	actions, err = manager.HandleOrderListProgress(a)
	require.NoError(t, err)
	require.Empty(t, actions.Modify)
	require.Empty(t, actions.Cancel)
}

func TestManagerSnapshotsOrderListState(t *testing.T) {
	c := cache.New()
	manager := NewManager(Config{Cache: c})
	list := model.OrderList{
		ID: "snapshot-list",
		Orders: []model.SubmitOrder{
			executionTestSubmit("entry", model.OrderSideBuy, decimal.RequireFromString("1")),
			executionTestSubmit("stop", model.OrderSideSell, decimal.RequireFromString("1")),
			executionTestSubmit("target", model.OrderSideSell, decimal.RequireFromString("1")),
		},
	}
	list.Orders[0].OrderListID = list.ID
	list.Orders[0].Contingency = model.ContingencyTypeOTO
	for i := 1; i < len(list.Orders); i++ {
		list.Orders[i].OrderListID = list.ID
		list.Orders[i].ParentClientOrderID = "entry"
		list.Orders[i].Contingency = model.ContingencyTypeOUO
		list.Orders[i].ReduceOnly = true
	}
	require.NoError(t, manager.IndexOrderList(list))

	snapshot, ok := manager.OrderListSnapshot("acct", list.ID)
	require.True(t, ok)
	require.Equal(t, model.AccountID("acct"), snapshot.AccountID)
	require.Equal(t, list.ID, snapshot.OrderListID)
	require.Equal(t, model.OrderListKindBracket, snapshot.Kind)
	require.Equal(t, OrderListStatusOpen, snapshot.Status)
	require.Equal(t, 3, snapshot.MemberCount)
	require.Equal(t, 0, snapshot.OpenCount)
	require.Equal(t, 0, snapshot.TerminalCount)
	require.Equal(t, 2, snapshot.HeldCount)
	require.Equal(t, []model.ClientOrderID{"entry", "stop", "target"}, snapshot.Members)
	require.Len(t, snapshot.HeldChildren, 1)
	require.Equal(t, model.ClientOrderID("entry"), snapshot.HeldChildren[0].ParentClientOrderID)
	require.Equal(t, []model.ClientOrderID{"stop", "target"}, submitClientOrderIDs(snapshot.HeldChildren[0].Orders))
	require.Empty(t, snapshot.Orders)
	require.Empty(t, snapshot.FillProgress)

	entryFilled := executionTestOrderReport("order-entry", "entry", model.OrderStatusFilled)
	entryFilled.OrderListID = list.ID
	entryFilled.Contingency = model.ContingencyTypeOTO
	entryFilled.FilledQuantity = decimal.RequireFromString("1")
	entryFilled.LeavesQuantity = decimal.Zero
	require.NoError(t, manager.ApplyOrderReport(entryFilled))
	_, err := manager.HandleOrderListProgress(entryFilled)
	require.NoError(t, err)

	stop := executionTestOrderReport("order-stop", "stop", model.OrderStatusAccepted)
	stop.OrderListID = list.ID
	stop.Contingency = model.ContingencyTypeOUO
	stop.Side = model.OrderSideSell
	target := executionTestOrderReport("order-target", "target", model.OrderStatusAccepted)
	target.OrderListID = list.ID
	target.Contingency = model.ContingencyTypeOUO
	target.Side = model.OrderSideSell
	require.NoError(t, manager.ApplyOrderReport(stop))
	require.NoError(t, manager.ApplyOrderReport(target))

	stop.Status = model.OrderStatusPartiallyFilled
	stop.FilledQuantity = decimal.RequireFromString("0.4")
	stop.LeavesQuantity = decimal.RequireFromString("0.6")
	require.NoError(t, manager.ApplyOrderReport(stop))
	_, err = manager.HandleOrderListProgress(stop)
	require.NoError(t, err)

	snapshot, ok = manager.OrderListSnapshot("acct", list.ID)
	require.True(t, ok)
	require.Equal(t, OrderListStatusOpen, snapshot.Status)
	require.Equal(t, 3, snapshot.MemberCount)
	require.Equal(t, 2, snapshot.OpenCount)
	require.Equal(t, 1, snapshot.TerminalCount)
	require.Equal(t, 0, snapshot.HeldCount)
	require.Empty(t, snapshot.HeldChildren)
	require.Len(t, snapshot.Orders, 3)
	require.Len(t, snapshot.FillProgress, 1)
	require.Equal(t, model.OrderID("order-stop"), snapshot.FillProgress[0].OrderID)
	require.True(t, decimal.RequireFromString("0.4").Equal(snapshot.FillProgress[0].FilledQuantity))

	snapshots := manager.OrderListSnapshots("acct")
	require.Len(t, snapshots, 1)
	require.Equal(t, list.ID, snapshots[0].OrderListID)
	_, ok = manager.OrderListSnapshot("acct", "missing-list")
	require.False(t, ok)
}

func TestManagerClosesOrderListAndClearsTransientState(t *testing.T) {
	c := cache.New()
	manager := NewManager(Config{Cache: c})
	list := model.OrderList{
		ID: "close-list",
		Orders: []model.SubmitOrder{
			executionTestSubmit("entry", model.OrderSideBuy, decimal.RequireFromString("1")),
			executionTestSubmit("stop", model.OrderSideSell, decimal.RequireFromString("1")),
			executionTestSubmit("target", model.OrderSideSell, decimal.RequireFromString("1")),
		},
	}
	list.Orders[0].OrderListID = list.ID
	list.Orders[0].Contingency = model.ContingencyTypeOTO
	for i := 1; i < len(list.Orders); i++ {
		list.Orders[i].OrderListID = list.ID
		list.Orders[i].ParentClientOrderID = "entry"
		list.Orders[i].Contingency = model.ContingencyTypeOCO
		list.Orders[i].ReduceOnly = true
	}
	require.NoError(t, manager.IndexOrderList(list))

	entryFilled := executionTestOrderReport("order-entry", "entry", model.OrderStatusFilled)
	entryFilled.OrderListID = list.ID
	entryFilled.Contingency = model.ContingencyTypeOTO
	entryFilled.FilledQuantity = decimal.RequireFromString("1")
	entryFilled.LeavesQuantity = decimal.Zero
	require.NoError(t, manager.ApplyOrderReport(entryFilled))
	actions, err := manager.HandleOrderListProgress(entryFilled)
	require.NoError(t, err)
	require.Len(t, actions.Submit, 2)

	stop := executionTestOrderReport("order-stop", "stop", model.OrderStatusAccepted)
	stop.OrderListID = list.ID
	stop.Contingency = model.ContingencyTypeOCO
	target := executionTestOrderReport("order-target", "target", model.OrderStatusAccepted)
	target.OrderListID = list.ID
	target.Contingency = model.ContingencyTypeOCO
	require.NoError(t, manager.ApplyOrderReport(stop))
	require.NoError(t, manager.ApplyOrderReport(target))

	stop.Status = model.OrderStatusFilled
	stop.FilledQuantity = decimal.RequireFromString("1")
	stop.LeavesQuantity = decimal.Zero
	require.NoError(t, manager.ApplyOrderReport(stop))
	actions, err = manager.HandleOrderListProgress(stop)
	require.NoError(t, err)
	require.Len(t, actions.Cancel, 1)

	target.Status = model.OrderStatusCanceled
	target.LeavesQuantity = decimal.Zero
	require.NoError(t, manager.ApplyOrderReport(target))
	actions, err = manager.HandleOrderListProgress(target)
	require.NoError(t, err)
	require.Empty(t, actions.Submit)
	require.Empty(t, actions.Modify)
	require.Empty(t, actions.Cancel)

	snapshot, ok := manager.OrderListSnapshot("acct", list.ID)
	require.True(t, ok)
	require.Equal(t, model.OrderListKindBracket, snapshot.Kind)
	require.Equal(t, OrderListStatusClosed, snapshot.Status)
	require.Equal(t, 3, snapshot.MemberCount)
	require.Equal(t, 0, snapshot.OpenCount)
	require.Equal(t, 3, snapshot.TerminalCount)
	require.Equal(t, 0, snapshot.HeldCount)
	require.Empty(t, snapshot.HeldChildren)
	require.Empty(t, snapshot.FillProgress)
}

func TestManagerCancelsHeldChildrenWhenOTOParentEndsWithoutFill(t *testing.T) {
	manager := NewManager(Config{Cache: cache.New()})
	list := model.OrderList{
		ID: "parent-canceled-list",
		Orders: []model.SubmitOrder{
			executionTestSubmit("entry", model.OrderSideBuy, decimal.RequireFromString("1")),
			executionTestSubmit("stop", model.OrderSideSell, decimal.RequireFromString("1")),
			executionTestSubmit("target", model.OrderSideSell, decimal.RequireFromString("1")),
		},
	}
	list.Orders[0].OrderListID = list.ID
	list.Orders[0].Contingency = model.ContingencyTypeOTO
	for i := 1; i < len(list.Orders); i++ {
		list.Orders[i].OrderListID = list.ID
		list.Orders[i].ParentClientOrderID = "entry"
		list.Orders[i].Contingency = model.ContingencyTypeOCO
		list.Orders[i].ReduceOnly = true
	}
	require.NoError(t, manager.IndexOrderList(list))

	entryCanceled := executionTestOrderReport("order-entry", "entry", model.OrderStatusCanceled)
	entryCanceled.OrderListID = list.ID
	entryCanceled.Contingency = model.ContingencyTypeOTO
	entryCanceled.LeavesQuantity = decimal.Zero
	require.NoError(t, manager.ApplyOrderReport(entryCanceled))
	actions, err := manager.HandleOrderListProgress(entryCanceled)
	require.NoError(t, err)
	require.Empty(t, actions.Submit)
	require.Empty(t, actions.Modify)
	require.Empty(t, actions.Cancel)

	snapshot, ok := manager.OrderListSnapshot("acct", list.ID)
	require.True(t, ok)
	require.Equal(t, OrderListStatusClosed, snapshot.Status)
	require.Equal(t, 0, snapshot.OpenCount)
	require.Equal(t, 1, snapshot.TerminalCount)
	require.Equal(t, 0, snapshot.HeldCount)
	require.Empty(t, snapshot.HeldChildren)
}

func executionTestSubmit(clientOrderID model.ClientOrderID, side model.OrderSide, quantity decimal.Decimal) model.SubmitOrder {
	return model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  executionTestInstrumentID(),
		ClientOrderID: clientOrderID,
		Side:          side,
		Type:          model.OrderTypeLimit,
		TimeInForce:   model.TimeInForceGTC,
		Quantity:      quantity,
		Price:         decimal.RequireFromString("100"),
	}
}

func executionTestOrderReport(orderID model.OrderID, clientOrderID model.ClientOrderID, status model.OrderStatus) model.OrderStatusReport {
	return model.OrderStatusReport{
		AccountID:       "acct",
		InstrumentID:    executionTestInstrumentID(),
		OrderID:         orderID,
		ClientOrderID:   clientOrderID,
		Side:            model.OrderSideBuy,
		Type:            model.OrderTypeLimit,
		TimeInForce:     model.TimeInForceGTC,
		Status:          status,
		Quantity:        decimal.RequireFromString("1"),
		FilledQuantity:  decimal.Zero,
		LeavesQuantity:  decimal.RequireFromString("1"),
		Price:           decimal.RequireFromString("100"),
		AveragePrice:    decimal.Zero,
		PostOnly:        true,
		ReduceOnly:      false,
		LastUpdatedTime: time.Unix(1, 0),
		ExpireTime:      time.Time{},
		TriggerPrice:    decimal.Zero,
		ActivationPrice: decimal.Zero,
		TrailingOffset:  decimal.Zero,
	}
}

func executionTestFill(tradeID model.TradeID, orderID model.OrderID, clientOrderID model.ClientOrderID, quantity string, price string) model.FillReport {
	return model.FillReport{
		AccountID:     "acct",
		InstrumentID:  executionTestInstrumentID(),
		OrderID:       orderID,
		ClientOrderID: clientOrderID,
		TradeID:       tradeID,
		Side:          model.OrderSideBuy,
		Price:         decimal.RequireFromString(price),
		Quantity:      decimal.RequireFromString(quantity),
		Timestamp:     time.Unix(2, 0),
	}
}

func executionTestInstrumentID() model.InstrumentID {
	return model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
}

func submitClientOrderIDs(orders []model.SubmitOrder) []model.ClientOrderID {
	ids := make([]model.ClientOrderID, 0, len(orders))
	for _, order := range orders {
		ids = append(ids, order.ClientOrderID)
	}
	return ids
}
