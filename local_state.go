package exchanges

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/shopspring/decimal"
)

// ============================================================================
// LocalOrderBook — per-exchange orderbook interface
// ============================================================================

// LocalOrderBook interface standardizes the output of locally maintained orderbooks.
// This allows the BaseAdapter and upper logic layers to consume depth data
// uniformly, regardless of whether the internal sync uses delta snapshots,
// buffer timestamps, or gapless polling.
//
// Each exchange implements this interface in its own orderbook.go file because
// synchronization protocols differ (Binance: diff+snapshot, Nado: gap detection,
// OKX: checksum validation, etc.).
type LocalOrderBook interface {
	// GetDepth returns the sorted top `limit` depth levels.
	// Bids are sorted descending (highest price first).
	// Asks are sorted ascending (lowest price first).
	GetDepth(limit int) ([]Level, []Level)

	// WaitReady blocks until the orderbook is initialized or the timeout expires.
	WaitReady(ctx context.Context, timeout time.Duration) bool

	// Timestamp returns the Unix millisecond timestamp of the last update.
	Timestamp() int64
}

// ============================================================================
// LocalState — unified local state manager
// ============================================================================

// LocalState is the legacy public compatibility surface for the account-runtime
// synchronizer. Prefer TradingAccount for new code. This type remains because
// adapters and existing callers already depend on it.
//
// It automatically maintains Orders, Positions, and Balance via WebSocket
// streams, and provides fan-out event subscriptions and integrated order
// tracking.
//
// Usage:
//
//	state := exchanges.NewLocalState(adp, logger)
//	state.Start(ctx)
//
//	// Query state
//	pos, _ := state.GetPosition("BTC")
//
//	// Place order with tracking
//	result, _ := state.PlaceOrder(ctx, params)
//	defer result.Done()
//	filled, _ := result.WaitTerminal(30 * time.Second)
type LocalState struct {
	adp    Exchange
	logger Logger
	mu     sync.RWMutex

	// Local state
	orders    map[string]*Order    // OrderID -> Order (open orders only)
	positions map[string]*Position // Symbol -> Position
	balance   decimal.Decimal

	// Event buses (fan-out)
	orderBus    *EventBus[Order]
	positionBus *EventBus[Position]

	started bool
	done    chan struct{}
}

// NewLocalState creates a new LocalState wrapping the given Exchange adapter.
func NewLocalState(adp Exchange, logger Logger) *LocalState {
	if logger == nil {
		logger = NopLogger
	}
	return &LocalState{
		adp:         adp,
		logger:      logger,
		orders:      make(map[string]*Order),
		positions:   make(map[string]*Position),
		orderBus:    NewEventBus[Order](),
		positionBus: NewEventBus[Position](),
		done:        make(chan struct{}),
	}
}

// Start initializes local state and subscribes to WebSocket streams.
// It performs: REST snapshot → WatchOrders → WatchPositions → periodic refresh.
func (s *LocalState) Start(ctx context.Context) (err error) {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return nil
	}
	s.started = true
	s.mu.Unlock()
	defer func() {
		if err != nil {
			s.mu.Lock()
			s.started = false
			s.balance = decimal.Zero
			s.positions = make(map[string]*Position)
			s.orders = make(map[string]*Order)
			s.mu.Unlock()
		}
	}()

	// 1. REST snapshot
	s.logger.Infow("local_state: fetching initial account state")
	acc, err := s.adp.FetchAccount(ctx)
	if err != nil {
		return fmt.Errorf("local_state: failed to get initial state: %w", err)
	}

	s.mu.Lock()
	s.balance = acc.TotalBalance
	for _, p := range acc.Positions {
		cp := p
		s.positions[p.Symbol] = &cp
	}
	for _, o := range acc.Orders {
		co := o
		s.orders[o.OrderID] = &co
	}
	s.mu.Unlock()
	s.logger.Infow("local_state: initial state loaded",
		"orders", len(acc.Orders), "positions", len(acc.Positions))

	// 2. Subscribe to order updates
	streamable, ok := s.adp.(Streamable)
	if !ok {
		return fmt.Errorf("local_state: adapter %s does not implement Streamable", s.adp.GetExchange())
	}

	if err := streamable.WatchOrders(ctx, func(o *Order) {
		s.applyOrderUpdate(o)
	}); err != nil {
		return fmt.Errorf("local_state: WatchOrders failed: %w", err)
	}

	// 3. Subscribe to position updates
	if watchErr := streamable.WatchPositions(ctx, func(p *Position) {
		s.applyPositionUpdate(p)
	}); watchErr != nil {
		s.logger.Warnw("local_state: WatchPositions failed (may not be supported)", "error", watchErr)
		// Not fatal — some adapters may not support position streaming
	}

	// 4. Periodic full refresh
	go s.periodicRefresh(ctx, 1*time.Minute)

	return nil
}

