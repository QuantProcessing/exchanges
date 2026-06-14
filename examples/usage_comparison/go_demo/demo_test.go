package godemo

import (
	"context"
	"testing"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/stretchr/testify/require"
)

func TestRunDemoShowsEndToEndTradingLifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := RunDemo(ctx)
	require.NoError(t, err)

	require.True(t, result.SignalTriggered)
	require.Equal(t, model.OrderStatusFilled, result.FinalOrder.Status)
	require.Equal(t, model.ClientOrderID("demo-imbalance-1"), result.FinalOrder.ClientOrderID)
	require.Len(t, result.Fills, 1)
	require.Equal(t, model.TradeID("demo-trade-1"), result.Fills[0].TradeID)
	require.Equal(t, model.PositionSideLong, result.Position.Side)
	require.Equal(t, "0.01", result.Position.Quantity.String())
	require.Equal(t, "1.01", result.Exposure.String())
	require.Contains(t, result.EventLog, "market:order_book")
	require.Contains(t, result.EventLog, "execution:fill")
}
