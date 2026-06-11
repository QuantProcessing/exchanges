package perp

import (
	"context"
	"testing"
)

func TestWSAccountCompanion_NewWsAccountClient(t *testing.T) {
	client := NewWsAccountClient(context.Background(), "api-key", "secret")
	if client.Client == nil || client.WsClient == nil {
		t.Fatalf("unexpected account client: %+v", client)
	}
}
