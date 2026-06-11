package account

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	exchanges "github.com/QuantProcessing/exchanges"
)

// PortfolioAccountKey identifies one managed TradingAccount instance.
// Quote is the normal grouping key for spot/perp accounts; Instrument is
// available for venues or future account types that need finer partitioning.
type PortfolioAccountKey struct {
	Exchange   string
	MarketType exchanges.MarketType
	Quote      exchanges.QuoteCurrency
	Instrument string
}

// PortfolioPosition is a position annotated with the account it came from.
type PortfolioPosition struct {
	Key      PortfolioAccountKey
	Position exchanges.Position
}

// PortfolioBalance is a spot balance annotated with the account it came from.
type PortfolioBalance struct {
	Key     PortfolioAccountKey
	Balance exchanges.SpotBalance
}

// PortfolioOrder is an open order annotated with the account it came from.
type PortfolioOrder struct {
	Key   PortfolioAccountKey
	Order exchanges.Order
}

type portfolioAccountEntry struct {
	perp *PerpTradingAccount
	spot *SpotTradingAccount
}

// PortfolioAccount composes multiple TradingAccount runtimes without adding
// routing policy. It is intentionally read-only in this phase: callers still
// place, cancel, and modify orders through the concrete TradingAccount they
// selected.
type PortfolioAccount struct {
	mu       sync.RWMutex
	accounts map[PortfolioAccountKey]portfolioAccountEntry
}

func NewPortfolioAccount() *PortfolioAccount {
	return &PortfolioAccount{accounts: make(map[PortfolioAccountKey]portfolioAccountEntry)}
}

func (p *PortfolioAccount) AddPerp(key PortfolioAccountKey, acct *PerpTradingAccount) error {
	if acct == nil {
		return fmt.Errorf("portfolio account: perp account required")
	}
	if key.MarketType == "" {
		key.MarketType = exchanges.MarketTypePerp
	}
	if key.MarketType != exchanges.MarketTypePerp {
		return fmt.Errorf("portfolio account: key market type %q is not perp", key.MarketType)
	}
	return p.add(key, portfolioAccountEntry{perp: acct})
}

func (p *PortfolioAccount) AddSpot(key PortfolioAccountKey, acct *SpotTradingAccount) error {
	if acct == nil {
		return fmt.Errorf("portfolio account: spot account required")
	}
	if key.MarketType == "" {
		key.MarketType = exchanges.MarketTypeSpot
	}
	if key.MarketType != exchanges.MarketTypeSpot {
		return fmt.Errorf("portfolio account: key market type %q is not spot", key.MarketType)
	}
	return p.add(key, portfolioAccountEntry{spot: acct})
}

func (p *PortfolioAccount) add(key PortfolioAccountKey, entry portfolioAccountEntry) error {
	key = normalizePortfolioAccountKey(key)
	if key.Exchange == "" {
		return fmt.Errorf("portfolio account: exchange required")
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.accounts == nil {
		p.accounts = make(map[PortfolioAccountKey]portfolioAccountEntry)
	}
	if _, exists := p.accounts[key]; exists {
		return fmt.Errorf("portfolio account: duplicate account key %s", key.String())
	}
	p.accounts[key] = entry
	return nil
}

func (p *PortfolioAccount) Keys() []PortfolioAccountKey {
	entries := p.snapshot()
	keys := make([]PortfolioAccountKey, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}
	sortPortfolioAccountKeys(keys)
	return keys
}

func (p *PortfolioAccount) Positions() []PortfolioPosition {
	entries := p.snapshot()
	keys := sortedPortfolioEntryKeys(entries)

	out := make([]PortfolioPosition, 0)
	for _, key := range keys {
		entry := entries[key]
		if entry.perp == nil {
			continue
		}
		for _, position := range entry.perp.Positions() {
			out = append(out, PortfolioPosition{Key: key, Position: position})
		}
	}
	return out
}

func (p *PortfolioAccount) Balances() []PortfolioBalance {
	entries := p.snapshot()
	keys := sortedPortfolioEntryKeys(entries)

	out := make([]PortfolioBalance, 0)
	for _, key := range keys {
		entry := entries[key]
		if entry.spot == nil {
			continue
		}
		for _, balance := range entry.spot.Balances() {
			out = append(out, PortfolioBalance{Key: key, Balance: balance})
		}
	}
	return out
}

func (p *PortfolioAccount) OpenOrders() []PortfolioOrder {
	entries := p.snapshot()
	keys := sortedPortfolioEntryKeys(entries)

	out := make([]PortfolioOrder, 0)
	for _, key := range keys {
		entry := entries[key]
		var orders []exchanges.Order
		if entry.perp != nil {
			orders = entry.perp.OpenOrders()
		} else if entry.spot != nil {
			orders = entry.spot.OpenOrders()
		}
		for _, order := range orders {
			out = append(out, PortfolioOrder{Key: key, Order: order})
		}
	}
	return out
}

func (p *PortfolioAccount) Health() map[PortfolioAccountKey]TradingAccountHealth {
	entries := p.snapshot()
	out := make(map[PortfolioAccountKey]TradingAccountHealth, len(entries))
	for key, entry := range entries {
		if entry.perp != nil {
			out[key] = entry.perp.Health()
		} else if entry.spot != nil {
			out[key] = entry.spot.Health()
		}
	}
	return out
}

func (p *PortfolioAccount) Close() {
	entries := p.snapshot()
	for _, key := range sortedPortfolioEntryKeys(entries) {
		entry := entries[key]
		if entry.perp != nil {
			entry.perp.Close()
		}
		if entry.spot != nil {
			entry.spot.Close()
		}
	}
}

func (p *PortfolioAccount) snapshot() map[PortfolioAccountKey]portfolioAccountEntry {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make(map[PortfolioAccountKey]portfolioAccountEntry, len(p.accounts))
	for key, entry := range p.accounts {
		out[key] = entry
	}
	return out
}

func normalizePortfolioAccountKey(key PortfolioAccountKey) PortfolioAccountKey {
	key.Exchange = strings.ToUpper(strings.TrimSpace(key.Exchange))
	key.Instrument = strings.ToUpper(strings.TrimSpace(key.Instrument))
	return key
}

func (key PortfolioAccountKey) String() string {
	parts := []string{key.Exchange, string(key.MarketType), string(key.Quote)}
	if key.Instrument != "" {
		parts = append(parts, key.Instrument)
	}
	return strings.Join(parts, "/")
}

func sortedPortfolioEntryKeys(entries map[PortfolioAccountKey]portfolioAccountEntry) []PortfolioAccountKey {
	keys := make([]PortfolioAccountKey, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}
	sortPortfolioAccountKeys(keys)
	return keys
}

func sortPortfolioAccountKeys(keys []PortfolioAccountKey) {
	sort.Slice(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		if a.Exchange != b.Exchange {
			return a.Exchange < b.Exchange
		}
		if a.MarketType != b.MarketType {
			return a.MarketType < b.MarketType
		}
		if a.Quote != b.Quote {
			return a.Quote < b.Quote
		}
		return a.Instrument < b.Instrument
	})
}
