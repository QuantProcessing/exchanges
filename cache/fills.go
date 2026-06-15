package cache

import "github.com/QuantProcessing/exchanges/model"

func (c *Cache) PutFill(fill model.FillReport) (bool, error) {
	if err := fill.Validate(); err != nil {
		return false, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fills[fill.AccountID] == nil {
		c.fills[fill.AccountID] = make(map[model.OrderID]map[model.TradeID]model.FillReport)
	}
	if c.fills[fill.AccountID][fill.OrderID] == nil {
		c.fills[fill.AccountID][fill.OrderID] = make(map[model.TradeID]model.FillReport)
	}
	if _, ok := c.fills[fill.AccountID][fill.OrderID][fill.TradeID]; ok {
		return false, nil
	}
	c.fills[fill.AccountID][fill.OrderID][fill.TradeID] = fill
	if c.fillTrades[fill.AccountID] == nil {
		c.fillTrades[fill.AccountID] = make(map[model.TradeID]model.FillReport)
	}
	c.fillTrades[fill.AccountID][fill.TradeID] = fill
	if fill.VenueOrderID != "" {
		if c.fillVenues[fill.AccountID] == nil {
			c.fillVenues[fill.AccountID] = make(map[model.VenueOrderID]map[model.TradeID]model.FillReport)
		}
		if c.fillVenues[fill.AccountID][fill.VenueOrderID] == nil {
			c.fillVenues[fill.AccountID][fill.VenueOrderID] = make(map[model.TradeID]model.FillReport)
		}
		c.fillVenues[fill.AccountID][fill.VenueOrderID][fill.TradeID] = fill
	}
	return true, nil
}

func (c *Cache) FillByTradeID(accountID model.AccountID, tradeID model.TradeID) (model.FillReport, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	fill, ok := c.fillTrades[accountID][tradeID]
	return fill, ok
}

func (c *Cache) FillsByVenueOrderID(accountID model.AccountID, venueOrderID model.VenueOrderID) []model.FillReport {
	c.mu.RLock()
	defer c.mu.RUnlock()
	fills := make([]model.FillReport, 0, len(c.fillVenues[accountID][venueOrderID]))
	for _, fill := range c.fillVenues[accountID][venueOrderID] {
		fills = append(fills, fill)
	}
	sortFills(fills)
	return fills
}

func (c *Cache) FillsForOrder(accountID model.AccountID, orderID model.OrderID) []model.FillReport {
	c.mu.RLock()
	defer c.mu.RUnlock()
	fills := make([]model.FillReport, 0, len(c.fills[accountID][orderID]))
	for _, fill := range c.fills[accountID][orderID] {
		fills = append(fills, fill)
	}
	sortFills(fills)
	return fills
}

func (c *Cache) PutDeferredFill(fill model.FillReport) (bool, error) {
	if err := fill.Validate(); err != nil {
		return false, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.deferredFills[fill.AccountID] == nil {
		c.deferredFills[fill.AccountID] = make(map[model.OrderID]map[model.TradeID]model.FillReport)
	}
	if c.deferredFills[fill.AccountID][fill.OrderID] == nil {
		c.deferredFills[fill.AccountID][fill.OrderID] = make(map[model.TradeID]model.FillReport)
	}
	if _, ok := c.deferredFills[fill.AccountID][fill.OrderID][fill.TradeID]; ok {
		return false, nil
	}
	c.deferredFills[fill.AccountID][fill.OrderID][fill.TradeID] = fill
	return true, nil
}

func (c *Cache) DeferredFillsForOrder(accountID model.AccountID, orderID model.OrderID) []model.FillReport {
	c.mu.RLock()
	defer c.mu.RUnlock()
	fills := make([]model.FillReport, 0, len(c.deferredFills[accountID][orderID]))
	for _, fill := range c.deferredFills[accountID][orderID] {
		fills = append(fills, fill)
	}
	sortFills(fills)
	return fills
}

func (c *Cache) ClearDeferredFillsForOrder(accountID model.AccountID, orderID model.OrderID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.deferredFills[accountID], orderID)
}
