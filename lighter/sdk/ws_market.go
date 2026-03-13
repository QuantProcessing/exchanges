package lighter

import "fmt"

// Public Channel Subscriptions

// SubscribeOrderBook subscribes to the order book channel for a specific market
func (c *WebsocketClient) SubscribeOrderBook(marketId int, cb func([]byte)) error {
	channel := fmt.Sprintf("order_book/%d", marketId)
	return c.Subscribe(channel, nil, cb)
}

// SubscribeMarketStats subscribes to market stats for a specific market
func (c *WebsocketClient) SubscribeMarketStats(marketId int, cb func([]byte)) error {
	channel := fmt.Sprintf("market_stats/%d", marketId)
	return c.Subscribe(channel, nil, cb)
}

// SubscribeAllMarketStats subscribes to market stats for all markets
func (c *WebsocketClient) SubscribeAllMarketStats(cb func([]byte)) error {
	channel := "market_stats/all"
	return c.Subscribe(channel, nil, cb)
}

// SubscribeTrades subscribes to the trades channel for a specific market
func (c *WebsocketClient) SubscribeTrades(marketId int, cb func([]byte)) error {
	channel := fmt.Sprintf("trade/%d", marketId)
	return c.Subscribe(channel, nil, cb)
}

// SubscribeHeight subscribes to blockchain height updates
func (c *WebsocketClient) SubscribeHeight(cb func([]byte)) error {
	channel := "height"
	return c.Subscribe(channel, nil, cb)

}

// Unsubscribe methods

func (c *WebsocketClient) UnsubscribeOrderBook(marketId int) error {
	channel := fmt.Sprintf("order_book/%d", marketId)
	return c.Unsubscribe(channel)
}

func (c *WebsocketClient) UnsubscribeTrades(marketId int) error {
	channel := fmt.Sprintf("trade/%d", marketId)
	return c.Unsubscribe(channel)
}
