package lighter

import "fmt"

// Private Channel Subscriptions (require authentication)

// SubscribeAccountAll subscribes to all account data
func (c *WebsocketClient) SubscribeAccountAll(accountId int64, authToken string, cb func([]byte)) error {
	channel := fmt.Sprintf("account_all/%d", accountId)
	return c.Subscribe(channel, &authToken, cb)
}

// SubscribeAccountMarket subscribes to account data for a specific market
func (c *WebsocketClient) SubscribeAccountMarket(marketId int, accountId int64, authToken string, cb func([]byte)) error {
	channel := fmt.Sprintf("account_market/%d/%d", marketId, accountId)
	return c.Subscribe(channel, &authToken, cb)
}

// SubscribeUserStats subscribes to user statistics
func (c *WebsocketClient) SubscribeUserStats(accountId int64, authToken string, cb func([]byte)) error {
	channel := fmt.Sprintf("user_stats/%d", accountId)
	return c.Subscribe(channel, &authToken, cb)
}

// SubscribeAccountTx subscribes to account transactions
func (c *WebsocketClient) SubscribeAccountTx(accountId int64, authToken string, cb func([]byte)) error {
	channel := fmt.Sprintf("account_tx/%d", accountId)
	return c.Subscribe(channel, &authToken, cb)
}

// SubscribeAccountAllOrders subscribes to all account orders
func (c *WebsocketClient) SubscribeAccountAllOrders(accountId int64, authToken string, cb func([]byte)) error {
	channel := fmt.Sprintf("account_all_orders/%d", accountId)
	return c.Subscribe(channel, &authToken, cb)
}

// SubscribeAccountOrders subscribes to account orders for a specific market
func (c *WebsocketClient) SubscribeAccountOrders(marketId int, accountId int64, authToken string, cb func([]byte)) error {
	channel := fmt.Sprintf("account_orders/%d/%d", marketId, accountId)
	return c.Subscribe(channel, &authToken, cb)
}

// SubscribeNotification subscribes to account notifications
func (c *WebsocketClient) SubscribeNotification(accountId int64, authToken string, cb func([]byte)) error {
	channel := fmt.Sprintf("notification/%d", accountId)
	return c.Subscribe(channel, &authToken, cb)
}

// SubscribeAccountAllTrades subscribes to all account trades
func (c *WebsocketClient) SubscribeAccountAllTrades(accountId int64, authToken string, cb func([]byte)) error {
	channel := fmt.Sprintf("account_all_trades/%d", accountId)
	return c.Subscribe(channel, &authToken, cb)
}

// SubscribeAccountAllPositions subscribes to all account positions
func (c *WebsocketClient) SubscribeAccountAllPositions(accountId int64, authToken string, cb func([]byte)) error {
	channel := fmt.Sprintf("account_all_positions/%d", accountId)
	return c.Subscribe(channel, &authToken, cb)
}
