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
		{"BTCUSDT→BTC/USDT", "BTCUSDT", "USDT", "BTC/USDT"},
		{"ETHUSDT→ETH/USDT", "ETHUSDT", "USDT", "ETH/USDT"},
		{"BTCUSDC→BTC/USDC", "BTCUSDC", "USDC", "BTC/USDC"},
		{"ETHUSDC→ETH/USDC", "ETHUSDC", "USDC", "ETH/USDC"},
		{"lowercase btcusdt", "btcusdt", "USDT", "BTC/USDT"},
		{"explicit quote overrides default", "BTCUSDC", "USDT", "BTC/USDC"},
		{"bare symbol", "BTC", "USDT", "BTC/USDT"},
		{"DOGEUSDC→DOGE/USDC", "DOGEUSDC", "USDC", "DOGE/USDC"},
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

func TestFormatAndExtractDocumentQuoteAwareInterpretation(t *testing.T) {
	usdtSymbol := FormatSymbolWithQuote("BTC", "USDT")
	usdcSymbol := FormatSymbolWithQuote("BTC", "USDC")

	if usdtSymbol != "btcusdt" {
		t.Fatalf("USDT formatter = %q, want btcusdt", usdtSymbol)
	}
	if usdcSymbol != "btcusdc" {
		t.Fatalf("USDC formatter = %q, want btcusdc", usdcSymbol)
	}

	if got := ExtractSymbolWithQuote("BTCUSDC", "USDT"); got != "BTC/USDC" {
		t.Fatalf("extractor should preserve explicit BTCUSDC quote, got %q", got)
	}
	if got := ExtractSymbolWithQuote("BTCUSDT", "USDC"); got != "BTC/USDT" {
		t.Fatalf("extractor should preserve explicit BTCUSDT quote, got %q", got)
	}
}

func TestFormatSymbol_DefaultQuote(t *testing.T) {
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

func TestExtractSymbol_DefaultQuote(t *testing.T) {
	tests := []struct {
		symbol string
		want   string
	}{
		{"BTCUSDT", "BTC/USDT"},
		{"ethusdt", "ETH/USDT"},
		{"BTC", "BTC/USDT"},
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
			want := sym + "/" + quote
			if extracted != want {
				t.Errorf("Roundtrip failed: %q → format(%q) → %q → extract → %q, want %q",
					sym, quote, formatted, extracted, want)
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
