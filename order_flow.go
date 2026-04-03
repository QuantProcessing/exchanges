package exchanges

import (
	"context"
	"fmt"
	"sync"
)

type OrderFlow struct {
	mu        sync.RWMutex
	latest    *Order
	ch        chan *Order
	done      chan struct{}
	closeOnce sync.Once
}

func newOrderFlow(initial *Order) *OrderFlow {
	f := &OrderFlow{
		ch:   make(chan *Order, 32),
		done: make(chan struct{}),
	}
	if initial != nil {
		copy := *initial
		f.latest = &copy
	}
	return f
}

func (f *OrderFlow) C() <-chan *Order {
	return f.ch
}

func (f *OrderFlow) Latest() *Order {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.latest == nil {
		return nil
	}
	copy := *f.latest
	return &copy
}

func (f *OrderFlow) Wait(ctx context.Context, predicate func(*Order) bool) (*Order, error) {
	if predicate == nil {
		return nil, fmt.Errorf("predicate required")
	}
	if latest := f.Latest(); latest != nil && predicate(latest) {
		return latest, nil
	}

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-f.done:
			if latest := f.Latest(); latest != nil && predicate(latest) {
				return latest, nil
			}
			return nil, fmt.Errorf("order flow closed")
		case order, ok := <-f.ch:
			if !ok {
				if latest := f.Latest(); latest != nil && predicate(latest) {
					return latest, nil
				}
				return nil, fmt.Errorf("order flow closed")
			}
			if predicate(order) {
				latest := f.Latest()
				if latest != nil {
					return latest, nil
				}
				return order, nil
			}
		}
	}
}

func (f *OrderFlow) Close() {
	f.closeOnce.Do(func() {
		f.mu.Lock()
		defer f.mu.Unlock()
		close(f.done)
		close(f.ch)
	})
}

func (f *OrderFlow) publish(order *Order) {
	if order == nil {
		return
	}

	copy := *order

	f.mu.Lock()
	defer f.mu.Unlock()

	select {
	case <-f.done:
		return
	default:
	}

	f.latest = &copy

	select {
	case f.ch <- &copy:
	default:
	}
}
