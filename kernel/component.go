package kernel

import (
	"context"
	"errors"
	"sync"
)

var ErrComponentFaulted = errors.New("component faulted")

type ComponentState string

const (
	ComponentStateInitialized ComponentState = "initialized"
	ComponentStateStarting    ComponentState = "starting"
	ComponentStateRunning     ComponentState = "running"
	ComponentStateStopping    ComponentState = "stopping"
	ComponentStateStopped     ComponentState = "stopped"
	ComponentStateDegraded    ComponentState = "degraded"
	ComponentStateFaulted     ComponentState = "faulted"
)

type ComponentHooks struct {
	Start func(context.Context) error
	Stop  func(context.Context) error
	Kill  func(context.Context) error
}

type Component struct {
	mu      sync.RWMutex
	id      string
	state   ComponentState
	lastErr error
	hooks   ComponentHooks
}

func NewComponent(id string, hooks ComponentHooks) *Component {
	return &Component{id: id, state: ComponentStateInitialized, hooks: hooks}
}

func (c *Component) State() ComponentState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

func (c *Component) Health() Health {
	c.mu.RLock()
	defer c.mu.RUnlock()
	health := Health{ID: c.id, State: c.state}
	if c.lastErr != nil {
		health.LastError = c.lastErr.Error()
	}
	return health
}

func (c *Component) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.state == ComponentStateFaulted {
		c.mu.Unlock()
		return ErrComponentFaulted
	}
	c.state = ComponentStateStarting
	c.lastErr = nil
	hook := c.hooks.Start
	c.mu.Unlock()

	if hook != nil {
		if err := hook(ctx); err != nil {
			c.Fault(err)
			return err
		}
	}
	c.mu.Lock()
	c.state = ComponentStateRunning
	c.mu.Unlock()
	return nil
}

func (c *Component) Stop(ctx context.Context) error {
	c.mu.Lock()
	if c.state == ComponentStateFaulted {
		c.mu.Unlock()
		return ErrComponentFaulted
	}
	c.state = ComponentStateStopping
	hook := c.hooks.Stop
	c.mu.Unlock()

	if hook != nil {
		if err := hook(ctx); err != nil {
			c.Fault(err)
			return err
		}
	}
	c.mu.Lock()
	c.state = ComponentStateStopped
	c.mu.Unlock()
	return nil
}

func (c *Component) Kill(ctx context.Context) error {
	hook := c.hooks.Kill
	if hook != nil {
		if err := hook(ctx); err != nil {
			c.Fault(err)
			return err
		}
	}
	c.mu.Lock()
	c.state = ComponentStateStopped
	c.mu.Unlock()
	return nil
}

func (c *Component) Degrade(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastErr = err
	if c.state != ComponentStateFaulted {
		c.state = ComponentStateDegraded
	}
}

func (c *Component) Fault(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastErr = err
	c.state = ComponentStateFaulted
}
