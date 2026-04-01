package aster

import (
	"testing"

	"github.com/QuantProcessing/exchanges/aster/sdk/perp"
	spot "github.com/QuantProcessing/exchanges/aster/sdk/spot"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestPerpOrderBookApplySnapshotAcceptsNextUpdateID(t *testing.T) {
	t.Parallel()

	ob := NewOrderBook("BTCUSDC")

	require.NoError(t, ob.ProcessUpdate(&perp.WsDepthEvent{
		FirstUpdateID: 101,
		FinalUpdateID: 102,
		Bids:          [][]interface{}{{"50000", "2"}},
		Asks:          [][]interface{}{{"50010", "3"}},
	}))

	err := ob.ApplySnapshot(&perp.DepthResponse{
		LastUpdateID: 100,
		Bids:         [][]string{{"49990", "1"}},
		Asks:         [][]string{{"50020", "4"}},
	})
	require.NoError(t, err)
	require.True(t, ob.IsInitialized())

	bidPrice, bidQty := ob.GetBestBid()
	require.True(t, bidPrice.Equal(decimal.RequireFromString("50000")))
	require.True(t, bidQty.Equal(decimal.RequireFromString("2")))
}

func TestSpotOrderBookApplySnapshotAcceptsNextUpdateID(t *testing.T) {
	t.Parallel()

	ob := NewSpotOrderBook("BTCUSDC")

	require.NoError(t, ob.ProcessUpdate(&spot.WsDepthEvent{
		FirstUpdateID: 101,
		FinalUpdateID: 102,
		Bids:          [][]interface{}{{"50000", "2"}},
		Asks:          [][]interface{}{{"50010", "3"}},
	}))

	err := ob.ApplySnapshot(&spot.DepthResponse{
		LastUpdateID: 100,
		Bids:         [][]string{{"49990", "1"}},
		Asks:         [][]string{{"50020", "4"}},
	})
	require.NoError(t, err)
	require.True(t, ob.IsInitialized())

	bidPrice, bidQty := ob.GetBestBid()
	require.True(t, bidPrice.Equal(decimal.RequireFromString("50000")))
	require.True(t, bidQty.Equal(decimal.RequireFromString("2")))
}

func TestSpotOrderBookProcessUpdateAcceptsMissingPreviousUpdateID(t *testing.T) {
	t.Parallel()

	ob := NewSpotOrderBook("BTCUSDT")

	require.NoError(t, ob.ProcessUpdate(&spot.WsDepthEvent{
		FirstUpdateID: 101,
		FinalUpdateID: 102,
		Bids:          [][]interface{}{{"50000", "1"}},
		Asks:          [][]interface{}{{"50010", "1"}},
	}))
	require.NoError(t, ob.ApplySnapshot(&spot.DepthResponse{
		LastUpdateID: 100,
		Bids:         [][]string{{"50000", "1"}},
		Asks:         [][]string{{"50010", "1"}},
	}))
	require.True(t, ob.IsInitialized())

	err := ob.ProcessUpdate(&spot.WsDepthEvent{
		FirstUpdateID: 103,
		FinalUpdateID: 104,
		Bids:          [][]interface{}{{"50000", "2"}},
	})
	require.NoError(t, err)
	require.True(t, ob.IsInitialized())
}
