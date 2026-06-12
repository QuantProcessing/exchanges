package platform

import (
	"context"
	"errors"
	"fmt"
	"sync"

	sharedcache "github.com/QuantProcessing/exchanges/cache"
	"github.com/QuantProcessing/exchanges/venue"
)

type DataEngine struct {
	mu      sync.RWMutex
	cache   *sharedcache.Cache
	clients map[string]venue.DataClient
}

func NewDataEngine(cache *sharedcache.Cache) *DataEngine {
	if cache == nil {
		cache = sharedcache.New()
	}
	return &DataEngine{
		cache:   cache,
		clients: make(map[string]venue.DataClient),
	}
}

func (e *DataEngine) AddClient(name string, client venue.DataClient) error {
	if err := validateClientName(name); err != nil {
		return err
	}
	if client == nil {
		return fmt.Errorf("%w: data %s", ErrNilClient, name)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.clients[name]; exists {
		return fmt.Errorf("%w: data %s", ErrClientExists, name)
	}
	e.clients[name] = client
	return nil
}

func (e *DataEngine) Start(ctx context.Context) error {
	clients := e.clientsSnapshot()
	for _, client := range clients {
		if err := client.Connect(ctx); err != nil {
			return err
		}
		if provider := client.Instruments(); provider != nil {
			for _, inst := range provider.List() {
				if err := e.cache.PutInstrument(inst); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (e *DataEngine) Stop(ctx context.Context) error {
	clients := e.clientsSnapshot()
	var stopErr error
	for _, client := range clients {
		if err := client.Disconnect(ctx); err != nil {
			stopErr = errors.Join(stopErr, err)
		}
	}
	return stopErr
}

func (e *DataEngine) clientsSnapshot() []venue.DataClient {
	e.mu.RLock()
	defer e.mu.RUnlock()
	clients := make([]venue.DataClient, 0, len(e.clients))
	for _, client := range e.clients {
		clients = append(clients, client)
	}
	return clients
}
