package platform

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/QuantProcessing/exchanges/account"
	"github.com/QuantProcessing/exchanges/bus"
	"github.com/QuantProcessing/exchanges/cache"
	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/portfolio"
	"github.com/QuantProcessing/exchanges/risk"
	"github.com/QuantProcessing/exchanges/strategy"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/shopspring/decimal"
)

const (
	TopicExecution  = "execution"
	TopicMarketData = "market.data"
)

type Config struct {
	Bus       *bus.Bus
	Cache     *cache.Cache
	Risk      *risk.Engine
	Portfolio *portfolio.Portfolio
}

type Node struct {
	mu    sync.RWMutex
	bus   *bus.Bus
	cache *cache.Cache
	risk  *risk.Engine
	pf    *portfolio.Portfolio

	dataClients      []venue.DataClient
	execClients      []venue.ExecutionClient
	dataSubs         map[string]venue.StreamingDataClient
	activeDataSubs   map[string]model.SubscribeMarketData
	pendingDataSubs  map[string]model.SubscribeMarketData
	execByAccount    map[model.AccountID]venue.ExecutionClient
	reconcilers      map[model.AccountID]*account.Reconciler
	factories        map[model.AccountID]*model.OrderFactory
	heldChildren     map[parentOrderKey][]model.SubmitOrder
	orderListMembers map[orderListKey][]model.ClientOrderID
	timers           map[string]context.CancelFunc
	cancel           context.CancelFunc
	wg               sync.WaitGroup
	ready            bool
	lastError        error
}

type parentOrderKey struct {
	accountID     model.AccountID
	clientOrderID model.ClientOrderID
}

type orderListKey struct {
	accountID   model.AccountID
	orderListID model.OrderListID
}

type Health struct {
	Ready     bool
	LastError error
}

func NewNode(cfg Config) *Node {
	b := cfg.Bus
	if b == nil {
		b = bus.New()
	}
	c := cfg.Cache
	if c == nil {
		c = cache.New()
	}
	pf := cfg.Portfolio
	if pf == nil {
		pf = portfolio.New(c)
	}
	return &Node{
		bus:              b,
		cache:            c,
		risk:             cfg.Risk,
		pf:               pf,
		dataSubs:         make(map[string]venue.StreamingDataClient),
		activeDataSubs:   make(map[string]model.SubscribeMarketData),
		pendingDataSubs:  make(map[string]model.SubscribeMarketData),
		execByAccount:    make(map[model.AccountID]venue.ExecutionClient),
		reconcilers:      make(map[model.AccountID]*account.Reconciler),
		factories:        make(map[model.AccountID]*model.OrderFactory),
		heldChildren:     make(map[parentOrderKey][]model.SubmitOrder),
		orderListMembers: make(map[orderListKey][]model.ClientOrderID),
		timers:           make(map[string]context.CancelFunc),
	}
}

func (n *Node) Bus() *bus.Bus       { return n.bus }
func (n *Node) Cache() *cache.Cache { return n.cache }
func (n *Node) Portfolio() *portfolio.Portfolio {
	return n.pf
}

func (n *Node) Clock() strategy.Clock { return strategy.WallClock{} }

func (n *Node) SetTimer(_ context.Context, name string, interval time.Duration) error {
	if err := strategy.ValidateTimer(name, interval); err != nil {
		return err
	}
	timerCtx, cancel := context.WithCancel(context.Background())
	n.mu.Lock()
	if n.timers == nil {
		n.timers = make(map[string]context.CancelFunc)
	}
	old := n.timers[name]
	n.timers[name] = cancel
	n.mu.Unlock()
	if old != nil {
		old()
	}
	n.wg.Add(1)
	go n.runTimer(timerCtx, name, interval)
	return nil
}

func (n *Node) CancelTimer(_ context.Context, name string) error {
	if name == "" {
		return fmt.Errorf("%w: name is required", strategy.ErrInvalidTimer)
	}
	n.mu.Lock()
	cancel := n.timers[name]
	delete(n.timers, name)
	n.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return nil
}

func (n *Node) runTimer(ctx context.Context, name string, interval time.Duration) {
	defer n.wg.Done()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			if err := n.bus.Publish(context.Background(), strategy.TopicTimer, strategy.TimerEvent{Name: name, Timestamp: now}); err != nil {
				n.recordError(err)
			}
		}
	}
}

func (n *Node) OrderFactory(accountID model.AccountID) *model.OrderFactory {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.factories == nil {
		n.factories = make(map[model.AccountID]*model.OrderFactory)
	}
	if n.factories[accountID] == nil {
		n.factories[accountID] = model.NewOrderFactory(accountID)
	}
	return n.factories[accountID]
}

func (n *Node) AddDataClient(client venue.DataClient) error {
	n.dataClients = append(n.dataClients, client)
	return nil
}

func (n *Node) AddExecutionClient(client venue.ExecutionClient) error {
	n.execClients = append(n.execClients, client)
	if client.AccountID() != "" {
		n.execByAccount[client.AccountID()] = client
	}
	return nil
}

func (n *Node) Start(ctx context.Context) error {
	runCtx, cancel := context.WithCancel(context.Background())
	n.cancel = cancel
	n.mu.Lock()
	n.ready = false
	n.lastError = nil
	n.mu.Unlock()
	for _, client := range n.dataClients {
		provider := client.Instruments()
		if provider != nil {
			if err := provider.LoadAll(ctx); err != nil {
				cancel()
				return err
			}
			for _, inst := range provider.List() {
				if err := n.cache.PutInstrument(inst); err != nil {
					cancel()
					return err
				}
			}
		}
		if err := client.Connect(ctx); err != nil {
			cancel()
			return err
		}
		if streaming, ok := client.(venue.StreamingDataClient); ok {
			if err := n.applyPendingMarketData(ctx, client.Venue(), streaming); err != nil {
				cancel()
				return err
			}
			n.wg.Add(1)
			go n.forwardMarketEvents(runCtx, streaming)
		}
	}
	for _, client := range n.execClients {
		reconciler := n.reconcilerFor(client.AccountID())
		if err := client.Connect(ctx); err != nil {
			cancel()
			return err
		}
		if err := n.reconcileExecution(ctx, client, reconciler); err != nil {
			cancel()
			return err
		}
		n.wg.Add(1)
		go n.forwardEvents(runCtx, client, reconciler)
	}
	n.mu.Lock()
	n.ready = true
	n.mu.Unlock()
	return nil
}

