package platform

import (
	"context"
	"errors"
	"fmt"
	"sync"

	sharedcache "github.com/QuantProcessing/exchanges/cache"
	"github.com/QuantProcessing/exchanges/venue"
)

var (
	ErrNilClient    = errors.New("platform nil client")
	ErrClientExists = errors.New("platform client already registered")
)

type Node struct {
	mu sync.RWMutex

	cache *sharedcache.Cache
	bus   *Bus
	data  *DataEngine
	exec  *ExecutionEngine

	started bool
	ready   bool
}

type NodeHealth struct {
	Started bool
	Ready   bool
}

func NewNode(cfg Config) *Node {
	c := cfg.Cache
	if c == nil {
		c = sharedcache.New()
	}
	bus := cfg.Bus
	if bus == nil {
		bus = NewBus()
	}
	return &Node{
		cache: c,
		bus:   bus,
		data:  NewDataEngine(c),
		exec:  NewExecutionEngine(c, bus),
	}
}

func (n *Node) AddDataClient(name string, client venue.DataClient) error {
	return n.data.AddClient(name, client)
}

func (n *Node) AddExecutionClient(name string, client venue.ExecutionClient) error {
	return n.exec.AddClient(name, client)
}

func (n *Node) Start(ctx context.Context) error {
	n.mu.Lock()
	if n.started {
		n.mu.Unlock()
		return nil
	}
	n.started = true
	n.ready = false
	n.mu.Unlock()

	if err := n.data.Start(ctx); err != nil {
		n.failStart()
		return err
	}
	if err := n.exec.Start(ctx); err != nil {
		_ = n.data.Stop(ctx)
		n.failStart()
		return err
	}

	n.mu.Lock()
	n.ready = true
	n.mu.Unlock()
	return nil
}

func (n *Node) Stop(ctx context.Context) error {
	n.mu.RLock()
	started := n.started
	n.mu.RUnlock()
	if !started {
		return nil
	}

	var stopErr error
	if err := n.exec.Stop(ctx); err != nil {
		stopErr = errors.Join(stopErr, err)
	}
	if err := n.data.Stop(ctx); err != nil {
		stopErr = errors.Join(stopErr, err)
	}

	n.mu.Lock()
	n.started = false
	n.ready = false
	n.mu.Unlock()
	return stopErr
}

func (n *Node) Bus() *Bus {
	return n.bus
}

func (n *Node) Cache() sharedcache.Facade {
	return n.cache.Facade()
}

func (n *Node) Health() NodeHealth {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return NodeHealth{Started: n.started, Ready: n.ready}
}

func (n *Node) failStart() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.started = false
	n.ready = false
}

func validateClientName(name string) error {
	if name == "" {
		return fmt.Errorf("%w: missing client name", ErrNilClient)
	}
	return nil
}
