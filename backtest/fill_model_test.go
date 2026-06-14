package backtest

import (
	"testing"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestFillModelConfigDefaultsToNautilusProbabilities(t *testing.T) {
	fillModel, err := NewFillModel(FillModelConfig{})
	require.NoError(t, err)
	ctx := FillContext{
		Order: model.OrderStatusReport{
			Side: model.OrderSideBuy,
		},
		Instrument: model.Instrument{PriceTick: decimal.RequireFromString("0.5")},
		Source:     FillSourceQuoteTick,
		Price:      decimal.RequireFromString("101"),
		LimitTouch: true,
		Taker:      true,
	}

	require.True(t, fillModel.ShouldFillLimitTouch(ctx))
	require.Equal(t, "101", fillModel.ApplySlippage(ctx, ctx.Price).String())
}
