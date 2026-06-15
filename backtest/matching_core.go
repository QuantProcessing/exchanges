package backtest

import (
	"time"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/shopspring/decimal"
)

type MatchingCoreConfig struct {
	Instrument model.Instrument
	FillModel  FillModel
}

type MatchingCore struct {
	instrument model.Instrument
	fillModel  FillModel
}

func NewMatchingCore(cfg MatchingCoreConfig) *MatchingCore {
	return &MatchingCore{
		instrument: cfg.Instrument,
		fillModel:  cfg.FillModel,
	}
}

type OrderBookMatchRequest struct {
	Order       model.OrderStatusReport
	Book        model.OrderBook
	Consumed    map[string]decimal.Decimal
	MaxQuantity decimal.Decimal
}

type Match struct {
	Source    FillSource
	Price     decimal.Decimal
	Quantity  decimal.Decimal
	Timestamp time.Time
}

func (c *MatchingCore) MatchOrderBook(req OrderBookMatchRequest) []Match {
	quantity := openQuantity(req.Order)
	if req.MaxQuantity.IsPositive() && req.MaxQuantity.LessThan(quantity) {
		quantity = req.MaxQuantity
	}
	if !quantity.IsPositive() {
		return nil
	}
	levels := req.Book.Asks
	if req.Order.Side == model.OrderSideSell {
		levels = req.Book.Bids
	}
	matches := make([]Match, 0, len(levels))
	for _, level := range levels {
		if !quantity.IsPositive() {
			break
		}
		if !canMatch(req.Order, level.Price) {
			break
		}
		available := level.Size.Sub(req.Consumed[level.Price.String()])
		fillQty := decimal.Min(quantity, available)
		if !fillQty.IsPositive() {
			continue
		}
		if !c.shouldFillLimitTouch(req.Order, level.Price, fillQty, req.Book.Timestamp, FillSourceOrderBook) {
			continue
		}
		matches = append(matches, Match{
			Source:    FillSourceOrderBook,
			Price:     level.Price,
			Quantity:  fillQty,
			Timestamp: req.Book.Timestamp,
		})
		quantity = quantity.Sub(fillQty)
	}
	return matches
}

func (c *MatchingCore) shouldFillLimitTouch(order model.OrderStatusReport, price decimal.Decimal, quantity decimal.Decimal, ts time.Time, source FillSource) bool {
	if !backtestPostOnlyCanFill(order, ts) {
		return false
	}
	if c == nil || c.fillModel == nil {
		return true
	}
	return c.fillModel.ShouldFillLimitTouch(FillContext{
		Order:      order,
		Instrument: c.instrument,
		Source:     source,
		Price:      price,
		Quantity:   quantity,
		Timestamp:  ts,
		LimitTouch: isLimitTouch(order, price),
		Taker:      !backtestFillIsMaker(order, ts),
	})
}
