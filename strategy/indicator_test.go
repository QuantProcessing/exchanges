package strategy

import (
	"testing"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestExponentialMovingAverageInitializesAndUpdates(t *testing.T) {
	ema, err := NewExponentialMovingAverage(3)
	require.NoError(t, err)
	require.Equal(t, "EMA(3)", ema.Name())
	require.False(t, ema.Initialized())

	require.NoError(t, ema.Update(decimal.RequireFromString("10")))
	require.True(t, ema.Value().Equal(decimal.RequireFromString("10")), ema.Value().String())
	require.False(t, ema.Initialized())

	require.NoError(t, ema.Update(decimal.RequireFromString("12")))
	require.True(t, ema.Value().Equal(decimal.RequireFromString("11")), ema.Value().String())
	require.False(t, ema.Initialized())

	require.NoError(t, ema.Update(decimal.RequireFromString("14")))
	require.True(t, ema.Value().Equal(decimal.RequireFromString("12.5")), ema.Value().String())
	require.True(t, ema.Initialized())

	require.NoError(t, ema.Update(decimal.RequireFromString("16")))
	require.True(t, ema.Value().Equal(decimal.RequireFromString("14.25")), ema.Value().String())
	require.Equal(t, 4, ema.Count())
}

func TestAverageTrueRangeInitializesAndUpdatesFromBars(t *testing.T) {
	atr, err := NewAverageTrueRange(3)
	require.NoError(t, err)
	require.Equal(t, "ATR(3)", atr.Name())
	require.False(t, atr.Initialized())

	for _, bar := range []model.Bar{
		indicatorBar("10", "8", "9"),
		indicatorBar("12", "9", "11"),
		indicatorBar("13", "10", "12"),
	} {
		require.NoError(t, atr.UpdateBar(bar))
	}
	require.True(t, atr.Initialized())
	require.True(t, atr.Value().Round(4).Equal(decimal.RequireFromString("2.6667")), atr.Value().String())

	require.NoError(t, atr.UpdateBar(indicatorBar("14", "11", "13")))
	require.True(t, atr.Value().Round(4).Equal(decimal.RequireFromString("2.7778")), atr.Value().String())
	require.Equal(t, 4, atr.Count())
}

func TestIndicatorsRejectInvalidPeriods(t *testing.T) {
	_, err := NewExponentialMovingAverage(0)
	require.ErrorContains(t, err, "period must be positive")

	_, err = NewAverageTrueRange(0)
	require.ErrorContains(t, err, "period must be positive")
}

func indicatorBar(high string, low string, close string) model.Bar {
	return model.Bar{
		BarType:   model.NewTimeBarType(model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"), time.Minute),
		Open:      decimal.RequireFromString(close),
		High:      decimal.RequireFromString(high),
		Low:       decimal.RequireFromString(low),
		Close:     decimal.RequireFromString(close),
		Volume:    decimal.RequireFromString("1"),
		Timestamp: time.Unix(1, 0),
	}
}
