package account

import (
	"fmt"
	"sync"
	"time"

	"github.com/QuantProcessing/exchanges/cache"
	"github.com/QuantProcessing/exchanges/model"
	"github.com/shopspring/decimal"
)

type Reconciler struct {
	mu                     sync.Mutex
	cache                  *cache.Cache
	positionRepairAttempts map[positionRepairKey]int
}

type MissingOpenOrderRepairPolicy struct {
	MissingStatus        model.OrderStatus
	RecentActivityWindow time.Duration
	Now                  time.Time
}

type PositionRepairPolicy struct {
	MaxAttempts int
	Now         time.Time
}

type PositionRepairResult struct {
	Generated  []model.PositionStatusReport
	Unresolved []PositionRepairDiscrepancy
}

type PositionRepairDiscrepancy struct {
	Kind         string
	AccountID    model.AccountID
	InstrumentID model.InstrumentID
	PositionID   model.PositionID
	Attempts     int
	Reason       string
}

type positionRepairKey struct {
	accountID    model.AccountID
	instrumentID model.InstrumentID
	positionID   model.PositionID
}

func NewReconciler(c *cache.Cache) *Reconciler {
	if c == nil {
		c = cache.New()
	}
	return &Reconciler{
		cache:                  c,
		positionRepairAttempts: make(map[positionRepairKey]int),
	}
}

func (r *Reconciler) Apply(event model.ExecutionEvent) error {
	if err := event.Validate(); err != nil {
		return err
	}
	if event.Account != nil {
		r.cache.PutAccount(*event.Account)
		return nil
	}
	if event.Order != nil {
		return r.applyOrder(*event.Order)
	}
	if event.Fill != nil {
		return r.applyFill(*event.Fill)
	}
	if event.Position != nil {
		return r.cache.PutPosition(*event.Position)
	}
	if event.PositionLifecycle != nil {
		return nil
	}
	return nil
}

func (r *Reconciler) applyOrder(report model.OrderStatusReport) error {
	if existing, ok := r.cache.Order(report.AccountID, report.OrderID); ok {
		nextStatus, ok := nextStatus(existing.Status, report.Status)
		if !ok {
			return fmt.Errorf("%w: invalid order transition %s -> %s", model.ErrInvalidOrder, existing.Status, report.Status)
		}
		report.Status = nextStatus
		if report.FilledQuantity.LessThan(existing.FilledQuantity) {
			return fmt.Errorf("%w: filled quantity moved backwards", model.ErrInvalidOrder)
		}
	}
	if err := r.cache.PutOrder(report); err != nil {
		return err
	}
	if report.FilledQuantity.IsZero() {
		return r.replayFills(report.AccountID, report.OrderID)
	}
	return nil
}

func (r *Reconciler) ReconcileMissingOpenOrders(accountID model.AccountID, instrumentID model.InstrumentID, reports []model.OrderStatusReport, missingStatus model.OrderStatus) ([]model.OrderStatusReport, error) {
	return r.ReconcileMissingOpenOrdersWithPolicy(accountID, instrumentID, reports, MissingOpenOrderRepairPolicy{
		MissingStatus: missingStatus,
	})
}

func (r *Reconciler) ReconcileMissingOpenOrdersWithPolicy(accountID model.AccountID, instrumentID model.InstrumentID, reports []model.OrderStatusReport, policy MissingOpenOrderRepairPolicy) ([]model.OrderStatusReport, error) {
	if accountID == "" {
		return nil, fmt.Errorf("%w: account id is required", model.ErrInvalidOrder)
	}
	if err := instrumentID.Validate(); err != nil {
		return nil, err
	}
	if !policy.MissingStatus.IsTerminal() {
		return nil, fmt.Errorf("%w: missing order status must be terminal", model.ErrInvalidOrder)
	}
	now := policy.Now
	if now.IsZero() {
		now = time.Now()
	}
	observed := newOrderSnapshotIndex(reports, accountID, instrumentID)
	generated := make([]model.OrderStatusReport, 0)
	for _, order := range r.cache.OpenOrders(accountID) {
		if order.InstrumentID != instrumentID || observed.contains(order) {
			continue
		}
		if orderHasRecentActivity(order, now, policy.RecentActivityWindow) {
			continue
		}
		order.Status = policy.MissingStatus
		order.LastUpdatedTime = now
		if err := r.applyOrder(order); err != nil {
			return generated, err
		}
		generated = append(generated, order)
	}
	return generated, nil
}

func (r *Reconciler) MissingPositionReports(accountID model.AccountID, instrumentID model.InstrumentID, reports []model.PositionStatusReport) ([]model.PositionStatusReport, error) {
	result, err := r.RepairPositionReports(accountID, instrumentID, reports, PositionRepairPolicy{})
	if err != nil {
		return nil, err
	}
	return result.Generated, nil
}

