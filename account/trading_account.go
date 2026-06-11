package account

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/venue"
)

type TradingAccountConfig struct {
	Instruments []model.InstrumentID
	Cache       *Cache
	Reconciler  *Reconciler
}

type TradingAccount struct {
	exec       venue.ExecutionClient
	cache      *Cache
	reconciler *Reconciler
	cfg        TradingAccountConfig

	mu             sync.RWMutex
	started        bool
	starting       bool
	closing        bool
	snapshotLoaded bool
	lastSnapshotAt time.Time
	streams        map[StreamName]StreamHealth
	cancel         context.CancelFunc
	wg             sync.WaitGroup
}

func NewTradingAccount(exec venue.ExecutionClient, cfg TradingAccountConfig) (*TradingAccount, error) {
	if exec == nil {
		return nil, fmt.Errorf("%w: nil execution client", model.ErrInvalidAccountState)
	}
	cache := cfg.Cache
	if cache == nil {
		cache = NewCache()
	}
	reconciler := cfg.Reconciler
	if reconciler == nil {
		reconciler = NewReconciler(cache)
	}
	cfg.Instruments = append([]model.InstrumentID(nil), cfg.Instruments...)
	cfg.Cache = cache
	cfg.Reconciler = reconciler
	return &TradingAccount{
		exec:       exec,
		cache:      cache,
		reconciler: reconciler,
		cfg:        cfg,
		streams:    initialStreamHealth(),
	}, nil
}

func (a *TradingAccount) Start(ctx context.Context) error {
	a.mu.Lock()
	if a.started || a.starting {
		a.mu.Unlock()
		return nil
	}
	a.starting = true
	a.closing = false
	a.snapshotLoaded = false
	a.lastSnapshotAt = time.Time{}
	a.streams = initialStreamHealth()
	for name, health := range a.streams {
		health.Status = StreamStatusStarting
		a.streams[name] = health
	}
	a.mu.Unlock()

	if err := a.reconcileSnapshot(ctx); err != nil {
		a.failStart(err)
		return err
	}
	if err := a.exec.Connect(ctx); err != nil {
		a.markStreamError(StreamOrders, err)
		a.failStart(err)
		return err
	}

	runCtx, cancel := context.WithCancel(context.Background())
	a.mu.Lock()
	a.cancel = cancel
	a.started = true
	a.starting = false
	a.snapshotLoaded = true
	a.lastSnapshotAt = time.Now()
	a.markStartingStreamsReadyLocked()
	a.mu.Unlock()

	a.wg.Add(1)
	go a.runEvents(runCtx)
	return nil
}

func (a *TradingAccount) Stop(ctx context.Context) error {
	a.mu.Lock()
	if !a.started && !a.starting {
		a.mu.Unlock()
		return nil
	}
	a.closing = true
	cancel := a.cancel
	a.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	done := make(chan struct{})
	go func() {
		a.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		return ctx.Err()
	}

	err := a.exec.Disconnect(ctx)
	a.reconciler.Close()

	a.mu.Lock()
	a.started = false
	a.starting = false
	a.closing = false
	a.markStreamsStoppedLocked()
	a.mu.Unlock()
	return err
}

func (a *TradingAccount) Ready() bool {
	health := a.Health()
	return health.Started && health.SnapshotLoaded && !health.Starting && !health.Closing
}

func (a *TradingAccount) Health() TradingAccountHealth {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return TradingAccountHealth{
		Started:        a.started,
		Starting:       a.starting,
		Closing:        a.closing,
		SnapshotLoaded: a.snapshotLoaded,
		LastSnapshotAt: a.lastSnapshotAt,
		Streams:        copyStreamHealthMap(a.streams),
	}
}

func (a *TradingAccount) Cache() *Cache {
	return a.cache
}

func (a *TradingAccount) AccountState() (model.AccountState, bool) {
	return a.cache.AccountState(a.exec.Venue(), a.exec.AccountID())
}

func (a *TradingAccount) FlowByOrderID(orderID model.OrderID) (*OrderTracker, bool) {
	return a.reconciler.FlowByOrderID(orderID)
}

func (a *TradingAccount) FlowByClientID(clientID model.ClientOrderID) (*OrderTracker, bool) {
	return a.reconciler.FlowByClientID(clientID)
}

func (a *TradingAccount) PositionsSnapshot() []model.PositionStatusReport {
	return a.reconciler.PositionsSnapshot()
}

func (a *TradingAccount) SubmitOrder(ctx context.Context, cmd model.SubmitOrder) (*OrderTracker, error) {
	if cmd.ClientID == "" {
		cmd.ClientID = model.NewClientOrderID()
	}
	flow := a.reconciler.EnsureFlowForClientID(cmd.ClientID)
	if err := a.exec.SubmitOrder(ctx, cmd); err != nil {
		return nil, err
	}
	return flow, nil
}

func (a *TradingAccount) CancelOrder(ctx context.Context, cmd model.CancelOrder) error {
	return a.exec.CancelOrder(ctx, cmd)
}

func (a *TradingAccount) CancelAllOrders(ctx context.Context, cmd model.CancelAllOrders) error {
	return a.exec.CancelAllOrders(ctx, cmd)
}

