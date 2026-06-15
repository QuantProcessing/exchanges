package cache

import "github.com/QuantProcessing/exchanges/model"

func (c *Cache) PutMarketEvent(event model.MarketEvent) error {
	if err := event.Validate(); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if event.Ticker != nil {
		c.tickers[event.Ticker.InstrumentID] = *event.Ticker
	}
	if event.OrderBook != nil {
		c.orderBooks[event.OrderBook.InstrumentID] = *event.OrderBook
	}
	if event.Trade != nil {
		c.trades[event.Trade.InstrumentID] = *event.Trade
	}
	if event.Quote != nil {
		c.quotes[event.Quote.InstrumentID] = *event.Quote
	}
	if event.Bar != nil {
		bar := *event.Bar
		bar.BarType = bar.BarType.Canonical()
		c.bars[bar.BarType] = bar
		existing, ok := c.latestBars[bar.BarType.InstrumentID]
		if !ok || bar.Timestamp.After(existing.Timestamp) || bar.Timestamp.Equal(existing.Timestamp) {
			c.latestBars[bar.BarType.InstrumentID] = bar
		}
	}
	if event.Custom != nil {
		custom := *event.Custom
		if c.customData[custom.InstrumentID] == nil {
			c.customData[custom.InstrumentID] = make(map[string]model.CustomData)
		}
		c.customData[custom.InstrumentID][custom.Type] = custom
	}
	return nil
}

func (c *Cache) Ticker(instrumentID model.InstrumentID) (model.Ticker, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ticker, ok := c.tickers[instrumentID]
	return ticker, ok
}

func (c *Cache) OrderBook(instrumentID model.InstrumentID) (model.OrderBook, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	book, ok := c.orderBooks[instrumentID]
	return book, ok
}

func (c *Cache) TradeTick(instrumentID model.InstrumentID) (model.TradeTick, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	trade, ok := c.trades[instrumentID]
	return trade, ok
}

func (c *Cache) QuoteTick(instrumentID model.InstrumentID) (model.QuoteTick, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	quote, ok := c.quotes[instrumentID]
	return quote, ok
}

func (c *Cache) Bar(barType model.BarType) (model.Bar, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	bar, ok := c.bars[barType.Canonical()]
	return bar, ok
}

func (c *Cache) LatestBar(instrumentID model.InstrumentID) (model.Bar, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	bar, ok := c.latestBars[instrumentID]
	return bar, ok
}

func (c *Cache) CustomData(instrumentID model.InstrumentID, typ string) (model.CustomData, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	data, ok := c.customData[instrumentID][typ]
	return data, ok
}
