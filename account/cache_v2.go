package account

import (
	"fmt"
	"sync"

	"github.com/QuantProcessing/exchanges/model"
)

type V2Cache struct {
	mu          sync.RWMutex
	instruments map[model.InstrumentID]model.Instrument
	accounts    map[v2AccountKey]model.AccountState
}

type v2AccountKey struct {
	venue model.Venue
	id    model.AccountID
}

func NewV2Cache() *V2Cache {
	return &V2Cache{
		instruments: make(map[model.InstrumentID]model.Instrument),
		accounts:    make(map[v2AccountKey]model.AccountState),
	}
}

func (c *V2Cache) PutInstrument(inst model.Instrument) error {
	if err := inst.Validate(); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.instruments[inst.ID] = inst
	return nil
}

func (c *V2Cache) Instrument(id model.InstrumentID) (model.Instrument, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	inst, ok := c.instruments[id]
	return inst, ok
}

func (c *V2Cache) ApplyAccountState(state model.AccountState) error {
	if state.Venue == "" || state.AccountID == "" {
		return fmt.Errorf("%w: missing account state key", model.ErrInvalidAccountState)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.accounts[v2AccountKey{venue: state.Venue, id: state.AccountID}] = state
	return nil
}

func (c *V2Cache) AccountState(v model.Venue, id model.AccountID) (model.AccountState, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	state, ok := c.accounts[v2AccountKey{venue: v, id: id}]
	return state, ok
}
