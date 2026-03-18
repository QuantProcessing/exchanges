
package grvt

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/joho/godotenv"
)

func GetEnv() (string, string, string) {
	godotenv.Load("../../../.env")
	apiKey := os.Getenv("GRVT_API_KEY")
	subaccount := os.Getenv("GRVT_SUB_ACCOUNT_ID")
	privateKey := os.Getenv("GRVT_PRIVATE_KEY")
	return apiKey, subaccount, privateKey
}

func TestGetFundingAccountSummary(t *testing.T) {
	apiKey, subAccountID, privateKey := GetEnv()
	client := NewClient().WithCredentials(apiKey, subAccountID, privateKey)
	fundingAccountSummary, err := client.GetFundingAccountSummary(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("%+v\n", fundingAccountSummary)
}
