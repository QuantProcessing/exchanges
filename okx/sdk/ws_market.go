package okx

import (
	"encoding/json"
	"fmt"
)

// SubscribeTicker subscribes to ticker channel.
func (c *WsClient) SubscribeTicker(instId string, handler func(*Ticker)) error {
	args := WsSubscribeArgs{
		Channel: "tickers",
		InstId:  instId,
	}

	return c.Subscribe(args, func(msg []byte) {
		var push WsPushData[Ticker]
		if err := json.Unmarshal(msg, &push); err != nil {
			fmt.Println("Error unmarshal ticker:", err)
			return
		}
		for _, d := range push.Data {
			val := d
			handler(&val)
		}
	})
}

// SubscribeOrderBook subscribes to books channel.
// Default depth is 400
func (c *WsClient) SubscribeOrderBook(instId string, handler func(*OrderBook, string)) error {
	args := WsSubscribeArgs{
		Channel: "books",
		InstId:  instId,
	}

	return c.Subscribe(args, func(msg []byte) {
		var push WsPushData[OrderBook]
		if err := json.Unmarshal(msg, &push); err != nil {
			fmt.Println("Error unmarshal book:", err)
			return
		}
		for _, d := range push.Data {
			val := d
			handler(&val, push.Action)
		}
	})
}
