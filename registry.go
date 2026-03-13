package exchanges

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// AdapterConstructor is the function signature that each exchange registers.
// It receives an options map and creates an adapter for the requested market type.
type AdapterConstructor func(ctx context.Context, marketType MarketType, opts map[string]string) (Exchange, error)

var (
	registryMu   sync.RWMutex
	constructors = make(map[string]AdapterConstructor)
)

// Register adds an exchange adapter constructor to the global registry.
// Call this from each exchange adapter's init() function:
//
//	func init() {
//	    exchanges.Register("BINANCE", func(ctx context.Context, mt exchanges.MarketType, opts map[string]string) (exchanges.Exchange, error) {
//	        return NewPerpAdapter(ctx, Options{
//	            APIKey:    opts["api_key"],
//	            SecretKey: opts["secret_key"],
//	        })
//	    })
//	}
func Register(name string, ctor AdapterConstructor) {
	name = strings.ToUpper(strings.TrimSpace(name))
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, exists := constructors[name]; exists {
		panic(fmt.Sprintf("exchanges: exchange %q already registered", name))
	}
	constructors[name] = ctor
}

// LookupConstructor finds a registered constructor by name.
func LookupConstructor(name string) (AdapterConstructor, error) {
	name = strings.ToUpper(strings.TrimSpace(name))
	registryMu.RLock()
	defer registryMu.RUnlock()
	ctor, ok := constructors[name]
	if !ok {
		return nil, fmt.Errorf("unsupported exchange: %s (registered: %v)", name, RegisteredExchanges())
	}
	return ctor, nil
}

// RegisteredExchanges returns the names of all registered exchanges.
func RegisteredExchanges() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	names := make([]string, 0, len(constructors))
	for name := range constructors {
		names = append(names, name)
	}
	return names
}
