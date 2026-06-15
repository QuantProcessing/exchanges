package backtest

import (
	"context"
	"testing"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/strategy"
	"github.com/shopspring/decimal"
)

func BenchmarkRunnerMarketOrder(b *testing.B) {
	events := []Event{tickerEvent(decimal.RequireFromString("100"))}
	for i := 0; i < b.N; i++ {
		runner := NewRunner(Config{
			Events: events,
			Strategies: []strategy.Strategy{&submittingStrategy{
				id: "benchmark-submitter",
			}},
		})
		if _, err := runner.Run(context.Background()); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMatchingCoreOrderBookDepth(b *testing.B) {
	inst := matchingCoreInstrument()
	core := NewMatchingCore(MatchingCoreConfig{Instrument: inst, FillModel: DefaultFillModel()})
	order := matchingCoreOrder(model.OrderTypeMarket, model.OrderSideBuy, decimal.RequireFromString("1"), decimal.Zero)
	book := model.OrderBook{
		InstrumentID: inst.ID,
		Asks: []model.OrderBookLevel{
			{Price: decimal.RequireFromString("100"), Size: decimal.RequireFromString("0.4")},
			{Price: decimal.RequireFromString("101"), Size: decimal.RequireFromString("0.8")},
		},
		Timestamp: time.Unix(10, 0),
	}
	consumed := map[string]decimal.Decimal{"101": decimal.RequireFromString("0.2")}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matches := core.MatchOrderBook(OrderBookMatchRequest{
			Order:    order,
			Book:     book,
			Consumed: consumed,
		})
		if len(matches) != 2 {
			b.Fatalf("expected two matches, got %d", len(matches))
		}
	}
}
