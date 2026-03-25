package decibel

import (
	"context"
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/stretchr/testify/require"
)

func TestDecibelRegistryConstructsPerpAdapter(t *testing.T) {
	previous := bootstrapMetadata
	bootstrapMetadata = func(context.Context, *Adapter) error { return nil }
	t.Cleanup(func() {
		bootstrapMetadata = previous
	})

	constructor, err := exchanges.LookupConstructor("DECIBEL")
	require.NoError(t, err)

	adp, err := constructor(context.Background(), exchanges.MarketTypePerp, map[string]string{
		"api_key":         "api-key",
		"private_key":     "private-key",
		"subaccount_addr": "0xsubaccount",
	})
	require.NoError(t, err)
	require.NotNil(t, adp)
	require.Equal(t, "DECIBEL", adp.GetExchange())
	require.Equal(t, exchanges.MarketTypePerp, adp.GetMarketType())
}

func TestDecibelRegistryRejectsUnsupportedMarketType(t *testing.T) {
	constructor, err := exchanges.LookupConstructor("DECIBEL")
	require.NoError(t, err)

	adp, err := constructor(context.Background(), exchanges.MarketTypeSpot, map[string]string{
		"api_key":         "api-key",
		"private_key":     "private-key",
		"subaccount_addr": "0xsubaccount",
	})
	require.Nil(t, adp)
	require.ErrorContains(t, err, "unsupported market type")
}
