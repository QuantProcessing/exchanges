package execution

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/QuantProcessing/exchanges/cache"
	"github.com/QuantProcessing/exchanges/kernel"
	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/venue"
)

var (
	ErrClientNotFound     = errors.New("execution client not found")
	ErrAlgorithmNotFound  = errors.New("execution algorithm not found")
	ErrAlgorithmIDMissing = errors.New("execution algorithm id missing")
)

type Client interface {
	venue.ExecutionClient
}

type ExecutionAlgorithm interface {
	ID() model.ExecAlgorithmID
	SubmitOrder(context.Context, model.SubmitOrder) (model.OrderStatusReport, error)
}

type OrderListAlgorithm interface {
	SubmitOrderList(context.Context, model.SubmitOrderList) ([]model.OrderStatusReport, error)
}

type OrderListSubmitter interface {
	SubmitOrderList(context.Context, model.SubmitOrderList) ([]model.OrderStatusReport, error)
}

type BatchCanceler interface {
	BatchCancelOrders(context.Context, model.BatchCancelOrders) ([]model.OrderStatusReport, error)
}

type CancelAllCanceler interface {
	CancelAllOrders(context.Context, model.CancelAllOrders) ([]model.OrderStatusReport, error)
}

type EngineConfig struct {
	Cache    *cache.Cache
	Manager  *Manager
	Emulator *Emulator
}

type Engine struct {
	mu             sync.RWMutex
	cache          *cache.Cache
	manager        *Manager
	emulator       *Emulator
	component      *kernel.Component
	events         chan model.ExecutionEvent
	clients        map[model.AccountID]Client
	algorithms     map[model.ExecAlgorithmID]ExecutionAlgorithm
	externalClaims map[model.InstrumentID]model.StrategyID
	starts         int64
	stops          int64
	submits        int64
	cancels        int64
	modifies       int64
	queries        int64
}

type Health struct {
	kernel.Health
	Clients    int
	Algorithms int
	Starts     int64
	Stops      int64
	Submits    int64
	Cancels    int64
	Modifies   int64
	Queries    int64
}

func NewEngine(cfg EngineConfig) *Engine {
	c := cfg.Cache
	if c == nil {
		c = cache.New()
	}
	manager := cfg.Manager
	if manager == nil {
		manager = NewManager(Config{Cache: c})
	}
	emulator := cfg.Emulator
	if emulator != nil {
		emulator.bind(manager)
	}
	engine := &Engine{
		cache:          c,
		manager:        manager,
		emulator:       emulator,
		events:         make(chan model.ExecutionEvent, 256),
		clients:        make(map[model.AccountID]Client),
		algorithms:     make(map[model.ExecAlgorithmID]ExecutionAlgorithm),
		externalClaims: make(map[model.InstrumentID]model.StrategyID),
	}
	engine.component = kernel.NewComponent("execution.engine", kernel.ComponentHooks{
		Start: engine.startClients,
		Stop:  engine.stopClients,
	})
	return engine
}

func (e *Engine) Manager() *Manager {
	if e == nil {
		return nil
	}
	return e.manager
}

func (e *Engine) Snapshot(accountID model.AccountID) cache.Snapshot {
	return e.cache.Snapshot(accountID)
}

func (e *Engine) Purge(accountID model.AccountID, policy cache.PurgePolicy) cache.PurgeResult {
	return e.cache.Purge(accountID, policy)
}

func (e *Engine) Events() <-chan model.ExecutionEvent {
	if e == nil {
		return nil
	}
	return e.events
}

