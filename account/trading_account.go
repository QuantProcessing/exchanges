package account

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/QuantProcessing/exchanges/cache"
	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/venue"
)

const defaultTrackerBufferSize = 16

type TradingAccountConfig struct {
	Cache                   *cache.Cache
	Instruments             []model.InstrumentID
	BufferSize              int
	ReconcileInterval       time.Duration
	MissingOrderRepairDelay time.Duration
}

type TradingAccountHealth struct {
	Ready                bool
	AccountReady         bool
	OrderStreamReady     bool
	FillsUnsupported     bool
	PositionsUnsupported bool
	Reconnects           int64
	Reconciliations      int64
	AccountEvents        int64
	OrderEvents          int64
	FillEvents           int64
	PositionEvents       int64
	SlowSubscriberDrops  int64
	LastEventTime        time.Time
	LastReconcileTime    time.Time
	LastError            error
}

type TradingAccount struct {
	client             venue.ExecutionClient
	cache              *cache.Cache
	reconciler         *Reconciler
	instruments        []model.InstrumentID
	bufferSize         int
	reconcileInterval  time.Duration
	missingRepairDelay time.Duration

	mu       sync.RWMutex
	cancel   context.CancelFunc
	trackers map[*OrderTracker]struct{}
	health   TradingAccountHealth
	started  bool
	wg       sync.WaitGroup
}

func NewTradingAccount(client venue.ExecutionClient, cfg TradingAccountConfig) (*TradingAccount, error) {
	if client == nil {
		return nil, fmt.Errorf("%w: execution client is required", model.ErrNotSupported)
	}
	for _, instrumentID := range cfg.Instruments {
		if err := instrumentID.Validate(); err != nil {
			return nil, err
		}
	}
	c := cfg.Cache
	if c == nil {
		c = cache.New()
	}
	bufferSize := cfg.BufferSize
	if bufferSize <= 0 {
		bufferSize = defaultTrackerBufferSize
	}
	return &TradingAccount{
		client:             client,
		cache:              c,
		reconciler:         NewReconciler(c),
		instruments:        append([]model.InstrumentID(nil), cfg.Instruments...),
		bufferSize:         bufferSize,
		reconcileInterval:  cfg.ReconcileInterval,
		missingRepairDelay: cfg.MissingOrderRepairDelay,
		trackers:           make(map[*OrderTracker]struct{}),
	}, nil
}

func (a *TradingAccount) Cache() *cache.Cache { return a.cache }

func (a *TradingAccount) Health() TradingAccountHealth {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.health
}

func (a *TradingAccount) Start(ctx context.Context) error {
	a.mu.Lock()
	if a.started {
		a.mu.Unlock()
		return nil
	}
	runCtx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	a.health = TradingAccountHealth{}
	a.started = true
	a.mu.Unlock()

	if err := a.client.Connect(ctx); err != nil {
		a.recordError(err)
		a.markStopped()
		cancel()
		return err
	}
	if err := a.reconcileStartup(ctx); err != nil {
		a.recordError(err)
		_ = a.client.Disconnect(ctx)
		a.markStopped()
		cancel()
		return err
	}

	a.mu.Lock()
	a.health.Ready = true
	a.health.AccountReady = true
	a.health.OrderStreamReady = a.client.Events() != nil
	a.mu.Unlock()

	if events := a.client.Events(); events != nil {
		a.wg.Add(1)
		go a.forwardEvents(runCtx, events)
	}
	if a.reconcileInterval > 0 {
		a.wg.Add(1)
		go a.reconcilePeriodically(runCtx, a.reconcileInterval)
	}
	return nil
}

func (a *TradingAccount) Stop(ctx context.Context) error {
	a.mu.Lock()
	if !a.started {
		a.mu.Unlock()
		return nil
	}
	cancel := a.cancel
	a.started = false
	a.health.Ready = false
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	a.wg.Wait()
	a.closeTrackers()
	return a.client.Disconnect(ctx)
}

func (a *TradingAccount) SubmitOrder(ctx context.Context, order model.SubmitOrder) (*OrderTracker, error) {
	if order.AccountID == "" {
		order.AccountID = a.client.AccountID()
	}
	if err := order.Validate(); err != nil {
		a.recordError(err)
		return nil, err
	}
	report, err := a.client.SubmitOrder(ctx, order)
	if err != nil {
		a.recordError(err)
		return nil, err
	}
	tracker := newOrderTracker(a, report, a.bufferSize)
	a.registerTracker(tracker)
	if err := a.applyAndPublish(model.ExecutionEvent{Order: &report}); err != nil {
		tracker.Close()
		return nil, err
	}
	return tracker, nil
}

