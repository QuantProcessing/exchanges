package binance

import (
	"context"
	"testing"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/sdk/binance/perp"
	"github.com/QuantProcessing/exchanges/sdk/binance/spot"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/stretchr/testify/require"
)

func TestMarketDataClientFetchTickerRejectsUnloadedInstrument(t *testing.T) {
	provider := newInstrumentProviderForTest(nil)
	client := newMarketDataClient(provider, nil, nil)

	_, err := client.FetchTicker(context.Background(), model.MustInstrumentID("BTC-USDT-PERP.BINANCE"))
	require.ErrorIs(t, err, model.ErrInstrumentNotLoaded)
}

func TestMarketDataClientFetchTickerRoutesSpot(t *testing.T) {
	provider := newInstrumentProviderForTest([]instrumentSeed{
		{RawSymbol: "BTCUSDT", Product: venue.ProductHintSpot, Base: model.BTC, Quote: model.USDT},
	})
	require.NoError(t, provider.LoadAll(context.Background()))
	spotClient := &fakeSpotMarketData{}
	client := newMarketDataClient(provider, spotClient, nil)

	got, err := client.FetchTicker(context.Background(), model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"))
	require.NoError(t, err)
	require.Equal(t, "BTCUSDT", spotClient.bookTickerSymbol)
	require.Equal(t, model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"), got.InstrumentID)
	require.Equal(t, "100.1", got.Bid.String())
	require.Equal(t, "100.2", got.Ask.String())
	require.Equal(t, "100.15", got.Last.String())
}

func TestMarketDataClientFetchOrderBookRoutesPerp(t *testing.T) {
	provider := newInstrumentProviderForTest([]instrumentSeed{
		{RawSymbol: "BTCUSDT", Product: venue.ProductHintPerp, Base: model.BTC, Quote: model.USDT},
	})
	require.NoError(t, provider.LoadAll(context.Background()))
	perpClient := &fakePerpMarketData{}
	client := newMarketDataClient(provider, nil, perpClient)

	got, err := client.FetchOrderBook(context.Background(), model.MustInstrumentID("BTC-USDT-PERP.BINANCE"), 5)
	require.NoError(t, err)
	require.Equal(t, "BTCUSDT", perpClient.depthSymbol)
	require.Equal(t, 5, perpClient.depthLimit)
	require.Len(t, got.Bids, 1)
	require.Equal(t, "99.9", got.Bids[0].Price.String())
	require.Len(t, got.Asks, 1)
	require.Equal(t, "100.3", got.Asks[0].Price.String())
}

type fakeSpotMarketData struct {
	bookTickerSymbol string
	tickerSymbol     string
	depthSymbol      string
	depthLimit       int
}

func (f *fakeSpotMarketData) BookTicker(ctx context.Context, symbol string) (*spot.BookTickerResponse, error) {
	f.bookTickerSymbol = symbol
	return &spot.BookTickerResponse{
		Symbol:   symbol,
		BidPrice: "100.1",
		BidQty:   "1",
		AskPrice: "100.2",
		AskQty:   "2",
	}, nil
}

func (f *fakeSpotMarketData) Ticker(ctx context.Context, symbol string) (*spot.TickerResponse, error) {
	f.tickerSymbol = symbol
	return &spot.TickerResponse{Symbol: symbol, LastPrice: "100.15"}, nil
}

func (f *fakeSpotMarketData) Depth(ctx context.Context, symbol string, limit int) (*spot.DepthResponse, error) {
	f.depthSymbol = symbol
	f.depthLimit = limit
	return &spot.DepthResponse{
		Bids: [][]string{{"99.9", "1"}},
		Asks: [][]string{{"100.3", "2"}},
	}, nil
}

type fakePerpMarketData struct {
	tickerSymbol string
	depthSymbol  string
	depthLimit   int
}

func (f *fakePerpMarketData) Ticker(ctx context.Context, symbol string) (*perp.TickerResponse, error) {
	f.tickerSymbol = symbol
	return &perp.TickerResponse{Symbol: symbol, LastPrice: "100.15"}, nil
}

func (f *fakePerpMarketData) Depth(ctx context.Context, symbol string, limit int) (*perp.DepthResponse, error) {
	f.depthSymbol = symbol
	f.depthLimit = limit
	return &perp.DepthResponse{
		Bids: [][]string{{"99.9", "1"}},
		Asks: [][]string{{"100.3", "2"}},
	}, nil
}
