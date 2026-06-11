package okx

import (
	"context"
	"strconv"
	"testing"
)

func TestClient_GetAccountBalance(t *testing.T) {
	got, err := newLivePrivateClient(t).GetAccountBalance(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetAccountBalance: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil balance slice")
	}
}

func TestClient_GetPositions(t *testing.T) {
	instType := "SWAP"
	got, err := newLivePrivateClient(t).GetPositions(context.Background(), &instType, nil)
	if err != nil {
		t.Fatalf("GetPositions: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil positions slice")
	}
}

func TestClient_GetAccountConfig(t *testing.T) {
	got, err := newLivePrivateClient(t).GetAccountConfig(context.Background())
	if err != nil {
		t.Fatalf("GetAccountConfig: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected account config")
	}
}

func TestClient_SetPositionMode(t *testing.T) {
	got, err := requireOKXLiveWrite(t).SetPositionMode(context.Background(), okxEnvOrDefault("OKX_TEST_POSITION_MODE", "net_mode"))
	if err != nil {
		t.Fatalf("SetPositionMode: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil position mode response")
	}
}

func TestClient_SetLeverage(t *testing.T) {
	leverage, err := strconv.Atoi(okxEnvOrDefault("OKX_TEST_LEVERAGE", "1"))
	if err != nil {
		t.Fatalf("parse OKX_TEST_LEVERAGE: %v", err)
	}
	got, err := requireOKXLiveWrite(t).SetLeverage(context.Background(), SetLeverage{
		InstId:  okxEnvOrDefault("OKX_TEST_LEVERAGE_INST_ID", okxSwapInstID),
		Lever:   leverage,
		MgnMode: okxEnvOrDefault("OKX_TEST_MARGIN_MODE", "cross"),
	})
	if err != nil {
		t.Fatalf("SetLeverage: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil leverage response")
	}
}

func TestClient_GetTradeFee(t *testing.T) {
	got, err := newLivePrivateClient(t).GetTradeFee(context.Background(), "SPOT", nil)
	if err != nil {
		t.Fatalf("GetTradeFee: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil fee response")
	}
}
