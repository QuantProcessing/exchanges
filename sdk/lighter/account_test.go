package lighter

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestClient_GetAccountActiveOrders(t *testing.T) {
	got, err := newLivePrivateClient(t).GetAccountActiveOrders(context.Background(), lighterMarketID(t))
	if err != nil {
		t.Fatalf("GetAccountActiveOrders: %v", err)
	}
	if got == nil {
		t.Fatal("expected active orders")
	}
}

func TestClient_GetNextNonce(t *testing.T) {
	nonce, err := newLivePrivateClient(t).GetNextNonce(context.Background())
	if err != nil {
		t.Fatalf("GetNextNonce: %v", err)
	}
	if nonce < 0 {
		t.Fatalf("unexpected nonce: %d", nonce)
	}
}

func TestClient_GetAccount(t *testing.T) {
	got, err := newLivePrivateClient(t).GetAccount(context.Background())
	if err != nil {
		t.Fatalf("GetAccount: %v", err)
	}
	if got == nil {
		t.Fatal("expected account")
	}
}

func TestClient_GetInactiveOrders(t *testing.T) {
	marketID := lighterMarketID(t)
	got, err := newLivePrivateClient(t).GetInactiveOrders(context.Background(), &marketID, 10)
	if err != nil {
		t.Fatalf("GetInactiveOrders: %v", err)
	}
	if got == nil {
		t.Fatal("expected inactive orders")
	}
}

func TestClient_GetAccountTxs(t *testing.T) {
	got, err := newLivePrivateClient(t).GetAccountTxs(context.Background(), 10)
	if err != nil {
		t.Fatalf("GetAccountTxs: %v", err)
	}
	if got == nil {
		t.Fatal("expected account txs")
	}
}

func TestClient_GetPnL(t *testing.T) {
	end := time.Now().UnixMilli()
	start := end - int64(24*time.Hour/time.Millisecond)
	got, err := newLivePrivateClient(t).GetPnL(context.Background(), start, end)
	if err != nil {
		t.Fatalf("GetPnL: %v", err)
	}
	if got == nil {
		t.Fatal("expected pnl")
	}
}

func TestClient_GetAccountLimits(t *testing.T) {
	got, err := newLivePrivateClient(t).GetAccountLimits(context.Background())
	if err != nil {
		t.Fatalf("GetAccountLimits: %v", err)
	}
	if got == nil {
		t.Fatal("expected account limits")
	}
}

func TestClient_GetAccountMetadata(t *testing.T) {
	got, err := newLivePrivateClient(t).GetAccountMetadata(context.Background())
	if err != nil {
		t.Fatalf("GetAccountMetadata: %v", err)
	}
	if got == nil {
		t.Fatal("expected account metadata")
	}
}

func TestClient_ChangeAccountTier(t *testing.T) {
	client := requireLighterLiveWrite(t, "LIGHTER_TEST_ACCOUNT_TIER")
	got, err := client.ChangeAccountTier(context.Background(), os.Getenv("LIGHTER_TEST_ACCOUNT_TIER"))
	if err != nil {
		t.Fatalf("ChangeAccountTier: %v", err)
	}
	if got == nil {
		t.Fatal("expected account tier response")
	}
}

func TestClient_GetPositionFunding(t *testing.T) {
	marketID := lighterMarketID(t)
	got, err := newLivePrivateClient(t).GetPositionFunding(context.Background(), &marketID, 10, nil)
	if err != nil {
		t.Fatalf("GetPositionFunding: %v", err)
	}
	if got == nil {
		t.Fatal("expected position funding")
	}
}

func TestClient_GetApiKeys(t *testing.T) {
	got, err := newLivePrivateClient(t).GetApiKeys(context.Background())
	if err != nil {
		t.Fatalf("GetApiKeys: %v", err)
	}
	if got == nil {
		t.Fatal("expected api keys")
	}
}

func TestClient_GetReferralPoints(t *testing.T) {
	got, err := newLivePrivateClient(t).GetReferralPoints(context.Background())
	if err != nil {
		t.Fatalf("GetReferralPoints: %v", err)
	}
	if got == nil {
		t.Fatal("expected referral points")
	}
}

func TestClient_GetAccountsByL1Address(t *testing.T) {
	address := os.Getenv("LIGHTER_TEST_L1_ADDRESS")
	if address == "" {
		t.Skip("LIGHTER_TEST_L1_ADDRESS is required for GetAccountsByL1Address live test")
	}
	got, err := newLiveClient().GetAccountsByL1Address(context.Background(), address)
	if err != nil {
		t.Fatalf("GetAccountsByL1Address: %v", err)
	}
	if got == nil {
		t.Fatal("expected accounts by L1 address")
	}
}

func TestClient_UpdateLeverage(t *testing.T) {
	client := requireLighterLiveWrite(t)
	got, err := client.UpdateLeverage(
		context.Background(),
		lighterMarketID(t),
		uint16(lighterIntEnv(t, "LIGHTER_TEST_LEVERAGE", 2)),
		uint8(lighterIntEnv(t, "LIGHTER_TEST_MARGIN_MODE", 0)),
	)
	if err != nil {
		t.Fatalf("UpdateLeverage: %v", err)
	}
	if got == nil {
		t.Fatal("expected update leverage response")
	}
}
