package data

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/QuantProcessing/exchanges/model"
)

type Catalog interface {
	Write(context.Context, ...model.MarketEvent) error
	Query(context.Context, model.DataRequest) (model.DataResponse, error)
}

type ReplayCatalog interface {
	Catalog
	Events(context.Context) ([]model.MarketEvent, error)
}

type MemoryCatalog struct {
	mu     sync.RWMutex
	events []model.MarketEvent
}

func NewMemoryCatalog(events ...model.MarketEvent) *MemoryCatalog {
	c := &MemoryCatalog{}
	_ = c.Write(context.Background(), events...)
	return c
}

func (c *MemoryCatalog) Write(_ context.Context, events ...model.MarketEvent) error {
	if c == nil {
		return fmt.Errorf("%w: catalog is nil", model.ErrInvalidMarketData)
	}
	valid := make([]model.MarketEvent, 0, len(events))
	for _, event := range events {
		if err := event.Validate(); err != nil {
			return err
		}
		valid = append(valid, event)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, valid...)
	sort.SliceStable(c.events, func(i, j int) bool {
		return marketEventTime(c.events[i]).Before(marketEventTime(c.events[j]))
	})
	return nil
}

func (c *MemoryCatalog) Query(_ context.Context, request model.DataRequest) (model.DataResponse, error) {
	if c == nil {
		return model.DataResponse{}, fmt.Errorf("%w: catalog is nil", model.ErrInvalidMarketData)
	}
	if err := request.Validate(); err != nil {
		return model.DataResponse{}, err
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	events := make([]model.MarketEvent, 0)
	for _, event := range c.events {
		if !catalogEventMatches(request, event) {
			continue
		}
		events = append(events, event)
		if request.Limit > 0 && len(events) >= request.Limit {
			break
		}
	}
	if len(events) == 0 {
		return model.DataResponse{}, fmt.Errorf("%w: no catalog data for request %s", model.ErrInvalidMarketData, request.RequestID)
	}
	response := model.DataResponse{
		Metadata:     responseMetadata(request),
		RequestID:    request.RequestID,
		InstrumentID: request.InstrumentID,
		Type:         request.Type,
		BarType:      request.BarType.Canonical(),
		Events:       events,
		IsFinal:      true,
	}
	if err := response.Validate(); err != nil {
		return model.DataResponse{}, err
	}
	return response, nil
}

func (c *MemoryCatalog) Events(context.Context) ([]model.MarketEvent, error) {
	if c == nil {
		return nil, fmt.Errorf("%w: catalog is nil", model.ErrInvalidMarketData)
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return append([]model.MarketEvent(nil), c.events...), nil
}

func catalogEventMatches(request model.DataRequest, event model.MarketEvent) bool {
	if event.InstrumentID() != request.InstrumentID {
		return false
	}
	if !marketEventMatchesRequestType(request, event) {
		return false
	}
	ts := marketEventTime(event)
	if !request.Start.IsZero() && ts.Before(request.Start) {
		return false
	}
	if !request.End.IsZero() && !ts.Before(request.End) {
		return false
	}
	if request.Type == model.MarketDataTypeBar && event.Bar != nil {
		return event.Bar.BarType.Canonical() == request.BarType.Canonical()
	}
	if request.Type == model.MarketDataTypeCustom && event.Custom != nil {
		return event.Custom.Type == request.CustomType
	}
	return true
}

func marketEventMatchesRequestType(request model.DataRequest, event model.MarketEvent) bool {
	switch request.Type {
	case model.MarketDataTypeTicker:
		return event.Ticker != nil
	case model.MarketDataTypeOrderBook:
		return event.OrderBook != nil
	case model.MarketDataTypeTradeTick:
		return event.Trade != nil
	case model.MarketDataTypeQuoteTick:
		return event.Quote != nil
	case model.MarketDataTypeBar:
		return event.Bar != nil
	case model.MarketDataTypeFundingRate:
		return event.FundingRate != nil
	case model.MarketDataTypeCustom:
		return event.Custom != nil
	default:
		return false
	}
}

func marketEventTime(event model.MarketEvent) time.Time {
	switch {
	case event.Ticker != nil:
		return event.Ticker.Timestamp
	case event.OrderBook != nil:
		return event.OrderBook.Timestamp
	case event.Trade != nil:
		return event.Trade.Timestamp
	case event.Quote != nil:
		return event.Quote.Timestamp
	case event.Bar != nil:
		return event.Bar.Timestamp
	case event.FundingRate != nil:
		return event.FundingRate.Timestamp
	case event.Custom != nil:
		return event.Custom.Timestamp
	default:
		return time.Time{}
	}
}

func EventTime(event model.MarketEvent) time.Time {
	return marketEventTime(event)
}

func responseMetadata(request model.DataRequest) model.CommandMetadata {
	metadata := request.Metadata.Clone()
	if metadata.CorrelationID == "" {
		metadata.CorrelationID = model.CorrelationID(request.RequestID)
	}
	return metadata
}
