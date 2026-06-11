package account

import (
	"sync"

	"github.com/QuantProcessing/exchanges/model"
)

type V2OrderFlow struct {
	mu        sync.RWMutex
	latest    *model.OrderStatusReport
	fills     []model.FillReport
	orderCh   chan model.OrderStatusReport
	fillCh    chan model.FillReport
	done      chan struct{}
	closed    bool
	closeOnce sync.Once
}

func newV2OrderFlow(initial *model.OrderStatusReport) *V2OrderFlow {
	f := &V2OrderFlow{
		orderCh: make(chan model.OrderStatusReport, 64),
		fillCh:  make(chan model.FillReport, 64),
		done:    make(chan struct{}),
	}
	if initial != nil {
		copy := *initial
		f.latest = &copy
	}
	return f
}

func (f *V2OrderFlow) C() <-chan model.OrderStatusReport {
	return f.orderCh
}

func (f *V2OrderFlow) Fills() <-chan model.FillReport {
	return f.fillCh
}

func (f *V2OrderFlow) Latest() (model.OrderStatusReport, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.latest == nil {
		return model.OrderStatusReport{}, false
	}
	return *f.latest, true
}

func (f *V2OrderFlow) FillsSnapshot() []model.FillReport {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return append([]model.FillReport(nil), f.fills...)
}

func (f *V2OrderFlow) publishOrder(report model.OrderStatusReport) {
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

func (f *V2OrderFlow) publishFill(fill model.FillReport) {
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

func (f *V2OrderFlow) Close() {
	f.closeOnce.Do(func() {
		f.mu.Lock()
		f.closed = true
		f.mu.Unlock()
		close(f.done)
		close(f.orderCh)
		close(f.fillCh)
	})
}
