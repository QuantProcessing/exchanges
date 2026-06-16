package model

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

type MarketDataType string

const (
	MarketDataTypeTicker      MarketDataType = "ticker"
	MarketDataTypeOrderBook   MarketDataType = "order_book"
	MarketDataTypeTradeTick   MarketDataType = "trade_tick"
	MarketDataTypeQuoteTick   MarketDataType = "quote_tick"
	MarketDataTypeBar         MarketDataType = "bar"
	MarketDataTypeFundingRate MarketDataType = "funding_rate"
	MarketDataTypeCustom      MarketDataType = "custom"
)

func (t MarketDataType) Validate() error {
	switch t {
	case MarketDataTypeTicker, MarketDataTypeOrderBook, MarketDataTypeTradeTick, MarketDataTypeQuoteTick, MarketDataTypeBar, MarketDataTypeFundingRate, MarketDataTypeCustom:
		return nil
	default:
		return fmt.Errorf("%w: invalid market data type %q", ErrInvalidMarketData, t)
	}
}

type SubscribeMarketData struct {
	InstrumentID InstrumentID
	Type         MarketDataType
	Depth        int
	BarType      BarType
}

func (s SubscribeMarketData) Validate() error {
	if err := s.InstrumentID.Validate(); err != nil {
		return err
	}
	if err := s.Type.Validate(); err != nil {
		return err
	}
	if s.Type == MarketDataTypeOrderBook && s.Depth <= 0 {
		return fmt.Errorf("%w: order book subscription depth must be positive", ErrInvalidMarketData)
	}
	if s.Type != MarketDataTypeOrderBook && s.Depth < 0 {
		return fmt.Errorf("%w: subscription depth cannot be negative", ErrInvalidMarketData)
	}
	if s.Type == MarketDataTypeBar {
		barType := s.BarType.Canonical()
		if err := barType.Validate(); err != nil {
			return err
		}
		if barType.InstrumentID != s.InstrumentID {
			return fmt.Errorf("%w: bar subscription instrument mismatch", ErrInvalidMarketData)
		}
	}
	return nil
}

func (s SubscribeMarketData) Key() string {
	if s.Type == MarketDataTypeBar {
		return s.Type.String() + ":" + s.BarType.Canonical().String()
	}
	return s.Type.String() + ":" + s.InstrumentID.String()
}

func (t MarketDataType) String() string { return string(t) }

type AggressorSide string

const (
	AggressorSideBuyer       AggressorSide = "buyer"
	AggressorSideSeller      AggressorSide = "seller"
	AggressorSideNoAggressor AggressorSide = "no_aggressor"
)

func (s AggressorSide) Validate() error {
	switch s {
	case AggressorSideBuyer, AggressorSideSeller, AggressorSideNoAggressor:
		return nil
	default:
		return fmt.Errorf("%w: invalid aggressor side %q", ErrInvalidMarketData, s)
	}
}

type BarAggregation string
type BarPriceType string
type AggregationSource string

const (
	BarAggregationTime BarAggregation = "time"

	BarPriceTypeLast BarPriceType = "last"

	AggregationSourceExternal AggregationSource = "external"
	AggregationSourceInternal AggregationSource = "internal"
)

type BarType struct {
	InstrumentID      InstrumentID
	Step              time.Duration
	Aggregation       BarAggregation
	PriceType         BarPriceType
	AggregationSource AggregationSource
}

func NewTimeBarType(instrumentID InstrumentID, step time.Duration) BarType {
	return BarType{
		InstrumentID:      instrumentID,
		Step:              step,
		Aggregation:       BarAggregationTime,
		PriceType:         BarPriceTypeLast,
		AggregationSource: AggregationSourceExternal,
	}
}

func (t BarType) Canonical() BarType {
	if t.Aggregation == "" {
		t.Aggregation = BarAggregationTime
	}
	if t.PriceType == "" {
		t.PriceType = BarPriceTypeLast
	}
	if t.AggregationSource == "" {
		t.AggregationSource = AggregationSourceExternal
	}
	return t
}

func (t BarType) String() string {
	t = t.Canonical()
	if t.InstrumentID.String() == "" {
		return ""
	}
	return fmt.Sprintf("%s:%s:%s:%s:%s", t.InstrumentID.String(), t.Step.String(), t.Aggregation, t.PriceType, t.AggregationSource)
}

