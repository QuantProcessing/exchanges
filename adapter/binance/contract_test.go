package binance

import (
	"context"
	"testing"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/testsuite"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/stretchr/testify/require"
)

func TestContractSuites(t *testing.T) {
	instID := model.MustInstrumentID("BTC-USDT-PERP.BINANCE")
	provider := newInstrumentProviderForTest([]instrumentSeed{
		{RawSymbol: "BTCUSDT", Product: venue.ProductHintPerp, Base: model.BTC, Quote: model.USDT},
	})
	require.NoError(t, provider.LoadAll(context.Background()))
	inst, ok := provider.Get(instID)
	require.True(t, ok)

	testsuite.RunModelContractSuite(t, testsuite.ModelContractSuiteConfig{
		Instrument: inst,
		Account: model.AccountState{
			AccountID: "acct",
			Venue:     model.VenueBinance,
			Type:      model.AccountTypeMargin,
			Reported:  true,
		},
	})

	testsuite.RunVenueContractSuite(t, testsuite.VenueContractSuiteConfig{
		Provider:                 provider,
		MarketData:               newMarketDataClient(provider, nil, &fakePerpMarketData{}),
		InstrumentID:             instID,
		ExpectTradesUnsupported:  true,
		ExpectStreamsUnsupported: true,
	})

	testsuite.RunAccountLifecycleSuite(t, testsuite.AccountLifecycleSuiteConfig{
		Execution:   newPerpExecutionClient("acct", provider, &fakePerpExecution{}, nil),
		Instruments: []model.InstrumentID{instID},
	})
}
