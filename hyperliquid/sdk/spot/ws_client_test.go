package spot

import (
	"context"
	"testing"

	hyperliquid "github.com/QuantProcessing/exchanges/hyperliquid/sdk"
)

func TestWSClientCompanion_NewWebsocketClient(t *testing.T) {
	base := hyperliquid.NewWebsocketClient(context.Background())
	client := NewWebsocketClient(base)
	if client.WebsocketClient != base {
		t.Fatal("expected base websocket client to be embedded")
	}
}
