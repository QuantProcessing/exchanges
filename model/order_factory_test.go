package model

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestOrderFactoryCreatesLimitOrderWithDefaultsAndGeneratedClientID(t *testing.T) {
	factory := NewOrderFactory("acct", WithClientOrderIDPrefix("demo"))

	order := factory.Limit(
		MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
		OrderSideBuy,
		decimal.RequireFromString("0.01"),
		decimal.RequireFromString("101"),
	)

	require.NoError(t, order.Validate())
	require.Equal(t, AccountID("acct"), order.AccountID)
	require.Equal(t, ClientOrderID("demo-1"), order.ClientOrderID)
	require.Equal(t, OrderTypeLimit, order.Type)
	require.Equal(t, TimeInForceGTC, order.TimeInForce)
	require.Equal(t, "0.01", order.Quantity.String())
	require.Equal(t, "101", order.Price.String())
}

func TestOrderFactoryCreatesMarketOrderWithExplicitOptions(t *testing.T) {
	factory := NewOrderFactory("acct")

	order := factory.Market(
		MustInstrumentID("ETH-USDT-PERP.BINANCE"),
		OrderSideSell,
		decimal.RequireFromString("2"),
		WithClientOrderID("manual-1"),
		WithTimeInForce(TimeInForceIOC),
		WithReduceOnly(),
	)

	require.NoError(t, order.Validate())
	require.Equal(t, ClientOrderID("manual-1"), order.ClientOrderID)
	require.Equal(t, OrderTypeMarket, order.Type)
	require.Equal(t, TimeInForceIOC, order.TimeInForce)
	require.True(t, order.ReduceOnly)
	require.True(t, order.ExpireTime.IsZero())
}

func TestOrderFactoryCreatesGTDLimitOrderWithExpireTime(t *testing.T) {
	expire := time.Unix(100, 0)
	factory := NewOrderFactory("acct")

	order := factory.Limit(
		MustInstrumentID("ETH-USDT-PERP.BINANCE"),
		OrderSideSell,
		decimal.RequireFromString("2"),
		decimal.RequireFromString("1000"),
		WithTimeInForce(TimeInForceGTD),
		WithExpireTime(expire),
	)

	require.NoError(t, order.Validate())
	require.Equal(t, TimeInForceGTD, order.TimeInForce)
	require.Equal(t, expire, order.ExpireTime)
}

func TestOrderFactoryAppliesAdvancedLimitOptions(t *testing.T) {
	factory := NewOrderFactory("acct")

	order := factory.Limit(
		MustInstrumentID("ETH-USDT-PERP.BINANCE"),
		OrderSideBuy,
		decimal.RequireFromString("2"),
		decimal.RequireFromString("1000"),
		WithPostOnly(),
		WithTriggerPrice(decimal.RequireFromString("999")),
		WithTrailingOffset(decimal.RequireFromString("5")),
	)

	require.True(t, order.PostOnly)
	require.Equal(t, "999", order.TriggerPrice.String())
	require.Equal(t, "5", order.TrailingOffset.String())
}

func TestOrderFactoryCreatesAdvancedNautilusOrderTypes(t *testing.T) {
	factory := NewOrderFactory("acct", WithClientOrderIDPrefix("adv"))
	instID := MustInstrumentID("ETH-USDT-PERP.BINANCE")

	orders := []SubmitOrder{
		factory.MarketToLimit(instID, OrderSideBuy, decimal.RequireFromString("1")),
		factory.StopMarket(instID, OrderSideSell, decimal.RequireFromString("1"), decimal.RequireFromString("990")),
		factory.StopLimit(instID, OrderSideSell, decimal.RequireFromString("1"), decimal.RequireFromString("980"), decimal.RequireFromString("990")),
		factory.MarketIfTouched(instID, OrderSideBuy, decimal.RequireFromString("1"), decimal.RequireFromString("970")),
		factory.LimitIfTouched(instID, OrderSideBuy, decimal.RequireFromString("1"), decimal.RequireFromString("975"), decimal.RequireFromString("970")),
		factory.TrailingStopMarket(instID, OrderSideSell, decimal.RequireFromString("1"), decimal.RequireFromString("10"), WithActivationPrice(decimal.RequireFromString("1000"))),
		factory.TrailingStopLimit(instID, OrderSideSell, decimal.RequireFromString("1"), decimal.RequireFromString("985"), decimal.RequireFromString("10"), WithActivationPrice(decimal.RequireFromString("1000"))),
	}

	require.Equal(t, OrderTypeMarketToLimit, orders[0].Type)
	require.Equal(t, OrderTypeStopMarket, orders[1].Type)
	require.Equal(t, OrderTypeStopLimit, orders[2].Type)
	require.Equal(t, OrderTypeMarketIfTouched, orders[3].Type)
	require.Equal(t, OrderTypeLimitIfTouched, orders[4].Type)
	require.Equal(t, OrderTypeTrailingStopMarket, orders[5].Type)
	require.Equal(t, OrderTypeTrailingStopLimit, orders[6].Type)
	for _, order := range orders {
		require.NoError(t, order.Validate(), order.Type)
	}
	require.Equal(t, ClientOrderID("adv-7"), orders[6].ClientOrderID)
}

func TestOrderFactoryCreatesBracketOrderList(t *testing.T) {
	factory := NewOrderFactory("acct", WithClientOrderIDPrefix("bracket"))
	instID := MustInstrumentID("ETH-USDT-PERP.BINANCE")

	list := factory.Bracket(BracketOrderRequest{
		InstrumentID: instID,
		Side:         OrderSideBuy,
		Quantity:     decimal.RequireFromString("2"),
		EntryPrice:   decimal.RequireFromString("1000"),
		TakeProfit:   decimal.RequireFromString("1100"),
		StopLoss:     decimal.RequireFromString("950"),
	})

	require.NoError(t, list.Validate())
	require.Equal(t, OrderListID("bracket-list-1"), list.ID)
	require.Len(t, list.Orders, 3)

	entry := list.Orders[0]
	stopLoss := list.Orders[1]
	takeProfit := list.Orders[2]

	require.Equal(t, OrderTypeLimit, entry.Type)
	require.Equal(t, ContingencyTypeOTO, entry.Contingency)
	require.Equal(t, OrderListID("bracket-list-1"), entry.OrderListID)

	require.Equal(t, entry.ClientOrderID, stopLoss.ParentClientOrderID)
	require.Equal(t, OrderTypeStopMarket, stopLoss.Type)
	require.Equal(t, ContingencyTypeOCO, stopLoss.Contingency)
	require.True(t, stopLoss.ReduceOnly)
	require.Equal(t, "950", stopLoss.TriggerPrice.String())

	require.Equal(t, entry.ClientOrderID, takeProfit.ParentClientOrderID)
	require.Equal(t, OrderTypeLimit, takeProfit.Type)
	require.Equal(t, ContingencyTypeOCO, takeProfit.Contingency)
	require.True(t, takeProfit.ReduceOnly)
	require.Equal(t, "1100", takeProfit.Price.String())
}
