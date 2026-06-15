package backtest

import "context"

type Catalog interface {
	Events(context.Context) ([]Event, error)
}

type MemoryCatalog struct {
	events []Event
}

func NewMemoryCatalog(events ...Event) *MemoryCatalog {
	return &MemoryCatalog{events: append([]Event(nil), events...)}
}

func (c *MemoryCatalog) Add(event Event) {
	if c == nil {
		return
	}
	c.events = append(c.events, event)
}

func (c *MemoryCatalog) Events(context.Context) ([]Event, error) {
	if c == nil {
		return nil, nil
	}
	return append([]Event(nil), c.events...), nil
}