func (r *Reconciler) RepairPositionReports(accountID model.AccountID, instrumentID model.InstrumentID, reports []model.PositionStatusReport, policy PositionRepairPolicy) (PositionRepairResult, error) {
	if accountID == "" {
		return PositionRepairResult{}, fmt.Errorf("%w: account id is required", model.ErrInvalidOrder)
	}
	if err := instrumentID.Validate(); err != nil {
		return PositionRepairResult{}, err
	}
	now := policy.Now
	if now.IsZero() {
		now = time.Now()
	}
	observed := newPositionSnapshotIndex(reports, accountID, instrumentID)
	result := PositionRepairResult{
		Generated:  make([]model.PositionStatusReport, 0),
		Unresolved: make([]PositionRepairDiscrepancy, 0),
	}
	for _, position := range r.cache.PositionsForInstrument(instrumentID) {
		if position.AccountID != accountID {
			continue
		}
		if report, ok := observed.match(position); ok {
			if positionsMatch(position, report) {
				r.clearPositionRepairAttempt(positionRepairKeyFor(position))
				continue
			}
			r.appendPositionRepair(position, report, positionDiscrepancyKind(position, report), policy.MaxAttempts, &result)
			continue
		}
		if position.Side == model.PositionSideFlat || position.Quantity.IsZero() {
			r.clearPositionRepairAttempt(positionRepairKeyFor(position))
			continue
		}
		flat := position
		flat.Side = model.PositionSideFlat
		flat.Quantity = decimal.Zero
		flat.Timestamp = now
		r.appendPositionRepair(position, flat, "position_missing_from_venue", policy.MaxAttempts, &result)
	}
	return result, nil
}

func (r *Reconciler) applyFill(fill model.FillReport) error {
	stored, err := r.cache.PutFill(fill)
	if err != nil || !stored {
		return err
	}
	order, ok := r.cache.Order(fill.AccountID, fill.OrderID)
	if !ok {
		return nil
	}
	order = applyFillToOrder(order, fill)
	return r.cache.PutOrder(order)
}

func (r *Reconciler) replayFills(accountID model.AccountID, orderID model.OrderID) error {
	fills := r.cache.FillsForOrder(accountID, orderID)
	for _, fill := range fills {
		order, ok := r.cache.Order(accountID, orderID)
		if !ok {
			return nil
		}
		order = applyFillToOrder(order, fill)
		if err := r.cache.PutOrder(order); err != nil {
			return err
		}
	}
	return nil
}

func applyFillToOrder(order model.OrderStatusReport, fill model.FillReport) model.OrderStatusReport {
	previousFilled := order.FilledQuantity
	nextFilled := previousFilled.Add(fill.Quantity)
	if order.Quantity.IsPositive() && nextFilled.GreaterThan(order.Quantity) {
		nextFilled = order.Quantity
	}
	order.FilledQuantity = nextFilled
	order.AveragePrice = averageFillPrice(order.AveragePrice, previousFilled, fill.Price, fill.Quantity)
	if order.Quantity.IsPositive() {
		order.LeavesQuantity = order.Quantity.Sub(nextFilled)
		if order.LeavesQuantity.IsNegative() {
			order.LeavesQuantity = decimal.Zero
		}
		if nextFilled.GreaterThanOrEqual(order.Quantity) {
			order.Status = model.OrderStatusFilled
			order.LeavesQuantity = decimal.Zero
		} else if nextFilled.IsPositive() {
			order.Status = model.OrderStatusPartiallyFilled
		}
	} else if nextFilled.IsPositive() {
		order.Status = model.OrderStatusPartiallyFilled
	}
	if fill.Timestamp.After(order.LastUpdatedTime) {
		order.LastUpdatedTime = fill.Timestamp
	}
	return order
}

func averageFillPrice(previousAvg decimal.Decimal, previousQty decimal.Decimal, fillPrice decimal.Decimal, fillQty decimal.Decimal) decimal.Decimal {
	nextQty := previousQty.Add(fillQty)
	if nextQty.IsZero() {
		return decimal.Zero
	}
	if previousAvg.IsZero() || previousQty.IsZero() {
		return fillPrice
	}
	previousNotional := previousAvg.Mul(previousQty)
	fillNotional := fillPrice.Mul(fillQty)
	return previousNotional.Add(fillNotional).Div(nextQty)
}

type orderSnapshotIndex struct {
	orderIDs       map[model.OrderID]struct{}
	clientOrderIDs map[model.ClientOrderID]struct{}
	venueOrderIDs  map[model.VenueOrderID]struct{}
}

func newOrderSnapshotIndex(reports []model.OrderStatusReport, accountID model.AccountID, instrumentID model.InstrumentID) orderSnapshotIndex {
	idx := orderSnapshotIndex{
		orderIDs:       make(map[model.OrderID]struct{}),
		clientOrderIDs: make(map[model.ClientOrderID]struct{}),
		venueOrderIDs:  make(map[model.VenueOrderID]struct{}),
	}
	for _, report := range reports {
		if report.AccountID != accountID || report.InstrumentID != instrumentID {
			continue
		}
		if report.OrderID != "" {
			idx.orderIDs[report.OrderID] = struct{}{}
		}
		if report.ClientOrderID != "" {
			idx.clientOrderIDs[report.ClientOrderID] = struct{}{}
		}
		if report.VenueOrderID != "" {
			idx.venueOrderIDs[report.VenueOrderID] = struct{}{}
		}
	}
	return idx
}

