package aster

import (
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
)

func TestOptions_QuoteCurrency_Default_DEX(t *testing.T) {
	opts := Options{}
	q, err := opts.quoteCurrency()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// DEX default is USDC
	if q != exchanges.QuoteCurrencyUSDC {
		t.Errorf("default quote = %q, want %q (DEX default)", q, exchanges.QuoteCurrencyUSDC)
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
