package testsuite

import (
	"testing"

	"github.com/QuantProcessing/exchanges/account"
	"github.com/QuantProcessing/exchanges/model"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

type V2ModelSuiteConfig struct {
	Instrument model.Instrument
	Account    model.AccountState
}

func RunV2ModelSuite(t *testing.T, cfg V2ModelSuiteConfig) {
	t.Helper()

	t.Run("InstrumentID", func(t *testing.T) {
		got, err := model.ParseInstrumentID("BTC-USDT-PERP.BINANCE")
		require.NoError(t, err)
		require.Equal(t, model.InstrumentID{Symbol: "BTC-USDT-PERP", Venue: model.VenueBinance}, got)
		require.Equal(t, "BTC-USDT-PERP.BINANCE", got.String())
	})

	t.Run("InstrumentValidation", func(t *testing.T) {
		require.NoError(t, cfg.Instrument.Validate())
	})

	t.Run("BalanceInvariant", func(t *testing.T) {
		total := model.Money{Amount: decimal.RequireFromString("10"), Currency: model.USDT}
		locked := model.Money{Amount: decimal.RequireFromString("2"), Currency: model.USDT}
		free := model.Money{Amount: decimal.RequireFromString("8"), Currency: model.USDT}
		bal, err := model.NewBalance(total, locked, free)
		require.NoError(t, err)
		require.True(t, bal.Total.Amount.Equal(total.Amount))
	})

	t.Run("AccountStateReplacement", func(t *testing.T) {
		cache := account.NewV2Cache()
		require.NoError(t, cache.ApplyAccountState(cfg.Account))
		got, ok := cache.AccountState(cfg.Account.Venue, cfg.Account.AccountID)
		require.True(t, ok)
		require.Equal(t, cfg.Account.AccountID, got.AccountID)
		require.Equal(t, cfg.Account.Venue, got.Venue)
	})
}
