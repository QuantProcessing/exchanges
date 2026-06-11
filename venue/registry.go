package venue

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/QuantProcessing/exchanges/model"
)

var ErrUnknownVenue = errors.New("unknown venue")

type Constructor func(ctx context.Context, cfg map[string]string) (Adapter, error)

type Registry struct {
	mu    sync.RWMutex
	ctors map[model.Venue]Constructor
}

func NewRegistry() *Registry {
	return &Registry{ctors: make(map[model.Venue]Constructor)}
}

func (r *Registry) Register(v model.Venue, ctor Constructor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ctors[v] = ctor
}

func (r *Registry) Open(ctx context.Context, v model.Venue, cfg map[string]string) (Adapter, error) {
	r.mu.RLock()
	ctor := r.ctors[v]
	r.mu.RUnlock()
	if ctor == nil {
		return nil, fmt.Errorf("%w: %s", ErrUnknownVenue, v)
	}
	return ctor(ctx, cfg)
}
