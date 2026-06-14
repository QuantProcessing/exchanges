package okx

import (
	"encoding/json"
	"fmt"
)

// SubscribeTicker subscribes to ticker channel.
func (c *WSClient) SubscribeTicker(instId string, handler func(*Ticker)) error {
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
func (c *WSClient) SubscribeOrderBook(instId string, handler func(*OrderBook, string)) error {
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

// SubscribeTrades subscribes to public trades channel.
func (c *WSClient) SubscribeTrades(instId string, handler func(*PublicTrade)) error {
	args := WsSubscribeArgs{
		Channel: "trades",
		InstId:  instId,
	}

	return c.Subscribe(args, func(msg []byte) {
		var push WsPushData[PublicTrade]
		if err := json.Unmarshal(msg, &push); err != nil {
			fmt.Println("Error unmarshal trades:", err)
			return
		}
		for _, d := range push.Data {
			val := d
			handler(&val)
		}
	})
}

// SubscribeCandles subscribes to a public candle channel such as candle1m.
func (c *WSClient) SubscribeCandles(instId string, channel string, handler func(Candle)) error {
	args := WsSubscribeArgs{
		Channel: channel,
		InstId:  instId,
	}

	return c.Subscribe(args, func(msg []byte) {
		var push WsPushData[Candle]
		if err := json.Unmarshal(msg, &push); err != nil {
			fmt.Println("Error unmarshal candles:", err)
			return
		}
		for _, d := range push.Data {
			handler(d)
		}
	})
}
