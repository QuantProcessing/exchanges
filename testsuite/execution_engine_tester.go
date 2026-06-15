package testsuite

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/QuantProcessing/exchanges/backtest"
	"github.com/QuantProcessing/exchanges/bus"
	"github.com/QuantProcessing/exchanges/cache"
	"github.com/QuantProcessing/exchanges/execution"
	"github.com/QuantProcessing/exchanges/kernel"
	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/platform"
	"github.com/QuantProcessing/exchanges/strategy"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/shopspring/decimal"
)

type ExecutionEngineTesterConfig struct{}

type ExecutionEngineTester struct {
	cfg ExecutionEngineTesterConfig
}

func NewExecutionEngineTester(cfg ExecutionEngineTesterConfig) *ExecutionEngineTester {
	return &ExecutionEngineTester{cfg: cfg}
}

func (e *ExecutionEngineTester) Run(ctx context.Context, t *testing.T) ContractReport {
	t.Helper()
	return runContractCases(t, "execution-engine", []contractCase{
		{id: "TC-EXENG01", name: "Engine routes submit commands and caches reports", run: func() error {
			return e.runEngineSubmit(ctx)
		}},
		{id: "TC-EXENG02", name: "Engine routes cancel modify and query", run: func() error {
			return e.runEngineCancelModifyQuery(ctx)
		}},
		{id: "TC-EXENG03", name: "Engine rejects missing account clients", run: func() error {
			_, err := execution.NewEngine(execution.EngineConfig{}).SubmitOrder(ctx, executionEngineSubmit("client-1", model.OrderSideBuy, decimal.RequireFromString("1")))
			if !errors.Is(err, execution.ErrClientNotFound) {
				return fmt.Errorf("expected ErrClientNotFound, got %v", err)
			}
			return nil
		}},
		{id: "TC-EXENG04", name: "Manager caches and pops submit commands", run: func() error {
			return e.runManagerSubmitCache()
		}},
		{id: "TC-EXENG05", name: "Manager rejects closed-order regressions", run: func() error {
			return e.runManagerTransitionGuard()
		}},
		{id: "TC-EXENG06", name: "Manager deduplicates fills and rejects overfills", run: func() error {
			return e.runManagerFillGuard()
		}},
		{id: "TC-EXENG14", name: "Manager defers fills until order reports arrive", run: func() error {
			return e.runManagerFillDeferral()
		}},
		{id: "TC-EXENG07", name: "Manager determines netting and hedging position IDs", run: func() error {
			return e.runManagerPositionIDs()
		}},
		{id: "TC-EXENG08", name: "Manager releases OTO children and cancels OCO siblings", run: func() error {
			return e.runManagerOrderListActions()
		}},
		{id: "TC-EXENG13", name: "Manager reduces OUO siblings on partial fills", run: func() error {
			return e.runManagerOuoActions()
		}},
		{id: "TC-EXENG17", name: "Manager snapshots durable order-list state", run: func() error {
			return e.runManagerOrderListSnapshot()
		}},
		{id: "TC-EXENG18", name: "Manager applies leg fills without parent order reports", run: func() error {
			return e.runManagerLegFillWithoutOrder()
		}},
		{id: "TC-EXENG19", name: "Engine routes execution-algorithm orders before venue submission", run: func() error {
			return e.runEngineExecAlgorithmRouting(ctx)
		}},
		{id: "TC-EXENG20", name: "Engine emulates trigger orders until market data releases them", run: func() error {
			return e.runEngineTriggerEmulation(ctx)
		}},
		{id: "TC-EXENG21", name: "Engine publishes emulated triggered and released lifecycle events", run: func() error {
			return e.runEngineEmulationLifecycleEvents(ctx)
		}},
		{id: "TC-EXENG22", name: "Platform feeds data-engine market events into execution emulator", run: func() error {
			return e.runPlatformDataEngineEmulation(ctx)
		}},
		{id: "TC-EXENG23", name: "Engine emulates trailing stop market trigger updates", run: func() error {
			return e.runEngineTrailingStopEmulation(ctx)
		}},
		{id: "TC-EXENG24", name: "Engine transforms released emulated orders before venue submission", run: func() error {
			return e.runEngineReleaseOrderTransform(ctx)
		}},
		{id: "TC-EXENG25", name: "Engine emulates trailing stop offset types", run: func() error {
			return e.runEngineTrailingOffsetTypes(ctx)
		}},
		{id: "TC-EXENG26", name: "Engine emulates trailing stop limit releases", run: func() error {
			return e.runEngineTrailingStopLimitEmulation(ctx)
		}},
		{id: "TC-EXENG27", name: "Engine emulates orders from trigger instruments", run: func() error {
			return e.runEngineTriggerInstrumentEmulation(ctx)
		}},
		{id: "TC-EXENG28", name: "Engine emulates bid ask triggers from order books", run: func() error {
			return e.runEngineOrderBookTriggerEmulation(ctx)
		}},
		{id: "TC-EXENG29", name: "Engine uses synthetic trigger instruments for emulation", run: func() error {
			return e.runEngineSyntheticTriggerInstrumentEmulation(ctx)
		}},
		{id: "TC-EXENG30", name: "Engine initial matches emulated orders from cached market data", run: func() error {
			return e.runEngineInitialEmulationMatch(ctx)
		}},
		{id: "TC-EXENG31", name: "Engine cancels emulated orders locally", run: func() error {
			return e.runEngineLocalEmulatedCancel(ctx)
		}},
		{id: "TC-EXENG32", name: "Engine modifies emulated orders locally and rematches", run: func() error {
			return e.runEngineLocalEmulatedModify(ctx)
		}},
		{id: "TC-EXENG33", name: "Engine cancel-all cancels emulated orders locally", run: func() error {
			return e.runEngineLocalEmulatedCancelAll(ctx)
		}},
		{id: "TC-EXENG34", name: "Engine matching core releases emulated limit orders only when marketable", run: func() error {
			return e.runEngineEmulatedLimitMatching(ctx)
		}},
		{id: "TC-EXENG35", name: "Engine health tracks kernel lifecycle state", run: func() error {
			return e.runEngineKernelLifecycle(ctx)
		}},
		{id: "TC-EXENG09", name: "Platform routes order commands through execution engine", run: func() error {
			return e.runPlatformExecutionEngineDelegation(ctx)
		}},
		{id: "TC-EXENG10", name: "Backtest routes order commands through execution engine", run: func() error {
			return e.runBacktestExecutionEngineDelegation(ctx)
		}},
		{id: "TC-EXENG11", name: "Engine routes composite commands and account queries", run: func() error {
			return e.runEngineCompositeCommands(ctx)
		}},
		{id: "TC-EXENG12", name: "Engine generates execution reports and mass status", run: func() error {
			return e.runEngineReportGeneration(ctx)
		}},
		{id: "TC-EXENG15", name: "Engine claims external order reports by instrument", run: func() error {
			return e.runEngineExternalOrderClaim(ctx)
		}},
		{id: "TC-EXENG16", name: "Engine snapshots and purges execution state", run: func() error {
			return e.runEngineSnapshotPurge()
		}},
	})
}

func (e *ExecutionEngineTester) runEngineSubmit(ctx context.Context) error {
	client := newExecutionEngineFakeClient("acct")
	c := cache.New()
	engine := execution.NewEngine(execution.EngineConfig{Cache: c})
	if err := engine.AddClient(client); err != nil {
		return err
	}
	if err := engine.Start(ctx); err != nil {
		return err
	}
	defer engine.Stop(ctx)
	order := executionEngineSubmit("client-1", model.OrderSideBuy, decimal.RequireFromString("1"))
	report, err := engine.SubmitOrder(ctx, order)
	if err != nil {
		return err
	}
	if report.Status != model.OrderStatusAccepted {
		return fmt.Errorf("expected accepted report, got %s", report.Status)
	}
	if _, ok := c.OrderByClientID("acct", "client-1"); !ok {
		return fmt.Errorf("submitted order was not cached")
	}
	return nil
}

func (e *ExecutionEngineTester) runEngineCancelModifyQuery(ctx context.Context) error {
	client := newExecutionEngineFakeClient("acct")
	engine := execution.NewEngine(execution.EngineConfig{})
	if err := engine.AddClient(client); err != nil {
		return err
	}
	if err := engine.Start(ctx); err != nil {
		return err
	}
	defer engine.Stop(ctx)
	order := executionEngineSubmit("client-1", model.OrderSideBuy, decimal.RequireFromString("1"))
	report, err := engine.SubmitOrder(ctx, order)
	if err != nil {
		return err
	}
	if _, err := engine.ModifyOrder(ctx, model.ModifyOrder{
		AccountID:     "acct",
		InstrumentID:  order.InstrumentID,
		ClientOrderID: order.ClientOrderID,
		OrderID:       report.OrderID,
		Quantity:      decimal.RequireFromString("1"),
	}); err != nil {
		return err
	}
	if _, err := engine.QueryOrder(ctx, model.QueryOrder{
		AccountID:     "acct",
		InstrumentID:  order.InstrumentID,
		ClientOrderID: order.ClientOrderID,
		OrderID:       report.OrderID,
	}); err != nil {
		return err
	}
	if _, err := engine.CancelOrder(ctx, model.CancelOrder{
		AccountID:     "acct",
		InstrumentID:  order.InstrumentID,
		ClientOrderID: order.ClientOrderID,
		OrderID:       report.OrderID,
	}); err != nil {
		return err
	}
	if !client.HasCall("modify:client-1") || !client.HasCall("query:client-1") || !client.HasCall("cancel:client-1") {
		return fmt.Errorf("expected modify/query/cancel calls, got %v", client.Calls())
	}
	return nil
}

func (e *ExecutionEngineTester) runManagerSubmitCache() error {
	manager := execution.NewManager(execution.Config{Cache: cache.New()})
	order := executionEngineSubmit("client-1", model.OrderSideBuy, decimal.RequireFromString("1"))
	if err := manager.CacheSubmitCommand(order); err != nil {
		return err
	}
	if _, ok := manager.SubmitCommand("client-1"); !ok {
		return fmt.Errorf("submit command was not cached")
	}
	if _, ok := manager.PopSubmitCommand("client-1"); !ok {
		return fmt.Errorf("submit command was not popped")
	}
	if _, ok := manager.SubmitCommand("client-1"); ok {
		return fmt.Errorf("submit command remained cached after pop")
	}
	return nil
}

func (e *ExecutionEngineTester) runManagerTransitionGuard() error {
	manager := execution.NewManager(execution.Config{Cache: cache.New()})
	filled := executionEngineOrderReport("order-1", "client-1", model.OrderStatusFilled)
	filled.FilledQuantity = decimal.RequireFromString("1")
	filled.LeavesQuantity = decimal.Zero
	if err := manager.ApplyOrderReport(filled); err != nil {
		return err
	}
	regressed := filled
	regressed.Status = model.OrderStatusAccepted
	regressed.FilledQuantity = decimal.Zero
	regressed.LeavesQuantity = decimal.RequireFromString("1")
	if !errors.Is(manager.ApplyOrderReport(regressed), execution.ErrInvalidTransition) {
		return fmt.Errorf("expected invalid transition")
	}
	return nil
}

func (e *ExecutionEngineTester) runManagerFillGuard() error {
	manager := execution.NewManager(execution.Config{Cache: cache.New()})
	order := executionEngineOrderReport("order-1", "client-1", model.OrderStatusAccepted)
	if err := manager.ApplyOrderReport(order); err != nil {
		return err
	}
	applied, err := manager.ApplyFill(executionEngineFill("trade-1", "order-1", "client-1", "0.4", "100"))
	if err != nil || !applied {
		return fmt.Errorf("expected first fill to apply: applied=%v err=%v", applied, err)
	}
	applied, err = manager.ApplyFill(executionEngineFill("trade-1", "order-1", "client-1", "0.4", "100"))
	if err != nil || applied {
		return fmt.Errorf("expected duplicate fill to be ignored: applied=%v err=%v", applied, err)
	}
	if _, err := manager.ApplyFill(executionEngineFill("trade-2", "order-1", "client-1", "0.7", "101")); !errors.Is(err, execution.ErrOverfill) {
		return fmt.Errorf("expected overfill, got %v", err)
	}
	return nil
}

func (e *ExecutionEngineTester) runManagerFillDeferral() error {
	c := cache.New()
	manager := execution.NewManager(execution.Config{Cache: c})
	fill := executionEngineFill("trade-deferred", "order-deferred", "client-deferred", "0.4", "100")
	applied, err := manager.ApplyFill(fill)
	if err != nil {
		return err
	}
	if applied {
		return fmt.Errorf("fill applied before order report arrived")
	}
	if len(c.DeferredFillsForOrder("acct", "order-deferred")) != 1 {
		return fmt.Errorf("expected deferred fill")
	}
	order := executionEngineOrderReport("order-deferred", "client-deferred", model.OrderStatusAccepted)
	if err := manager.ApplyOrderReport(order); err != nil {
		return err
	}
	if len(c.DeferredFillsForOrder("acct", "order-deferred")) != 0 {
		return fmt.Errorf("deferred fill was not cleared")
	}
	cached, ok := c.Order("acct", "order-deferred")
	if !ok || cached.Status != model.OrderStatusPartiallyFilled || !cached.FilledQuantity.Equal(decimal.RequireFromString("0.4")) {
		return fmt.Errorf("deferred fill was not replayed into order: %+v", cached)
	}
	return nil
}