// Close stops the LocalState and releases resources.
func (s *LocalState) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.started {
		return
	}
	s.started = false
	close(s.done)
	s.orderBus.Close()
	s.positionBus.Close()
}

// ============================================================================
// State Queries (read local cache, no network)
// ============================================================================

// GetOrder returns a copy of the order if found in local state.
func (s *LocalState) GetOrder(orderID string) (*Order, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	o, ok := s.orders[orderID]
	if !ok {
		return nil, false
	}
	co := *o
	return &co, true
}

// GetPosition returns a copy of the position for the symbol.
func (s *LocalState) GetPosition(symbol string) (*Position, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.positions[symbol]
	if !ok {
		return nil, false
	}
	cp := *p
	return &cp, true
}

// GetAllOpenOrders returns copies of all open orders.
func (s *LocalState) GetAllOpenOrders() []Order {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Order, 0, len(s.orders))
	for _, o := range s.orders {
		result = append(result, *o)
	}
	return result
}

// GetAllPositions returns copies of all positions.
func (s *LocalState) GetAllPositions() []Position {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Position, 0, len(s.positions))
	for _, p := range s.positions {
		result = append(result, *p)
	}
	return result
}

// GetBalance returns the last known balance.
func (s *LocalState) GetBalance() decimal.Decimal {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.balance
}

// ============================================================================
// Event Subscriptions (fan-out, multiple consumers supported)
// ============================================================================

// SubscribeOrders returns a subscription for all order update events.
// Call sub.Unsubscribe() when done.
func (s *LocalState) SubscribeOrders() *Subscription[Order] {
	return s.orderBus.Subscribe()
}

// SubscribePositions returns a subscription for all position update events.
func (s *LocalState) SubscribePositions() *Subscription[Position] {
	return s.positionBus.Subscribe()
}

// ============================================================================
// Order Placement with Tracking
// ============================================================================

// OrderResult holds the result of PlaceOrder along with a filtered subscription
// for tracking this specific order's lifecycle.
type OrderResult struct {
	Order  *Order               // Initial order snapshot
	Sub    *Subscription[Order] // Filtered updates for this order only
	cancel func()
}

// WaitTerminal blocks until the order reaches a terminal state
// (FILLED, CANCELLED, or REJECTED) or the timeout expires.
func (r *OrderResult) WaitTerminal(timeout time.Duration) (*Order, error) {
	timer := time.After(timeout)
	for {
		select {
		case o, ok := <-r.Sub.C:
			if !ok {
				return nil, fmt.Errorf("subscription closed")
			}
			if o.Status == OrderStatusFilled ||
				o.Status == OrderStatusCancelled ||
				o.Status == OrderStatusRejected {
				return o, nil
			}
		case <-timer:
			orderID := r.Order.OrderID
			if orderID == "" {
				orderID = r.Order.ClientOrderID
			}
			return nil, fmt.Errorf("timeout waiting for order %s to reach terminal state", orderID)
		}
	}
}

// Done releases the subscription resources. Always call this when finished.
func (r *OrderResult) Done() {
	if r.cancel != nil {
		r.cancel()
		return
	}
	if r.Sub != nil {
		r.Sub.Unsubscribe()
	}
}

