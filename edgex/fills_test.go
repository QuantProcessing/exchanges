package edgex

import (
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	sdkperp "github.com/QuantProcessing/exchanges/edgex/sdk/perp"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestMapOrderFillReturnsExecutionDetails(t *testing.T) {
	adp := &Adapter{
		contractToSymbol: map[string]string{"c1": "BTC"},
		coinIdToCoin: map[string]*sdkperp.Coin{
			"coin-1": {CoinId: "coin-1", CoinName: "USDC"},
		},
	}

	fill := adp.mapOrderFill(&sdkperp.OrderFillTransaction{
		CoinId:      "coin-1",
		ContractId:  "c1",
		OrderId:     "order-1",
		OrderSide:   "SELL",
		FillSize:    "0.5",
		FillPrice:   "101.25",
		FillFee:     "0.01",
		MatchFillId: "trade-1",
		MatchTime:   "1700000000000",
	})

	require.Equal(t, "trade-1", fill.TradeID)
	require.Equal(t, "order-1", fill.OrderID)
	require.Equal(t, "BTC", fill.Symbol)
	require.Equal(t, exchanges.OrderSideSell, fill.Side)
	require.True(t, fill.Price.Equal(decimal.RequireFromString("101.25")))
	require.True(t, fill.Quantity.Equal(decimal.RequireFromString("0.5")))
	require.True(t, fill.Fee.Equal(decimal.RequireFromString("0.01")))
	require.Equal(t, "USDC", fill.FeeAsset)
	require.EqualValues(t, 1700000000000, fill.Timestamp)
}
