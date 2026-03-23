package grvt

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
		{SubAccountID: "7"},
		{PrivateKey: "private"},
		{APIKey: "key", PrivateKey: "private"},
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

func TestOptions_QuoteCurrency_Default(t *testing.T) {
	opts := Options{}
	q, err := opts.quoteCurrency()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// GRVT currently only supports USDT
	if q != exchanges.QuoteCurrencyUSDT {
		t.Errorf("default quote = %q, want %q", q, exchanges.QuoteCurrencyUSDT)
	}
}

func TestOptions_QuoteCurrency_USDT(t *testing.T) {
	opts := Options{QuoteCurrency: exchanges.QuoteCurrencyUSDT}
	q, err := opts.quoteCurrency()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if q != exchanges.QuoteCurrencyUSDT {
		t.Errorf("quote = %q, want %q", q, exchanges.QuoteCurrencyUSDT)
	}
}

func TestOptions_QuoteCurrency_Unsupported(t *testing.T) {
	unsupported := []exchanges.QuoteCurrency{
		exchanges.QuoteCurrencyUSDC, // not yet supported
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
