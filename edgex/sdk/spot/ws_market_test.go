//go:build edgex

package spot

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestSubMarketData(t *testing.T) {
	wsClient := NewWsMarketClient(context.Background())
	err := wsClient.Connect()
	if err != nil {
		fmt.Println(err)
		return
	}
	err = wsClient.SubscribeKline("90000002", PriceTypeLastPrice, KlineInterval1m, func(event *WsKlineEvent) {
		fmt.Println(event)
	})

	if err != nil {
		fmt.Println(err)
		return
	}

	timeout := time.NewTimer(5 * time.Second)
	<-timeout.C
}
