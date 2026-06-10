package account

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
)

// placementClientIDInitializer lets adapters customise client-id formatting
// (e.g. Backpack requires a numeric uint32-range value).
type placementClientIDInitializer interface {
	EnsureClientID(params *exchanges.OrderParams) error
}

// baseTradeClient owns the market-agnostic execution machinery shared by
// PerpTradingAccount and SpotTradingAccount: order cache, order/fill flow
// fusion, client-id management, run lifecycle. It is unaware of positions,
// balances, leverage, or any market-specific concept.
//
// Each concrete *TradingAccount embeds baseTradeClient and adds its own
// state (positions/balances), its own Place strongly-typed entry point, and
// its own Start that wires in market-specific streams (e.g. WatchPositions).
type baseTradeClient struct {
	mu          sync.RWMutex // protects orders
	lifecycleMu sync.Mutex
	runMu       sync.RWMutex

	adp    exchanges.Exchange
	logger exchanges.Logger

	orders   map[string]*exchanges.Order
	orderBus *eventBus[exchanges.Order]
	flows    *orderFlowRegistry

	healthMu       sync.RWMutex
	streams        map[StreamName]StreamHealth
	snapshotLoaded bool
	lastSnapshotAt time.Time

	started   bool
	starting  bool
	closing   bool
	runCancel context.CancelFunc
	runGen    uint64
}

func newBaseTradeClient(adp exchanges.Exchange, logger exchanges.Logger) baseTradeClient {
	if logger == nil {
		logger = exchanges.NopLogger
	}
	return baseTradeClient{
		adp:      adp,
		logger:   logger,
		orders:   make(map[string]*exchanges.Order),
		orderBus: newEventBus[exchanges.Order](),
		flows:    newOrderFlowRegistry(),
		streams:  initialStreamHealth(),
	}
}

// =============================================================================
// Place / Cancel / Track — market-agnostic order lifecycle
// =============================================================================

// placeGeneric is the shared REST place path. Concrete subtypes call this
// after converting their strongly-typed params into exchanges.OrderParams.
func (b *baseTradeClient) placeGeneric(ctx context.Context, params *exchanges.OrderParams) (*OrderFlow, error) {
	if err := b.ensurePlacementClientID(params); err != nil {
		return nil, err
	}

	flow := b.flows.Register(pendingPlacementOrder(params))
	order, err := b.adp.PlaceOrder(ctx, params)
	if err != nil {
		flow.Close()
		return nil, err
	}

	if order != nil && strings.TrimSpace(order.ClientOrderID) == "" {
		order = cloneOrder(order)
		order.ClientOrderID = params.ClientID
	}
	flow.seedPlacement(order)
	b.flows.Bind(flow, order)
	return flow, nil
}

func (b *baseTradeClient) placeGenericWS(ctx context.Context, params *exchanges.OrderParams) (*OrderFlow, error) {
	if err := b.ensurePlacementClientID(params); err != nil {
		return nil, err
	}

	flow := b.flows.Register(pendingPlacementOrder(params))
	if err := b.adp.PlaceOrderWS(ctx, params); err != nil {
		flow.Close()
		return nil, err
	}
	return flow, nil
}

func (b *baseTradeClient) Cancel(ctx context.Context, orderID, symbol string) error {
	return b.adp.CancelOrder(ctx, orderID, symbol)
}

func (b *baseTradeClient) CancelWS(ctx context.Context, orderID, symbol string) error {
	return b.adp.CancelOrderWS(ctx, orderID, symbol)
}

func (b *baseTradeClient) Track(orderID, clientOrderID string) (*OrderFlow, error) {
	if strings.TrimSpace(orderID) == "" && strings.TrimSpace(clientOrderID) == "" {
		return nil, fmt.Errorf("order id or client order id required")
	}
	return b.flows.Register(&exchanges.Order{
		OrderID:       orderID,
		ClientOrderID: clientOrderID,
	}), nil
}

