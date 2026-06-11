package perp

import (
	"encoding/json"
	"testing"
)

func TestWSTypeCompanion_DepthEventDecode(t *testing.T) {
	var event WsDepthEvent
	if err := json.Unmarshal([]byte(`{"e":"depthUpdate","s":"BTCUSDT"}`), &event); err != nil {
		t.Fatalf("decode depth event: %v", err)
	}
	if event.EventType != "depthUpdate" || event.Symbol != "BTCUSDT" {
		t.Fatalf("unexpected event: %+v", event)
	}
}
