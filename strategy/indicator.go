package strategy

import (
	"fmt"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/shopspring/decimal"
)

type Indicator interface {
	Name() string
	Initialized() bool
	Count() int
	Value() decimal.Decimal
	Reset()
}

type ExponentialMovingAverage struct {
	period      int
	count       int
	multiplier  decimal.Decimal
	value       decimal.Decimal
	initialized bool
}

func NewExponentialMovingAverage(period int) (*ExponentialMovingAverage, error) {
	if period <= 0 {
		return nil, fmt.Errorf("%w: period must be positive", ErrInvalidIndicator)
	}
	return &ExponentialMovingAverage{
		period:     period,
		multiplier: decimal.NewFromInt(2).Div(decimal.NewFromInt(int64(period + 1))),
	}, nil
}

func (e *ExponentialMovingAverage) Name() string {
	if e == nil {
		return "EMA(0)"
	}
	return fmt.Sprintf("EMA(%d)", e.period)
}

func (e *ExponentialMovingAverage) Initialized() bool {
	return e != nil && e.initialized
}

func (e *ExponentialMovingAverage) Count() int {
	if e == nil {
		return 0
	}
	return e.count
}

func (e *ExponentialMovingAverage) Value() decimal.Decimal {
	if e == nil {
		return decimal.Zero
	}
	return e.value
}

func (e *ExponentialMovingAverage) Reset() {
	if e == nil {
		return
	}
	e.count = 0
	e.value = decimal.Zero
	e.initialized = false
}

func (e *ExponentialMovingAverage) Update(value decimal.Decimal) error {
	if e == nil || e.period <= 0 {
		return ErrInvalidIndicator
	}
	e.count++
	if e.count == 1 {
		e.value = value
	} else {
		e.value = value.Sub(e.value).Mul(e.multiplier).Add(e.value)
	}
	e.initialized = e.count >= e.period
	return nil
}

type AverageTrueRange struct {
	period      int
	count       int
	value       decimal.Decimal
	trSum       decimal.Decimal
	prevClose   decimal.Decimal
	hasPrevious bool
	initialized bool
}

func NewAverageTrueRange(period int) (*AverageTrueRange, error) {
	if period <= 0 {
		return nil, fmt.Errorf("%w: period must be positive", ErrInvalidIndicator)
	}
	return &AverageTrueRange{period: period}, nil
}

func (a *AverageTrueRange) Name() string {
	if a == nil {
		return "ATR(0)"
	}
	return fmt.Sprintf("ATR(%d)", a.period)
}

func (a *AverageTrueRange) Initialized() bool {
	return a != nil && a.initialized
}

func (a *AverageTrueRange) Count() int {
	if a == nil {
		return 0
	}
	return a.count
}

func (a *AverageTrueRange) Value() decimal.Decimal {
	if a == nil {
		return decimal.Zero
	}
	return a.value
}

func (a *AverageTrueRange) Reset() {
	if a == nil {
		return
	}
	a.count = 0
	a.value = decimal.Zero
	a.trSum = decimal.Zero
	a.prevClose = decimal.Zero
	a.hasPrevious = false
	a.initialized = false
}

func (a *AverageTrueRange) UpdateBar(bar model.Bar) error {
	if a == nil || a.period <= 0 {
		return ErrInvalidIndicator
	}
	if err := bar.Validate(); err != nil {
		return err
	}
	tr := a.trueRange(bar)
	a.count++
	switch {
	case a.count < a.period:
		a.trSum = a.trSum.Add(tr)
	case a.count == a.period:
		a.trSum = a.trSum.Add(tr)
		a.value = a.trSum.Div(decimal.NewFromInt(int64(a.period)))
		a.initialized = true
	default:
		period := decimal.NewFromInt(int64(a.period))
		a.value = a.value.Mul(period.Sub(decimal.NewFromInt(1))).Add(tr).Div(period)
	}
	a.prevClose = bar.Close
	a.hasPrevious = true
	return nil
}

func (a *AverageTrueRange) trueRange(bar model.Bar) decimal.Decimal {
	highLow := bar.High.Sub(bar.Low).Abs()
	if !a.hasPrevious {
		return highLow
	}
	highPrevClose := bar.High.Sub(a.prevClose).Abs()
	lowPrevClose := bar.Low.Sub(a.prevClose).Abs()
	tr := highLow
	if highPrevClose.GreaterThan(tr) {
		tr = highPrevClose
	}
	if lowPrevClose.GreaterThan(tr) {
		tr = lowPrevClose
	}
	return tr
}

var ErrInvalidIndicator = fmt.Errorf("invalid indicator")
