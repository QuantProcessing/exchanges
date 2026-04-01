package exchanges_test

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveLiveOrderBookSymbolFromListPrefersExactSymbol(t *testing.T) {
	t.Parallel()

	symbol, err := resolveLiveOrderBookSymbolFromList("btc", []string{"ETH", "BTC", "SOL"})
	require.NoError(t, err)
	require.Equal(t, "BTC", symbol)
}

func TestResolveLiveOrderBookSymbolFromListFailsWhenSymbolUnavailable(t *testing.T) {
	t.Parallel()

	_, err := resolveLiveOrderBookSymbolFromList("BTC", []string{"ETH", "SOL"})
	require.Error(t, err)
}
