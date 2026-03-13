package spot

import (
	"context"
	"testing"
	"time"
)

func TestWsClient_IsConnected(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := NewWsMarketClient(ctx)

	// Before connection
	if client.IsConnected() {
		t.Error("Client should not be connected before Connect() is called")
	}

	// After connection
	if err := client.Connect(); err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	if !client.IsConnected() {
		t.Error("Client should be connected after Connect() is called")
	}

	// After close
	client.Close()
	time.Sleep(100 * time.Millisecond) // Give it time to close

	if client.IsConnected() {
		t.Error("Client should not be connected after Close() is called")
	}
}
