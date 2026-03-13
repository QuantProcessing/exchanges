package exchanges

import (
	"fmt"
	"sort"
	"sync"
)

// Manager manages multiple exchange adapters
type Manager struct {
	adapters map[string]Exchange
	mu       sync.RWMutex
}

// NewManager creates a new adapter manager
func NewManager() *Manager {
	return &Manager{
		adapters: make(map[string]Exchange),
	}
}

// Register registers an adapter
func (m *Manager) Register(name string, adp Exchange) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.adapters[name] = adp
}

// GetAdapter gets the adapter for a specific exchange
func (m *Manager) GetAdapter(name string) (Exchange, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	adp, ok := m.adapters[name]
	if !ok {
		return nil, fmt.Errorf("exchange not found: %s", name)
	}
	return adp, nil
}

// GetAllAdapters returns all registered adapters
func (m *Manager) GetAllAdapters() []Exchange {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]Exchange, 0, len(m.adapters))
	for _, adp := range m.adapters {
		list = append(list, adp)
	}
	return list
}

// GetExchangeNames returns sorted list of registered exchange names
func (m *Manager) GetExchangeNames() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.adapters))
	for name := range m.adapters {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// CloseAll closes all adapters
func (m *Manager) CloseAll() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, adp := range m.adapters {
		_ = adp.Close()
	}
	return nil
}
