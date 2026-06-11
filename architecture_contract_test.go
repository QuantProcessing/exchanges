package exchanges_test

import (
	"context"
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/bybit"
	_ "github.com/QuantProcessing/exchanges/config/all"
	"github.com/QuantProcessing/exchanges/okx"
	"github.com/stretchr/testify/require"
)

func TestTradingAccountReadyCapabilitiesRequireOrderStream(t *testing.T) {
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
			if caps.TradingAccountReady {
				require.True(t, caps.WatchOrders, "TradingAccountReady requires a real order stream")
			}
		})
	}
}

func TestUnsupportedOptionAdapterSurfacesReturnErrNotSupported(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	okxOptions, err := okx.NewOptionAdapter(ctx, okx.Options{})
	require.NoError(t, err)
	_, err = okxOptions.FetchTicker(ctx, "BTC-USD-251226-100000-C")
	require.ErrorIs(t, err, exchanges.ErrNotSupported)
	err = okxOptions.PlaceOrderWS(ctx, &exchanges.OrderParams{})
	require.ErrorIs(t, err, exchanges.ErrNotSupported)
	err = okxOptions.WatchTicker(ctx, "BTC-USD-251226-100000-C", nil)
	require.ErrorIs(t, err, exchanges.ErrNotSupported)

	bybitOptions, err := bybit.NewOptionAdapter(ctx, bybit.Options{})
	require.NoError(t, err)
	_, err = bybitOptions.FetchOrderBook(ctx, "BTC-26DEC25-100000-C", 1)
	require.ErrorIs(t, err, exchanges.ErrNotSupported)
	err = bybitOptions.CancelOrderWS(ctx, "order-id", "BTC-26DEC25-100000-C")
	require.ErrorIs(t, err, exchanges.ErrNotSupported)
	err = bybitOptions.WatchTrades(ctx, "BTC-26DEC25-100000-C", nil)
	require.ErrorIs(t, err, exchanges.ErrNotSupported)
}
