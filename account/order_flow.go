package account

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/shopspring/decimal"
)

type flowEventKind int

const (
	flowEventOrder flowEventKind = iota
	flowEventFill
)

type flowEvent struct {
	kind  flowEventKind
	order *exchanges.Order
	fill  *exchanges.Fill
}

type OrderFlow struct {
	mu              sync.Mutex
	base            *exchanges.Order
	latest          *exchanges.Order
	lastFill        *exchanges.Fill
	orderCh         chan *exchanges.Order
	fillCh          chan *exchanges.Fill
	publicQ         []flowEvent
	publicWake      chan struct{}
	waiters         map[*orderFlowWaiter]struct{}
	done            chan struct{}
	pubWG           sync.WaitGroup
	closed          bool
	closeOnce       sync.Once
	fillTotalQty    decimal.Decimal
	fillTotalQuote  decimal.Decimal
	orderFilledSeen bool
	seenFills       map[string]struct{}
}

type orderFlowWaiter struct {
	predicate func(*exchanges.Order) bool
	ch        chan *exchanges.Order
}

func newOrderFlow(initial *exchanges.Order) *OrderFlow {
	f := &OrderFlow{
		orderCh:    make(chan *exchanges.Order, 64),
		fillCh:     make(chan *exchanges.Fill, 64),
		publicWake: make(chan struct{}, 1),
		waiters:    make(map[*orderFlowWaiter]struct{}),
		done:       make(chan struct{}),
		seenFills:  make(map[string]struct{}),
	}
	if initial != nil {
		f.base = cloneOrder(initial)
		f.latest = cloneOrder(initial)
	}
	f.pubWG.Add(1)
	go f.dispatchPublic()
	return f
}

func (f *OrderFlow) C() <-chan *exchanges.Order {
	return f.orderCh
}

func (f *OrderFlow) Fills() <-chan *exchanges.Fill {
	return f.fillCh
}

func (f *OrderFlow) Latest() *exchanges.Order {
	f.mu.Lock()
	defer f.mu.Unlock()
	return cloneOrder(f.latest)
}

func (f *OrderFlow) Wait(ctx context.Context, predicate func(*exchanges.Order) bool) (*exchanges.Order, error) {
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
		ch:        make(chan *exchanges.Order, 1),
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
		close(f.orderCh)
		close(f.fillCh)
	})
}

func (f *OrderFlow) publish(order *exchanges.Order) {
	f.publishOrder(order)
}

func (f *OrderFlow) seedPlacement(order *exchanges.Order) {
	if order == nil {
		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return
	}

	f.base = mergePlacementOrder(f.base, order)
	f.latest = mergePlacementOrder(f.latest, order)
}

func (f *OrderFlow) publishOrder(order *exchanges.Order) {
	if order == nil {
		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return
	}

	f.base = cloneOrder(order)
	if order.Status == exchanges.OrderStatusFilled {
		f.orderFilledSeen = true
	}

	merged := f.recomputeMergedLocked()
	f.latest = cloneOrder(merged)
	f.publicQ = append(f.publicQ, flowEvent{kind: flowEventOrder, order: cloneOrder(merged)})
	f.notifyMatchedWaitersLocked(merged)
	f.wakePublicLocked()
}

func (f *OrderFlow) publishFill(fill *exchanges.Fill) {
	if fill == nil {
		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return
	}

	key := fillDedupKey(fill)
	if _, seen := f.seenFills[key]; seen {
		return
	}
	f.seenFills[key] = struct{}{}

	copyFill := cloneFill(fill)
	f.lastFill = copyFill
	f.fillTotalQty = f.fillTotalQty.Add(fill.Quantity)
	f.fillTotalQuote = f.fillTotalQuote.Add(fill.Price.Mul(fill.Quantity))
	f.publicQ = append(f.publicQ, flowEvent{kind: flowEventFill, fill: cloneFill(copyFill)})

	merged := f.recomputeMergedLocked()
	f.latest = cloneOrder(merged)
	f.publicQ = append(f.publicQ, flowEvent{kind: flowEventOrder, order: cloneOrder(merged)})
	f.notifyMatchedWaitersLocked(merged)
	f.wakePublicLocked()
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

		switch next.kind {
		case flowEventFill:
			select {
			case <-f.done:
				return
			case f.fillCh <- cloneFill(next.fill):
				f.shiftPublic()
			default:
				f.waitForPublicWake()
			}
		default:
			select {
			case <-f.done:
				return
			case f.orderCh <- cloneOrder(next.order):
				f.shiftPublic()
			default:
				f.waitForPublicWake()
			}
		}
	}
}

func (f *OrderFlow) recomputeMergedLocked() *exchanges.Order {
	merged := cloneOrder(f.base)
	if merged == nil {
		merged = &exchanges.Order{}
	}

	if f.lastFill != nil {
		merged.OrderID = firstNonEmpty(merged.OrderID, f.lastFill.OrderID)
		merged.ClientOrderID = firstNonEmpty(merged.ClientOrderID, f.lastFill.ClientOrderID)
		merged.Symbol = firstNonEmpty(merged.Symbol, f.lastFill.Symbol)
		if merged.Side == "" {
			merged.Side = f.lastFill.Side
		}
		merged.LastFillQuantity = f.lastFill.Quantity
		merged.LastFillPrice = f.lastFill.Price
		if merged.Timestamp == 0 {
			merged.Timestamp = f.lastFill.Timestamp
		}
	}

	if f.fillTotalQty.GreaterThan(merged.FilledQuantity) {
		merged.FilledQuantity = f.fillTotalQty
	}
	if f.fillTotalQty.IsPositive() {
		avg := f.fillTotalQuote.Div(f.fillTotalQty)
		merged.AverageFillPrice = normalizeDecimal(avg)
	}

	if isRawNonFillTerminal(merged.Status) {
		return merged
	}

	if !f.fillTotalQty.IsPositive() && merged.Status == exchanges.OrderStatusFilled {
		if f.latest != nil && f.latest.Status != exchanges.OrderStatusFilled {
			merged.Status = f.latest.Status
		} else {
			merged.Status = exchanges.OrderStatusPending
		}
		return merged
	}

	if f.fillTotalQty.IsPositive() {
		if merged.Quantity.IsPositive() && !f.fillTotalQty.LessThan(merged.Quantity) {
			merged.Status = exchanges.OrderStatusFilled
		} else if f.orderFilledSeen {
			merged.Status = exchanges.OrderStatusFilled
		} else {
			merged.Status = exchanges.OrderStatusPartiallyFilled
		}
	}

	return merged
}

