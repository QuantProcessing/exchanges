package bitget

import (
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/bitget/sdk"
)

func TestUTAPrivateArgUsesOfficialTopicShape(t *testing.T) {
	got := utaPrivateArg("order")
	if got.InstType != "UTA" || got.Topic != "order" {
		t.Fatalf("unexpected UTA private arg: %+v", got)
	}
	if got.Channel != "" || got.InstID != "" || got.Symbol != "" {
		t.Fatalf("UTA private arg should not use classic channel fields: %+v", got)
	}
}

func TestMapUTAFill(t *testing.T) {
	got := mapUTAFill("BTC/USDT", sdk.FillRecord{
		OrderID:    "order-1",
		ClientOID:  "client-1",
		ExecID:     "fill-1",
		Symbol:     "BTCUSDT",
		Side:       "buy",
		ExecPrice:  "100.5",
		ExecQty:    "0.2",
		TradeScope: "maker",
		ExecTime:   "1700000000000",
		FeeDetail: []sdk.FeeDetail{{
			FeeCoin: "USDT",
			Fee:     "0.01",
		}},
	})
	if got.TradeID != "fill-1" || got.OrderID != "order-1" || got.ClientOrderID != "client-1" {
		t.Fatalf("unexpected fill identifiers: %+v", got)
	}
	if got.Symbol != "BTC/USDT" || got.Side != exchanges.OrderSideBuy || !got.IsMaker {
		t.Fatalf("unexpected fill market fields: %+v", got)
	}
	if !got.Price.Equal(parseDecimal("100.5")) || !got.Quantity.Equal(parseDecimal("0.2")) || !got.Fee.Equal(parseDecimal("0.01")) {
		t.Fatalf("unexpected fill decimals: %+v", got)
	}
	if got.FeeAsset != "USDT" || got.Timestamp != 1700000000000 {
		t.Fatalf("unexpected fee/timestamp fields: %+v", got)
	}
}