func (t BarType) Validate() error {
	t = t.Canonical()
	if err := t.InstrumentID.Validate(); err != nil {
		return err
	}
	if t.Step <= 0 {
		return fmt.Errorf("%w: bar type step must be positive", ErrInvalidMarketData)
	}
	if t.Aggregation != BarAggregationTime {
		return fmt.Errorf("%w: unsupported bar aggregation %q", ErrInvalidMarketData, t.Aggregation)
	}
	if t.PriceType != BarPriceTypeLast {
		return fmt.Errorf("%w: unsupported bar price type %q", ErrInvalidMarketData, t.PriceType)
	}
	switch t.AggregationSource {
	case AggregationSourceExternal, AggregationSourceInternal:
		return nil
	default:
		return fmt.Errorf("%w: unsupported bar aggregation source %q", ErrInvalidMarketData, t.AggregationSource)
	}
}

type Ticker struct {
	InstrumentID InstrumentID
	Bid          decimal.Decimal
	Ask          decimal.Decimal
	Last         decimal.Decimal
	Timestamp    time.Time
}

func (t Ticker) Validate() error {
	if err := t.InstrumentID.Validate(); err != nil {
		return err
	}
	if t.Bid.IsNegative() || t.Ask.IsNegative() || t.Last.IsNegative() {
		return fmt.Errorf("%w: negative ticker price", ErrInvalidMarketData)
	}
	if t.Bid.IsPositive() && t.Ask.IsPositive() && t.Bid.GreaterThan(t.Ask) {
		return fmt.Errorf("%w: crossed ticker", ErrInvalidMarketData)
	}
	return nil
}

type TradeTick struct {
	InstrumentID  InstrumentID
	Price         decimal.Decimal
	Size          decimal.Decimal
	AggressorSide AggressorSide
	TradeID       TradeID
	Timestamp     time.Time
	InitTime      time.Time
}

func (t TradeTick) Validate() error {
	if err := t.InstrumentID.Validate(); err != nil {
		return err
	}
	if !t.Price.IsPositive() || !t.Size.IsPositive() {
		return fmt.Errorf("%w: non-positive trade tick price or size", ErrInvalidMarketData)
	}
	if err := t.AggressorSide.Validate(); err != nil {
		return err
	}
	if t.TradeID == "" {
		return fmt.Errorf("%w: missing trade tick trade id", ErrInvalidMarketData)
	}
	return nil
}

type QuoteTick struct {
	InstrumentID InstrumentID
	BidPrice     decimal.Decimal
	AskPrice     decimal.Decimal
	BidSize      decimal.Decimal
	AskSize      decimal.Decimal
	Timestamp    time.Time
	InitTime     time.Time
}

func (q QuoteTick) Validate() error {
	if err := q.InstrumentID.Validate(); err != nil {
		return err
	}
	if !q.BidPrice.IsPositive() || !q.AskPrice.IsPositive() || !q.BidSize.IsPositive() || !q.AskSize.IsPositive() {
		return fmt.Errorf("%w: non-positive quote tick price or size", ErrInvalidMarketData)
	}
	if q.BidPrice.GreaterThan(q.AskPrice) {
		return fmt.Errorf("%w: crossed quote tick", ErrInvalidMarketData)
	}
	return nil
}

type Bar struct {
	BarType    BarType
	Open       decimal.Decimal
	High       decimal.Decimal
	Low        decimal.Decimal
	Close      decimal.Decimal
	Volume     decimal.Decimal
	Timestamp  time.Time
	InitTime   time.Time
	IsRevision bool
}

func (b Bar) Validate() error {
	barType := b.BarType.Canonical()
	if err := barType.Validate(); err != nil {
		return err
	}
	if !b.Open.IsPositive() || !b.High.IsPositive() || !b.Low.IsPositive() || !b.Close.IsPositive() {
		return fmt.Errorf("%w: non-positive bar price", ErrInvalidMarketData)
	}
	if b.Volume.IsNegative() {
		return fmt.Errorf("%w: negative bar volume", ErrInvalidMarketData)
	}
	if b.High.LessThan(b.Open) || b.High.LessThan(b.Low) || b.High.LessThan(b.Close) {
		return fmt.Errorf("%w: bar high below OHLC value", ErrInvalidMarketData)
	}
	if b.Low.GreaterThan(b.Open) || b.Low.GreaterThan(b.Close) {
		return fmt.Errorf("%w: bar low above OHLC value", ErrInvalidMarketData)
	}
	return nil
}

