package margin

import (
	"context"
	"os"
	"strconv"
	"testing"

	"github.com/QuantProcessing/exchanges/internal/testenv"
)

func TestClient_GetAccount(t *testing.T) {
	got, err := newLivePrivateClient(t).GetAccount(context.Background())
	if err != nil {
		testenv.SkipIfTransientLiveNetworkError(t, err, "Binance margin account endpoint")
		t.Fatalf("GetAccount: %v", err)
	}
	if got == nil {
		t.Fatal("expected margin account")
	}
}

func TestClient_GetIsolatedAccount(t *testing.T) {
	got, err := newLivePrivateClient(t).GetIsolatedAccount(context.Background(), marginEnvOrDefault("BINANCE_MARGIN_TEST_SYMBOLS", "BTCUSDT"))
	if err != nil {
		testenv.SkipIfTransientLiveNetworkError(t, err, "Binance isolated margin account endpoint")
		t.Fatalf("GetIsolatedAccount: %v", err)
	}
	if got == nil {
		t.Fatal("expected isolated margin account")
	}
}

func TestClient_Borrow(t *testing.T) {
	client := requireBinanceMarginLiveWrite(t, "BINANCE_MARGIN_TEST_ASSET", "BINANCE_MARGIN_TEST_AMOUNT")
	amount, err := strconv.ParseFloat(os.Getenv("BINANCE_MARGIN_TEST_AMOUNT"), 64)
	if err != nil {
		t.Fatalf("parse BINANCE_MARGIN_TEST_AMOUNT: %v", err)
	}

	tranID, err := client.Borrow(
		context.Background(),
		os.Getenv("BINANCE_MARGIN_TEST_ASSET"),
		amount,
		os.Getenv("BINANCE_MARGIN_TEST_ISOLATED") == "1",
		os.Getenv("BINANCE_MARGIN_TEST_SYMBOL"),
	)
	if err != nil {
		t.Fatalf("Borrow: %v", err)
	}
	if tranID == 0 {
		t.Fatal("expected transaction id")
	}
}

func TestClient_Repay(t *testing.T) {
	client := requireBinanceMarginLiveWrite(t, "BINANCE_MARGIN_TEST_ASSET", "BINANCE_MARGIN_TEST_AMOUNT")
	amount, err := strconv.ParseFloat(os.Getenv("BINANCE_MARGIN_TEST_AMOUNT"), 64)
	if err != nil {
		t.Fatalf("parse BINANCE_MARGIN_TEST_AMOUNT: %v", err)
	}

	tranID, err := client.Repay(
		context.Background(),
		os.Getenv("BINANCE_MARGIN_TEST_ASSET"),
		amount,
		os.Getenv("BINANCE_MARGIN_TEST_ISOLATED") == "1",
		os.Getenv("BINANCE_MARGIN_TEST_SYMBOL"),
	)
	if err != nil {
		t.Fatalf("Repay: %v", err)
	}
	if tranID == 0 {
		t.Fatal("expected transaction id")
	}
}
