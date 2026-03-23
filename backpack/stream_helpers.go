package backpack

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/backpack/sdk"
)

type depthSnapshotClient interface {
	GetOrderBook(ctx context.Context, symbol string, limit int) (*sdk.Depth, error)
}

func refreshOrderBookSnapshot(client depthSnapshotClient, symbol string, ob *OrderBook) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	snapshot, err := client.GetOrderBook(ctx, symbol, 1000)
	if err != nil {
		return err
	}
	return ob.ApplySnapshot(snapshot)
}

func emitOrderBookUpdate(cb exchanges.OrderBookCallback, ob *OrderBook, symbol string, eventTime int64) {
	if cb == nil || !ob.IsInitialized() {
		return
	}

	bids, asks := ob.GetDepth(20)
	cb(&exchanges.OrderBook{
		Symbol:    strings.ToUpper(symbol),
		Timestamp: microsToMillis(eventTime),
		Bids:      bids,
		Asks:      asks,
	})
}

func decodeDepthEvent(payload json.RawMessage) (*sdk.DepthEvent, error) {
	var event sdk.DepthEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, err
	}
	return &event, nil
}

func decodeOrderUpdate(payload json.RawMessage) (*sdk.OrderUpdateEvent, error) {
	var event sdk.OrderUpdateEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, err
	}
	return &event, nil
}

func decodePositionUpdate(payload json.RawMessage) (*sdk.PositionUpdateEvent, error) {
	var event sdk.PositionUpdateEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, err
	}
	return &event, nil
}
