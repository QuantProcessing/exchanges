package main

import (
	"context"
	"fmt"

	"github.com/QuantProcessing/exchanges/adapter/binance"
	"github.com/QuantProcessing/exchanges/platform"
)

func main() {
	ctx := context.Background()

	spotData, err := binance.NewSpotDataClient(ctx, binance.Options{})
	if err != nil {
		panic(err)
	}
	perpExec, err := binance.NewPerpExecutionClient(ctx, binance.Options{})
	if err != nil {
		panic(err)
	}

	node := platform.NewNode(platform.Config{})
	if err := node.AddDataClient("binance-spot-data", spotData); err != nil {
		panic(err)
	}
	if err := node.AddExecutionClient("binance-perp-exec", perpExec); err != nil {
		panic(err)
	}

	sub, events := node.Bus().Subscribe(platform.TopicExecutionEvents, 64)
	defer sub.Close()

	_ = events
	fmt.Println("platform spread-arbitrage runtime assembled")
}