func (e *ExecutionEngineTester) runManagerPositionIDs() error {
	instID := executionEngineInstrumentID()
	netting := execution.NewManager(execution.Config{PositionIDMode: execution.PositionIDModeNetting})
	if netting.DeterminePositionID("acct", instID, "strategy-a") != model.PositionID(instID.String()) {
		return fmt.Errorf("netting position id mismatch")
	}
	hedging := execution.NewManager(execution.Config{PositionIDMode: execution.PositionIDModeHedging})
	first := hedging.DeterminePositionID("acct", instID, "strategy-a")
	second := hedging.DeterminePositionID("acct", instID, "strategy-b")
	if first == second || first == "" || second == "" {
		return fmt.Errorf("hedging position ids were not distinct")
	}
	return nil
}

func (e *ExecutionEngineTester) runManagerOrderListActions() error {
	manager := execution.NewManager(execution.Config{Cache: cache.New()})
	list := executionEngineBracketList()
	if err := manager.IndexOrderList(list); err != nil {
		return err
	}
	entryFilled := executionEngineOrderReport("order-entry", "entry", model.OrderStatusFilled)
	entryFilled.OrderListID = list.ID
	entryFilled.Contingency = model.ContingencyTypeOTO
	entryFilled.FilledQuantity = decimal.RequireFromString("1")
	entryFilled.LeavesQuantity = decimal.Zero
	actions, err := manager.HandleOrderListProgress(entryFilled)
	if err != nil {
		return err
	}
	if len(actions.Submit) != 2 || len(actions.Cancel) != 0 {
		return fmt.Errorf("expected two released children, got %+v", actions)
	}
	stop := executionEngineOrderReport("order-stop", "stop", model.OrderStatusAccepted)
	stop.OrderListID = list.ID
	stop.Contingency = model.ContingencyTypeOCO
	target := executionEngineOrderReport("order-target", "target", model.OrderStatusAccepted)
	target.OrderListID = list.ID
	target.Contingency = model.ContingencyTypeOCO
	if err := manager.ApplyOrderReport(stop); err != nil {
		return err
	}
	if err := manager.ApplyOrderReport(target); err != nil {
		return err
	}
	stop.Status = model.OrderStatusFilled
	stop.FilledQuantity = decimal.RequireFromString("1")
	stop.LeavesQuantity = decimal.Zero
	actions, err = manager.HandleOrderListProgress(stop)
	if err != nil {
		return err
	}
	if len(actions.Cancel) != 1 || actions.Cancel[0].ClientOrderID != "target" {
		return fmt.Errorf("expected target cancel action, got %+v", actions)
	}
	return nil
}

func (e *ExecutionEngineTester) runManagerOuoActions() error {
	c := cache.New()
	manager := execution.NewManager(execution.Config{Cache: c})
	list := executionEngineBulkList("ouo-list", "ouo-a", "ouo-b", "ouo-c")
	for i := range list.Orders {
		list.Orders[i].Side = model.OrderSideSell
		list.Orders[i].Contingency = model.ContingencyTypeOUO
	}
	if err := manager.IndexOrderList(list); err != nil {
		return err
	}
	a := executionEngineOrderReport("order-ouo-a", "ouo-a", model.OrderStatusAccepted)
	a.OrderListID = list.ID
	a.Contingency = model.ContingencyTypeOUO
	a.Side = model.OrderSideSell
	b := executionEngineOrderReport("order-ouo-b", "ouo-b", model.OrderStatusPartiallyFilled)
	b.OrderListID = list.ID
	b.Contingency = model.ContingencyTypeOUO
	b.Side = model.OrderSideSell
	b.FilledQuantity = decimal.RequireFromString("0.2")
	b.LeavesQuantity = decimal.RequireFromString("0.8")
	cSibling := executionEngineOrderReport("order-ouo-c", "ouo-c", model.OrderStatusPartiallyFilled)
	cSibling.OrderListID = list.ID
	cSibling.Contingency = model.ContingencyTypeOUO
	cSibling.Side = model.OrderSideSell
	cSibling.FilledQuantity = decimal.RequireFromString("0.8")
	cSibling.LeavesQuantity = decimal.RequireFromString("0.2")
	for _, report := range []model.OrderStatusReport{a, b, cSibling} {
		if err := manager.ApplyOrderReport(report); err != nil {
			return err
		}
	}
	a.Status = model.OrderStatusPartiallyFilled
	a.FilledQuantity = decimal.RequireFromString("0.4")
	a.LeavesQuantity = decimal.RequireFromString("0.6")
	if err := manager.ApplyOrderReport(a); err != nil {
		return err
	}
	actions, err := manager.HandleOrderListProgress(a)
	if err != nil {
		return err
	}
	if len(actions.Modify) != 1 || actions.Modify[0].ClientOrderID != "ouo-b" || !actions.Modify[0].Quantity.Equal(decimal.RequireFromString("0.6")) {
		return fmt.Errorf("expected OUO sibling resize, got %+v", actions.Modify)
	}
	if len(actions.Cancel) != 1 || actions.Cancel[0].ClientOrderID != "ouo-c" {
		return fmt.Errorf("expected OUO over-reduced sibling cancel, got %+v", actions.Cancel)
	}
	actions, err = manager.HandleOrderListProgress(a)
	if err != nil {
		return err
	}
	if len(actions.Modify) != 0 || len(actions.Cancel) != 0 {
		return fmt.Errorf("expected duplicate OUO progress to be ignored, got %+v", actions)
	}
	return nil
}

func (e *ExecutionEngineTester) runManagerOrderListSnapshot() error {
	manager := execution.NewManager(execution.Config{Cache: cache.New()})
	list := executionEngineBracketList()
	for i := 1; i < len(list.Orders); i++ {
		list.Orders[i].Contingency = model.ContingencyTypeOUO
		list.Orders[i].ReduceOnly = true
	}
	if err := manager.IndexOrderList(list); err != nil {
		return err
	}
	snapshot, ok := manager.OrderListSnapshot("acct", list.ID)
	if !ok {
		return fmt.Errorf("order-list snapshot missing")
	}
	if len(snapshot.Members) != 3 || snapshot.Members[0] != "entry" || snapshot.Members[1] != "stop" || snapshot.Members[2] != "target" {
		return fmt.Errorf("unexpected order-list members: %+v", snapshot.Members)
	}
	if snapshot.Kind != model.OrderListKindBracket ||
		snapshot.Status != execution.OrderListStatusOpen ||
		snapshot.MemberCount != 3 ||
		snapshot.OpenCount != 0 ||
		snapshot.TerminalCount != 0 ||
		snapshot.HeldCount != 2 {
		return fmt.Errorf("unexpected initial order-list lifecycle snapshot: %+v", snapshot)
	}
	if len(snapshot.HeldChildren) != 1 || snapshot.HeldChildren[0].ParentClientOrderID != "entry" || len(snapshot.HeldChildren[0].Orders) != 2 {
		return fmt.Errorf("unexpected held children snapshot: %+v", snapshot.HeldChildren)
	}

	entryFilled := executionEngineOrderReport("order-entry", "entry", model.OrderStatusFilled)
	entryFilled.OrderListID = list.ID
	entryFilled.Contingency = model.ContingencyTypeOTO
	entryFilled.FilledQuantity = decimal.RequireFromString("1")
	entryFilled.LeavesQuantity = decimal.Zero
	if err := manager.ApplyOrderReport(entryFilled); err != nil {
		return err
	}
	if _, err := manager.HandleOrderListProgress(entryFilled); err != nil {
		return err
	}
	stop := executionEngineOrderReport("order-stop", "stop", model.OrderStatusAccepted)
	stop.OrderListID = list.ID
	stop.Contingency = model.ContingencyTypeOUO
	stop.Side = model.OrderSideSell
	target := executionEngineOrderReport("order-target", "target", model.OrderStatusAccepted)
	target.OrderListID = list.ID
	target.Contingency = model.ContingencyTypeOUO
	target.Side = model.OrderSideSell
	if err := manager.ApplyOrderReport(stop); err != nil {
		return err
	}
	if err := manager.ApplyOrderReport(target); err != nil {
		return err
	}
	stop.Status = model.OrderStatusPartiallyFilled
	stop.FilledQuantity = decimal.RequireFromString("0.4")
	stop.LeavesQuantity = decimal.RequireFromString("0.6")
	if err := manager.ApplyOrderReport(stop); err != nil {
		return err
	}
	if _, err := manager.HandleOrderListProgress(stop); err != nil {
		return err
	}
	snapshot, ok = manager.OrderListSnapshot("acct", list.ID)
	if !ok || len(snapshot.HeldChildren) != 0 || len(snapshot.Orders) != 3 || len(snapshot.FillProgress) != 1 {
		return fmt.Errorf("unexpected progressed order-list snapshot: ok=%v snapshot=%+v", ok, snapshot)
	}
	if snapshot.Status != execution.OrderListStatusOpen ||
		snapshot.MemberCount != 3 ||
		snapshot.OpenCount != 2 ||
		snapshot.TerminalCount != 1 ||
		snapshot.HeldCount != 0 {
		return fmt.Errorf("unexpected progressed order-list lifecycle snapshot: %+v", snapshot)
	}
	if snapshot.FillProgress[0].OrderID != "order-stop" || !snapshot.FillProgress[0].FilledQuantity.Equal(decimal.RequireFromString("0.4")) {
		return fmt.Errorf("unexpected OUO fill progress: %+v", snapshot.FillProgress)
	}
	stop.Status = model.OrderStatusFilled
	stop.FilledQuantity = decimal.RequireFromString("1")
	stop.LeavesQuantity = decimal.Zero
	if err := manager.ApplyOrderReport(stop); err != nil {
		return err
	}
	if _, err := manager.HandleOrderListProgress(stop); err != nil {
		return err
	}
	target.Status = model.OrderStatusCanceled
	target.LeavesQuantity = decimal.Zero
	if err := manager.ApplyOrderReport(target); err != nil {
		return err
	}
	if _, err := manager.HandleOrderListProgress(target); err != nil {
		return err
	}
	snapshot, ok = manager.OrderListSnapshot("acct", list.ID)
	if !ok ||
		snapshot.Status != execution.OrderListStatusClosed ||
		snapshot.OpenCount != 0 ||
		snapshot.TerminalCount != 3 ||
		snapshot.HeldCount != 0 ||
		len(snapshot.FillProgress) != 0 {
		return fmt.Errorf("unexpected closed order-list lifecycle snapshot: ok=%v snapshot=%+v", ok, snapshot)
	}
	snapshots := manager.OrderListSnapshots("acct")
	if len(snapshots) != 1 || snapshots[0].OrderListID != list.ID {
		return fmt.Errorf("unexpected order-list snapshot collection: %+v", snapshots)
	}
	return nil
}

func (e *ExecutionEngineTester) runManagerLegFillWithoutOrder() error {
	c := cache.New()
	manager := execution.NewManager(execution.Config{Cache: c})
	fill := executionEngineFill("trade-leg-1", "", "spread-LEG-1", "0.25", "101")
	fill.VenueOrderID = "venue-LEG-1"
	fill.PositionID = "leg-position-1"
	fill.IsLeg = true
	applied, err := manager.ApplyFill(fill)
	if err != nil {
		return err
	}
	if !applied {
		return fmt.Errorf("leg fill was not applied")
	}
	cached, ok := c.FillByTradeID("acct", "trade-leg-1")
	if !ok {
		return fmt.Errorf("leg fill was not cached")
	}
	if !cached.IsLegFill() || cached.PositionID != "leg-position-1" {
		return fmt.Errorf("unexpected leg fill identity: %+v", cached)
	}
	if deferred := c.DeferredFillsForOrder("acct", ""); len(deferred) != 0 {
		return fmt.Errorf("leg fill was deferred: %+v", deferred)
	}
	applied, err = manager.ApplyFill(fill)
	if err != nil {
		return err
	}
	if applied {
		return fmt.Errorf("duplicate leg fill was applied")
	}
	return nil
}

