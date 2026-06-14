package account

import (
	"fmt"

	"github.com/QuantProcessing/exchanges/cache"
	"github.com/QuantProcessing/exchanges/model"
	"github.com/shopspring/decimal"
)

type Reconciler struct {
	cache *cache.Cache
}

func NewReconciler(c *cache.Cache) *Reconciler {
	if c == nil {
		c = cache.New()
	}
	return &Reconciler{cache: c}
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
	if accountID == "" {
		return nil, fmt.Errorf("%w: account id is required", model.ErrInvalidOrder)
	}
	if err := instrumentID.Validate(); err != nil {
		return nil, err
	}
	if !missingStatus.IsTerminal() {
		return nil, fmt.Errorf("%w: missing order status must be terminal", model.ErrInvalidOrder)
	}
	observed := newOrderSnapshotIndex(reports, accountID, instrumentID)
	generated := make([]model.OrderStatusReport, 0)
	for _, order := range r.cache.OpenOrders(accountID) {
		if order.InstrumentID != instrumentID || observed.contains(order) {
			continue
		}
		order.Status = missingStatus
		if err := r.applyOrder(order); err != nil {
			return generated, err
		}
		generated = append(generated, order)
	}
	return generated, nil
}

func (r *Reconciler) MissingPositionReports(accountID model.AccountID, instrumentID model.InstrumentID, reports []model.PositionStatusReport) ([]model.PositionStatusReport, error) {
	if accountID == "" {
		return nil, fmt.Errorf("%w: account id is required", model.ErrInvalidOrder)
	}
	if err := instrumentID.Validate(); err != nil {
		return nil, err
	}
	observed := newPositionSnapshotIndex(reports, accountID, instrumentID)
	generated := make([]model.PositionStatusReport, 0)
	for _, position := range r.cache.PositionsForInstrument(instrumentID) {
		if position.AccountID != accountID || observed.contains(position) || position.Side == model.PositionSideFlat || position.Quantity.IsZero() {
			continue
		}
		position.Side = model.PositionSideFlat
		position.Quantity = decimal.Zero
		generated = append(generated, position)
	}
	return generated, nil
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

type positionSnapshotIndex struct {
	positionIDs map[model.PositionID]struct{}
	hasPosition bool
}

func newPositionSnapshotIndex(reports []model.PositionStatusReport, accountID model.AccountID, instrumentID model.InstrumentID) positionSnapshotIndex {
	idx := positionSnapshotIndex{positionIDs: make(map[model.PositionID]struct{})}
	for _, report := range reports {
		if report.AccountID != accountID || report.InstrumentID != instrumentID {
			continue
		}
		idx.hasPosition = true
		if report.PositionID != "" {
			idx.positionIDs[report.PositionID] = struct{}{}
		}
	}
	return idx
}

func (i positionSnapshotIndex) contains(position model.PositionStatusReport) bool {
	if position.PositionID != "" {
		if _, ok := i.positionIDs[position.PositionID]; ok {
			return true
		}
	}
	return i.hasPosition
}

func nextStatus(from model.OrderStatus, to model.OrderStatus) (model.OrderStatus, bool) {
	return NextOrderStatus(from, to)
}
