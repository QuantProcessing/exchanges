package live

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/QuantProcessing/exchanges/bus"
	"github.com/QuantProcessing/exchanges/kernel"
	"github.com/QuantProcessing/exchanges/platform"
	"github.com/QuantProcessing/exchanges/strategy"
)

type Config struct {
	Node       *platform.Node
	Bus        *bus.Bus
	Strategies []strategy.Strategy
}

const fatalMonitorInterval = 10 * time.Millisecond

type Runner struct {
	mu             sync.RWMutex
	stopMu         sync.Mutex
	node           *platform.Node
	engine         *strategy.Engine
	component      *kernel.Component
	strategyIDs    []string
	strategyStates map[string]kernel.ComponentState
	runCancel      context.CancelFunc
}

func NewRunner(cfg Config) *Runner {
	b := cfg.Bus
	if b == nil && cfg.Node != nil {
		b = cfg.Node.Bus()
	}
	if b == nil {
		b = bus.New()
	}
	node := cfg.Node
	if node == nil {
		node = platform.NewNode(platform.Config{Bus: b})
	}
	engine := strategy.NewEngine(b, strategy.WithRuntime(node))
	strategyIDs := make([]string, 0, len(cfg.Strategies))
	strategyStates := make(map[string]kernel.ComponentState, len(cfg.Strategies))
	for _, s := range cfg.Strategies {
		if s != nil {
			id := s.ID()
			strategyIDs = append(strategyIDs, id)
			strategyStates[id] = kernel.ComponentStateInitialized
		}
		_ = engine.Add(s)
	}
	return &Runner{
		node:           node,
		engine:         engine,
		component:      kernel.NewComponent("live.runner", kernel.ComponentHooks{}),
		strategyIDs:    strategyIDs,
		strategyStates: strategyStates,
	}
}

func (r *Runner) Start(ctx context.Context) (err error) {
	r.ensureComponent()
	if r.component.State() == kernel.ComponentStateFaulted {
		return kernel.ErrComponentFaulted
	}
	runCtx, cancel := context.WithCancel(context.Background())
	r.stopMu.Lock()
	r.runCancel = cancel
	r.stopMu.Unlock()
	defer func() {
		if err != nil {
			cancel()
			r.stopMu.Lock()
			r.runCancel = nil
			r.stopMu.Unlock()
			r.component.Fault(err)
		}
	}()
	if err := r.node.Start(ctx); err != nil {
		return err
	}
	r.setAllStrategiesState(kernel.ComponentStateStarting)
	if err := r.engine.Start(ctx); err != nil {
		r.setAllStrategiesState(kernel.ComponentStateFaulted)
		_ = r.node.Stop(ctx)
		return err
	}
	r.setAllStrategiesState(kernel.ComponentStateRunning)
	if err := r.component.Start(ctx); err != nil {
		return err
	}
	go r.monitorFatal(runCtx)
	return nil
}

func (r *Runner) Stop(ctx context.Context) error {
	r.ensureComponent()
	r.stopMu.Lock()
	defer r.stopMu.Unlock()
	if r.component.State() == kernel.ComponentStateStopped {
		return nil
	}
	if r.runCancel != nil {
		r.runCancel()
		r.runCancel = nil
	}
	r.setAllStrategiesState(kernel.ComponentStateStopping)
	engineErr := r.engine.Stop(ctx)
	if engineErr != nil {
		r.setAllStrategiesState(kernel.ComponentStateFaulted)
	} else {
		r.setAllStrategiesState(kernel.ComponentStateStopped)
	}
	stopErr := errors.Join(engineErr, r.node.Stop(ctx))
	if stopErr != nil {
		r.component.Fault(stopErr)
		return stopErr
	}
	return r.component.Stop(ctx)
}

func (r *Runner) monitorFatal(ctx context.Context) {
	ticker := time.NewTicker(fatalMonitorInterval)
	defer ticker.Stop()
	engineErrs := r.engine.Errors()
	for {
		select {
		case <-ctx.Done():
			return
		case err := <-engineErrs:
			if err != nil {
				r.requestGracefulShutdown(err)
				return
			}
		case <-ticker.C:
			health := r.node.Health()
			if health.LastError != nil && health.State != kernel.ComponentStateStopped {
				r.requestGracefulShutdown(health.LastError)
				return
			}
		}
	}
}

func (r *Runner) requestGracefulShutdown(err error) {
	if err == nil {
		return
	}
	r.ensureComponent()
	r.component.Degrade(err)
	_ = r.Stop(context.Background())
}

func (r *Runner) Health() Health {
	if r == nil {
		return Health{State: kernel.ComponentStateInitialized}
	}
	r.ensureComponent()
	componentHealth := r.component.Health()
	health := Health{
		State:     componentHealth.State,
		LastError: componentHealth.LastError,
	}
	health.Strategies = r.strategyHealth()
	if r.node != nil {
		health.Platform = r.node.Health()
	}
	return health
}

func (r *Runner) ensureComponent() {
	if r.component == nil {
		r.component = kernel.NewComponent("live.runner", kernel.ComponentHooks{})
	}
}

func (r *Runner) setAllStrategiesState(state kernel.ComponentState) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.strategyStates == nil {
		r.strategyStates = make(map[string]kernel.ComponentState)
	}
	for _, id := range r.strategyIDs {
		r.strategyStates[id] = state
	}
}

func (r *Runner) strategyHealth() []StrategyHealth {
	r.mu.RLock()
	defer r.mu.RUnlock()
	snapshots := make([]StrategyHealth, 0, len(r.strategyIDs))
	for _, id := range r.strategyIDs {
		state := r.strategyStates[id]
		if state == "" {
			state = kernel.ComponentStateInitialized
		}
		snapshots = append(snapshots, StrategyHealth{
			ID:    id,
			State: state,
		})
	}
	return snapshots
}
