package account

import (
	"sync"

	exchanges "github.com/QuantProcessing/exchanges"
)

const pendingFillCap = 32

type orderFlowRegistry struct {
	mu              sync.Mutex
	byOrderID       map[string]*OrderFlow
	byClientID      map[string]*OrderFlow
	all             map[*OrderFlow]struct{}
	pendingByOrder  map[string][]*exchanges.Fill
	pendingByClient map[string][]*exchanges.Fill
}

func newOrderFlowRegistry() *orderFlowRegistry {
	return &orderFlowRegistry{
		byOrderID:       make(map[string]*OrderFlow),
		byClientID:      make(map[string]*OrderFlow),
		all:             make(map[*OrderFlow]struct{}),
		pendingByOrder:  make(map[string][]*exchanges.Fill),
		pendingByClient: make(map[string][]*exchanges.Fill),
	}
}

func (r *orderFlowRegistry) Register(initial *exchanges.Order) *OrderFlow {
	flow := newOrderFlow(initial)

	r.mu.Lock()
	r.all[flow] = struct{}{}
	orderID := ""
	clientOrderID := ""
	if initial != nil {
		orderID = initial.OrderID
		clientOrderID = initial.ClientOrderID
		if orderID != "" {
			r.byOrderID[orderID] = flow
		}
		if clientOrderID != "" {
			r.byClientID[clientOrderID] = flow
		}
	}
	pending := r.takePendingLocked(orderID, clientOrderID)
	r.mu.Unlock()

	for _, fill := range pending {
		flow.publishFill(fill)
	}

	r.unregisterOnClose(flow)
	return flow
}

func (r *orderFlowRegistry) Add(flow *OrderFlow) {
	if flow == nil {
		return
	}

	r.mu.Lock()
	r.all[flow] = struct{}{}
	r.mu.Unlock()

	r.unregisterOnClose(flow)
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
	r.pendingByOrder = make(map[string][]*exchanges.Fill)
	r.pendingByClient = make(map[string][]*exchanges.Fill)
	r.mu.Unlock()

	for _, flow := range flows {
		if flow != nil {
			flow.Close()
		}
	}
}

func (r *orderFlowRegistry) Route(update *exchanges.Order) {
	r.RouteOrder(update)
}

func (r *orderFlowRegistry) RouteOrder(update *exchanges.Order) {
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
	pending := r.takePendingLocked(update.OrderID, update.ClientOrderID)
	r.mu.Unlock()

	if flow == nil {
		return
	}

	flow.publishOrder(update)
	for _, fill := range pending {
		flow.publishFill(fill)
	}
	r.unregisterIfTerminal(flow)
}

func (r *orderFlowRegistry) RouteFill(fill *exchanges.Fill) {
	if fill == nil {
		return
	}

	r.mu.Lock()
	flow := r.byOrderID[fill.OrderID]
	if flow == nil && fill.ClientOrderID != "" {
		flow = r.byClientID[fill.ClientOrderID]
	}
	if flow == nil {
		r.storePendingLocked(fill)
		r.mu.Unlock()
		return
	}
	r.mu.Unlock()

	flow.publishFill(fill)
	r.unregisterIfTerminal(flow)
}

func (r *orderFlowRegistry) Unregister(flow *OrderFlow) {
	if flow == nil {
		return
	}

	r.mu.Lock()
	delete(r.all, flow)
	for orderID, candidate := range r.byOrderID {
		if candidate == flow {
			delete(r.byOrderID, orderID)
		}
	}
	for clientOrderID, candidate := range r.byClientID {
		if candidate == flow {
			delete(r.byClientID, clientOrderID)
		}
	}
	r.mu.Unlock()
}

func (r *orderFlowRegistry) unregisterOnClose(flow *OrderFlow) {
	go func() {
		<-flow.done
		r.Unregister(flow)
	}()
}

func (r *orderFlowRegistry) unregisterIfTerminal(flow *OrderFlow) {
	if flow == nil {
		return
	}
	latest := flow.Latest()
	if latest == nil || !isMergedTerminalStatus(latest.Status) {
		return
	}
	r.Unregister(flow)
}

func (r *orderFlowRegistry) storePendingLocked(fill *exchanges.Fill) {
	if fill.OrderID != "" {
		r.pendingByOrder[fill.OrderID] = appendBoundedFill(r.pendingByOrder[fill.OrderID], fill)
	}
	if fill.ClientOrderID != "" {
		r.pendingByClient[fill.ClientOrderID] = appendBoundedFill(r.pendingByClient[fill.ClientOrderID], fill)
	}
}

func appendBoundedFill(queue []*exchanges.Fill, fill *exchanges.Fill) []*exchanges.Fill {
	queue = append(queue, cloneFill(fill))
	if len(queue) > pendingFillCap {
		queue[0] = nil
		queue = queue[1:]
	}
	return queue
}

func (r *orderFlowRegistry) takePendingLocked(orderID, clientOrderID string) []*exchanges.Fill {
	result := make([]*exchanges.Fill, 0)
	seen := make(map[string]struct{})
	appendUnique := func(queue []*exchanges.Fill) {
		for _, fill := range queue {
			if fill == nil {
				continue
			}
			key := fillDedupKey(fill)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			result = append(result, cloneFill(fill))
		}
	}

	if orderID != "" {
		appendUnique(r.pendingByOrder[orderID])
		delete(r.pendingByOrder, orderID)
	}
	if clientOrderID != "" {
		appendUnique(r.pendingByClient[clientOrderID])
		delete(r.pendingByClient, clientOrderID)
	}

	return result
}

func isMergedTerminalStatus(status exchanges.OrderStatus) bool {
	return status == exchanges.OrderStatusFilled ||
		status == exchanges.OrderStatusCancelled ||
		status == exchanges.OrderStatusRejected
}