// PlaceOrder places an order and returns an OrderResult with a subscription
// that receives only this order's status updates.
//
// Usage:
//
//	result, err := state.PlaceOrder(ctx, params)
//	defer result.Done()
//	filled, err := result.WaitTerminal(30 * time.Second)
func (s *LocalState) PlaceOrder(ctx context.Context, params *OrderParams) (*OrderResult, error) {
	// 1. Subscribe to all order events BEFORE placing (prevent race)
	allSub := s.orderBus.Subscribe()

	// 2. Place the order
	order, err := s.adp.PlaceOrder(ctx, params)
	if err != nil {
		allSub.Unsubscribe()
		return nil, err
	}

	return s.trackOrderResult(allSub, order), nil
}

// PlaceOrderWS submits an order via the adapter's explicit WS path and tracks
// the resulting lifecycle updates by ClientID.
func (s *LocalState) PlaceOrderWS(ctx context.Context, params *OrderParams) (*OrderResult, error) {
	if params.ClientID == "" {
		return nil, fmt.Errorf("client id required for PlaceOrderWS")
	}

	allSub := s.orderBus.Subscribe()
	if err := s.adp.PlaceOrderWS(ctx, params); err != nil {
		allSub.Unsubscribe()
		return nil, err
	}

	order := &Order{
		ClientOrderID: params.ClientID,
		Symbol:        params.Symbol,
		Side:          params.Side,
		Type:          params.Type,
		Quantity:      params.Quantity,
		Price:         params.Price,
		Status:        OrderStatusPending,
		Timestamp:     time.Now().UnixMilli(),
	}

	return s.trackOrderResult(allSub, order), nil
}

func (s *LocalState) trackOrderResult(allSub *Subscription[Order], order *Order) *OrderResult {
	orderID := order.OrderID
	clientOrderID := order.ClientOrderID
	filteredCh := make(chan *Order, 16)
	result := &OrderResult{
		Order:  order,
		cancel: allSub.Unsubscribe,
	}

	go func() {
		defer close(filteredCh)
		for o := range allSub.C {
			match := (orderID != "" && o.OrderID == orderID) ||
				(clientOrderID != "" && o.ClientOrderID == clientOrderID)
			if match {
				updated := *o
				if updated.OrderID == "" {
					updated.OrderID = result.Order.OrderID
				}
				if updated.ClientOrderID == "" {
					updated.ClientOrderID = result.Order.ClientOrderID
				}
				*result.Order = updated
				orderID = result.Order.OrderID
				clientOrderID = result.Order.ClientOrderID

				select {
				case filteredCh <- o:
				default:
				}
				// Auto-close on terminal state
				if o.Status == OrderStatusFilled ||
					o.Status == OrderStatusCancelled ||
					o.Status == OrderStatusRejected {
					allSub.Unsubscribe()
					return
				}
			}
		}
	}()

	filteredSub := &Subscription[Order]{
		C: filteredCh,
	}

	result.Sub = filteredSub
	return result
}

// ============================================================================
// Internal state management
// ============================================================================

func (s *LocalState) applyOrderUpdate(o *Order) {
	s.mu.Lock()
	isTerminal := o.Status == OrderStatusFilled ||
		o.Status == OrderStatusCancelled ||
		o.Status == OrderStatusRejected

	if isTerminal {
		delete(s.orders, o.OrderID)
	} else {
		co := *o
		s.orders[o.OrderID] = &co
	}
	s.mu.Unlock()

	// Fan-out to all subscribers
	s.orderBus.Publish(o)
}

func (s *LocalState) applyPositionUpdate(p *Position) {
	s.mu.Lock()
	cp := *p
	s.positions[p.Symbol] = &cp
	s.mu.Unlock()

	s.positionBus.Publish(p)
}

func (s *LocalState) periodicRefresh(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.done:
			return
		case <-ticker.C:
			acc, err := s.adp.FetchAccount(ctx)
			if err != nil {
				s.logger.Errorw("local_state: periodic refresh failed", "error", err)
				continue
			}
			s.mu.Lock()
			s.balance = acc.TotalBalance
			s.positions = make(map[string]*Position)
			s.orders = make(map[string]*Order)
			for _, p := range acc.Positions {
				cp := p
				s.positions[p.Symbol] = &cp
			}
			for _, o := range acc.Orders {
				co := o
				s.orders[o.OrderID] = &co
			}
			s.mu.Unlock()
			s.logger.Debugw("local_state: periodic refresh completed", "balance", acc.TotalBalance)
		}
	}
}
