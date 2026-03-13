package lighter

import (
	"context"
	"testing"
)

func TestOrder_PlaceMarketOrder(t *testing.T) {
	privateKey, accountIndex, keyIndex := GetEnv()
	client := NewClient().WithCredentials(privateKey, accountIndex, uint8(keyIndex))
	orderID, err := client.PlaceOrder(context.Background(), CreateOrderRequest{
		MarketId:    1,
		BaseAmount:  10000,
		Price:       94000,
		IsAsk:       0,
		OrderType:   OrderTypeMarket,
		TimeInForce: OrderTimeInForceImmediateOrCancel,
		OrderExpiry: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("orderID", orderID)
}
