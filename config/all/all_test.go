package all_test

import (
	"context"
	"testing"

	_ "github.com/QuantProcessing/exchanges/config/all"
	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/testsuite"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/stretchr/testify/require"
)

func TestAllRegistersBinanceVenue(t *testing.T) {
	adp, err := venue.Open(context.Background(), model.Venue("BINANCE"), map[string]string{"account_type": "spot"})
	require.NoError(t, err)
	require.Equal(t, model.Venue("BINANCE"), adp.Venue())
	require.NotNil(t, adp.Data())
	require.NotNil(t, adp.Execution())
	require.NoError(t, adp.Close(context.Background()))

	adp, err = venue.Open(context.Background(), model.Venue("BINANCE"), map[string]string{"account_type": "perp"})
	require.NoError(t, err)
	require.Equal(t, model.Venue("BINANCE"), adp.Venue())
	require.NotNil(t, adp.Data())
	require.NotNil(t, adp.Execution())
	require.NoError(t, adp.Close(context.Background()))
}

func TestAllRegistersAsterVenue(t *testing.T) {
	adp, err := venue.Open(context.Background(), model.Venue("ASTER"), map[string]string{"account_type": "spot"})
	require.NoError(t, err)
	require.Equal(t, model.Venue("ASTER"), adp.Venue())
	require.NotNil(t, adp.Data())
	require.NotNil(t, adp.Execution())
	require.NoError(t, adp.Close(context.Background()))

	adp, err = venue.Open(context.Background(), model.Venue("ASTER"), map[string]string{"account_type": "perp"})
	require.NoError(t, err)
	require.Equal(t, model.Venue("ASTER"), adp.Venue())
	require.NotNil(t, adp.Data())
	require.NotNil(t, adp.Execution())
	require.NoError(t, adp.Close(context.Background()))
}

func TestAllRegistersOKXVenue(t *testing.T) {
	adp, err := venue.Open(context.Background(), model.Venue("OKX"), map[string]string{"account_type": "spot"})
	require.NoError(t, err)
	require.Equal(t, model.Venue("OKX"), adp.Venue())
	require.NotNil(t, adp.Data())
	require.NotNil(t, adp.Execution())
	require.NoError(t, adp.Close(context.Background()))

	adp, err = venue.Open(context.Background(), model.Venue("OKX"), map[string]string{"account_type": "swap"})
	require.NoError(t, err)
	require.Equal(t, model.Venue("OKX"), adp.Venue())
	require.NotNil(t, adp.Data())
	require.NotNil(t, adp.Execution())
	require.NoError(t, adp.Close(context.Background()))
}

func TestAllRegistersBybitVenue(t *testing.T) {
	adp, err := venue.Open(context.Background(), model.Venue("BYBIT"), map[string]string{"account_type": "spot"})
	require.NoError(t, err)
	require.Equal(t, model.Venue("BYBIT"), adp.Venue())
	require.NotNil(t, adp.Data())
	require.NotNil(t, adp.Execution())
	require.NoError(t, adp.Close(context.Background()))

	adp, err = venue.Open(context.Background(), model.Venue("BYBIT"), map[string]string{"account_type": "linear"})
	require.NoError(t, err)
	require.Equal(t, model.Venue("BYBIT"), adp.Venue())
	require.NotNil(t, adp.Data())
	require.NotNil(t, adp.Execution())
	require.NoError(t, adp.Close(context.Background()))
}

func TestAllRegistersBitgetVenue(t *testing.T) {
	adp, err := venue.Open(context.Background(), model.Venue("BITGET"), map[string]string{"account_type": "spot"})
	require.NoError(t, err)
	require.Equal(t, model.Venue("BITGET"), adp.Venue())
	require.NotNil(t, adp.Data())
	require.NotNil(t, adp.Execution())
	require.NoError(t, adp.Close(context.Background()))

	adp, err = venue.Open(context.Background(), model.Venue("BITGET"), map[string]string{"account_type": "perp"})
	require.NoError(t, err)
	require.Equal(t, model.Venue("BITGET"), adp.Venue())
	require.NotNil(t, adp.Data())
	require.NotNil(t, adp.Execution())
	require.NoError(t, adp.Close(context.Background()))
}

func TestAllRegistersHyperliquidVenue(t *testing.T) {
	adp, err := venue.Open(context.Background(), model.Venue("HYPERLIQUID"), map[string]string{"account_type": "spot"})
	require.NoError(t, err)
	require.Equal(t, model.Venue("HYPERLIQUID"), adp.Venue())
	require.NotNil(t, adp.Data())
	require.NotNil(t, adp.Execution())
	require.NoError(t, adp.Close(context.Background()))

	adp, err = venue.Open(context.Background(), model.Venue("HYPERLIQUID"), map[string]string{"account_type": "perp"})
	require.NoError(t, err)
	require.Equal(t, model.Venue("HYPERLIQUID"), adp.Venue())
	require.NotNil(t, adp.Data())
	require.NotNil(t, adp.Execution())
	require.NoError(t, adp.Close(context.Background()))
}

func TestAllRegistersLighterVenue(t *testing.T) {
	adp, err := venue.Open(context.Background(), model.Venue("LIGHTER"), map[string]string{"account_type": "perp"})
	require.NoError(t, err)
	require.Equal(t, model.Venue("LIGHTER"), adp.Venue())
	require.NotNil(t, adp.Data())
	require.NotNil(t, adp.Execution())
	require.NoError(t, adp.Close(context.Background()))

	_, err = venue.Open(context.Background(), model.Venue("LIGHTER"), map[string]string{"account_type": "spot"})
	require.ErrorIs(t, err, model.ErrNotSupported)
}

