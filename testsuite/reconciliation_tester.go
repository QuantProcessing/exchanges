package testsuite

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/QuantProcessing/exchanges/account"
	"github.com/QuantProcessing/exchanges/cache"
	"github.com/QuantProcessing/exchanges/execution"
	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/shopspring/decimal"
)

type ReconciliationTesterConfig struct {
	Cache        *cache.Cache
	AccountID    model.AccountID
	InstrumentID model.InstrumentID
}

type ReconciliationTester struct {
	cfg ReconciliationTesterConfig
}

func NewReconciliationTester(cfg ReconciliationTesterConfig) *ReconciliationTester {
	if cfg.AccountID == "" {
		cfg.AccountID = "acct"
	}
	if cfg.InstrumentID == (model.InstrumentID{}) {
		cfg.InstrumentID = executionEngineInstrumentID()
	}
	if cfg.Cache == nil {
		cfg.Cache = cache.New()
	}
	return &ReconciliationTester{cfg: cfg}
}

func (r *ReconciliationTester) Run(ctx context.Context, t *testing.T) ContractReport {
	t.Helper()
	_ = ctx
	return runContractCases(t, "reconciliation", []contractCase{
		{id: "TC-REC01", name: "Reconciler applies mass status reports with audit counters", run: func() error {
			return r.runMassStatusReconciliation()
		}},
		{id: "TC-REC02", name: "Reconciler applies only missing fills inside lookback", run: func() error {
			return r.runMissingFillLookbackDedupe()
		}},
		{id: "TC-REC03", name: "Reconciler detects order state and filled quantity discrepancies", run: func() error {
			return r.runOrderDiscrepancyDetection()
		}},
		{id: "TC-REC04", name: "Engine query falls back to venue-order-id reports", run: func() error {
			return r.runVenueOrderIDQueryFallback(ctx)
		}},
		{id: "TC-REC05", name: "Reconciler defers fill-before-order reports and replays them", run: func() error {
			return r.runFillBeforeOrderReplay()
		}},
		{id: "TC-REC06", name: "TradingAccount skips recent missing open order repair until threshold", run: func() error {
			return r.runMissingOpenOrderRepairThreshold(ctx)
		}},
		{id: "TC-REC07", name: "Reconciler stops position repair after retry limit with unresolved discrepancies", run: func() error {
			return r.runPositionRepairRetryLimit()
		}},
		{id: "TC-REC08", name: "Reconciler imports or explicitly rejects external orders", run: func() error {
			return r.runExternalOrderImportOrReject()
		}},
		{id: "TC-REC09", name: "Reconciler audit trail tracks success error and unresolved state", run: func() error {
			return r.runAuditTrail()
		}},
	})
}

func (r *ReconciliationTester) runMassStatusReconciliation() error {
	reconciler := execution.NewReconciler(execution.ReconciliationConfig{Cache: r.cfg.Cache})
	order := executionEngineOrderReport("rec-order-1", "rec-client-1", model.OrderStatusAccepted)
	order.AccountID = r.cfg.AccountID
	order.InstrumentID = r.cfg.InstrumentID
	order.Quantity = decimal.RequireFromString("2")
	order.LeavesQuantity = decimal.RequireFromString("2")
	fill := executionEngineFill("rec-trade-1", order.OrderID, order.ClientOrderID, "0.5", "100")
	fill.AccountID = r.cfg.AccountID
	fill.InstrumentID = r.cfg.InstrumentID
	fill.PositionID = "rec-position-1"
	position := executionEnginePosition("rec-position-1")
	position.AccountID = r.cfg.AccountID
	position.InstrumentID = r.cfg.InstrumentID
	position.Quantity = decimal.RequireFromString("0.5")
	account := model.AccountSnapshot{
		AccountID: r.cfg.AccountID,
		Venue:     "BINANCE",
		Balances: []model.Balance{{
			Currency: "USDT",
			Free:     "999.99",
			Locked:   "0.01",
			Total:    "1000",
		}},
	}
	result, err := reconciler.ReconcileMassStatus(model.ExecutionMassStatus{
		AccountID: r.cfg.AccountID,
		Venue:     "BINANCE",
		Accounts:  []model.AccountSnapshot{account},
		Orders:    []model.OrderStatusReport{order},
		Fills:     []model.FillReport{fill},
		Positions: []model.PositionStatusReport{position},
		Timestamp: time.Unix(11, 0),
	})
	if err != nil {
		return err
	}
	if result.CaseID != execution.ReconciliationCaseMassStatus ||
		result.AccountsApplied != 1 ||
		result.OrdersApplied != 1 ||
		result.FillsApplied != 1 ||
		result.PositionsApplied != 1 ||
		len(result.Unresolved) != 0 {
		return fmt.Errorf("unexpected reconciliation audit: %+v", result)
	}
	if got, ok := r.cfg.Cache.Order(r.cfg.AccountID, order.OrderID); !ok || got.Status != model.OrderStatusPartiallyFilled {
		return fmt.Errorf("reconciled order not partially filled: ok=%v order=%+v", ok, got)
	}
	if _, ok := r.cfg.Cache.FillByTradeID(r.cfg.AccountID, fill.TradeID); !ok {
		return fmt.Errorf("reconciled fill not cached")
	}
	if _, ok := r.cfg.Cache.Position(r.cfg.AccountID, position.PositionID); !ok {
		return fmt.Errorf("reconciled position not cached")
	}
	if _, ok := r.cfg.Cache.Account(r.cfg.AccountID); !ok {
		return fmt.Errorf("reconciled account not cached")
	}
	return nil
}

