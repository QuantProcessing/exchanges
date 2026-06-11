package account

import (
	"fmt"
	"sync"

	"github.com/QuantProcessing/exchanges/model"
)

type Cache struct {
	mu          sync.RWMutex
	instruments map[model.InstrumentID]model.Instrument
	accounts    map[accountKey]model.AccountState
}

type accountKey struct {
	venue model.Venue
	id    model.AccountID
}

func NewCache() *Cache {
	return &Cache{
		instruments: make(map[model.InstrumentID]model.Instrument),
		accounts:    make(map[accountKey]model.AccountState),
	}
}

func (c *Cache) PutInstrument(inst model.Instrument) error {
	if err := inst.Validate(); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.instruments[inst.ID] = inst
	return nil
}

func (c *Cache) Instrument(id model.InstrumentID) (model.Instrument, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	inst, ok := c.instruments[id]
	return inst, ok
}

func (c *Cache) ApplyAccountState(state model.AccountState) error {
	if state.Venue == "" || state.AccountID == "" {
		return fmt.Errorf("%w: missing account state key", model.ErrInvalidAccountState)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.accounts[accountKey{venue: state.Venue, id: state.AccountID}] = state
	return nil
}

func (c *Cache) AccountState(v model.Venue, id model.AccountID) (model.AccountState, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	state, ok := c.accounts[accountKey{venue: v, id: id}]
	return state, ok
}
