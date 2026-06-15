package cache

import "github.com/QuantProcessing/exchanges/model"

func (c *Cache) PutAccount(account model.AccountSnapshot) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.accounts[account.AccountID] = account
	c.accountHistory[account.AccountID] = append(c.accountHistory[account.AccountID], account)
}

func (c *Cache) Account(id model.AccountID) (model.AccountSnapshot, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	account, ok := c.accounts[id]
	return account, ok
}

func (c *Cache) AccountHistory(id model.AccountID) []model.AccountSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return append([]model.AccountSnapshot(nil), c.accountHistory[id]...)
}
