package main

import (
	"context"
	"fmt"
	"log"
	"time"

	godemo "github.com/QuantProcessing/exchanges/examples/usage_comparison/go_demo"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := godemo.RunDemo(ctx)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("signal_triggered=%v\n", result.SignalTriggered)
	fmt.Printf("final_order=%s status=%s filled=%s leaves=%s\n",
		result.FinalOrder.OrderID,
		result.FinalOrder.Status,
		result.FinalOrder.FilledQuantity,
		result.FinalOrder.LeavesQuantity,
	)
	fmt.Printf("fills=%d first_trade=%s\n", len(result.Fills), result.Fills[0].TradeID)
	fmt.Printf("position=%s qty=%s entry=%s\n",
		result.Position.Side,
		result.Position.Quantity,
		result.Position.EntryPrice,
	)
	fmt.Printf("usdt_exposure=%s\n", result.Exposure)
	fmt.Printf("events=%v\n", result.EventLog)
}
