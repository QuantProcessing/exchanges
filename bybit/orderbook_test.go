package bybit

import (
	"testing"

	"github.com/QuantProcessing/exchanges/bybit/sdk"
	"github.com/stretchr/testify/require"
)

func TestOrderBookSnapshotAndDeltaUpdate(t *testing.T) {
	ob := NewOrderBook("BTCUSDT")

	ob.ProcessSnapshot(&sdk.WSOrderBookData{
		Symbol:   "BTCUSDT",
		UpdateID: 10,
		CTS:      1710000000000,
		Bids:     [][]sdk.NumberString{{"49999", "1.2"}},
		Asks:     [][]sdk.NumberString{{"50001", "0.8"}},
	})
	require.True(t, ob.IsInitialized())

	ob.ProcessDelta(&sdk.WSOrderBookData{
		Symbol:   "BTCUSDT",
		UpdateID: 11,
		CTS:      1710000000100,
		Bids:     [][]sdk.NumberString{{"49999", "1.5"}},
		Asks:     [][]sdk.NumberString{{"50001", "0"}},
	})

	bids, asks := ob.GetDepth(5)
	require.Len(t, bids, 1)
	require.Equal(t, "1.5", bids[0].Quantity.String())
	require.Len(t, asks, 0)
}
