package spot

import (
	"context"
	"testing"
)

const binanceSpotTestSymbol = "BTCUSDT"

func TestClient_Depth(t *testing.T) {
	got, err := newLiveClient().Depth(context.Background(), binanceSpotTestSymbol, 5)
	if err != nil {
		t.Fatalf("Depth: %v", err)
	}
	if got.LastUpdateID == 0 || len(got.Bids) == 0 || len(got.Asks) == 0 {
		t.Fatalf("unexpected depth response: %+v", got)
	}
}

func TestClient_Klines(t *testing.T) {
	got, err := newLiveClient().Klines(context.Background(), binanceSpotTestSymbol, "1m", 1, 0, 0)
	if err != nil {
		t.Fatalf("Klines: %v", err)
	}
	if len(got) != 1 || len(got[0]) < 6 {
		t.Fatalf("unexpected klines response: %+v", got)
	}
}

func TestClient_Ticker(t *testing.T) {
	got, err := newLiveClient().Ticker(context.Background(), binanceSpotTestSymbol)
	if err != nil {
		t.Fatalf("Ticker: %v", err)
	}
	if got.Symbol != binanceSpotTestSymbol || got.LastPrice == "" {
		t.Fatalf("unexpected ticker response: %+v", got)
	}
}

func TestClient_TickerRequiresSymbol(t *testing.T) {
	if _, err := newLiveClient().Ticker(context.Background(), ""); err == nil {
		t.Fatal("expected missing symbol error")
	}
}

func TestClient_BookTicker(t *testing.T) {
	got, err := newLiveClient().BookTicker(context.Background(), binanceSpotTestSymbol)
	if err != nil {
		t.Fatalf("BookTicker: %v", err)
	}
	if got.Symbol != binanceSpotTestSymbol || got.BidPrice == "" || got.AskPrice == "" {
		t.Fatalf("unexpected book ticker response: %+v", got)
	}
}

func TestClient_BookTickerRequiresSymbol(t *testing.T) {
	if _, err := newLiveClient().BookTicker(context.Background(), ""); err == nil {
		t.Fatal("expected missing symbol error")
	}
}

func TestClient_ExchangeInfo(t *testing.T) {
	got, err := newLiveClient().ExchangeInfo(context.Background())
	if err != nil {
		t.Fatalf("ExchangeInfo: %v", err)
	}
	if len(got.Symbols) == 0 {
		t.Fatalf("unexpected exchange info response: %+v", got)
	}
}
