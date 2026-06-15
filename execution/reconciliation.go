package execution

import (
	"sync"
	"time"

	"github.com/QuantProcessing/exchanges/cache"
	"github.com/QuantProcessing/exchanges/model"
)

const ReconciliationCaseMassStatus = "TC-REC01"
const ReconciliationCaseMissingFills = "TC-REC02"
const ReconciliationCaseOrderDiscrepancy = "TC-REC03"
const ReconciliationCaseFillBeforeOrder = "TC-REC05"
const ReconciliationCaseExternalOrders = "TC-REC08"

type ReconciliationConfig struct {
	Cache   *cache.Cache
	Manager *Manager
}

type ExternalOrderPolicy struct {
	AllowImport bool
	StrategyID  model.StrategyID
}

type ReconciliationResult struct {
	CaseID            string
	AccountID         model.AccountID
	InstrumentID      model.InstrumentID
	StartedAt         time.Time
	CompletedAt       time.Time
	AccountsApplied   int
	OrdersApplied     int
	FillsApplied      int
	FillsDeferred     int
	PositionsApplied  int
	ReportsScanned    int
	DuplicatesSkipped int
	LookbackSkipped   int
	Unresolved        []ReconciliationDiscrepancy
}

type ReconciliationDiscrepancy struct {
	Kind         string
	AccountID    model.AccountID
	InstrumentID model.InstrumentID
	OrderID      model.OrderID
	PositionID   model.PositionID
	TradeID      model.TradeID
	Reason       string
}

type ReconciliationAudit struct {
	LastResult      ReconciliationResult
	LastSuccess     ReconciliationResult
	LastError       string
	LastErrorResult ReconciliationResult
	Unresolved      []ReconciliationDiscrepancy
	History         []ReconciliationResult
}

type Reconciler struct {
	mu              sync.Mutex
	cache           *cache.Cache
	manager         *Manager
	last            ReconciliationResult
	lastSuccess     ReconciliationResult
	lastError       string
	lastErrorResult ReconciliationResult
	unresolved      []ReconciliationDiscrepancy
	history         []ReconciliationResult
}

func NewReconciler(cfg ReconciliationConfig) *Reconciler {
	c := cfg.Cache
	if c == nil && cfg.Manager != nil {
		c = cfg.Manager.cache
	}
	if c == nil {
		c = cache.New()
	}
	manager := cfg.Manager
	if manager == nil {
		manager = NewManager(Config{Cache: c})
	}
	return &Reconciler{
		cache:   c,
		manager: manager,
	}
}

func (r *Reconciler) ReconcileMassStatus(status model.ExecutionMassStatus) (ReconciliationResult, error) {
	if err := status.Validate(); err != nil {
		return ReconciliationResult{}, err
	}
	if r == nil {
		r = NewReconciler(ReconciliationConfig{})
	}
	started := time.Now()
	result := ReconciliationResult{
		CaseID:       ReconciliationCaseMassStatus,
		AccountID:    status.AccountID,
		InstrumentID: firstMassStatusInstrument(status),
		StartedAt:    started,
	}
	for _, account := range status.Accounts {
		r.cache.PutAccount(account)
		result.AccountsApplied++
	}
	for _, order := range status.Orders {
		if err := r.manager.ApplyOrderReport(order); err != nil {
			result.CompletedAt = time.Now()
			r.remember(result, err)
			return result, err
		}
		result.OrdersApplied++
	}
	for _, fill := range status.Fills {
		if r.deferredFillExists(fill) {
			result.DuplicatesSkipped++
			continue
		}
		stored, err := r.manager.ApplyFill(fill)
		if err != nil {
			result.CompletedAt = time.Now()
			r.remember(result, err)
			return result, err
		}
		if stored {
			result.FillsApplied++
		} else if r.deferredFillExists(fill) {
			result.FillsDeferred++
		}
	}
	for _, position := range status.Positions {
		if err := r.cache.PutPosition(position); err != nil {
			result.CompletedAt = time.Now()
			r.remember(result, err)
			return result, err
		}
		result.PositionsApplied++
	}
	result.CompletedAt = time.Now()
	r.remember(result, nil)
	return result, nil
}

func (r *Reconciler) ReconcileMissingFills(reports []model.FillReport, lookbackStart time.Time) (ReconciliationResult, error) {
	if r == nil {
		r = NewReconciler(ReconciliationConfig{})
	}
	result := ReconciliationResult{
		CaseID:    ReconciliationCaseMissingFills,
		StartedAt: time.Now(),
	}
	for _, fill := range reports {
		if err := fill.Validate(); err != nil {
			result.CompletedAt = time.Now()
			r.remember(result, err)
			return result, err
		}
		if result.AccountID == "" {
			result.AccountID = fill.AccountID
		}
		if result.InstrumentID == (model.InstrumentID{}) {
			result.InstrumentID = fill.InstrumentID
		}
		result.ReportsScanned++
		if !lookbackStart.IsZero() && fill.Timestamp.Before(lookbackStart) {
			result.LookbackSkipped++
			continue
		}
		if _, ok := r.cache.FillByTradeID(fill.AccountID, fill.TradeID); ok {
			result.DuplicatesSkipped++
			continue
		}
		if r.deferredFillExists(fill) {
			result.DuplicatesSkipped++
			continue
		}
		stored, err := r.manager.ApplyFill(fill)
		if err != nil {
			result.CompletedAt = time.Now()
			r.remember(result, err)
			return result, err
		}
		if stored {
			result.FillsApplied++
		} else if r.deferredFillExists(fill) {
			result.FillsDeferred++
		}
	}
	result.CompletedAt = time.Now()
	r.remember(result, nil)
	return result, nil
}