func (r *ReconciliationTester) runVenueOrderIDQueryFallback(ctx context.Context) error {
	client := &reconciliationReportOnlyClient{accountID: r.cfg.AccountID}
	report := executionEngineOrderReport("rec-venue-query-order", "", model.OrderStatusAccepted)
	report.AccountID = r.cfg.AccountID
	report.InstrumentID = r.cfg.InstrumentID
	report.VenueOrderID = "rec-venue-only-1"
	client.orderReports = []model.OrderStatusReport{report}
	c := cache.New()
	engine := execution.NewEngine(execution.EngineConfig{Cache: c})
	if err := engine.AddClient(client); err != nil {
		return err
	}
	got, err := engine.QueryOrder(ctx, model.QueryOrder{
		AccountID:    r.cfg.AccountID,
		InstrumentID: r.cfg.InstrumentID,
		VenueOrderID: report.VenueOrderID,
	})
	if err != nil {
		return err
	}
	if got.OrderID != report.OrderID || got.VenueOrderID != report.VenueOrderID {
		return fmt.Errorf("unexpected fallback query report: %+v", got)
	}
	if _, ok := c.OrderByVenueID(r.cfg.AccountID, report.VenueOrderID); !ok {
		return fmt.Errorf("venue-order-id fallback report was not cached")
	}
	if countExecutionEngineCalls(client.calls, "generate-orders:"+r.cfg.InstrumentID.String()) != 1 {
		return fmt.Errorf("expected one report generation fallback, got %v", client.calls)
	}
	if _, err := engine.QueryOrder(ctx, model.QueryOrder{
		AccountID:    r.cfg.AccountID,
		InstrumentID: r.cfg.InstrumentID,
		VenueOrderID: report.VenueOrderID,
	}); err != nil {
		return err
	}
	if countExecutionEngineCalls(client.calls, "generate-orders:"+r.cfg.InstrumentID.String()) != 1 {
		return fmt.Errorf("expected cached venue-order-id query to avoid second fallback, got %v", client.calls)
	}
	return nil
}

type reconciliationReportOnlyClient struct {
	accountID    model.AccountID
	calls        []string
	orderReports []model.OrderStatusReport
}

func (c *reconciliationReportOnlyClient) Venue() model.Venue         { return "BINANCE" }
func (c *reconciliationReportOnlyClient) AccountID() model.AccountID { return c.accountID }
func (c *reconciliationReportOnlyClient) Connect(context.Context) error {
	c.calls = append(c.calls, "connect")
	return nil
}
func (c *reconciliationReportOnlyClient) Disconnect(context.Context) error {
	c.calls = append(c.calls, "disconnect")
	return nil
}
func (c *reconciliationReportOnlyClient) Health() venue.ExecutionHealth {
	return venue.ExecutionHealth{Connected: true, AccountReady: true}
}
func (c *reconciliationReportOnlyClient) QueryAccount(context.Context) (model.AccountSnapshot, error) {
	c.calls = append(c.calls, "query-account")
	return model.AccountSnapshot{AccountID: c.accountID, Venue: c.Venue()}, nil
}
func (c *reconciliationReportOnlyClient) SubmitOrder(_ context.Context, order model.SubmitOrder) (model.OrderStatusReport, error) {
	c.calls = append(c.calls, "submit:"+string(order.ClientOrderID))
	report := executionEngineOrderReport("accepted-"+model.OrderID(order.ClientOrderID), order.ClientOrderID, model.OrderStatusAccepted)
	report.AccountID = order.AccountID
	report.InstrumentID = order.InstrumentID
	return report, nil
}
func (c *reconciliationReportOnlyClient) CancelOrder(_ context.Context, cancel model.CancelOrder) (model.OrderStatusReport, error) {
	c.calls = append(c.calls, "cancel:"+string(cancel.ClientOrderID))
	report := executionEngineOrderReport(cancel.OrderID, cancel.ClientOrderID, model.OrderStatusCanceled)
	report.AccountID = cancel.AccountID
	report.InstrumentID = cancel.InstrumentID
	return report, nil
}
func (c *reconciliationReportOnlyClient) GenerateOrderStatusReports(_ context.Context, instrumentID model.InstrumentID) ([]model.OrderStatusReport, error) {
	c.calls = append(c.calls, "generate-orders:"+instrumentID.String())
	return append([]model.OrderStatusReport(nil), c.orderReports...), nil
}
func (c *reconciliationReportOnlyClient) Events() <-chan model.ExecutionEvent { return nil }

