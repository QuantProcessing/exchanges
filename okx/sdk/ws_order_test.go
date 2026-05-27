package okx

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestPlaceMarketOrderWs(t *testing.T) {
	apiKey, secretKey, passphrase := GetEnv(t)
	wsClient := NewWSClient(context.Background()).WithCredentials(apiKey, secretKey, passphrase)
	err := wsClient.Connect()
	if err != nil {
		t.Fatal(err)
	}

	// monitor order
	instId := "BTC-USDT-SWAP"
	instIdCode := mustGetTestInstIdCode(t, instId)
	wsClient.SubscribeOrders("SWAP", &instId, func(orders *Order) {
		fmt.Printf("Subscribe Order update: %+v\n", orders)
	})

	// subscribe is async
	// time.Sleep(2 * time.Second)

	req := OrderRequest{
		InstId:     "BTC-USDT-SWAP",
		InstIdCode: &instIdCode,
		Side:       "buy",
		Sz:         "0.01",
		TdMode:     "isolated",
		OrdType:    "market",
	}
	order, err := wsClient.PlaceOrderWS(&req)
	if err != nil {
		fmt.Printf("Place Order error: %v\n", err)
		return
	}
	fmt.Printf("Place Order success: %+v\n", order)

	timeout := time.NewTicker(10 * time.Second)
	<-timeout.C
}

func TestPlaceLimitOrderWs(t *testing.T) {
	apiKey, secretKey, passphrase := GetEnv(t)
	wsClient := NewWSClient(context.Background()).WithCredentials(apiKey, secretKey, passphrase)
	err := wsClient.Connect()
	if err != nil {
		t.Fatal(err)
	}

	// monitor order
	instId := "BTC-USDT-SWAP"
	instIdCode := mustGetTestInstIdCode(t, instId)
	wsClient.SubscribeOrders("SWAP", &instId, func(orders *Order) {
		fmt.Printf("Subscribe Order update: %+v\n", orders)
	})

	// subscribe is async
	// time.Sleep(2 * time.Second)

	px := "80000"
	req := OrderRequest{
		InstId:     "BTC-USDT-SWAP",
		InstIdCode: &instIdCode,
		Side:       "buy",
		Sz:         "0.01",
		Px:         &px,
		TdMode:     "isolated",
		OrdType:    "limit",
	}
	order, err := wsClient.PlaceOrderWS(&req)
	if err != nil {
		fmt.Printf("Place Order error: %v\n", err)
		return
	}
	fmt.Printf("Place Order success: %+v\n", order)

	timeout := time.NewTicker(10 * time.Second)
	<-timeout.C
}

func TestCancelOrderWs(t *testing.T) {
	apiKey, secretKey, passphrase := GetEnv(t)
	wsClient := NewWSClient(context.Background()).WithCredentials(apiKey, secretKey, passphrase)
	err := wsClient.Connect()
	if err != nil {
		t.Fatal(err)
	}

	// monitor order
	instId := "BTC-USDT-SWAP"
	instIdCode := mustGetTestInstIdCode(t, instId)
	wsClient.SubscribeOrders("SWAP", &instId, func(orders *Order) {
		fmt.Printf("Subscribe Order update: %+v\n", orders)
	})

	ordId := "3144956797848346624"
	order, err := wsClient.CancelOrderWS(instIdCode, &ordId, nil)
	if err != nil {
		fmt.Printf("Cancel Order error: %v\n", err)
		return
	}
	fmt.Printf("Cancel Order success: %+v\n", order)

	timeout := time.NewTicker(5 * time.Second)
	<-timeout.C
}

func mustGetTestInstIdCode(t *testing.T, instID string) int64 {
	t.Helper()

	instType := "SPOT"
	if strings.Contains(instID, "-SWAP") {
		instType = "SWAP"
	}

	client := NewClient()
	insts, err := client.GetInstruments(context.Background(), instType)
	if err != nil {
		t.Fatalf("get instruments: %v", err)
	}

	for _, inst := range insts {
		if inst.InstId == instID && inst.InstIdCode != nil {
			return *inst.InstIdCode
		}
	}

	t.Fatalf("instIdCode not found for %s", instID)
	return 0
}
