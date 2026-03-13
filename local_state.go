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
// LocalStateManager — generic order/position/balance tracker
// ============================================================================

// LocalStateManager provides thread-safe local state management for adapters.
// It tracks Orders, Positions, and Balances, typically updated via WebSocket streams.
// Unlike LocalOrderBook (which is per-exchange), this is a shared concrete
// implementation because order/position state management logic is identical
// across all exchanges.
type LocalStateManager struct {
	mu     sync.RWMutex
	logger Logger

	positions map[string]*Position // Symbol -> Position
	orders    map[string]*Order    // OrderID -> Order
	balance   decimal.Decimal

	// Channels for broadcasting updates if users want stream access
	orderUpdateCh    chan *Order
	positionUpdateCh chan *Position

	initialized bool
}

// NewLocalStateManager initializes a new LocalStateManager
func NewLocalStateManager(logger Logger) *LocalStateManager {
	if logger == nil {
		logger = NopLogger
	}
	return &LocalStateManager{
		logger:           logger,
		positions:        make(map[string]*Position),
		orders:           make(map[string]*Order),
		orderUpdateCh:    make(chan *Order, 100),
		positionUpdateCh: make(chan *Position, 100),
		initialized:      false,
	}
}

// SetInitialState populates the state from a REST API snapshot
func (m *LocalStateManager) SetInitialState(balance decimal.Decimal, positions []Position, orders []Order) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.balance = balance
	m.positions = make(map[string]*Position)
	m.orders = make(map[string]*Order)

	for _, p := range positions {
		cp := p
		m.positions[p.Symbol] = &cp
	}
	for _, o := range orders {
		co := o
		m.orders[o.OrderID] = &co
	}
	m.initialized = true
}

// ApplyOrderUpdate applies an incremental order update
func (m *LocalStateManager) ApplyOrderUpdate(o *Order) {
	m.mu.Lock()
	defer m.mu.Unlock()

	isTerminal := o.Status == OrderStatusFilled || o.Status == OrderStatusCancelled || o.Status == OrderStatusRejected

	if isTerminal {
		delete(m.orders, o.OrderID)
	} else {
		co := *o
		m.orders[o.OrderID] = &co
	}

	// Non-blocking broadcast
	select {
	case m.orderUpdateCh <- o:
	default:
		m.logger.Warnw("order update channel full, dropping update", "orderID", o.OrderID)
	}
}

// ApplyPositionUpdate applies an incremental position update
func (m *LocalStateManager) ApplyPositionUpdate(p *Position) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cp := *p
	m.positions[p.Symbol] = &cp

	// Non-blocking broadcast
	select {
	case m.positionUpdateCh <- p:
	default:
		m.logger.Warnw("position update channel full, dropping update", "symbol", p.Symbol)
	}
}

// UpdateBalance applies an incremental balance update
func (m *LocalStateManager) UpdateBalance(balance decimal.Decimal) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.balance = balance
}

// GetOrder returns a copy of the desired order if it exists locally
func (m *LocalStateManager) GetOrder(orderID string) (*Order, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	o, ok := m.orders[orderID]
	if !ok {
		return nil, false
	}
	co := *o
	return &co, true
}

// GetPosition returns a copy of the desired position if it exists locally
func (m *LocalStateManager) GetPosition(symbol string) (*Position, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.positions[symbol]
	if !ok {
		return nil, false
	}
	cp := *p
	return &cp, true
}

// GetAllPositions returns a list of all positions
func (m *LocalStateManager) GetAllPositions() []Position {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Position, 0, len(m.positions))
	for _, p := range m.positions {
		result = append(result, *p)
	}
	return result
}

// GetAllOpenOrders returns a list of all current open orders
func (m *LocalStateManager) GetAllOpenOrders() []Order {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Order, 0, len(m.orders))
	for _, o := range m.orders {
		result = append(result, *o)
	}
	return result
}

// GetBalance returns the cached total balance
func (m *LocalStateManager) GetBalance() decimal.Decimal {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.balance
}

// GetOrderStream gets the read-only channel for order updates
func (m *LocalStateManager) GetOrderStream() <-chan *Order {
	return m.orderUpdateCh
}

// GetPositionStream gets the read-only channel for position updates
func (m *LocalStateManager) GetPositionStream() <-chan *Position {
	return m.positionUpdateCh
}

// IsInitialized returns whether state has been populated
func (m *LocalStateManager) IsInitialized() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.initialized
}

// Close closes the LocalStateManager (channels cleaned up by GC).
func (m *LocalStateManager) Close() {}

// ============================================================================
// AccountManager — auto-sync wrapper over PerpExchange
// ============================================================================

// AccountManager automates local state management for a perp adapter.
// It synchronizes Positions, Orders, and Balance via WebSocket and exposes read-only channels.
// It requires the adapter to implement both PerpExchange and Streamable.
type AccountManager struct {
	adapter    PerpExchange
	streamable Streamable
	logger     Logger
	mu         sync.RWMutex

	// Local State
	positions map[string]*Position // Symbol -> Position
	orders    map[string]*Order    // OrderID -> Order
	balance   decimal.Decimal

	// Channels for broadcasting updates
	orderUpdateCh    chan *Order
	positionUpdateCh chan *Position

	started bool
	done    chan struct{}
}

