package sdk

import (
	"context"
	"testing"
)

func TestClient_GetInstruments(t *testing.T) {
	got, err := newLiveClient().GetInstruments(context.Background(), bitgetSpotCategory, "")
	if err != nil {
		t.Fatalf("GetInstruments: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected spot instruments")
	}
}

func TestClient_GetTicker(t *testing.T) {
	got, err := newLiveClient().GetTicker(context.Background(), bitgetPerpCategory, bitgetPerpSymbol)
	if err != nil {
		t.Fatalf("GetTicker: %v", err)
	}
	if got.Symbol != bitgetPerpSymbol {
		t.Fatalf("unexpected ticker symbol: %s", got.Symbol)
	}
}

func TestClient_GetOrderBook(t *testing.T) {
	got, err := newLiveClient().GetOrderBook(context.Background(), bitgetSpotCategory, bitgetSpotSymbol, 5)
	if err != nil {
		t.Fatalf("GetOrderBook: %v", err)
	}
	if len(got.Asks) == 0 || len(got.Bids) == 0 {
		t.Fatalf("expected non-empty order book: %+v", got)
	}
}

func TestClient_GetRecentFills(t *testing.T) {
	got, err := newLiveClient().GetRecentFills(context.Background(), bitgetSpotCategory, bitgetSpotSymbol, 10)
	if err != nil {
		t.Fatalf("GetRecentFills: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected recent fills")
	}
}

func TestClient_GetOpenInterest(t *testing.T) {
	got, err := newLiveClient().GetOpenInterest(context.Background(), bitgetPerpSymbol, bitgetPerpCategory)
	if err != nil {
		t.Fatalf("GetOpenInterest: %v", err)
	}
	if len(got.List) == 0 || got.List[0].Symbol == "" || got.TS == "" {
		t.Fatalf("expected open interest symbol: %+v", got)
	}
}

func TestClient_GetHistoryFundRate(t *testing.T) {
	got, err := newLiveClient().GetHistoryFundRate(context.Background(), bitgetPerpSymbol, bitgetPerpCategory, 2, 1)
	if err != nil {
		t.Fatalf("GetHistoryFundRate: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected funding history")
	}
}

func TestClient_GetCandles(t *testing.T) {
	got, err := newLiveClient().GetCandles(context.Background(), bitgetPerpCategory, bitgetPerpSymbol, "1m", "market", 0, 0, 2)
	if err != nil {
		t.Fatalf("GetCandles: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected candles")
	}
}