func (b *baseTradeClient) ensurePlacementClientID(params *exchanges.OrderParams) error {
	if params == nil {
		return fmt.Errorf("order params required")
	}
	if initializer, ok := b.adp.(placementClientIDInitializer); ok {
		if err := initializer.EnsureClientID(params); err != nil {
			return err
		}
	}
	params.ClientID = strings.TrimSpace(params.ClientID)
	if params.ClientID == "" {
		params.ClientID = exchanges.GenerateID()
	}
	return nil
}

func pendingPlacementOrder(params *exchanges.OrderParams) *exchanges.Order {
	if params == nil {
		return &exchanges.Order{Status: exchanges.OrderStatusPending, Timestamp: time.Now().UnixMilli()}
	}
	return &exchanges.Order{
		ClientOrderID: params.ClientID,
		Symbol:        params.Symbol,
		Side:          params.Side,
		Type:          params.Type,
		Quantity:      params.Quantity,
		Price:         params.Price,
		OrderPrice:    params.Price,
		ReduceOnly:    params.ReduceOnly,
		PostOnly:      params.PostOnly,
		TimeInForce:   params.TimeInForce,
		Status:        exchanges.OrderStatusPending,
		Timestamp:     time.Now().UnixMilli(),
	}
}

func isTerminalOrderStatus(status exchanges.OrderStatus) bool {
	return status == exchanges.OrderStatusFilled ||
		status == exchanges.OrderStatusCancelled ||
		status == exchanges.OrderStatusRejected
}

// =============================================================================
// Order cache queries
// =============================================================================

func (b *baseTradeClient) OpenOrder(orderID string) (*exchanges.Order, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	o, ok := b.orders[orderID]
	if !ok {
		return nil, false
	}
	c := *o
	return &c, true
}

func (b *baseTradeClient) OpenOrders() []exchanges.Order {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]exchanges.Order, 0, len(b.orders))
	for _, o := range b.orders {
		out = append(out, *o)
	}
	return out
}

func (b *baseTradeClient) SubscribeOrders() *Subscription[exchanges.Order] {
	return b.orderBus.Subscribe()
}

// =============================================================================
// Stream wiring — invoked by subtype Start()
// =============================================================================

// startOrderAndFillStreams subscribes to WatchOrders (mandatory) and
// WatchFills (optional via ErrNotSupported).
func (b *baseTradeClient) startOrderAndFillStreams(runCtx context.Context, runGen uint64) error {
	return b.startOrderAndFillStreamsWithPolicy(runCtx, runGen, true)
}

func (b *baseTradeClient) startOrderAndFillStreamsWithPolicy(runCtx context.Context, runGen uint64, requireOrders bool) error {
	streamable, ok := b.adp.(exchanges.Streamable)
	if !ok {
		return fmt.Errorf("trade_client: adapter %s does not implement Streamable", b.adp.GetExchange())
	}

	if err := streamable.WatchOrders(runCtx, func(o *exchanges.Order) {
		b.applyOrderUpdate(runGen, o)
	}); err != nil {
		if requireOrders || !errors.Is(err, exchanges.ErrNotSupported) {
			b.markStreamError(StreamOrders, err)
			return fmt.Errorf("trade_client: WatchOrders failed: %w", err)
		}
		b.markStreamUnsupported(StreamOrders, err)
		b.logger.Warnw("trade_client: WatchOrders not supported", "error", err)
	} else {
		b.markStreamReady(StreamOrders)
	}

	if err := streamable.WatchFills(runCtx, func(f *exchanges.Fill) {
		b.applyFillUpdate(runGen, f)
	}); err != nil {
		if !errors.Is(err, exchanges.ErrNotSupported) {
			b.markStreamError(StreamFills, err)
			return fmt.Errorf("trade_client: WatchFills failed: %w", err)
		}
		b.markStreamUnsupported(StreamFills, err)
		b.logger.Warnw("trade_client: WatchFills not supported", "error", err)
	} else {
		b.markStreamReady(StreamFills)
	}
	return nil
}

func (b *baseTradeClient) applyOrderSnapshot(runGen uint64, orders []exchanges.Order) {
	if !b.isActiveRun(runGen) {
		return
	}
	next := make(map[string]*exchanges.Order, len(orders))
	for _, o := range orders {
		c := o
		next[o.OrderID] = &c
	}
	b.mu.Lock()
	if b.isActiveRun(runGen) {
		b.orders = next
	}
	b.mu.Unlock()
}

