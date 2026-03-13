//go:build edgex

package perp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/joho/godotenv"
)

func GetEnv() (string, string) {
	godotenv.Load("../../../.env")
	return os.Getenv("EDGEX_STARK_PRIVATE_KEY"), os.Getenv("EDGEX_ACCOUNT_ID")
}

func TestGetAccountAsset(t *testing.T) {
	starkPrivateKey, accountID := GetEnv()
	client := NewClient().WithCredentials(starkPrivateKey, accountID)
	res, err := client.GetAccountAsset(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	jsonData, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("Account Asset:", string(jsonData))
}

func TestGetOpenOrders(t *testing.T) {
	starkPrivateKey, accountID := GetEnv()
	client := NewClient().WithCredentials(starkPrivateKey, accountID)
	res, err := client.GetOpenOrders(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	jsonData, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("Open Orders:", string(jsonData))
}
