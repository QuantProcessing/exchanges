package cache

import "github.com/QuantProcessing/exchanges/model"

type Residuals struct {
	OpenOrders                   int
	OpenPositions                int
	DeferredFills                int
	InconsistentOrderMappings    int
	InconsistentPositionMappings int
}

func (c *Cache) Residuals(accountID model.AccountID) Residuals {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.residualsLocked(accountID)
}
