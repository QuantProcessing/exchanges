package sdk

import (
	"context"
	"testing"
)

func TestClient_GetInstruments(t *testing.T) {
	got, err := newLiveClient().GetInstruments(context.Background(), "linear")
	if err != nil {
		t.Fatalf("GetInstruments: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected at least one instrument")
	}
}

func TestClient_GetInstrumentsForBase(t *testing.T) {
	got, err := newLiveClient().GetInstrumentsForBase(context.Background(), "linear", "BTC")
	if err != nil {
		t.Fatalf("GetInstrumentsForBase: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected BTC linear instruments")
	}
}

func TestClient_GetTicker(t *testing.T) {
	got, err := newLiveClient().GetTicker(context.Background(), "spot", bybitSpotSymbol)
	if err != nil {
		t.Fatalf("GetTicker: %v", err)
	}
	if got.Symbol != bybitSpotSymbol {
		t.Fatalf("unexpected ticker symbol: %s", got.Symbol)
	}
}

func TestClient_GetOrderBook(t *testing.T) {
	got, err := newLiveClient().GetOrderBook(context.Background(), "linear", bybitLinearSymbol, 5)
	if err != nil {
		t.Fatalf("GetOrderBook: %v", err)
	}
	if len(got.Asks) == 0 || len(got.Bids) == 0 {
		t.Fatalf("expected non-empty order book: %+v", got)
	}
}

func TestClient_GetRecentTrades(t *testing.T) {
	got, err := newLiveClient().GetRecentTrades(context.Background(), "spot", bybitSpotSymbol, 10)
	if err != nil {
		t.Fatalf("GetRecentTrades: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected recent trades")
	}
}

func TestClient_GetKlines(t *testing.T) {
	got, err := newLiveClient().GetKlines(context.Background(), "linear", bybitLinearSymbol, "60", 0, 0, 10)
	if err != nil {
		t.Fatalf("GetKlines: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected klines")
	}
}

func TestClient_GetOpenInterest(t *testing.T) {
	got, err := newLiveClient().GetOpenInterest(context.Background(), "linear", bybitLinearSymbol, "5min", 0, 0, 50, "")
	if err != nil {
		t.Fatalf("GetOpenInterest: %v", err)
	}
	if len(got.List) == 0 {
		t.Fatal("expected open interest history")
	}
}

func TestClient_GetFundingHistory(t *testing.T) {
	got, err := newLiveClient().GetFundingHistory(context.Background(), "linear", bybitLinearSymbol, 0, 0, 2)
	if err != nil {
		t.Fatalf("GetFundingHistory: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected funding history")
	}
}
