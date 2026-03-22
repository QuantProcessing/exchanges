package backpack

import (
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/stretchr/testify/require"
)

func TestOptionsQuoteCurrencyDefaultsToUSDC(t *testing.T) {
	q, err := (Options{}).quoteCurrency()
	require.NoError(t, err)
	require.Equal(t, exchanges.QuoteCurrencyUSDC, q)
}

func TestOptionsRejectsUnsupportedQuoteCurrency(t *testing.T) {
	_, err := (Options{QuoteCurrency: exchanges.QuoteCurrencyUSDT}).quoteCurrency()
	require.Error(t, err)
}
