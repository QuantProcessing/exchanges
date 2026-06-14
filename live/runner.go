package live

import (
	"context"
	"errors"

	"github.com/QuantProcessing/exchanges/bus"
	"github.com/QuantProcessing/exchanges/cache"
	"github.com/QuantProcessing/exchanges/platform"
	"github.com/QuantProcessing/exchanges/portfolio"
	"github.com/QuantProcessing/exchanges/risk"
	"github.com/QuantProcessing/exchanges/strategy"
	"github.com/QuantProcessing/exchanges/venue"
)

type Config struct {
	Node       *platform.Node
	Bus        *bus.Bus
	Strategies []strategy.Strategy
}

type Runner struct {
	node   *platform.Node
	engine *strategy.Engine
}

type NodeConfig struct {
	Bus              *bus.Bus
	Cache            *cache.Cache
	Risk             *risk.Engine
	Portfolio        *portfolio.Portfolio
	DataClients      []venue.DataClient
	ExecutionClients []venue.ExecutionClient
	Strategies       []strategy.Strategy
}

type TradingNode struct {
	node      *platform.Node
	runner    *Runner
	portfolio *portfolio.Portfolio
}

func NewTradingNode(cfg NodeConfig) (*TradingNode, error) {
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
	node := platform.NewNode(platform.Config{
		Bus:       b,
		Cache:     c,
		Risk:      cfg.Risk,
		Portfolio: pf,
	})
	for _, client := range cfg.DataClients {
		if err := node.AddDataClient(client); err != nil {
			return nil, err
		}
	}
	for _, client := range cfg.ExecutionClients {
		if err := node.AddExecutionClient(client); err != nil {
			return nil, err
		}
	}
	return &TradingNode{
		node:      node,
		runner:    NewRunner(Config{Node: node, Bus: b, Strategies: cfg.Strategies}),
		portfolio: pf,
	}, nil
}

func (n *TradingNode) Start(ctx context.Context) error {
	return n.runner.Start(ctx)
}

func (n *TradingNode) Stop(ctx context.Context) error {
	return n.runner.Stop(ctx)
}

func (n *TradingNode) Platform() *platform.Node { return n.node }
func (n *TradingNode) Cache() *cache.Cache      { return n.node.Cache() }
func (n *TradingNode) Bus() *bus.Bus            { return n.node.Bus() }
func (n *TradingNode) Portfolio() *portfolio.Portfolio {
	return n.portfolio
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
	for _, s := range cfg.Strategies {
		_ = engine.Add(s)
	}
	return &Runner{node: node, engine: engine}
}

func (r *Runner) Start(ctx context.Context) error {
	if err := r.engine.Start(ctx); err != nil {
		return err
	}
	if err := r.node.Start(ctx); err != nil {
		_ = r.engine.Stop(ctx)
		return err
	}
	return nil
}

func (r *Runner) Stop(ctx context.Context) error {
	return errors.Join(r.node.Stop(ctx), r.engine.Stop(ctx))
}