// NewAccountManager creates a new manager for the given perp adapter.
// The adapter must implement both PerpExchange and Streamable.
func NewAccountManager(adapter PerpExchange, logger Logger) (*AccountManager, error) {
	streamable, ok := adapter.(Streamable)
	if !ok {
		return nil, fmt.Errorf("adapter %s does not implement Streamable", adapter.GetExchange())
	}
	if logger == nil {
		logger = NopLogger
	}
	return &AccountManager{
		adapter:          adapter,
		streamable:       streamable,
		logger:           logger,
		positions:        make(map[string]*Position),
		orders:           make(map[string]*Order),
		orderUpdateCh:    make(chan *Order, 100),
		positionUpdateCh: make(chan *Position, 100),
		done:             make(chan struct{}),
	}, nil
}

// Start initializes the account state and subscribes to updates.
func (m *AccountManager) Start(ctx context.Context, syncInterval time.Duration) error {
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return nil
	}
	m.started = true
	m.mu.Unlock()

	if syncInterval == 0 {
		syncInterval = 1 * time.Minute
	}

	m.logger.Infow("fetching initial account state")
	acc, err := m.adapter.FetchAccount(ctx)
	if err != nil {
		return fmt.Errorf("failed to get initial account state: %w", err)
	}

	m.mu.Lock()
	m.balance = acc.TotalBalance
	m.positions = make(map[string]*Position)
	m.orders = make(map[string]*Order)
	for _, p := range acc.Positions {
		cp := p
		m.positions[p.Symbol] = &cp
	}
	for _, o := range acc.Orders {
		co := o
		m.orders[o.OrderID] = &co
	}
	m.mu.Unlock()
	m.logger.Infow("initial state loaded", "orders", len(m.orders), "positions", len(m.positions))

	// Subscribe to Order Updates
	err = m.streamable.WatchOrders(ctx, func(o *Order) {
		m.handleOrderUpdate(o)
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe order update: %w", err)
	}

	// Subscribe to Position Updates
	err = m.streamable.WatchPositions(ctx, func(p *Position) {
		m.handlePositionUpdate(p)
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe position update: %w", err)
	}

	m.startBalanceSync(ctx, syncInterval)
	return nil
}

// Close closes the manager.
func (m *AccountManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.started {
		return
	}
	m.started = false
	close(m.done)
}

func (m *AccountManager) handleOrderUpdate(o *Order) {
	m.mu.Lock()
	defer m.mu.Unlock()

	isTerminal := o.Status == OrderStatusFilled || o.Status == OrderStatusCancelled || o.Status == OrderStatusRejected
	if isTerminal {
		delete(m.orders, o.OrderID)
	} else {
		co := *o
		m.orders[o.OrderID] = &co
	}

	select {
	case m.orderUpdateCh <- o:
	default:
		m.logger.Warnw("order update channel full, dropping update", "orderID", o.OrderID)
	}
}

func (m *AccountManager) handlePositionUpdate(p *Position) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cp := *p
	m.positions[p.Symbol] = &cp

	select {
	case m.positionUpdateCh <- p:
	default:
		m.logger.Warnw("position update channel full, dropping update", "symbol", p.Symbol)
	}
}

// GetOrder returns a copy of the order if found.
func (m *AccountManager) GetOrder(orderID string) (*Order, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	o, ok := m.orders[orderID]
	if !ok {
		return nil, false
	}
	co := *o
	return &co, true
}

// GetPosition returns a copy of the position for the symbol.
func (m *AccountManager) GetPosition(symbol string) (*Position, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.positions[symbol]
	if !ok {
		return nil, false
	}
	cp := *p
	return &cp, true
}

// GetOrderStream returns the read-only channel for order updates.
func (m *AccountManager) GetOrderStream() <-chan *Order {
	return m.orderUpdateCh
}

// GetPositionStream returns the read-only channel for position updates.
func (m *AccountManager) GetPositionStream() <-chan *Position {
	return m.positionUpdateCh
}

// GetAllPositions returns a copy of all current positions.
func (m *AccountManager) GetAllPositions() []*Position {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Position, 0, len(m.positions))
	for _, p := range m.positions {
		cp := *p
		result = append(result, &cp)
	}
	return result
}

// GetLocalBalance returns the last known balance.
func (m *AccountManager) GetLocalBalance() decimal.Decimal {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.balance
}

// ForceRefresh manually re-fetches full state from REST API.
func (m *AccountManager) ForceRefresh(ctx context.Context) error {
	acc, err := m.adapter.FetchAccount(ctx)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.balance = acc.TotalBalance
	m.positions = make(map[string]*Position)
	m.orders = make(map[string]*Order)

	for _, p := range acc.Positions {
		cp := p
		m.positions[p.Symbol] = &cp
	}
	for _, o := range acc.Orders {
		co := o
		m.orders[o.OrderID] = &co
	}
	return nil
}

func (m *AccountManager) startBalanceSync(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-m.done:
				return
			case <-ticker.C:
				if err := m.ForceRefresh(ctx); err != nil {
					m.logger.Errorw("failed to refresh state", "error", err)
				} else {
					m.logger.Debugw("periodic state refresh completed", "balance", m.GetLocalBalance())
				}
			}
		}
	}()
}