func (a *TradingAccount) CancelOrder(ctx context.Context, cancel model.CancelOrder) (model.OrderStatusReport, error) {
	if cancel.AccountID == "" {
		cancel.AccountID = a.client.AccountID()
	}
	if err := cancel.Validate(); err != nil {
		a.recordError(err)
		return model.OrderStatusReport{}, err
	}
	report, err := a.client.CancelOrder(ctx, cancel)
	if err != nil {
		a.recordError(err)
		return model.OrderStatusReport{}, err
	}
	report = fillCancelReport(report, cancel)
	if err := a.applyAndPublish(model.ExecutionEvent{Order: &report}); err != nil {
		a.recordError(err)
		return model.OrderStatusReport{}, err
	}
	return report, nil
}

func (a *TradingAccount) ModifyOrder(ctx context.Context, modify model.ModifyOrder) (model.OrderStatusReport, error) {
	if modify.AccountID == "" {
		modify.AccountID = a.client.AccountID()
	}
	if err := modify.Validate(); err != nil {
		a.recordError(err)
		return model.OrderStatusReport{}, err
	}
	modifier, ok := a.client.(venue.OrderModifier)
	if !ok {
		err := fmt.Errorf("%w: execution client does not support order modification", model.ErrNotSupported)
		a.recordError(err)
		return model.OrderStatusReport{}, err
	}
	report, err := modifier.ModifyOrder(ctx, modify)
	if err != nil {
		a.recordError(err)
		return model.OrderStatusReport{}, err
	}
	report = fillModifyReport(report, modify)
	if err := a.applyAndPublish(model.ExecutionEvent{Order: &report}); err != nil {
		a.recordError(err)
		return model.OrderStatusReport{}, err
	}
	return report, nil
}

func (a *TradingAccount) QueryOrder(ctx context.Context, query model.QueryOrder) (model.OrderStatusReport, error) {
	if query.AccountID == "" {
		query.AccountID = a.client.AccountID()
	}
	if err := query.Validate(); err != nil {
		a.recordError(err)
		return model.OrderStatusReport{}, err
	}
	if report, ok := a.findOrder(query); ok {
		return fillQueryReport(report, query), nil
	}
	if querier, ok := a.client.(venue.OrderQuerier); ok {
		report, err := querier.QueryOrder(ctx, query)
		if err == nil {
			report = fillQueryReport(report, query)
			if applyErr := a.applyAndPublish(model.ExecutionEvent{Order: &report}); applyErr != nil {
				a.recordError(applyErr)
				return model.OrderStatusReport{}, applyErr
			}
			return report, nil
		}
		if !errors.Is(err, model.ErrNotSupported) && !errors.Is(err, model.ErrInvalidOrder) {
			a.recordError(err)
			return model.OrderStatusReport{}, err
		}
	}
	reports, err := a.client.GenerateOrderStatusReports(ctx, query.InstrumentID)
	if err != nil {
		a.recordError(err)
		return model.OrderStatusReport{}, err
	}
	for _, report := range reports {
		if !queryMatchesReport(query, report) {
			continue
		}
		report = fillQueryReport(report, query)
		if err := a.applyAndPublish(model.ExecutionEvent{Order: &report}); err != nil {
			a.recordError(err)
			return model.OrderStatusReport{}, err
		}
		return report, nil
	}
	err = fmt.Errorf("%w: order not found", model.ErrInvalidOrder)
	a.recordError(err)
	return model.OrderStatusReport{}, err
}

func (a *TradingAccount) QueryAccount(ctx context.Context) (model.AccountSnapshot, error) {
	account, err := a.client.QueryAccount(ctx)
	if err != nil {
		a.recordError(err)
		return model.AccountSnapshot{}, err
	}
	if err := a.applyAndPublish(model.ExecutionEvent{Account: &account}); err != nil {
		a.recordError(err)
		return model.AccountSnapshot{}, err
	}
	return account, nil
}

func (a *TradingAccount) reconcileStartup(ctx context.Context) error {
	return a.reconcileExecution(ctx, false)
}