func (n *Node) SubscribeMarketData(ctx context.Context, sub model.SubscribeMarketData) error {
	if err := sub.Validate(); err != nil {
		return err
	}
	if _, ok := n.cache.Instrument(sub.InstrumentID); !ok {
		if _, found := n.streamingDataClient(sub.InstrumentID.Venue); !found {
			return fmt.Errorf("%w: no streaming data client for venue %s", model.ErrNotSupported, sub.InstrumentID.Venue)
		}
		n.mu.Lock()
		n.pendingDataSubs[sub.Key()] = sub
		n.mu.Unlock()
		return nil
	}
	client, ok := n.streamingDataClient(sub.InstrumentID.Venue)
	if !ok {
		return fmt.Errorf("%w: no streaming data client for venue %s", model.ErrNotSupported, sub.InstrumentID.Venue)
	}
	if err := client.SubscribeMarketData(ctx, sub); err != nil {
		return err
	}
	n.mu.Lock()
	n.dataSubs[sub.Key()] = client
	n.activeDataSubs[sub.Key()] = sub
	n.mu.Unlock()
	return nil
}

func (n *Node) SubscribeTicker(ctx context.Context, instrumentID model.InstrumentID) error {
	return n.SubscribeMarketData(ctx, model.SubscribeMarketData{
		InstrumentID: instrumentID,
		Type:         model.MarketDataTypeTicker,
	})
}

func (n *Node) SubscribeTradeTicks(ctx context.Context, instrumentID model.InstrumentID) error {
	return n.SubscribeMarketData(ctx, model.SubscribeMarketData{
		InstrumentID: instrumentID,
		Type:         model.MarketDataTypeTradeTick,
	})
}

func (n *Node) SubscribeQuoteTicks(ctx context.Context, instrumentID model.InstrumentID) error {
	return n.SubscribeMarketData(ctx, model.SubscribeMarketData{
		InstrumentID: instrumentID,
		Type:         model.MarketDataTypeQuoteTick,
	})
}

func (n *Node) SubscribeBars(ctx context.Context, barType model.BarType) error {
	barType = barType.Canonical()
	return n.SubscribeMarketData(ctx, model.SubscribeMarketData{
		InstrumentID: barType.InstrumentID,
		Type:         model.MarketDataTypeBar,
		BarType:      barType,
	})
}

func (n *Node) SubscribeOrderBookDepth(ctx context.Context, instrumentID model.InstrumentID, depth int) error {
	return n.SubscribeMarketData(ctx, model.SubscribeMarketData{
		InstrumentID: instrumentID,
		Type:         model.MarketDataTypeOrderBook,
		Depth:        depth,
	})
}

func (n *Node) UnsubscribeMarketData(ctx context.Context, sub model.SubscribeMarketData) error {
	if err := sub.Validate(); err != nil {
		return err
	}
	n.mu.Lock()
	if _, ok := n.pendingDataSubs[sub.Key()]; ok {
		delete(n.pendingDataSubs, sub.Key())
		n.mu.Unlock()
		return nil
	}
	n.mu.Unlock()
	n.mu.RLock()
	client, ok := n.dataSubs[sub.Key()]
	n.mu.RUnlock()
	if !ok {
		var found bool
		client, found = n.streamingDataClient(sub.InstrumentID.Venue)
		if !found {
			return fmt.Errorf("%w: no streaming data client for venue %s", model.ErrNotSupported, sub.InstrumentID.Venue)
		}
	}
	if err := client.UnsubscribeMarketData(ctx, sub); err != nil {
		return err
	}
	n.mu.Lock()
	delete(n.dataSubs, sub.Key())
	delete(n.activeDataSubs, sub.Key())
	n.mu.Unlock()
	return nil
}

func (n *Node) UnsubscribeTicker(ctx context.Context, instrumentID model.InstrumentID) error {
	return n.UnsubscribeMarketData(ctx, model.SubscribeMarketData{
		InstrumentID: instrumentID,
		Type:         model.MarketDataTypeTicker,
	})
}

func (n *Node) UnsubscribeTradeTicks(ctx context.Context, instrumentID model.InstrumentID) error {
	return n.UnsubscribeMarketData(ctx, model.SubscribeMarketData{
		InstrumentID: instrumentID,
		Type:         model.MarketDataTypeTradeTick,
	})
}

func (n *Node) UnsubscribeQuoteTicks(ctx context.Context, instrumentID model.InstrumentID) error {
	return n.UnsubscribeMarketData(ctx, model.SubscribeMarketData{
		InstrumentID: instrumentID,
		Type:         model.MarketDataTypeQuoteTick,
	})
}

func (n *Node) UnsubscribeBars(ctx context.Context, barType model.BarType) error {
	barType = barType.Canonical()
	return n.UnsubscribeMarketData(ctx, model.SubscribeMarketData{
		InstrumentID: barType.InstrumentID,
		Type:         model.MarketDataTypeBar,
		BarType:      barType,
	})
}

func (n *Node) UnsubscribeOrderBookDepth(ctx context.Context, instrumentID model.InstrumentID, depth int) error {
	return n.UnsubscribeMarketData(ctx, model.SubscribeMarketData{
		InstrumentID: instrumentID,
		Type:         model.MarketDataTypeOrderBook,
		Depth:        depth,
	})
}

func (n *Node) SubmitOrder(ctx context.Context, order model.SubmitOrder) (model.OrderStatusReport, error) {
	if err := order.Validate(); err != nil {
		_ = n.publishOrderDenied(ctx, order, err)
		return model.OrderStatusReport{}, err
	}
	if n.risk != nil {
		if err := n.risk.Check(order); err != nil {
			_ = n.publishOrderDenied(ctx, order, err)
			return model.OrderStatusReport{}, err
		}
	}
	client, ok := n.executionClient(order.AccountID)
	if !ok {
		return model.OrderStatusReport{}, fmt.Errorf("%w: no execution client for account %s", model.ErrNotSupported, order.AccountID)
	}
	reconciler := n.reconcilerFor(order.AccountID)
	if err := n.publishOrderLifecycle(ctx, reconciler, orderSubmittedLifecycle(order)); err != nil {
		return model.OrderStatusReport{}, err
	}
	report, err := client.SubmitOrder(ctx, order)
	if err != nil {
		_ = n.publishOrderLifecycle(ctx, reconciler, orderRejectedLifecycle(order, err))
		return model.OrderStatusReport{}, err
	}
	report = fillSubmittedReport(report, order)
	if err := n.applyAndPublish(ctx, reconciler, model.ExecutionEvent{Order: &report}); err != nil {
		return model.OrderStatusReport{}, err
	}
	if err := n.publishOrderLifecycle(ctx, reconciler, orderLifecycleFromReport(report, model.OrderEventAccepted, model.OrderStatusSubmitted, "")); err != nil {
		return model.OrderStatusReport{}, err
	}
	return report, nil
}