func (e *Engine) AddClient(client Client) error {
	if client == nil {
		return fmt.Errorf("%w: execution client is required", model.ErrInvalidOrder)
	}
	if client.AccountID() == "" {
		return fmt.Errorf("%w: execution client account id is required", model.ErrInvalidOrder)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.clients[client.AccountID()] = client
	return nil
}

func (e *Engine) AddAlgorithm(algorithm ExecutionAlgorithm) error {
	if algorithm == nil {
		return fmt.Errorf("%w: execution algorithm is required", model.ErrInvalidOrder)
	}
	id := algorithm.ID()
	if id == "" {
		return ErrAlgorithmIDMissing
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.algorithms == nil {
		e.algorithms = make(map[model.ExecAlgorithmID]ExecutionAlgorithm)
	}
	e.algorithms[id] = algorithm
	return nil
}

func (e *Engine) RegisterExternalOrderClaim(instrumentID model.InstrumentID, strategyID model.StrategyID) error {
	if err := instrumentID.Validate(); err != nil {
		return err
	}
	if strategyID == "" {
		return fmt.Errorf("%w: external order claim strategy id is required", model.ErrInvalidOrder)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.externalClaims == nil {
		e.externalClaims = make(map[model.InstrumentID]model.StrategyID)
	}
	if existing := e.externalClaims[instrumentID]; existing != "" {
		return fmt.Errorf("%w: external order claim for %s already exists for %s", model.ErrInvalidOrder, instrumentID, existing)
	}
	e.externalClaims[instrumentID] = strategyID
	return nil
}

func (e *Engine) ExternalOrderClaim(instrumentID model.InstrumentID) (model.StrategyID, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	strategyID, ok := e.externalClaims[instrumentID]
	return strategyID, ok
}

func (e *Engine) ExternalOrderClaimInstruments() []model.InstrumentID {
	e.mu.RLock()
	defer e.mu.RUnlock()
	instruments := make([]model.InstrumentID, 0, len(e.externalClaims))
	for instrumentID := range e.externalClaims {
		instruments = append(instruments, instrumentID)
	}
	sort.Slice(instruments, func(i, j int) bool { return instruments[i].String() < instruments[j].String() })
	return instruments
}

func (e *Engine) Health() Health {
	e.mu.RLock()
	defer e.mu.RUnlock()
	componentHealth := kernel.Health{ID: "execution.engine", State: kernel.ComponentStateInitialized}
	if e.component != nil {
		componentHealth = e.component.Health()
	}
	return Health{
		Health:     componentHealth,
		Clients:    len(e.clients),
		Algorithms: len(e.algorithms),
		Starts:     e.starts,
		Stops:      e.stops,
		Submits:    e.submits,
		Cancels:    e.cancels,
		Modifies:   e.modifies,
		Queries:    e.queries,
	}
}

func (e *Engine) Start(ctx context.Context) error {
	e.ensureComponent()
	if e.component.State() == kernel.ComponentStateRunning {
		return nil
	}
	return e.component.Start(ctx)
}

func (e *Engine) Stop(ctx context.Context) error {
	e.ensureComponent()
	if e.component.State() == kernel.ComponentStateStopped {
		return nil
	}
	return e.component.Stop(ctx)
}

func (e *Engine) startClients(ctx context.Context) error {
	e.mu.RLock()
	clients := make([]Client, 0, len(e.clients))
	for _, client := range e.clients {
		clients = append(clients, client)
	}
	e.mu.RUnlock()
	var err error
	for _, client := range clients {
		err = errors.Join(err, client.Connect(ctx))
	}
	if err == nil {
		e.mu.Lock()
		e.starts++
		e.mu.Unlock()
	}
	return err
}

func (e *Engine) stopClients(ctx context.Context) error {
	e.mu.RLock()
	clients := make([]Client, 0, len(e.clients))
	for _, client := range e.clients {
		clients = append(clients, client)
	}
	e.mu.RUnlock()
	var err error
	for _, client := range clients {
		err = errors.Join(err, client.Disconnect(ctx))
	}
	if err == nil {
		e.mu.Lock()
		e.stops++
		e.mu.Unlock()
	}
	return err
}

func (e *Engine) ensureComponent() {
	if e.component != nil {
		return
	}
	e.component = kernel.NewComponent("execution.engine", kernel.ComponentHooks{
		Start: e.startClients,
		Stop:  e.stopClients,
	})
}

func (e *Engine) SubmitOrder(ctx context.Context, order model.SubmitOrder) (model.OrderStatusReport, error) {
	if order.EmulationTrigger.IsActive() {
		return e.submitOrderToEmulator(ctx, order)
	}
	if order.Metadata.ExecAlgorithmID != "" {
		return e.submitOrderToAlgorithm(ctx, order, order.Metadata.ExecAlgorithmID)
	}
	if err := e.manager.CacheSubmitCommand(order); err != nil {
		return model.OrderStatusReport{}, err
	}
	client, err := e.client(order.AccountID)
	if err != nil {
		return model.OrderStatusReport{}, err
	}
	report, err := client.SubmitOrder(ctx, order)
	if err != nil {
		return model.OrderStatusReport{}, err
	}
	if err := e.manager.ApplyOrderReport(report); err != nil {
		return model.OrderStatusReport{}, err
	}
	e.mu.Lock()
	e.submits++
	e.mu.Unlock()
	return report, nil
}

func (e *Engine) submitOrderToEmulator(ctx context.Context, order model.SubmitOrder) (model.OrderStatusReport, error) {
	if e.emulator == nil {
		return model.OrderStatusReport{}, fmt.Errorf("%w: execution emulator is not configured", model.ErrNotSupported)
	}
	report, release, err := e.emulator.SubmitOrder(order)
	if err != nil {
		return model.OrderStatusReport{}, err
	}
	if release != nil {
		e.publishOrderLifecycle(release.Triggered, model.OrderEventTriggered, model.OrderStatusInitialized, "")
		e.publishOrderLifecycle(release.Released, model.OrderEventReleased, model.OrderStatusTriggered, "")
		accepted, err := e.SubmitOrder(ctx, release.Order)
		if err != nil {
			return accepted, err
		}
		e.mu.Lock()
		e.submits++
		e.mu.Unlock()
		return accepted, nil
	}
	e.publishOrderLifecycle(report, model.OrderEventEmulated, model.OrderStatusInitialized, "")
	e.mu.Lock()
	e.submits++
	e.mu.Unlock()
	return report, nil
}

func (e *Engine) submitOrderToAlgorithm(ctx context.Context, order model.SubmitOrder, algorithmID model.ExecAlgorithmID) (model.OrderStatusReport, error) {
	algorithm, err := e.algorithm(algorithmID)
	if err != nil {
		return model.OrderStatusReport{}, err
	}
	report, err := algorithm.SubmitOrder(ctx, order)
	if err != nil {
		return model.OrderStatusReport{}, err
	}
	if err := e.manager.ApplyOrderReport(report); err != nil {
		return model.OrderStatusReport{}, err
	}
	e.mu.Lock()
	e.submits++
	e.mu.Unlock()
	return report, nil
}

func (e *Engine) SubmitOrderList(ctx context.Context, command model.SubmitOrderList) ([]model.OrderStatusReport, error) {
	command = command.WithCommandMetadataDefaults()
	if err := command.Validate(); err != nil {
		return nil, err
	}
	if err := e.manager.IndexOrderList(command.List); err != nil {
		return nil, err
	}
	if algorithmID := command.Metadata.ExecAlgorithmID; algorithmID != "" {
		return e.submitOrderListToAlgorithm(ctx, command, algorithmID)
	}
	client, err := e.client(command.AccountID)
	if err != nil {
		return nil, err
	}
	if submitter, ok := client.(OrderListSubmitter); ok {
		reports, err := submitter.SubmitOrderList(ctx, command)
		if err != nil {
			return reports, err
		}
		if err := e.applyOrderReports(reports); err != nil {
			return reports, err
		}
		e.addSubmits(int64(len(reports)))
		return reports, nil
	}
	reports := make([]model.OrderStatusReport, 0, len(command.List.Orders))
	for _, order := range command.List.Orders {
		if order.ParentClientOrderID != "" {
			continue
		}
		report, err := e.SubmitOrder(ctx, order)
		if err != nil {
			return reports, err
		}
		reports = append(reports, report)
	}
	return reports, nil
}

func (e *Engine) submitOrderListToAlgorithm(ctx context.Context, command model.SubmitOrderList, algorithmID model.ExecAlgorithmID) ([]model.OrderStatusReport, error) {
	algorithm, err := e.algorithm(algorithmID)
	if err != nil {
		return nil, err
	}
	if submitter, ok := algorithm.(OrderListAlgorithm); ok {
		reports, err := submitter.SubmitOrderList(ctx, command)
		if err != nil {
			return reports, err
		}
		if err := e.applyOrderReports(reports); err != nil {
			return reports, err
		}
		e.addSubmits(int64(len(reports)))
		return reports, nil
	}
	reports := make([]model.OrderStatusReport, 0, len(command.List.Orders))
	for _, order := range command.List.Orders {
		report, err := algorithm.SubmitOrder(ctx, order)
		if err != nil {
			return reports, err
		}
		if err := e.manager.ApplyOrderReport(report); err != nil {
			return reports, err
		}
		reports = append(reports, report)
	}
	e.addSubmits(int64(len(reports)))
	return reports, nil
}

func (e *Engine) ProcessMarketEvent(ctx context.Context, event model.MarketEvent) ([]model.OrderStatusReport, error) {
	if e.emulator == nil {
		if err := event.Validate(); err != nil {
			return nil, err
		}
		return nil, nil
	}
	releases, err := e.emulator.ProcessMarketEvent(event)
	if err != nil {
		return nil, err
	}
	reports := make([]model.OrderStatusReport, 0, len(releases))
	for _, release := range releases {
		e.publishOrderLifecycle(release.Triggered, model.OrderEventTriggered, model.OrderStatusEmulated, "")
		e.publishOrderLifecycle(release.Released, model.OrderEventReleased, model.OrderStatusTriggered, "")
		report, err := e.SubmitOrder(ctx, release.Order)
		if err != nil {
			return reports, err
		}
		reports = append(reports, report)
	}
	return reports, nil
}

func (e *Engine) publishOrderLifecycle(report model.OrderStatusReport, kind model.OrderEventKind, previous model.OrderStatus, reason string) {
	if e == nil || e.events == nil {
		return
	}
	reportCopy := report
	lifecycle := model.OrderLifecycleEvent{
		Metadata:       report.Metadata,
		AccountID:      report.AccountID,
		InstrumentID:   report.InstrumentID,
		OrderID:        report.OrderID,
		ClientOrderID:  report.ClientOrderID,
		VenueOrderID:   report.VenueOrderID,
		Kind:           kind,
		PreviousStatus: previous,
		Status:         report.Status,
		Reason:         reason,
		Report:         &reportCopy,
	}
	e.events <- model.ExecutionEvent{Lifecycle: &lifecycle}
}

func (e *Engine) CancelOrder(ctx context.Context, cancel model.CancelOrder) (model.OrderStatusReport, error) {
	if err := cancel.Validate(); err != nil {
		return model.OrderStatusReport{}, err
	}
	if report, handled, err := e.cancelEmulatedOrder(cancel); handled || err != nil {
		if err != nil {
			return report, err
		}
		return report, nil
	}
	client, err := e.client(cancel.AccountID)
	if err != nil {
		return model.OrderStatusReport{}, err
	}
	report, err := client.CancelOrder(ctx, cancel)
	if err != nil {
		return model.OrderStatusReport{}, err
	}
	if err := e.manager.ApplyOrderReport(report); err != nil {
		return model.OrderStatusReport{}, err
	}
	e.mu.Lock()
	e.cancels++
	e.mu.Unlock()
	return report, nil
}

func (e *Engine) cancelEmulatedOrder(cancel model.CancelOrder) (model.OrderStatusReport, bool, error) {
	if e.emulator == nil {
		return model.OrderStatusReport{}, false, nil
	}
	report, handled, err := e.emulator.CancelOrder(cancel)
	if !handled || err != nil {
		return report, handled, err
	}
	e.publishOrderLifecycle(report, model.OrderEventCanceled, model.OrderStatusEmulated, "")
	e.mu.Lock()
	e.cancels++
	e.mu.Unlock()
	return report, true, nil
}

func (e *Engine) BatchCancelOrders(ctx context.Context, batch model.BatchCancelOrders) ([]model.OrderStatusReport, error) {
	if err := batch.Validate(); err != nil {
		return nil, err
	}
	reports := make([]model.OrderStatusReport, 0, len(batch.Cancels))
	remaining := make([]model.CancelOrder, 0, len(batch.Cancels))
	var batchErr error
	for _, cancel := range batch.Cancels {
		cancel.Metadata = cancel.Metadata.WithDefaults(batch.Metadata)
		if cancel.AccountID == "" {
			cancel.AccountID = batch.AccountID
		}
		if cancel.InstrumentID == (model.InstrumentID{}) {
			cancel.InstrumentID = batch.InstrumentID
		}
		report, handled, err := e.cancelEmulatedOrder(cancel)
		if err != nil {
			batchErr = errors.Join(batchErr, err)
			continue
		}
		if handled {
			reports = append(reports, report)
			continue
		}
		remaining = append(remaining, cancel)
	}
	if len(remaining) == 0 {
		return reports, batchErr
	}
	batch.Cancels = remaining
	client, err := e.client(batch.AccountID)
	if err != nil {
		return reports, errors.Join(batchErr, err)
	}
	if canceler, ok := client.(BatchCanceler); ok {
		venueReports, err := canceler.BatchCancelOrders(ctx, batch)
		if err != nil {
			return append(reports, venueReports...), errors.Join(batchErr, err)
		}
		if err := e.applyOrderReports(venueReports); err != nil {
			return append(reports, venueReports...), errors.Join(batchErr, err)
		}
		e.addCancels(int64(len(venueReports)))
		return append(reports, venueReports...), batchErr
	}
	for _, cancel := range batch.Cancels {
		report, err := e.CancelOrder(ctx, cancel)
		if err != nil {
			batchErr = errors.Join(batchErr, err)
			continue
		}
		reports = append(reports, report)
	}
	return reports, batchErr
}

func (e *Engine) CancelAllOrders(ctx context.Context, cancelAll model.CancelAllOrders) ([]model.OrderStatusReport, error) {
	if err := cancelAll.Validate(); err != nil {
		return nil, err
	}
	emulatedBatch := model.BatchCancelOrders{
		Metadata:     cancelAll.Metadata,
		AccountID:    cancelAll.AccountID,
		InstrumentID: cancelAll.InstrumentID,
	}
	nonEmulatedMatches := 0
	for _, order := range e.cache.OpenOrders(cancelAll.AccountID) {
		if !cancelAll.MatchesOrder(order) {
			continue
		}
		if order.Status != model.OrderStatusEmulated {
			nonEmulatedMatches++
			continue
		}
		emulatedBatch.Cancels = append(emulatedBatch.Cancels, model.CancelOrder{
			Metadata:      cancelAll.Metadata,
			AccountID:     order.AccountID,
			InstrumentID:  order.InstrumentID,
			OrderID:       order.OrderID,
			ClientOrderID: order.ClientOrderID,
		})
	}
	localReports := make([]model.OrderStatusReport, 0, len(emulatedBatch.Cancels))
	var localErr error
	if len(emulatedBatch.Cancels) > 0 {
		var err error
		localReports, err = e.BatchCancelOrders(ctx, emulatedBatch)
		localErr = errors.Join(localErr, err)
		if nonEmulatedMatches == 0 {
			return localReports, localErr
		}
	}
	client, err := e.client(cancelAll.AccountID)
	if err != nil {
		return localReports, errors.Join(localErr, err)
	}
	if canceler, ok := client.(CancelAllCanceler); ok {
		reports, err := canceler.CancelAllOrders(ctx, cancelAll)
		if err != nil {
			return append(localReports, reports...), errors.Join(localErr, err)
		}
		if err := e.applyOrderReports(reports); err != nil {
			return append(localReports, reports...), errors.Join(localErr, err)
		}
		e.addCancels(int64(len(reports)))
		return append(localReports, reports...), localErr
	}
	batch := model.BatchCancelOrders{
		Metadata:     cancelAll.Metadata,
		AccountID:    cancelAll.AccountID,
		InstrumentID: cancelAll.InstrumentID,
	}
	for _, order := range e.cache.OpenOrders(cancelAll.AccountID) {
		if !cancelAll.MatchesOrder(order) {
			continue
		}
		batch.Cancels = append(batch.Cancels, model.CancelOrder{
			Metadata:      cancelAll.Metadata,
			AccountID:     order.AccountID,
			InstrumentID:  order.InstrumentID,
			OrderID:       order.OrderID,
			ClientOrderID: order.ClientOrderID,
		})
	}
	if len(batch.Cancels) == 0 {
		return nil, nil
	}
	return e.BatchCancelOrders(ctx, batch)
}

func (e *Engine) ModifyOrder(ctx context.Context, modify model.ModifyOrder) (model.OrderStatusReport, error) {
	if err := modify.Validate(); err != nil {
		return model.OrderStatusReport{}, err
	}
	if e.emulator != nil {
		if report, release, handled, err := e.emulator.ModifyOrder(modify); handled || err != nil {
			if err != nil {
				return report, err
			}
			e.publishOrderLifecycle(report, model.OrderEventUpdated, model.OrderStatusEmulated, "")
			e.mu.Lock()
			e.modifies++
			e.mu.Unlock()
			if release != nil {
				e.publishOrderLifecycle(release.Triggered, model.OrderEventTriggered, model.OrderStatusEmulated, "")
				e.publishOrderLifecycle(release.Released, model.OrderEventReleased, model.OrderStatusTriggered, "")
				return e.SubmitOrder(ctx, release.Order)
			}
			return report, nil
		}
	}
	client, err := e.client(modify.AccountID)
	if err != nil {
		return model.OrderStatusReport{}, err
	}
	modifier, ok := client.(venue.OrderModifier)
	if !ok {
		return model.OrderStatusReport{}, fmt.Errorf("%w: execution client does not support modify", model.ErrNotSupported)
	}
	report, err := modifier.ModifyOrder(ctx, modify)
	if err != nil {
		return model.OrderStatusReport{}, err
	}
	if err := e.manager.ApplyOrderReport(report); err != nil {
		return model.OrderStatusReport{}, err
	}
	e.mu.Lock()
	e.modifies++
	e.mu.Unlock()
	return report, nil
}

func (e *Engine) QueryAccount(ctx context.Context, query model.QueryAccount) (model.AccountSnapshot, error) {
	if err := query.Validate(); err != nil {
		return model.AccountSnapshot{}, err
	}
	client, err := e.client(query.AccountID)
	if err != nil {
		return model.AccountSnapshot{}, err
	}
	snapshot, err := client.QueryAccount(ctx)
	if err != nil {
		return model.AccountSnapshot{}, err
	}
	if snapshot.AccountID == "" {
		snapshot.AccountID = query.AccountID
	}
	if snapshot.Venue == "" {
		snapshot.Venue = client.Venue()
	}
	if err := snapshot.Validate(); err != nil {
		return model.AccountSnapshot{}, err
	}
	e.cache.PutAccount(snapshot)
	return snapshot, nil
}

func (e *Engine) GenerateOrderStatusReports(ctx context.Context, command model.GenerateOrderStatusReports) ([]model.OrderStatusReport, error) {
	if err := command.Validate(); err != nil {
		return nil, err
	}
	client, err := e.client(command.AccountID)
	if err != nil {
		return nil, err
	}
	reports, err := client.GenerateOrderStatusReports(ctx, command.InstrumentID)
	if err != nil {
		return nil, err
	}
	filtered := make([]model.OrderStatusReport, 0, len(reports))
	for _, report := range reports {
		report = e.claimExternalOrderReport(report)
		if !matchesOrderReportCommand(report, command) {
			continue
		}
		if err := e.manager.ApplyOrderReport(report); err != nil {
			return filtered, err
		}
		filtered = append(filtered, report)
	}
	return filtered, nil
}

func (e *Engine) GenerateFillReports(ctx context.Context, command model.GenerateFillReports) ([]model.FillReport, error) {
	if err := command.Validate(); err != nil {
		return nil, err
	}
	client, err := e.client(command.AccountID)
	if err != nil {
		return nil, err
	}
	generator, ok := client.(venue.FillReportGenerator)
	if !ok {
		return nil, fmt.Errorf("%w: execution client does not support fill reports", model.ErrNotSupported)
	}
	fills, err := generator.GenerateFillReports(ctx, command.InstrumentID)
	if err != nil {
		return nil, err
	}
	filtered := make([]model.FillReport, 0, len(fills))
	for _, fill := range fills {
		if !matchesFillReportCommand(fill, command) {
			continue
		}
		if _, err := e.manager.ApplyFill(fill); err != nil {
			return filtered, err
		}
		filtered = append(filtered, fill)
	}
	return filtered, nil
}

func (e *Engine) GeneratePositionStatusReports(ctx context.Context, command model.GeneratePositionStatusReports) ([]model.PositionStatusReport, error) {
	if err := command.Validate(); err != nil {
		return nil, err
	}
	client, err := e.client(command.AccountID)
	if err != nil {
		return nil, err
	}
	generator, ok := client.(venue.PositionStatusReportGenerator)
	if !ok {
		return nil, fmt.Errorf("%w: execution client does not support position reports", model.ErrNotSupported)
	}
	positions, err := generator.GeneratePositionStatusReports(ctx, command.InstrumentID)
	if err != nil {
		return nil, err
	}
	filtered := make([]model.PositionStatusReport, 0, len(positions))
	for _, position := range positions {
		if !matchesPositionReportCommand(position, command) {
			continue
		}
		if err := e.cache.PutPosition(position); err != nil {
			return filtered, err
		}
		filtered = append(filtered, position)
	}
	return filtered, nil
}

func (e *Engine) GenerateExecutionMassStatus(ctx context.Context, command model.GenerateExecutionMassStatus) (model.ExecutionMassStatus, error) {
	if err := command.Validate(); err != nil {
		return model.ExecutionMassStatus{}, err
	}
	account, err := e.QueryAccount(ctx, model.QueryAccount{
		AccountID: command.AccountID,
	})
	if err != nil {
		return model.ExecutionMassStatus{}, err
	}
	orders, err := e.GenerateOrderStatusReports(ctx, model.GenerateOrderStatusReports{
		Metadata:      command.Metadata,
		AccountID:     command.AccountID,
		InstrumentID:  command.InstrumentID,
		OrderID:       command.OrderID,
		VenueOrderID:  command.VenueOrderID,
		ClientOrderID: command.ClientOrderID,
	})
	if err != nil {
		return model.ExecutionMassStatus{}, err
	}
	fills, err := e.GenerateFillReports(ctx, model.GenerateFillReports{
		Metadata:      command.Metadata,
		AccountID:     command.AccountID,
		InstrumentID:  command.InstrumentID,
		OrderID:       command.OrderID,
		VenueOrderID:  command.VenueOrderID,
		ClientOrderID: command.ClientOrderID,
	})
	if err != nil && !errors.Is(err, model.ErrNotSupported) {
		return model.ExecutionMassStatus{}, err
	}
	positions, err := e.GeneratePositionStatusReports(ctx, model.GeneratePositionStatusReports{
		Metadata:        command.Metadata,
		AccountID:       command.AccountID,
		InstrumentID:    command.InstrumentID,
		PositionID:      command.PositionID,
		VenuePositionID: command.VenuePositionID,
	})
	if err != nil && !errors.Is(err, model.ErrNotSupported) {
		return model.ExecutionMassStatus{}, err
	}
	status := model.ExecutionMassStatus{
		Metadata:  command.Metadata,
		AccountID: command.AccountID,
		Venue:     account.Venue,
		Accounts:  []model.AccountSnapshot{account},
		Orders:    orders,
		Fills:     fills,
		Positions: positions,
	}
	if err := status.Validate(); err != nil {
		return model.ExecutionMassStatus{}, err
	}
	return status, nil
}

func (e *Engine) QueryOrder(ctx context.Context, query model.QueryOrder) (model.OrderStatusReport, error) {
	if err := query.Validate(); err != nil {
		return model.OrderStatusReport{}, err
	}
	if isVenueOrderIDOnlyQuery(query) {
		if report, ok := e.cachedOrder(query); ok {
			return report, nil
		}
	}
	client, err := e.client(query.AccountID)
	if err != nil {
		return model.OrderStatusReport{}, err
	}
	querier, ok := client.(venue.OrderQuerier)
	if !ok {
		return e.queryOrderFromReports(ctx, query)
	}
	report, err := querier.QueryOrder(ctx, query)
	if err != nil {
		if errors.Is(err, model.ErrNotSupported) {
			return e.queryOrderFromReports(ctx, query)
		}
		return model.OrderStatusReport{}, err
	}
	if err := e.manager.ApplyOrderReport(report); err != nil {
		return model.OrderStatusReport{}, err
	}
	e.mu.Lock()
	e.queries++
	e.mu.Unlock()
	return report, nil
}

func isVenueOrderIDOnlyQuery(query model.QueryOrder) bool {
	return query.VenueOrderID != "" && query.OrderID == "" && query.ClientOrderID == ""
}

func (e *Engine) cachedOrder(query model.QueryOrder) (model.OrderStatusReport, bool) {
	if query.OrderID != "" {
		if report, ok := e.cache.Order(query.AccountID, query.OrderID); ok {
			return report, true
		}
	}
	if query.ClientOrderID != "" {
		if report, ok := e.cache.OrderByClientID(query.AccountID, query.ClientOrderID); ok {
			return report, true
		}
	}
	if query.VenueOrderID != "" {
		if report, ok := e.cache.OrderByVenueID(query.AccountID, query.VenueOrderID); ok {
			return report, true
		}
	}
	return model.OrderStatusReport{}, false
}

func (e *Engine) queryOrderFromReports(ctx context.Context, query model.QueryOrder) (model.OrderStatusReport, error) {
	reports, err := e.GenerateOrderStatusReports(ctx, model.GenerateOrderStatusReports{
		Metadata:      query.Metadata,
		AccountID:     query.AccountID,
		InstrumentID:  query.InstrumentID,
		OrderID:       query.OrderID,
		VenueOrderID:  query.VenueOrderID,
		ClientOrderID: query.ClientOrderID,
	})
	if err != nil {
		return model.OrderStatusReport{}, err
	}
	if len(reports) == 0 {
		return model.OrderStatusReport{}, fmt.Errorf("%w: order report not found", model.ErrInvalidOrder)
	}
	e.mu.Lock()
	e.queries++
	e.mu.Unlock()
	return reports[0], nil
}

func (e *Engine) applyOrderReports(reports []model.OrderStatusReport) error {
	for _, report := range reports {
		if err := e.manager.ApplyOrderReport(report); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) addSubmits(count int64) {
	e.mu.Lock()
	e.submits += count
	e.mu.Unlock()
}

func (e *Engine) addCancels(count int64) {
	e.mu.Lock()
	e.cancels += count
	e.mu.Unlock()
}

func (e *Engine) claimExternalOrderReport(report model.OrderStatusReport) model.OrderStatusReport {
	if report.Metadata.StrategyID != "" || report.ClientOrderID != "" {
		return report
	}
	e.mu.RLock()
	strategyID := e.externalClaims[report.InstrumentID]
	e.mu.RUnlock()
	if strategyID == "" {
		strategyID = "EXTERNAL"
	}
	report.Metadata.StrategyID = strategyID
	return report
}

func (e *Engine) client(accountID model.AccountID) (Client, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	client := e.clients[accountID]
	if client == nil {
		return nil, fmt.Errorf("%w: account %s", ErrClientNotFound, accountID)
	}
	return client, nil
}

func (e *Engine) algorithm(id model.ExecAlgorithmID) (ExecutionAlgorithm, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	algorithm := e.algorithms[id]
	if algorithm == nil {
		return nil, fmt.Errorf("%w: %s", ErrAlgorithmNotFound, id)
	}
	return algorithm, nil
}

func matchesOrderReportCommand(report model.OrderStatusReport, command model.GenerateOrderStatusReports) bool {
	if report.AccountID != command.AccountID || report.InstrumentID != command.InstrumentID {
		return false
	}
	if command.OrderID != "" && report.OrderID != command.OrderID {
		return false
	}
	if command.VenueOrderID != "" && report.VenueOrderID != command.VenueOrderID {
		return false
	}
	if command.ClientOrderID != "" && report.ClientOrderID != command.ClientOrderID {
		return false
	}
	return true
}

func matchesFillReportCommand(fill model.FillReport, command model.GenerateFillReports) bool {
	if fill.AccountID != command.AccountID || fill.InstrumentID != command.InstrumentID {
		return false
	}
	if command.OrderID != "" && fill.OrderID != command.OrderID {
		return false
	}
	if command.VenueOrderID != "" && fill.VenueOrderID != command.VenueOrderID {
		return false
	}
	if command.ClientOrderID != "" && fill.ClientOrderID != command.ClientOrderID {
		return false
	}
	if command.StartTradeID != "" && fill.TradeID < command.StartTradeID {
		return false
	}
	return true
}

func matchesPositionReportCommand(position model.PositionStatusReport, command model.GeneratePositionStatusReports) bool {
	if position.AccountID != command.AccountID || position.InstrumentID != command.InstrumentID {
		return false
	}
	if command.PositionID != "" && position.PositionID != command.PositionID {
		return false
	}
	if command.VenuePositionID != "" && position.VenuePositionID != command.VenuePositionID {
		return false
	}
	return true
}
