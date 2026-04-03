package exchanges_test

import (
	"context"
	"fmt"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/shopspring/decimal"
)

func ExampleNewTradingAccount() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	adp := &localStateStubExchange{
		placeResp: &exchanges.Order{
			ClientOrderID: "cli-1",
			Symbol:        "ETH",
			Side:          exchanges.OrderSideBuy,
			Type:          exchanges.OrderTypeLimit,
			Quantity:      decimal.RequireFromString("0.1"),
			Price:         decimal.RequireFromString("100"),
			Status:        exchanges.OrderStatusPending,
		},
		updates: []*exchanges.Order{{
			OrderID:       "exch-1",
			ClientOrderID: "cli-1",
			Symbol:        "ETH",
			Side:          exchanges.OrderSideBuy,
			Type:          exchanges.OrderTypeLimit,
			Quantity:      decimal.RequireFromString("0.1"),
			Price:         decimal.RequireFromString("100"),
			Status:        exchanges.OrderStatusNew,
		}},
	}

	acct := exchanges.NewTradingAccount(adp, nil)
	if err := acct.Start(ctx); err != nil {
		panic(err)
	}
	defer acct.Close()

	flow, err := acct.Place(ctx, &exchanges.OrderParams{
		Symbol:   "ETH",
		Side:     exchanges.OrderSideBuy,
		Type:     exchanges.OrderTypeLimit,
		Quantity: decimal.RequireFromString("0.1"),
		Price:    decimal.RequireFromString("100"),
	})
	if err != nil {
		panic(err)
	}
	defer flow.Close()

	got, err := flow.Wait(ctx, func(o *exchanges.Order) bool {
		return o.Status == exchanges.OrderStatusNew
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(got.OrderID, got.Status)
	// Output: exch-1 NEW
}
