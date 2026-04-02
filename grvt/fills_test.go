package grvt

import (
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	sdkgrvt "github.com/QuantProcessing/exchanges/grvt/sdk"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestMapGrvtFillReturnsExecutionDetails(t *testing.T) {
	adp := &Adapter{quoteCurrency: "USDT"}

	got := adp.mapGrvtFill(&sdkgrvt.WsFill{
		EventTime:     "1700000000000000",
		Instrument:    "BTC_USDT_Perp",
		IsBuyer:       true,
		IsTaker:       false,
		Size:          "0.5",
		Price:         "101.25",
		Fee:           "0.01",
		TradeID:       "trade-1",
		OrderID:       "order-1",
		ClientOrderID: "client-1",
	})

	require.Equal(t, "trade-1", got.TradeID)
	require.Equal(t, "order-1", got.OrderID)
	require.Equal(t, "client-1", got.ClientOrderID)
	require.Equal(t, "BTC", got.Symbol)
	require.Equal(t, exchanges.OrderSideBuy, got.Side)
	require.True(t, got.Price.Equal(decimal.RequireFromString("101.25")))
	require.True(t, got.Quantity.Equal(decimal.RequireFromString("0.5")))
	require.True(t, got.Fee.Equal(decimal.RequireFromString("0.01")))
	require.True(t, got.IsMaker)
	require.EqualValues(t, 1700000000, got.Timestamp)
}
