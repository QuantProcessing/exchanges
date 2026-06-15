package live

import (
	"context"

	"github.com/QuantProcessing/exchanges/bus"
	"github.com/QuantProcessing/exchanges/cache"
	"github.com/QuantProcessing/exchanges/platform"
	"github.com/QuantProcessing/exchanges/portfolio"
	"github.com/QuantProcessing/exchanges/risk"
)

type Node struct {
	platform  *platform.Node
	runner    *Runner
	portfolio *portfolio.Portfolio
	risk      *risk.Engine
}

type TradingNode = Node

func NewNode(cfg NodeConfig) (*Node, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	b := cfg.Bus
	if b == nil {
		b = bus.New()
	}
	c := cfg.Cache
	if c == nil {
		c = cache.New()
	}
	r := cfg.Risk
	if r == nil {
		r = risk.NewEngine(c, cfg.RiskConfig)
	}
	pf := cfg.Portfolio
	if pf == nil {
		pf = portfolio.New(c)
	}
	node := platform.NewNode(platform.Config{
		Bus:             b,
		Cache:           c,
		Risk:            r,
		Portfolio:       pf,
		ReconnectPolicy: cfg.ReconnectPolicy,
		Logger:          cfg.Logger,
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
	return &Node{
		platform:  node,
		runner:    NewRunner(Config{Node: node, Bus: b, Strategies: cfg.Strategies}),
		portfolio: pf,
		risk:      r,
	}, nil
}

func NewTradingNode(cfg NodeConfig) (*TradingNode, error) {
	return NewNode(cfg)
}

func (n *Node) Start(ctx context.Context) error {
	return n.runner.Start(ctx)
}

func (n *Node) Stop(ctx context.Context) error {
	return n.runner.Stop(ctx)
}

func (n *Node) Health() Health {
	return n.runner.Health()
}

func (n *Node) Platform() *platform.Node { return n.platform }
func (n *Node) Cache() *cache.Cache      { return n.platform.Cache() }
func (n *Node) Bus() *bus.Bus            { return n.platform.Bus() }
func (n *Node) Portfolio() *portfolio.Portfolio {
	return n.portfolio
}
func (n *Node) Risk() *risk.Engine { return n.risk }
