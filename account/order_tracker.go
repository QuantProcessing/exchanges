package account

import (
	"sync"

	"github.com/QuantProcessing/exchanges/model"
)

type OrderTracker struct {
	account       *TradingAccount
	orderID       model.OrderID
	clientOrderID model.ClientOrderID
	orders        chan model.OrderStatusReport
	fills         chan model.FillReport

	mu     sync.RWMutex
	latest model.OrderStatusReport
	closed bool
}

func newOrderTracker(account *TradingAccount, report model.OrderStatusReport, bufferSize int) *OrderTracker {
	if bufferSize <= 0 {
		bufferSize = defaultTrackerBufferSize
	}
	return &OrderTracker{
		account:       account,
		orderID:       report.OrderID,
		clientOrderID: report.ClientOrderID,
		orders:        make(chan model.OrderStatusReport, bufferSize),
		fills:         make(chan model.FillReport, bufferSize),
	}
}

func (t *OrderTracker) C() <-chan model.OrderStatusReport { return t.orders }

func (t *OrderTracker) Fills() <-chan model.FillReport { return t.fills }

func (t *OrderTracker) Latest() (model.OrderStatusReport, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.latest, t.latest.OrderID != ""
}

func (t *OrderTracker) Close() {
	if t == nil {
		return
	}
	if t.account != nil {
		t.account.unregisterTracker(t)
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return
	}
	t.closed = true
	close(t.orders)
	close(t.fills)
}

func (t *OrderTracker) matchesOrder(order model.OrderStatusReport) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if order.OrderID != "" && t.orderID != "" && order.OrderID == t.orderID {
		return true
	}
	return order.ClientOrderID != "" && t.clientOrderID != "" && order.ClientOrderID == t.clientOrderID
}

func (t *OrderTracker) matchesFill(fill model.FillReport) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if fill.OrderID != "" && t.orderID != "" && fill.OrderID == t.orderID {
		return true
	}
	return fill.ClientOrderID != "" && t.clientOrderID != "" && fill.ClientOrderID == t.clientOrderID
}

func (t *OrderTracker) sendOrder(report model.OrderStatusReport) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return true
	}
	if t.orderID == "" {
		t.orderID = report.OrderID
	}
	if t.clientOrderID == "" {
		t.clientOrderID = report.ClientOrderID
	}
	t.latest = report
	select {
	case t.orders <- report:
		return true
	default:
		return false
	}
}

func (t *OrderTracker) sendFill(fill model.FillReport) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return true
	}
	select {
	case t.fills <- fill:
		return true
	default:
		return false
	}
}