func (r *ReconciliationTester) runOrderDiscrepancyDetection() error {
	c := cache.New()
	reconciler := execution.NewReconciler(execution.ReconciliationConfig{Cache: c})
	local := executionEngineOrderReport("rec-order-3", "rec-client-3", model.OrderStatusAccepted)
	local.AccountID = r.cfg.AccountID
	local.InstrumentID = r.cfg.InstrumentID
	local.Quantity = decimal.RequireFromString("2")
	local.FilledQuantity = decimal.RequireFromString("0.25")
	local.LeavesQuantity = decimal.RequireFromString("1.75")
	if err := c.PutOrder(local); err != nil {
		return err
	}
	venue := local
	venue.Status = model.OrderStatusFilled
	venue.FilledQuantity = decimal.RequireFromString("2")
	venue.LeavesQuantity = decimal.Zero
	result, err := reconciler.DetectOrderDiscrepancies([]model.OrderStatusReport{venue})
	if err != nil {
		return err
	}
	if result.CaseID != execution.ReconciliationCaseOrderDiscrepancy ||
		result.ReportsScanned != 1 ||
		len(result.Unresolved) != 2 ||
		result.Unresolved[0].Kind != "order_open_state_mismatch" ||
		result.Unresolved[1].Kind != "order_filled_quantity_mismatch" {
		return fmt.Errorf("unexpected order discrepancy audit: %+v", result)
	}
	cached, ok := c.Order(r.cfg.AccountID, local.OrderID)
	if !ok || cached.Status != model.OrderStatusAccepted || !cached.FilledQuantity.Equal(decimal.RequireFromString("0.25")) {
		return fmt.Errorf("discrepancy detection mutated local order: ok=%v order=%+v", ok, cached)
	}
	return nil
}

