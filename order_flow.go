package exchanges

import (
	"context"
	"fmt"
	"sync"
)

type OrderFlow struct {
	mu        sync.RWMutex
	latest    *Order
	events    []*Order
	ch        chan *Order
	notify    chan struct{}
	closed    bool
	closeOnce sync.Once
}

func newOrderFlow(initial *Order) *OrderFlow {
	f := &OrderFlow{
		ch:     make(chan *Order, 32),
		notify: make(chan struct{}),
	}
	if initial != nil {
		copy := cloneOrder(initial)
		f.latest = copy
		f.events = append(f.events, copy)
	}
	return f
}

func (f *OrderFlow) C() <-chan *Order {
	return f.ch
}

func (f *OrderFlow) Latest() *Order {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return cloneOrder(f.latest)
}

func (f *OrderFlow) Wait(ctx context.Context, predicate func(*Order) bool) (*Order, error) {
	if predicate == nil {
		return nil, fmt.Errorf("predicate required")
	}

	f.mu.Lock()
	for {
		if latest := cloneOrder(f.latest); latest != nil && predicate(latest) {
			f.mu.Unlock()
			return latest, nil
		}
		for _, order := range f.events {
			if order != nil && predicate(order) {
				copy := cloneOrder(order)
				f.mu.Unlock()
				return copy, nil
			}
		}
		if f.closed {
			f.mu.Unlock()
			return nil, fmt.Errorf("order flow closed")
		}
		notify := f.notify
		f.mu.Unlock()

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-notify:
		}

		f.mu.Lock()
	}
}

func (f *OrderFlow) Close() {
	f.closeOnce.Do(func() {
		f.mu.Lock()
		f.closed = true
		notify := f.notify
		f.notify = nil
		close(f.ch)
		f.mu.Unlock()
		if notify != nil {
			close(notify)
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
	f.events = append(f.events, copy)
	notify := f.notify
	f.notify = make(chan struct{})

	select {
	case f.ch <- copy:
	default:
	}

	f.mu.Unlock()
	if notify != nil {
		close(notify)
	}
}

func cloneOrder(order *Order) *Order {
	if order == nil {
		return nil
	}
	copy := *order
	return &copy
}
