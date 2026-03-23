package backpack

import (
	"context"
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/backpack/sdk"
	"github.com/stretchr/testify/require"
)

func TestSpotFetchBalanceMissingQuoteReturnsErrSymbolNotFound(t *testing.T) {
	client := &backpackStubClient{
		getMarkets: func(context.Context) ([]sdk.Market, error) {
			return []sdk.Market{testBackpackSpotMarket()}, nil
		},
		getBalances: func(context.Context) (map[string]sdk.CapitalBalance, error) {
			return map[string]sdk.CapitalBalance{
				"USDT": {
					Available: "10",
					Locked:    "0",
					Staked:    "0",
				},
			}, nil
		},
	}

	adp, err := newSpotAdapterWithClient(context.Background(), func() {}, Options{}, exchanges.QuoteCurrencyUSDC, client)
	require.NoError(t, err)

	_, err = adp.FetchBalance(context.Background())
	require.ErrorIs(t, err, exchanges.ErrSymbolNotFound)
}
