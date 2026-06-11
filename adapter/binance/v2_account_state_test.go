package binance

import (
	"encoding/json"
	"testing"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/sdk/binance/perp"
	"github.com/QuantProcessing/exchanges/sdk/binance/spot"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestV2SpotAccountStateBalances(t *testing.T) {
	var resp spot.AccountResponse
	require.NoError(t, json.Unmarshal([]byte(`{
		"balances": [
			{"asset": "USDT", "free": "8", "locked": "2"},
			{"asset": "BTC", "free": "0", "locked": "0"}
		]
	}`), &resp))

	state, err := v2SpotAccountState("acct", &resp)
	require.NoError(t, err)
	require.Equal(t, model.AccountTypeCash, state.Type)
	require.Len(t, state.Balances, 1)
	require.True(t, state.Balances[0].Total.Amount.Equal(decimal.RequireFromString("10")))
	require.True(t, state.Balances[0].Free.Amount.Equal(decimal.RequireFromString("8")))
	require.True(t, state.Balances[0].Locked.Amount.Equal(decimal.RequireFromString("2")))
}

func TestV2PerpAccountStateBalancesAndAccountWideMargin(t *testing.T) {
	var resp perp.AccountResponse
	require.NoError(t, json.Unmarshal([]byte(`{
		"assets": [
			{"asset": "USDT", "marginBalance": "100", "availableBalance": "80", "initialMargin": "12", "maintMargin": "4"}
		],
		"positions": [
			{"symbol": "BTCUSDT", "positionAmt": "0.5", "entryPrice": "100", "unrealizedProfit": "3", "positionSide": "BOTH"}
		]
	}`), &resp))

	state, err := v2PerpAccountState("acct", &resp)
	require.NoError(t, err)
	require.Equal(t, model.AccountTypeMargin, state.Type)
	require.Len(t, state.Balances, 1)
	require.Len(t, state.Margins, 1)
	require.Nil(t, state.Margins[0].Instrument)
	require.True(t, state.Margins[0].Initial.Amount.Equal(decimal.RequireFromString("12")))
	require.Len(t, state.Positions, 1)
	require.Equal(t, model.MustInstrumentID("BTC-USDT-PERP.BINANCE"), state.Positions[0].InstrumentID)
	require.Equal(t, model.PositionSideLong, state.Positions[0].Side)
}
