package decibel

import (
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/stretchr/testify/require"
)

func TestDecibelOptionsRejectsPartialCredentials(t *testing.T) {
	testCases := []Options{
		{APIKey: "api-key"},
		{PrivateKey: "private-key"},
		{SubaccountAddr: "0xsubaccount"},
		{APIKey: "api-key", PrivateKey: "private-key"},
		{APIKey: "api-key", SubaccountAddr: "0xsubaccount"},
		{PrivateKey: "private-key", SubaccountAddr: "0xsubaccount"},
	}

	for _, opts := range testCases {
		err := opts.validateCredentials()
		require.ErrorIs(t, err, exchanges.ErrAuthFailed)
	}
}

func TestDecibelOptionsRejectsEmptyCredentials(t *testing.T) {
	err := (Options{}).validateCredentials()
	require.ErrorIs(t, err, exchanges.ErrAuthFailed)
}

func TestDecibelOptionsAcceptsFullCredentialSet(t *testing.T) {
	opts := Options{
		APIKey:         "api-key",
		PrivateKey:     "private-key",
		SubaccountAddr: "0xsubaccount",
	}

	require.NoError(t, opts.validateCredentials())
}