func (r *ReconciliationTester) runFillBeforeOrderReplay() error {
	c := cache.New()
	reconciler := execution.NewReconciler(execution.ReconciliationConfig{Cache: c})
	fill := executionEngineFill("rec-trade-before-order", "rec-order-5", "rec-client-5", "0.4", "100")
	fill.AccountID = r.cfg.AccountID
	fill.InstrumentID = r.cfg.InstrumentID
	fill.Timestamp = time.Unix(120, 0)
	missing, err := reconciler.ReconcileMissingFills([]model.FillReport{fill}, time.Unix(100, 0))
	if err != nil {
		return err
	}
	if missing.CaseID != execution.ReconciliationCaseMissingFills ||
		missing.ReportsScanned != 1 ||
		missing.FillsApplied != 0 ||
		missing.FillsDeferred != 1 ||
		len(missing.Unresolved) != 0 {
		return fmt.Errorf("unexpected fill-before-order missing-fill audit: %+v", missing)
	}
	if deferred := c.DeferredFillsForOrder(r.cfg.AccountID, fill.OrderID); len(deferred) != 1 {
		return fmt.Errorf("expected one deferred fill, got %+v", deferred)
	}
	if _, ok := c.FillByTradeID(r.cfg.AccountID, fill.TradeID); ok {
		return fmt.Errorf("fill was cached before matching order report")
	}
	order := executionEngineOrderReport(fill.OrderID, fill.ClientOrderID, model.OrderStatusAccepted)
	order.AccountID = r.cfg.AccountID
	order.InstrumentID = r.cfg.InstrumentID
	order.Quantity = decimal.RequireFromString("1")
	order.LeavesQuantity = decimal.RequireFromString("1")
	massStatus, err := reconciler.ReconcileMassStatus(model.ExecutionMassStatus{
		AccountID: r.cfg.AccountID,
		Venue:     "BINANCE",
		Orders:    []model.OrderStatusReport{order},
		Timestamp: time.Unix(121, 0),
	})
	if err != nil {
		return err
	}
	if massStatus.CaseID != execution.ReconciliationCaseMassStatus || massStatus.OrdersApplied != 1 || len(massStatus.Unresolved) != 0 {
		return fmt.Errorf("unexpected fill-before-order mass-status audit: %+v", massStatus)
	}
	if deferred := c.DeferredFillsForOrder(r.cfg.AccountID, fill.OrderID); len(deferred) != 0 {
		return fmt.Errorf("deferred fill was not cleared: %+v", deferred)
	}
	if _, ok := c.FillByTradeID(r.cfg.AccountID, fill.TradeID); !ok {
		return fmt.Errorf("deferred fill was not replayed")
	}
	cached, ok := c.Order(r.cfg.AccountID, fill.OrderID)
	if !ok ||
		cached.Status != model.OrderStatusPartiallyFilled ||
		!cached.FilledQuantity.Equal(decimal.RequireFromString("0.4")) ||
		!cached.LeavesQuantity.Equal(decimal.RequireFromString("0.6")) {
		return fmt.Errorf("deferred fill did not update order: ok=%v order=%+v", ok, cached)
	}
	return nil
}

