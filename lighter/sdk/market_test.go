package lighter

import (
	"context"
	"os"
	"strconv"
	"testing"

	"github.com/joho/godotenv"
)

func GetEnv() (string, int64, uint64) {
	godotenv.Load("../../../.env")
	privateKey := os.Getenv("EXCHANGES_LIGHTER_PRIVATE_KEY")
	accountIndex, _ := strconv.ParseInt(os.Getenv("EXCHANGES_LIGHTER_ACCOUNT_INDEX"), 10, 64)
	keyIndex, _ := strconv.ParseUint(os.Getenv("EXCHANGES_LIGHTER_KEY_INDEX"), 10, 8)
	return privateKey, accountIndex, keyIndex
}

func TestGetOrderBookDetails(t *testing.T) {
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
