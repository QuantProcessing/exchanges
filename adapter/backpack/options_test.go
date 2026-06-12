package backpack

import (
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/stretchr/testify/require"
)

func TestValidateCredentialsAllowsEmptySet(t *testing.T) {
	require.NoError(t, (Options{}).validateCredentials())
}

func TestValidateCredentialsRejectsPartialSet(t *testing.T) {
	testCases := []Options{
		{APIKey: "key"},
		{PrivateKey: "private"},
	}

	for _, opts := range testCases {
		err := opts.validateCredentials()
		require.ErrorIs(t, err, exchanges.ErrAuthFailed)
	}
}

func TestOptionsQuoteCurrencyDefaultsToUSDC(t *testing.T) {
	q, err := (Options{}).quoteCurrency()
	require.NoError(t, err)
	require.Equal(t, exchanges.QuoteCurrencyUSDC, q)
}

func TestOptionsRejectsUnsupportedQuoteCurrency(t *testing.T) {
	_, err := (Options{QuoteCurrency: exchanges.QuoteCurrencyUSDT}).quoteCurrency()
	require.Error(t, err)
}
