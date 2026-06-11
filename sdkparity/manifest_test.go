package sdkparity

import (
	"strings"
	"testing"
)

func TestParseMarkdownTable(t *testing.T) {
	input := strings.NewReader(`
| Exchange | Product | Method | Path | Status | Local Symbol |
| --- | --- | --- | --- | --- | --- |
| BINANCE | spot | GET | /api/v3/depth | implemented-sdk | binance/sdk/spot.Client.Depth |
| BINANCE | spot | POST | /api/v3/order/oco | intentionally-unsupported |  |
`)

	rows, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0].Exchange != "BINANCE" || rows[0].Path != "/api/v3/depth" {
		t.Fatalf("unexpected first row: %#v", rows[0])
	}
	if rows[1].Status != StatusIntentionallyUnsupported {
		t.Fatalf("unexpected second status: %q", rows[1].Status)
	}
}

func TestParseRejectsImplementedWithoutLocalSymbol(t *testing.T) {
	input := strings.NewReader(`
| Exchange | Product | Method | Path | Status | Local Symbol |
| --- | --- | --- | --- | --- | --- |
| OKX | swap | POST | /api/v5/trade/order | implemented-sdk |  |
`)

	_, err := Parse(input)
	if err == nil {
		t.Fatal("expected validation error")
	}
}
