package cache

import (
	"sort"

	"github.com/QuantProcessing/exchanges/model"
)

type Snapshot struct {
	AccountID       model.AccountID
	Account         model.AccountSnapshot
	AccountHistory  []model.AccountSnapshot
	OpenOrders      []model.OrderStatusReport
	ClosedOrders    []model.OrderStatusReport
	OpenPositions   []model.PositionStatusReport
	ClosedPositions []model.PositionStatusReport
	DeferredFills   []model.FillReport
	Residuals       Residuals
}

type PurgePolicy struct {
	ClosedOrdersLimit     int
	ClosedPositionsLimit  int
	AccountSnapshotsLimit int
}

type PurgeResult struct {
	ClosedOrders     int
	ClosedPositions  int
	AccountSnapshots int
}

func (c *Cache) Snapshot(accountID model.AccountID) Snapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()

	snapshot := Snapshot{
		AccountID:       accountID,
		Account:         c.accounts[accountID],
		AccountHistory:  append([]model.AccountSnapshot(nil), c.accountHistory[accountID]...),
		OpenOrders:      make([]model.OrderStatusReport, 0),
		ClosedOrders:    make([]model.OrderStatusReport, 0),
		OpenPositions:   make([]model.PositionStatusReport, 0),
		ClosedPositions: make([]model.PositionStatusReport, 0),
		DeferredFills:   c.deferredFillsLocked(accountID),
	}
	for _, order := range c.orders[accountID] {
		if order.Status.IsOpen() {
			snapshot.OpenOrders = append(snapshot.OpenOrders, order)
			continue
		}
		if order.Status != "" {
			snapshot.ClosedOrders = append(snapshot.ClosedOrders, order)
		}
	}
	for _, position := range c.positions[accountID] {
		if positionIsOpen(position) {
			snapshot.OpenPositions = append(snapshot.OpenPositions, position)
			continue
		}
		snapshot.ClosedPositions = append(snapshot.ClosedPositions, position)
	}
	sortOrders(snapshot.OpenOrders)
	sortOrders(snapshot.ClosedOrders)
	sortPositions(snapshot.OpenPositions)
	sortPositions(snapshot.ClosedPositions)
	snapshot.Residuals = c.residualsLocked(accountID)
	return snapshot
}

func (c *Cache) Purge(accountID model.AccountID, policy PurgePolicy) PurgeResult {
	c.mu.Lock()
	defer c.mu.Unlock()

	result := PurgeResult{}
	for _, order := range closedOrdersToPurge(c.orders[accountID], policy.ClosedOrdersLimit) {
		c.removeOrderLocked(order)
		result.ClosedOrders++
	}
	for _, position := range closedPositionsToPurge(c.positions[accountID], policy.ClosedPositionsLimit) {
		c.removePositionLocked(position)
		result.ClosedPositions++
	}
	if policy.AccountSnapshotsLimit >= 0 {
		history := c.accountHistory[accountID]
		if len(history) > policy.AccountSnapshotsLimit {
			result.AccountSnapshots = len(history) - policy.AccountSnapshotsLimit
			c.accountHistory[accountID] = append([]model.AccountSnapshot(nil), history[result.AccountSnapshots:]...)
			if len(c.accountHistory[accountID]) == 0 {
				delete(c.accounts, accountID)
			} else {
				c.accounts[accountID] = c.accountHistory[accountID][len(c.accountHistory[accountID])-1]
			}
		}
	}
	return result
}

func (c *Cache) removeOrderLocked(order model.OrderStatusReport) {
	c.removeOrderIndexesLocked(order)
	delete(c.orders[order.AccountID], order.OrderID)
}

func (c *Cache) removePositionLocked(position model.PositionStatusReport) {
	delete(c.positions[position.AccountID], position.PositionID)
	if c.positionInst[position.AccountID][position.InstrumentID] == position.PositionID {
		delete(c.positionInst[position.AccountID], position.InstrumentID)
	}
	if position.VenuePositionID != "" && c.positionVenues[position.AccountID][position.VenuePositionID] == position.PositionID {
		delete(c.positionVenues[position.AccountID], position.VenuePositionID)
	}
	removePositionIndex(c.positionStrategies, position.AccountID, position.Metadata.StrategyID, position.PositionID)
}

func (c *Cache) deferredFillsLocked(accountID model.AccountID) []model.FillReport {
	fills := make([]model.FillReport, 0)
	for _, byTrade := range c.deferredFills[accountID] {
		for _, fill := range byTrade {
			fills = append(fills, fill)
		}
	}
	sortFills(fills)
	return fills
}

func (c *Cache) residualsLocked(accountID model.AccountID) Residuals {
	residuals := Residuals{}
	for _, order := range c.orders[accountID] {
		if order.Status.IsOpen() {
			residuals.OpenOrders++
		}
	}
	for _, position := range c.positions[accountID] {
		if positionIsOpen(position) {
			residuals.OpenPositions++
		}
	}
	for _, byTrade := range c.deferredFills[accountID] {
		residuals.DeferredFills += len(byTrade)
	}
	for _, orderID := range c.orderClients[accountID] {
		if _, ok := c.orders[accountID][orderID]; !ok {
			residuals.InconsistentOrderMappings++
		}
	}
	for _, orderID := range c.orderVenues[accountID] {
		if _, ok := c.orders[accountID][orderID]; !ok {
			residuals.InconsistentOrderMappings++
		}
	}
	for _, positionID := range c.positionInst[accountID] {
		if _, ok := c.positions[accountID][positionID]; !ok {
			residuals.InconsistentPositionMappings++
		}
	}
	for _, positionID := range c.positionVenues[accountID] {
		if _, ok := c.positions[accountID][positionID]; !ok {
			residuals.InconsistentPositionMappings++
		}
	}
	for _, ids := range c.positionStrategies[accountID] {
		for positionID := range ids {
			if _, ok := c.positions[accountID][positionID]; !ok {
				residuals.InconsistentPositionMappings++
			}
		}
	}
	return residuals
}

func closedOrdersToPurge(orders map[model.OrderID]model.OrderStatusReport, keep int) []model.OrderStatusReport {
	if keep < 0 {
		return nil
	}
	closed := make([]model.OrderStatusReport, 0, len(orders))
	for _, order := range orders {
		if order.Status != "" && !order.Status.IsOpen() {
			closed = append(closed, order)
		}
	}
	sort.Slice(closed, func(i, j int) bool {
		if closed[i].LastUpdatedTime.Equal(closed[j].LastUpdatedTime) {
			return closed[i].OrderID < closed[j].OrderID
		}
		return closed[i].LastUpdatedTime.Before(closed[j].LastUpdatedTime)
	})
	if len(closed) <= keep {
		return nil
	}
	return closed[:len(closed)-keep]
}

func closedPositionsToPurge(positions map[model.PositionID]model.PositionStatusReport, keep int) []model.PositionStatusReport {
	if keep < 0 {
		return nil
	}
	closed := make([]model.PositionStatusReport, 0, len(positions))
	for _, position := range positions {
		if !positionIsOpen(position) {
			closed = append(closed, position)
		}
	}
	sort.Slice(closed, func(i, j int) bool {
		if closed[i].Timestamp.Equal(closed[j].Timestamp) {
			return closed[i].PositionID < closed[j].PositionID
		}
		return closed[i].Timestamp.Before(closed[j].Timestamp)
	})
	if len(closed) <= keep {
		return nil
	}
	return closed[:len(closed)-keep]
}
