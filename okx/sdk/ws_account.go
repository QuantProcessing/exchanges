package okx

import (
	"encoding/json"
	"fmt"
)

// SubscribeOrders subscribes to orders channel.
func (c *WsClient) SubscribeOrders(instType string, instId *string, handler func(*Order)) error {
	args := WsSubscribeArgs{
		Channel:  "orders",
		InstType: instType, // SPOT, SWAP, FUTURES, OPTION, ANY
	}
	if instId != nil {
		args.InstId = *instId
	}

	return c.Subscribe(args, func(msg []byte) {
		var push WsPushData[Order]
		if err := json.Unmarshal(msg, &push); err != nil {
			fmt.Println("Error unmarshal orders:", err)
			return
		}
		for _, d := range push.Data {
			val := d
			handler(&val)
		}
	})
}

// SubscribePositions subscribes to positions channel.
func (c *WsClient) SubscribePositions(instType string, handler func(*Position)) error {
	args := WsSubscribeArgs{
		Channel:  "positions",
		InstType: instType,
	}

	return c.Subscribe(args, func(msg []byte) {
		var push WsPushData[Position]
		if err := json.Unmarshal(msg, &push); err != nil {
			fmt.Println("Error unmarshal positions:", err)
			return
		}
		for _, d := range push.Data {
			val := d
			handler(&val)
		}
	})
}
