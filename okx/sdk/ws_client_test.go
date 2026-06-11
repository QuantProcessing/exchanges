package okx

import (
	"context"
	"testing"
)

func TestWSClientCompanion_NewWSClient(t *testing.T) {
	client := NewWSClient(context.Background())
	if client.URL != WSBaseURL || client.Subs == nil || client.PendingReqs == nil {
		t.Fatalf("unexpected ws client: %+v", client)
	}
}
