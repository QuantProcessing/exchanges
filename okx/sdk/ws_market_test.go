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

func TestWSClientConstructorCompatibility(t *testing.T) {
	ctx := context.Background()

	modern := NewWSClient(ctx)
	legacy := NewWsClient(ctx)

	var modernFromLegacy *WSClient = legacy
	var legacyFromModern *WsClient = modern

	if modernFromLegacy != legacy {
		t.Fatalf("legacy constructor returned incompatible type")
	}
	if legacyFromModern != modern {
		t.Fatalf("modern constructor returned incompatible alias type")
	}
	if modern.URL != legacy.URL {
		t.Fatalf("constructors should initialize the same URL: got %q and %q", modern.URL, legacy.URL)
	}
	if modern.Subs == nil || legacy.PendingReqs == nil {
		t.Fatalf("constructors should initialize websocket client maps")
	}
}

func TestSubscribeTicker(t *testing.T) {
	wsClient := NewWSClient(context.Background())
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
	wsClient := NewWSClient(context.Background())
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