func (e *ExecutionEngineTester) runPlatformExecutionEngineDelegation(ctx context.Context) error {
	client := newExecutionEngineFakeClient("acct")
	node := platform.NewNode(platform.Config{})
	if err := node.AddExecutionClient(client); err != nil {
		return err
	}
	if err := node.Start(ctx); err != nil {
		return err
	}
	defer node.Stop(context.Background())
	order := executionEngineSubmit("client-platform-engine", model.OrderSideBuy, decimal.RequireFromString("1"))
	report, err := node.SubmitOrder(ctx, order)
	if err != nil {
		return err
	}
	if _, err := node.ModifyOrder(ctx, model.ModifyOrder{
		AccountID:     order.AccountID,
		InstrumentID:  order.InstrumentID,
		ClientOrderID: order.ClientOrderID,
		OrderID:       report.OrderID,
		Quantity:      decimal.RequireFromString("1"),
		Price:         decimal.RequireFromString("99"),
	}); err != nil {
		return err
	}
	if _, err := node.QueryOrder(ctx, model.QueryOrder{
		AccountID:     order.AccountID,
		InstrumentID:  order.InstrumentID,
		ClientOrderID: "client-platform-engine-query",
	}); err != nil {
		return err
	}
	if _, err := node.CancelOrder(ctx, model.CancelOrder{
		AccountID:     order.AccountID,
		InstrumentID:  order.InstrumentID,
		ClientOrderID: order.ClientOrderID,
		OrderID:       report.OrderID,
	}); err != nil {
		return err
	}
	health := node.ExecutionEngine().Health()
	if health.Submits != 1 || health.Modifies != 1 || health.Queries != 1 || health.Cancels != 1 {
		return fmt.Errorf("platform did not route through execution engine: %+v", health)
	}
	return nil
}

func (e *ExecutionEngineTester) runEngineCompositeCommands(ctx context.Context) error {
	client := newExecutionEngineFakeClient("acct")
	engine := execution.NewEngine(execution.EngineConfig{})
	if err := engine.AddClient(client); err != nil {
		return err
	}
	list := executionEngineBulkList("bulk-list", "bulk-1", "bulk-2")
	reports, err := engine.SubmitOrderList(ctx, model.SubmitOrderList{
		AccountID: "acct",
		List:      list,
	})
	if err != nil {
		return err
	}
	if len(reports) != 2 {
		return fmt.Errorf("expected two list reports, got %d", len(reports))
	}
	if _, err := engine.BatchCancelOrders(ctx, model.BatchCancelOrders{
		AccountID:    "acct",
		InstrumentID: executionEngineInstrumentID(),
		Cancels: []model.CancelOrder{
			{ClientOrderID: "bulk-1"},
			{ClientOrderID: "bulk-2"},
		},
	}); err != nil {
		return err
	}
	if _, err := engine.SubmitOrder(ctx, executionEngineSubmit("cancel-all-buy", model.OrderSideBuy, decimal.RequireFromString("1"))); err != nil {
		return err
	}
	if _, err := engine.SubmitOrder(ctx, executionEngineSubmit("cancel-all-sell", model.OrderSideSell, decimal.RequireFromString("1"))); err != nil {
		return err
	}
	cancelAllReports, err := engine.CancelAllOrders(ctx, model.CancelAllOrders{
		AccountID:    "acct",
		InstrumentID: executionEngineInstrumentID(),
		OrderSide:    model.OrderSideBuy,
	})
	if err != nil {
		return err
	}
	if len(cancelAllReports) != 1 || cancelAllReports[0].ClientOrderID != "cancel-all-buy" {
		return fmt.Errorf("expected buy-only cancel-all report, got %+v", cancelAllReports)
	}
	snapshot, err := engine.QueryAccount(ctx, model.QueryAccount{AccountID: "acct"})
	if err != nil {
		return err
	}
	if snapshot.AccountID != "acct" {
		return fmt.Errorf("account query mismatch: %+v", snapshot)
	}
	health := engine.Health()
	if health.Submits != 4 || health.Cancels != 3 {
		return fmt.Errorf("composite command counters mismatch: %+v", health)
	}
	return nil
}

func (e *ExecutionEngineTester) runEngineReportGeneration(ctx context.Context) error {
	client := newExecutionEngineFakeClient("acct")
	order := executionEngineOrderReport("order-report-1", "client-report-1", model.OrderStatusAccepted)
	fill := executionEngineFill("trade-report-1", "order-report-1", "client-report-1", "1", "101")
	position := executionEnginePosition("position-report-1")
	client.orderReports = []model.OrderStatusReport{order}
	client.fillReports = []model.FillReport{fill}
	client.positionReports = []model.PositionStatusReport{position}
	engine := execution.NewEngine(execution.EngineConfig{})
	if err := engine.AddClient(client); err != nil {
		return err
	}
	status, err := engine.GenerateExecutionMassStatus(ctx, model.GenerateExecutionMassStatus{
		AccountID:    "acct",
		InstrumentID: executionEngineInstrumentID(),
	})
	if err != nil {
		return err
	}
	if len(status.Accounts) != 1 || len(status.Orders) != 1 || len(status.Fills) != 1 || len(status.Positions) != 1 {
		return fmt.Errorf("unexpected mass status: %+v", status)
	}
	return nil
}

func (e *ExecutionEngineTester) runBacktestExecutionEngineDelegation(ctx context.Context) error {
	engine := backtest.NewEngine(backtest.EngineConfig{})
	probe := &executionEngineBacktestProbe{instrumentID: executionEngineInstrumentID()}
	engine.AddStrategy(strategy.NewTyped("execution-engine-backtest-probe", probe))
	result, err := engine.Run(ctx)
	if err != nil {
		return err
	}
	health := result.Execution
	if health.Submits != 1 || health.Modifies != 1 || health.Queries != 1 || health.Cancels != 1 {
		return fmt.Errorf("backtest did not route through execution engine: %+v", health)
	}
	if probe.cancelStatus != model.OrderStatusCanceled {
		return fmt.Errorf("expected canceled order, got %s", probe.cancelStatus)
	}
	return nil
}

func (e *ExecutionEngineTester) runEngineExternalOrderClaim(ctx context.Context) error {
	client := newExecutionEngineFakeClient("acct")
	report := executionEngineOrderReport("external-order-1", "", model.OrderStatusAccepted)
	report.Metadata = model.CommandMetadata{}
	client.orderReports = []model.OrderStatusReport{report}
	c := cache.New()
	engine := execution.NewEngine(execution.EngineConfig{Cache: c})
	if err := engine.AddClient(client); err != nil {
		return err
	}
	if err := engine.RegisterExternalOrderClaim(executionEngineInstrumentID(), "strategy-claim"); err != nil {
		return err
	}
	if err := engine.RegisterExternalOrderClaim(executionEngineInstrumentID(), "other-strategy"); err == nil {
		return fmt.Errorf("expected duplicate external order claim to fail")
	}
	claim, ok := engine.ExternalOrderClaim(executionEngineInstrumentID())
	if !ok || claim != "strategy-claim" {
		return fmt.Errorf("external claim mismatch: %s ok=%v", claim, ok)
	}
	reports, err := engine.GenerateOrderStatusReports(ctx, model.GenerateOrderStatusReports{
		AccountID:    "acct",
		InstrumentID: executionEngineInstrumentID(),
	})
	if err != nil {
		return err
	}
	if len(reports) != 1 || reports[0].Metadata.StrategyID != "strategy-claim" {
		return fmt.Errorf("external order was not claimed: %+v", reports)
	}
	if len(c.OrdersByStrategy("acct", "strategy-claim")) != 1 {
		return fmt.Errorf("claimed external order was not indexed by strategy")
	}
	return nil
}

func (e *ExecutionEngineTester) runEngineSnapshotPurge() error {
	c := cache.New()
	engine := execution.NewEngine(execution.EngineConfig{Cache: c})
	c.PutAccount(model.AccountSnapshot{AccountID: "acct", Venue: "BINANCE"})
	openOrder := executionEngineOrderReport("open-order", "open-client", model.OrderStatusAccepted)
	closedOrder := executionEngineOrderReport("closed-order", "closed-client", model.OrderStatusFilled)
	closedOrder.FilledQuantity = closedOrder.Quantity
	closedOrder.LeavesQuantity = decimal.Zero
	if err := engine.Manager().ApplyOrderReport(openOrder); err != nil {
		return err
	}
	if err := engine.Manager().ApplyOrderReport(closedOrder); err != nil {
		return err
	}
	openPosition := executionEnginePosition("open-position")
	if err := c.PutPosition(openPosition); err != nil {
		return err
	}
	closedPosition := executionEnginePosition("closed-position")
	closedPosition.Side = model.PositionSideFlat
	closedPosition.Quantity = decimal.Zero
	if err := c.PutPosition(closedPosition); err != nil {
		return err
	}
	snapshot := engine.Snapshot("acct")
	if len(snapshot.OpenOrders) != 1 || len(snapshot.ClosedOrders) != 1 || len(snapshot.OpenPositions) != 1 || len(snapshot.ClosedPositions) != 1 {
		return fmt.Errorf("unexpected execution snapshot: %+v", snapshot)
	}
	result := engine.Purge("acct", cache.PurgePolicy{ClosedOrdersLimit: 0, ClosedPositionsLimit: 0})
	if result.ClosedOrders != 1 || result.ClosedPositions != 1 {
		return fmt.Errorf("unexpected purge result: %+v", result)
	}
	snapshot = engine.Snapshot("acct")
	if len(snapshot.OpenOrders) != 1 || len(snapshot.ClosedOrders) != 0 || len(snapshot.OpenPositions) != 1 || len(snapshot.ClosedPositions) != 0 {
		return fmt.Errorf("unexpected post-purge snapshot: %+v", snapshot)
	}
	return nil
}

func (e *ExecutionEngineTester) runEngineExecAlgorithmRouting(ctx context.Context) error {
	client := newExecutionEngineFakeClient("acct")
	algorithm := &executionEngineFakeAlgorithm{id: "twap"}
	c := cache.New()
	engine := execution.NewEngine(execution.EngineConfig{Cache: c})
	if err := engine.AddClient(client); err != nil {
		return err
	}
	if err := engine.AddAlgorithm(algorithm); err != nil {
		return err
	}
	order := executionEngineSubmit("client-algo", model.OrderSideBuy, decimal.RequireFromString("1"))
	order.Metadata.ExecAlgorithmID = "twap"
	report, err := engine.SubmitOrder(ctx, order)
	if err != nil {
		return err
	}
	if report.Status != model.OrderStatusEmulated {
		return fmt.Errorf("expected emulated algorithm report, got %s", report.Status)
	}
	if len(algorithm.orders) != 1 || algorithm.orders[0].ClientOrderID != "client-algo" {
		return fmt.Errorf("algorithm did not receive order: %+v", algorithm.orders)
	}
	if client.HasCall("submit:client-algo") {
		return fmt.Errorf("venue client received algorithm-managed order")
	}
	cached, ok := c.OrderByClientID("acct", "client-algo")
	if !ok || cached.Status != model.OrderStatusEmulated || cached.Metadata.ExecAlgorithmID != "twap" {
		return fmt.Errorf("algorithm order was not cached: ok=%v report=%+v", ok, cached)
	}
	if engine.Health().Algorithms != 1 {
		return fmt.Errorf("algorithm health count mismatch: %+v", engine.Health())
	}
	missing := executionEngineSubmit("client-missing-algo", model.OrderSideBuy, decimal.RequireFromString("1"))
	missing.Metadata.ExecAlgorithmID = "missing"
	if _, err := engine.SubmitOrder(ctx, missing); !errors.Is(err, execution.ErrAlgorithmNotFound) {
		return fmt.Errorf("expected ErrAlgorithmNotFound, got %v", err)
	}
	return nil
}

