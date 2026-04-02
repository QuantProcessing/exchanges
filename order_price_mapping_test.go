package exchanges_test

import (
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestExplicitOrderPriceFieldsCanCoexistWithLegacyPrice(t *testing.T) {
	order := exchanges.Order{
		Price:            decimal.RequireFromString("100"),
		OrderPrice:       decimal.RequireFromString("100"),
		AverageFillPrice: decimal.RequireFromString("101"),
		LastFillPrice:    decimal.RequireFromString("102"),
	}

	require.Equal(t, order.Price, order.OrderPrice)
	require.Equal(t, "101", order.AverageFillPrice.String())
	require.Equal(t, "102", order.LastFillPrice.String())
}