func (f *OrderFlow) notifyMatchedWaitersLocked(order *exchanges.Order) {
	waiters := make([]*orderFlowWaiter, 0, len(f.waiters))
	for waiter := range f.waiters {
		waiters = append(waiters, waiter)
	}

	matched := make([]*orderFlowWaiter, 0, len(waiters))
	for _, waiter := range waiters {
		if waiter != nil && waiter.predicate != nil && waiter.predicate(order) {
			matched = append(matched, waiter)
		}
	}

	for _, waiter := range matched {
		delete(f.waiters, waiter)
		select {
		case waiter.ch <- cloneOrder(order):
		default:
		}
	}
}

func (f *OrderFlow) wakePublicLocked() {
	select {
	case f.publicWake <- struct{}{}:
	default:
	}
}

func (f *OrderFlow) shiftPublic() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.publicQ) == 0 {
		return
	}
	f.publicQ[0] = flowEvent{}
	f.publicQ = f.publicQ[1:]
}

func (f *OrderFlow) waitForPublicWake() {
	select {
	case <-f.done:
		return
	case <-f.publicWake:
	case <-time.After(1 * time.Millisecond):
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

func cloneOrder(order *exchanges.Order) *exchanges.Order {
	if order == nil {
		return nil
	}
	copy := *order
	return &copy
}

func mergePlacementOrder(current, placement *exchanges.Order) *exchanges.Order {
	if placement == nil {
		return cloneOrder(current)
	}
	if current == nil {
		return cloneOrder(placement)
	}

	merged := cloneOrder(current)
	merged.OrderID = firstNonEmpty(merged.OrderID, placement.OrderID)
	merged.ClientOrderID = firstNonEmpty(merged.ClientOrderID, placement.ClientOrderID)
	merged.Symbol = firstNonEmpty(merged.Symbol, placement.Symbol)
	if merged.Side == "" {
		merged.Side = placement.Side
	}
	if merged.Type == "" || merged.Type == exchanges.OrderTypeUnknown {
		merged.Type = placement.Type
	}
	if !merged.Quantity.IsPositive() && placement.Quantity.IsPositive() {
		merged.Quantity = placement.Quantity
	}
	if !merged.Price.IsPositive() && placement.Price.IsPositive() {
		merged.Price = placement.Price
	}
	if !merged.OrderPrice.IsPositive() {
		if placement.OrderPrice.IsPositive() {
			merged.OrderPrice = placement.OrderPrice
		} else if placement.Price.IsPositive() {
			merged.OrderPrice = placement.Price
		}
	}
	if merged.TimeInForce == "" {
		merged.TimeInForce = placement.TimeInForce
	}
	if !merged.ReduceOnly {
		merged.ReduceOnly = placement.ReduceOnly
	}
	if merged.Status == "" || merged.Status == exchanges.OrderStatusPending || merged.Status == exchanges.OrderStatusUnknown {
		merged.Status = placement.Status
	}
	if !merged.FilledQuantity.IsPositive() && placement.FilledQuantity.IsPositive() {
		merged.FilledQuantity = placement.FilledQuantity
	}
	if !merged.LastFillQuantity.IsPositive() && placement.LastFillQuantity.IsPositive() {
		merged.LastFillQuantity = placement.LastFillQuantity
	}
	if !merged.LastFillPrice.IsPositive() && placement.LastFillPrice.IsPositive() {
		merged.LastFillPrice = placement.LastFillPrice
	}
	if !merged.AverageFillPrice.IsPositive() && placement.AverageFillPrice.IsPositive() {
		merged.AverageFillPrice = placement.AverageFillPrice
	}
	if !merged.Fee.IsPositive() && placement.Fee.IsPositive() {
		merged.Fee = placement.Fee
	}
	if merged.Timestamp == 0 {
		merged.Timestamp = placement.Timestamp
	}
	return merged
}

func cloneFill(fill *exchanges.Fill) *exchanges.Fill {
	if fill == nil {
		return nil
	}
	copy := *fill
	return &copy
}

func fillDedupKey(fill *exchanges.Fill) string {
	if strings.TrimSpace(fill.TradeID) != "" {
		return "trade:" + fill.TradeID
	}
	return fmt.Sprintf("%s|%s|%d|%s|%s|%s",
		fill.OrderID,
		fill.ClientOrderID,
		fill.Timestamp,
		fill.Price.String(),
		fill.Quantity.String(),
		fill.Fee.String(),
	)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func isRawNonFillTerminal(status exchanges.OrderStatus) bool {
	return status == exchanges.OrderStatusCancelled ||
		status == exchanges.OrderStatusRejected
}

func normalizeDecimal(value decimal.Decimal) decimal.Decimal {
	normalized, err := decimal.NewFromString(value.String())
	if err != nil {
		return value
	}
	return normalized
}
