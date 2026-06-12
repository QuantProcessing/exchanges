package okx

import (
	"context"
	"testing"
)

func TestWSClientConstructorCompatibility(t *testing.T) {
	ctx := context.Background()

	modern := NewWSClient(ctx)
	legacy := NewWsClient(ctx)

	var modernFromLegacy *WSClient = legacy
	var legacyFromModern *WsClient = modern

	if modernFromLegacy != legacy {
		t.Fatalf("legacy constructor returned incompatible type")
	}
	if legacyFromModern != modern {
		t.Fatalf("modern constructor returned incompatible alias type")
	}
	if modern.URL != legacy.URL {
		t.Fatalf("constructors should initialize the same URL: got %q and %q", modern.URL, legacy.URL)
	}
	if modern.Subs == nil || legacy.PendingReqs == nil {
		t.Fatalf("constructors should initialize websocket client maps")
	}
}

func TestWSClient_SubscribeTicker(t *testing.T) {
	client := newLivePublicOKXWSClient(t)

	if err := client.SubscribeTicker(okxSpotInstID, func(*Ticker) {}); err != nil {
		t.Fatalf("SubscribeTicker: %v", err)
	}
	if client.Subs[WsSubscribeArgs{Channel: "tickers", InstId: okxSpotInstID}] == nil {
		t.Fatalf("expected ticker subscription to be registered")
	}
}

func TestWSClient_SubscribeOrderBook(t *testing.T) {
	client := newLivePublicOKXWSClient(t)

	err := client.SubscribeOrderBook(okxSpotInstID, func(*OrderBook, string) {})
	if err != nil {
		t.Fatalf("SubscribeOrderBook failed: %v", err)
	}
	if client.Subs[WsSubscribeArgs{Channel: "books", InstId: okxSpotInstID}] == nil {
		t.Fatalf("expected order book subscription to be registered")
	}
}
