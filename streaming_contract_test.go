package exchanges_test

import (
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestOrderSupportsExplicitOrderAndFillPrices(t *testing.T) {
	order := exchanges.Order{
		OrderID:          "1",
		Price:            decimal.RequireFromString("100"),
		OrderPrice:       decimal.RequireFromString("100"),
		AverageFillPrice: decimal.RequireFromString("101"),
		LastFillPrice:    decimal.RequireFromString("102"),
		LastFillQuantity: decimal.RequireFromString("0.5"),
	}

	require.Equal(t, "100", order.OrderPrice.String())
	require.Equal(t, "101", order.AverageFillPrice.String())
	require.Equal(t, "102", order.LastFillPrice.String())
	require.Equal(t, "0.5", order.LastFillQuantity.String())
}

func TestFillCarriesExecutionDetails(t *testing.T) {
	fill := exchanges.Fill{
		TradeID:       "t-1",
		OrderID:       "o-1",
		ClientOrderID: "c-1",
		Symbol:        "BTC",
		Side:          exchanges.OrderSideBuy,
		Price:         decimal.RequireFromString("101"),
		Quantity:      decimal.RequireFromString("0.25"),
		Fee:           decimal.RequireFromString("0.01"),
		FeeAsset:      "USDT",
		IsMaker:       true,
		Timestamp:     123,
	}

	require.Equal(t, "t-1", fill.TradeID)
	require.True(t, fill.IsMaker)
}