func (e *ExecutionEngineTester) runEngineTriggerEmulation(ctx context.Context) error {
	client := newExecutionEngineFakeClient("acct")
	c := cache.New()
	engine := execution.NewEngine(execution.EngineConfig{Cache: c, Emulator: execution.NewEmulator(execution.EmulatorConfig{})})
	if err := engine.AddClient(client); err != nil {
		return err
	}
	order := executionEngineSubmit("client-emulated-stop", model.OrderSideBuy, decimal.RequireFromString("1"))
	order.Type = model.OrderTypeStopMarket
	order.Price = decimal.Zero
	order.TriggerPrice = decimal.RequireFromString("101")
	order.EmulationTrigger = model.TriggerTypeBidAsk
	report, err := engine.SubmitOrder(ctx, order)
	if err != nil {
		return err
	}
	if report.Status != model.OrderStatusEmulated {
		return fmt.Errorf("expected emulated report, got %s", report.Status)
	}
	if client.HasCall("submit:client-emulated-stop") {
		return fmt.Errorf("venue client received held emulated order")
	}
	if cached, ok := c.OrderByClientID("acct", "client-emulated-stop"); !ok || cached.Status != model.OrderStatusEmulated {
		return fmt.Errorf("emulated order was not cached: ok=%v report=%+v", ok, cached)
	}
	reports, err := engine.ProcessMarketEvent(ctx, model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: order.InstrumentID,
		BidPrice:     decimal.RequireFromString("100"),
		AskPrice:     decimal.RequireFromString("100.5"),
		BidSize:      decimal.RequireFromString("1"),
		AskSize:      decimal.RequireFromString("1"),
	}})
	if err != nil {
		return err
	}
	if len(reports) != 0 || client.HasCall("submit:client-emulated-stop") {
		return fmt.Errorf("emulated order released before trigger: reports=%+v calls=%+v", reports, client.Calls())
	}
	reports, err = engine.ProcessMarketEvent(ctx, model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: order.InstrumentID,
		BidPrice:     decimal.RequireFromString("100.9"),
		AskPrice:     decimal.RequireFromString("101"),
		BidSize:      decimal.RequireFromString("1"),
		AskSize:      decimal.RequireFromString("1"),
	}})
	if err != nil {
		return err
	}
	if len(reports) != 1 || reports[0].Status != model.OrderStatusAccepted {
		return fmt.Errorf("expected one accepted release report, got %+v", reports)
	}
	if cached, ok := c.OrderByClientID("acct", "client-emulated-stop"); !ok || cached.Status != model.OrderStatusAccepted {
		return fmt.Errorf("released order was not cached as accepted: ok=%v report=%+v", ok, cached)
	}
	if len(c.OpenOrders("acct")) != 1 {
		return fmt.Errorf("expected one open released order, got %+v", c.OpenOrders("acct"))
	}
	reports, err = engine.ProcessMarketEvent(ctx, model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: order.InstrumentID,
		BidPrice:     decimal.RequireFromString("101"),
		AskPrice:     decimal.RequireFromString("101.5"),
		BidSize:      decimal.RequireFromString("1"),
		AskSize:      decimal.RequireFromString("1"),
	}})
	if err != nil {
		return err
	}
	if len(reports) != 0 || countExecutionEngineCalls(client.Calls(), "submit:client-emulated-stop") != 1 {
		return fmt.Errorf("emulated order released more than once: reports=%+v calls=%+v", reports, client.Calls())
	}
	return nil
}

func (e *ExecutionEngineTester) runEngineEmulationLifecycleEvents(ctx context.Context) error {
	client := newExecutionEngineFakeClient("acct")
	engine := execution.NewEngine(execution.EngineConfig{Emulator: execution.NewEmulator(execution.EmulatorConfig{})})
	if err := engine.AddClient(client); err != nil {
		return err
	}
	order := executionEngineSubmit("client-emulated-events", model.OrderSideBuy, decimal.RequireFromString("1"))
	order.Type = model.OrderTypeStopMarket
	order.Price = decimal.Zero
	order.TriggerPrice = decimal.RequireFromString("101")
	order.EmulationTrigger = model.TriggerTypeBidAsk
	report, err := engine.SubmitOrder(ctx, order)
	if err != nil {
		return err
	}
	if report.Status != model.OrderStatusEmulated {
		return fmt.Errorf("expected emulated report, got %s", report.Status)
	}
	emulated, err := readExecutionEngineLifecycle(engine.Events())
	if err != nil {
		return err
	}
	if err := requireExecutionEngineLifecycle(emulated, order.ClientOrderID, model.OrderEventEmulated, model.OrderStatusInitialized, model.OrderStatusEmulated); err != nil {
		return err
	}
	reports, err := engine.ProcessMarketEvent(ctx, model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: order.InstrumentID,
		BidPrice:     decimal.RequireFromString("100"),
		AskPrice:     decimal.RequireFromString("100.5"),
		BidSize:      decimal.RequireFromString("1"),
		AskSize:      decimal.RequireFromString("1"),
	}})
	if err != nil {
		return err
	}
	if len(reports) != 0 {
		return fmt.Errorf("emulated order released before trigger: reports=%+v", reports)
	}
	if err := requireNoExecutionEngineEvent(engine.Events()); err != nil {
		return err
	}
	reports, err = engine.ProcessMarketEvent(ctx, model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: order.InstrumentID,
		BidPrice:     decimal.RequireFromString("100.9"),
		AskPrice:     decimal.RequireFromString("101"),
		BidSize:      decimal.RequireFromString("1"),
		AskSize:      decimal.RequireFromString("1"),
	}})
	if err != nil {
		return err
	}
	if len(reports) != 1 || reports[0].Status != model.OrderStatusAccepted {
		return fmt.Errorf("expected one accepted release report, got %+v", reports)
	}
	triggered, err := readExecutionEngineLifecycle(engine.Events())
	if err != nil {
		return err
	}
	if err := requireExecutionEngineLifecycle(triggered, order.ClientOrderID, model.OrderEventTriggered, model.OrderStatusEmulated, model.OrderStatusTriggered); err != nil {
		return err
	}
	released, err := readExecutionEngineLifecycle(engine.Events())
	if err != nil {
		return err
	}
	if err := requireExecutionEngineLifecycle(released, order.ClientOrderID, model.OrderEventReleased, model.OrderStatusTriggered, model.OrderStatusReleased); err != nil {
		return err
	}
	reports, err = engine.ProcessMarketEvent(ctx, model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: order.InstrumentID,
		BidPrice:     decimal.RequireFromString("101"),
		AskPrice:     decimal.RequireFromString("101.5"),
		BidSize:      decimal.RequireFromString("1"),
		AskSize:      decimal.RequireFromString("1"),
	}})
	if err != nil {
		return err
	}
	if len(reports) != 0 {
		return fmt.Errorf("emulated order released more than once: reports=%+v", reports)
	}
	return requireNoExecutionEngineEvent(engine.Events())
}

func (e *ExecutionEngineTester) runPlatformDataEngineEmulation(ctx context.Context) error {
	instrumentID := executionEngineInstrumentID()
	dataClient := newDataEngineFakeClient(instrumentID)
	execClient := newExecutionEngineFakeClient("acct")
	c := cache.New()
	node := platform.NewNode(platform.Config{
		Cache:           c,
		ExecutionEngine: execution.NewEngine(execution.EngineConfig{Cache: c, Emulator: execution.NewEmulator(execution.EmulatorConfig{})}),
	})
	if err := node.AddDataClient(dataClient); err != nil {
		return err
	}
	if err := node.AddExecutionClient(execClient); err != nil {
		return err
	}
	if err := node.Start(ctx); err != nil {
		return err
	}
	defer node.Stop(context.Background())
	events := node.Bus().Subscribe(platform.TopicExecution, 16)
	defer events.Close()

	order := executionEngineSubmit("client-platform-emulated-stop", model.OrderSideBuy, decimal.RequireFromString("1"))
	order.Type = model.OrderTypeStopMarket
	order.Price = decimal.Zero
	order.TriggerPrice = decimal.RequireFromString("101")
	order.EmulationTrigger = model.TriggerTypeBidAsk
	report, err := node.SubmitOrder(ctx, order)
	if err != nil {
		return err
	}
	if report.Status != model.OrderStatusEmulated {
		return fmt.Errorf("expected emulated report, got %s", report.Status)
	}
	if execClient.HasCall("submit:client-platform-emulated-stop") {
		return fmt.Errorf("venue client received held emulated order")
	}
	quoteSub := model.SubscribeMarketData{InstrumentID: instrumentID, Type: model.MarketDataTypeQuoteTick}
	if err := waitExecutionEngineCondition(ctx, time.Second, func() bool {
		return dataClient.SubscriptionCount(quoteSub) == 1
	}); err != nil {
		return err
	}
	if _, err := readExecutionEngineBusLifecycle(ctx, events.C(), model.OrderEventSubmitted); err != nil {
		return err
	}
	emulated, err := readExecutionEngineBusLifecycle(ctx, events.C(), model.OrderEventEmulated)
	if err != nil {
		return err
	}
	if emulated.Status != model.OrderStatusEmulated || emulated.PreviousStatus != model.OrderStatusInitialized {
		return fmt.Errorf("unexpected emulated lifecycle: %+v", emulated)
	}

	dataClient.Emit(model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: instrumentID,
		BidPrice:     decimal.RequireFromString("100"),
		AskPrice:     decimal.RequireFromString("100.5"),
		BidSize:      decimal.RequireFromString("1"),
		AskSize:      decimal.RequireFromString("1"),
	}})
	if err := waitExecutionEngineCondition(ctx, time.Second, func() bool {
		return node.DataEngine().Health().Events >= 1
	}); err != nil {
		return err
	}
	if execClient.HasCall("submit:client-platform-emulated-stop") {
		return fmt.Errorf("venue client received emulated order before trigger")
	}

	dataClient.Emit(model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: instrumentID,
		BidPrice:     decimal.RequireFromString("100.9"),
		AskPrice:     decimal.RequireFromString("101"),
		BidSize:      decimal.RequireFromString("1"),
		AskSize:      decimal.RequireFromString("1"),
	}})
	triggered, err := readExecutionEngineBusLifecycle(ctx, events.C(), model.OrderEventTriggered)
	if err != nil {
		return err
	}
	if triggered.Status != model.OrderStatusTriggered || triggered.PreviousStatus != model.OrderStatusEmulated {
		return fmt.Errorf("unexpected triggered lifecycle: %+v", triggered)
	}
	released, err := readExecutionEngineBusLifecycle(ctx, events.C(), model.OrderEventReleased)
	if err != nil {
		return err
	}
	if released.Status != model.OrderStatusReleased || released.PreviousStatus != model.OrderStatusTriggered {
		return fmt.Errorf("unexpected released lifecycle: %+v", released)
	}
	accepted, err := readExecutionEngineBusLifecycle(ctx, events.C(), model.OrderEventAccepted)
	if err != nil {
		return err
	}
	if accepted.Status != model.OrderStatusAccepted || accepted.PreviousStatus != model.OrderStatusReleased {
		return fmt.Errorf("unexpected accepted lifecycle: %+v", accepted)
	}
	if countExecutionEngineCalls(execClient.Calls(), "submit:client-platform-emulated-stop") != 1 {
		return fmt.Errorf("expected one venue submit after trigger, got %v", execClient.Calls())
	}
	if cached, ok := node.Cache().OrderByClientID("acct", "client-platform-emulated-stop"); !ok || cached.Status != model.OrderStatusAccepted {
		return fmt.Errorf("released order was not cached as accepted: ok=%v report=%+v", ok, cached)
	}
	return nil
}

func (e *ExecutionEngineTester) runEngineTrailingStopEmulation(ctx context.Context) error {
	client := newExecutionEngineFakeClient("acct")
	engine := execution.NewEngine(execution.EngineConfig{Emulator: execution.NewEmulator(execution.EmulatorConfig{})})
	if err := engine.AddClient(client); err != nil {
		return err
	}
	order := executionEngineSubmit("client-trailing-emulated", model.OrderSideSell, decimal.RequireFromString("1"))
	order.Type = model.OrderTypeTrailingStopMarket
	order.Price = decimal.Zero
	order.TriggerPrice = decimal.Zero
	order.ActivationPrice = decimal.RequireFromString("100")
	order.TrailingOffset = decimal.RequireFromString("5")
	order.EmulationTrigger = model.TriggerTypeBidAsk
	report, err := engine.SubmitOrder(ctx, order)
	if err != nil {
		return err
	}
	if report.Status != model.OrderStatusEmulated {
		return fmt.Errorf("expected emulated report, got %s", report.Status)
	}
	if _, err := readExecutionEngineLifecycle(engine.Events()); err != nil {
		return err
	}
	reports, err := engine.ProcessMarketEvent(ctx, model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: order.InstrumentID,
		BidPrice:     decimal.RequireFromString("100"),
		AskPrice:     decimal.RequireFromString("101"),
		BidSize:      decimal.RequireFromString("1"),
		AskSize:      decimal.RequireFromString("1"),
	}})
	if err != nil {
		return err
	}
	if len(reports) != 0 {
		return fmt.Errorf("trailing order released at activation: reports=%+v", reports)
	}
	if err := requireNoExecutionEngineEvent(engine.Events()); err != nil {
		return err
	}
	reports, err = engine.ProcessMarketEvent(ctx, model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: order.InstrumentID,
		BidPrice:     decimal.RequireFromString("110"),
		AskPrice:     decimal.RequireFromString("111"),
		BidSize:      decimal.RequireFromString("1"),
		AskSize:      decimal.RequireFromString("1"),
	}})
	if err != nil {
		return err
	}
	if len(reports) != 0 || client.HasCall("submit:client-trailing-emulated") {
		return fmt.Errorf("trailing order released before drawdown: reports=%+v calls=%+v", reports, client.Calls())
	}
	if err := requireNoExecutionEngineEvent(engine.Events()); err != nil {
		return err
	}
	reports, err = engine.ProcessMarketEvent(ctx, model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: order.InstrumentID,
		BidPrice:     decimal.RequireFromString("104"),
		AskPrice:     decimal.RequireFromString("105"),
		BidSize:      decimal.RequireFromString("1"),
		AskSize:      decimal.RequireFromString("1"),
	}})
	if err != nil {
		return err
	}
	if len(reports) != 1 || reports[0].Status != model.OrderStatusAccepted {
		return fmt.Errorf("expected one accepted trailing release report, got %+v", reports)
	}
	triggered, err := readExecutionEngineLifecycle(engine.Events())
	if err != nil {
		return err
	}
	if triggered.Kind != model.OrderEventTriggered || triggered.Status != model.OrderStatusTriggered {
		return fmt.Errorf("unexpected trailing triggered lifecycle: %+v", triggered)
	}
	if triggered.Report == nil || triggered.Report.TriggerPrice.String() != "105" {
		return fmt.Errorf("expected trailing trigger price 105, got %+v", triggered.Report)
	}
	released, err := readExecutionEngineLifecycle(engine.Events())
	if err != nil {
		return err
	}
	if released.Kind != model.OrderEventReleased || released.PreviousStatus != model.OrderStatusTriggered {
		return fmt.Errorf("unexpected trailing released lifecycle: %+v", released)
	}
	if countExecutionEngineCalls(client.Calls(), "submit:client-trailing-emulated") != 1 {
		return fmt.Errorf("expected one venue submit after trailing trigger, got %v", client.Calls())
	}
	return nil
}

