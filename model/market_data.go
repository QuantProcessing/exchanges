package model

import (
	"time"

	"github.com/shopspring/decimal"
)

type Ticker struct {
	InstrumentID InstrumentID
	Bid          decimal.Decimal
	Ask          decimal.Decimal
	Last         decimal.Decimal
	EventTime    time.Time
}

type OrderBookLevel struct {
	Price decimal.Decimal
	Size  decimal.Decimal
}

type OrderBook struct {
	InstrumentID InstrumentID
	Bids         []OrderBookLevel
	Asks         []OrderBookLevel
	EventTime    time.Time
}

type Trade struct {
	InstrumentID InstrumentID
	TradeID      TradeID
	Price        decimal.Decimal
	Size         decimal.Decimal
	Side         OrderSide
	EventTime    time.Time
}

type BarSpec struct {
	Interval string
}

type Bar struct {
	InstrumentID InstrumentID
	Spec         BarSpec
	Open         decimal.Decimal
	High         decimal.Decimal
	Low          decimal.Decimal
	Close        decimal.Decimal
	Volume       decimal.Decimal
	EventTime    time.Time
}

type OptionGreeks struct {
	InstrumentID    InstrumentID
	Delta           decimal.Decimal
	Gamma           decimal.Decimal
	Vega            decimal.Decimal
	Theta           decimal.Decimal
	MarkIV          decimal.Decimal
	UnderlyingPrice decimal.Decimal
	EventTime       time.Time
}

type OptionChainEntry struct {
	Instrument InstrumentID
	Bid        decimal.Decimal
	Ask        decimal.Decimal
	Greeks     *OptionGreeks
}

type OptionChainSlice struct {
	Series    OptionSeriesID
	Entries   []OptionChainEntry
	EventTime time.Time
}
