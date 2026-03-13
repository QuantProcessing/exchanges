package common

import (
	"context"
	"testing"
)

func TestNonceManager_Fetch(t *testing.T) {
	nm, err := NewNonceManager("https://mainnet.zkelliot.ai", 2854, 2)
	if err != nil {
		t.Error(err)
	}
	nonce, err := nm.Fetch(context.Background())
	if err != nil {
		t.Error(err)
	}
	t.Logf("Nonce: %d", nonce)
}
