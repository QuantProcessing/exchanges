package spot

import (
	"encoding/json"

	"github.com/QuantProcessing/exchanges/hyperliquid/sdk"
)

// Helper to subscribe to L2Book
func (c *WebsocketClient) SubscribeL2Book(coin string, handler func(hyperliquid.WsL2Book)) error {
	sub := map[string]string{
		"type": "l2Book",
		"coin": coin,
	}

	return c.WebsocketClient.Subscribe("l2Book", sub, func(msg hyperliquid.WsMessage) {
		var data hyperliquid.WsL2Book
		if err := json.Unmarshal(msg.Data, &data); err != nil {
			return
		}
		if data.Coin == coin {
			handler(data)
		}
	})
}

// SubscribeTrades
func (c *WebsocketClient) SubscribeTrades(coin string, handler func([]hyperliquid.WsTrade)) error {
	sub := map[string]string{
		"type": "trades",
		"coin": coin,
	}

	return c.WebsocketClient.Subscribe("trades", sub, func(msg hyperliquid.WsMessage) {
		var data []hyperliquid.WsTrade
		if err := json.Unmarshal(msg.Data, &data); err != nil {
			return
		}
		if len(data) > 0 && data[0].Coin == coin {
			handler(data)
		}
	})
}

func (c *WebsocketClient) SubscribeBbo(coin string, handler func(hyperliquid.WsBbo)) error {
	sub := map[string]string{
		"type": "bbo",
		"coin": coin,
	}

	return c.WebsocketClient.Subscribe("bbo", sub, func(msg hyperliquid.WsMessage) {
		var data hyperliquid.WsBbo
		if err := json.Unmarshal(msg.Data, &data); err != nil {
			return
		}
		if data.Coin == coin {
			handler(data)
		}
	})
}

// Unsubscribe methods

func (c *WebsocketClient) UnsubscribeL2Book(coin string) error {
	sub := map[string]string{
		"type": "l2Book",
		"coin": coin,
	}
	return c.WebsocketClient.Unsubscribe("l2Book", sub)
}

func (c *WebsocketClient) UnsubscribeTrades(coin string) error {
	sub := map[string]string{
		"type": "trades",
		"coin": coin,
	}
	return c.WebsocketClient.Unsubscribe("trades", sub)
}

func (c *WebsocketClient) UnsubscribeBbo(coin string) error {
	sub := map[string]string{
		"type": "bbo",
		"coin": coin,
	}
	return c.WebsocketClient.Unsubscribe("bbo", sub)
}
