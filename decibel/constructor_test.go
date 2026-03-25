package decibel

import (
	"context"
	"errors"
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/stretchr/testify/require"
)

func TestDecibelRegistryConstructsPerpAdapterCallsBootstrap(t *testing.T) {
	previous := bootstrapMetadata
	var (
		callCount int
		seen      *Adapter
	)
	bootstrapMetadata = func(_ context.Context, adp *Adapter) error {
		callCount++
		seen = adp
		return nil
	}
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
	got, ok := adp.(*Adapter)
	require.True(t, ok)
	require.Equal(t, "DECIBEL", adp.GetExchange())
	require.Equal(t, exchanges.MarketTypePerp, adp.GetMarketType())
	require.Equal(t, 1, callCount)
	require.Same(t, got, seen)
	require.Equal(t, "api-key", seen.apiKey)
	require.Equal(t, "private-key", seen.privateKey)
	require.Equal(t, "0xsubaccount", seen.subaccountAddr)
}

func TestDecibelRegistryReturnsBootstrapError(t *testing.T) {
	previous := bootstrapMetadata
	bootstrapErr := errors.New("bootstrap failed")
	bootstrapMetadata = func(context.Context, *Adapter) error {
		return bootstrapErr
	}
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
	require.Nil(t, adp)
	require.ErrorContains(t, err, "decibel init")
	require.ErrorIs(t, err, bootstrapErr)
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