func (r *ReconciliationTester) runMissingOpenOrderRepairThreshold(ctx context.Context) error {
	c := cache.New()
	now := time.Now()
	recent := executionEngineOrderReport("rec-recent-order", "rec-recent-client", model.OrderStatusAccepted)
	recent.AccountID = r.cfg.AccountID
	recent.InstrumentID = r.cfg.InstrumentID
	recent.Quantity = decimal.RequireFromString("1")
	recent.LeavesQuantity = decimal.RequireFromString("1")
	recent.LastUpdatedTime = now.Add(-10 * time.Second)
	stale := recent
	stale.OrderID = "rec-stale-order"
	stale.ClientOrderID = "rec-stale-client"
	stale.LastUpdatedTime = now.Add(-2 * time.Hour)
	if err := c.PutOrder(recent); err != nil {
		return err
	}
	if err := c.PutOrder(stale); err != nil {
		return err
	}
	client := &reconciliationReportOnlyClient{accountID: r.cfg.AccountID}
	acct, err := account.NewTradingAccount(client, account.TradingAccountConfig{
		Cache:                   c,
		Instruments:             []model.InstrumentID{r.cfg.InstrumentID},
		ReconcileInterval:       10 * time.Millisecond,
		MissingOrderRepairDelay: time.Hour,
	})
	if err != nil {
		return err
	}
	if err := acct.Start(ctx); err != nil {
		return err
	}
	defer acct.Stop(context.Background())
	deadline := time.Now().Add(500 * time.Millisecond)
	for {
		cached, ok := c.Order(r.cfg.AccountID, stale.OrderID)
		if ok && cached.Status == model.OrderStatusCanceled {
			break
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("stale missing open order was not repaired before deadline")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cachedRecent, ok := c.Order(r.cfg.AccountID, recent.OrderID)
	if !ok || cachedRecent.Status != model.OrderStatusAccepted {
		return fmt.Errorf("recent missing open order was repaired too early: ok=%v order=%+v", ok, cachedRecent)
	}
	return nil
}

func (r *ReconciliationTester) runPositionRepairRetryLimit() error {
	c := cache.New()
	reconciler := account.NewReconciler(c)
	missing := executionEnginePosition("rec-missing-position")
	missing.AccountID = r.cfg.AccountID
	missing.InstrumentID = r.cfg.InstrumentID
	missing.Quantity = decimal.RequireFromString("1")
	stale := executionEnginePosition("rec-stale-position")
	stale.AccountID = r.cfg.AccountID
	stale.InstrumentID = r.cfg.InstrumentID
	stale.Quantity = decimal.RequireFromString("2")
	venueStale := stale
	venueStale.Quantity = decimal.RequireFromString("1")
	if err := reconciler.Apply(model.ExecutionEvent{Position: &missing}); err != nil {
		return err
	}
	if err := reconciler.Apply(model.ExecutionEvent{Position: &stale}); err != nil {
		return err
	}
	policy := account.PositionRepairPolicy{MaxAttempts: 2}
	first, err := reconciler.RepairPositionReports(r.cfg.AccountID, r.cfg.InstrumentID, []model.PositionStatusReport{venueStale}, policy)
	if err != nil {
		return err
	}
	if len(first.Generated) != 2 || len(first.Unresolved) != 0 {
		return fmt.Errorf("unexpected first position repair result: %+v", first)
	}
	second, err := reconciler.RepairPositionReports(r.cfg.AccountID, r.cfg.InstrumentID, []model.PositionStatusReport{venueStale}, policy)
	if err != nil {
		return err
	}
	if len(second.Generated) != 2 || len(second.Unresolved) != 0 {
		return fmt.Errorf("unexpected second position repair result: %+v", second)
	}
	third, err := reconciler.RepairPositionReports(r.cfg.AccountID, r.cfg.InstrumentID, []model.PositionStatusReport{venueStale}, policy)
	if err != nil {
		return err
	}
	if len(third.Generated) != 0 || len(third.Unresolved) != 2 {
		return fmt.Errorf("unexpected exhausted position repair result: %+v", third)
	}
	if !hasPositionDiscrepancy(third.Unresolved, "rec-missing-position", "position_missing_from_venue", 2) {
		return fmt.Errorf("missing unresolved missing-position discrepancy: %+v", third.Unresolved)
	}
	if !hasPositionDiscrepancy(third.Unresolved, "rec-stale-position", "position_quantity_mismatch", 2) {
		return fmt.Errorf("missing unresolved stale-position discrepancy: %+v", third.Unresolved)
	}
	return nil
}

func (r *ReconciliationTester) runExternalOrderImportOrReject() error {
	external := executionEngineOrderReport("rec-external-order", "", model.OrderStatusAccepted)
	external.AccountID = r.cfg.AccountID
	external.InstrumentID = r.cfg.InstrumentID
	external.ClientOrderID = ""
	external.Metadata = model.CommandMetadata{}
	importCache := cache.New()
	importReconciler := execution.NewReconciler(execution.ReconciliationConfig{Cache: importCache})
	imported, err := importReconciler.ReconcileExternalOrders([]model.OrderStatusReport{external}, execution.ExternalOrderPolicy{
		AllowImport: true,
		StrategyID:  "EXTERNAL",
	})
	if err != nil {
		return err
	}
	if imported.CaseID != execution.ReconciliationCaseExternalOrders ||
		imported.ReportsScanned != 1 ||
		imported.OrdersApplied != 1 ||
		len(imported.Unresolved) != 0 {
		return fmt.Errorf("unexpected external import audit: %+v", imported)
	}
	cached, ok := importCache.Order(r.cfg.AccountID, external.OrderID)
	if !ok || cached.Metadata.StrategyID != "EXTERNAL" {
		return fmt.Errorf("external order was not imported under EXTERNAL strategy: ok=%v order=%+v", ok, cached)
	}
	if orders := importCache.OrdersByStrategy(r.cfg.AccountID, "EXTERNAL"); len(orders) != 1 {
		return fmt.Errorf("external order was not strategy-indexed: %+v", orders)
	}
	rejectCache := cache.New()
	rejectReconciler := execution.NewReconciler(execution.ReconciliationConfig{Cache: rejectCache})
	rejected, err := rejectReconciler.ReconcileExternalOrders([]model.OrderStatusReport{external}, execution.ExternalOrderPolicy{})
	if err != nil {
		return err
	}
	if rejected.CaseID != execution.ReconciliationCaseExternalOrders ||
		rejected.ReportsScanned != 1 ||
		rejected.OrdersApplied != 0 ||
		len(rejected.Unresolved) != 1 ||
		rejected.Unresolved[0].Kind != "external_order_rejected" {
		return fmt.Errorf("unexpected external rejection audit: %+v", rejected)
	}
	if _, ok := rejectCache.Order(r.cfg.AccountID, external.OrderID); ok {
		return fmt.Errorf("rejected external order was cached")
	}
	return nil
}

func (r *ReconciliationTester) runAuditTrail() error {
	c := cache.New()
	reconciler := execution.NewReconciler(execution.ReconciliationConfig{Cache: c})
	order := executionEngineOrderReport("rec-audit-order", "rec-audit-client", model.OrderStatusAccepted)
	order.AccountID = r.cfg.AccountID
	order.InstrumentID = r.cfg.InstrumentID
	order.Quantity = decimal.RequireFromString("1")
	order.LeavesQuantity = decimal.RequireFromString("1")
	if _, err := reconciler.ReconcileMassStatus(model.ExecutionMassStatus{
		AccountID: r.cfg.AccountID,
		Venue:     "BINANCE",
		Orders:    []model.OrderStatusReport{order},
		Timestamp: time.Unix(130, 0),
	}); err != nil {
		return err
	}
	venue := order
	venue.Status = model.OrderStatusFilled
	venue.FilledQuantity = decimal.RequireFromString("1")
	venue.LeavesQuantity = decimal.Zero
	if _, err := reconciler.DetectOrderDiscrepancies([]model.OrderStatusReport{venue}); err != nil {
		return err
	}
	invalidFill := model.FillReport{AccountID: r.cfg.AccountID, TradeID: "rec-audit-invalid-fill"}
	if _, err := reconciler.ReconcileMissingFills([]model.FillReport{invalidFill}, time.Time{}); err == nil {
		return fmt.Errorf("expected invalid fill to fail")
	}
	audit := reconciler.AuditTrail()
	if len(audit.History) != 3 ||
		audit.History[0].CaseID != execution.ReconciliationCaseMassStatus ||
		audit.LastSuccess.CaseID != execution.ReconciliationCaseOrderDiscrepancy ||
		audit.LastErrorResult.CaseID != execution.ReconciliationCaseMissingFills ||
		audit.LastError == "" ||
		audit.LastResult.CaseID != execution.ReconciliationCaseMissingFills ||
		len(audit.Unresolved) != 2 ||
		audit.Unresolved[0].Kind != "order_open_state_mismatch" ||
		audit.Unresolved[1].Kind != "order_filled_quantity_mismatch" {
		return fmt.Errorf("unexpected audit trail: %+v", audit)
	}
	return nil
}

func (r *ReconciliationTester) runMissingFillLookbackDedupe() error {
	c := cache.New()
	reconciler := execution.NewReconciler(execution.ReconciliationConfig{Cache: c})
	order := executionEngineOrderReport("rec-order-2", "rec-client-2", model.OrderStatusAccepted)
	order.AccountID = r.cfg.AccountID
	order.InstrumentID = r.cfg.InstrumentID
	order.Quantity = decimal.RequireFromString("3")
	order.LeavesQuantity = decimal.RequireFromString("3")
	if err := c.PutOrder(order); err != nil {
		return err
	}
	existing := executionEngineFill("rec-trade-existing", order.OrderID, order.ClientOrderID, "0.25", "100")
	existing.AccountID = r.cfg.AccountID
	existing.InstrumentID = r.cfg.InstrumentID
	existing.Timestamp = time.Unix(100, 0)
	if stored, err := c.PutFill(existing); err != nil {
		return err
	} else if !stored {
		return fmt.Errorf("expected fixture fill to store")
	}
	old := existing
	old.TradeID = "rec-trade-old"
	old.Quantity = decimal.RequireFromString("0.5")
	old.Timestamp = time.Unix(50, 0)
	missing := existing
	missing.TradeID = "rec-trade-missing"
	missing.Quantity = decimal.RequireFromString("0.75")
	missing.Timestamp = time.Unix(110, 0)
	result, err := reconciler.ReconcileMissingFills([]model.FillReport{existing, old, missing}, time.Unix(90, 0))
	if err != nil {
		return err
	}
	if result.CaseID != execution.ReconciliationCaseMissingFills ||
		result.ReportsScanned != 3 ||
		result.FillsApplied != 1 ||
		result.DuplicatesSkipped != 1 ||
		result.LookbackSkipped != 1 ||
		len(result.Unresolved) != 0 {
		return fmt.Errorf("unexpected missing-fill audit: %+v", result)
	}
	if _, ok := c.FillByTradeID(r.cfg.AccountID, missing.TradeID); !ok {
		return fmt.Errorf("missing fill was not reconciled")
	}
	if _, ok := c.FillByTradeID(r.cfg.AccountID, old.TradeID); ok {
		return fmt.Errorf("old fill outside lookback was reconciled")
	}
	return nil
}

func hasPositionDiscrepancy(discrepancies []account.PositionRepairDiscrepancy, positionID model.PositionID, kind string, attempts int) bool {
	for _, discrepancy := range discrepancies {
		if discrepancy.PositionID == positionID && discrepancy.Kind == kind && discrepancy.Attempts == attempts {
			return true
		}
	}
	return false
}