func (n *Node) SubmitOrderList(ctx context.Context, list model.OrderList) ([]model.OrderStatusReport, error) {
	if err := list.Validate(); err != nil {
		return nil, err
	}
	for _, order := range list.Orders {
		if order.ParentClientOrderID != "" {
			continue
		}
		if n.risk != nil {
			if err := n.risk.Check(order); err != nil {
				_ = n.publishOrderDenied(ctx, order, err)
				return nil, err
			}
		}
		if _, ok := n.executionClient(order.AccountID); !ok {
			return nil, fmt.Errorf("%w: no execution client for account %s", model.ErrNotSupported, order.AccountID)
		}
	}
	n.indexOrderList(list)
	reports := make([]model.OrderStatusReport, 0, len(list.Orders))
	for _, order := range list.Orders {
		if order.ParentClientOrderID != "" {
			continue
		}
		report, err := n.SubmitOrder(ctx, order)
		if err != nil {
			return reports, err
		}
		reports = append(reports, report)
	}
	return reports, nil
}

func (n *Node) ModifyOrder(ctx context.Context, modify model.ModifyOrder) (model.OrderStatusReport, error) {
	if err := modify.Validate(); err != nil {
		_ = n.publishOrderModifyRejected(ctx, modify, model.OrderStatusAccepted, nil, err)
		return model.OrderStatusReport{}, err
	}
	existing, ok := n.findOrder(modify)
	if !ok {
		err := fmt.Errorf("%w: order not found", model.ErrInvalidOrder)
		_ = n.publishOrderModifyRejected(ctx, modify, model.OrderStatusAccepted, nil, err)
		return model.OrderStatusReport{}, err
	}
	modify = fillModifyIdentity(modify, existing)
	updated, _, err := model.ApplyOrderModification(existing, modify)
	if err != nil {
		_ = n.publishOrderModifyRejected(ctx, modify, model.OrderStatusAccepted, &existing, err)
		return model.OrderStatusReport{}, err
	}

	reconciler := n.reconcilerFor(modify.AccountID)
	pending := existing
	pending.Status = model.OrderStatusPendingUpdate
	pending.LastUpdatedTime = time.Now()
	if err := n.applyAndPublish(ctx, reconciler, model.ExecutionEvent{Order: &pending}); err != nil {
		return model.OrderStatusReport{}, err
	}
	if err := n.publishOrderLifecycle(ctx, reconciler, orderLifecycleFromReport(pending, model.OrderEventPendingUpdate, existing.Status, "")); err != nil {
		return model.OrderStatusReport{}, err
	}

	if n.risk != nil {
		if err := n.risk.Check(submitFromOrderReport(updated)); err != nil {
			_ = n.restoreAndRejectModify(ctx, reconciler, existing, modify, err)
			return model.OrderStatusReport{}, err
		}
	}
	client, ok := n.executionClient(modify.AccountID)
	if !ok {
		err := fmt.Errorf("%w: no execution client for account %s", model.ErrNotSupported, modify.AccountID)
		_ = n.restoreAndRejectModify(ctx, reconciler, existing, modify, err)
		return model.OrderStatusReport{}, err
	}
	modifier, ok := client.(venue.OrderModifier)
	if !ok {
		err := fmt.Errorf("%w: execution client does not support order modification", model.ErrNotSupported)
		_ = n.restoreAndRejectModify(ctx, reconciler, existing, modify, err)
		return model.OrderStatusReport{}, err
	}
	report, err := modifier.ModifyOrder(ctx, modify)
	if err != nil {
		_ = n.restoreAndRejectModify(ctx, reconciler, existing, modify, err)
		return model.OrderStatusReport{}, err
	}
	report = fillModifiedReport(report, updated)
	if err := n.applyAndPublish(ctx, reconciler, model.ExecutionEvent{Order: &report}); err != nil {
		return model.OrderStatusReport{}, err
	}
	if err := n.publishOrderLifecycle(ctx, reconciler, orderLifecycleFromReport(report, model.OrderEventUpdated, model.OrderStatusPendingUpdate, "")); err != nil {
		return model.OrderStatusReport{}, err
	}
	return report, nil
}

func (n *Node) publishOrderDenied(ctx context.Context, order model.SubmitOrder, cause error) error {
	if order.AccountID == "" || order.ClientOrderID == "" {
		return nil
	}
	lifecycle := model.OrderLifecycleEvent{
		AccountID:     order.AccountID,
		InstrumentID:  order.InstrumentID,
		ClientOrderID: order.ClientOrderID,
		Kind:          model.OrderEventDenied,
		Status:        model.OrderStatusDenied,
	}
	if cause != nil {
		lifecycle.Reason = cause.Error()
	}
	event := model.ExecutionEvent{Lifecycle: &lifecycle}
	if err := event.Validate(); err != nil {
		return nil
	}
	if err := n.reconcilerFor(order.AccountID).Apply(event); err != nil {
		return err
	}
	return n.bus.Publish(ctx, TopicExecution, event)
}

func (n *Node) publishOrderModifyRejected(ctx context.Context, modify model.ModifyOrder, previous model.OrderStatus, report *model.OrderStatusReport, cause error) error {
	lifecycle := model.OrderLifecycleEvent{
		AccountID:      modify.AccountID,
		InstrumentID:   modify.InstrumentID,
		OrderID:        modify.OrderID,
		ClientOrderID:  modify.ClientOrderID,
		VenueOrderID:   modify.VenueOrderID,
		Kind:           model.OrderEventModifyRejected,
		PreviousStatus: previous,
		Status:         model.OrderStatusAccepted,
		Report:         report,
	}
	if cause != nil {
		lifecycle.Reason = cause.Error()
	}
	event := model.ExecutionEvent{Lifecycle: &lifecycle}
	if err := event.Validate(); err != nil {
		return nil
	}
	reconciler := n.reconcilerFor(modify.AccountID)
	if err := reconciler.Apply(event); err != nil {
		return err
	}
	return n.bus.Publish(ctx, TopicExecution, event)
}

func (n *Node) publishOrderCancelRejected(ctx context.Context, cancel model.CancelOrder, previous model.OrderStatus, report *model.OrderStatusReport, cause error) error {
	lifecycle := model.OrderLifecycleEvent{
		AccountID:      cancel.AccountID,
		InstrumentID:   cancel.InstrumentID,
		OrderID:        cancel.OrderID,
		ClientOrderID:  cancel.ClientOrderID,
		Kind:           model.OrderEventCancelRejected,
		PreviousStatus: previous,
		Status:         model.OrderStatusAccepted,
		Report:         report,
	}
	if cause != nil {
		lifecycle.Reason = cause.Error()
	}
	event := model.ExecutionEvent{Lifecycle: &lifecycle}
	if err := event.Validate(); err != nil {
		return nil
	}
	reconciler := n.reconcilerFor(cancel.AccountID)
	if err := reconciler.Apply(event); err != nil {
		return err
	}
	return n.bus.Publish(ctx, TopicExecution, event)
}

func (n *Node) publishOrderLifecycle(ctx context.Context, reconciler *account.Reconciler, lifecycle model.OrderLifecycleEvent) error {
	event := model.ExecutionEvent{Lifecycle: &lifecycle}
	if err := reconciler.Apply(event); err != nil {
		return err
	}
	return n.bus.Publish(ctx, TopicExecution, event)
}

