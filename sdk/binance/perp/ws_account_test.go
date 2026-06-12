package perp

import (
	"context"
	"testing"
)

func TestWSAccountCompanion_NewWsAccountClient(t *testing.T) {
	client := NewWsAccountClient(context.Background(), "api-key", "secret")
	if client.Client == nil || client.WsClient == nil || client.BaseURL != WSBaseURL {
		t.Fatalf("unexpected account client: %+v", client)
	}
}

func TestWSAccountCompanion_WithURLSetsBaseURL(t *testing.T) {
	client := NewWsAccountClient(context.Background(), "api-key", "secret")
	client.WithURL("wss://example.test/private")
	if client.BaseURL != "wss://example.test/private" {
		t.Fatalf("unexpected base url: %s", client.BaseURL)
	}
}

func TestWSAccountCompanion_SetOnResubscribe(t *testing.T) {
	client := NewWsAccountClient(context.Background(), "api-key", "secret")
	called := false
	client.SetOnResubscribe(func() {
		called = true
	})
	client.onResubscribe()
	if !called {
		t.Fatal("expected on resubscribe hook to be stored")
	}
}
