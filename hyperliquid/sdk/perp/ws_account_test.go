package perp

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/QuantProcessing/exchanges/internal/testenv"
	"github.com/QuantProcessing/exchanges/hyperliquid/sdk"
)

func requireSoakEnv(t *testing.T) {
	t.Helper()
	testenv.RequireSoak(t, "HYPERLIQUID_PRIVATE_KEY", "HYPERLIQUID_ACCOUNT_ADDR")
}

func GetEnv() (string, string, string) {
	privateKey := os.Getenv("HYPERLIQUID_PRIVATE_KEY")
	vault := os.Getenv("HYPERLIQUID_VAULT")
	accountAddr := os.Getenv("HYPERLIQUID_ACCOUNT_ADDR")
	return privateKey, vault, accountAddr
}

func TestSubscribeOrderUpdates(t *testing.T) {
	requireSoakEnv(t)
	privateKey, _, accountAddr := GetEnv()
	baseClient := hyperliquid.NewWebsocketClient(context.Background())
	wsClient := NewWebsocketClient(baseClient).WithCredentials(privateKey, accountAddr)
	err := wsClient.Connect()
	if err != nil {
		fmt.Println(err)
		return
	}

	err = wsClient.SubscribeOrderUpdates(accountAddr, func(orderUpdates []hyperliquid.WsOrderUpdate) {
		fmt.Printf("%+v\n", orderUpdates)
	})
	if err != nil {
		fmt.Println(err)
		return
	}

	timeout := time.NewTimer(3 * time.Minute)
	<-timeout.C
}

func TestSubscribeWebData2(t *testing.T) {
	requireSoakEnv(t)
	privateKey, _, accountAddr := GetEnv()
	baseClient := hyperliquid.NewWebsocketClient(context.Background())
	wsClient := NewWebsocketClient(baseClient).WithCredentials(privateKey, accountAddr)
	err := wsClient.Connect()
	if err != nil {
		fmt.Println(err)
		return
	}

	err = wsClient.SubscribeWebData2(accountAddr, func(pos PerpPosition) {
		fmt.Printf("%+v\n", pos)
	})
	if err != nil {
		fmt.Println(err)
		return
	}

	timeout := time.NewTimer(3 * time.Minute)
	<-timeout.C
}