func (e *ExecutionEngineTester) runEngineLocalEmulatedCancel(ctx context.Context) error {
	client := newExecutionEngineFakeClient("acct")
	c := cache.New()
	engine := execution.NewEngine(execution.EngineConfig{Cache: c, Emulator: execution.NewEmulator(execution.EmulatorConfig{})})
	if err := engine.AddClient(client); err != nil {
		return err
	}
	order := executionEngineSubmit("client-local-cancel", model.OrderSideBuy, decimal.RequireFromString("1"))
	order.Type = model.OrderTypeStopMarket
	order.Price = decimal.Zero
	order.TriggerPrice = decimal.RequireFromString("101")
	order.EmulationTrigger = model.TriggerTypeBidAsk
	report, err := engine.SubmitOrder(ctx, order)
	if err != nil {
		return err
	}
	if report.Status != model.OrderStatusEmulated {
		return fmt.Errorf("expected emulated report, got %+v", report)
	}
	if _, err := readExecutionEngineLifecycle(engine.Events()); err != nil {
		return err
	}
	cancelReport, err := engine.CancelOrder(ctx, model.CancelOrder{
		AccountID:     order.AccountID,
		InstrumentID:  order.InstrumentID,
		ClientOrderID: order.ClientOrderID,
	})
	if err != nil {
		return err
	}
	if cancelReport.Status != model.OrderStatusCanceled || cancelReport.OrderID != report.OrderID {
		return fmt.Errorf("expected local emulated cancel report, got %+v", cancelReport)
	}
	if client.HasCall("cancel:client-local-cancel") {
		return fmt.Errorf("emulated cancel reached venue client: %v", client.Calls())
	}
	canceled, err := readExecutionEngineLifecycle(engine.Events())
	if err != nil {
		return err
	}
	if canceled.Kind != model.OrderEventCanceled || canceled.PreviousStatus != model.OrderStatusEmulated {
		return fmt.Errorf("unexpected local cancel lifecycle: %+v", canceled)
	}
	if _, err := engine.ProcessMarketEvent(ctx, model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: order.InstrumentID,
		BidPrice:     decimal.RequireFromString("100.9"),
		AskPrice:     decimal.RequireFromString("101"),
		BidSize:      decimal.RequireFromString("1"),
		AskSize:      decimal.RequireFromString("1"),
	}}); err != nil {
		return err
	}
	if client.HasCall("submit:client-local-cancel") {
		return fmt.Errorf("canceled emulated order was later submitted: %v", client.Calls())
	}
	return nil
}

func (e *ExecutionEngineTester) runEngineLocalEmulatedModify(ctx context.Context) error {
	client := newExecutionEngineFakeClient("acct")
	c := cache.New()
	quote := model.QuoteTick{
		InstrumentID: executionEngineInstrumentID(),
		BidPrice:     decimal.RequireFromString("100.9"),
		AskPrice:     decimal.RequireFromString("101"),
		BidSize:      decimal.RequireFromString("1"),
		AskSize:      decimal.RequireFromString("1"),
	}
	if err := c.PutMarketEvent(model.MarketEvent{Quote: &quote}); err != nil {
		return err
	}
	engine := execution.NewEngine(execution.EngineConfig{Cache: c, Emulator: execution.NewEmulator(execution.EmulatorConfig{})})
	if err := engine.AddClient(client); err != nil {
		return err
	}
	order := executionEngineSubmit("client-local-modify", model.OrderSideBuy, decimal.RequireFromString("1"))
	order.Type = model.OrderTypeStopMarket
	order.Price = decimal.Zero
	order.TriggerPrice = decimal.RequireFromString("105")
	order.EmulationTrigger = model.TriggerTypeBidAsk
	report, err := engine.SubmitOrder(ctx, order)
	if err != nil {
		return err
	}
	if report.Status != model.OrderStatusEmulated {
		return fmt.Errorf("expected emulated report, got %+v", report)
	}
	if _, err := readExecutionEngineLifecycle(engine.Events()); err != nil {
		return err
	}
	modifyReport, err := engine.ModifyOrder(ctx, model.ModifyOrder{
		AccountID:     order.AccountID,
		InstrumentID:  order.InstrumentID,
		ClientOrderID: order.ClientOrderID,
		OrderID:       report.OrderID,
		TriggerPrice:  decimal.RequireFromString("100.5"),
	})
	if err != nil {
		return err
	}
	if modifyReport.Status != model.OrderStatusAccepted {
		return fmt.Errorf("expected released modify to submit accepted venue order, got %+v", modifyReport)
	}
	if client.HasCall("modify:client-local-modify") {
		return fmt.Errorf("emulated modify reached venue modifier: %v", client.Calls())
	}
	if countExecutionEngineCalls(client.Calls(), "submit:client-local-modify") != 1 {
		return fmt.Errorf("expected one venue submit after local emulated modify rematch, got %v", client.Calls())
	}
	updated, err := readExecutionEngineLifecycle(engine.Events())
	if err != nil {
		return err
	}
	if updated.Kind != model.OrderEventUpdated || updated.PreviousStatus != model.OrderStatusEmulated || updated.Status != model.OrderStatusEmulated {
		return fmt.Errorf("unexpected local modify lifecycle: %+v", updated)
	}
	triggered, err := readExecutionEngineLifecycle(engine.Events())
	if err != nil {
		return err
	}
	if triggered.Kind != model.OrderEventTriggered || triggered.PreviousStatus != model.OrderStatusEmulated {
		return fmt.Errorf("unexpected local modify trigger lifecycle: %+v", triggered)
	}
	released, err := readExecutionEngineLifecycle(engine.Events())
	if err != nil {
		return err
	}
	if released.Kind != model.OrderEventReleased || released.PreviousStatus != model.OrderStatusTriggered {
		return fmt.Errorf("unexpected local modify release lifecycle: %+v", released)
	}
	submitted := client.SubmittedOrders()
	if len(submitted) != 1 || submitted[0].Type != model.OrderTypeMarket || submitted[0].EmulationTrigger != model.TriggerTypeNoTrigger {
		return fmt.Errorf("expected transformed market release after local modify, got %+v", submitted)
	}
	return nil
}

func (e *ExecutionEngineTester) runEngineLocalEmulatedCancelAll(ctx context.Context) error {
	client := &executionEngineCancelAllFakeClient{executionEngineFakeClient: newExecutionEngineFakeClient("acct")}
	c := cache.New()
	engine := execution.NewEngine(execution.EngineConfig{Cache: c, Emulator: execution.NewEmulator(execution.EmulatorConfig{})})
	if err := engine.AddClient(client); err != nil {
		return err
	}
	order := executionEngineSubmit("client-local-cancel-all", model.OrderSideBuy, decimal.RequireFromString("1"))
	order.Type = model.OrderTypeStopMarket
	order.Price = decimal.Zero
	order.TriggerPrice = decimal.RequireFromString("101")
	order.EmulationTrigger = model.TriggerTypeBidAsk
	report, err := engine.SubmitOrder(ctx, order)
	if err != nil {
		return err
	}
	if report.Status != model.OrderStatusEmulated {
		return fmt.Errorf("expected emulated report, got %+v", report)
	}
	if _, err := readExecutionEngineLifecycle(engine.Events()); err != nil {
		return err
	}
	cancelReports, err := engine.CancelAllOrders(ctx, model.CancelAllOrders{
		AccountID:    order.AccountID,
		InstrumentID: order.InstrumentID,
		OrderSide:    model.OrderSideBuy,
	})
	if err != nil {
		return err
	}
	if len(cancelReports) != 1 || cancelReports[0].Status != model.OrderStatusCanceled || cancelReports[0].OrderID != report.OrderID {
		return fmt.Errorf("expected one local emulated cancel-all report, got %+v", cancelReports)
	}
	if client.HasCall("cancel-all:"+order.InstrumentID.String()) || client.HasCall("cancel:client-local-cancel-all") {
		return fmt.Errorf("emulated cancel-all reached venue client: %v", client.Calls())
	}
	canceled, err := readExecutionEngineLifecycle(engine.Events())
	if err != nil {
		return err
	}
	if canceled.Kind != model.OrderEventCanceled || canceled.PreviousStatus != model.OrderStatusEmulated {
		return fmt.Errorf("unexpected local cancel-all lifecycle: %+v", canceled)
	}
	if _, err := engine.ProcessMarketEvent(ctx, model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: order.InstrumentID,
		BidPrice:     decimal.RequireFromString("100.9"),
		AskPrice:     decimal.RequireFromString("101"),
		BidSize:      decimal.RequireFromString("1"),
		AskSize:      decimal.RequireFromString("1"),
	}}); err != nil {
		return err
	}
	if client.HasCall("submit:client-local-cancel-all") {
		return fmt.Errorf("cancel-all emulated order was later submitted: %v", client.Calls())
	}
	return nil
}

func (e *ExecutionEngineTester) runEngineInitialEmulationMatch(ctx context.Context) error {
	client := newExecutionEngineFakeClient("acct")
	c := cache.New()
	if err := c.PutMarketEvent(model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: executionEngineInstrumentID(),
		BidPrice:     decimal.RequireFromString("100"),
		AskPrice:     decimal.RequireFromString("101"),
		BidSize:      decimal.RequireFromString("1"),
		AskSize:      decimal.RequireFromString("1"),
	}}); err != nil {
		return err
	}
	engine := execution.NewEngine(execution.EngineConfig{Cache: c, Emulator: execution.NewEmulator(execution.EmulatorConfig{})})
	if err := engine.AddClient(client); err != nil {
		return err
	}
	order := executionEngineSubmit("client-initial-trigger", model.OrderSideBuy, decimal.RequireFromString("1"))
	order.Type = model.OrderTypeStopMarket
	order.Price = decimal.Zero
	order.TriggerPrice = decimal.RequireFromString("101")
	order.EmulationTrigger = model.TriggerTypeBidAsk
	report, err := engine.SubmitOrder(ctx, order)
	if err != nil {
		return err
	}
	if report.Status != model.OrderStatusAccepted || !client.HasCall("submit:client-initial-trigger") {
		return fmt.Errorf("expected initial emulation release to venue, report=%+v calls=%+v", report, client.Calls())
	}
	triggered, err := readExecutionEngineLifecycle(engine.Events())
	if err != nil {
		return err
	}
	if triggered.Kind != model.OrderEventTriggered || triggered.PreviousStatus != model.OrderStatusInitialized {
		return fmt.Errorf("unexpected initial triggered lifecycle: %+v", triggered)
	}
	released, err := readExecutionEngineLifecycle(engine.Events())
	if err != nil {
		return err
	}
	if released.Kind != model.OrderEventReleased || released.PreviousStatus != model.OrderStatusTriggered {
		return fmt.Errorf("unexpected initial released lifecycle: %+v", released)
	}
	return requireNoExecutionEngineEvent(engine.Events())
}

