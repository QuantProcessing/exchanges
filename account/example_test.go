package account_test

import (
	"context"
	"fmt"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/account"
	"github.com/shopspring/decimal"
)

func ExampleOrderFlow_Fills() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	adp := &accountRuntimeStubExchange{
		placeResp: &exchanges.Order{
			ClientOrderID: "cli-1",
			Symbol:        "ETH",
			Side:          exchanges.OrderSideBuy,
			Type:          exchanges.OrderTypeLimit,
			Quantity:      decimal.RequireFromString("0.1"),
			Price:         decimal.RequireFromString("100"),
			Status:        exchanges.OrderStatusPending,
		},
	}

	acct := account.NewTradingAccount(adp, nil)
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

	adp.EmitOrder(&exchanges.Order{
		OrderID:       "exch-1",
		ClientOrderID: "cli-1",
		Symbol:        "ETH",
		Side:          exchanges.OrderSideBuy,
		Type:          exchanges.OrderTypeLimit,
		Quantity:      decimal.RequireFromString("0.1"),
		Price:         decimal.RequireFromString("100"),
		Status:        exchanges.OrderStatusNew,
	})
	adp.EmitFill(&exchanges.Fill{
		TradeID:       "trade-1",
		OrderID:       "exch-1",
		ClientOrderID: "cli-1",
		Symbol:        "ETH",
		Side:          exchanges.OrderSideBuy,
		Price:         decimal.RequireFromString("101"),
		Quantity:      decimal.RequireFromString("0.04"),
		Timestamp:     1,
	})

	fill := <-flow.Fills()

	order, err := flow.Wait(ctx, func(o *exchanges.Order) bool {
		return o.LastFillQuantity.Equal(decimal.RequireFromString("0.04"))
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(fill.TradeID, order.Status, order.LastFillPrice)
	// Output: trade-1 PARTIALLY_FILLED 101
}
