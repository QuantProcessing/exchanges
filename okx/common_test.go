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
		{"perp USDT", "BTC-USDT-SWAP", "USDT", "BTC/USDT"},
		{"perp USDC", "BTC-USDC-SWAP", "USDC", "BTC/USDC"},
		{"spot USDT", "ETH-USDT", "USDT", "ETH/USDT"},
		{"spot USDC", "ETH-USDC", "USDC", "ETH/USDC"},
		{"explicit quote overrides default", "BTC-USDC-SWAP", "USDT", "BTC/USDC"},
		{"bare symbol", "BTC", "USDT", "BTC/USDT"},
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

func TestFormatAndExtractDocumentQuoteAwareInterpretation(t *testing.T) {
	usdtPerp := FormatSymbolWithQuote("BTC", "USDT", "SWAP")
	usdcPerp := FormatSymbolWithQuote("BTC", "USDC", "SWAP")

	if usdtPerp != "BTC-USDT-SWAP" {
		t.Fatalf("USDT perp formatter = %q, want BTC-USDT-SWAP", usdtPerp)
	}
	if usdcPerp != "BTC-USDC-SWAP" {
		t.Fatalf("USDC perp formatter = %q, want BTC-USDC-SWAP", usdcPerp)
	}

	if got := ExtractSymbolWithQuote("BTC-USDC-SWAP", "USDT"); got != "BTC/USDC" {
		t.Fatalf("extractor should preserve explicit BTC-USDC-SWAP quote, got %q", got)
	}
	if got := ExtractSymbolWithQuote("BTC-USDT-SWAP", "USDC"); got != "BTC/USDT" {
		t.Fatalf("extractor should preserve explicit BTC-USDT-SWAP quote, got %q", got)
	}
}

func TestFormatSymbol_DefaultQuote(t *testing.T) {
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

func TestExtractSymbol_DefaultQuote(t *testing.T) {
	tests := []struct {
		symbol string
		want   string
	}{
		{"BTC-USDT-SWAP", "BTC/USDT"},
		{"ETH-USDT", "ETH/USDT"},
		{"BTC", "BTC/USDT"},
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
			want := sym + "/" + quote
			if extracted != want {
				t.Errorf("Roundtrip perp failed: %q → %q → %q, want %q", sym, formatted, extracted, want)
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
			want := sym + "/" + quote
			if extracted != want {
				t.Errorf("Roundtrip spot failed: %q → %q → %q, want %q", sym, formatted, extracted, want)
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
