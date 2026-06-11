package okx

import (
	"context"
	"testing"
)

func TestClient_GetTickers(t *testing.T) {
	got, err := newLiveClient().GetTickers(context.Background(), "SPOT", nil)
	if err != nil {
		t.Fatalf("GetTickers: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected tickers")
	}
}

func TestClient_GetTicker(t *testing.T) {
	got, err := newLiveClient().GetTicker(context.Background(), okxSpotInstID)
	if err != nil {
		t.Fatalf("GetTicker: %v", err)
	}
	if len(got) == 0 || got[0].InstId != okxSpotInstID {
		t.Fatalf("unexpected ticker response: %+v", got)
	}
}

func TestClient_GetOrderBook(t *testing.T) {
	size := 5
	got, err := newLiveClient().GetOrderBook(context.Background(), okxSpotInstID, &size)
	if err != nil {
		t.Fatalf("GetOrderBook: %v", err)
	}
	if len(got) == 0 || len(got[0].Asks) == 0 || len(got[0].Bids) == 0 {
		t.Fatalf("unexpected order book response: %+v", got)
	}
}

func TestClient_GetInstruments(t *testing.T) {
	got, err := newLiveClient().GetInstruments(context.Background(), "SPOT")
	if err != nil {
		t.Fatalf("GetInstruments: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected instruments")
	}
}

func TestClient_GetInstrumentsByFamily(t *testing.T) {
	got, err := newLiveClient().GetInstrumentsByFamily(context.Background(), "SWAP", "BTC-USDT")
	if err != nil {
		t.Fatalf("GetInstrumentsByFamily: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected instruments by family")
	}
}

func TestClient_GetCandles(t *testing.T) {
	bar := "1m"
	limit := 1
	got, err := newLiveClient().GetCandles(context.Background(), okxSpotInstID, &bar, nil, nil, &limit)
	if err != nil {
		t.Fatalf("GetCandles: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected candles")
	}
}

func TestClient_GetTrades(t *testing.T) {
	limit := 1
	got, err := newLiveClient().GetTrades(context.Background(), okxSpotInstID, &limit)
	if err != nil {
		t.Fatalf("GetTrades: %v", err)
	}
	if len(got) == 0 || got[0].InstId != okxSpotInstID {
		t.Fatalf("unexpected trades response: %+v", got)
	}
}

func TestClient_GetFundingRate(t *testing.T) {
	got, err := newLiveClient().GetFundingRate(context.Background(), okxSwapInstID)
	if err != nil {
		t.Fatalf("GetFundingRate: %v", err)
	}
	if got.Symbol != okxSwapInstID || got.FundingRate == "" {
		t.Fatalf("unexpected funding rate response: %+v", got)
	}
}

func TestClient_GetAllFundingRates(t *testing.T) {
	got, err := newLiveClient().GetAllFundingRates(context.Background())
	if err != nil {
		t.Fatalf("GetAllFundingRates: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil all funding rates")
	}
}

func TestClient_GetOpenInterest(t *testing.T) {
	got, err := newLiveClient().GetOpenInterest(context.Background(), okxSwapInstID)
	if err != nil {
		t.Fatalf("GetOpenInterest: %v", err)
	}
	if got.InstId != okxSwapInstID || got.OI == "" {
		t.Fatalf("unexpected open interest response: %+v", got)
	}
}

func TestClient_GetFundingRateHistory(t *testing.T) {
	got, err := newLiveClient().GetFundingRateHistory(context.Background(), okxSwapInstID, 0, 0, 1)
	if err != nil {
		t.Fatalf("GetFundingRateHistory: %v", err)
	}
	if len(got) == 0 || got[0].InstId != okxSwapInstID {
		t.Fatalf("unexpected funding history response: %+v", got)
	}
}

func TestClient_GetHistoryTrades(t *testing.T) {
	got, err := newLiveClient().GetHistoryTrades(context.Background(), okxSpotInstID, 1, "", "", 1)
	if err != nil {
		t.Fatalf("GetHistoryTrades: %v", err)
	}
	if len(got) == 0 || got[0].InstId != okxSpotInstID {
		t.Fatalf("unexpected history trades response: %+v", got)
	}
}