func (a *TradingAccount) reconcileExecution(ctx context.Context, repairMissing bool) error {
	apply := func(event model.ExecutionEvent) error {
		if repairMissing {
			return a.applyAndPublish(event)
		}
		return a.reconciler.Apply(event)
	}
	account, err := a.client.QueryAccount(ctx)
	if err != nil {
		return err
	}
	if err := apply(model.ExecutionEvent{Account: &account}); err != nil {
		return err
	}
	fillsSupported := false
	fillGenerator, hasFillGenerator := a.client.(venue.FillReportGenerator)
	positionsSupported := false
	positionGenerator, hasPositionGenerator := a.client.(venue.PositionStatusReportGenerator)
	for _, instrumentID := range a.instruments {
		reports, err := a.client.GenerateOrderStatusReports(ctx, instrumentID)
		if err != nil {
			return err
		}
		for _, report := range reports {
			if err := apply(model.ExecutionEvent{Order: &report}); err != nil {
				return err
			}
		}
		if hasFillGenerator {
			fills, err := fillGenerator.GenerateFillReports(ctx, instrumentID)
			if errors.Is(err, model.ErrNotSupported) {
				fills = nil
			} else if err != nil {
				return err
			} else {
				fillsSupported = true
			}
			for _, fill := range fills {
				if err := apply(model.ExecutionEvent{Fill: &fill}); err != nil {
					return err
				}
			}
		}
		if hasPositionGenerator {
			positions, err := positionGenerator.GeneratePositionStatusReports(ctx, instrumentID)
			if errors.Is(err, model.ErrNotSupported) {
				positions = nil
			} else if err != nil {
				return err
			} else {
				positionsSupported = true
			}
			for _, position := range positions {
				if err := apply(model.ExecutionEvent{Position: &position}); err != nil {
					return err
				}
			}
			if repairMissing && err == nil {
				missing, err := a.reconciler.MissingPositionReports(a.client.AccountID(), instrumentID, positions)
				if err != nil {
					return err
				}
				for _, position := range missing {
					if err := apply(model.ExecutionEvent{Position: &position}); err != nil {
						return err
					}
				}
			}
		}
		if repairMissing {
			missing, err := a.reconciler.ReconcileMissingOpenOrdersWithPolicy(a.client.AccountID(), instrumentID, reports, MissingOpenOrderRepairPolicy{
				MissingStatus:        model.OrderStatusCanceled,
				RecentActivityWindow: a.missingRepairDelay,
			})
			if err != nil {
				return err
			}
			for _, report := range missing {
				a.publishOrder(report)
			}
		}
	}
	a.mu.Lock()
	a.health.FillsUnsupported = !hasFillGenerator || !fillsSupported && len(a.instruments) > 0
	a.health.PositionsUnsupported = !hasPositionGenerator || !positionsSupported && len(a.instruments) > 0
	a.mu.Unlock()
	return nil
}

func (a *TradingAccount) forwardEvents(ctx context.Context, events <-chan model.ExecutionEvent) {
	defer a.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-events:
			if !ok {
				a.mu.Lock()
				a.health.OrderStreamReady = false
				a.mu.Unlock()
				next, err := a.recoverExecutionStream(ctx, events)
				if err != nil {
					a.recordError(err)
					return
				}
				events = next
				continue
			}
			if err := a.applyAndPublish(event); err != nil {
				a.recordError(err)
			}
		}
	}
}

func (a *TradingAccount) recoverExecutionStream(ctx context.Context, closed <-chan model.ExecutionEvent) (<-chan model.ExecutionEvent, error) {
	if err := a.client.Connect(ctx); err != nil {
		return nil, err
	}
	if resubscriber, ok := a.client.(venue.ExecutionResubscriber); ok {
		if err := resubscriber.ResubscribeExecution(ctx); err != nil {
			return nil, err
		}
	}
	if err := a.reconcileExecution(ctx, true); err != nil {
		return nil, err
	}
	next := a.client.Events()
	if next == nil || next == closed {
		return nil, fmt.Errorf("%w: execution client did not expose a fresh event stream", model.ErrNotSupported)
	}
	a.mu.Lock()
	a.health.OrderStreamReady = true
	a.health.Reconnects++
	a.mu.Unlock()
	return next, nil
}

func (a *TradingAccount) reconcilePeriodically(ctx context.Context, interval time.Duration) {
	defer a.wg.Done()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := a.reconcileExecution(ctx, true); err != nil {
				a.recordError(err)
				continue
			}
			a.mu.Lock()
			a.health.Reconciliations++
			a.health.LastReconcileTime = time.Now()
			a.mu.Unlock()
		}
	}
}

func (a *TradingAccount) applyAndPublish(event model.ExecutionEvent) error {
	if err := a.reconciler.Apply(event); err != nil {
		return err
	}
	now := time.Now()
	a.mu.Lock()
	a.health.LastEventTime = now
	if event.Account != nil {
		a.health.AccountEvents++
	}
	if event.Order != nil {
		a.health.OrderEvents++
	}
	if event.Fill != nil {
		a.health.FillEvents++
	}
	if event.Position != nil {
		a.health.PositionEvents++
	}
	a.mu.Unlock()
	if event.Order != nil {
		a.publishOrder(*event.Order)
	}
	if event.Fill != nil {
		a.publishFill(*event.Fill)
	}
	return nil
}

