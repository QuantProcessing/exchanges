package exchanges

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type OrderFlow struct {
	mu         sync.Mutex
	latest     *Order
	ch         chan *Order
	publicQ    []*Order
	publicWake chan struct{}
	waiters    map[*orderFlowWaiter]struct{}
	done       chan struct{}
	pubWG      sync.WaitGroup
	closed     bool
	closeOnce  sync.Once
}

type orderFlowWaiter struct {
	predicate func(*Order) bool
	ch        chan *Order
}

func newOrderFlow(initial *Order) *OrderFlow {
	f := &OrderFlow{
		ch:         make(chan *Order),
		publicWake: make(chan struct{}, 1),
		waiters:    make(map[*orderFlowWaiter]struct{}),
		done:       make(chan struct{}),
	}
	if initial != nil {
		f.latest = cloneOrder(initial)
	}
	f.pubWG.Add(1)
	go f.dispatchPublic()
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
	if f.closed {
		f.mu.Unlock()
		return nil, fmt.Errorf("order flow closed")
	}
	initial := cloneOrder(f.latest)
	f.mu.Unlock()

	if initial != nil && predicate(initial) {
		return initial, nil
	}

	waiter := &orderFlowWaiter{
		predicate: predicate,
		ch:        make(chan *Order, 1),
	}

	f.mu.Lock()
	if f.closed {
		f.mu.Unlock()
		return nil, fmt.Errorf("order flow closed")
	}
	f.waiters[waiter] = struct{}{}
	snapshot := cloneOrder(f.latest)
	f.mu.Unlock()

	if snapshot != nil && predicate(snapshot) {
		f.removeWaiter(waiter)
		return snapshot, nil
	}

	defer f.removeWaiter(waiter)

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-f.done:
		return nil, fmt.Errorf("order flow closed")
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
		close(f.done)
		select {
		case f.publicWake <- struct{}{}:
		default:
		}
		f.waiters = nil
		f.mu.Unlock()

		f.pubWG.Wait()
		close(f.ch)
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

	waiters := make([]*orderFlowWaiter, 0, len(f.waiters))
	for waiter := range f.waiters {
		waiters = append(waiters, waiter)
	}
	f.publicQ = append(f.publicQ, cloneOrder(copy))
	select {
	case f.publicWake <- struct{}{}:
	default:
	}

	f.mu.Unlock()

	matched := make([]*orderFlowWaiter, 0, len(waiters))
	for _, waiter := range waiters {
		if waiter != nil && waiter.predicate != nil && waiter.predicate(copy) {
			matched = append(matched, waiter)
		}
	}

	if len(matched) > 0 {
		f.mu.Lock()
		if !f.closed && f.waiters != nil {
			for _, waiter := range matched {
				delete(f.waiters, waiter)
			}
		}
		f.mu.Unlock()
	}

	for _, waiter := range matched {
		select {
		case waiter.ch <- cloneOrder(copy):
		default:
		}
	}
}

func (f *OrderFlow) dispatchPublic() {
	defer f.pubWG.Done()

	for {
		f.mu.Lock()
		if len(f.publicQ) == 0 {
			if f.closed {
				f.mu.Unlock()
				return
			}
			wake := f.publicWake
			f.mu.Unlock()

			select {
			case <-wake:
			case <-f.done:
				return
			}
			continue
		}

		next := f.publicQ[0]
		f.mu.Unlock()

		select {
		case <-f.done:
			return
		case f.ch <- cloneOrder(next):
			f.mu.Lock()
			if len(f.publicQ) > 0 {
				f.publicQ[0] = nil
				f.publicQ = f.publicQ[1:]
			}
			f.mu.Unlock()
		default:
			select {
			case <-f.done:
				return
			case <-f.publicWake:
			case <-time.After(1 * time.Millisecond):
			}
		}
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
