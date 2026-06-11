package okx

import (
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
)

func TestApplySDKRequestOpts(t *testing.T) {
	payload := map[string]any{"instId": "BTC-USDT-SWAP"}
	applySDKRequestOpts(payload, exchanges.SDKRequestOpts{})
	if _, ok := payload["recvWindow"]; ok {
		t.Fatal("empty opts should not set recvWindow")
	}

	applySDKRequestOpts(payload, exchanges.SDKRequestOpts{
		RecvWindowMillis: 2500,
		ClientRequestID:  "client-1",
	})
	if payload["recvWindow"] != int64(2500) {
		t.Fatalf("unexpected recvWindow: %#v", payload["recvWindow"])
	}
	if payload["clOrdId"] != "client-1" {
		t.Fatalf("unexpected client id: %#v", payload["clOrdId"])
	}
}
