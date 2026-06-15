package cache

import (
	"sort"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/shopspring/decimal"
)

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

func (c *Cache) addOrderIndexesLocked(order model.OrderStatusReport) {
	addOrderIndex(c.orderStrategies, order.AccountID, order.Metadata.StrategyID, order.OrderID)
	addOrderIndex(c.orderPositions, order.AccountID, order.PositionID, order.OrderID)
	addOrderIndex(c.orderLists, order.AccountID, order.OrderListID, order.OrderID)
	addOrderIndex(c.orderExecSpawns, order.AccountID, order.Metadata.ExecSpawnID, order.OrderID)
}

func (c *Cache) removeOrderIndexesLocked(order model.OrderStatusReport) {
	removeOrderIndex(c.orderStrategies, order.AccountID, order.Metadata.StrategyID, order.OrderID)
	removeOrderIndex(c.orderPositions, order.AccountID, order.PositionID, order.OrderID)
	removeOrderIndex(c.orderLists, order.AccountID, order.OrderListID, order.OrderID)
	removeOrderIndex(c.orderExecSpawns, order.AccountID, order.Metadata.ExecSpawnID, order.OrderID)
	if order.ClientOrderID != "" {
		delete(c.orderClients[order.AccountID], order.ClientOrderID)
	}
	if order.VenueOrderID != "" {
		delete(c.orderVenues[order.AccountID], order.VenueOrderID)
	}
}

func addOrderIndex[K comparable](index map[model.AccountID]map[K]map[model.OrderID]struct{}, accountID model.AccountID, key K, orderID model.OrderID) {
	var zero K
	if key == zero {
		return
	}
	if index[accountID] == nil {
		index[accountID] = make(map[K]map[model.OrderID]struct{})
	}
	if index[accountID][key] == nil {
		index[accountID][key] = make(map[model.OrderID]struct{})
	}
	index[accountID][key][orderID] = struct{}{}
}

func removeOrderIndex[K comparable](index map[model.AccountID]map[K]map[model.OrderID]struct{}, accountID model.AccountID, key K, orderID model.OrderID) {
	var zero K
	if key == zero || index[accountID] == nil || index[accountID][key] == nil {
		return
	}
	delete(index[accountID][key], orderID)
	if len(index[accountID][key]) == 0 {
		delete(index[accountID], key)
	}
}

func addPositionIndex[K comparable](index map[model.AccountID]map[K]map[model.PositionID]struct{}, accountID model.AccountID, key K, positionID model.PositionID) {
	var zero K
	if key == zero {
		return
	}
	if index[accountID] == nil {
		index[accountID] = make(map[K]map[model.PositionID]struct{})
	}
	if index[accountID][key] == nil {
		index[accountID][key] = make(map[model.PositionID]struct{})
	}
	index[accountID][key][positionID] = struct{}{}
}

func removePositionIndex[K comparable](index map[model.AccountID]map[K]map[model.PositionID]struct{}, accountID model.AccountID, key K, positionID model.PositionID) {
	var zero K
	if key == zero || index[accountID] == nil || index[accountID][key] == nil {
		return
	}
	delete(index[accountID][key], positionID)
	if len(index[accountID][key]) == 0 {
		delete(index[accountID], key)
	}
}

func (c *Cache) ordersByIDSetLocked(accountID model.AccountID, ids map[model.OrderID]struct{}) []model.OrderStatusReport {
	orders := make([]model.OrderStatusReport, 0, len(ids))
	for orderID := range ids {
		if order, ok := c.orders[accountID][orderID]; ok {
			orders = append(orders, order)
		}
	}
	sortOrders(orders)
	return orders
}

func (c *Cache) positionsByIDSetLocked(accountID model.AccountID, ids map[model.PositionID]struct{}) []model.PositionStatusReport {
	positions := make([]model.PositionStatusReport, 0, len(ids))
	for positionID := range ids {
		if position, ok := c.positions[accountID][positionID]; ok {
			positions = append(positions, position)
		}
	}
	sortPositions(positions)
	return positions
}

func sortOrders(orders []model.OrderStatusReport) {
	sort.Slice(orders, func(i, j int) bool {
		return orders[i].OrderID < orders[j].OrderID
	})
}

func sortFills(fills []model.FillReport) {
	sort.Slice(fills, func(i, j int) bool {
		if fills[i].Timestamp.Equal(fills[j].Timestamp) {
			return fills[i].TradeID < fills[j].TradeID
		}
		return fills[i].Timestamp.Before(fills[j].Timestamp)
	})
}

func sortPositions(positions []model.PositionStatusReport) {
	sort.Slice(positions, func(i, j int) bool {
		if positions[i].InstrumentID == positions[j].InstrumentID {
			return positions[i].PositionID < positions[j].PositionID
		}
		return positions[i].InstrumentID.String() < positions[j].InstrumentID.String()
	})
}

func positionIsOpen(position model.PositionStatusReport) bool {
	return position.Side != model.PositionSideFlat && !position.Quantity.IsZero()
}
