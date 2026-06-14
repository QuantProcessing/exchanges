package backtest

import (
	"context"

	"github.com/QuantProcessing/exchanges/bus"
	"github.com/QuantProcessing/exchanges/strategy"
)

type EngineConfig struct {
	Bus *bus.Bus
}

type Engine struct {
	bus        *bus.Bus
	events     []Event
	strategies []strategy.Strategy
}

func NewEngine(cfg EngineConfig) *Engine {
	b := cfg.Bus
	if b == nil {
		b = bus.New()
	}
	return &Engine{bus: b}
}

func (e *Engine) AddData(event Event) {
	e.events = append(e.events, event)
}

func (e *Engine) AddStrategy(strategy strategy.Strategy) {
	e.strategies = append(e.strategies, strategy)
}

func (e *Engine) Run(ctx context.Context) (Result, error) {
	return NewRunner(Config{
		Bus:        e.bus,
		Events:     e.events,
		Strategies: e.strategies,
	}).Run(ctx)
}
