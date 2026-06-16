package examples

import (
	"time"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/shopspring/decimal"
)

type OrderFactoryWalkthrough struct {
	Market       model.SubmitOrder
	PostOnly     model.SubmitOrder
	StopMarket   model.SubmitOrder
	TrailingStop model.SubmitOrder
	Bracket      model.OrderList
}

// BuildOrdersWithOrderFactory shows how strategy code should create commands:
// use OrderFactory for account identity, client order IDs, list IDs, and shared
// command metadata instead of assembling those fields by hand.
func BuildOrdersWithOrderFactory() OrderFactoryWalkthrough {
	instrumentID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	factory := model.NewOrderFactory(
		"paper-main",
		model.WithClientOrderIDPrefix("intro"),
		model.WithOrderMetadata(model.CommandMetadata{
			TraderID:      "trader-001",
			StrategyID:    "order-factory-walkthrough",
			CorrelationID: "research-run-2026-06-16",
			TsInit:        time.Date(2026, 6, 16, 9, 0, 0, 0, time.UTC),
			Params: map[string]string{
				"example": "02_build_orders_with_order_factory",
			},
		}),
	)

	return OrderFactoryWalkthrough{
		Market: factory.Market(
			instrumentID,
			model.OrderSideBuy,
			decimal.RequireFromString("0.01"),
		),
		PostOnly: factory.Limit(
			instrumentID,
			model.OrderSideBuy,
			decimal.RequireFromString("0.01"),
			decimal.RequireFromString("100.00"),
			model.WithPostOnly(),
		),
		StopMarket: factory.StopMarket(
			instrumentID,
			model.OrderSideSell,
			decimal.RequireFromString("0.01"),
			decimal.RequireFromString("95.00"),
			model.WithReduceOnly(),
		),
		TrailingStop: factory.TrailingStopMarket(
			instrumentID,
			model.OrderSideSell,
			decimal.RequireFromString("0.01"),
			decimal.RequireFromString("1.50"),
			model.WithTrailingOffsetType(model.TrailingOffsetTypePrice),
			model.WithReduceOnly(),
		),
		Bracket: factory.Bracket(model.BracketOrderRequest{
			InstrumentID: instrumentID,
			Side:         model.OrderSideBuy,
			Quantity:     decimal.RequireFromString("0.01"),
			EntryPrice:   decimal.RequireFromString("100.00"),
			TakeProfit:   decimal.RequireFromString("104.00"),
			StopLoss:     decimal.RequireFromString("98.00"),
		}),
	}
}
