package nado

import (
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	sdknado "github.com/QuantProcessing/exchanges/nado/sdk"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestMapCancelledOrderBuildsTerminalSnapshot(t *testing.T) {
	t.Parallel()

	adp := &Adapter{
		idMap: map[int64]string{
			2: "ETH",
		},
	}

	got := adp.mapCancelledOrder(&sdknado.Order{
		ProductID:      2,
		Digest:         "ord-1",
		Amount:         "-2000000000000000",
		UnfilledAmount: "1500000000000000",
		PriceX18:       "1234000000000000000000",
		PlacedAt:       1710000000,
	})

	require.NotNil(t, got)
	require.Equal(t, "ord-1", got.OrderID)
	require.Equal(t, "ETH", got.Symbol)
	require.Equal(t, exchanges.OrderSideSell, got.Side)
	require.Equal(t, exchanges.OrderStatusCancelled, got.Status)
	require.True(t, got.Quantity.Equal(decimal.RequireFromString("0.002")))
	require.True(t, got.FilledQuantity.Equal(decimal.RequireFromString("0.0005")))
	require.True(t, got.Price.Equal(decimal.RequireFromString("1234")))
	require.True(t, got.OrderPrice.Equal(decimal.RequireFromString("1234")))
}