type OrderBookLevel struct {
	Price decimal.Decimal
	Size  decimal.Decimal
}

type OrderBook struct {
	InstrumentID InstrumentID
	Bids         []OrderBookLevel
	Asks         []OrderBookLevel
	Timestamp    time.Time
}

func (b OrderBook) Validate() error {
	if err := b.InstrumentID.Validate(); err != nil {
		return err
	}
	for _, level := range append(append([]OrderBookLevel{}, b.Bids...), b.Asks...) {
		if !level.Price.IsPositive() || !level.Size.IsPositive() {
			return fmt.Errorf("%w: non-positive book level", ErrInvalidMarketData)
		}
	}
	if len(b.Bids) > 0 && len(b.Asks) > 0 && b.Bids[0].Price.GreaterThanOrEqual(b.Asks[0].Price) {
		return fmt.Errorf("%w: crossed book", ErrInvalidMarketData)
	}
	return nil
}

type FundingRate struct {
	InstrumentID InstrumentID
	// Rate is the venue's settlement-interval funding rate. FundingInterval
	// declares that interval when the venue exposes it; callers that need an
	// hourly rate should derive it explicitly from these fields.
	Rate            decimal.Decimal
	MarkPrice       decimal.Decimal
	IndexPrice      decimal.Decimal
	NextFundingTime time.Time
	FundingInterval time.Duration
	Timestamp       time.Time
	InitTime        time.Time
}

func (f FundingRate) Validate() error {
	if err := f.InstrumentID.Validate(); err != nil {
		return err
	}
	if f.MarkPrice.IsNegative() || f.IndexPrice.IsNegative() {
		return fmt.Errorf("%w: negative funding reference price", ErrInvalidMarketData)
	}
	if f.FundingInterval < 0 {
		return fmt.Errorf("%w: funding interval cannot be negative", ErrInvalidMarketData)
	}
	return nil
}

type CustomData struct {
	InstrumentID InstrumentID
	Type         string
	Fields       map[string]string
	Timestamp    time.Time
	InitTime     time.Time
}

func (d CustomData) Validate() error {
	if err := d.InstrumentID.Validate(); err != nil {
		return err
	}
	if d.Type == "" {
		return fmt.Errorf("%w: missing custom data type", ErrInvalidMarketData)
	}
	return nil
}

type MarketEvent struct {
	Ticker      *Ticker
	OrderBook   *OrderBook
	Trade       *TradeTick
	Quote       *QuoteTick
	Bar         *Bar
	FundingRate *FundingRate
	Custom      *CustomData
}

func (e MarketEvent) InstrumentID() InstrumentID {
	switch {
	case e.Ticker != nil:
		return e.Ticker.InstrumentID
	case e.OrderBook != nil:
		return e.OrderBook.InstrumentID
	case e.Trade != nil:
		return e.Trade.InstrumentID
	case e.Quote != nil:
		return e.Quote.InstrumentID
	case e.Bar != nil:
		return e.Bar.BarType.Canonical().InstrumentID
	case e.FundingRate != nil:
		return e.FundingRate.InstrumentID
	case e.Custom != nil:
		return e.Custom.InstrumentID
	default:
		return InstrumentID{}
	}
}

func (e MarketEvent) Validate() error {
	count := 0
	if e.Ticker != nil {
		count++
		if err := e.Ticker.Validate(); err != nil {
			return err
		}
	}
	if e.OrderBook != nil {
		count++
		if err := e.OrderBook.Validate(); err != nil {
			return err
		}
	}
	if e.Trade != nil {
		count++
		if err := e.Trade.Validate(); err != nil {
			return err
		}
	}
	if e.Quote != nil {
		count++
		if err := e.Quote.Validate(); err != nil {
			return err
		}
	}
	if e.Bar != nil {
		count++
		if err := e.Bar.Validate(); err != nil {
			return err
		}
	}
	if e.FundingRate != nil {
		count++
		if err := e.FundingRate.Validate(); err != nil {
			return err
		}
	}
	if e.Custom != nil {
		count++
		if err := e.Custom.Validate(); err != nil {
			return err
		}
	}
	if count != 1 {
		return fmt.Errorf("%w: market event needs one payload", ErrInvalidMarketData)
	}
	return nil
}
