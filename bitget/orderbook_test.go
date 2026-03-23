package bitget

import (
	"testing"

	"github.com/QuantProcessing/exchanges/bitget/sdk"
	"github.com/stretchr/testify/require"
)

func TestOrderBookSnapshotAndSequentialUpdate(t *testing.T) {
	ob := NewOrderBook("BTCUSDT")

	err := ob.ProcessUpdate("snapshot", &sdk.WSOrderBookData{
		Seq:  1,
		TS:   "1710000000000",
		Bids: [][]sdk.NumberString{{"49999", "1.2"}},
		Asks: [][]sdk.NumberString{{"50001", "0.8"}},
	})
	require.NoError(t, err)
	require.True(t, ob.IsInitialized())

	err = ob.ProcessUpdate("update", &sdk.WSOrderBookData{
		PSeq: 1,
		Seq:  2,
		TS:   "1710000000100",
		Bids: [][]sdk.NumberString{{"49999", "1.5"}},
		Asks: [][]sdk.NumberString{{"50001", "0"}},
	})
	require.NoError(t, err)

	bids, asks := ob.GetDepth(5)
	require.Len(t, bids, 1)
	require.Equal(t, "1.5", bids[0].Quantity.String())
	require.Len(t, asks, 0)
}

func TestOrderBookGapResetsInitialization(t *testing.T) {
	ob := NewOrderBook("BTCUSDT")

	require.NoError(t, ob.ProcessUpdate("snapshot", &sdk.WSOrderBookData{
		Seq:  10,
		Bids: [][]sdk.NumberString{{"49999", "1"}},
		Asks: [][]sdk.NumberString{{"50001", "1"}},
	}))

	err := ob.ProcessUpdate("update", &sdk.WSOrderBookData{
		PSeq: 8,
		Seq:  11,
		Bids: [][]sdk.NumberString{{"49998", "1"}},
	})
	require.Error(t, err)
	require.False(t, ob.IsInitialized())
}
