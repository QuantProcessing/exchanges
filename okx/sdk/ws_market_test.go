package okx

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/joho/godotenv"
)

func GetEnv() (string, string, string) {
	godotenv.Load("../../.env")
	apiKey := os.Getenv("OKX_API_KEY")
	secretKey := os.Getenv("OKX_API_SECRET")
	passphrase := os.Getenv("OKX_API_PASSPHRASE")
	return apiKey, secretKey, passphrase
}

func TestSubscribeTicker(t *testing.T) {
	wsClient := NewWsClient(context.Background())
	err := wsClient.Connect()
	if err != nil {
		t.Fatal(err)
	}
	wsClient.SubscribeTicker("BTC-USDT-SWAP", func(ticker *Ticker) {
		fmt.Println(ticker)
	})
	timeout := time.NewTimer(1 * time.Minute)
	<-timeout.C
}

func TestSubscribeOrderBook(t *testing.T) {
	wsClient := NewWsClient(context.Background())
	err := wsClient.Connect()
	if err != nil {
		t.Fatal(err)
	}
	err = wsClient.SubscribeOrderBook("BTC-USDT-SWAP", func(ob *OrderBook, action string) {
		fmt.Printf("Received OrderBook: %+v, Action: %s\n", ob, action)
	})
	if err != nil {
		t.Fatalf("SubscribeOrderBook failed: %v", err)
	}
	timeout := time.NewTimer(1 * time.Minute)
	<-timeout.C
}