func (n *Node) restoreAndRejectModify(ctx context.Context, reconciler *account.Reconciler, existing model.OrderStatusReport, modify model.ModifyOrder, cause error) error {
	restored := existing
	restored.Status = model.OrderStatusAccepted
	if err := n.applyAndPublish(ctx, reconciler, model.ExecutionEvent{Order: &restored}); err != nil {
		return err
	}
	return n.publishOrderModifyRejected(ctx, modify, model.OrderStatusPendingUpdate, &restored, cause)
}

func (n *Node) restoreAndRejectCancel(ctx context.Context, reconciler *account.Reconciler, existing model.OrderStatusReport, cancel model.CancelOrder, cause error) error {
	restored := existing
	restored.Status = model.OrderStatusAccepted
	if err := n.applyAndPublish(ctx, reconciler, model.ExecutionEvent{Order: &restored}); err != nil {
		return err
	}
	return n.publishOrderCancelRejected(ctx, cancel, model.OrderStatusPendingCancel, &restored, cause)
}

func (n *Node) CancelOrder(ctx context.Context, cancel model.CancelOrder) (model.OrderStatusReport, error) {
	if err := cancel.Validate(); err != nil {
		_ = n.publishOrderCancelRejected(ctx, cancel, model.OrderStatusAccepted, nil, err)
		return model.OrderStatusReport{}, err
	}
	existing, ok := n.findCancelableOrder(cancel)
	if !ok {
		err := fmt.Errorf("%w: order not found", model.ErrInvalidOrder)
		_ = n.publishOrderCancelRejected(ctx, cancel, model.OrderStatusAccepted, nil, err)
		return model.OrderStatusReport{}, err
	}
	if !existing.Status.IsOpen() || existing.Status == model.OrderStatusPendingCancel {
		err := fmt.Errorf("%w: order is not cancelable", model.ErrInvalidOrder)
		_ = n.publishOrderCancelRejected(ctx, cancel, model.OrderStatusAccepted, &existing, err)
		return model.OrderStatusReport{}, err
	}
	cancel = fillCancelIdentity(cancel, existing)
	reconciler := n.reconcilerFor(cancel.AccountID)
	pending := existing
	pending.Status = model.OrderStatusPendingCancel
	pending.LastUpdatedTime = time.Now()
	if err := n.applyAndPublish(ctx, reconciler, model.ExecutionEvent{Order: &pending}); err != nil {
		return model.OrderStatusReport{}, err
	}
	if err := n.publishOrderLifecycle(ctx, reconciler, orderLifecycleFromReport(pending, model.OrderEventPendingCancel, existing.Status, "")); err != nil {
		return model.OrderStatusReport{}, err
	}
	client, ok := n.executionClient(cancel.AccountID)
	if !ok {
		err := fmt.Errorf("%w: no execution client for account %s", model.ErrNotSupported, cancel.AccountID)
		_ = n.restoreAndRejectCancel(ctx, reconciler, existing, cancel, err)
		return model.OrderStatusReport{}, err
	}
	report, err := client.CancelOrder(ctx, cancel)
	if err != nil {
		_ = n.restoreAndRejectCancel(ctx, reconciler, existing, cancel, err)
		return model.OrderStatusReport{}, err
	}
	report = fillCanceledReport(report, cancel, existing)
	if err := n.applyAndPublish(ctx, n.reconcilerFor(cancel.AccountID), model.ExecutionEvent{Order: &report}); err != nil {
		return model.OrderStatusReport{}, err
	}
	if err := n.publishOrderLifecycle(ctx, reconciler, orderLifecycleFromReport(report, model.OrderEventCanceled, model.OrderStatusPendingCancel, "")); err != nil {
		return model.OrderStatusReport{}, err
	}
	return report, nil
}

func (n *Node) BatchCancelOrders(ctx context.Context, batch model.BatchCancelOrders) ([]model.OrderStatusReport, error) {
	if err := batch.Validate(); err != nil {
		return nil, err
	}
	reports := make([]model.OrderStatusReport, 0, len(batch.Cancels))
	var batchErr error
	for _, cancel := range batch.Cancels {
		cancel = fillBatchCancelIdentity(cancel, batch.AccountID, batch.InstrumentID)
		report, err := n.CancelOrder(ctx, cancel)
		if err != nil {
			batchErr = errors.Join(batchErr, err)
			continue
		}
		reports = append(reports, report)
	}
	return reports, batchErr
}

func (n *Node) CancelAllOrders(ctx context.Context, cancelAll model.CancelAllOrders) ([]model.OrderStatusReport, error) {
	if err := cancelAll.Validate(); err != nil {
		return nil, err
	}
	orders := n.cache.OpenOrders(cancelAll.AccountID)
	batch := model.BatchCancelOrders{
		AccountID:    cancelAll.AccountID,
		InstrumentID: cancelAll.InstrumentID,
	}
	for _, order := range orders {
		if !cancelAll.MatchesOrder(order) {
			continue
		}
		batch.Cancels = append(batch.Cancels, model.CancelOrder{
			AccountID:     order.AccountID,
			InstrumentID:  order.InstrumentID,
			OrderID:       order.OrderID,
			ClientOrderID: order.ClientOrderID,
		})
	}
	if len(batch.Cancels) == 0 {
		return nil, nil
	}
	return n.BatchCancelOrders(ctx, batch)
}

func (n *Node) QueryOrder(ctx context.Context, query model.QueryOrder) (model.OrderStatusReport, error) {
	if err := query.Validate(); err != nil {
		return model.OrderStatusReport{}, err
	}
	if order, ok := n.findQueriedOrder(query); ok {
		return order, nil
	}
	client, ok := n.executionClient(query.AccountID)
	if !ok {
		return model.OrderStatusReport{}, fmt.Errorf("%w: no execution client for account %s", model.ErrNotSupported, query.AccountID)
	}
	if querier, ok := client.(venue.OrderQuerier); ok {
		report, err := querier.QueryOrder(ctx, query)
		if err != nil {
			return model.OrderStatusReport{}, err
		}
		report = fillQueriedReport(report, query)
		if err := n.applyAndPublish(ctx, n.reconcilerFor(query.AccountID), model.ExecutionEvent{Order: &report}); err != nil {
			return model.OrderStatusReport{}, err
		}
		return report, nil
	}
	reports, err := client.GenerateOrderStatusReports(ctx, query.InstrumentID)
	if err != nil {
		return model.OrderStatusReport{}, err
	}
	for _, report := range reports {
		report = fillQueriedReport(report, query)
		if !matchesOrderQuery(report, query) {
			continue
		}
		if err := n.applyAndPublish(ctx, n.reconcilerFor(query.AccountID), model.ExecutionEvent{Order: &report}); err != nil {
			return model.OrderStatusReport{}, err
		}
		return report, nil
	}
	return model.OrderStatusReport{}, fmt.Errorf("%w: order not found", model.ErrInvalidOrder)
}

