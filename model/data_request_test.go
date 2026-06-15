package model

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestDataRequestValidatesHistoricalBarsTicksBooksAndCustomData(t *testing.T) {
	instID := MustInstrumentID("BTC-USDT-PERP.BINANCE")
	barType := NewTimeBarType(instID, time.Minute)
	windowStart := time.Unix(100, 0)
	windowEnd := time.Unix(200, 0)

	requests := []DataRequest{
		{
			Metadata:     CommandMetadata{CommandID: "request-bars"},
			RequestID:    "bars-001",
			InstrumentID: instID,
			Type:         MarketDataTypeBar,
			BarType:      barType,
			Start:        windowStart,
			End:          windowEnd,
			Limit:        100,
		},
		{
			Metadata:     CommandMetadata{CommandID: "request-trades"},
			RequestID:    "trades-001",
			InstrumentID: instID,
			Type:         MarketDataTypeTradeTick,
			Start:        windowStart,
			End:          windowEnd,
			Limit:        100,
		},
		{
			Metadata:     CommandMetadata{CommandID: "request-quotes"},
			RequestID:    "quotes-001",
			InstrumentID: instID,
			Type:         MarketDataTypeQuoteTick,
			Start:        windowStart,
			End:          windowEnd,
			Limit:        100,
		},
		{
			Metadata:     CommandMetadata{CommandID: "request-book"},
			RequestID:    "book-001",
			InstrumentID: instID,
			Type:         MarketDataTypeOrderBook,
			Depth:        25,
			Start:        windowStart,
			End:          windowEnd,
		},
		{
			Metadata:     CommandMetadata{CommandID: "request-custom"},
			RequestID:    "custom-001",
			InstrumentID: instID,
			Type:         MarketDataTypeCustom,
			CustomType:   "funding_rate",
			Start:        windowStart,
			End:          windowEnd,
		},
	}

	for _, request := range requests {
		require.NoError(t, request.Validate(), request.RequestID)
	}

	request := requests[0]
	request.RequestID = ""
	require.ErrorIs(t, request.Validate(), ErrInvalidMarketData)

	request = requests[0]
	request.End = request.Start
	require.ErrorIs(t, request.Validate(), ErrInvalidMarketData)

	request = requests[4]
	request.CustomType = ""
	require.ErrorIs(t, request.Validate(), ErrInvalidMarketData)
}

func TestDataResponseValidatesPayloadsAgainstRequestShape(t *testing.T) {
	instID := MustInstrumentID("BTC-USDT-PERP.BINANCE")
	request := DataRequest{
		RequestID:    "quotes-001",
		InstrumentID: instID,
		Type:         MarketDataTypeQuoteTick,
		Start:        time.Unix(100, 0),
		End:          time.Unix(200, 0),
	}
	response := DataResponse{
		Metadata:     CommandMetadata{CorrelationID: "quotes-001"},
		RequestID:    request.RequestID,
		InstrumentID: instID,
		Type:         request.Type,
		Events: []MarketEvent{{
			Quote: &QuoteTick{
				InstrumentID: instID,
				BidPrice:     decimal.RequireFromString("100"),
				AskPrice:     decimal.RequireFromString("101"),
				BidSize:      decimal.RequireFromString("1"),
				AskSize:      decimal.RequireFromString("1"),
			},
		}},
		IsFinal: true,
	}
	require.NoError(t, response.Validate())

	response.Events[0] = MarketEvent{Trade: &TradeTick{
		InstrumentID:  instID,
		Price:         decimal.RequireFromString("100"),
		Size:          decimal.RequireFromString("1"),
		AggressorSide: AggressorSideBuyer,
		TradeID:       "trade-001",
	}}
	require.ErrorIs(t, response.Validate(), ErrInvalidMarketData)
}

func TestCustomDataCanTravelAsAMarketEvent(t *testing.T) {
	instID := MustInstrumentID("BTC-USDT-PERP.BINANCE")
	event := MarketEvent{Custom: &CustomData{
		InstrumentID: instID,
		Type:         "funding_rate",
		Fields:       map[string]string{"rate": "0.0001"},
		Timestamp:    time.Unix(100, 0),
	}}

	require.NoError(t, event.Validate())
	require.Equal(t, instID, event.InstrumentID())
}
