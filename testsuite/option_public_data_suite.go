package testsuite

import (
	"context"
	"testing"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"

	"github.com/stretchr/testify/require"
)

// OptionPublicDataConfig describes the unauthenticated option-market data
// surface expected from public-data-only option adapters.
type OptionPublicDataConfig struct {
	Underlying string
	Symbol     string
	Native     string
	Interval   exchanges.Interval
	KlineOpts  *exchanges.KlineOpts
	BookDepth  int
}

// RunOptionPublicDataSuite verifies public option data methods.
func RunOptionPublicDataSuite(t *testing.T, adp exchanges.OptionExchange, cfg OptionPublicDataConfig) {
	runOptionPublicDataSuite(t, adp, cfg, false)
}

// RunOptionPublicDataOnlySuite verifies public option data methods and that
// private/trading/streaming methods remain explicitly unsupported.
func RunOptionPublicDataOnlySuite(t *testing.T, adp exchanges.OptionExchange, cfg OptionPublicDataConfig) {
	runOptionPublicDataSuite(t, adp, cfg, true)
}

func runOptionPublicDataSuite(t *testing.T, adp exchanges.OptionExchange, cfg OptionPublicDataConfig, requireUnsupportedPrivate bool) {
	t.Helper()

	if cfg.Interval == "" {
		cfg.Interval = exchanges.Interval1h
	}
	if cfg.BookDepth <= 0 {
		cfg.BookDepth = 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	t.Run("ListOptionContracts", func(t *testing.T) {
		contracts, err := adp.ListOptionContracts(ctx, cfg.Underlying)
		require.NoError(t, err)
		require.NotEmpty(t, contracts)
		require.Contains(t, optionContractSymbols(contracts), cfg.Symbol)
	})

	t.Run("FetchOptionContract", func(t *testing.T) {
		contract, err := adp.FetchOptionContract(ctx, cfg.Symbol)
		require.NoError(t, err)
		require.Equal(t, cfg.Symbol, contract.Symbol)
		if cfg.Native != "" {
			contract, err = adp.FetchOptionContract(ctx, cfg.Native)
			require.NoError(t, err)
			require.Equal(t, cfg.Symbol, contract.Symbol)
		}
	})

	t.Run("FetchSymbolDetails", func(t *testing.T) {
		details, err := adp.FetchSymbolDetails(ctx, cfg.Symbol)
		require.NoError(t, err)
		require.Equal(t, cfg.Symbol, details.Symbol)
	})

	t.Run("FetchTicker", func(t *testing.T) {
		ticker, err := adp.FetchTicker(ctx, cfg.Symbol)
		require.NoError(t, err)
		require.Equal(t, cfg.Symbol, ticker.Symbol)
	})

	t.Run("FetchOrderBook", func(t *testing.T) {
		book, err := adp.FetchOrderBook(ctx, cfg.Symbol, cfg.BookDepth)
		require.NoError(t, err)
		require.Equal(t, cfg.Symbol, book.Symbol)
	})

	t.Run("FetchTrades", func(t *testing.T) {
		trades, err := adp.FetchTrades(ctx, cfg.Symbol, 1)
		require.NoError(t, err)
		for _, trade := range trades {
			require.Equal(t, cfg.Symbol, trade.Symbol)
		}
	})

	t.Run("FetchKlines", func(t *testing.T) {
		klines, err := adp.FetchKlines(ctx, cfg.Symbol, cfg.Interval, cfg.KlineOpts)
		require.NoError(t, err)
		for _, kline := range klines {
			require.Equal(t, cfg.Symbol, kline.Symbol)
			require.Equal(t, cfg.Interval, kline.Interval)
		}
	})

	t.Run("UnsupportedPrivateAndStreamingMethods", func(t *testing.T) {
		if !requireUnsupportedPrivate {
			requireUnsupportedOptionStreamingMethods(t, ctx, adp, cfg.Symbol)
			return
		}
		requireUnsupportedPublicDataOnlyMethods(t, ctx, adp, cfg.Symbol)
	})
}

func optionContractSymbols(contracts []exchanges.OptionContract) []string {
	out := make([]string, 0, len(contracts))
	for _, contract := range contracts {
		out = append(out, contract.Symbol)
	}
	return out
}

func requireUnsupportedOptionStreamingMethods(t *testing.T, ctx context.Context, adp exchanges.Exchange, symbol string) {
	t.Helper()

	require.ErrorIs(t, adp.PlaceOrderWS(ctx, &exchanges.OrderParams{Symbol: symbol}), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.CancelOrderWS(ctx, "order-id", symbol), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.WatchOrderBook(ctx, symbol, 0, nil), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.StopWatchOrderBook(ctx, symbol), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.WatchOrders(ctx, nil), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.WatchFills(ctx, nil), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.WatchPositions(ctx, nil), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.WatchTicker(ctx, symbol, nil), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.WatchTrades(ctx, symbol, nil), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.WatchKlines(ctx, symbol, exchanges.Interval1m, nil), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.StopWatchOrders(ctx), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.StopWatchFills(ctx), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.StopWatchPositions(ctx), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.StopWatchTicker(ctx, symbol), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.StopWatchTrades(ctx, symbol), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.StopWatchKlines(ctx, symbol, exchanges.Interval1m), exchanges.ErrNotSupported)
}

func requireUnsupportedPublicDataOnlyMethods(t *testing.T, ctx context.Context, adp exchanges.Exchange, symbol string) {
	t.Helper()

	_, err := adp.PlaceOrder(ctx, &exchanges.OrderParams{Symbol: symbol})
	require.ErrorIs(t, err, exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.PlaceOrderWS(ctx, &exchanges.OrderParams{Symbol: symbol}), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.CancelOrder(ctx, "order-id", symbol), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.CancelOrderWS(ctx, "order-id", symbol), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.CancelAllOrders(ctx, symbol), exchanges.ErrNotSupported)
	_, err = adp.FetchOrderByID(ctx, "order-id", symbol)
	require.ErrorIs(t, err, exchanges.ErrNotSupported)
	_, err = adp.FetchOrders(ctx, symbol)
	require.ErrorIs(t, err, exchanges.ErrNotSupported)
	_, err = adp.FetchOpenOrders(ctx, symbol)
	require.ErrorIs(t, err, exchanges.ErrNotSupported)
	_, err = adp.FetchAccount(ctx)
	require.ErrorIs(t, err, exchanges.ErrNotSupported)
	_, err = adp.FetchBalance(ctx)
	require.ErrorIs(t, err, exchanges.ErrNotSupported)
	_, err = adp.FetchFeeRate(ctx, symbol)
	require.ErrorIs(t, err, exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.WatchOrderBook(ctx, symbol, 0, nil), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.StopWatchOrderBook(ctx, symbol), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.WatchOrders(ctx, nil), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.WatchFills(ctx, nil), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.WatchPositions(ctx, nil), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.WatchTicker(ctx, symbol, nil), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.WatchTrades(ctx, symbol, nil), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.WatchKlines(ctx, symbol, exchanges.Interval1m, nil), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.StopWatchOrders(ctx), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.StopWatchFills(ctx), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.StopWatchPositions(ctx), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.StopWatchTicker(ctx, symbol), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.StopWatchTrades(ctx, symbol), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.StopWatchKlines(ctx, symbol, exchanges.Interval1m), exchanges.ErrNotSupported)
}
