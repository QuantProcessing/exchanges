package binance

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
		{SecretKey: "secret"},
	}

	for _, opts := range testCases {
		err := opts.validateCredentials()
		require.ErrorIs(t, err, exchanges.ErrAuthFailed)
	}
}
