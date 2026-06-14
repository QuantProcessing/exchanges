package model

import "errors"

var (
	ErrInvalidInstrumentID   = errors.New("invalid instrument id")
	ErrInvalidInstrument     = errors.New("invalid instrument")
	ErrInstrumentNotFound    = errors.New("instrument not found")
	ErrInvalidMarketData     = errors.New("invalid market data")
	ErrInvalidOrder          = errors.New("invalid order")
	ErrInvalidAccount        = errors.New("invalid account")
	ErrInvalidExecutionEvent = errors.New("invalid execution event")
	ErrNotSupported          = errors.New("not supported")
)
