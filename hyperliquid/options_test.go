package hyperliquid

import (
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/stretchr/testify/require"
)

func TestOptions_QuoteCurrency_Default_DEX(t *testing.T) {
	opts := Options{}
	q, err := opts.quoteCurrency()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if q != exchanges.QuoteCurrencyUSDC {
		t.Errorf("default quote = %q, want %q (DEX default)", q, exchanges.QuoteCurrencyUSDC)
	}
}

func TestOptions_QuoteCurrency_Unsupported(t *testing.T) {
	unsupported := []exchanges.QuoteCurrency{
		exchanges.QuoteCurrencyUSDT,
		exchanges.QuoteCurrencyDUSD,
	}
	for _, q := range unsupported {
		opts := Options{QuoteCurrency: q}
		_, err := opts.quoteCurrency()
		if err == nil {
			t.Errorf("expected error for unsupported %q, got nil", q)
		}
	}
}

func TestOptions_ValidateCredentials_AllowsPublicAndReadOnlyConfigs(t *testing.T) {
	cases := []Options{
		{},
		{AccountAddr: "0x1234"},
		{PrivateKey: "4f3edf983ac636a65a842ce7c78d9aa706d3b113bce036f1ee7e737d6f71f6b2"},
	}

	for _, opts := range cases {
		require.NoError(t, opts.validateCredentials())
	}
}

func TestOptions_ValidateCredentials_RejectsInvalidPrivateKey(t *testing.T) {
	err := Options{PrivateKey: "not-a-private-key"}.validateCredentials()
	require.ErrorIs(t, err, exchanges.ErrAuthFailed)
}

func TestOptions_AccountAddr_DerivesFromPrivateKey(t *testing.T) {
	opts := Options{PrivateKey: "4f3edf983ac636a65a842ce7c78d9aa706d3b113bce036f1ee7e737d6f71f6b2"}
	require.Equal(t, "0xb942f2747DA5C1bB6353a8B51a58e84Ae843c333", opts.accountAddr())
}

func TestOptions_AccountAddr_PrefersExplicitAddress(t *testing.T) {
	opts := Options{
		PrivateKey:  "4f3edf983ac636a65a842ce7c78d9aa706d3b113bce036f1ee7e737d6f71f6b2",
		AccountAddr: "0xabc",
	}
	require.Equal(t, "0xabc", opts.accountAddr())
}
