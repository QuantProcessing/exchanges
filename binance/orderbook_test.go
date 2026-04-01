package binance

import (
	"testing"

	"github.com/QuantProcessing/exchanges/binance/sdk/spot"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestSpotOrderBookProcessUpdateAcceptsEventsWithoutPreviousUpdateID(t *testing.T) {
	ob := NewSpotOrderBook("BTC")

	require.NoError(t, ob.ProcessUpdate(&spot.WsDepthEvent{
		FirstUpdateID: 100,
		FinalUpdateID: 100,
		Bids:          [][]string{{"100", "1"}},
		Asks:          [][]string{{"101", "2"}},
	}))

	require.NoError(t, ob.ApplySnapshot(&spot.DepthResponse{
		LastUpdateID: 100,
		Bids:         [][]string{{"100", "1"}},
		Asks:         [][]string{{"101", "2"}},
	}))
	require.True(t, ob.IsInitialized())

	err := ob.ProcessUpdate(&spot.WsDepthEvent{
		FirstUpdateID: 101,
		FinalUpdateID: 102,
		Bids:          [][]string{{"100", "3"}},
	})
	require.NoError(t, err)
	require.True(t, ob.IsInitialized())

	bidPrice, bidQty := ob.GetBestBid()
	require.True(t, bidPrice.Equal(decimal.RequireFromString("100")))
	require.True(t, bidQty.Equal(decimal.RequireFromString("3")))
}
