package sdk

import (
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
)

func TestApplySDKRequestOptsString(t *testing.T) {
	params := map[string]string{"symbol": "BTCUSDT"}
	applySDKRequestOptsString(params, exchanges.SDKRequestOpts{})
	if _, ok := params["recvWindow"]; ok {
		t.Fatal("empty opts should not set recvWindow")
	}

	applySDKRequestOptsString(params, exchanges.SDKRequestOpts{
		RecvWindowMillis: 2500,
		ClientRequestID:  "client-1",
	})
	if params["recvWindow"] != "2500" {
		t.Fatalf("unexpected recvWindow: %#v", params["recvWindow"])
	}
	if params["clientOid"] != "client-1" {
		t.Fatalf("unexpected client id: %#v", params["clientOid"])
	}
}
