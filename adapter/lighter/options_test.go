package lighter

import (
	"errors"
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
)

func TestValidateCredentialsAllowsEmptySet(t *testing.T) {
	if err := (Options{}).validateCredentials(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateCredentialsRejectsInvalidPartialSets(t *testing.T) {
	testCases := []Options{
		{PrivateKey: "private"},
		{KeyIndex: "1"},
		{RoToken: "token"},
		{PrivateKey: "private", RoToken: "token"},
	}

	for _, opts := range testCases {
		err := opts.validateCredentials()
		if err == nil {
			t.Fatal("expected partial credentials to be rejected")
		}
		if !errors.Is(err, exchanges.ErrAuthFailed) {
			t.Fatalf("expected ErrAuthFailed, got %v", err)
		}
	}
}

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
