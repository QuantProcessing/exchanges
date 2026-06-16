package model

import (
	"fmt"
	"time"
)

type DataRequestID string

type DataRequest struct {
	Metadata     CommandMetadata
	RequestID    DataRequestID
	InstrumentID InstrumentID
	Type         MarketDataType
	BarType      BarType
	Start        time.Time
	End          time.Time
	Limit        int
	Depth        int
	CustomType   string
}

func (r DataRequest) Validate() error {
	if r.RequestID == "" {
		return fmt.Errorf("%w: missing request id", ErrInvalidMarketData)
	}
	if err := r.InstrumentID.Validate(); err != nil {
		return err
	}
	if err := r.Type.Validate(); err != nil {
		return err
	}
	if r.Limit < 0 {
		return fmt.Errorf("%w: negative request limit", ErrInvalidMarketData)
	}
	if !r.Start.IsZero() && !r.End.IsZero() && !r.End.After(r.Start) {
		return fmt.Errorf("%w: request end must be after start", ErrInvalidMarketData)
	}
	switch r.Type {
	case MarketDataTypeBar:
		barType := r.BarType.Canonical()
		if err := barType.Validate(); err != nil {
			return err
		}
		if barType.InstrumentID != r.InstrumentID {
			return fmt.Errorf("%w: bar request instrument mismatch", ErrInvalidMarketData)
		}
	case MarketDataTypeOrderBook:
		if r.Depth <= 0 {
			return fmt.Errorf("%w: order book request depth must be positive", ErrInvalidMarketData)
		}
	case MarketDataTypeCustom:
		if r.CustomType == "" {
			return fmt.Errorf("%w: missing custom data type", ErrInvalidMarketData)
		}
	default:
		if r.Depth < 0 {
			return fmt.Errorf("%w: request depth cannot be negative", ErrInvalidMarketData)
		}
	}
	return nil
}

type DataResponse struct {
	Metadata     CommandMetadata
	RequestID    DataRequestID
	InstrumentID InstrumentID
	Type         MarketDataType
	BarType      BarType
	Events       []MarketEvent
	IsFinal      bool
}

func (r DataResponse) Validate() error {
	if r.RequestID == "" {
		return fmt.Errorf("%w: missing response request id", ErrInvalidMarketData)
	}
	if err := r.InstrumentID.Validate(); err != nil {
		return err
	}
	if err := r.Type.Validate(); err != nil {
		return err
	}
	if r.Type == MarketDataTypeBar {
		barType := r.BarType.Canonical()
		if err := barType.Validate(); err != nil {
			return err
		}
		if barType.InstrumentID != r.InstrumentID {
			return fmt.Errorf("%w: bar response instrument mismatch", ErrInvalidMarketData)
		}
	}
	for _, event := range r.Events {
		if err := event.Validate(); err != nil {
			return err
		}
		if event.InstrumentID() != r.InstrumentID {
			return fmt.Errorf("%w: response instrument mismatch", ErrInvalidMarketData)
		}
		if !marketEventMatchesType(event, r.Type) {
			return fmt.Errorf("%w: response event type mismatch", ErrInvalidMarketData)
		}
	}
	return nil
}

func marketEventMatchesType(event MarketEvent, typ MarketDataType) bool {
	switch typ {
	case MarketDataTypeTicker:
		return event.Ticker != nil
	case MarketDataTypeOrderBook:
		return event.OrderBook != nil
	case MarketDataTypeTradeTick:
		return event.Trade != nil
	case MarketDataTypeQuoteTick:
		return event.Quote != nil
	case MarketDataTypeBar:
		return event.Bar != nil
	case MarketDataTypeFundingRate:
		return event.FundingRate != nil
	case MarketDataTypeCustom:
		return event.Custom != nil
	default:
		return false
	}
}