func (i orderSnapshotIndex) contains(order model.OrderStatusReport) bool {
	if order.OrderID != "" {
		if _, ok := i.orderIDs[order.OrderID]; ok {
			return true
		}
	}
	if order.ClientOrderID != "" {
		if _, ok := i.clientOrderIDs[order.ClientOrderID]; ok {
			return true
		}
	}
	if order.VenueOrderID != "" {
		if _, ok := i.venueOrderIDs[order.VenueOrderID]; ok {
			return true
		}
	}
	return false
}

func orderHasRecentActivity(order model.OrderStatusReport, now time.Time, window time.Duration) bool {
	if window <= 0 {
		return false
	}
	last := order.LastUpdatedTime
	if order.Metadata.TsInit.After(last) {
		last = order.Metadata.TsInit
	}
	if last.IsZero() {
		return false
	}
	return now.Sub(last) < window
}

type positionSnapshotIndex struct {
	positionIDs      map[model.PositionID]model.PositionStatusReport
	venuePositionIDs map[model.VenuePositionID]model.PositionStatusReport
}

func newPositionSnapshotIndex(reports []model.PositionStatusReport, accountID model.AccountID, instrumentID model.InstrumentID) positionSnapshotIndex {
	idx := positionSnapshotIndex{
		positionIDs:      make(map[model.PositionID]model.PositionStatusReport),
		venuePositionIDs: make(map[model.VenuePositionID]model.PositionStatusReport),
	}
	for _, report := range reports {
		if report.AccountID != accountID || report.InstrumentID != instrumentID {
			continue
		}
		if report.PositionID != "" {
			idx.positionIDs[report.PositionID] = report
		}
		if report.VenuePositionID != "" {
			idx.venuePositionIDs[report.VenuePositionID] = report
		}
	}
	return idx
}

func (i positionSnapshotIndex) contains(position model.PositionStatusReport) bool {
	_, ok := i.match(position)
	return ok
}

func (i positionSnapshotIndex) match(position model.PositionStatusReport) (model.PositionStatusReport, bool) {
	if position.PositionID != "" {
		if report, ok := i.positionIDs[position.PositionID]; ok {
			return report, true
		}
	}
	if position.VenuePositionID != "" {
		if report, ok := i.venuePositionIDs[position.VenuePositionID]; ok {
			return report, true
		}
	}
	return model.PositionStatusReport{}, false
}

func nextStatus(from model.OrderStatus, to model.OrderStatus) (model.OrderStatus, bool) {
	return NextOrderStatus(from, to)
}

func (r *Reconciler) appendPositionRepair(local model.PositionStatusReport, repair model.PositionStatusReport, kind string, maxAttempts int, result *PositionRepairResult) {
	key := positionRepairKeyFor(local)
	attempts := r.positionRepairAttemptsFor(key)
	if maxAttempts > 0 && attempts >= maxAttempts {
		result.Unresolved = append(result.Unresolved, PositionRepairDiscrepancy{
			Kind:         kind,
			AccountID:    local.AccountID,
			InstrumentID: local.InstrumentID,
			PositionID:   local.PositionID,
			Attempts:     attempts,
			Reason:       fmt.Sprintf("position discrepancy unresolved after %d repair attempts", attempts),
		})
		return
	}
	r.incrementPositionRepairAttempt(key)
	result.Generated = append(result.Generated, repair)
}

func (r *Reconciler) positionRepairAttemptsFor(key positionRepairKey) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ensurePositionRepairAttempts()
	return r.positionRepairAttempts[key]
}

func (r *Reconciler) incrementPositionRepairAttempt(key positionRepairKey) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ensurePositionRepairAttempts()
	r.positionRepairAttempts[key]++
}

func (r *Reconciler) clearPositionRepairAttempt(key positionRepairKey) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ensurePositionRepairAttempts()
	delete(r.positionRepairAttempts, key)
}

func (r *Reconciler) ensurePositionRepairAttempts() {
	if r.positionRepairAttempts == nil {
		r.positionRepairAttempts = make(map[positionRepairKey]int)
	}
}

func positionRepairKeyFor(position model.PositionStatusReport) positionRepairKey {
	return positionRepairKey{
		accountID:    position.AccountID,
		instrumentID: position.InstrumentID,
		positionID:   position.PositionID,
	}
}

func positionsMatch(local model.PositionStatusReport, venue model.PositionStatusReport) bool {
	return local.Side == venue.Side && local.Quantity.Equal(venue.Quantity)
}

func positionDiscrepancyKind(local model.PositionStatusReport, venue model.PositionStatusReport) string {
	if local.Side != venue.Side {
		return "position_side_mismatch"
	}
	if !local.Quantity.Equal(venue.Quantity) {
		return "position_quantity_mismatch"
	}
	return "position_report_mismatch"
}
