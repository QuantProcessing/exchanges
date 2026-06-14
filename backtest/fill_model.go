package backtest

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/shopspring/decimal"
)

type FillSource string

const (
	FillSourceTicker    FillSource = "ticker"
	FillSourceOrderBook FillSource = "order_book"
	FillSourceQuoteTick FillSource = "quote_tick"
	FillSourceTradeTick FillSource = "trade_tick"
	FillSourceBar       FillSource = "bar"
)

type FillContext struct {
	Order      model.OrderStatusReport
	Instrument model.Instrument
	Source     FillSource
	Price      decimal.Decimal
	Quantity   decimal.Decimal
	Timestamp  time.Time
	LimitTouch bool
	Taker      bool
}

type FillModel interface {
	ShouldFillLimitTouch(FillContext) bool
	ApplySlippage(FillContext, decimal.Decimal) decimal.Decimal
}

type FillModelConfig struct {
	ProbFillOnLimit    float64
	ProbFillOnLimitSet bool
	ProbSlippage       float64
	RandomSeed         int64
}

type ProbabilisticFillModel struct {
	probFillOnLimit float64
	probSlippage    float64
	rng             *rand.Rand
}

func DefaultFillModel() *ProbabilisticFillModel {
	model, _ := NewFillModel(FillModelConfig{})
	return model
}

func NewFillModel(cfg FillModelConfig) (*ProbabilisticFillModel, error) {
	probFillOnLimit := cfg.ProbFillOnLimit
	if !cfg.ProbFillOnLimitSet && probFillOnLimit == 0 {
		probFillOnLimit = 1
	}
	if err := validateProbability("prob_fill_on_limit", probFillOnLimit); err != nil {
		return nil, err
	}
	if err := validateProbability("prob_slippage", cfg.ProbSlippage); err != nil {
		return nil, err
	}
	return &ProbabilisticFillModel{
		probFillOnLimit: probFillOnLimit,
		probSlippage:    cfg.ProbSlippage,
		rng:             rand.New(rand.NewSource(cfg.RandomSeed)),
	}, nil
}

func validateProbability(name string, value float64) error {
	if value < 0 || value > 1 {
		return fmt.Errorf("%s must be in [0,1]", name)
	}
	return nil
}

func (m *ProbabilisticFillModel) ShouldFillLimitTouch(ctx FillContext) bool {
	if !ctx.LimitTouch {
		return true
	}
	return m.draw(ctx, m.probFillOnLimit)
}

func (m *ProbabilisticFillModel) ApplySlippage(ctx FillContext, price decimal.Decimal) decimal.Decimal {
	if !ctx.Taker || !isL1FillSource(ctx.Source) || !m.draw(ctx, m.probSlippage) {
		return price
	}
	tick := ctx.Instrument.PriceTick
	if !tick.IsPositive() {
		return price
	}
	if ctx.Order.Side == model.OrderSideSell {
		slipped := price.Sub(tick)
		if slipped.IsPositive() {
			return slipped
		}
		return price
	}
	return price.Add(tick)
}

func (m *ProbabilisticFillModel) draw(_ FillContext, probability float64) bool {
	if probability <= 0 {
		return false
	}
	if probability >= 1 {
		return true
	}
	return m.rng.Float64() < probability
}

func isL1FillSource(source FillSource) bool {
	switch source {
	case FillSourceTicker, FillSourceQuoteTick, FillSourceTradeTick, FillSourceBar:
		return true
	default:
		return false
	}
}
