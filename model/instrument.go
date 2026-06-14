package model

import (
	"fmt"

	"github.com/shopspring/decimal"
)

type InstrumentType string

const (
	InstrumentTypeSpot   InstrumentType = "spot"
	InstrumentTypePerp   InstrumentType = "perp"
	InstrumentTypeFuture InstrumentType = "future"
	InstrumentTypeOption InstrumentType = "option"
)

type InstrumentStatus string

const (
	InstrumentStatusTrading InstrumentStatus = "trading"
	InstrumentStatusHalted  InstrumentStatus = "halted"
)

type Instrument struct {
	ID          InstrumentID
	RawSymbol   string
	Type        InstrumentType
	Base        Currency
	Quote       Currency
	Settle      Currency
	PriceTick   decimal.Decimal
	SizeTick    decimal.Decimal
	MakerFee    decimal.Decimal
	TakerFee    decimal.Decimal
	MarginInit  decimal.Decimal
	MarginMaint decimal.Decimal
	Status      InstrumentStatus
}

func (i Instrument) Validate() error {
	if err := i.ID.Validate(); err != nil {
		return err
	}
	if i.RawSymbol == "" || i.Type == "" || i.Base == "" || i.Quote == "" {
		return fmt.Errorf("%w: missing identity fields", ErrInvalidInstrument)
	}
	if i.Type != InstrumentTypeSpot && i.Settle == "" {
		return fmt.Errorf("%w: missing settle currency", ErrInvalidInstrument)
	}
	if !i.PriceTick.IsPositive() || !i.SizeTick.IsPositive() {
		return fmt.Errorf("%w: non-positive increments", ErrInvalidInstrument)
	}
	if i.MakerFee.IsNegative() || i.TakerFee.IsNegative() {
		return fmt.Errorf("%w: negative fee rate", ErrInvalidInstrument)
	}
	if i.MarginInit.IsNegative() || i.MarginMaint.IsNegative() {
		return fmt.Errorf("%w: negative margin rate", ErrInvalidInstrument)
	}
	if i.Status == "" {
		return fmt.Errorf("%w: missing status", ErrInvalidInstrument)
	}
	return nil
}

func (i Instrument) ValidatePrice(price decimal.Decimal) error {
	if !price.IsPositive() {
		return fmt.Errorf("%w: non-positive price", ErrInvalidOrder)
	}
	if !isMultipleOf(price, i.PriceTick) {
		return fmt.Errorf("%w: price does not match tick size", ErrInvalidOrder)
	}
	return nil
}

func (i Instrument) ValidateSize(size decimal.Decimal) error {
	if !size.IsPositive() {
		return fmt.Errorf("%w: non-positive size", ErrInvalidOrder)
	}
	if !isMultipleOf(size, i.SizeTick) {
		return fmt.Errorf("%w: size does not match lot size", ErrInvalidOrder)
	}
	return nil
}

func isMultipleOf(value decimal.Decimal, increment decimal.Decimal) bool {
	if !increment.IsPositive() {
		return false
	}
	return value.Mod(increment).IsZero()
}
