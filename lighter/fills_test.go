package lighter

import (
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	sdklighter "github.com/QuantProcessing/exchanges/lighter/sdk"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestMapLighterTradeToFillReturnsPerpExecutionDetails(t *testing.T) {
	fill := mapLighterTradeToFill(sdklighter.Trade{
		TradeId:      11,
		MarketId:     1,
		Size:         "0.5",
		Price:        "101.25",
		AskId:        900,
		BidId:        901,
		AskAccountId: 123,
		BidAccountId: 456,
		IsMakerAsk:   true,
		Timestamp:    1700000000,
	}, map[int]string{1: "BTC"}, 123)

	require.NotNil(t, fill)
	require.Equal(t, "11", fill.TradeID)
	require.Equal(t, "900", fill.OrderID)
	require.Equal(t, "BTC", fill.Symbol)
	require.Equal(t, exchanges.OrderSideSell, fill.Side)
	require.True(t, fill.Price.Equal(decimal.RequireFromString("101.25")))
	require.True(t, fill.Quantity.Equal(decimal.RequireFromString("0.5")))
	require.True(t, fill.IsMaker)
	require.EqualValues(t, 1700000000000, fill.Timestamp)
}

func TestMapLighterTradeToFillReturnsSpotExecutionDetails(t *testing.T) {
	fill := mapLighterTradeToFill(sdklighter.Trade{
		TradeId:      22,
		MarketId:     2,
		Size:         "1.25",
		Price:        "202.5",
		AskId:        902,
		BidId:        903,
		AskAccountId: 123,
		BidAccountId: 456,
		IsMakerAsk:   false,
		Timestamp:    1700000123,
	}, map[int]string{2: "ETH"}, 456)

	require.NotNil(t, fill)
	require.Equal(t, "22", fill.TradeID)
	require.Equal(t, "903", fill.OrderID)
	require.Equal(t, "ETH", fill.Symbol)
	require.Equal(t, exchanges.OrderSideBuy, fill.Side)
	require.True(t, fill.Price.Equal(decimal.RequireFromString("202.5")))
	require.True(t, fill.Quantity.Equal(decimal.RequireFromString("1.25")))
	require.True(t, fill.IsMaker)
	require.EqualValues(t, 1700000123000, fill.Timestamp)
}

func TestMapLighterTradeToFillIgnoresUnrelatedAccounts(t *testing.T) {
	require.Nil(t, mapLighterTradeToFill(sdklighter.Trade{
		AskAccountId: 1,
		BidAccountId: 2,
	}, map[int]string{}, 999))
}
