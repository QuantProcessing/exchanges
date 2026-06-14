package venue

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/QuantProcessing/exchanges/model"
)

var ErrUnknownVenue = errors.New("unknown venue")

type Constructor func(context.Context, map[string]string) (Adapter, error)

type Registry struct {
	mu    sync.RWMutex
	ctors map[model.Venue]Constructor
}

func NewRegistry() *Registry {
	return &Registry{ctors: make(map[model.Venue]Constructor)}
}

func (r *Registry) Register(venue model.Venue, ctor Constructor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ctors[venue] = ctor
}

func (r *Registry) Open(ctx context.Context, venue model.Venue, cfg map[string]string) (Adapter, error) {
	r.mu.RLock()
	ctor := r.ctors[venue]
	r.mu.RUnlock()
	if ctor == nil {
		return nil, fmt.Errorf("%w: %s", ErrUnknownVenue, venue)
	}
	return ctor(ctx, cfg)
}

var defaultRegistry = NewRegistry()

func Register(v model.Venue, ctor Constructor) {
	defaultRegistry.Register(v, ctor)
}

func Open(ctx context.Context, v model.Venue, cfg map[string]string) (Adapter, error) {
	return defaultRegistry.Open(ctx, v, cfg)
}