func (e *ExecutionEngineTester) runEngineEmulatedLimitMatching(ctx context.Context) error {
	client := newExecutionEngineFakeClient("acct")
	engine := execution.NewEngine(execution.EngineConfig{Emulator: execution.NewEmulator(execution.EmulatorConfig{})})
	if err := engine.AddClient(client); err != nil {
		return err
	}
	order := executionEngineSubmit("client-emulated-limit", model.OrderSideBuy, decimal.RequireFromString("1"))
	order.Type = model.OrderTypeLimit
	order.Price = decimal.RequireFromString("100")
	order.TriggerPrice = decimal.Zero
	order.EmulationTrigger = model.TriggerTypeBidAsk
	report, err := engine.SubmitOrder(ctx, order)
	if err != nil {
		return err
	}
	if report.Status != model.OrderStatusEmulated || len(client.SubmittedOrders()) != 0 {
		return fmt.Errorf("expected held emulated limit order, report=%+v submitted=%+v", report, client.SubmittedOrders())
	}
	if _, err := readExecutionEngineLifecycle(engine.Events()); err != nil {
		return err
	}
	reports, err := engine.ProcessMarketEvent(ctx, model.MarketEvent{OrderBook: &model.OrderBook{
		InstrumentID: order.InstrumentID,
		Asks: []model.OrderBookLevel{{
			Price: decimal.RequireFromString("101"),
			Size:  decimal.RequireFromString("1"),
		}},
		Timestamp: time.Unix(10, 0),
	}})
	if err != nil {
		return err
	}
	if len(reports) != 0 || len(client.SubmittedOrders()) != 0 {
		return fmt.Errorf("expected unmarketable emulated limit to stay local, reports=%+v submitted=%+v", reports, client.SubmittedOrders())
	}
	reports, err = engine.ProcessMarketEvent(ctx, model.MarketEvent{OrderBook: &model.OrderBook{
		InstrumentID: order.InstrumentID,
		Asks: []model.OrderBookLevel{{
			Price: decimal.RequireFromString("100"),
			Size:  decimal.RequireFromString("1"),
		}},
		Timestamp: time.Unix(11, 0),
	}})
	if err != nil {
		return err
	}
	submitted := client.SubmittedOrders()
	if len(reports) != 1 ||
		reports[0].Status != model.OrderStatusAccepted ||
		reports[0].Type != model.OrderTypeLimit ||
		len(submitted) != 1 ||
		submitted[0].Type != model.OrderTypeLimit ||
		submitted[0].EmulationTrigger != model.TriggerTypeNoTrigger ||
		!submitted[0].TriggerPrice.IsZero() {
		return fmt.Errorf("expected marketable emulated limit release, reports=%+v submitted=%+v", reports, submitted)
	}
	return nil
}

func (e *ExecutionEngineTester) runEngineKernelLifecycle(ctx context.Context) error {
	client := newExecutionEngineFakeClient("acct")
	engine := execution.NewEngine(execution.EngineConfig{})
	if err := engine.AddClient(client); err != nil {
		return err
	}
	if health := engine.Health(); health.State != kernel.ComponentStateInitialized || health.Starts != 0 || health.Stops != 0 {
		return fmt.Errorf("unexpected initial execution health: %+v", health)
	}
	if err := engine.Start(ctx); err != nil {
		return err
	}
	if health := engine.Health(); health.State != kernel.ComponentStateRunning || health.Starts != 1 || health.Stops != 0 {
		return fmt.Errorf("unexpected running execution health: %+v", health)
	}
	if err := engine.Stop(ctx); err != nil {
		return err
	}
	if health := engine.Health(); health.State != kernel.ComponentStateStopped || health.Starts != 1 || health.Stops != 1 {
		return fmt.Errorf("unexpected stopped execution health: %+v", health)
	}
	return nil
}

func (e *ExecutionEngineTester) runEngineSyntheticTriggerInstrumentEmulation(ctx context.Context) error {
	client := newExecutionEngineFakeClient("acct")
	c := cache.New()
	synthID := executionEngineSyntheticInstrumentID()
	if err := c.PutSyntheticInstrument(model.SyntheticInstrument{
		ID:             synthID,
		PricePrecision: 2,
		PriceTick:      decimal.RequireFromString("0.25"),
		Components: []model.InstrumentID{
			executionEngineInstrumentID(),
			executionEngineTriggerInstrumentID(),
		},
		Formula: "BTC-USDT-SPOT.BINANCE - ETH-USDT-SPOT.BINANCE",
	}); err != nil {
		return err
	}
	engine := execution.NewEngine(execution.EngineConfig{Cache: c, Emulator: execution.NewEmulator(execution.EmulatorConfig{})})
	if err := engine.AddClient(client); err != nil {
		return err
	}
	order := executionEngineSubmit("client-synthetic-trigger", model.OrderSideSell, decimal.RequireFromString("1"))
	order.Type = model.OrderTypeTrailingStopMarket
	order.Price = decimal.Zero
	order.TriggerPrice = decimal.Zero
	order.TriggerInstrumentID = synthID
	order.ActivationPrice = decimal.RequireFromString("100")
	order.TrailingOffset = decimal.RequireFromString("10")
	order.TrailingOffsetType = model.TrailingOffsetTypeTicks
	order.EmulationTrigger = model.TriggerTypeBidAsk
	report, err := engine.SubmitOrder(ctx, order)
	if err != nil {
		return err
	}
	if report.Status != model.OrderStatusEmulated || report.TriggerInstrumentID != synthID {
		return fmt.Errorf("expected emulated synthetic trigger report, got %+v", report)
	}
	if _, err := readExecutionEngineLifecycle(engine.Events()); err != nil {
		return err
	}
	for _, bid := range []string{"100", "105"} {
		if _, err := engine.ProcessMarketEvent(ctx, model.MarketEvent{Quote: &model.QuoteTick{
			InstrumentID: synthID,
			BidPrice:     decimal.RequireFromString(bid),
			AskPrice:     decimal.RequireFromString("106"),
			BidSize:      decimal.RequireFromString("1"),
			AskSize:      decimal.RequireFromString("1"),
		}}); err != nil {
			return err
		}
	}
	reports, err := engine.ProcessMarketEvent(ctx, model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: synthID,
		BidPrice:     decimal.RequireFromString("102.5"),
		AskPrice:     decimal.RequireFromString("103.5"),
		BidSize:      decimal.RequireFromString("1"),
		AskSize:      decimal.RequireFromString("1"),
	}})
	if err != nil {
		return err
	}
	if len(reports) != 1 || reports[0].Status != model.OrderStatusAccepted {
		return fmt.Errorf("expected synthetic trigger release report, got %+v", reports)
	}
	triggered, err := readExecutionEngineLifecycle(engine.Events())
	if err != nil {
		return err
	}
	if triggered.Report == nil || triggered.Report.TriggerPrice.String() != "102.5" || triggered.Report.TriggerInstrumentID != synthID {
		return fmt.Errorf("unexpected synthetic trigger lifecycle report: %+v", triggered.Report)
	}
	if _, err := readExecutionEngineLifecycle(engine.Events()); err != nil {
		return err
	}
	if countExecutionEngineCalls(client.Calls(), "submit:client-synthetic-trigger") != 1 {
		return fmt.Errorf("expected one venue submit after synthetic trigger, got %v", client.Calls())
	}
	return nil
}

func (e *ExecutionEngineTester) runEngineOrderBookTriggerEmulation(ctx context.Context) error {
	client := newExecutionEngineFakeClient("acct")
	engine := execution.NewEngine(execution.EngineConfig{Emulator: execution.NewEmulator(execution.EmulatorConfig{})})
	if err := engine.AddClient(client); err != nil {
		return err
	}
	order := executionEngineSubmit("client-book-trigger", model.OrderSideBuy, decimal.RequireFromString("1"))
	order.Type = model.OrderTypeStopMarket
	order.Price = decimal.Zero
	order.TriggerPrice = decimal.RequireFromString("201")
	order.EmulationTrigger = model.TriggerTypeBidAsk
	report, err := engine.SubmitOrder(ctx, order)
	if err != nil {
		return err
	}
	if report.Status != model.OrderStatusEmulated {
		return fmt.Errorf("expected emulated report, got %+v", report)
	}
	if _, err := readExecutionEngineLifecycle(engine.Events()); err != nil {
		return err
	}
	reports, err := engine.ProcessMarketEvent(ctx, model.MarketEvent{OrderBook: &model.OrderBook{
		InstrumentID: order.InstrumentID,
		Bids: []model.OrderBookLevel{{
			Price: decimal.RequireFromString("200"),
			Size:  decimal.RequireFromString("1"),
		}},
		Asks: []model.OrderBookLevel{{
			Price: decimal.RequireFromString("201"),
			Size:  decimal.RequireFromString("1"),
		}},
	}})
	if err != nil {
		return err
	}
	if len(reports) != 1 || reports[0].Status != model.OrderStatusAccepted {
		return fmt.Errorf("expected order-book trigger release report, got %+v", reports)
	}
	triggered, err := readExecutionEngineLifecycle(engine.Events())
	if err != nil {
		return err
	}
	if triggered.Kind != model.OrderEventTriggered || triggered.Status != model.OrderStatusTriggered {
		return fmt.Errorf("unexpected order-book triggered lifecycle: %+v", triggered)
	}
	if _, err := readExecutionEngineLifecycle(engine.Events()); err != nil {
		return err
	}
	if countExecutionEngineCalls(client.Calls(), "submit:client-book-trigger") != 1 {
		return fmt.Errorf("expected one venue submit after order-book trigger, got %v", client.Calls())
	}
	return nil
}

func (e *ExecutionEngineTester) runEngineTriggerInstrumentEmulation(ctx context.Context) error {
	client := newExecutionEngineFakeClient("acct")
	engine := execution.NewEngine(execution.EngineConfig{Emulator: execution.NewEmulator(execution.EmulatorConfig{})})
	if err := engine.AddClient(client); err != nil {
		return err
	}
	order := executionEngineSubmit("client-trigger-instrument", model.OrderSideBuy, decimal.RequireFromString("1"))
	order.Type = model.OrderTypeStopMarket
	order.Price = decimal.Zero
	order.TriggerPrice = decimal.RequireFromString("201")
	order.TriggerInstrumentID = executionEngineTriggerInstrumentID()
	order.EmulationTrigger = model.TriggerTypeBidAsk
	report, err := engine.SubmitOrder(ctx, order)
	if err != nil {
		return err
	}
	if report.Status != model.OrderStatusEmulated || report.TriggerInstrumentID != executionEngineTriggerInstrumentID() {
		return fmt.Errorf("expected emulated trigger-instrument report, got %+v", report)
	}
	if _, err := readExecutionEngineLifecycle(engine.Events()); err != nil {
		return err
	}
	reports, err := engine.ProcessMarketEvent(ctx, model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: order.InstrumentID,
		BidPrice:     decimal.RequireFromString("200"),
		AskPrice:     decimal.RequireFromString("201"),
		BidSize:      decimal.RequireFromString("1"),
		AskSize:      decimal.RequireFromString("1"),
	}})
	if err != nil {
		return err
	}
	if len(reports) != 0 || client.HasCall("submit:client-trigger-instrument") {
		return fmt.Errorf("order instrument market data released trigger-instrument order: reports=%+v calls=%+v", reports, client.Calls())
	}
	if err := requireNoExecutionEngineEvent(engine.Events()); err != nil {
		return err
	}
	reports, err = engine.ProcessMarketEvent(ctx, model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: executionEngineTriggerInstrumentID(),
		BidPrice:     decimal.RequireFromString("200"),
		AskPrice:     decimal.RequireFromString("201"),
		BidSize:      decimal.RequireFromString("1"),
		AskSize:      decimal.RequireFromString("1"),
	}})
	if err != nil {
		return err
	}
	if len(reports) != 1 || reports[0].Status != model.OrderStatusAccepted {
		return fmt.Errorf("expected trigger-instrument release report, got %+v", reports)
	}
	triggered, err := readExecutionEngineLifecycle(engine.Events())
	if err != nil {
		return err
	}
	if triggered.Report == nil || triggered.Report.TriggerInstrumentID != executionEngineTriggerInstrumentID() {
		return fmt.Errorf("unexpected trigger-instrument lifecycle report: %+v", triggered.Report)
	}
	if _, err := readExecutionEngineLifecycle(engine.Events()); err != nil {
		return err
	}
	if countExecutionEngineCalls(client.Calls(), "submit:client-trigger-instrument") != 1 {
		return fmt.Errorf("expected one venue submit after trigger-instrument event, got %v", client.Calls())
	}
	return nil
}

