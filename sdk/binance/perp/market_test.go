package perp

import (
	"context"
	"testing"
)

func TestClient_Depth(t *testing.T) {
	got, err := newLiveClient().Depth(context.Background(), binancePerpTestSymbol, 5)
	if err != nil {
		t.Fatalf("Depth: %v", err)
	}
	if got.LastUpdateID == 0 || len(got.Bids) == 0 || len(got.Asks) == 0 {
		t.Fatalf("unexpected depth response: %+v", got)
	}
}

func TestClient_Klines(t *testing.T) {
	got, err := newLiveClient().Klines(context.Background(), binancePerpTestSymbol, "1m", 1, 0, 0)
	if err != nil {
		t.Fatalf("Klines: %v", err)
	}
	if len(got) != 1 || len(got[0]) < 6 {
		t.Fatalf("unexpected klines response: %+v", got)
	}
}

func TestClient_ContinousKlines(t *testing.T) {
	got, err := newLiveClient().ContinousKlines(context.Background(), binancePerpTestSymbol, "PERPETUAL", "1m", 1, 0, 0)
	if err != nil {
		t.Fatalf("ContinousKlines: %v", err)
	}
	if len(got) != 1 || len(got[0]) < 6 {
		t.Fatalf("unexpected continuous klines response: %+v", got)
	}
}

func TestClient_Ticker(t *testing.T) {
	got, err := newLiveClient().Ticker(context.Background(), binancePerpTestSymbol)
	if err != nil {
		t.Fatalf("Ticker: %v", err)
	}
	if got.Symbol != binancePerpTestSymbol || got.LastPrice == "" {
		t.Fatalf("unexpected ticker response: %+v", got)
	}
}

func TestClient_TickerRequiresSymbol(t *testing.T) {
	if _, err := newLiveClient().Ticker(context.Background(), ""); err == nil {
		t.Fatal("expected missing symbol error")
	}
}

func TestClient_MarkPrice(t *testing.T) {
	got, err := newLiveClient().MarkPrice(context.Background(), binancePerpTestSymbol)
	if err != nil {
		t.Fatalf("MarkPrice: %v", err)
	}
	if got.Symbol != binancePerpTestSymbol || got.MarkPrice == "" {
		t.Fatalf("unexpected mark price response: %+v", got)
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

func TestClient_GetAggTrades(t *testing.T) {
	got, err := newLiveClient().GetAggTrades(context.Background(), binancePerpTestSymbol, 1)
	if err != nil {
		t.Fatalf("GetAggTrades: %v", err)
	}
	if len(got) == 0 || got[0].Price == "" {
		t.Fatalf("unexpected aggregate trades response: %+v", got)
	}
}

func TestClient_GetAggTradesPaged(t *testing.T) {
	got, err := newLiveClient().GetAggTradesPaged(context.Background(), AggTradesQuery{Symbol: binancePerpTestSymbol, Limit: 1})
	if err != nil {
		t.Fatalf("GetAggTradesPaged: %v", err)
	}
	if len(got) == 0 || got[0].Price == "" {
		t.Fatalf("unexpected paged aggregate trades response: %+v", got)
	}
}

func TestClient_GetFundingInfo(t *testing.T) {
	got, err := newLiveClient().GetFundingInfo(context.Background())
	if err != nil {
		t.Fatalf("GetFundingInfo: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil funding info slice")
	}
}

func TestClient_GetFundingIntervalHours(t *testing.T) {
	got, err := newLiveClient().GetFundingIntervalHours(context.Background(), binancePerpTestSymbol)
	if err != nil {
		t.Fatalf("GetFundingIntervalHours: %v", err)
	}
	if got <= 0 {
		t.Fatalf("unexpected funding interval: %d", got)
	}
}

func TestClient_GetFundingRate(t *testing.T) {
	got, err := newLiveClient().GetFundingRate(context.Background(), binancePerpTestSymbol)
	if err != nil {
		t.Fatalf("GetFundingRate: %v", err)
	}
	if got.Symbol != binancePerpTestSymbol || got.LastFundingRate == "" {
		t.Fatalf("unexpected funding rate response: %+v", got)
	}
}

func TestClient_GetAllFundingRates(t *testing.T) {
	got, err := newLiveClient().GetAllFundingRates(context.Background())
	if err != nil {
		t.Fatalf("GetAllFundingRates: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected funding rates")
	}
}

func TestClient_GetOpenInterest(t *testing.T) {
	got, err := newLiveClient().GetOpenInterest(context.Background(), binancePerpTestSymbol)
	if err != nil {
		t.Fatalf("GetOpenInterest: %v", err)
	}
	if got.Symbol != binancePerpTestSymbol || got.OpenInterest == "" {
		t.Fatalf("unexpected open interest response: %+v", got)
	}
}

func TestClient_GetFundingRateHistory(t *testing.T) {
	got, err := newLiveClient().GetFundingRateHistory(context.Background(), binancePerpTestSymbol, 0, 0, 1)
	if err != nil {
		t.Fatalf("GetFundingRateHistory: %v", err)
	}
	if len(got) == 0 || got[0].Symbol != binancePerpTestSymbol {
		t.Fatalf("unexpected funding history response: %+v", got)
	}
}

func TestConvertToHourlyRate(t *testing.T) {
	got, err := convertToHourlyRate("0.0008", 8)
	if err != nil {
		t.Fatalf("convertToHourlyRate: %v", err)
	}
	if got != "0.0001000000" {
		t.Fatalf("unexpected hourly rate: %s", got)
	}
	if _, err := convertToHourlyRate("0.0008", 0); err == nil {
		t.Fatal("expected interval validation error")
	}
}
