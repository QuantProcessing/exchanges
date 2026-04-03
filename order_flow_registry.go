package exchanges

import "sync"

type orderFlowRegistry struct {
	mu         sync.Mutex
	byOrderID  map[string]*OrderFlow
	byClientID map[string]*OrderFlow
}

func newOrderFlowRegistry() *orderFlowRegistry {
	return &orderFlowRegistry{
		byOrderID:  make(map[string]*OrderFlow),
		byClientID: make(map[string]*OrderFlow),
	}
}

func (r *orderFlowRegistry) Register(initial *Order) *OrderFlow {
	r.mu.Lock()
	defer r.mu.Unlock()

	flow := newOrderFlow(initial)
	if initial != nil {
		if initial.OrderID != "" {
			r.byOrderID[initial.OrderID] = flow
		}
		if initial.ClientOrderID != "" {
			r.byClientID[initial.ClientOrderID] = flow
		}
	}
	return flow
}

func (r *orderFlowRegistry) Route(update *Order) {
	if update == nil {
		return
	}

	r.mu.Lock()
	flow := r.byOrderID[update.OrderID]
	if flow == nil && update.ClientOrderID != "" {
		flow = r.byClientID[update.ClientOrderID]
	}
	if flow != nil {
		if update.OrderID != "" {
			r.byOrderID[update.OrderID] = flow
		}
		if update.ClientOrderID != "" {
			r.byClientID[update.ClientOrderID] = flow
		}
	}
	r.mu.Unlock()

	if flow != nil {
		flow.publish(update)
	}
}
