package sdk

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClientPlaceOrderDelegatesToExistingOrderExecutionPath(t *testing.T) {
	t.Parallel()

	var gotMethod string
	var gotPath string
	var gotInstruction string
	client := NewClient().WithCredentials("api-key", testSeedBase64())
	client.baseURL = "https://example.test"
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			gotMethod = r.Method
			gotPath = r.URL.Path
			gotInstruction = r.Header.Get("X-API-Key")
			return &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(
					`{"id":"123","symbol":"BTC_USDC","side":"Bid","orderType":"Limit","quantity":"1","price":"100","status":"New","clientId":7}`,
				)),
				Header: make(http.Header),
			}, nil
		}),
	}

	order, err := client.PlaceOrder(context.Background(), CreateOrderRequest{
		Symbol:    "BTC_USDC",
		Side:      "Bid",
		OrderType: "Limit",
		Quantity:  "1",
		Price:     "100",
		ClientID:  7,
	})
	require.NoError(t, err)
	require.Equal(t, http.MethodPost, gotMethod)
	require.Equal(t, "/api/v1/order", gotPath)
	require.Equal(t, "api-key", gotInstruction)
	require.Equal(t, "123", order.ID)
}

func testSeedBase64() string {
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = byte(i + 1)
	}
	return base64.StdEncoding.EncodeToString(seed)
}