func (n *Node) QueryAccount(ctx context.Context, query model.QueryAccount) (model.AccountSnapshot, error) {
	if err := query.Validate(); err != nil {
		return model.AccountSnapshot{}, err
	}
	client, ok := n.executionClient(query.AccountID)
	if !ok {
		return model.AccountSnapshot{}, fmt.Errorf("%w: no execution client for account %s", model.ErrNotSupported, query.AccountID)
	}
	snapshot, err := client.QueryAccount(ctx)
	if err != nil {
		return model.AccountSnapshot{}, err
	}
	if snapshot.AccountID == "" {
		snapshot.AccountID = query.AccountID
	}
	if err := n.applyAndPublish(ctx, n.reconcilerFor(snapshot.AccountID), model.ExecutionEvent{Account: &snapshot}); err != nil {
		return model.AccountSnapshot{}, err
	}
	return snapshot, nil
}

func (n *Node) forwardEvents(ctx context.Context, client venue.ExecutionClient, reconciler *account.Reconciler) {
	defer n.wg.Done()
	events := client.Events()
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-events:
			if !ok {
				if err := n.recoverExecutionStream(context.Background(), client, reconciler); err != nil {
					n.recordError(err)
					return
				}
				next := client.Events()
				if next == events {
					n.recordError(fmt.Errorf("%w: execution client reused closed event channel", model.ErrNotSupported))
					return
				}
				events = next
				continue
			}
			if err := n.applyAndPublish(context.Background(), reconciler, event); err != nil {
				n.recordError(err)
			}
		}
	}
}

func (n *Node) recoverExecutionStream(ctx context.Context, client venue.ExecutionClient, reconciler *account.Reconciler) error {
	if err := client.Connect(ctx); err != nil {
		return err
	}
	if resubscriber, ok := client.(venue.ExecutionResubscriber); ok {
		if err := resubscriber.ResubscribeExecution(ctx); err != nil {
			return err
		}
	}
	return n.reconcileExecution(ctx, client, reconciler)
}

func (n *Node) reconcileExecution(ctx context.Context, client venue.ExecutionClient, reconciler *account.Reconciler) error {
	snapshot, err := client.QueryAccount(ctx)
	if err != nil {
		return err
	}
	if err := n.applyAndPublish(ctx, reconciler, model.ExecutionEvent{Account: &snapshot}); err != nil {
		return err
	}
	for _, inst := range n.cache.Instruments() {
		if err := n.reconcileInstrument(ctx, client, reconciler, inst.ID); err != nil {
			return err
		}
	}
	return nil
}

func (n *Node) reconcileInstrument(ctx context.Context, client venue.ExecutionClient, reconciler *account.Reconciler, instrumentID model.InstrumentID) error {
	reports, err := client.GenerateOrderStatusReports(ctx, instrumentID)
	if err != nil && !errors.Is(err, model.ErrNotSupported) {
		return err
	}
	for _, report := range reports {
		report := report
		if err := n.applyAndPublish(ctx, reconciler, model.ExecutionEvent{Order: &report}); err != nil {
			return err
		}
	}
	if fillGenerator, ok := client.(venue.FillReportGenerator); ok {
		fills, err := fillGenerator.GenerateFillReports(ctx, instrumentID)
		if err != nil && !errors.Is(err, model.ErrNotSupported) {
			return err
		}
		for _, fill := range fills {
			fill := fill
			if err := n.applyAndPublish(ctx, reconciler, model.ExecutionEvent{Fill: &fill}); err != nil {
				return err
			}
		}
	}
	var positions []model.PositionStatusReport
	positionSnapshotReady := false
	if positionGenerator, ok := client.(venue.PositionStatusReportGenerator); ok {
		generated, err := positionGenerator.GeneratePositionStatusReports(ctx, instrumentID)
		if err != nil && !errors.Is(err, model.ErrNotSupported) {
			return err
		}
		if err == nil {
			positions = generated
			positionSnapshotReady = true
		}
		for _, position := range positions {
			position := position
			if err := n.applyAndPublish(ctx, reconciler, model.ExecutionEvent{Position: &position}); err != nil {
				return err
			}
		}
	}
	if positionSnapshotReady {
		missingPositions, err := reconciler.MissingPositionReports(client.AccountID(), instrumentID, positions)
		if err != nil {
			return err
		}
		for _, position := range missingPositions {
			position := position
			if err := n.applyAndPublish(ctx, reconciler, model.ExecutionEvent{Position: &position}); err != nil {
				return err
			}
		}
	}
	missing, err := reconciler.ReconcileMissingOpenOrders(client.AccountID(), instrumentID, reports, model.OrderStatusCanceled)
	if err != nil {
		return err
	}
	for _, report := range missing {
		report := report
		event := model.ExecutionEvent{Order: &report}
		if err := n.bus.Publish(ctx, TopicExecution, event); err != nil {
			return err
		}
		if err := n.handleOrderListProgress(ctx, report); err != nil {
			return err
		}
	}
	return nil
}

func (n *Node) forwardMarketEvents(ctx context.Context, client venue.StreamingDataClient) {
	defer n.wg.Done()
	events := client.Events()
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-events:
			if !ok {
				select {
				case <-ctx.Done():
					return
				default:
				}
				if err := n.recoverMarketDataStream(context.Background(), client); err != nil {
					n.recordError(err)
					return
				}
				next := client.Events()
				if next == events {
					n.recordError(fmt.Errorf("%w: data client reused closed event channel", model.ErrNotSupported))
					return
				}
				events = next
				continue
			}
			if err := n.applyMarketAndPublish(context.Background(), event); err != nil {
				n.recordError(err)
			}
		}
	}
}

func (n *Node) recoverMarketDataStream(ctx context.Context, client venue.StreamingDataClient) error {
	if connectable, ok := client.(interface{ Connect(context.Context) error }); ok {
		if err := connectable.Connect(ctx); err != nil {
			return err
		}
	}
	return n.resubscribeMarketData(ctx, client)
}

func (n *Node) resubscribeMarketData(ctx context.Context, client venue.StreamingDataClient) error {
	n.mu.RLock()
	subs := make([]model.SubscribeMarketData, 0)
	for key, subClient := range n.dataSubs {
		if subClient == client {
			subs = append(subs, n.activeDataSubs[key])
		}
	}
	n.mu.RUnlock()
	for _, sub := range subs {
		if err := client.SubscribeMarketData(ctx, sub); err != nil {
			return err
		}
	}
	return nil
}

