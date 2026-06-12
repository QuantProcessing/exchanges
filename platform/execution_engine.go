package platform

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/QuantProcessing/exchanges/account"
	sharedcache "github.com/QuantProcessing/exchanges/cache"
	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/venue"
)

const TopicExecutionEvents = "events.execution"

type ExecutionEngine struct {
	mu          sync.RWMutex
	cache       *sharedcache.Cache
	bus         *Bus
	clients     map[string]venue.ExecutionClient
	reconcilers map[string]*account.Reconciler
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

func NewExecutionEngine(cache *sharedcache.Cache, bus *Bus) *ExecutionEngine {
	if cache == nil {
		cache = sharedcache.New()
	}
	if bus == nil {
		bus = NewBus()
	}
	return &ExecutionEngine{
		cache:       cache,
		bus:         bus,
		clients:     make(map[string]venue.ExecutionClient),
		reconcilers: make(map[string]*account.Reconciler),
	}
}

func (e *ExecutionEngine) AddClient(name string, client venue.ExecutionClient) error {
	if err := validateClientName(name); err != nil {
		return err
	}
	if client == nil {
		return fmt.Errorf("%w: execution %s", ErrNilClient, name)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.clients[name]; exists {
		return fmt.Errorf("%w: execution %s", ErrClientExists, name)
	}
	e.clients[name] = client
	e.reconcilers[name] = account.NewReconciler(e.cache)
	return nil
}

func (e *ExecutionEngine) Start(ctx context.Context) error {
	runCtx, cancel := context.WithCancel(context.Background())
	e.mu.Lock()
	e.cancel = cancel
	e.mu.Unlock()

	for name, client := range e.clientsSnapshot() {
		reconciler := e.reconciler(name)
		if err := client.QueryAccount(ctx); err != nil {
			cancel()
			return err
		}
		if err := e.drainClientEvents(client, reconciler); err != nil {
			cancel()
			return err
		}
		if err := e.generateStartupReports(ctx, client, reconciler); err != nil {
			cancel()
			return err
		}
		if err := client.Connect(ctx); err != nil {
			cancel()
			return err
		}
		e.wg.Add(1)
		go e.forwardClientEvents(runCtx, client, reconciler)
	}
	return nil
}

func (e *ExecutionEngine) Stop(ctx context.Context) error {
	e.mu.Lock()
	cancel := e.cancel
	e.cancel = nil
	e.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	e.wg.Wait()

	var stopErr error
	for _, client := range e.clientsSnapshot() {
		if err := client.Disconnect(ctx); err != nil {
			stopErr = errors.Join(stopErr, err)
		}
	}
	return stopErr
}

func (e *ExecutionEngine) generateStartupReports(ctx context.Context, client venue.ExecutionClient, reconciler *account.Reconciler) error {
	for _, inst := range e.cache.Instruments() {
		orderReports, err := client.GenerateOrderStatusReports(ctx, venue.OrderStatusQuery{InstrumentID: inst.ID})
		if err != nil && !errors.Is(err, model.ErrNotSupported) {
			return err
		}
		for _, report := range orderReports {
			ev := model.ExecutionEvent{Order: &report}
			if err := e.applyAndPublish(reconciler, ev); err != nil {
				return err
			}
		}

		fillReports, err := client.GenerateFillReports(ctx, venue.FillQuery{InstrumentID: inst.ID})
		if err != nil && !errors.Is(err, model.ErrNotSupported) {
			return err
		}
		for _, report := range fillReports {
			ev := model.ExecutionEvent{Fill: &report}
			if err := e.applyAndPublish(reconciler, ev); err != nil {
				return err
			}
		}

		positionReports, err := client.GeneratePositionStatusReports(ctx, venue.PositionQuery{InstrumentID: inst.ID})
		if err != nil && !errors.Is(err, model.ErrNotSupported) {
			return err
		}
		for _, report := range positionReports {
			ev := model.ExecutionEvent{Position: &report}
			if err := e.applyAndPublish(reconciler, ev); err != nil {
				return err
			}
		}
	}
	return nil
}

func (e *ExecutionEngine) drainClientEvents(client venue.ExecutionClient, reconciler *account.Reconciler) error {
	events := client.Events()
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				return nil
			}
			if err := e.applyAndPublish(reconciler, ev); err != nil {
				return err
			}
		default:
			return nil
		}
	}
}

func (e *ExecutionEngine) forwardClientEvents(ctx context.Context, client venue.ExecutionClient, reconciler *account.Reconciler) {
	defer e.wg.Done()
	events := client.Events()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-events:
			if !ok {
				return
			}
			_ = e.applyAndPublish(reconciler, ev)
		}
	}
}

func (e *ExecutionEngine) applyAndPublish(reconciler *account.Reconciler, ev model.ExecutionEvent) error {
	if reconciler != nil {
		if err := reconciler.ApplyEvent(ev); err != nil {
			return err
		}
	}
	return e.bus.Publish(TopicExecutionEvents, ev)
}

func (e *ExecutionEngine) clientsSnapshot() map[string]venue.ExecutionClient {
	e.mu.RLock()
	defer e.mu.RUnlock()
	clients := make(map[string]venue.ExecutionClient, len(e.clients))
	for name, client := range e.clients {
		clients[name] = client
	}
	return clients
}

func (e *ExecutionEngine) reconciler(name string) *account.Reconciler {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.reconcilers[name]
}