func (a *TradingAccount) registerTracker(tracker *OrderTracker) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.trackers[tracker] = struct{}{}
}

func (a *TradingAccount) unregisterTracker(tracker *OrderTracker) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.trackers, tracker)
}

func (a *TradingAccount) publishOrder(report model.OrderStatusReport) {
	for _, tracker := range a.matchingTrackers(func(t *OrderTracker) bool { return t.matchesOrder(report) }) {
		if !tracker.sendOrder(report) {
			a.incrementDrops()
		}
	}
}

func (a *TradingAccount) publishFill(fill model.FillReport) {
	for _, tracker := range a.matchingTrackers(func(t *OrderTracker) bool { return t.matchesFill(fill) }) {
		if !tracker.sendFill(fill) {
			a.incrementDrops()
		}
	}
}

func (a *TradingAccount) matchingTrackers(match func(*OrderTracker) bool) []*OrderTracker {
	a.mu.RLock()
	trackers := make([]*OrderTracker, 0, len(a.trackers))
	for tracker := range a.trackers {
		if match(tracker) {
			trackers = append(trackers, tracker)
		}
	}
	a.mu.RUnlock()
	return trackers
}

func (a *TradingAccount) closeTrackers() {
	a.mu.RLock()
	trackers := make([]*OrderTracker, 0, len(a.trackers))
	for tracker := range a.trackers {
		trackers = append(trackers, tracker)
	}
	a.mu.RUnlock()
	for _, tracker := range trackers {
		tracker.Close()
	}
}

func (a *TradingAccount) recordError(err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.health.LastError = err
}

func (a *TradingAccount) incrementDrops() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.health.SlowSubscriberDrops++
}

func (a *TradingAccount) markStopped() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.started = false
	a.health.Ready = false
}

func (a *TradingAccount) findOrder(query model.QueryOrder) (model.OrderStatusReport, bool) {
	if query.OrderID != "" {
		if report, ok := a.cache.Order(query.AccountID, query.OrderID); ok {
			return report, true
		}
	}
	if query.ClientOrderID != "" {
		if report, ok := a.cache.OrderByClientID(query.AccountID, query.ClientOrderID); ok {
			return report, true
		}
	}
	if query.VenueOrderID != "" {
		if report, ok := a.cache.OrderByVenueID(query.AccountID, query.VenueOrderID); ok {
			return report, true
		}
	}
	return model.OrderStatusReport{}, false
}

func fillCancelReport(report model.OrderStatusReport, cancel model.CancelOrder) model.OrderStatusReport {
	report.Metadata = cancel.Metadata.WithDefaults(report.Metadata)
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
	if report.Status == "" {
		report.Status = model.OrderStatusCanceled
	}
	return report
}

func fillModifyReport(report model.OrderStatusReport, modify model.ModifyOrder) model.OrderStatusReport {
	report.Metadata = modify.Metadata.WithDefaults(report.Metadata)
	if report.AccountID == "" {
		report.AccountID = modify.AccountID
	}
	if report.InstrumentID == (model.InstrumentID{}) {
		report.InstrumentID = modify.InstrumentID
	}
	if report.OrderID == "" {
		report.OrderID = modify.OrderID
	}
	if report.ClientOrderID == "" {
		report.ClientOrderID = modify.ClientOrderID
	}
	if report.VenueOrderID == "" {
		report.VenueOrderID = modify.VenueOrderID
	}
	if report.Status == "" {
		report.Status = model.OrderStatusAccepted
	}
	return report
}

func fillQueryReport(report model.OrderStatusReport, query model.QueryOrder) model.OrderStatusReport {
	report.Metadata = query.Metadata.WithDefaults(report.Metadata)
	if report.AccountID == "" {
		report.AccountID = query.AccountID
	}
	if report.InstrumentID == (model.InstrumentID{}) {
		report.InstrumentID = query.InstrumentID
	}
	return report
}

func queryMatchesReport(query model.QueryOrder, report model.OrderStatusReport) bool {
	if report.AccountID != "" && report.AccountID != query.AccountID {
		return false
	}
	if report.InstrumentID != query.InstrumentID {
		return false
	}
	if query.OrderID != "" && report.OrderID == query.OrderID {
		return true
	}
	if query.ClientOrderID != "" && report.ClientOrderID == query.ClientOrderID {
		return true
	}
	return query.VenueOrderID != "" && report.VenueOrderID == query.VenueOrderID
}
