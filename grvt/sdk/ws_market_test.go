//go:build grvt

package grvt

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestSubscribeOrderbookDelta(t *testing.T) {
	apiKey, subaccount, privateKey := GetEnv()
	client := NewClient().WithCredentials(apiKey, subaccount, privateKey)
	wsClient := NewMarketWebsocketClient(context.Background(), client)
	err := wsClient.Connect()
	if err != nil {
		fmt.Println(err)
		return
	}
	wsClient.SubscribeOrderbookDelta("BTC_USDT_Perp", OrderBookDeltaRate100, func(data WsFeeData[OrderBook]) error {
		fmt.Println(data)
		return nil
	})

	timeout := time.NewTimer(5 * time.Second)
	<-timeout.C
}