func (n *Node) applyPendingMarketData(ctx context.Context, venueID model.Venue, client venue.StreamingDataClient) error {
	n.mu.RLock()
	pending := make([]model.SubscribeMarketData, 0, len(n.pendingDataSubs))
	for _, sub := range n.pendingDataSubs {
		if sub.InstrumentID.Venue == venueID {
			pending = append(pending, sub)
		}
	}
	n.mu.RUnlock()
	for _, sub := range pending {
		if _, ok := n.cache.Instrument(sub.InstrumentID); !ok {
			return fmt.Errorf("%w: instrument %s is not loaded", model.ErrInvalidInstrumentID, sub.InstrumentID)
		}
		if err := client.SubscribeMarketData(ctx, sub); err != nil {
			return err
		}
		n.mu.Lock()
		n.dataSubs[sub.Key()] = client
		n.activeDataSubs[sub.Key()] = sub
		delete(n.pendingDataSubs, sub.Key())
		n.mu.Unlock()
	}
	return nil
}

func (n *Node) applyAndPublish(ctx context.Context, reconciler *account.Reconciler, event model.ExecutionEvent) error {
	positionLifecycle := n.derivePositionLifecycle(event)
	if err := reconciler.Apply(event); err != nil {
		return err
	}
	if n.pf != nil && event.Fill != nil {
		if err := n.pf.ApplyFill(*event.Fill); err != nil {
			return err
		}
	}
	if err := n.bus.Publish(ctx, TopicExecution, event); err != nil {
		return err
	}
	if event.Order != nil {
		return n.handleOrderListProgress(ctx, *event.Order)
	}
	if event.Fill != nil {
		order, ok := n.cache.Order(event.Fill.AccountID, event.Fill.OrderID)
		if ok {
			if err := n.handleOrderListProgress(ctx, order); err != nil {
				return err
			}
		}
	}
	if positionLifecycle != nil {
		lifecycleEvent := model.ExecutionEvent{PositionLifecycle: positionLifecycle}
		if err := reconciler.Apply(lifecycleEvent); err != nil {
			return err
		}
		if err := n.bus.Publish(ctx, TopicExecution, lifecycleEvent); err != nil {
			return err
		}
	}
	return nil
}

func (n *Node) applyMarketAndPublish(ctx context.Context, event model.MarketEvent) error {
	if n.pf != nil {
		if err := n.pf.ApplyMarketEvent(event); err != nil {
			return err
		}
	} else {
		if err := n.cache.PutMarketEvent(event); err != nil {
			return err
		}
	}
	return n.bus.Publish(ctx, TopicMarketData, event)
}

func (n *Node) derivePositionLifecycle(event model.ExecutionEvent) *model.PositionLifecycleEvent {
	if event.Position == nil {
		return nil
	}
	var previous *model.PositionStatusReport
	if existing, ok := n.cache.Position(event.Position.AccountID, event.Position.PositionID); ok {
		previous = &existing
	}
	lifecycle, ok := model.NewPositionLifecycleEvent(previous, *event.Position)
	if !ok {
		return nil
	}
	return &lifecycle
}

func (n *Node) executionClient(accountID model.AccountID) (venue.ExecutionClient, bool) {
	n.mu.RLock()
	defer n.mu.RUnlock()
	client, ok := n.execByAccount[accountID]
	return client, ok
}

func (n *Node) streamingDataClient(venueID model.Venue) (venue.StreamingDataClient, bool) {
	n.mu.RLock()
	defer n.mu.RUnlock()
	for _, client := range n.dataClients {
		if client.Venue() != venueID {
			continue
		}
		streaming, ok := client.(venue.StreamingDataClient)
		if ok {
			return streaming, true
		}
	}
	return nil, false
}

func (n *Node) reconcilerFor(accountID model.AccountID) *account.Reconciler {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.reconcilers == nil {
		n.reconcilers = make(map[model.AccountID]*account.Reconciler)
	}
	if n.reconcilers[accountID] == nil {
		n.reconcilers[accountID] = account.NewReconciler(n.cache)
	}
	return n.reconcilers[accountID]
}

func (n *Node) findOrder(modify model.ModifyOrder) (model.OrderStatusReport, bool) {
	if modify.OrderID != "" {
		if order, ok := n.cache.Order(modify.AccountID, modify.OrderID); ok {
			return order, true
		}
	}
	if modify.ClientOrderID != "" {
		if order, ok := n.cache.OrderByClientID(modify.AccountID, modify.ClientOrderID); ok {
			return order, true
		}
	}
	if modify.VenueOrderID != "" {
		if order, ok := n.cache.OrderByVenueID(modify.AccountID, modify.VenueOrderID); ok {
			return order, true
		}
	}
	return model.OrderStatusReport{}, false
}

func (n *Node) findCancelableOrder(cancel model.CancelOrder) (model.OrderStatusReport, bool) {
	if cancel.OrderID != "" {
		if order, ok := n.cache.Order(cancel.AccountID, cancel.OrderID); ok {
			return order, true
		}
	}
	if cancel.ClientOrderID != "" {
		if order, ok := n.cache.OrderByClientID(cancel.AccountID, cancel.ClientOrderID); ok {
			return order, true
		}
	}
	return model.OrderStatusReport{}, false
}

func (n *Node) findQueriedOrder(query model.QueryOrder) (model.OrderStatusReport, bool) {
	if query.OrderID != "" {
		if order, ok := n.cache.Order(query.AccountID, query.OrderID); ok {
			return order, true
		}
	}
	if query.ClientOrderID != "" {
		if order, ok := n.cache.OrderByClientID(query.AccountID, query.ClientOrderID); ok {
			return order, true
		}
	}
	if query.VenueOrderID != "" {
		if order, ok := n.cache.OrderByVenueID(query.AccountID, query.VenueOrderID); ok {
			return order, true
		}
	}
	return model.OrderStatusReport{}, false
}

func fillSubmittedReport(report model.OrderStatusReport, order model.SubmitOrder) model.OrderStatusReport {
	if report.AccountID == "" {
		report.AccountID = order.AccountID
	}
	if report.InstrumentID == (model.InstrumentID{}) {
		report.InstrumentID = order.InstrumentID
	}
	if report.ClientOrderID == "" {
		report.ClientOrderID = order.ClientOrderID
	}
	if report.OrderListID == "" {
		report.OrderListID = order.OrderListID
	}
	if report.ParentClientOrderID == "" {
		report.ParentClientOrderID = order.ParentClientOrderID
	}
	if report.Contingency == "" {
		report.Contingency = order.Contingency
	}
	if report.Side == "" {
		report.Side = order.Side
	}
	if report.Type == "" {
		report.Type = order.Type
	}
	if report.Quantity.IsZero() {
		report.Quantity = order.Quantity
	}
	if report.Price.IsZero() {
		report.Price = order.Price
	}
	if report.TriggerPrice.IsZero() {
		report.TriggerPrice = order.TriggerPrice
	}
	if report.ActivationPrice.IsZero() {
		report.ActivationPrice = order.ActivationPrice
	}
	if report.TrailingOffset.IsZero() {
		report.TrailingOffset = order.TrailingOffset
	}
	if report.TimeInForce == "" {
		report.TimeInForce = order.TimeInForce
	}
	if report.ExpireTime.IsZero() {
		report.ExpireTime = order.ExpireTime
	}
	report.PostOnly = report.PostOnly || order.PostOnly
	report.ReduceOnly = report.ReduceOnly || order.ReduceOnly
	if report.LeavesQuantity.IsZero() && report.Quantity.IsPositive() && report.Status.IsOpen() {
		report.LeavesQuantity = report.Quantity.Sub(report.FilledQuantity)
	}
	return report
}

