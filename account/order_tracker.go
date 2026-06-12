package account

import (
	"sync"

	"github.com/QuantProcessing/exchanges/model"
)

type OrderTracker struct {
	mu        sync.RWMutex
	latest    *model.OrderStatusReport
	events    []model.OrderEvent
	fills     []model.FillReport
	orderCh   chan model.OrderStatusReport
	eventCh   chan model.OrderEvent
	fillCh    chan model.FillReport
	done      chan struct{}
	closed    bool
	closeOnce sync.Once
}

func newOrderTracker(initial *model.OrderStatusReport) *OrderTracker {
	f := &OrderTracker{
		orderCh: make(chan model.OrderStatusReport, 64),
		eventCh: make(chan model.OrderEvent, 64),
		fillCh:  make(chan model.FillReport, 64),
		done:    make(chan struct{}),
	}
	if initial != nil {
		copy := *initial
		f.latest = &copy
	}
	return f
}

func (f *OrderTracker) C() <-chan model.OrderStatusReport {
	return f.orderCh
}

func (f *OrderTracker) Fills() <-chan model.FillReport {
	return f.fillCh
}

func (f *OrderTracker) Events() <-chan model.OrderEvent {
	return f.eventCh
}

func (f *OrderTracker) Latest() (model.OrderStatusReport, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.latest == nil {
		return model.OrderStatusReport{}, false
	}
	return *f.latest, true
}

func (f *OrderTracker) FillsSnapshot() []model.FillReport {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return append([]model.FillReport(nil), f.fills...)
}

func (f *OrderTracker) EventsSnapshot() []model.OrderEvent {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return append([]model.OrderEvent(nil), f.events...)
}

func (f *OrderTracker) publishOrder(report model.OrderStatusReport) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return
	}
	copy := report
	f.latest = &copy

	select {
	case f.orderCh <- report:
	default:
	}
}

func (f *OrderTracker) publishEvent(event model.OrderEvent) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return
	}
	f.events = append(f.events, event)

	select {
	case f.eventCh <- event:
	default:
	}
}

func (f *OrderTracker) publishFill(fill model.FillReport) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return
	}
	f.fills = append(f.fills, fill)

	select {
	case f.fillCh <- fill:
	default:
	}
}

func (f *OrderTracker) Close() {
	f.closeOnce.Do(func() {
		f.mu.Lock()
		f.closed = true
		f.mu.Unlock()
		close(f.done)
		close(f.orderCh)
		close(f.eventCh)
		close(f.fillCh)
	})
}
