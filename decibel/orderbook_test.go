package decibel

import (
	"context"
	"testing"
	"time"

	decibelws "github.com/QuantProcessing/exchanges/decibel/sdk/ws"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestDecibelOrderBookSeedsAndUpdatesFromDepthMessages(t *testing.T) {
	ob := NewOrderBook("BTC")

	ob.ProcessDepth(decibelws.MarketDepthMessage{
		Topic:      "depth:0xbtc:1",
		Market:     "0xbtc",
		UpdateType: decibelws.DepthUpdateSnapshot,
		Bids: []decibelws.DepthLevel{
			{Price: decimal.RequireFromString("50000"), Size: decimal.RequireFromString("2.5")},
			{Price: decimal.RequireFromString("49950"), Size: decimal.RequireFromString("1.25")},
		},
		Asks: []decibelws.DepthLevel{
			{Price: decimal.RequireFromString("50050"), Size: decimal.RequireFromString("3.5")},
			{Price: decimal.RequireFromString("50100"), Size: decimal.RequireFromString("4.25")},
		},
	})

	require.True(t, ob.WaitReady(context.Background(), time.Second))

	snapshot := ob.ToAdapterOrderBook(10)
	require.Equal(t, "BTC", snapshot.Symbol)
	require.Len(t, snapshot.Bids, 2)
	require.Len(t, snapshot.Asks, 2)
	require.True(t, decimal.RequireFromString("50000").Equal(snapshot.Bids[0].Price))
	require.True(t, decimal.RequireFromString("50050").Equal(snapshot.Asks[0].Price))

	ob.ProcessDepth(decibelws.MarketDepthMessage{
		Topic:      "depth:0xbtc:1",
		Market:     "0xbtc",
		UpdateType: decibelws.DepthUpdateSnapshot,
		Bids: []decibelws.DepthLevel{
			{Price: decimal.RequireFromString("50010"), Size: decimal.RequireFromString("1.5")},
		},
		Asks: []decibelws.DepthLevel{
			{Price: decimal.RequireFromString("50060"), Size: decimal.RequireFromString("2.0")},
		},
	})

	snapshot = ob.ToAdapterOrderBook(10)
	require.Len(t, snapshot.Bids, 1)
	require.Len(t, snapshot.Asks, 1)
	require.True(t, decimal.RequireFromString("50010").Equal(snapshot.Bids[0].Price))
	require.True(t, decimal.RequireFromString("1.5").Equal(snapshot.Bids[0].Quantity))
	require.True(t, decimal.RequireFromString("50060").Equal(snapshot.Asks[0].Price))
	require.True(t, decimal.RequireFromString("2.0").Equal(snapshot.Asks[0].Quantity))
}

func TestDecibelOrderBookAppliesDepthDeltasWithoutReplacingUntouchedLevels(t *testing.T) {
	ob := NewOrderBook("BTC")

	ob.ProcessDepth(decibelws.MarketDepthMessage{
		Topic:      "depth:0xbtc:1",
		Market:     "0xbtc",
		UpdateType: decibelws.DepthUpdateSnapshot,
		Bids: []decibelws.DepthLevel{
			{Price: decimal.RequireFromString("50000"), Size: decimal.RequireFromString("2.5")},
			{Price: decimal.RequireFromString("49950"), Size: decimal.RequireFromString("1.25")},
		},
		Asks: []decibelws.DepthLevel{
			{Price: decimal.RequireFromString("50050"), Size: decimal.RequireFromString("3.5")},
			{Price: decimal.RequireFromString("50100"), Size: decimal.RequireFromString("4.25")},
		},
	})

	ob.ProcessDepth(decibelws.MarketDepthMessage{
		Topic:      "depth:0xbtc:1",
		Market:     "0xbtc",
		UpdateType: decibelws.DepthUpdateDelta,
		Bids: []decibelws.DepthLevel{
			{Price: decimal.RequireFromString("50010"), Size: decimal.RequireFromString("1.5")},
			{Price: decimal.RequireFromString("49950"), Size: decimal.Zero},
		},
		Asks: []decibelws.DepthLevel{
			{Price: decimal.RequireFromString("50060"), Size: decimal.RequireFromString("2.0")},
			{Price: decimal.RequireFromString("50100"), Size: decimal.Zero},
		},
	})

	snapshot := ob.ToAdapterOrderBook(10)
	require.Len(t, snapshot.Bids, 2)
	require.Len(t, snapshot.Asks, 2)
	require.True(t, decimal.RequireFromString("50010").Equal(snapshot.Bids[0].Price))
	require.True(t, decimal.RequireFromString("1.5").Equal(snapshot.Bids[0].Quantity))
	require.True(t, decimal.RequireFromString("50000").Equal(snapshot.Bids[1].Price))
	require.True(t, decimal.RequireFromString("2.5").Equal(snapshot.Bids[1].Quantity))
	require.True(t, decimal.RequireFromString("50050").Equal(snapshot.Asks[0].Price))
	require.True(t, decimal.RequireFromString("3.5").Equal(snapshot.Asks[0].Quantity))
	require.True(t, decimal.RequireFromString("50060").Equal(snapshot.Asks[1].Price))
	require.True(t, decimal.RequireFromString("2.0").Equal(snapshot.Asks[1].Quantity))
}

func TestDecibelOrderBookIgnoresInitialDeltaUntilSnapshot(t *testing.T) {
	ob := NewOrderBook("BTC")

	ob.ProcessDepth(decibelws.MarketDepthMessage{
		Topic:      "depth:0xbtc:1",
		Market:     "0xbtc",
		UpdateType: decibelws.DepthUpdateDelta,
		Bids: []decibelws.DepthLevel{
			{Price: decimal.RequireFromString("50010"), Size: decimal.RequireFromString("1.5")},
		},
		Asks: []decibelws.DepthLevel{
			{Price: decimal.RequireFromString("50060"), Size: decimal.RequireFromString("2.0")},
		},
	})

	require.False(t, ob.WaitReady(context.Background(), 50*time.Millisecond))
	snapshot := ob.ToAdapterOrderBook(10)
	require.Empty(t, snapshot.Bids)
	require.Empty(t, snapshot.Asks)

	ob.ProcessDepth(decibelws.MarketDepthMessage{
		Topic:      "depth:0xbtc:1",
		Market:     "0xbtc",
		UpdateType: decibelws.DepthUpdateSnapshot,
		Bids: []decibelws.DepthLevel{
			{Price: decimal.RequireFromString("50000"), Size: decimal.RequireFromString("2.5")},
		},
		Asks: []decibelws.DepthLevel{
			{Price: decimal.RequireFromString("50050"), Size: decimal.RequireFromString("3.5")},
		},
	})

	require.True(t, ob.WaitReady(context.Background(), time.Second))
	snapshot = ob.ToAdapterOrderBook(10)
	require.Len(t, snapshot.Bids, 1)
	require.Len(t, snapshot.Asks, 1)
}
