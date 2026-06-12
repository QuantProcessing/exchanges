package account

import (
	"fmt"
	"sync"

	"github.com/QuantProcessing/exchanges/model"
)

type Reconciler struct {
	cache           *Cache
	orderSM         OrderStateMachine
	mu              sync.RWMutex
	flowsByOrderID  map[model.OrderID]*OrderTracker
	flowsByClientID map[model.ClientOrderID]*OrderTracker
	positions       map[positionKey]model.PositionStatusReport
}

type positionKey struct {
	accountID    model.AccountID
	instrumentID model.InstrumentID
	positionID   model.PositionID
	side         model.PositionSide
}

func NewReconciler(cache *Cache) *Reconciler {
	if cache == nil {
		cache = NewCache()
	}
	return &Reconciler{
		cache:           cache,
		flowsByOrderID:  make(map[model.OrderID]*OrderTracker),
		flowsByClientID: make(map[model.ClientOrderID]*OrderTracker),
		positions:       make(map[positionKey]model.PositionStatusReport),
	}
}

func (r *Reconciler) ApplyEvent(ev model.ExecutionEvent) error {
	if ev.AccountState != nil {
		if err := r.ApplyAccountState(*ev.AccountState); err != nil {
			return err
		}
	}
	if ev.Order != nil {
		if err := r.ApplyOrderStatusReport(*ev.Order); err != nil {
			return err
		}
	}
	if ev.OrderEvent != nil {
		if err := r.ApplyOrderEvent(*ev.OrderEvent); err != nil {
			return err
		}
	}
	if ev.Fill != nil {
		if err := r.ApplyFillReport(*ev.Fill); err != nil {
			return err
		}
	}
	if ev.Position != nil {
		if err := r.ApplyPositionStatusReport(*ev.Position); err != nil {
			return err
		}
	}
	return nil
}

func (r *Reconciler) ApplyAccountState(state model.AccountState) error {
	if err := r.cache.ApplyAccountState(state); err != nil {
		return err
	}
	for _, pos := range state.Positions {
		if err := r.ApplyPositionStatusReport(pos); err != nil {
			return err
		}
	}
	return nil
}

func (r *Reconciler) ApplyOrderStatusReport(report model.OrderStatusReport) error {
	if report.AccountID == "" {
		return fmt.Errorf("%w: missing order account id", model.ErrInvalidAccountState)
	}
	if err := report.InstrumentID.Validate(); err != nil {
		return err
	}
	if err := r.cache.PutOrderStatus(report); err != nil {
		return err
	}
	flow := r.flowForOrderLocked(report.OrderID, report.ClientID)
	flow.publishOrder(report)
	return nil
}

func (r *Reconciler) ApplyOrderEvent(event model.OrderEvent) error {
	flow := r.flowForOrderLocked(event.OrderID, event.ClientID)
	current, ok := flow.Latest()
	var currentPtr *model.OrderStatusReport
	if ok {
		currentPtr = &current
	}
	next, changed, err := r.orderSM.ApplyEvent(currentPtr, event)
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}
	if err := r.cache.PutOrderStatus(next); err != nil {
		return err
	}
	flow.publishOrder(next)
	flow.publishEvent(event)
	return nil
}

func (r *Reconciler) ApplyFillReport(report model.FillReport) error {
	if report.AccountID == "" {
		return fmt.Errorf("%w: missing fill account id", model.ErrInvalidAccountState)
	}
	if err := report.InstrumentID.Validate(); err != nil {
		return err
	}
	if report.TradeID != "" {
		if err := r.cache.PutFill(report); err != nil {
			return err
		}
	}
	flow := r.flowForOrderLocked(report.OrderID, report.ClientID)
	flow.publishFill(report)
	return nil
}

func (r *Reconciler) ApplyPositionStatusReport(report model.PositionStatusReport) error {
	if report.AccountID == "" {
		return fmt.Errorf("%w: missing position account id", model.ErrInvalidAccountState)
	}
	if err := report.InstrumentID.Validate(); err != nil {
		return err
	}
	if err := r.cache.PutPosition(report); err != nil {
		return err
	}
	key := positionKey{
		accountID:    report.AccountID,
		instrumentID: report.InstrumentID,
		positionID:   report.PositionID,
		side:         report.Side,
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.positions[key] = report
	return nil
}

func (r *Reconciler) EnsureFlowForClientID(clientID model.ClientOrderID) *OrderTracker {
	r.mu.Lock()
	defer r.mu.Unlock()
	if flow, ok := r.flowsByClientID[clientID]; ok {
		return flow
	}
	flow := newOrderTracker(nil)
	if clientID != "" {
		r.flowsByClientID[clientID] = flow
	}
	return flow
}

func (r *Reconciler) FlowByOrderID(orderID model.OrderID) (*OrderTracker, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	flow, ok := r.flowsByOrderID[orderID]
	return flow, ok
}

func (r *Reconciler) FlowByClientID(clientID model.ClientOrderID) (*OrderTracker, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	flow, ok := r.flowsByClientID[clientID]
	return flow, ok
}

func (r *Reconciler) PositionsSnapshot() []model.PositionStatusReport {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]model.PositionStatusReport, 0, len(r.positions))
	for _, pos := range r.positions {
		out = append(out, pos)
	}
	return out
}

func (r *Reconciler) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	seen := make(map[*OrderTracker]struct{}, len(r.flowsByOrderID)+len(r.flowsByClientID))
	for _, flow := range r.flowsByOrderID {
		seen[flow] = struct{}{}
	}
	for _, flow := range r.flowsByClientID {
		seen[flow] = struct{}{}
	}
	for flow := range seen {
		flow.Close()
	}
}

func (r *Reconciler) flowForOrderLocked(orderID model.OrderID, clientID model.ClientOrderID) *OrderTracker {
	r.mu.Lock()
	defer r.mu.Unlock()

	if orderID != "" {
		if flow, ok := r.flowsByOrderID[orderID]; ok {
			if clientID != "" {
				r.flowsByClientID[clientID] = flow
			}
			return flow
		}
	}
	if clientID != "" {
		if flow, ok := r.flowsByClientID[clientID]; ok {
			if orderID != "" {
				r.flowsByOrderID[orderID] = flow
			}
			return flow
		}
	}

	flow := newOrderTracker(nil)
	if orderID != "" {
		r.flowsByOrderID[orderID] = flow
	}
	if clientID != "" {
		r.flowsByClientID[clientID] = flow
	}
	return flow
}
