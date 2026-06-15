package cache

import (
	"sort"

	"github.com/QuantProcessing/exchanges/model"
)

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
	if existing, ok := c.orders[order.AccountID][order.OrderID]; ok {
		c.removeOrderIndexesLocked(existing)
	}
	if order.ClientOrderID != "" {
		if existingOrderID := c.orderClients[order.AccountID][order.ClientOrderID]; existingOrderID != "" && existingOrderID != order.OrderID {
			if existing, ok := c.orders[order.AccountID][existingOrderID]; ok {
				c.removeOrderIndexesLocked(existing)
				delete(c.orders[order.AccountID], existingOrderID)
			}
		}
	}
	if order.VenueOrderID != "" {
		if existingOrderID := c.orderVenues[order.AccountID][order.VenueOrderID]; existingOrderID != "" && existingOrderID != order.OrderID {
			if existing, ok := c.orders[order.AccountID][existingOrderID]; ok {
				c.removeOrderIndexesLocked(existing)
				delete(c.orders[order.AccountID], existingOrderID)
			}
		}
	}
	c.orders[order.AccountID][order.OrderID] = order
	if order.ClientOrderID != "" {
		c.orderClients[order.AccountID][order.ClientOrderID] = order.OrderID
	}
	if order.VenueOrderID != "" {
		c.orderVenues[order.AccountID][order.VenueOrderID] = order.OrderID
	}
	c.addOrderIndexesLocked(order)
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

func (c *Cache) ClosedOrders(accountID model.AccountID) []model.OrderStatusReport {
	c.mu.RLock()
	defer c.mu.RUnlock()
	orders := make([]model.OrderStatusReport, 0, len(c.orders[accountID]))
	for _, order := range c.orders[accountID] {
		if order.Status != "" && !order.Status.IsOpen() {
			orders = append(orders, order)
		}
	}
	sortOrders(orders)
	return orders
}

func (c *Cache) OrdersByStrategy(accountID model.AccountID, strategyID model.StrategyID) []model.OrderStatusReport {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ordersByIDSetLocked(accountID, c.orderStrategies[accountID][strategyID])
}

func (c *Cache) OrdersByPositionID(accountID model.AccountID, positionID model.PositionID) []model.OrderStatusReport {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ordersByIDSetLocked(accountID, c.orderPositions[accountID][positionID])
}

func (c *Cache) OrdersByOrderListID(accountID model.AccountID, orderListID model.OrderListID) []model.OrderStatusReport {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ordersByIDSetLocked(accountID, c.orderLists[accountID][orderListID])
}

func (c *Cache) OrdersByExecSpawnID(accountID model.AccountID, execSpawnID model.ExecSpawnID) []model.OrderStatusReport {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ordersByIDSetLocked(accountID, c.orderExecSpawns[accountID][execSpawnID])
}
