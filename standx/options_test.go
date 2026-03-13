package standx

import (
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
)

func TestOptions_QuoteCurrency_Default(t *testing.T) {
	opts := Options{}
	q, err := opts.quoteCurrency()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if q != exchanges.QuoteCurrencyDUSD {
		t.Errorf("default quote = %q, want %q", q, exchanges.QuoteCurrencyDUSD)
	}
}

func TestOptions_QuoteCurrency_DUSD(t *testing.T) {
	opts := Options{QuoteCurrency: exchanges.QuoteCurrencyDUSD}
	q, err := opts.quoteCurrency()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if q != exchanges.QuoteCurrencyDUSD {
		t.Errorf("quote = %q, want %q", q, exchanges.QuoteCurrencyDUSD)
	}
}

func TestOptions_QuoteCurrency_Unsupported(t *testing.T) {
	unsupported := []exchanges.QuoteCurrency{
		exchanges.QuoteCurrencyUSDT,
		exchanges.QuoteCurrencyUSDC,
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
