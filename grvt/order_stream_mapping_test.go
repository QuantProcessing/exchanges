package grvt

import (
	"testing"

	sdkgrvt "github.com/QuantProcessing/exchanges/grvt/sdk"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestMapGrvtOrderStream_UsesOverviewFieldsOnly(t *testing.T) {
	t.Parallel()

	adp := &Adapter{quoteCurrency: "USDT"}

	got := adp.mapGrvtOrderStream(&sdkgrvt.Order{
		OrderID: "order-1",
		Legs: []sdkgrvt.OrderLeg{{
			Instrument:    "BTC_USDT_Perp",
			IsBuyintAsset: true,
			Size:          "2",
			LimitPrice:    "50000",
		}},
		Metadata: sdkgrvt.OrderMetadata{
			ClientOrderID: "client-1",
			CreatedTime:   "1710000000000000",
		},
		State: sdkgrvt.OrderState{
			Status:       sdkgrvt.OrderStatusFilled,
			TradedSize:   []string{"1.25"},
			AvgFillPrice: []string{"49900"},
		},
	})

	require.Equal(t, "order-1", got.OrderID)
	require.Equal(t, "client-1", got.ClientOrderID)
	require.Equal(t, decimal.RequireFromString("2"), got.Quantity)
	require.Equal(t, decimal.RequireFromString("1.25"), got.FilledQuantity)
	require.Equal(t, decimal.RequireFromString("50000"), got.OrderPrice)
	require.Equal(t, decimal.RequireFromString("50000"), got.Price)
	require.True(t, got.AverageFillPrice.IsZero())
	require.True(t, got.LastFillPrice.IsZero())
	require.True(t, got.LastFillQuantity.IsZero())
}
