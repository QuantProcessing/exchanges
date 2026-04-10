package all_test

import (
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	_ "github.com/QuantProcessing/exchanges/config/all"
	"github.com/stretchr/testify/require"
)

func TestAllPackageRegistersKnownExchangeConstructors(t *testing.T) {
	t.Parallel()

	_, err := exchanges.LookupConstructor("BINANCE")
	require.NoError(t, err)

	_, err = exchanges.LookupConstructor("OKX")
	require.NoError(t, err)

	_, err = exchanges.LookupConstructor("ASTER")
	require.NoError(t, err)

	_, err = exchanges.LookupConstructor("BYBIT")
	require.NoError(t, err)
}
