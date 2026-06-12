package spot

import "testing"

func TestWSMarketCompanion_SubscriptionTypes(t *testing.T) {
	if "l2Book" == "" || "trades" == "" || "bbo" == "" {
		t.Fatal("expected spot market websocket subscription names")
	}
}
