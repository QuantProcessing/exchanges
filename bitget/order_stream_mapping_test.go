package bitget

import (
	"testing"

	sdkbitget "github.com/QuantProcessing/exchanges/bitget/sdk"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestMapClassicMixOrderStream_UsesOverviewFieldsOnly(t *testing.T) {
	t.Parallel()

	got := mapClassicMixOrderStream("BTC", sdkbitget.ClassicMixOrderRecord{
		OrderID:    "mix-order",
		ClientOID:  "mix-client",
		Size:       "2",
		BaseVolume: "1.25",
		Price:      "50000",
		PriceAvg:   "49900",
		Status:     "partial-fill",
		Side:       "buy",
		OrderType:  "limit",
		Force:      "gtc",
		ReduceOnly: "yes",
		CTime:      "1710000000000",
		UTime:      "1710000001000",
	})

	require.Equal(t, "mix-order", got.OrderID)
	require.Equal(t, "mix-client", got.ClientOrderID)
	require.Equal(t, decimal.RequireFromString("2"), got.Quantity)
	require.Equal(t, decimal.RequireFromString("1.25"), got.FilledQuantity)
	require.Equal(t, decimal.RequireFromString("50000"), got.OrderPrice)
	require.Equal(t, decimal.RequireFromString("50000"), got.Price)
	require.True(t, got.AverageFillPrice.IsZero())
	require.True(t, got.LastFillPrice.IsZero())
	require.True(t, got.LastFillQuantity.IsZero())
	require.True(t, got.Fee.IsZero())
}

func TestMapClassicSpotOrderStream_UsesOverviewFieldsOnly(t *testing.T) {
	t.Parallel()

	got := mapClassicSpotOrderStream("BTC", sdkbitget.ClassicSpotOrderRecord{
		OrderID:       "spot-order",
		ClientOID:     "spot-client",
		Size:          "3",
		AccBaseVolume: "1.5",
		Price:         "51000",
		FillPrice:     "50900",
		PriceAvg:      "50800",
		Status:        "partial-fill",
		Side:          "buy",
		OrderType:     "limit",
		Force:         "gtc",
		CTime:         "1710000002000",
		UTime:         "1710000003000",
	})

	require.Equal(t, "spot-order", got.OrderID)
	require.Equal(t, "spot-client", got.ClientOrderID)
	require.Equal(t, decimal.RequireFromString("3"), got.Quantity)
	require.Equal(t, decimal.RequireFromString("1.5"), got.FilledQuantity)
	require.Equal(t, decimal.RequireFromString("51000"), got.OrderPrice)
	require.Equal(t, decimal.RequireFromString("51000"), got.Price)
	require.True(t, got.AverageFillPrice.IsZero())
	require.True(t, got.LastFillPrice.IsZero())
	require.True(t, got.LastFillQuantity.IsZero())
	require.True(t, got.Fee.IsZero())
}
