package backpack

import (
	"strconv"
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/backpack/sdk"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestToCreateOrderRequestLimit(t *testing.T) {
	market := sdk.Market{Symbol: "BTC_USDC_PERP"}
	params := &exchanges.OrderParams{
		Symbol:      "BTC",
		Side:        exchanges.OrderSideBuy,
		Type:        exchanges.OrderTypeLimit,
		Quantity:    decimal.RequireFromString("0.5"),
		Price:       decimal.RequireFromString("50000"),
		TimeInForce: exchanges.TimeInForceIOC,
		ReduceOnly:  true,
		ClientID:    "42",
	}

	got, err := toCreateOrderRequest(market, params)
	require.NoError(t, err)
	require.Equal(t, "BTC_USDC_PERP", got.Symbol)
	require.Equal(t, "Bid", got.Side)
	require.Equal(t, "Limit", got.OrderType)
	require.Equal(t, "0.5", got.Quantity)
	require.Equal(t, "50000", got.Price)
	require.Equal(t, "IOC", got.TimeInForce)
	require.EqualValues(t, 42, got.ClientID)
	require.True(t, got.ReduceOnly)
}

func TestToCancelOrderRequest(t *testing.T) {
	got := toCancelOrderRequest("99", "BTC_USDC")
	require.Equal(t, "99", got.OrderID)
	require.Equal(t, "BTC_USDC", got.Symbol)
}

func TestToCreateOrderRequestGeneratesBackpackSafeClientID(t *testing.T) {
	market := sdk.Market{Symbol: "BTC_USDC_PERP"}
	params := &exchanges.OrderParams{
		Symbol:   "BTC",
		Side:     exchanges.OrderSideBuy,
		Type:     exchanges.OrderTypeMarket,
		Quantity: decimal.RequireFromString("0.5"),
	}

	got, err := toCreateOrderRequest(market, params)
	require.NoError(t, err)
	require.NotEmpty(t, params.ClientID)
	require.NotZero(t, got.ClientID)
}

func TestToCreateOrderRequestRejectsUnencodableClientID(t *testing.T) {
	market := sdk.Market{Symbol: "BTC_USDC_PERP"}
	params := &exchanges.OrderParams{
		Symbol:   "BTC",
		Side:     exchanges.OrderSideBuy,
		Type:     exchanges.OrderTypeMarket,
		Quantity: decimal.RequireFromString("0.5"),
		ClientID: "9999999999999",
	}

	_, err := toCreateOrderRequest(market, params)
	require.Error(t, err)
}

func TestGenerateClientIDReturnsBackpackSafeNumericString(t *testing.T) {
	got := GenerateClientID()
	require.NotEmpty(t, got)
	require.NotEqual(t, "0", got)

	parsed, err := strconv.ParseUint(got, 10, 32)
	require.NoError(t, err)
	require.NotZero(t, parsed)
}
