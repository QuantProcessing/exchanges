package exchanges

import (
	"fmt"
	"strings"
	"sync"
)

// Capabilities describes the stable behaviors an adapter exposes.
//
// It is intentionally separate from Exchange so existing adapters can adopt it
// without a breaking interface change. A false value means either unsupported
// or not claimed by the adapter.
type Capabilities struct {
	PlaceOrder          bool `json:"place_order"`
	PlaceOrderWS        bool `json:"place_order_ws"`
	CancelOrderWS       bool `json:"cancel_order_ws"`
	WatchOrderBook      bool `json:"watch_order_book"`
	WatchOrders         bool `json:"watch_orders"`
	WatchFills          bool `json:"watch_fills"`
	WatchPositions      bool `json:"watch_positions"`
	WatchTicker         bool `json:"watch_ticker"`
	WatchTrades         bool `json:"watch_trades"`
	WatchKlines         bool `json:"watch_klines"`
	FetchOpenOrders     bool `json:"fetch_open_orders"`
	FetchOrderHistory   bool `json:"fetch_order_history"`
	ModifyOrder         bool `json:"modify_order"`
	TransferAsset       bool `json:"transfer_asset"`
	TradingAccountReady bool `json:"trading_account_ready"`
}

// CapabilityProvider is implemented by adapters that expose static support
// claims for user-facing feature discovery.
type CapabilityProvider interface {
	Capabilities() Capabilities
}

type capabilityKey struct {
	exchange   string
	marketType MarketType
}

var (
	capabilityRegistryMu sync.RWMutex
	capabilityRegistry   = make(map[capabilityKey]Capabilities)
)

// GetCapabilities returns an adapter's claimed capabilities when available.
// Values that have not adopted CapabilityProvider return the zero value.
func GetCapabilities(adp any) Capabilities {
	if adp == nil {
		return Capabilities{}
	}
	provider, ok := adp.(CapabilityProvider)
	if !ok {
		return Capabilities{}
	}
	return provider.Capabilities()
}

// RegisterCapabilities records an exchange/market capability claim.
func RegisterCapabilities(name string, marketType MarketType, caps Capabilities) {
	key := capabilityRegistryKey(name, marketType)

	capabilityRegistryMu.Lock()
	defer capabilityRegistryMu.Unlock()
	if _, exists := capabilityRegistry[key]; exists {
		panic(fmt.Sprintf("exchanges: capabilities for %q/%s already registered", key.exchange, key.marketType))
	}
	capabilityRegistry[key] = caps
}

// LookupCapabilities finds registered capability claims for an exchange/market.
func LookupCapabilities(name string, marketType MarketType) (Capabilities, bool) {
	key := capabilityRegistryKey(name, marketType)

	capabilityRegistryMu.RLock()
	defer capabilityRegistryMu.RUnlock()
	caps, ok := capabilityRegistry[key]
	return caps, ok
}

func capabilityRegistryKey(name string, marketType MarketType) capabilityKey {
	return capabilityKey{
		exchange:   strings.ToUpper(strings.TrimSpace(name)),
		marketType: MarketType(strings.ToLower(strings.TrimSpace(string(marketType)))),
	}
}