func fillQueriedReport(report model.OrderStatusReport, query model.QueryOrder) model.OrderStatusReport {
	if report.AccountID == "" {
		report.AccountID = query.AccountID
	}
	if report.InstrumentID == (model.InstrumentID{}) {
		report.InstrumentID = query.InstrumentID
	}
	if report.OrderID == "" {
		report.OrderID = query.OrderID
	}
	if report.ClientOrderID == "" {
		report.ClientOrderID = query.ClientOrderID
	}
	if report.VenueOrderID == "" {
		report.VenueOrderID = query.VenueOrderID
	}
	return report
}

func matchesOrderQuery(report model.OrderStatusReport, query model.QueryOrder) bool {
	if query.OrderID != "" && report.OrderID != query.OrderID {
		return false
	}
	if query.ClientOrderID != "" && report.ClientOrderID != query.ClientOrderID {
		return false
	}
	if query.VenueOrderID != "" && report.VenueOrderID != query.VenueOrderID {
		return false
	}
	return report.AccountID == query.AccountID && report.InstrumentID == query.InstrumentID
}

func fillModifyIdentity(modify model.ModifyOrder, existing model.OrderStatusReport) model.ModifyOrder {
	if modify.OrderID == "" {
		modify.OrderID = existing.OrderID
	}
	if modify.ClientOrderID == "" {
		modify.ClientOrderID = existing.ClientOrderID
	}
	if modify.VenueOrderID == "" {
		modify.VenueOrderID = existing.VenueOrderID
	}
	if modify.InstrumentID == (model.InstrumentID{}) {
		modify.InstrumentID = existing.InstrumentID
	}
	return modify
}

func fillCancelIdentity(cancel model.CancelOrder, existing model.OrderStatusReport) model.CancelOrder {
	if cancel.OrderID == "" {
		cancel.OrderID = existing.OrderID
	}
	if cancel.ClientOrderID == "" {
		cancel.ClientOrderID = existing.ClientOrderID
	}
	if cancel.InstrumentID == (model.InstrumentID{}) {
		cancel.InstrumentID = existing.InstrumentID
	}
	return cancel
}

func fillBatchCancelIdentity(cancel model.CancelOrder, accountID model.AccountID, instrumentID model.InstrumentID) model.CancelOrder {
	if cancel.AccountID == "" {
		cancel.AccountID = accountID
	}
	if cancel.InstrumentID == (model.InstrumentID{}) {
		cancel.InstrumentID = instrumentID
	}
	return cancel
}

func submitFromOrderReport(report model.OrderStatusReport) model.SubmitOrder {
	return model.SubmitOrder{
		AccountID:           report.AccountID,
		InstrumentID:        report.InstrumentID,
		OrderListID:         report.OrderListID,
		ParentClientOrderID: report.ParentClientOrderID,
		ClientOrderID:       report.ClientOrderID,
		Side:                report.Side,
		Type:                report.Type,
		Contingency:         report.Contingency,
		TimeInForce:         report.TimeInForce,
		Quantity:            report.Quantity,
		Price:               report.Price,
		TriggerPrice:        report.TriggerPrice,
		ActivationPrice:     report.ActivationPrice,
		TrailingOffset:      report.TrailingOffset,
		PostOnly:            report.PostOnly,
		ReduceOnly:          report.ReduceOnly,
		ExpireTime:          report.ExpireTime,
	}
}

func fillModifiedReport(report model.OrderStatusReport, updated model.OrderStatusReport) model.OrderStatusReport {
	if report.AccountID == "" {
		report.AccountID = updated.AccountID
	}
	if report.InstrumentID == (model.InstrumentID{}) {
		report.InstrumentID = updated.InstrumentID
	}
	if report.OrderListID == "" {
		report.OrderListID = updated.OrderListID
	}
	if report.ParentClientOrderID == "" {
		report.ParentClientOrderID = updated.ParentClientOrderID
	}
	if report.OrderID == "" {
		report.OrderID = updated.OrderID
	}
	if report.VenueOrderID == "" {
		report.VenueOrderID = updated.VenueOrderID
	}
	if report.ClientOrderID == "" {
		report.ClientOrderID = updated.ClientOrderID
	}
	if report.Status == "" {
		report.Status = model.OrderStatusAccepted
	}
	if report.Side == "" {
		report.Side = updated.Side
	}
	if report.Type == "" {
		report.Type = updated.Type
	}
	if report.Contingency == "" {
		report.Contingency = updated.Contingency
	}
	if report.Quantity.IsZero() {
		report.Quantity = updated.Quantity
	}
	if report.FilledQuantity.IsZero() {
		report.FilledQuantity = updated.FilledQuantity
	}
	if report.LeavesQuantity.IsZero() && report.Quantity.IsPositive() && report.Status.IsOpen() {
		report.LeavesQuantity = report.Quantity.Sub(report.FilledQuantity)
		if report.LeavesQuantity.IsNegative() {
			report.LeavesQuantity = decimal.Zero
		}
	}
	if report.Price.IsZero() {
		report.Price = updated.Price
	}
	if report.TriggerPrice.IsZero() {
		report.TriggerPrice = updated.TriggerPrice
	}
	if report.ActivationPrice.IsZero() {
		report.ActivationPrice = updated.ActivationPrice
	}
	if report.TrailingOffset.IsZero() {
		report.TrailingOffset = updated.TrailingOffset
	}
	if report.TimeInForce == "" {
		report.TimeInForce = updated.TimeInForce
	}
	if report.ExpireTime.IsZero() {
		report.ExpireTime = updated.ExpireTime
	}
	report.PostOnly = report.PostOnly || updated.PostOnly
	report.ReduceOnly = report.ReduceOnly || updated.ReduceOnly
	return report
}

func orderLifecycleFromReport(report model.OrderStatusReport, kind model.OrderEventKind, previous model.OrderStatus, reason string) model.OrderLifecycleEvent {
	return model.OrderLifecycleEvent{
		AccountID:      report.AccountID,
		InstrumentID:   report.InstrumentID,
		OrderID:        report.OrderID,
		ClientOrderID:  report.ClientOrderID,
		VenueOrderID:   report.VenueOrderID,
		Kind:           kind,
		PreviousStatus: previous,
		Status:         report.Status,
		Reason:         reason,
		Report:         &report,
	}
}

