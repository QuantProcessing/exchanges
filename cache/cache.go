package cache

import (
	"sort"
	"sync"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/shopspring/decimal"
)

type Cache struct {
	mu           sync.RWMutex
	instruments  map[model.InstrumentID]model.Instrument
	accounts     map[model.AccountID]model.AccountSnapshot
	orders       map[model.AccountID]map[model.OrderID]model.OrderStatusReport
	orderClients map[model.AccountID]map[model.ClientOrderID]model.OrderID
	orderVenues  map[model.AccountID]map[model.VenueOrderID]model.OrderID
	fills        map[model.AccountID]map[model.OrderID]map[model.TradeID]model.FillReport
	positions    map[model.AccountID]map[model.PositionID]model.PositionStatusReport
	positionInst map[model.AccountID]map[model.InstrumentID]model.PositionID
	tickers      map[model.InstrumentID]model.Ticker
	orderBooks   map[model.InstrumentID]model.OrderBook
	trades       map[model.InstrumentID]model.TradeTick
	quotes       map[model.InstrumentID]model.QuoteTick
	bars         map[model.BarType]model.Bar
	latestBars   map[model.InstrumentID]model.Bar
}

func New() *Cache {
	return &Cache{
		instruments:  make(map[model.InstrumentID]model.Instrument),
		accounts:     make(map[model.AccountID]model.AccountSnapshot),
		orders:       make(map[model.AccountID]map[model.OrderID]model.OrderStatusReport),
		orderClients: make(map[model.AccountID]map[model.ClientOrderID]model.OrderID),
		orderVenues:  make(map[model.AccountID]map[model.VenueOrderID]model.OrderID),
		fills:        make(map[model.AccountID]map[model.OrderID]map[model.TradeID]model.FillReport),
		positions:    make(map[model.AccountID]map[model.PositionID]model.PositionStatusReport),
		positionInst: make(map[model.AccountID]map[model.InstrumentID]model.PositionID),
		tickers:      make(map[model.InstrumentID]model.Ticker),
		orderBooks:   make(map[model.InstrumentID]model.OrderBook),
		trades:       make(map[model.InstrumentID]model.TradeTick),
		quotes:       make(map[model.InstrumentID]model.QuoteTick),
		bars:         make(map[model.BarType]model.Bar),
		latestBars:   make(map[model.InstrumentID]model.Bar),
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

func (c *Cache) Instruments() []model.Instrument {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]model.Instrument, 0, len(c.instruments))
	for _, inst := range c.instruments {
		out = append(out, inst)
	}
	return out
}

func (c *Cache) PutAccount(account model.AccountSnapshot) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.accounts[account.AccountID] = account
}

func (c *Cache) Account(id model.AccountID) (model.AccountSnapshot, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	account, ok := c.accounts[id]
	return account, ok
}

func (c *Cache) PutOrder(order model.OrderStatusReport) error {
	order = normalizeOrder(order)
	if err := order.Validate(); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.orders[order.AccountID] == nil {
		c.orders[order.AccountID] = make(map[model.OrderID]model.OrderStatusReport)
	}
	if c.orderClients[order.AccountID] == nil {
		c.orderClients[order.AccountID] = make(map[model.ClientOrderID]model.OrderID)
	}
	if c.orderVenues[order.AccountID] == nil {
		c.orderVenues[order.AccountID] = make(map[model.VenueOrderID]model.OrderID)
	}
	c.orders[order.AccountID][order.OrderID] = order
	if order.ClientOrderID != "" {
		c.orderClients[order.AccountID][order.ClientOrderID] = order.OrderID
	}
	if order.VenueOrderID != "" {
		c.orderVenues[order.AccountID][order.VenueOrderID] = order.OrderID
	}
	return nil
}

func (c *Cache) Order(accountID model.AccountID, orderID model.OrderID) (model.OrderStatusReport, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.orders[accountID] == nil {
		return model.OrderStatusReport{}, false
	}
	order, ok := c.orders[accountID][orderID]
	return order, ok
}

func (c *Cache) OrderByClientID(accountID model.AccountID, clientOrderID model.ClientOrderID) (model.OrderStatusReport, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	orderID, ok := c.orderClients[accountID][clientOrderID]
	if !ok {
		return model.OrderStatusReport{}, false
	}
	order, ok := c.orders[accountID][orderID]
	return order, ok
}

func (c *Cache) OrderByVenueID(accountID model.AccountID, venueOrderID model.VenueOrderID) (model.OrderStatusReport, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	orderID, ok := c.orderVenues[accountID][venueOrderID]
	if !ok {
		return model.OrderStatusReport{}, false
	}
	order, ok := c.orders[accountID][orderID]
	return order, ok
}

func (c *Cache) Orders(accountID model.AccountID) []model.OrderStatusReport {
	c.mu.RLock()
	defer c.mu.RUnlock()
	orders := make([]model.OrderStatusReport, 0, len(c.orders[accountID]))
	for _, order := range c.orders[accountID] {
		orders = append(orders, order)
	}
	sort.Slice(orders, func(i, j int) bool {
		return orders[i].OrderID < orders[j].OrderID
	})
	return orders
}

func (c *Cache) OpenOrders(accountID model.AccountID) []model.OrderStatusReport {
	c.mu.RLock()
	defer c.mu.RUnlock()
	orders := make([]model.OrderStatusReport, 0, len(c.orders[accountID]))
	for _, order := range c.orders[accountID] {
		if order.Status.IsOpen() {
			orders = append(orders, order)
		}
	}
	sort.Slice(orders, func(i, j int) bool {
		return orders[i].OrderID < orders[j].OrderID
	})
	return orders
}

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
	return true, nil
}

func (c *Cache) FillsForOrder(accountID model.AccountID, orderID model.OrderID) []model.FillReport {
	c.mu.RLock()
	defer c.mu.RUnlock()
	fills := make([]model.FillReport, 0, len(c.fills[accountID][orderID]))
	for _, fill := range c.fills[accountID][orderID] {
		fills = append(fills, fill)
	}
	sort.Slice(fills, func(i, j int) bool {
		if fills[i].Timestamp.Equal(fills[j].Timestamp) {
			return fills[i].TradeID < fills[j].TradeID
		}
		return fills[i].Timestamp.Before(fills[j].Timestamp)
	})
	return fills
}

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
	c.positions[position.AccountID][position.PositionID] = position
	c.positionInst[position.AccountID][position.InstrumentID] = position.PositionID
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

func normalizeOrder(order model.OrderStatusReport) model.OrderStatusReport {
	if order.Quantity.IsPositive() && order.LeavesQuantity.IsZero() && order.Status.IsOpen() {
		order.LeavesQuantity = order.Quantity.Sub(order.FilledQuantity)
	}
	if order.Status == model.OrderStatusFilled && order.Quantity.IsPositive() && order.FilledQuantity.IsZero() {
		order.FilledQuantity = order.Quantity
	}
	if order.Status == model.OrderStatusFilled {
		order.LeavesQuantity = decimal.Zero
	}
	return order
}
