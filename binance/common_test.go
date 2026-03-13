package binance

import (
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
)

// ============================================================================
// FormatSymbolWithQuote / ExtractSymbolWithQuote
// ============================================================================

func TestFormatSymbolWithQuote(t *testing.T) {
	tests := []struct {
		name   string
		symbol string
		quote  string
		want   string
	}{
		{"BTC+USDT", "BTC", "USDT", "btcusdt"},
		{"ETH+USDT", "ETH", "USDT", "ethusdt"},
		{"BTC+USDC", "BTC", "USDC", "btcusdc"},
		{"ETH+USDC", "eth", "USDC", "ethusdc"},
		{"already has suffix USDT", "btcusdt", "USDT", "btcusdt"},
		{"already has suffix USDC", "BTCUSDC", "USDC", "btcusdc"},
		{"case insensitive input", "Btc", "USDT", "btcusdt"},
		{"DOGE+USDT", "DOGE", "USDT", "dogeusdt"},
		{"DOGE+USDC", "DOGE", "USDC", "dogeusdc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatSymbolWithQuote(tt.symbol, tt.quote)
			if got != tt.want {
				t.Errorf("FormatSymbolWithQuote(%q, %q) = %q, want %q", tt.symbol, tt.quote, got, tt.want)
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
		{"BTCUSDT→BTC", "BTCUSDT", "USDT", "BTC"},
		{"ETHUSDT→ETH", "ETHUSDT", "USDT", "ETH"},
		{"BTCUSDC→BTC", "BTCUSDC", "USDC", "BTC"},
		{"ETHUSDC→ETH", "ETHUSDC", "USDC", "ETH"},
		{"lowercase btcusdt", "btcusdt", "USDT", "BTC"},
		{"no suffix match", "BTCUSDC", "USDT", "BTCUSDC"},
		{"bare symbol", "BTC", "USDT", "BTC"},
		{"DOGEUSDC→DOGE", "DOGEUSDC", "USDC", "DOGE"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractSymbolWithQuote(tt.symbol, tt.quote)
			if got != tt.want {
				t.Errorf("ExtractSymbolWithQuote(%q, %q) = %q, want %q", tt.symbol, tt.quote, got, tt.want)
			}
		})
	}
}

// Backward compatibility: original FormatSymbol/ExtractSymbol still work as USDT
func TestFormatSymbol_BackwardCompat(t *testing.T) {
	tests := []struct {
		symbol string
		want   string
	}{
		{"BTC", "btcusdt"},
		{"ETH", "ethusdt"},
		{"btcusdt", "btcusdt"},
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
		{"BTCUSDT", "BTC"},
		{"ethusdt", "ETH"},
		{"BTC", "BTC"},
	}
	for _, tt := range tests {
		got := ExtractSymbol(tt.symbol)
		if got != tt.want {
			t.Errorf("ExtractSymbol(%q) = %q, want %q", tt.symbol, got, tt.want)
		}
	}
}

// FormatSymbol and ExtractSymbol are inverse operations
func TestFormatExtract_Roundtrip(t *testing.T) {
	quotes := []string{"USDT", "USDC"}
	symbols := []string{"BTC", "ETH", "DOGE", "SOL"}

	for _, quote := range quotes {
		for _, sym := range symbols {
			formatted := FormatSymbolWithQuote(sym, quote)
			extracted := ExtractSymbolWithQuote(formatted, quote)
			if extracted != sym {
				t.Errorf("Roundtrip failed: %q → format(%q) → %q → extract → %q, want %q",
					sym, quote, formatted, extracted, sym)
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

func TestOptions_QuoteCurrency_USDC(t *testing.T) {
	opts := Options{QuoteCurrency: exchanges.QuoteCurrencyUSDC}
	q, err := opts.quoteCurrency()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if q != exchanges.QuoteCurrencyUSDC {
		t.Errorf("quote = %q, want %q", q, exchanges.QuoteCurrencyUSDC)
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
			t.Errorf("expected error for unsupported quote %q, got nil", q)
		}
	}
}