func TestAllRegistersNadoVenue(t *testing.T) {
	adp, err := venue.Open(context.Background(), model.Venue("NADO"), map[string]string{"account_type": "perp"})
	require.NoError(t, err)
	require.Equal(t, model.Venue("NADO"), adp.Venue())
	require.NotNil(t, adp.Data())
	require.NotNil(t, adp.Execution())
	require.NoError(t, adp.Close(context.Background()))

	_, err = venue.Open(context.Background(), model.Venue("NADO"), map[string]string{"account_type": "spot"})
	require.ErrorIs(t, err, model.ErrNotSupported)
}

func TestAllRegistersEdgeXVenue(t *testing.T) {
	adp, err := venue.Open(context.Background(), model.Venue("EDGEX"), map[string]string{"account_type": "perp"})
	require.NoError(t, err)
	require.Equal(t, model.Venue("EDGEX"), adp.Venue())
	require.NotNil(t, adp.Data())
	require.NotNil(t, adp.Execution())
	require.NoError(t, adp.Close(context.Background()))

	_, err = venue.Open(context.Background(), model.Venue("EDGEX"), map[string]string{"account_type": "spot"})
	require.ErrorIs(t, err, model.ErrNotSupported)
}

func TestAllRegistersGRVTVenue(t *testing.T) {
	adp, err := venue.Open(context.Background(), model.Venue("GRVT"), map[string]string{"account_type": "perp"})
	require.NoError(t, err)
	require.Equal(t, model.Venue("GRVT"), adp.Venue())
	require.NotNil(t, adp.Data())
	require.NotNil(t, adp.Execution())
	require.NoError(t, adp.Close(context.Background()))

	_, err = venue.Open(context.Background(), model.Venue("GRVT"), map[string]string{"account_type": "spot"})
	require.ErrorIs(t, err, model.ErrNotSupported)
}

func TestAllRegistersStandXVenue(t *testing.T) {
	adp, err := venue.Open(context.Background(), model.Venue("STANDX"), map[string]string{"account_type": "perp"})
	require.NoError(t, err)
	require.Equal(t, model.Venue("STANDX"), adp.Venue())
	require.NotNil(t, adp.Data())
	require.NotNil(t, adp.Execution())
	require.NoError(t, adp.Close(context.Background()))

	_, err = venue.Open(context.Background(), model.Venue("STANDX"), map[string]string{"account_type": "spot"})
	require.ErrorIs(t, err, model.ErrNotSupported)
}

func TestAllRegistersBackpackVenue(t *testing.T) {
	adp, err := venue.Open(context.Background(), model.Venue("BACKPACK"), map[string]string{"account_type": "perp"})
	require.NoError(t, err)
	require.Equal(t, model.Venue("BACKPACK"), adp.Venue())
	require.NotNil(t, adp.Data())
	require.NotNil(t, adp.Execution())
	require.NoError(t, adp.Close(context.Background()))

	_, err = venue.Open(context.Background(), model.Venue("BACKPACK"), map[string]string{"account_type": "spot"})
	require.ErrorIs(t, err, model.ErrNotSupported)
}

func TestAllRegisteredAdaptersSatisfyDeclaredCapabilityContracts(t *testing.T) {
	cases := []struct {
		name  string
		venue model.Venue
		cfg   map[string]string
	}{
		{name: "binance_spot", venue: "BINANCE", cfg: map[string]string{"account_type": "spot"}},
		{name: "binance_perp", venue: "BINANCE", cfg: map[string]string{"account_type": "perp"}},
		{name: "aster_spot", venue: "ASTER", cfg: map[string]string{"account_type": "spot"}},
		{name: "aster_perp", venue: "ASTER", cfg: map[string]string{"account_type": "perp"}},
		{name: "okx_spot", venue: "OKX", cfg: map[string]string{"account_type": "spot"}},
		{name: "okx_swap", venue: "OKX", cfg: map[string]string{"account_type": "swap"}},
		{name: "bybit_spot", venue: "BYBIT", cfg: map[string]string{"account_type": "spot"}},
		{name: "bybit_linear", venue: "BYBIT", cfg: map[string]string{"account_type": "linear"}},
		{name: "bitget_spot", venue: "BITGET", cfg: map[string]string{"account_type": "spot"}},
		{name: "bitget_perp", venue: "BITGET", cfg: map[string]string{"account_type": "perp"}},
		{name: "hyperliquid_spot", venue: "HYPERLIQUID", cfg: map[string]string{"account_type": "spot"}},
		{name: "hyperliquid_perp", venue: "HYPERLIQUID", cfg: map[string]string{"account_type": "perp"}},
		{name: "lighter_perp", venue: "LIGHTER", cfg: map[string]string{"account_type": "perp"}},
		{name: "nado_perp", venue: "NADO", cfg: map[string]string{"account_type": "perp"}},
		{name: "edgex_perp", venue: "EDGEX", cfg: map[string]string{"account_type": "perp"}},
		{name: "grvt_perp", venue: "GRVT", cfg: map[string]string{"account_type": "perp"}},
		{name: "standx_perp", venue: "STANDX", cfg: map[string]string{"account_type": "perp"}},
		{name: "backpack_perp", venue: "BACKPACK", cfg: map[string]string{"account_type": "perp"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			adp, err := venue.Open(context.Background(), tc.venue, tc.cfg)
			require.NoError(t, err)
			t.Cleanup(func() {
				require.NoError(t, adp.Close(context.Background()))
			})
			testsuite.RunAdapterCapabilitySuite(t, testsuite.AdapterCapabilityConfig{Adapter: adp})
		})
	}
}