func (b *baseTradeClient) applyOrderUpdate(runGen uint64, order *exchanges.Order) {
	if order == nil || !b.isActiveRun(runGen) {
		return
	}
	b.mu.Lock()
	if !b.isActiveRun(runGen) {
		b.mu.Unlock()
		return
	}
	if isTerminalOrderStatus(order.Status) {
		delete(b.orders, order.OrderID)
	} else {
		c := *order
		b.orders[order.OrderID] = &c
	}
	b.mu.Unlock()

	if !b.isActiveRun(runGen) {
		return
	}
	dropped := b.orderBus.Publish(order)
	b.markStreamEvent(StreamOrders, dropped)
	b.flows.RouteOrder(order)
}

func (b *baseTradeClient) applyFillUpdate(runGen uint64, fill *exchanges.Fill) {
	if fill == nil || !b.isActiveRun(runGen) {
		return
	}
	b.markStreamEvent(StreamFills, 0)
	b.flows.RouteFill(fill)
}

func (b *baseTradeClient) applyCurrentOrderUpdate(order *exchanges.Order) {
	b.runMu.RLock()
	runGen := b.runGen
	active := b.runCancel != nil && (b.started || b.starting)
	b.runMu.RUnlock()
	if !active {
		return
	}
	b.applyOrderUpdate(runGen, order)
}

func (b *baseTradeClient) resetOrderCache() {
	b.mu.Lock()
	b.orders = make(map[string]*exchanges.Order)
	b.mu.Unlock()
}

func (b *baseTradeClient) closeBaseStreams() {
	b.orderBus.Close()
	b.flows.CloseAll()
	b.markStreamsStopped()
}

// periodicRefresh polls FetchAccount on the given interval and delegates
// snapshot interpretation to the subtype via the apply callback.
func (b *baseTradeClient) periodicRefresh(
	ctx context.Context,
	interval time.Duration,
	runGen uint64,
	apply func(*exchanges.Account),
) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			acc, err := b.adp.FetchAccount(ctx)
			if err != nil {
				if ctx.Err() != nil || !b.isActiveRun(runGen) {
					return
				}
				b.logger.Errorw("trade_client: periodic refresh failed", "error", err)
				continue
			}
			if !b.isActiveRun(runGen) {
				return
			}
			apply(acc)
			b.logger.Debugw("trade_client: periodic refresh applied")
		}
	}
}

// =============================================================================
// Run lifecycle primitives
// =============================================================================

func (b *baseTradeClient) beginRun(runCancel context.CancelFunc) (uint64, bool) {
	b.runMu.Lock()
	defer b.runMu.Unlock()

	if b.closing {
		return 0, false
	}
	b.runGen++
	b.runCancel = runCancel
	b.started = false
	b.starting = true
	b.resetHealthForStart()
	return b.runGen, true
}

func (b *baseTradeClient) failRunStart(runGen uint64) context.CancelFunc {
	b.runMu.Lock()
	defer b.runMu.Unlock()

	if b.runGen != runGen {
		return nil
	}
	cancel := b.runCancel
	b.started = false
	b.starting = false
	b.runCancel = nil
	return cancel
}

func (b *baseTradeClient) finishRunStart(runGen uint64) bool {
	b.runMu.Lock()
	defer b.runMu.Unlock()

	if b.runGen != runGen || b.runCancel == nil || !b.starting {
		return false
	}
	b.starting = false
	b.started = true
	return true
}

func (b *baseTradeClient) closeRun() context.CancelFunc {
	b.runMu.Lock()
	defer b.runMu.Unlock()

	cancel := b.runCancel
	b.runGen++
	b.started = false
	b.starting = false
	b.closing = true
	b.runCancel = nil
	return cancel
}

func (b *baseTradeClient) isActiveRun(runGen uint64) bool {
	b.runMu.RLock()
	defer b.runMu.RUnlock()
	return b.runGen == runGen && b.runCancel != nil && (b.started || b.starting)
}

func (b *baseTradeClient) clearClosingFlag() {
	b.runMu.Lock()
	b.closing = false
	b.runMu.Unlock()
}
