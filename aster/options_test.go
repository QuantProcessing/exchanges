package aster

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

func TestValidateCredentialsRejectsPartialSet(t *testing.T) {
	testCases := []Options{
		{APIKey: "key"},
		{SecretKey: "secret"},
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
	if q != exchanges.QuoteCurrencyUSDT {
		t.Errorf("default quote = %q, want %q", q, exchanges.QuoteCurrencyUSDT)
	}
}

func TestOptions_QuoteCurrency_Supported(t *testing.T) {
	supported := []exchanges.QuoteCurrency{
		exchanges.QuoteCurrencyUSDT,
		exchanges.QuoteCurrencyUSDC,
	}
	for _, want := range supported {
		opts := Options{QuoteCurrency: want}
		got, err := opts.quoteCurrency()
		if err != nil {
			t.Fatalf("unexpected error for %q: %v", want, err)
		}
		if got != want {
			t.Errorf("quote = %q, want %q", got, want)
		}
	}
}

func TestOptions_QuoteCurrency_Unsupported(t *testing.T) {
	unsupported := []exchanges.QuoteCurrency{
		exchanges.QuoteCurrencyDUSD,
		"EUR",
	}
	for _, q := range unsupported {
		opts := Options{QuoteCurrency: q}
		_, err := opts.quoteCurrency()
		if err == nil {
			t.Errorf("expected error for unsupported %q, got nil", q)
		}
	}
}
