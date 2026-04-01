package hyperliquid

import (
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	hyperliquidsdk "github.com/QuantProcessing/exchanges/hyperliquid/sdk"
	"github.com/stretchr/testify/require"
)

func TestAdapterLocalOrderBookSnapshotUsesSharedBook(t *testing.T) {
	t.Parallel()

	a := &Adapter{
		BaseAdapter: exchanges.NewBaseAdapter("HYPERLIQUID", exchanges.MarketTypePerp, nil),
	}

	ob := NewOrderBook("BTC")
	ob.ProcessSnapshot(hyperliquidsdk.WsL2Book{
		Coin: "BTC",
		Time: 1710000000000,
		Levels: [][]hyperliquidsdk.WsLevel{
			{
				{Px: "50000", Sz: "1"},
				{Px: "49999", Sz: "2"},
			},
			{
				{Px: "50001", Sz: "3"},
				{Px: "50002", Sz: "4"},
			},
		},
	})
	a.SetLocalOrderBook("BTC", ob)

	snapshot := a.localOrderBookSnapshot("BTC", 1)
	require.NotNil(t, snapshot)
	require.Equal(t, "BTC", snapshot.Symbol)
	require.Len(t, snapshot.Bids, 1)
	require.Len(t, snapshot.Asks, 1)
	require.Equal(t, "50000", snapshot.Bids[0].Price.String())
	require.Equal(t, "50001", snapshot.Asks[0].Price.String())
}
