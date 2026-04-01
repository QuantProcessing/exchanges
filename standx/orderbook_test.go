package standx

import (
	"testing"

	standxsdk "github.com/QuantProcessing/exchanges/standx/sdk"
	"github.com/stretchr/testify/require"
)

func TestOrderBookToAdapterOrderBookAppliesDepth(t *testing.T) {
	t.Parallel()

	ob := NewOrderBook("BTC")
	ob.UpdateSnapshot(standxsdk.WSDepthData{
		Bids: [][]string{
			{"50000", "1"},
			{"49999", "2"},
		},
		Asks: [][]string{
			{"50001", "3"},
			{"50002", "4"},
		},
	})

	snapshot := ob.ToAdapterOrderBook(1)
	require.NotNil(t, snapshot)
	require.Equal(t, "BTC", snapshot.Symbol)
	require.Len(t, snapshot.Bids, 1)
	require.Len(t, snapshot.Asks, 1)
	require.Equal(t, "50000", snapshot.Bids[0].Price.String())
	require.Equal(t, "50001", snapshot.Asks[0].Price.String())
}
