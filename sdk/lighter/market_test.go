package lighter

import (
	"context"
	"os"
	"testing"
	"time"
)

func lighterMarketID(t *testing.T) int {
	t.Helper()
	return lighterIntEnv(t, "LIGHTER_TEST_MARKET_ID", lighterTestMarketID)
}

func TestClient_GetAssetDetails(t *testing.T) {
	got, err := newLiveClient().GetAssetDetails(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetAssetDetails: %v", err)
	}
	if got == nil {
		t.Fatal("expected asset details")
	}
}

func TestClient_GetOrderBookDetails(t *testing.T) {
	marketID := lighterMarketID(t)
	got, err := newLiveClient().GetOrderBookDetails(context.Background(), &marketID, nil)
	if err != nil {
		t.Fatalf("GetOrderBookDetails: %v", err)
	}
	if got == nil {
		t.Fatal("expected order book details")
	}
}

func TestClient_GetOrderBooks(t *testing.T) {
	marketID := lighterMarketID(t)
	got, err := newLiveClient().GetOrderBooks(context.Background(), &marketID)
	if err != nil {
		t.Fatalf("GetOrderBooks: %v", err)
	}
	if got == nil {
		t.Fatal("expected order books")
	}
}

func TestClient_GetRecentTrades(t *testing.T) {
	got, err := newLiveClient().GetRecentTrades(context.Background(), lighterMarketID(t), 10)
	if err != nil {
		t.Fatalf("GetRecentTrades: %v", err)
	}
	if got == nil {
		t.Fatal("expected recent trades")
	}
}

func TestClient_GetOrderBookOrders(t *testing.T) {
	got, err := newLiveClient().GetOrderBookOrders(context.Background(), lighterMarketID(t), 10)
	if err != nil {
		t.Fatalf("GetOrderBookOrders: %v", err)
	}
	if got == nil {
		t.Fatal("expected order book orders")
	}
}

func TestClient_GetFundingRates(t *testing.T) {
	got, err := newLiveClient().GetFundingRates(context.Background())
	if err != nil {
		t.Fatalf("GetFundingRates: %v", err)
	}
	if got == nil {
		t.Fatal("expected funding rates")
	}
}

func TestClient_GetFundingRate(t *testing.T) {
	got, err := newLiveClient().GetFundingRate(context.Background(), lighterMarketID(t))
	if err != nil {
		t.Fatalf("GetFundingRate: %v", err)
	}
	if got == nil {
		t.Fatal("expected funding rate")
	}
}

func TestClient_GetAllFundingRates(t *testing.T) {
	got, err := newLiveClient().GetAllFundingRates(context.Background())
	if err != nil {
		t.Fatalf("GetAllFundingRates: %v", err)
	}
	if got == nil {
		t.Fatal("expected funding rates slice")
	}
}

func TestClient_GetExchangeStats(t *testing.T) {
	got, err := newLiveClient().GetExchangeStats(context.Background())
	if err != nil {
		t.Fatalf("GetExchangeStats: %v", err)
	}
	if got == nil {
		t.Fatal("expected exchange stats")
	}
}

func TestClient_GetCandlesticks(t *testing.T) {
	end := time.Now().UnixMilli()
	start := end - int64(time.Hour/time.Millisecond)
	got, err := newLiveClient().GetCandlesticks(context.Background(), lighterMarketID(t), "1m", start, end, 10)
	if err != nil {
		t.Fatalf("GetCandlesticks: %v", err)
	}
	if got == nil || len(got.Candlesticks) == 0 {
		t.Fatal("expected candlesticks")
	}
}

func TestClient_GetFundingHistory(t *testing.T) {
	marketID := lighterMarketID(t)
	end := time.Now().UnixMilli()
	start := end - int64(24*time.Hour/time.Millisecond)
	got, err := newLiveClient().GetFundingHistory(context.Background(), marketID, "1h", start, end, 10)
	if err != nil {
		t.Fatalf("GetFundingHistory: %v", err)
	}
	if got == nil || len(got.Fundings) == 0 {
		t.Fatal("expected funding history")
	}
}

func TestClient_GetTransferFeeInfo(t *testing.T) {
	got, err := newLivePrivateClient(t).GetTransferFeeInfo(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetTransferFeeInfo: %v", err)
	}
	if got == nil {
		t.Fatal("expected transfer fee info")
	}
}

func TestClient_GetWithdrawalDelay(t *testing.T) {
	got, err := newLiveClient().GetWithdrawalDelay(context.Background())
	if err != nil {
		t.Fatalf("GetWithdrawalDelay: %v", err)
	}
	if got == nil || got.Seconds <= 0 {
		t.Fatal("expected withdrawal delay")
	}
}

func TestClient_GetAnnouncements(t *testing.T) {
	got, err := newLiveClient().GetAnnouncements(context.Background())
	if err != nil {
		t.Fatalf("GetAnnouncements: %v", err)
	}
	if got == nil {
		t.Fatal("expected announcements")
	}
}

func TestClient_GetL1Metadata(t *testing.T) {
	client := newLivePrivateClient(t)
	l1Address := os.Getenv("LIGHTER_TEST_L1_ADDRESS")
	if l1Address == "" {
		t.Skip("LIGHTER_TEST_L1_ADDRESS is required for GetL1Metadata live test")
	}
	got, err := client.GetL1Metadata(context.Background(), l1Address)
	if err != nil {
		t.Fatalf("GetL1Metadata: %v", err)
	}
	if got == nil || got.L1Address == "" {
		t.Fatal("expected l1 metadata")
	}
}

func TestClient_GetPublicPoolsMetadata(t *testing.T) {
	got, err := newLiveClient().GetPublicPoolsMetadata(context.Background(), "all", 0, 10, nil)
	if err != nil {
		t.Fatalf("GetPublicPoolsMetadata: %v", err)
	}
	if got == nil {
		t.Fatal("expected public pools metadata")
	}
}
