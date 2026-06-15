package backtest

import (
	"testing"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestMatchingCoreWalksOrderBookDepthWithoutMutatingBook(t *testing.T) {
	inst := matchingCoreInstrument()
	order := matchingCoreOrder(model.OrderTypeMarket, model.OrderSideBuy, decimal.RequireFromString("1"), decimal.Zero)
	book := model.OrderBook{
		InstrumentID: inst.ID,
		Asks: []model.OrderBookLevel{
			{Price: decimal.RequireFromString("100"), Size: decimal.RequireFromString("0.4")},
			{Price: decimal.RequireFromString("101"), Size: decimal.RequireFromString("0.8")},
		},
		Timestamp: time.Unix(10, 0),
	}
	core := NewMatchingCore(MatchingCoreConfig{Instrument: inst, FillModel: DefaultFillModel()})

	matches := core.MatchOrderBook(OrderBookMatchRequest{
		Order:    order,
		Book:     book,
		Consumed: map[string]decimal.Decimal{"101": decimal.RequireFromString("0.2")},
	})

	require.Len(t, matches, 2)
	require.Equal(t, FillSourceOrderBook, matches[0].Source)
	require.True(t, decimal.RequireFromString("100").Equal(matches[0].Price))
	require.True(t, decimal.RequireFromString("0.4").Equal(matches[0].Quantity))
	require.True(t, decimal.RequireFromString("101").Equal(matches[1].Price))
	require.True(t, decimal.RequireFromString("0.6").Equal(matches[1].Quantity))
	require.True(t, decimal.RequireFromString("0.8").Equal(book.Asks[1].Size))
}

func TestMatchingCoreUsesFillModelForLimitTouch(t *testing.T) {
	inst := matchingCoreInstrument()
	order := matchingCoreOrder(model.OrderTypeLimit, model.OrderSideBuy, decimal.RequireFromString("1"), decimal.RequireFromString("100"))
	book := model.OrderBook{
		InstrumentID: inst.ID,
		Asks: []model.OrderBookLevel{
			{Price: decimal.RequireFromString("100"), Size: decimal.RequireFromString("0.4")},
			{Price: decimal.RequireFromString("99"), Size: decimal.RequireFromString("0.6")},
		},
		Timestamp: time.Unix(10, 0),
	}
	core := NewMatchingCore(MatchingCoreConfig{Instrument: inst, FillModel: rejectLimitTouchFillModel{}})

	matches := core.MatchOrderBook(OrderBookMatchRequest{Order: order, Book: book})

	require.Len(t, matches, 1)
	require.True(t, decimal.RequireFromString("99").Equal(matches[0].Price))
	require.True(t, decimal.RequireFromString("0.6").Equal(matches[0].Quantity))
}

func matchingCoreInstrument() model.Instrument {
	return model.Instrument{
		ID:        model.MustInstrumentID("BTC-USDT-PERP.BINANCE"),
		RawSymbol: "BTCUSDT",
		Type:      model.InstrumentTypePerp,
		Base:      "BTC",
		Quote:     "USDT",
		Settle:    "USDT",
		PriceTick: decimal.RequireFromString("0.01"),
		SizeTick:  decimal.RequireFromString("0.001"),
		Status:    model.InstrumentStatusTrading,
	}
}

func matchingCoreOrder(orderType model.OrderType, side model.OrderSide, quantity decimal.Decimal, price decimal.Decimal) model.OrderStatusReport {
	return model.OrderStatusReport{
		AccountID:      "backtest",
		InstrumentID:   model.MustInstrumentID("BTC-USDT-PERP.BINANCE"),
		OrderID:        "order-1",
		ClientOrderID:  "client-1",
		Status:         model.OrderStatusAccepted,
		Side:           side,
		Type:           orderType,
		Quantity:       quantity,
		LeavesQuantity: quantity,
		Price:          price,
	}
}

type rejectLimitTouchFillModel struct{}

func (rejectLimitTouchFillModel) ShouldFillLimitTouch(ctx FillContext) bool {
	return !ctx.LimitTouch
}

func (rejectLimitTouchFillModel) ApplySlippage(_ FillContext, price decimal.Decimal) decimal.Decimal {
	return price
}
