package sdk

import (
	"context"
	"testing"

	"github.com/QuantProcessing/exchanges/internal/testenv"
)

func TestClient_GetWalletBalance(t *testing.T) {
	got, err := newLivePrivateClient(t).GetWalletBalance(context.Background(), "UNIFIED", "")
	if err != nil {
		testenv.SkipIfTransientLiveNetworkError(t, err, "Bybit wallet balance endpoint")
		t.Fatalf("GetWalletBalance: %v", err)
	}
	if len(got.List) == 0 {
		t.Fatal("expected at least one wallet balance record")
	}
}

func TestClient_GetAccountInfo(t *testing.T) {
	got, err := newLivePrivateClient(t).GetAccountInfo(context.Background())
	if err != nil {
		testenv.SkipIfTransientLiveNetworkError(t, err, "Bybit account info endpoint")
		t.Fatalf("GetAccountInfo: %v", err)
	}
	if got.UnifiedMarginStatus == 0 {
		t.Fatal("expected unified margin status")
	}
}

func TestClient_GetFeeRates(t *testing.T) {
	got, err := newLivePrivateClient(t).GetFeeRates(context.Background(), "linear", bybitLinearSymbol)
	if err != nil {
		testenv.SkipIfTransientLiveNetworkError(t, err, "Bybit fee rates endpoint")
		t.Fatalf("GetFeeRates: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected fee rate records")
	}
}

func TestClient_GetPositions(t *testing.T) {
	got, err := newLivePrivateClient(t).GetPositions(context.Background(), "linear", "", "USDT")
	if err != nil {
		testenv.SkipIfTransientLiveNetworkError(t, err, "Bybit positions endpoint")
		t.Fatalf("GetPositions: %v", err)
	}
	if got == nil {
		t.Fatal("expected positions slice")
	}
}

func TestClient_SetLeverage(t *testing.T) {
	client := requireBybitLiveWrite(t)
	symbol := bybitEnvOrDefault("BYBIT_TEST_SYMBOL", bybitLinearSymbol)
	leverage := bybitEnvOrDefault("BYBIT_TEST_LEVERAGE", "2")

	err := client.SetLeverage(context.Background(), SetLeverageRequest{
		Category:     "linear",
		Symbol:       symbol,
		BuyLeverage:  leverage,
		SellLeverage: leverage,
	})
	if err != nil {
		t.Fatalf("SetLeverage: %v", err)
	}
}
