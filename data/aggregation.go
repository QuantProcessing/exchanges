package data

import (
	"fmt"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/shopspring/decimal"
)

type BarAggregator struct {
	barType model.BarType
	current *model.Bar
}

func NewBarAggregator(barType model.BarType) (*BarAggregator, error) {
	barType = barType.Canonical()
	if err := barType.Validate(); err != nil {
		return nil, err
	}
	return &BarAggregator{barType: barType}, nil
}

func (a *BarAggregator) Update(event model.MarketEvent) (model.Bar, bool, error) {
	if a == nil {
		return model.Bar{}, false, fmt.Errorf("%w: aggregator is nil", model.ErrInvalidMarketData)
	}
	price, volume, ts, ok := aggregationInput(event)
	if !ok {
		return model.Bar{}, false, nil
	}
	if event.InstrumentID() != a.barType.InstrumentID {
		return model.Bar{}, false, nil
	}
	bucket := ts.Truncate(a.barType.Step)
	if a.current == nil {
		a.current = &model.Bar{
			BarType:   a.barType,
			Open:      price,
			High:      price,
			Low:       price,
			Close:     price,
			Volume:    volume,
			Timestamp: bucket,
			InitTime:  ts,
		}
		return model.Bar{}, false, nil
	}
	if !a.current.Timestamp.Equal(bucket) {
		completed := *a.current
		a.current = &model.Bar{
			BarType:   a.barType,
			Open:      price,
			High:      price,
			Low:       price,
			Close:     price,
			Volume:    volume,
			Timestamp: bucket,
			InitTime:  ts,
		}
		return completed, true, completed.Validate()
	}
	if price.GreaterThan(a.current.High) {
		a.current.High = price
	}
	if price.LessThan(a.current.Low) {
		a.current.Low = price
	}
	a.current.Close = price
	a.current.Volume = a.current.Volume.Add(volume)
	return model.Bar{}, false, nil
}

func (a *BarAggregator) Flush() (model.Bar, bool, error) {
	if a == nil || a.current == nil {
		return model.Bar{}, false, nil
	}
	bar := *a.current
	a.current = nil
	return bar, true, bar.Validate()
}

func aggregationInput(event model.MarketEvent) (decimal.Decimal, decimal.Decimal, time.Time, bool) {
	switch {
	case event.Trade != nil:
		return event.Trade.Price, event.Trade.Size, event.Trade.Timestamp, true
	case event.Ticker != nil && event.Ticker.Last.IsPositive():
		return event.Ticker.Last, decimal.Zero, event.Ticker.Timestamp, true
	case event.Quote != nil:
		mid := event.Quote.BidPrice.Add(event.Quote.AskPrice).Div(decimal.NewFromInt(2))
		return mid, decimal.Zero, event.Quote.Timestamp, true
	default:
		return decimal.Zero, decimal.Zero, time.Time{}, false
	}
}
