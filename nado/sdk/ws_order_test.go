package nado

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestWsPlaceOrder(t *testing.T) {
	privateKey, subaccount := GetEnv()
	if subaccount == "" {
		subaccount = "default"
	}

	WsApiClient, err := NewWsApiClient(context.Background(), privateKey)
	if err != nil {
		t.Fatal(err)
	}
	err = WsApiClient.Connect()
	if err != nil {
		t.Fatal(err)
	}

	order, err := WsApiClient.PlaceOrder(context.Background(), ClientOrderInput{
		ProductId:  2,
		Price:      "80000",
		Amount:     "0.01",
		Side:       OrderSideBuy,
		OrderType:  OrderTypeLimit,
		ReduceOnly: false,
		PostOnly:   false,
	})
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(order)

	time.Sleep(10 * time.Second)

	cancelResp, err := WsApiClient.CancelOrders(context.Background(), CancelOrdersInput{
		ProductIds: []int64{2},
		Digests:    []string{order.Digest},
	})
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("order canceled", cancelResp)
}

func TestWsCancelAndPlace(t *testing.T) {
	privateKey, subaccount := GetEnv()
	if subaccount == "" {
		subaccount = "default"
	}

	WsApiClient, err := NewWsApiClient(context.Background(), privateKey)
	if err != nil {
		t.Fatal(err)
	}
	err = WsApiClient.Connect()
	if err != nil {
		t.Fatal(err)
	}

	order, err := WsApiClient.PlaceOrder(context.Background(), ClientOrderInput{
		ProductId:  2,
		Price:      "80000",
		Amount:     "0.01",
		Side:       OrderSideBuy,
		OrderType:  OrderTypeLimit,
		ReduceOnly: false,
		PostOnly:   false,
	})
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(order)

	time.Sleep(10 * time.Second)

	cancelResp, err := WsApiClient.CancelAndPlace(context.Background(), CancelOrdersInput{
		ProductIds: []int64{2},
		Digests:    []string{order.Digest},
	}, ClientOrderInput{
		ProductId:  2,
		Price:      "80010",
		Amount:     "0.01",
		Side:       OrderSideBuy,
		OrderType:  OrderTypeLimit,
		ReduceOnly: false,
		PostOnly:   false,
	})
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("order canceled", cancelResp)
}
