package bybit

import (
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
)

func TestOptionsQuoteCurrencyDefault(t *testing.T) {
	opts := Options{}

	quote, err := opts.quoteCurrency()
	if err != nil {
		t.Fatalf("quoteCurrency() error = %v", err)
	}
	if quote != exchanges.QuoteCurrencyUSDT {
		t.Fatalf("quoteCurrency() = %q, want %q", quote, exchanges.QuoteCurrencyUSDT)
	}
}

func TestOptionsQuoteCurrencyRejectsUnsupported(t *testing.T) {
	opts := Options{QuoteCurrency: exchanges.QuoteCurrencyDUSD}

	if _, err := opts.quoteCurrency(); err == nil {
		t.Fatal("quoteCurrency() error = nil, want unsupported quote error")
	}
}