func (r *Reconciler) DetectOrderDiscrepancies(reports []model.OrderStatusReport) (ReconciliationResult, error) {
	if r == nil {
		r = NewReconciler(ReconciliationConfig{})
	}
	result := ReconciliationResult{
		CaseID:    ReconciliationCaseOrderDiscrepancy,
		StartedAt: time.Now(),
	}
	for _, report := range reports {
		if err := report.Validate(); err != nil {
			result.CompletedAt = time.Now()
			r.remember(result, err)
			return result, err
		}
		if result.AccountID == "" {
			result.AccountID = report.AccountID
		}
		if result.InstrumentID == (model.InstrumentID{}) {
			result.InstrumentID = report.InstrumentID
		}
		result.ReportsScanned++
		local, ok := r.localOrder(report)
		if !ok {
			continue
		}
		if local.Status.IsOpen() != report.Status.IsOpen() {
			result.Unresolved = append(result.Unresolved, ReconciliationDiscrepancy{
				Kind:         "order_open_state_mismatch",
				AccountID:    report.AccountID,
				InstrumentID: report.InstrumentID,
				OrderID:      report.OrderID,
				Reason:       string(local.Status) + " != " + string(report.Status),
			})
		}
		if !local.FilledQuantity.Equal(report.FilledQuantity) {
			result.Unresolved = append(result.Unresolved, ReconciliationDiscrepancy{
				Kind:         "order_filled_quantity_mismatch",
				AccountID:    report.AccountID,
				InstrumentID: report.InstrumentID,
				OrderID:      report.OrderID,
				Reason:       local.FilledQuantity.String() + " != " + report.FilledQuantity.String(),
			})
		}
	}
	result.CompletedAt = time.Now()
	r.remember(result, nil)
	return result, nil
}

func (r *Reconciler) ReconcileExternalOrders(reports []model.OrderStatusReport, policy ExternalOrderPolicy) (ReconciliationResult, error) {
	if r == nil {
		r = NewReconciler(ReconciliationConfig{})
	}
	result := ReconciliationResult{
		CaseID:    ReconciliationCaseExternalOrders,
		StartedAt: time.Now(),
	}
	strategyID := policy.StrategyID
	if strategyID == "" {
		strategyID = "EXTERNAL"
	}
	for _, report := range reports {
		if err := report.Validate(); err != nil {
			result.CompletedAt = time.Now()
			r.remember(result, err)
			return result, err
		}
		if result.AccountID == "" {
			result.AccountID = report.AccountID
		}
		if result.InstrumentID == (model.InstrumentID{}) {
			result.InstrumentID = report.InstrumentID
		}
		result.ReportsScanned++
		if isExternalOrderReport(report) {
			if !policy.AllowImport {
				result.Unresolved = append(result.Unresolved, ReconciliationDiscrepancy{
					Kind:         "external_order_rejected",
					AccountID:    report.AccountID,
					InstrumentID: report.InstrumentID,
					OrderID:      report.OrderID,
					Reason:       "external order import disabled",
				})
				continue
			}
			report.Metadata.StrategyID = strategyID
		}
		if err := r.manager.ApplyOrderReport(report); err != nil {
			result.CompletedAt = time.Now()
			r.remember(result, err)
			return result, err
		}
		result.OrdersApplied++
	}
	result.CompletedAt = time.Now()
	r.remember(result, nil)
	return result, nil
}

func (r *Reconciler) LastResult() ReconciliationResult {
	if r == nil {
		return ReconciliationResult{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.last
}

func (r *Reconciler) AuditTrail() ReconciliationAudit {
	if r == nil {
		return ReconciliationAudit{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return ReconciliationAudit{
		LastResult:      r.last,
		LastSuccess:     r.lastSuccess,
		LastError:       r.lastError,
		LastErrorResult: r.lastErrorResult,
		Unresolved:      append([]ReconciliationDiscrepancy(nil), r.unresolved...),
		History:         append([]ReconciliationResult(nil), r.history...),
	}
}

func (r *Reconciler) remember(result ReconciliationResult, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.last = result
	r.history = append(r.history, result)
	if len(result.Unresolved) > 0 {
		r.unresolved = append([]ReconciliationDiscrepancy(nil), result.Unresolved...)
	}
	if err != nil {
		r.lastError = err.Error()
		r.lastErrorResult = result
		return
	}
	r.lastSuccess = result
}

func (r *Reconciler) localOrder(report model.OrderStatusReport) (model.OrderStatusReport, bool) {
	if report.OrderID != "" {
		if order, ok := r.cache.Order(report.AccountID, report.OrderID); ok {
			return order, true
		}
	}
	if report.ClientOrderID != "" {
		if order, ok := r.cache.OrderByClientID(report.AccountID, report.ClientOrderID); ok {
			return order, true
		}
	}
	if report.VenueOrderID != "" {
		if order, ok := r.cache.OrderByVenueID(report.AccountID, report.VenueOrderID); ok {
			return order, true
		}
	}
	return model.OrderStatusReport{}, false
}

func (r *Reconciler) deferredFillExists(fill model.FillReport) bool {
	for _, deferred := range r.cache.DeferredFillsForOrder(fill.AccountID, fill.OrderID) {
		if deferred.TradeID == fill.TradeID {
			return true
		}
	}
	return false
}

func isExternalOrderReport(report model.OrderStatusReport) bool {
	return report.ClientOrderID == "" && report.Metadata.StrategyID == ""
}

func firstMassStatusInstrument(status model.ExecutionMassStatus) model.InstrumentID {
	if len(status.Orders) > 0 {
		return status.Orders[0].InstrumentID
	}
	if len(status.Fills) > 0 {
		return status.Fills[0].InstrumentID
	}
	if len(status.Positions) > 0 {
		return status.Positions[0].InstrumentID
	}
	return model.InstrumentID{}
}
