package testsuite

import (
	"context"
	"errors"
	"testing"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/stretchr/testify/require"
)

type VenueContractSuiteConfig struct {
	Provider                 venue.InstrumentProvider
	MarketData               venue.MarketDataClient
	InstrumentID             model.InstrumentID
	ExpectTradesUnsupported  bool
	ExpectStreamsUnsupported bool
}

func RunVenueContractSuite(t *testing.T, cfg VenueContractSuiteConfig) {
	t.Helper()
	require.NotNil(t, cfg.Provider, "Provider is required")
	require.NotNil(t, cfg.MarketData, "MarketData is required")
	require.NoError(t, cfg.InstrumentID.Validate())

	ctx := context.Background()

	t.Run("ProviderLoadAllAndGet", func(t *testing.T) {
		require.NoError(t, cfg.Provider.LoadAll(ctx))
		inst, ok := cfg.Provider.Get(cfg.InstrumentID)
		require.True(t, ok)
		require.Equal(t, cfg.InstrumentID, inst.ID)
		require.NoError(t, inst.Validate())
	})

	t.Run("ProviderLoad", func(t *testing.T) {
		inst, err := cfg.Provider.Load(ctx, cfg.InstrumentID)
		require.NoError(t, err)
		require.Equal(t, cfg.InstrumentID, inst.ID)
	})

	t.Run("MarketDataByInstrumentID", func(t *testing.T) {
		ticker, err := cfg.MarketData.FetchTicker(ctx, cfg.InstrumentID)
		require.NoError(t, err)
		require.Equal(t, cfg.InstrumentID, ticker.InstrumentID)

		book, err := cfg.MarketData.FetchOrderBook(ctx, cfg.InstrumentID, 5)
		require.NoError(t, err)
		require.Equal(t, cfg.InstrumentID, book.InstrumentID)
	})

	if cfg.ExpectTradesUnsupported {
		t.Run("TradesUnsupported", func(t *testing.T) {
			_, err := cfg.MarketData.FetchTrades(ctx, cfg.InstrumentID, venue.TradeQuery{})
			require.True(t, errors.Is(err, model.ErrNotSupported))
		})
	}

	if cfg.ExpectStreamsUnsupported {
		t.Run("StreamsUnsupported", func(t *testing.T) {
			_, err := cfg.MarketData.SubscribeTicker(ctx, cfg.InstrumentID, nil)
			require.True(t, errors.Is(err, model.ErrNotSupported))
			_, err = cfg.MarketData.SubscribeOrderBook(ctx, cfg.InstrumentID, 5, nil)
			require.True(t, errors.Is(err, model.ErrNotSupported))
		})
	}
}
