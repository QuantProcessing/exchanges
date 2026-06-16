package examples

import (
	"context"

	"github.com/QuantProcessing/exchanges/adapter/binance"
	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/venue"
)

// FetchTickerWithDataClient shows the smallest normalized market-data path:
// connect a venue.DataClient, request one ticker, and disconnect it.
func FetchTickerWithDataClient(ctx context.Context, client venue.DataClient, instrumentID model.InstrumentID) (model.Ticker, error) {
	if err := client.Connect(ctx); err != nil {
		return model.Ticker{}, err
	}
	defer client.Disconnect(context.Background())

	return client.FetchTicker(ctx, instrumentID)
}

// FetchBTCUSDTFromBinanceSpot is the same workflow using the concrete Binance
// spot adapter. It calls Binance public endpoints, so tests compile this helper
// but do not execute it by default.
func FetchBTCUSDTFromBinanceSpot(ctx context.Context) (model.Ticker, error) {
	adapter, err := binance.NewSpotAdapter(ctx, binance.Options{})
	if err != nil {
		return model.Ticker{}, err
	}
	defer adapter.Close(context.Background())

	return FetchTickerWithDataClient(ctx, adapter.Data(), model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"))
}
