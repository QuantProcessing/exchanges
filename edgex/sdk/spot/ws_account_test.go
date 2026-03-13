
package spot

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestOrderUpdate(t *testing.T) {
	starkPrivateKey, accountID := GetEnv()
	client := NewWsAccountClient(context.Background(), starkPrivateKey, accountID)
	client.Connect()

	client.SubscribeOrderUpdate(func(orders []Order) {
		fmt.Println(orders)
	})

	timeout := time.NewTimer(300 * time.Second)

	<-timeout.C
	client.Close()
}

func TestPositionUpdate(t *testing.T) {
	starkPrivateKey, accountID := GetEnv()
	client := NewWsAccountClient(context.Background(), starkPrivateKey, accountID)
	client.Connect()

	client.SubscribePositionUpdate(func(positions []PositionInfo) {
		fmt.Println(positions)
	})

	timeout := time.NewTimer(30 * time.Second)
	<-timeout.C
	client.Close()

}
