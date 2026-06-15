package cache

import (
	"sort"

	"github.com/QuantProcessing/exchanges/model"
)

func (c *Cache) PutPosition(position model.PositionStatusReport) error {
	if err := position.Validate(); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.positions[position.AccountID] == nil {
		c.positions[position.AccountID] = make(map[model.PositionID]model.PositionStatusReport)
	}
	if c.positionInst[position.AccountID] == nil {
		c.positionInst[position.AccountID] = make(map[model.InstrumentID]model.PositionID)
	}
	if c.positionVenues[position.AccountID] == nil {
		c.positionVenues[position.AccountID] = make(map[model.VenuePositionID]model.PositionID)
	}
	if c.positionStrategies[position.AccountID] == nil {
		c.positionStrategies[position.AccountID] = make(map[model.StrategyID]map[model.PositionID]struct{})
	}
	if existing, ok := c.positions[position.AccountID][position.PositionID]; ok {
		if existing.VenuePositionID != "" && existing.VenuePositionID != position.VenuePositionID {
			delete(c.positionVenues[position.AccountID], existing.VenuePositionID)
		}
		removePositionIndex(c.positionStrategies, existing.AccountID, existing.Metadata.StrategyID, existing.PositionID)
	}
	c.positions[position.AccountID][position.PositionID] = position
	c.positionInst[position.AccountID][position.InstrumentID] = position.PositionID
	if position.VenuePositionID != "" {
		c.positionVenues[position.AccountID][position.VenuePositionID] = position.PositionID
	}
	addPositionIndex(c.positionStrategies, position.AccountID, position.Metadata.StrategyID, position.PositionID)
	return nil
}

func (c *Cache) Position(accountID model.AccountID, positionID model.PositionID) (model.PositionStatusReport, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	position, ok := c.positions[accountID][positionID]
	return position, ok
}

func (c *Cache) PositionByInstrument(accountID model.AccountID, instrumentID model.InstrumentID) (model.PositionStatusReport, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	positionID, ok := c.positionInst[accountID][instrumentID]
	if !ok {
		return model.PositionStatusReport{}, false
	}
	position, ok := c.positions[accountID][positionID]
	return position, ok
}

func (c *Cache) PositionByVenueID(accountID model.AccountID, venuePositionID model.VenuePositionID) (model.PositionStatusReport, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	positionID, ok := c.positionVenues[accountID][venuePositionID]
	if !ok {
		return model.PositionStatusReport{}, false
	}
	position, ok := c.positions[accountID][positionID]
	return position, ok
}

func (c *Cache) Positions(accountID model.AccountID) []model.PositionStatusReport {
	c.mu.RLock()
	defer c.mu.RUnlock()
	positions := make([]model.PositionStatusReport, 0, len(c.positions[accountID]))
	for _, position := range c.positions[accountID] {
		positions = append(positions, position)
	}
	sortPositions(positions)
	return positions
}

func (c *Cache) OpenPositions(accountID model.AccountID) []model.PositionStatusReport {
	c.mu.RLock()
	defer c.mu.RUnlock()
	positions := make([]model.PositionStatusReport, 0, len(c.positions[accountID]))
	for _, position := range c.positions[accountID] {
		if positionIsOpen(position) {
			positions = append(positions, position)
		}
	}
	sortPositions(positions)
	return positions
}

func (c *Cache) ClosedPositions(accountID model.AccountID) []model.PositionStatusReport {
	c.mu.RLock()
	defer c.mu.RUnlock()
	positions := make([]model.PositionStatusReport, 0, len(c.positions[accountID]))
	for _, position := range c.positions[accountID] {
		if !positionIsOpen(position) {
			positions = append(positions, position)
		}
	}
	sortPositions(positions)
	return positions
}

func (c *Cache) PositionsByStrategy(accountID model.AccountID, strategyID model.StrategyID) []model.PositionStatusReport {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.positionsByIDSetLocked(accountID, c.positionStrategies[accountID][strategyID])
}

func (c *Cache) PositionsForInstrument(instrumentID model.InstrumentID) []model.PositionStatusReport {
	c.mu.RLock()
	defer c.mu.RUnlock()
	positions := make([]model.PositionStatusReport, 0)
	for _, byPositionID := range c.positions {
		for _, position := range byPositionID {
			if position.InstrumentID == instrumentID {
				positions = append(positions, position)
			}
		}
	}
	sort.Slice(positions, func(i, j int) bool {
		if positions[i].AccountID == positions[j].AccountID {
			return positions[i].PositionID < positions[j].PositionID
		}
		return positions[i].AccountID < positions[j].AccountID
	})
	return positions
}
