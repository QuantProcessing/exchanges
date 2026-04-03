package exchanges

import "sync"

type orderFlowRegistry struct {
	mu         sync.Mutex
	byOrderID  map[string]*OrderFlow
	byClientID map[string]*OrderFlow
	all        map[*OrderFlow]struct{}
}

func newOrderFlowRegistry() *orderFlowRegistry {
	return &orderFlowRegistry{
		byOrderID:  make(map[string]*OrderFlow),
		byClientID: make(map[string]*OrderFlow),
		all:        make(map[*OrderFlow]struct{}),
	}
}

func (r *orderFlowRegistry) Register(initial *Order) *OrderFlow {
	flow := newOrderFlow(initial)

	r.mu.Lock()
	r.all[flow] = struct{}{}
	if initial != nil {
		if initial.OrderID != "" {
			r.byOrderID[initial.OrderID] = flow
		}
		if initial.ClientOrderID != "" {
			r.byClientID[initial.ClientOrderID] = flow
		}
	}
	r.mu.Unlock()

	return flow
}

func (r *orderFlowRegistry) Add(flow *OrderFlow) {
	if flow == nil {
		return
	}

	r.mu.Lock()
	r.all[flow] = struct{}{}
	r.mu.Unlock()
}

func (r *orderFlowRegistry) CloseAll() {
	r.mu.Lock()
	flows := make([]*OrderFlow, 0, len(r.all))
	for flow := range r.all {
		flows = append(flows, flow)
	}
	r.byOrderID = make(map[string]*OrderFlow)
	r.byClientID = make(map[string]*OrderFlow)
	r.all = make(map[*OrderFlow]struct{})
	r.mu.Unlock()

	for _, flow := range flows {
		if flow != nil {
			flow.Close()
		}
	}
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
		r.all[flow] = struct{}{}
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
