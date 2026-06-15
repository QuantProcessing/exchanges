package live

import (
	"fmt"
	"reflect"

	"github.com/QuantProcessing/exchanges/bus"
	"github.com/QuantProcessing/exchanges/cache"
	"github.com/QuantProcessing/exchanges/portfolio"
	"github.com/QuantProcessing/exchanges/risk"
	"github.com/QuantProcessing/exchanges/strategy"
	"github.com/QuantProcessing/exchanges/venue"
)

type NodeBuilder struct {
	cfg NodeConfig
}

func NewNodeBuilder() *NodeBuilder {
	return &NodeBuilder{}
}

func (b *NodeBuilder) WithBus(bus *bus.Bus) *NodeBuilder {
	b.cfg.Bus = bus
	return b
}

func (b *NodeBuilder) WithCache(cache *cache.Cache) *NodeBuilder {
	b.cfg.Cache = cache
	return b
}

func (b *NodeBuilder) WithRisk(risk *risk.Engine) *NodeBuilder {
	b.cfg.Risk = risk
	return b
}

func (b *NodeBuilder) WithRiskConfig(cfg risk.Config) *NodeBuilder {
	b.cfg.RiskConfig = cfg
	return b
}

func (b *NodeBuilder) WithPortfolio(portfolio *portfolio.Portfolio) *NodeBuilder {
	b.cfg.Portfolio = portfolio
	return b
}

func (b *NodeBuilder) WithReconnectPolicy(policy RetryPolicy) *NodeBuilder {
	b.cfg.ReconnectPolicy = policy
	return b
}

func (b *NodeBuilder) AddDataClient(client venue.DataClient) *NodeBuilder {
	b.cfg.DataClients = append(b.cfg.DataClients, client)
	return b
}

func (b *NodeBuilder) AddExecutionClient(client venue.ExecutionClient) *NodeBuilder {
	b.cfg.ExecutionClients = append(b.cfg.ExecutionClients, client)
	return b
}

func (b *NodeBuilder) AddStrategy(strategy strategy.Strategy) *NodeBuilder {
	b.cfg.Strategies = append(b.cfg.Strategies, strategy)
	return b
}

func (b *NodeBuilder) Build() (*Node, error) {
	if b == nil {
		return NewNode(NodeConfig{})
	}
	return NewNode(b.cfg)
}

func (cfg NodeConfig) Validate() error {
	for i, client := range cfg.DataClients {
		if isNilComponent(client) {
			return fmt.Errorf("data client %d is nil", i)
		}
	}
	for i, client := range cfg.ExecutionClients {
		if isNilComponent(client) {
			return fmt.Errorf("execution client %d is nil", i)
		}
	}
	for i, strategy := range cfg.Strategies {
		if isNilComponent(strategy) {
			return fmt.Errorf("strategy %d is nil", i)
		}
	}
	return nil
}

func isNilComponent(value any) bool {
	if value == nil {
		return true
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}
