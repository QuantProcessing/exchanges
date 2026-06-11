package hyperliquid

import (
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
)

func TestApplySDKRequestOpts(t *testing.T) {
	payload := map[string]any{"type": "order"}
	applySDKRequestOpts(payload, exchanges.SDKRequestOpts{})
	if _, ok := payload["expiresAfter"]; ok {
		t.Fatal("empty opts should not set expiresAfter")
	}

	applySDKRequestOpts(payload, exchanges.SDKRequestOpts{
		RecvWindowMillis: 2500,
		ClientRequestID:  "client-1",
	})
	if _, ok := payload["expiresAfter"].(int64); !ok {
		t.Fatalf("expected expiresAfter int64, got %#v", payload["expiresAfter"])
	}
	if payload["clientRequestId"] != "client-1" {
		t.Fatalf("unexpected client id: %#v", payload["clientRequestId"])
	}
}
