package model

import "errors"

var (
	ErrInvalidInstrumentID = errors.New("invalid instrument id")
	ErrInstrumentNotLoaded = errors.New("instrument not loaded")
	ErrInvalidMoney        = errors.New("invalid money")
	ErrInvalidInstrument   = errors.New("invalid instrument")
	ErrInvalidAccountState = errors.New("invalid account state")
	ErrNotSupported        = errors.New("not supported")
)
