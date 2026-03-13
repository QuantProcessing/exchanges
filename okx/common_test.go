package okx

import (
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
)

// ============================================================================
// FormatSymbolWithQuote / FormatSpotSymbolWithQuote / ExtractSymbolWithQuote
// ============================================================================

func TestFormatSymbolWithQuote_Perp(t *testing.T) {
	tests := []struct {
		name     string
		symbol   string
		quote    string
		instType string
		want     string
	}{
		{"BTC+USDT+SWAP", "BTC", "USDT", "SWAP", "BTC-USDT-SWAP"},
		{"ETH+USDT+SWAP", "ETH", "USDT", "SWAP", "ETH-USDT-SWAP"},
		{"BTC+USDC+SWAP", "BTC", "USDC", "SWAP", "BTC-USDC-SWAP"},
		{"ETH+USDC+SWAP", "eth", "USDC", "SWAP", "ETH-USDC-SWAP"},
		{"already formatted", "BTC-USDT-SWAP", "USDT", "SWAP", "BTC-USDT-SWAP"},
		{"SOL+USDC+SWAP", "SOL", "USDC", "SWAP", "SOL-USDC-SWAP"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatSymbolWithQuote(tt.symbol, tt.quote, tt.instType)
			if got != tt.want {
				t.Errorf("FormatSymbolWithQuote(%q, %q, %q) = %q, want %q",
					tt.symbol, tt.quote, tt.instType, got, tt.want)
			}
		})
	}
}

func TestFormatSpotSymbolWithQuote(t *testing.T) {
	tests := []struct {
		name   string
		symbol string
		quote  string
		want   string
	}{
		{"BTC+USDT", "BTC", "USDT", "BTC-USDT"},
		{"ETH+USDC", "ETH", "USDC", "ETH-USDC"},
		{"already has dash", "BTC-USDT", "USDT", "BTC-USDT"},
		{"already has dash different", "BTC-USDC", "USDT", "BTC-USDC"},
		{"lowercase", "btc", "USDT", "BTC-USDT"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatSpotSymbolWithQuote(tt.symbol, tt.quote)
			if got != tt.want {
				t.Errorf("FormatSpotSymbolWithQuote(%q, %q) = %q, want %q",
					tt.symbol, tt.quote, got, tt.want)
			}
		})
	}
}

func TestExtractSymbolWithQuote(t *testing.T) {
	tests := []struct {
		name   string
		symbol string
		quote  string
		want   string
	}{
		{"perp USDT", "BTC-USDT-SWAP", "USDT", "BTC"},
		{"perp USDC", "BTC-USDC-SWAP", "USDC", "BTC"},
		{"spot USDT", "ETH-USDT", "USDT", "ETH"},
		{"spot USDC", "ETH-USDC", "USDC", "ETH"},
		{"no match", "BTC-USDC-SWAP", "USDT", "BTC-USDC-SWAP"},
		{"bare symbol", "BTC", "USDT", "BTC"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractSymbolWithQuote(tt.symbol, tt.quote)
			if got != tt.want {
				t.Errorf("ExtractSymbolWithQuote(%q, %q) = %q, want %q",
					tt.symbol, tt.quote, got, tt.want)
			}
		})
	}
}

// Backward compatibility
func TestFormatSymbol_BackwardCompat(t *testing.T) {
	tests := []struct {
		symbol string
		want   string
	}{
		{"BTC", "BTC-USDT-SWAP"},
		{"ETH", "ETH-USDT-SWAP"},
		{"BTC-USDT-SWAP", "BTC-USDT-SWAP"},
	}
	for _, tt := range tests {
		got := FormatSymbol(tt.symbol)
		if got != tt.want {
			t.Errorf("FormatSymbol(%q) = %q, want %q", tt.symbol, got, tt.want)
		}
	}
}

func TestExtractSymbol_BackwardCompat(t *testing.T) {
	tests := []struct {
		symbol string
		want   string
	}{
		{"BTC-USDT-SWAP", "BTC"},
		{"ETH-USDT", "ETH"},
		{"BTC", "BTC"},
	}
	for _, tt := range tests {
		got := ExtractSymbol(tt.symbol)
		if got != tt.want {
			t.Errorf("ExtractSymbol(%q) = %q, want %q", tt.symbol, got, tt.want)
		}
	}
}

// Roundtrip: format → extract
func TestFormatExtract_Roundtrip_Perp(t *testing.T) {
	quotes := []string{"USDT", "USDC"}
	symbols := []string{"BTC", "ETH", "DOGE", "SOL"}

	for _, quote := range quotes {
		for _, sym := range symbols {
			formatted := FormatSymbolWithQuote(sym, quote, "SWAP")
			extracted := ExtractSymbolWithQuote(formatted, quote)
			if extracted != sym {
				t.Errorf("Roundtrip perp failed: %q → %q → %q, want %q", sym, formatted, extracted, sym)
			}
		}
	}
}

func TestFormatExtract_Roundtrip_Spot(t *testing.T) {
	quotes := []string{"USDT", "USDC"}
	symbols := []string{"BTC", "ETH", "DOGE"}

	for _, quote := range quotes {
		for _, sym := range symbols {
			formatted := FormatSpotSymbolWithQuote(sym, quote)
			extracted := ExtractSymbolWithQuote(formatted, quote)
			if extracted != sym {
				t.Errorf("Roundtrip spot failed: %q → %q → %q, want %q", sym, formatted, extracted, sym)
			}
		}
	}
}

// ============================================================================
// Options QuoteCurrency Validation
// ============================================================================

func TestOptions_QuoteCurrency_Default(t *testing.T) {
	opts := Options{}
	q, err := opts.quoteCurrency()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if q != exchanges.QuoteCurrencyUSDT {
		t.Errorf("default quote = %q, want %q (CEX default)", q, exchanges.QuoteCurrencyUSDT)
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
		"BTC",
	}
	for _, q := range unsupported {
		opts := Options{QuoteCurrency: q}
		_, err := opts.quoteCurrency()
		if err == nil {
			t.Errorf("expected error for unsupported %q, got nil", q)
		}
	}
}
