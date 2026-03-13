
package grvt

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"
)

// TestLimitOrderLifecycle: 0.1 ETH Limit Buy @ 3000 -> Cancel
func TestLimitOrderLifecycle(t *testing.T) {
	apiKey, subaccount, privateKey := GetEnv()
	if apiKey == "" {
		t.Skip("Skipping test: credentials not set")
	}

	client := NewClient().WithCredentials(apiKey, subaccount, privateKey)
	wsClient := NewAccountRpcWebsocketClient(context.Background(), client)
	err := wsClient.Connect()
	if err != nil {
		t.Fatal(err)
	}
	defer wsClient.Close()
	time.Sleep(2 * time.Second) // Wait for connection

	// Parse subaccount
	var sa uint64
	fmt.Sscan(subaccount, &sa)

	// 1. Place Limit Buy Order
	limitReq := &OrderRequest{
		SubAccountID: sa,
		IsMarket:     false,
		TimeInForce:  GTT,
		PostOnly:     true, // Post only for limit
		ReduceOnly:   false,
		Legs: []OrderLeg{
			{
				Instrument:    "ETH_USDT_Perp",
				Size:          "0.1",
				LimitPrice:    "3000",
				IsBuyintAsset: true,
			},
		},
		Metadata: OrderMetadata{
			ClientOrderID: fmt.Sprintf("%d", time.Now().UnixNano()),
		},
		Signature: OrderSignature{
			Expiration: strconv.FormatInt(time.Now().Add(5*time.Minute).UnixNano(), 10),
			Nonce:      uint32(time.Now().Unix()),
		},
	}

	t.Log("Placing Limit Order...")
	resp, err := wsClient.PlaceOrder(context.Background(), limitReq)
	if err != nil {
		t.Fatalf("PlaceOrder failed: %v", err)
	}
	t.Logf("PlaceOrder success: OrderID=%s", resp.Result.OrderID)

	// 2. Cancel Order
	time.Sleep(1 * time.Second)
	t.Log("Cancelling Order...")
	cancelReq := &CancelOrderRequest{
		SubAccountID: subaccount, // CancelOrder takes string subaccount? Check types.go. Yes, CancelOrderRequest.SubAccountID is string.
		OrderID:      &resp.Result.OrderID,
	}
	cancelResp, err := wsClient.CancelOrder(context.Background(), cancelReq)
	if err != nil {
		t.Fatalf("CancelOrder failed: %v", err)
	}
	t.Logf("CancelOrder success: %+v", cancelResp)
}

// TestMarketOrderLifecycle: 0.1 ETH Market Buy -> Market Sell (Close)
func TestMarketOrderLifecycle(t *testing.T) {
	apiKey, subaccount, privateKey := GetEnv()
	if apiKey == "" {
		t.Skip("Skipping test: credentials not set")
	}

	client := NewClient().WithCredentials(apiKey, subaccount, privateKey)
	wsClient := NewAccountRpcWebsocketClient(context.Background(), client)
	err := wsClient.Connect()
	if err != nil {
		t.Fatal(err)
	}
	defer wsClient.Close()
	time.Sleep(2 * time.Second)

	var sa uint64
	fmt.Sscan(subaccount, &sa)

	// 1. Place Market Buy Order
	marketBuyReq := &OrderRequest{
		SubAccountID: sa,
		IsMarket:     true,
		TimeInForce:  IOC, // Market orders usually IOC or FOK
		PostOnly:     false,
		ReduceOnly:   false,
		Legs: []OrderLeg{
			{
				Instrument:    "ETH_USDT_Perp",
				Size:          "0.1",
				LimitPrice:    "0", // Market order
				IsBuyintAsset: true,
			},
		},
		Metadata: OrderMetadata{
			ClientOrderID: fmt.Sprintf("%d", time.Now().UnixNano()),
		},
		Signature: OrderSignature{
			Expiration: strconv.FormatInt(time.Now().Add(1*time.Minute).UnixNano(), 10),
			Nonce:      uint32(time.Now().Unix()),
		},
	}

	t.Log("Placing Market Buy Order...")
	resp, err := wsClient.PlaceOrder(context.Background(), marketBuyReq)
	if err != nil {
		t.Fatalf("Market Buy failed: %v", err)
	}
	t.Logf("Market Buy success: OrderID=%s", resp.Result.OrderID)

	// Wait for fill
	time.Sleep(2 * time.Second)

	// 2. Close Position (Market Sell)
	marketSellReq := &OrderRequest{
		SubAccountID: sa,
		IsMarket:     true,
		TimeInForce:  IOC,
		PostOnly:     false,
		ReduceOnly:   true, // Close position
		Legs: []OrderLeg{
			{
				Instrument:    "ETH_USDT_Perp",
				Size:          "0.1",
				LimitPrice:    "0",
				IsBuyintAsset: false, // Sell
			},
		},
		Metadata: OrderMetadata{
			ClientOrderID: fmt.Sprintf("%d", time.Now().UnixNano()),
		},
		Signature: OrderSignature{
			Expiration: strconv.FormatInt(time.Now().Add(1*time.Minute).UnixNano(), 10),
			Nonce:      uint32(time.Now().Unix()),
		},
	}

	t.Log("Placing Market Sell (Close) Order...")
	closeResp, err := wsClient.PlaceOrder(context.Background(), marketSellReq)
	if err != nil {
		t.Fatalf("Market Sell failed: %v", err)
	}
	t.Logf("Market Sell success: OrderID=%s", closeResp.Result.OrderID)
}
