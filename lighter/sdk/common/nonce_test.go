package common

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestNonceManager_Fetch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping external nonce service test under -short")
	}
	nm, err := NewNonceManager("https://mainnet.zklighter.elliot.ai", 2854, 2)
	if err != nil {
		t.Error(err)
	}
	var nonce int64
	for attempt := 0; attempt < 3; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		nonce, err = nm.Fetch(ctx)
		cancel()
		if err == nil {
			break
		}
		if attempt == 2 || !strings.Contains(err.Error(), "EOF") {
			t.Fatal(err)
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Logf("Nonce: %d", nonce)
}
