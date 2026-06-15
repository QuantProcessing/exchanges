package cache

import (
	"sort"

	"github.com/QuantProcessing/exchanges/model"
)

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

func (c *Cache) Instruments() []model.Instrument {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]model.Instrument, 0, len(c.instruments))
	for _, inst := range c.instruments {
		out = append(out, inst)
	}
	return out
}

func (c *Cache) PutSyntheticInstrument(inst model.SyntheticInstrument) error {
	if err := inst.Validate(); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.synthetics[inst.ID] = inst
	return nil
}

func (c *Cache) SyntheticInstrument(id model.InstrumentID) (model.SyntheticInstrument, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	inst, ok := c.synthetics[id]
	return inst, ok
}

func (c *Cache) SyntheticInstruments() []model.SyntheticInstrument {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]model.SyntheticInstrument, 0, len(c.synthetics))
	for _, inst := range c.synthetics {
		out = append(out, inst)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID.String() < out[j].ID.String()
	})
	return out
}