func (e *ExecutionEngineTester) runEngineTrailingStopLimitEmulation(ctx context.Context) error {
	client := newExecutionEngineFakeClient("acct")
	engine := execution.NewEngine(execution.EngineConfig{Emulator: execution.NewEmulator(execution.EmulatorConfig{})})
	if err := engine.AddClient(client); err != nil {
		return err
	}
	order := executionEngineSubmit("client-trailing-limit", model.OrderSideSell, decimal.RequireFromString("1"))
	order.Type = model.OrderTypeTrailingStopLimit
	order.Price = decimal.RequireFromString("104")
	order.TriggerPrice = decimal.Zero
	order.ActivationPrice = decimal.RequireFromString("100")
	order.TrailingOffset = decimal.RequireFromString("5")
	order.EmulationTrigger = model.TriggerTypeBidAsk
	report, err := engine.SubmitOrder(ctx, order)
	if err != nil {
		return err
	}
	if report.Status != model.OrderStatusEmulated {
		return fmt.Errorf("expected emulated report, got %s", report.Status)
	}
	if _, err := readExecutionEngineLifecycle(engine.Events()); err != nil {
		return err
	}
	if _, err := engine.ProcessMarketEvent(ctx, model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: order.InstrumentID,
		BidPrice:     decimal.RequireFromString("100"),
		AskPrice:     decimal.RequireFromString("101"),
		BidSize:      decimal.RequireFromString("1"),
		AskSize:      decimal.RequireFromString("1"),
	}}); err != nil {
		return err
	}
	if _, err := engine.ProcessMarketEvent(ctx, model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: order.InstrumentID,
		BidPrice:     decimal.RequireFromString("110"),
		AskPrice:     decimal.RequireFromString("111"),
		BidSize:      decimal.RequireFromString("1"),
		AskSize:      decimal.RequireFromString("1"),
	}}); err != nil {
		return err
	}
	reports, err := engine.ProcessMarketEvent(ctx, model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: order.InstrumentID,
		BidPrice:     decimal.RequireFromString("104"),
		AskPrice:     decimal.RequireFromString("105"),
		BidSize:      decimal.RequireFromString("1"),
		AskSize:      decimal.RequireFromString("1"),
	}})
	if err != nil {
		return err
	}
	if len(reports) != 1 || reports[0].Status != model.OrderStatusAccepted || reports[0].Type != model.OrderTypeLimit || reports[0].Price.String() != "104" {
		return fmt.Errorf("expected accepted released limit report at 104, got %+v", reports)
	}
	triggered, err := readExecutionEngineLifecycle(engine.Events())
	if err != nil {
		return err
	}
	if triggered.Report == nil || triggered.Report.Type != model.OrderTypeTrailingStopLimit || triggered.Report.TriggerPrice.String() != "105" {
		return fmt.Errorf("unexpected trailing stop limit trigger report: %+v", triggered.Report)
	}
	if _, err := readExecutionEngineLifecycle(engine.Events()); err != nil {
		return err
	}
	submitted := client.SubmittedOrders()
	if len(submitted) != 1 || submitted[0].Type != model.OrderTypeLimit || submitted[0].Price.String() != "104" || !submitted[0].TriggerPrice.IsZero() || !submitted[0].TrailingOffset.IsZero() {
		return fmt.Errorf("unexpected trailing stop limit venue submission: %+v", submitted)
	}
	return nil
}

func (e *ExecutionEngineTester) runEngineTrailingOffsetTypes(ctx context.Context) error {
	cases := []struct {
		clientOrderID model.ClientOrderID
		offset        string
		offsetType    model.TrailingOffsetType
		priceTick     string
		drawdownBid   string
		drawdownAsk   string
		triggerPrice  string
	}{
		{
			clientOrderID: "client-trailing-ticks",
			offset:        "10",
			offsetType:    model.TrailingOffsetTypeTicks,
			priceTick:     "0.5",
			drawdownBid:   "104",
			drawdownAsk:   "105",
			triggerPrice:  "105",
		},
		{
			clientOrderID: "client-trailing-bps",
			offset:        "500",
			offsetType:    model.TrailingOffsetTypeBasisPoints,
			priceTick:     "1",
			drawdownBid:   "104.5",
			drawdownAsk:   "105.5",
			triggerPrice:  "104.5",
		},
	}
	for _, tc := range cases {
		if err := e.runEngineTrailingOffsetTypeCase(ctx, tc.clientOrderID, tc.offset, tc.offsetType, tc.priceTick, tc.drawdownBid, tc.drawdownAsk, tc.triggerPrice); err != nil {
			return err
		}
	}
	return nil
}

func (e *ExecutionEngineTester) runEngineTrailingOffsetTypeCase(ctx context.Context, clientOrderID model.ClientOrderID, offset string, offsetType model.TrailingOffsetType, priceTick string, drawdownBid string, drawdownAsk string, triggerPrice string) error {
	client := newExecutionEngineFakeClient("acct")
	c := cache.New()
	if err := c.PutInstrument(executionEngineInstrument(priceTick)); err != nil {
		return err
	}
	engine := execution.NewEngine(execution.EngineConfig{Cache: c, Emulator: execution.NewEmulator(execution.EmulatorConfig{})})
	if err := engine.AddClient(client); err != nil {
		return err
	}
	order := executionEngineSubmit(clientOrderID, model.OrderSideSell, decimal.RequireFromString("1"))
	order.Type = model.OrderTypeTrailingStopMarket
	order.Price = decimal.Zero
	order.TriggerPrice = decimal.Zero
	order.ActivationPrice = decimal.RequireFromString("100")
	order.TrailingOffset = decimal.RequireFromString(offset)
	order.TrailingOffsetType = offsetType
	order.EmulationTrigger = model.TriggerTypeBidAsk
	report, err := engine.SubmitOrder(ctx, order)
	if err != nil {
		return err
	}
	if report.Status != model.OrderStatusEmulated {
		return fmt.Errorf("expected emulated report, got %s", report.Status)
	}
	if _, err := readExecutionEngineLifecycle(engine.Events()); err != nil {
		return err
	}
	if _, err := engine.ProcessMarketEvent(ctx, model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: order.InstrumentID,
		BidPrice:     decimal.RequireFromString("100"),
		AskPrice:     decimal.RequireFromString("101"),
		BidSize:      decimal.RequireFromString("1"),
		AskSize:      decimal.RequireFromString("1"),
	}}); err != nil {
		return err
	}
	if _, err := engine.ProcessMarketEvent(ctx, model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: order.InstrumentID,
		BidPrice:     decimal.RequireFromString("110"),
		AskPrice:     decimal.RequireFromString("111"),
		BidSize:      decimal.RequireFromString("1"),
		AskSize:      decimal.RequireFromString("1"),
	}}); err != nil {
		return err
	}
	reports, err := engine.ProcessMarketEvent(ctx, model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: order.InstrumentID,
		BidPrice:     decimal.RequireFromString(drawdownBid),
		AskPrice:     decimal.RequireFromString(drawdownAsk),
		BidSize:      decimal.RequireFromString("1"),
		AskSize:      decimal.RequireFromString("1"),
	}})
	if err != nil {
		return err
	}
	if len(reports) != 1 || reports[0].Status != model.OrderStatusAccepted {
		return fmt.Errorf("expected one accepted trailing release report, got %+v", reports)
	}
	triggered, err := readExecutionEngineLifecycle(engine.Events())
	if err != nil {
		return err
	}
	if triggered.Kind != model.OrderEventTriggered || triggered.Status != model.OrderStatusTriggered {
		return fmt.Errorf("unexpected trailing offset triggered lifecycle: %+v", triggered)
	}
	if triggered.Report == nil || triggered.Report.TriggerPrice.String() != triggerPrice || triggered.Report.TrailingOffsetType != offsetType {
		return fmt.Errorf("unexpected trailing offset trigger report: want price=%s type=%s got %+v", triggerPrice, offsetType, triggered.Report)
	}
	released, err := readExecutionEngineLifecycle(engine.Events())
	if err != nil {
		return err
	}
	if released.Kind != model.OrderEventReleased || released.PreviousStatus != model.OrderStatusTriggered {
		return fmt.Errorf("unexpected trailing offset released lifecycle: %+v", released)
	}
	if countExecutionEngineCalls(client.Calls(), "submit:"+string(clientOrderID)) != 1 {
		return fmt.Errorf("expected one venue submit after trailing offset trigger, got %v", client.Calls())
	}
	return nil
}

func (e *ExecutionEngineTester) runEngineReleaseOrderTransform(ctx context.Context) error {
	marketClient := newExecutionEngineFakeClient("acct")
	marketEngine := execution.NewEngine(execution.EngineConfig{Emulator: execution.NewEmulator(execution.EmulatorConfig{})})
	if err := marketEngine.AddClient(marketClient); err != nil {
		return err
	}
	marketOrder := executionEngineSubmit("client-release-market", model.OrderSideBuy, decimal.RequireFromString("1"))
	marketOrder.Type = model.OrderTypeStopMarket
	marketOrder.Price = decimal.Zero
	marketOrder.TriggerPrice = decimal.RequireFromString("101")
	marketOrder.EmulationTrigger = model.TriggerTypeBidAsk
	if _, err := marketEngine.SubmitOrder(ctx, marketOrder); err != nil {
		return err
	}
	if _, err := readExecutionEngineLifecycle(marketEngine.Events()); err != nil {
		return err
	}
	marketReports, err := marketEngine.ProcessMarketEvent(ctx, model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: marketOrder.InstrumentID,
		BidPrice:     decimal.RequireFromString("100.9"),
		AskPrice:     decimal.RequireFromString("101"),
		BidSize:      decimal.RequireFromString("1"),
		AskSize:      decimal.RequireFromString("1"),
	}})
	if err != nil {
		return err
	}
	if len(marketReports) != 1 || marketReports[0].Type != model.OrderTypeMarket {
		return fmt.Errorf("expected released stop-market venue report to be market, got %+v", marketReports)
	}
	marketSubmitted := marketClient.SubmittedOrders()
	if len(marketSubmitted) != 1 || marketSubmitted[0].Type != model.OrderTypeMarket || !marketSubmitted[0].TriggerPrice.IsZero() || marketSubmitted[0].EmulationTrigger != model.TriggerTypeNoTrigger {
		return fmt.Errorf("expected stop-market release to submit market order, got %+v", marketSubmitted)
	}

	limitClient := newExecutionEngineFakeClient("acct")
	limitEngine := execution.NewEngine(execution.EngineConfig{Emulator: execution.NewEmulator(execution.EmulatorConfig{})})
	if err := limitEngine.AddClient(limitClient); err != nil {
		return err
	}
	limitOrder := executionEngineSubmit("client-release-limit", model.OrderSideBuy, decimal.RequireFromString("1"))
	limitOrder.Type = model.OrderTypeStopLimit
	limitOrder.Price = decimal.RequireFromString("102")
	limitOrder.TriggerPrice = decimal.RequireFromString("101")
	limitOrder.EmulationTrigger = model.TriggerTypeBidAsk
	if _, err := limitEngine.SubmitOrder(ctx, limitOrder); err != nil {
		return err
	}
	if _, err := readExecutionEngineLifecycle(limitEngine.Events()); err != nil {
		return err
	}
	limitReports, err := limitEngine.ProcessMarketEvent(ctx, model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: limitOrder.InstrumentID,
		BidPrice:     decimal.RequireFromString("100.9"),
		AskPrice:     decimal.RequireFromString("101"),
		BidSize:      decimal.RequireFromString("1"),
		AskSize:      decimal.RequireFromString("1"),
	}})
	if err != nil {
		return err
	}
	if len(limitReports) != 1 || limitReports[0].Type != model.OrderTypeLimit {
		return fmt.Errorf("expected released stop-limit venue report to be limit, got %+v", limitReports)
	}
	limitSubmitted := limitClient.SubmittedOrders()
	if len(limitSubmitted) != 1 || limitSubmitted[0].Type != model.OrderTypeLimit || limitSubmitted[0].Price.String() != "102" || !limitSubmitted[0].TriggerPrice.IsZero() || limitSubmitted[0].EmulationTrigger != model.TriggerTypeNoTrigger {
		return fmt.Errorf("expected stop-limit release to submit limit order, got %+v", limitSubmitted)
	}
	return nil
}

type executionEngineBacktestProbe struct {
	instrumentID model.InstrumentID
	cancelStatus model.OrderStatus
}

func (p *executionEngineBacktestProbe) OnStart(ctx context.Context, rt strategy.Runtime) error {
	report, err := rt.SubmitOrder(ctx, model.SubmitOrder{
		AccountID:     "backtest",
		InstrumentID:  p.instrumentID,
		ClientOrderID: "bt-exec-engine-probe",
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeLimit,
		TimeInForce:   model.TimeInForceGTC,
		Quantity:      decimal.RequireFromString("1"),
		Price:         decimal.RequireFromString("90"),
	})
	if err != nil {
		return err
	}
	if _, err := rt.ModifyOrder(ctx, model.ModifyOrder{
		AccountID:     report.AccountID,
		InstrumentID:  report.InstrumentID,
		OrderID:       report.OrderID,
		ClientOrderID: report.ClientOrderID,
		Quantity:      decimal.RequireFromString("1"),
		Price:         decimal.RequireFromString("89"),
	}); err != nil {
		return err
	}
	if _, err := rt.QueryOrder(ctx, model.QueryOrder{
		AccountID:     report.AccountID,
		InstrumentID:  report.InstrumentID,
		OrderID:       report.OrderID,
		ClientOrderID: report.ClientOrderID,
	}); err != nil {
		return err
	}
	canceled, err := rt.CancelOrder(ctx, model.CancelOrder{
		AccountID:     report.AccountID,
		InstrumentID:  report.InstrumentID,
		OrderID:       report.OrderID,
		ClientOrderID: report.ClientOrderID,
	})
	if err != nil {
		return err
	}
	p.cancelStatus = canceled.Status
	return nil
}

type executionEngineFakeClient struct {
	accountID       model.AccountID
	calls           []string
	submitted       []model.SubmitOrder
	orderReports    []model.OrderStatusReport
	fillReports     []model.FillReport
	positionReports []model.PositionStatusReport
}

type executionEngineFakeAlgorithm struct {
	id     model.ExecAlgorithmID
	orders []model.SubmitOrder
}

func newExecutionEngineFakeClient(accountID model.AccountID) *executionEngineFakeClient {
	return &executionEngineFakeClient{accountID: accountID}
}

