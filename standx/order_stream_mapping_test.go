package standx

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	sdkstandx "github.com/QuantProcessing/exchanges/standx/sdk"
)

func TestMapSDKOrderToAdapterOrderStream_UsesOverviewFieldsOnly(t *testing.T) {
	t.Parallel()

	adp := &Adapter{}
	got := adp.mapSDKOrderToAdapterOrderStream(sdkstandx.Order{
		ID:           11,
		ClOrdID:      "client-11",
		Symbol:       "BTC-DUSD-PERP",
		Side:         "buy",
		OrderType:    "limit",
		Qty:          "2",
		FillQty:      "1.5",
		Price:        "50000",
		FillAvgPrice: "49900",
		Status:       sdkstandx.OrderStatusFilled,
		TimeInForce:  "gtc",
		ReduceOnly:   true,
	})

	require.Equal(t, "11", got.OrderID)
	require.Equal(t, "client-11", got.ClientOrderID)
	require.Equal(t, decimal.RequireFromString("2"), got.Quantity)
	require.Equal(t, decimal.RequireFromString("1.5"), got.FilledQuantity)
	require.Equal(t, decimal.RequireFromString("50000"), got.OrderPrice)
	require.Equal(t, decimal.RequireFromString("50000"), got.Price)
	require.True(t, got.AverageFillPrice.IsZero())
	require.True(t, got.LastFillPrice.IsZero())
	require.True(t, got.LastFillQuantity.IsZero())
}
