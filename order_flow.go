package exchanges

import (
	"context"
	"fmt"
	"sync"
)

type OrderFlow struct {
	mu        sync.Mutex
	latest    *Order
	ch        chan *Order
	waiters   map[*orderFlowWaiter]struct{}
	closed    bool
	closeOnce sync.Once
}

type orderFlowWaiter struct {
	predicate func(*Order) bool
	ch        chan *Order
}

func newOrderFlow(initial *Order) *OrderFlow {
	f := &OrderFlow{
		ch:      make(chan *Order, 32),
		waiters: make(map[*orderFlowWaiter]struct{}),
	}
	if initial != nil {
		f.latest = cloneOrder(initial)
	}
	return f
}

func (f *OrderFlow) C() <-chan *Order {
	return f.ch
}

func (f *OrderFlow) Latest() *Order {
	f.mu.Lock()
	defer f.mu.Unlock()
	return cloneOrder(f.latest)
}

func (f *OrderFlow) Wait(ctx context.Context, predicate func(*Order) bool) (*Order, error) {
	if predicate == nil {
		return nil, fmt.Errorf("predicate required")
	}

	f.mu.Lock()
	if latest := cloneOrder(f.latest); latest != nil && predicate(latest) {
		f.mu.Unlock()
		return latest, nil
	}
	if f.closed {
		f.mu.Unlock()
		return nil, fmt.Errorf("order flow closed")
	}

	waiter := &orderFlowWaiter{
		predicate: predicate,
		ch:        make(chan *Order, 1),
	}
	f.waiters[waiter] = struct{}{}
	f.mu.Unlock()

	defer f.removeWaiter(waiter)

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case order, ok := <-waiter.ch:
		if !ok {
			return nil, fmt.Errorf("order flow closed")
		}
		return order, nil
	}
}

func (f *OrderFlow) Close() {
	f.closeOnce.Do(func() {
		f.mu.Lock()
		if f.closed {
			f.mu.Unlock()
			return
		}
		f.closed = true
		waiters := make([]*orderFlowWaiter, 0, len(f.waiters))
		for waiter := range f.waiters {
			waiters = append(waiters, waiter)
		}
		f.waiters = nil
		close(f.ch)
		f.mu.Unlock()

		for _, waiter := range waiters {
			close(waiter.ch)
		}
	})
}

func (f *OrderFlow) publish(order *Order) {
	if order == nil {
		return
	}

	copy := cloneOrder(order)

	f.mu.Lock()
	if f.closed {
		f.mu.Unlock()
		return
	}

	f.latest = copy

	matched := make([]*orderFlowWaiter, 0, len(f.waiters))
	for waiter := range f.waiters {
		if waiter != nil && waiter.predicate != nil && waiter.predicate(copy) {
			matched = append(matched, waiter)
		}
	}
	for _, waiter := range matched {
		delete(f.waiters, waiter)
	}

	select {
	case f.ch <- cloneOrder(copy):
	default:
	}

	f.mu.Unlock()

	for _, waiter := range matched {
		waiter.ch <- cloneOrder(copy)
	}
}

func (f *OrderFlow) removeWaiter(waiter *orderFlowWaiter) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.waiters == nil {
		return
	}
	delete(f.waiters, waiter)
}

func cloneOrder(order *Order) *Order {
	if order == nil {
		return nil
	}
	copy := *order
	return &copy
}
