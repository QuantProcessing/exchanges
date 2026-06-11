package binance

import (
	"context"
	"testing"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/testsuite"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/stretchr/testify/require"
)

func TestV2ContractSuites(t *testing.T) {
	instID := model.MustInstrumentID("BTC-USDT-PERP.BINANCE")
	provider := newV2InstrumentProviderForTest([]v2InstrumentSeed{
		{RawSymbol: "BTCUSDT", Product: venue.ProductHintPerp, Base: model.BTC, Quote: model.USDT},
	})
	require.NoError(t, provider.LoadAll(context.Background()))
	inst, ok := provider.Get(instID)
	require.True(t, ok)

	testsuite.RunV2ModelSuite(t, testsuite.V2ModelSuiteConfig{
		Instrument: inst,
		Account: model.AccountState{
			AccountID: "acct",
			Venue:     model.VenueBinance,
			Type:      model.AccountTypeMargin,
			Reported:  true,
		},
	})

	testsuite.RunV2VenueSuite(t, testsuite.V2VenueSuiteConfig{
		Provider:                 provider,
		MarketData:               newV2MarketDataClient(provider, nil, &fakeV2PerpMarketData{}),
		InstrumentID:             instID,
		ExpectTradesUnsupported:  true,
		ExpectStreamsUnsupported: true,
	})

	testsuite.RunV2LifecycleSuite(t, testsuite.V2LifecycleSuiteConfig{
		Execution:   newV2ExecutionClient("acct", provider, nil, &fakeV2PerpExecution{}),
		Instruments: []model.InstrumentID{instID},
	})
}
