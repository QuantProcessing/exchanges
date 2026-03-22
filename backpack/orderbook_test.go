package backpack

import (
	"context"
	"testing"
	"time"

	"github.com/QuantProcessing/exchanges/backpack/sdk"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestOrderBookAppliesSnapshotAndBufferedUpdates(t *testing.T) {
	t.Parallel()

	ob := NewOrderBook("BTC_USDC")

	err := ob.ProcessUpdate(&sdk.DepthEvent{
		EventTime:     1710000000000000,
		FirstUpdateID: 101,
		FinalUpdateID: 102,
		Bids:          [][]string{{"50000", "2"}},
		Asks:          [][]string{{"50010", "3"}},
	})
	require.NoError(t, err)

	err = ob.ApplySnapshot(&sdk.Depth{
		LastUpdateID: "101",
		Bids:         [][]string{{"49990", "1"}},
		Asks:         [][]string{{"50020", "4"}},
		Timestamp:    1710000000000,
	})
	require.NoError(t, err)
	require.True(t, ob.WaitReady(context.Background(), time.Second))

	bids, asks := ob.GetDepth(5)
	require.Equal(t, decimal.RequireFromString("50000"), bids[0].Price)
	require.Equal(t, decimal.RequireFromString("2"), bids[0].Quantity)
	require.Equal(t, decimal.RequireFromString("50010"), asks[0].Price)
	require.Equal(t, decimal.RequireFromString("3"), asks[0].Quantity)
}

func TestOrderBookDetectsSequenceGap(t *testing.T) {
	t.Parallel()

	ob := NewOrderBook("BTC_USDC")
	err := ob.ApplySnapshot(&sdk.Depth{
		LastUpdateID: "100",
		Bids:         [][]string{{"49990", "1"}},
		Asks:         [][]string{{"50020", "4"}},
		Timestamp:    1710000000000,
	})
	require.NoError(t, err)

	_ = ob.ProcessUpdate(&sdk.DepthEvent{
		FirstUpdateID: 101,
		FinalUpdateID: 101,
		Bids:          [][]string{{"50000", "2"}},
	})

	err = ob.ProcessUpdate(&sdk.DepthEvent{
		FirstUpdateID: 103,
		FinalUpdateID: 103,
		Bids:          [][]string{{"50001", "1"}},
	})
	require.Error(t, err)
}