func (a *executionEngineFakeAlgorithm) ID() model.ExecAlgorithmID { return a.id }

func (a *executionEngineFakeAlgorithm) SubmitOrder(_ context.Context, order model.SubmitOrder) (model.OrderStatusReport, error) {
	a.orders = append(a.orders, order)
	report := executionEngineOrderReport(model.OrderID("algo-"+string(order.ClientOrderID)), order.ClientOrderID, model.OrderStatusEmulated)
	report.Metadata = order.Metadata
	report.AccountID = order.AccountID
	report.InstrumentID = order.InstrumentID
	report.Side = order.Side
	report.Type = order.Type
	report.TimeInForce = order.TimeInForce
	report.Quantity = order.Quantity
	report.LeavesQuantity = order.Quantity
	report.Price = order.Price
	return report, nil
}

func (c *executionEngineFakeClient) Venue() model.Venue         { return "BINANCE" }
func (c *executionEngineFakeClient) AccountID() model.AccountID { return c.accountID }
func (c *executionEngineFakeClient) Connect(context.Context) error {
	c.calls = append(c.calls, "connect")
	return nil
}
func (c *executionEngineFakeClient) Disconnect(context.Context) error {
	c.calls = append(c.calls, "disconnect")
	return nil
}
func (c *executionEngineFakeClient) Health() venue.ExecutionHealth {
	return venue.ExecutionHealth{Connected: true, AccountReady: true}
}
func (c *executionEngineFakeClient) QueryAccount(context.Context) (model.AccountSnapshot, error) {
	c.calls = append(c.calls, "query-account")
	return model.AccountSnapshot{AccountID: c.accountID, Venue: c.Venue()}, nil
}
func (c *executionEngineFakeClient) SubmitOrder(_ context.Context, order model.SubmitOrder) (model.OrderStatusReport, error) {
	c.calls = append(c.calls, "submit:"+string(order.ClientOrderID))
	c.submitted = append(c.submitted, order)
	report := executionEngineOrderReport(model.OrderID("accepted-"+string(order.ClientOrderID)), order.ClientOrderID, model.OrderStatusAccepted)
	report.AccountID = order.AccountID
	report.InstrumentID = order.InstrumentID
	report.Side = order.Side
	report.Type = order.Type
	report.TimeInForce = order.TimeInForce
	report.Quantity = order.Quantity
	report.LeavesQuantity = order.Quantity
	report.Price = order.Price
	return report, nil
}
func (c *executionEngineFakeClient) SubmittedOrders() []model.SubmitOrder {
	return append([]model.SubmitOrder(nil), c.submitted...)
}
func (c *executionEngineFakeClient) CancelOrder(_ context.Context, cancel model.CancelOrder) (model.OrderStatusReport, error) {
	c.calls = append(c.calls, "cancel:"+string(cancel.ClientOrderID))
	orderID := cancel.OrderID
	if orderID == "" {
		orderID = model.OrderID("accepted-" + string(cancel.ClientOrderID))
	}
	report := executionEngineOrderReport(orderID, cancel.ClientOrderID, model.OrderStatusCanceled)
	report.AccountID = cancel.AccountID
	report.InstrumentID = cancel.InstrumentID
	report.LeavesQuantity = decimal.Zero
	return report, nil
}
func (c *executionEngineFakeClient) GenerateOrderStatusReports(_ context.Context, id model.InstrumentID) ([]model.OrderStatusReport, error) {
	c.calls = append(c.calls, "generate-orders:"+id.String())
	return append([]model.OrderStatusReport(nil), c.orderReports...), nil
}
func (c *executionEngineFakeClient) Events() <-chan model.ExecutionEvent { return nil }
func (c *executionEngineFakeClient) ModifyOrder(_ context.Context, modify model.ModifyOrder) (model.OrderStatusReport, error) {
	c.calls = append(c.calls, "modify:"+string(modify.ClientOrderID))
	report := executionEngineOrderReport(modify.OrderID, modify.ClientOrderID, model.OrderStatusAccepted)
	report.AccountID = modify.AccountID
	report.InstrumentID = modify.InstrumentID
	return report, nil
}
func (c *executionEngineFakeClient) QueryOrder(_ context.Context, query model.QueryOrder) (model.OrderStatusReport, error) {
	c.calls = append(c.calls, "query:"+string(query.ClientOrderID))
	report := executionEngineOrderReport(model.OrderID("query-"+string(query.ClientOrderID)), query.ClientOrderID, model.OrderStatusAccepted)
	report.AccountID = query.AccountID
	report.InstrumentID = query.InstrumentID
	return report, nil
}
func (c *executionEngineFakeClient) Calls() []string {
	return append([]string(nil), c.calls...)
}

type executionEngineCancelAllFakeClient struct {
	*executionEngineFakeClient
}

func (c *executionEngineCancelAllFakeClient) CancelAllOrders(_ context.Context, cancelAll model.CancelAllOrders) ([]model.OrderStatusReport, error) {
	c.calls = append(c.calls, "cancel-all:"+cancelAll.InstrumentID.String())
	return []model.OrderStatusReport{{
		AccountID:     cancelAll.AccountID,
		InstrumentID:  cancelAll.InstrumentID,
		OrderID:       "venue-cancel-all",
		ClientOrderID: "venue-cancel-all",
		Status:        model.OrderStatusCanceled,
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeLimit,
		Quantity:      decimal.RequireFromString("1"),
		TimeInForce:   model.TimeInForceGTC,
	}}, nil
}

func (c *executionEngineFakeClient) GenerateFillReports(_ context.Context, id model.InstrumentID) ([]model.FillReport, error) {
	c.calls = append(c.calls, "generate-fills:"+id.String())
	return append([]model.FillReport(nil), c.fillReports...), nil
}
func (c *executionEngineFakeClient) GeneratePositionStatusReports(_ context.Context, id model.InstrumentID) ([]model.PositionStatusReport, error) {
	c.calls = append(c.calls, "generate-positions:"+id.String())
	return append([]model.PositionStatusReport(nil), c.positionReports...), nil
}
func (c *executionEngineFakeClient) HasCall(want string) bool {
	for _, call := range c.calls {
		if call == want {
			return true
		}
	}
	return false
}

func countExecutionEngineCalls(calls []string, want string) int {
	count := 0
	for _, call := range calls {
		if call == want {
			count++
		}
	}
	return count
}

func readExecutionEngineLifecycle(events <-chan model.ExecutionEvent) (model.OrderLifecycleEvent, error) {
	select {
	case event := <-events:
		if event.Lifecycle == nil {
			return model.OrderLifecycleEvent{}, fmt.Errorf("expected lifecycle event, got %+v", event)
		}
		return *event.Lifecycle, nil
	default:
		return model.OrderLifecycleEvent{}, fmt.Errorf("expected lifecycle event")
	}
}

func requireNoExecutionEngineEvent(events <-chan model.ExecutionEvent) error {
	select {
	case event := <-events:
		return fmt.Errorf("unexpected execution event: %+v", event)
	default:
		return nil
	}
}

func requireExecutionEngineLifecycle(event model.OrderLifecycleEvent, clientOrderID model.ClientOrderID, kind model.OrderEventKind, previous model.OrderStatus, status model.OrderStatus) error {
	if event.ClientOrderID != clientOrderID {
		return fmt.Errorf("expected lifecycle for %s, got %s", clientOrderID, event.ClientOrderID)
	}
	if event.Kind != kind {
		return fmt.Errorf("expected lifecycle kind %s, got %s", kind, event.Kind)
	}
	if event.PreviousStatus != previous {
		return fmt.Errorf("expected previous status %s, got %s", previous, event.PreviousStatus)
	}
	if event.Status != status {
		return fmt.Errorf("expected lifecycle status %s, got %s", status, event.Status)
	}
	if err := event.Validate(); err != nil {
		return err
	}
	return nil
}

func readExecutionEngineBusLifecycle(ctx context.Context, events <-chan bus.Envelope, kind model.OrderEventKind) (model.OrderLifecycleEvent, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	for {
		select {
		case env := <-events:
			event, ok := env.Message.(model.ExecutionEvent)
			if !ok || event.Lifecycle == nil || event.Lifecycle.Kind != kind {
				continue
			}
			return *event.Lifecycle, nil
		case <-ctx.Done():
			return model.OrderLifecycleEvent{}, fmt.Errorf("timed out waiting for lifecycle event %s", kind)
		}
	}
}

func waitExecutionEngineCondition(ctx context.Context, timeout time.Duration, ok func() bool) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if ok() {
			return nil
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for execution-engine condition")
		}
	}
}

func executionEngineSubmit(clientOrderID model.ClientOrderID, side model.OrderSide, quantity decimal.Decimal) model.SubmitOrder {
	return model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  executionEngineInstrumentID(),
		ClientOrderID: clientOrderID,
		Side:          side,
		Type:          model.OrderTypeLimit,
		TimeInForce:   model.TimeInForceGTC,
		Quantity:      quantity,
		Price:         decimal.RequireFromString("100"),
	}
}

func executionEngineOrderReport(orderID model.OrderID, clientOrderID model.ClientOrderID, status model.OrderStatus) model.OrderStatusReport {
	return model.OrderStatusReport{
		AccountID:       "acct",
		InstrumentID:    executionEngineInstrumentID(),
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
		LastUpdatedTime: time.Unix(1, 0),
	}
}

func executionEngineFill(tradeID model.TradeID, orderID model.OrderID, clientOrderID model.ClientOrderID, quantity string, price string) model.FillReport {
	return model.FillReport{
		AccountID:     "acct",
		InstrumentID:  executionEngineInstrumentID(),
		OrderID:       orderID,
		ClientOrderID: clientOrderID,
		TradeID:       tradeID,
		Side:          model.OrderSideBuy,
		Price:         decimal.RequireFromString(price),
		Quantity:      decimal.RequireFromString(quantity),
		Timestamp:     time.Unix(2, 0),
	}
}

func executionEngineBracketList() model.OrderList {
	list := model.OrderList{
		ID: "bracket-list",
		Orders: []model.SubmitOrder{
			executionEngineSubmit("entry", model.OrderSideBuy, decimal.RequireFromString("1")),
			executionEngineSubmit("stop", model.OrderSideSell, decimal.RequireFromString("1")),
			executionEngineSubmit("target", model.OrderSideSell, decimal.RequireFromString("1")),
		},
	}
	list.Orders[0].OrderListID = list.ID
	list.Orders[0].Contingency = model.ContingencyTypeOTO
	list.Orders[1].OrderListID = list.ID
	list.Orders[1].ParentClientOrderID = "entry"
	list.Orders[1].Contingency = model.ContingencyTypeOCO
	list.Orders[2].OrderListID = list.ID
	list.Orders[2].ParentClientOrderID = "entry"
	list.Orders[2].Contingency = model.ContingencyTypeOCO
	return list
}

func executionEngineBulkList(listID model.OrderListID, clientOrderIDs ...model.ClientOrderID) model.OrderList {
	list := model.OrderList{ID: listID}
	for _, clientOrderID := range clientOrderIDs {
		order := executionEngineSubmit(clientOrderID, model.OrderSideBuy, decimal.RequireFromString("1"))
		order.OrderListID = listID
		list.Orders = append(list.Orders, order)
	}
	return list
}

func executionEnginePosition(positionID model.PositionID) model.PositionStatusReport {
	return model.PositionStatusReport{
		AccountID:    "acct",
		InstrumentID: executionEngineInstrumentID(),
		PositionID:   positionID,
		Side:         model.PositionSideLong,
		Quantity:     decimal.RequireFromString("1"),
		EntryPrice:   decimal.RequireFromString("100"),
		Timestamp:    time.Unix(3, 0),
	}
}

func executionEngineInstrumentID() model.InstrumentID {
	return model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
}

func executionEngineTriggerInstrumentID() model.InstrumentID {
	return model.MustInstrumentID("ETH-USDT-SPOT.BINANCE")
}

func executionEngineSyntheticInstrumentID() model.InstrumentID {
	return model.MustInstrumentID("BTC-ETH-SPREAD.SYNTH")
}

func executionEngineInstrument(priceTick string) model.Instrument {
	return model.Instrument{
		ID:        executionEngineInstrumentID(),
		RawSymbol: "BTCUSDT",
		Type:      model.InstrumentTypeSpot,
		Base:      "BTC",
		Quote:     "USDT",
		PriceTick: decimal.RequireFromString(priceTick),
		SizeTick:  decimal.RequireFromString("1"),
		Status:    model.InstrumentStatusTrading,
	}
}

var _ venue.ExecutionClient = (*executionEngineFakeClient)(nil)
var _ venue.OrderModifier = (*executionEngineFakeClient)(nil)
var _ venue.OrderQuerier = (*executionEngineFakeClient)(nil)
var _ venue.FillReportGenerator = (*executionEngineFakeClient)(nil)
var _ venue.PositionStatusReportGenerator = (*executionEngineFakeClient)(nil)
