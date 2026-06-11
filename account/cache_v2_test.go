package account

import (
	"testing"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestV2CacheStoresInstrument(t *testing.T) {
	cache := NewV2Cache()
	inst := model.Instrument{
		ID:        model.MustInstrumentID("BTC-USDT-PERP.BINANCE"),
		RawSymbol: "BTCUSDT",
		Type:      model.InstrumentTypeCryptoPerp,
		Base:      model.BTC,
		Quote:     model.USDT,
		Settle:    model.USDT,
		PriceStep: decimal.RequireFromString("0.1"),
		SizeStep:  decimal.RequireFromString("0.001"),
	}
	require.NoError(t, cache.PutInstrument(inst))
	got, ok := cache.Instrument(inst.ID)
	require.True(t, ok)
	require.Equal(t, inst.ID, got.ID)
}

func TestV2CacheAppliesAccountStateReplacement(t *testing.T) {
	cache := NewV2Cache()
	total := model.Money{Amount: decimal.NewFromInt(10), Currency: model.USDT}
	free := model.Money{Amount: decimal.NewFromInt(8), Currency: model.USDT}
	bal, err := model.BalanceFromTotalAndFree(total, free)
	require.NoError(t, err)

	state := model.AccountState{
		AccountID: "acct",
		Venue:     model.VenueBinance,
		Type:      model.AccountTypeMargin,
		Balances:  []model.AccountBalance{bal},
	}
	require.NoError(t, cache.ApplyAccountState(state))

	got, ok := cache.AccountState(model.VenueBinance, "acct")
	require.True(t, ok)
	require.Len(t, got.Balances, 1)
	require.True(t, got.Balances[0].Free.Amount.Equal(decimal.NewFromInt(8)))
}