func orderSubmittedLifecycle(order model.SubmitOrder) model.OrderLifecycleEvent {
	return model.OrderLifecycleEvent{
		AccountID:      order.AccountID,
		InstrumentID:   order.InstrumentID,
		ClientOrderID:  order.ClientOrderID,
		Kind:           model.OrderEventSubmitted,
		PreviousStatus: model.OrderStatusInitialized,
		Status:         model.OrderStatusSubmitted,
	}
}

func orderRejectedLifecycle(order model.SubmitOrder, cause error) model.OrderLifecycleEvent {
	event := model.OrderLifecycleEvent{
		AccountID:      order.AccountID,
		InstrumentID:   order.InstrumentID,
		ClientOrderID:  order.ClientOrderID,
		Kind:           model.OrderEventRejected,
		PreviousStatus: model.OrderStatusSubmitted,
		Status:         model.OrderStatusRejected,
	}
	if cause != nil {
		event.Reason = cause.Error()
	}
	return event
}

func (n *Node) indexOrderList(list model.OrderList) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.heldChildren == nil {
		n.heldChildren = make(map[parentOrderKey][]model.SubmitOrder)
	}
	if n.orderListMembers == nil {
		n.orderListMembers = make(map[orderListKey][]model.ClientOrderID)
	}
	for _, order := range list.Orders {
		listKey := orderListKey{accountID: order.AccountID, orderListID: list.ID}
		n.orderListMembers[listKey] = appendUniqueClientOrderID(n.orderListMembers[listKey], order.ClientOrderID)
		if order.ParentClientOrderID == "" {
			continue
		}
		parentKey := parentOrderKey{accountID: order.AccountID, clientOrderID: order.ParentClientOrderID}
		n.heldChildren[parentKey] = append(n.heldChildren[parentKey], order)
	}
}

func (n *Node) handleOrderListProgress(ctx context.Context, order model.OrderStatusReport) error {
	if order.Status != model.OrderStatusFilled {
		return nil
	}
	if err := n.releaseHeldChildren(ctx, order); err != nil {
		return err
	}
	return n.cancelOcoSiblings(ctx, order)
}

func (n *Node) releaseHeldChildren(ctx context.Context, parent model.OrderStatusReport) error {
	key := parentOrderKey{accountID: parent.AccountID, clientOrderID: parent.ClientOrderID}
	n.mu.Lock()
	children := append([]model.SubmitOrder(nil), n.heldChildren[key]...)
	delete(n.heldChildren, key)
	n.mu.Unlock()
	for _, child := range children {
		if _, err := n.SubmitOrder(ctx, child); err != nil {
			return err
		}
	}
	return nil
}

func (n *Node) cancelOcoSiblings(ctx context.Context, filled model.OrderStatusReport) error {
	if filled.OrderListID == "" || filled.Contingency != model.ContingencyTypeOCO {
		return nil
	}
	key := orderListKey{accountID: filled.AccountID, orderListID: filled.OrderListID}
	n.mu.RLock()
	members := append([]model.ClientOrderID(nil), n.orderListMembers[key]...)
	n.mu.RUnlock()
	for _, clientOrderID := range members {
		if clientOrderID == "" || clientOrderID == filled.ClientOrderID {
			continue
		}
		sibling, ok := n.cache.OrderByClientID(filled.AccountID, clientOrderID)
		if !ok || !sibling.Status.IsOpen() {
			continue
		}
		_, err := n.CancelOrder(ctx, model.CancelOrder{
			AccountID:     sibling.AccountID,
			InstrumentID:  sibling.InstrumentID,
			OrderID:       sibling.OrderID,
			ClientOrderID: sibling.ClientOrderID,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func appendUniqueClientOrderID(ids []model.ClientOrderID, id model.ClientOrderID) []model.ClientOrderID {
	for _, existing := range ids {
		if existing == id {
			return ids
		}
	}
	return append(ids, id)
}

func fillCanceledReport(report model.OrderStatusReport, cancel model.CancelOrder, existing model.OrderStatusReport) model.OrderStatusReport {
	if report.AccountID == "" {
		report.AccountID = cancel.AccountID
	}
	if report.InstrumentID == (model.InstrumentID{}) {
		report.InstrumentID = cancel.InstrumentID
	}
	if report.OrderID == "" {
		report.OrderID = cancel.OrderID
	}
	if report.ClientOrderID == "" {
		report.ClientOrderID = cancel.ClientOrderID
	}
	if report.VenueOrderID == "" {
		report.VenueOrderID = existing.VenueOrderID
	}
	if report.OrderListID == "" {
		report.OrderListID = existing.OrderListID
	}
	if report.ParentClientOrderID == "" {
		report.ParentClientOrderID = existing.ParentClientOrderID
	}
	if report.Status == "" {
		report.Status = model.OrderStatusCanceled
	}
	if report.Side == "" {
		report.Side = existing.Side
	}
	if report.Type == "" {
		report.Type = existing.Type
	}
	if report.Contingency == "" {
		report.Contingency = existing.Contingency
	}
	if report.Quantity.IsZero() {
		report.Quantity = existing.Quantity
	}
	if report.FilledQuantity.IsZero() {
		report.FilledQuantity = existing.FilledQuantity
	}
	report.LeavesQuantity = decimal.Zero
	if report.Price.IsZero() {
		report.Price = existing.Price
	}
	if report.TriggerPrice.IsZero() {
		report.TriggerPrice = existing.TriggerPrice
	}
	if report.ActivationPrice.IsZero() {
		report.ActivationPrice = existing.ActivationPrice
	}
	if report.TrailingOffset.IsZero() {
		report.TrailingOffset = existing.TrailingOffset
	}
	if report.TimeInForce == "" {
		report.TimeInForce = existing.TimeInForce
	}
	if report.ExpireTime.IsZero() {
		report.ExpireTime = existing.ExpireTime
	}
	report.PostOnly = report.PostOnly || existing.PostOnly
	report.ReduceOnly = report.ReduceOnly || existing.ReduceOnly
	return report
}

func (n *Node) recordError(err error) {
	if err == nil {
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	n.lastError = err
}

func (n *Node) Stop(ctx context.Context) error {
	if n.cancel != nil {
		n.cancel()
	}
	n.cancelAllTimers()
	n.wg.Wait()
	var stopErr error
	for _, client := range n.execClients {
		stopErr = errors.Join(stopErr, client.Disconnect(ctx))
	}
	for _, client := range n.dataClients {
		stopErr = errors.Join(stopErr, client.Disconnect(ctx))
	}
	n.mu.Lock()
	n.ready = false
	n.mu.Unlock()
	return stopErr
}

func (n *Node) cancelAllTimers() {
	n.mu.Lock()
	timers := n.timers
	n.timers = make(map[string]context.CancelFunc)
	n.mu.Unlock()
	for _, cancel := range timers {
		cancel()
	}
}

func (n *Node) Health() Health {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return Health{Ready: n.ready, LastError: n.lastError}
}
