package lighter

import (
	"testing"

	sdklighter "github.com/QuantProcessing/exchanges/lighter/sdk"
	"github.com/stretchr/testify/require"
)

func TestMapLighterTradeToFillKeepsMillisecondTimestamp(t *testing.T) {
	fill := mapLighterTradeToFill(sdklighter.Trade{
		TradeId:      7,
		MarketId:     3,
		AskId:        91,
		AskAccountId: 11,
		IsMakerAsk:   true,
		Price:        "123.4",
		Size:         "0.5",
		Timestamp:    1700000000123,
	}, map[int]string{3: "BTC"}, 11)

	require.NotNil(t, fill)
	require.Equal(t, int64(1700000000123), fill.Timestamp)
}
