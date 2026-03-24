package lighter

import (
	"context"
	"os"
	"strconv"
	"testing"

	"github.com/QuantProcessing/exchanges/internal/testenv"
)

func GetEnv() (string, int64, uint64) {
	privateKey := os.Getenv("LIGHTER_PRIVATE_KEY")
	accountIndex, _ := strconv.ParseInt(os.Getenv("LIGHTER_ACCOUNT_INDEX"), 10, 64)
	keyIndex, _ := strconv.ParseUint(os.Getenv("LIGHTER_KEY_INDEX"), 10, 8)
	return privateKey, accountIndex, keyIndex
}

func requireFullEnv(t *testing.T) {
	t.Helper()
	testenv.RequireFull(t, "LIGHTER_PRIVATE_KEY", "LIGHTER_ACCOUNT_INDEX", "LIGHTER_KEY_INDEX")
}

func TestGetOrderBookDetails(t *testing.T) {
	requireFullEnv(t)
	privateKey, accountIndex, keyIndex := GetEnv()
	client := NewClient().WithCredentials(privateKey, accountIndex, uint8(keyIndex))
	res, err := client.GetOrderBookDetails(context.Background(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Code != 200 {
		t.Fatalf("expected 200, got %d", res.Code)
	}
	t.Log(res.OrderBookDetails)
}