func (a *TradingAccount) reconcileSnapshot(ctx context.Context) error {
	if err := a.exec.QueryAccount(ctx); err != nil {
		a.markStreamError(StreamBalances, err)
		return err
	}
	if err := a.drainExecutionEvents(); err != nil {
		return err
	}
	a.markStreamReady(StreamBalances)

	for _, id := range a.cfg.Instruments {
		orders, err := a.exec.GenerateOrderStatusReports(ctx, venue.OrderStatusQuery{InstrumentID: id})
		if err != nil {
			if errors.Is(err, model.ErrNotSupported) {
				a.markStreamUnsupported(StreamOrders, err)
			} else {
				a.markStreamError(StreamOrders, err)
				return err
			}
		}
		for _, report := range orders {
			if err := a.reconciler.ApplyOrderStatusReport(report); err != nil {
				a.markStreamError(StreamOrders, err)
				return err
			}
		}
		if !errors.Is(err, model.ErrNotSupported) {
			a.markStreamReady(StreamOrders)
		}

		fills, err := a.exec.GenerateFillReports(ctx, venue.FillQuery{InstrumentID: id})
		if err != nil {
			if errors.Is(err, model.ErrNotSupported) {
				a.markStreamUnsupported(StreamFills, err)
			} else {
				a.markStreamError(StreamFills, err)
				return err
			}
		}
		for _, report := range fills {
			if err := a.reconciler.ApplyFillReport(report); err != nil {
				a.markStreamError(StreamFills, err)
				return err
			}
		}
		if !errors.Is(err, model.ErrNotSupported) {
			a.markStreamReady(StreamFills)
		}

		positions, err := a.exec.GeneratePositionStatusReports(ctx, venue.PositionQuery{InstrumentID: id})
		if err != nil {
			if errors.Is(err, model.ErrNotSupported) {
				a.markStreamUnsupported(StreamPositions, err)
			} else {
				a.markStreamError(StreamPositions, err)
				return err
			}
		}
		for _, report := range positions {
			if err := a.reconciler.ApplyPositionStatusReport(report); err != nil {
				a.markStreamError(StreamPositions, err)
				return err
			}
		}
		if !errors.Is(err, model.ErrNotSupported) {
			a.markStreamReady(StreamPositions)
		}
	}
	return nil
}

func (a *TradingAccount) drainExecutionEvents() error {
	events := a.exec.Events()
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				return nil
			}
			if err := a.applyExecutionEvent(ev); err != nil {
				return err
			}
		default:
			return nil
		}
	}
}

func (a *TradingAccount) runEvents(ctx context.Context) {
	defer a.wg.Done()
	events := a.exec.Events()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-events:
			if !ok {
				return
			}
			if err := a.applyExecutionEvent(ev); err != nil {
				a.markStreamError(streamNameForExecutionEvent(ev), err)
			}
		}
	}
}

func (a *TradingAccount) applyExecutionEvent(ev model.ExecutionEvent) error {
	if err := a.reconciler.ApplyEvent(ev); err != nil {
		return err
	}
	a.markStreamEvent(streamNameForExecutionEvent(ev), 0)
	return nil
}

func streamNameForExecutionEvent(ev model.ExecutionEvent) StreamName {
	switch {
	case ev.AccountState != nil:
		return StreamBalances
	case ev.Order != nil:
		return StreamOrders
	case ev.Fill != nil:
		return StreamFills
	case ev.Position != nil:
		return StreamPositions
	default:
		return StreamOrders
	}
}

func (a *TradingAccount) failStart(error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.started = false
	a.starting = false
	a.closing = false
}

func (a *TradingAccount) markStreamReady(name StreamName) {
	a.updateStreamHealth(name, func(health StreamHealth) StreamHealth {
		if health.Status == StreamStatusUnsupported {
			return health
		}
		health.Status = StreamStatusReady
		health.Supported = true
		health.Ready = true
		health.LastError = ""
		health.LastErrorAt = time.Time{}
		return health
	})
}

func (a *TradingAccount) markStreamUnsupported(name StreamName, err error) {
	a.updateStreamHealth(name, func(health StreamHealth) StreamHealth {
		health.Status = StreamStatusUnsupported
		health.Supported = false
		health.Ready = false
		if err != nil {
			health.LastError = err.Error()
			health.LastErrorAt = time.Now()
		}
		return health
	})
}

func (a *TradingAccount) markStreamError(name StreamName, err error) {
	a.updateStreamHealth(name, func(health StreamHealth) StreamHealth {
		health.Status = StreamStatusError
		health.Supported = true
		health.Ready = false
		if err != nil {
			health.LastError = err.Error()
			health.LastErrorAt = time.Now()
		}
		return health
	})
}

func (a *TradingAccount) markStreamEvent(name StreamName, dropped uint64) {
	a.updateStreamHealth(name, func(health StreamHealth) StreamHealth {
		health.Events++
		health.DroppedEvents += dropped
		health.LastEventAt = time.Now()
		if health.Status == StreamStatusStarting || health.Status == StreamStatusUnknown {
			health.Status = StreamStatusReady
			health.Ready = true
		}
		return health
	})
}

func (a *TradingAccount) updateStreamHealth(name StreamName, update func(StreamHealth) StreamHealth) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.streams == nil {
		a.streams = initialStreamHealth()
	}
	health, ok := a.streams[name]
	if !ok {
		health = StreamHealth{Name: name, Status: StreamStatusUnknown, Supported: true}
	}
	a.streams[name] = update(health)
}

func (a *TradingAccount) markStartingStreamsReadyLocked() {
	if a.streams == nil {
		a.streams = initialStreamHealth()
	}
	for name, health := range a.streams {
		if health.Status == StreamStatusStarting || health.Status == StreamStatusUnknown {
			health.Status = StreamStatusReady
			health.Supported = true
			health.Ready = true
			a.streams[name] = health
		}
	}
}

func (a *TradingAccount) markStreamsStoppedLocked() {
	if a.streams == nil {
		a.streams = initialStreamHealth()
	}
	for name, health := range a.streams {
		health.Status = StreamStatusStopped
		health.Ready = false
		a.streams[name] = health
	}
}
