package backtest

import (
	"context"

	"github.com/QuantProcessing/exchanges/bus"
	"github.com/QuantProcessing/exchanges/data"
	"github.com/QuantProcessing/exchanges/strategy"
)

type EngineConfig struct {
	Bus         *bus.Bus
	Catalog     Catalog
	DataCatalog data.ReplayCatalog
}

type Engine struct {
	bus         *bus.Bus
	catalog     Catalog
	dataCatalog data.ReplayCatalog
	events      []Event
	strategies  []strategy.Strategy
}

func NewEngine(cfg EngineConfig) *Engine {
	b := cfg.Bus
	if b == nil {
		b = bus.New()
	}
	return &Engine{bus: b, catalog: cfg.Catalog, dataCatalog: cfg.DataCatalog}
}

func (e *Engine) AddData(event Event) {
	e.events = append(e.events, event)
}

func (e *Engine) AddStrategy(strategy strategy.Strategy) {
	e.strategies = append(e.strategies, strategy)
}

func (e *Engine) Run(ctx context.Context) (Result, error) {
	events := append([]Event(nil), e.events...)
	if e.catalog != nil {
		catalogEvents, err := e.catalog.Events(ctx)
		if err != nil {
			return Result{}, err
		}
		events = append(events, catalogEvents...)
	}
	if e.dataCatalog != nil {
		catalogEvents, err := e.dataCatalog.Events(ctx)
		if err != nil {
			return Result{}, err
		}
		for _, event := range catalogEvents {
			events = append(events, Event{
				At:      data.EventTime(event),
				Topic:   strategy.TopicMarketData,
				Message: event,
			})
		}
	}
	return NewRunner(Config{
		Bus:         e.bus,
		DataCatalog: e.dataCatalog,
		Events:      events,
		Strategies:  e.strategies,
	}).Run(ctx)
}
