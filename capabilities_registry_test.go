package exchanges_test

import (
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	_ "github.com/QuantProcessing/exchanges/config/all"
	"github.com/stretchr/testify/require"
)

func TestAllRegisteredAdaptersPublishCapabilities(t *testing.T) {
	t.Parallel()

	cases := []struct {
		exchange   string
		marketType exchanges.MarketType
	}{
		{"ASTER", exchanges.MarketTypePerp},
		{"ASTER", exchanges.MarketTypeSpot},
		{"BACKPACK", exchanges.MarketTypePerp},
		{"BACKPACK", exchanges.MarketTypeSpot},
		{"BINANCE", exchanges.MarketTypePerp},
		{"BINANCE", exchanges.MarketTypeSpot},
		{"BITGET", exchanges.MarketTypePerp},
		{"BITGET", exchanges.MarketTypeSpot},
		{"BYBIT", exchanges.MarketTypePerp},
		{"BYBIT", exchanges.MarketTypeSpot},
		{"EDGEX", exchanges.MarketTypePerp},
		{"GRVT", exchanges.MarketTypePerp},
		{"HYPERLIQUID", exchanges.MarketTypePerp},
		{"HYPERLIQUID", exchanges.MarketTypeSpot},
		{"LIGHTER", exchanges.MarketTypePerp},
		{"LIGHTER", exchanges.MarketTypeSpot},
		{"NADO", exchanges.MarketTypePerp},
		{"NADO", exchanges.MarketTypeSpot},
		{"OKX", exchanges.MarketTypePerp},
		{"OKX", exchanges.MarketTypeSpot},
		{"STANDX", exchanges.MarketTypePerp},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.exchange+"/"+string(tc.marketType), func(t *testing.T) {
			t.Parallel()

			caps, ok := exchanges.LookupCapabilities(tc.exchange, tc.marketType)
			require.True(t, ok)
			require.True(t, caps.PlaceOrder)
			require.True(t, caps.WatchOrderBook)
		})
	}
}

func TestRegisteredCapabilitiesCaptureAdapterDifferences(t *testing.T) {
	t.Parallel()

	binancePerp, ok := exchanges.LookupCapabilities("BINANCE", exchanges.MarketTypePerp)
	require.True(t, ok)
	require.True(t, binancePerp.PlaceOrderWS)
	require.True(t, binancePerp.WatchFills)
	require.True(t, binancePerp.WatchPositions)
	require.False(t, binancePerp.FetchOrderHistory)

	binanceSpot, ok := exchanges.LookupCapabilities("BINANCE", exchanges.MarketTypeSpot)
	require.True(t, ok)
	require.True(t, binanceSpot.PlaceOrderWS)
	require.True(t, binanceSpot.WatchFills)
	require.False(t, binanceSpot.WatchPositions)

	backpackPerp, ok := exchanges.LookupCapabilities("BACKPACK", exchanges.MarketTypePerp)
	require.True(t, ok)
	require.False(t, backpackPerp.PlaceOrderWS)
	require.True(t, backpackPerp.WatchOrders)
	require.False(t, backpackPerp.WatchTicker)

	lighterPerp, ok := exchanges.LookupCapabilities("LIGHTER", exchanges.MarketTypePerp)
	require.True(t, ok)
	require.True(t, lighterPerp.WatchPositions)
	require.False(t, lighterPerp.WatchKlines)

	okxPerp, ok := exchanges.LookupCapabilities("OKX", exchanges.MarketTypePerp)
	require.True(t, ok)
	require.True(t, okxPerp.PlaceOrderWS)
	require.False(t, okxPerp.WatchTrades)
	require.False(t, okxPerp.WatchKlines)

}

func TestRegisteredOptionCapabilitiesExposeRESTTradingOnly(t *testing.T) {
	t.Parallel()

	caps, ok := exchanges.LookupCapabilities("BINANCE", exchanges.MarketTypeOption)
	require.True(t, ok)
	require.True(t, caps.FetchOptionContracts)
	require.True(t, caps.PlaceOrder)
	require.True(t, caps.FetchOpenOrders)
	require.True(t, caps.FetchOrderHistory)
	require.False(t, caps.PlaceOrderWS)
	require.False(t, caps.WatchOrderBook)
	require.False(t, caps.WatchOrders)
	require.False(t, caps.WatchFills)
	require.False(t, caps.WatchPositions)
	require.False(t, caps.TradingAccountReady)

	_, ok = exchanges.LookupCapabilities("BITGET", exchanges.MarketTypeOption)
	require.False(t, ok)
	_, ok = exchanges.LookupCapabilities("OKX", exchanges.MarketTypeOption)
	require.False(t, ok)
	_, ok = exchanges.LookupCapabilities("BYBIT", exchanges.MarketTypeOption)
	require.False(t, ok)
}
