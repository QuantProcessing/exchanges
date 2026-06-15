package model

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

type InstrumentType string

const (
	InstrumentTypeSpot      InstrumentType = "spot"
	InstrumentTypePerp      InstrumentType = "perp"
	InstrumentTypeFuture    InstrumentType = "future"
	InstrumentTypeOption    InstrumentType = "option"
	InstrumentTypeSpread    InstrumentType = "spread"
	InstrumentTypeSynthetic InstrumentType = "synthetic"
	InstrumentTypeIndex     InstrumentType = "index"
	InstrumentTypeEquity    InstrumentType = "equity"
	InstrumentTypeBetting   InstrumentType = "betting"
)

func (t InstrumentType) Validate() error {
	switch t {
	case InstrumentTypeSpot,
		InstrumentTypePerp,
		InstrumentTypeFuture,
		InstrumentTypeOption,
		InstrumentTypeSpread,
		InstrumentTypeSynthetic,
		InstrumentTypeIndex,
		InstrumentTypeEquity,
		InstrumentTypeBetting:
		return nil
	default:
		return fmt.Errorf("%w: invalid instrument type %q", ErrInvalidInstrument, t)
	}
}

func (t InstrumentType) requiresBaseQuote() bool {
	switch t {
	case InstrumentTypeSpot, InstrumentTypePerp, InstrumentTypeFuture, InstrumentTypeOption:
		return true
	default:
		return false
	}
}

func (t InstrumentType) requiresSettle() bool {
	switch t {
	case InstrumentTypePerp, InstrumentTypeFuture, InstrumentTypeOption:
		return true
	default:
		return false
	}
}

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

type SyntheticInstrument struct {
	ID             InstrumentID
	PricePrecision int
	PriceTick      decimal.Decimal
	Components     []InstrumentID
	Formula        string
	Timestamp      time.Time
	InitTime       time.Time
}

func (s SyntheticInstrument) Validate() error {
	if err := s.ID.Validate(); err != nil {
		return err
	}
	if !s.ID.IsSynthetic() {
		return fmt.Errorf("%w: synthetic instrument requires SYNTH venue", ErrInvalidInstrument)
	}
	if s.PricePrecision < 0 || s.PricePrecision > 9 {
		return fmt.Errorf("%w: invalid synthetic price precision", ErrInvalidInstrument)
	}
	if !s.PriceTick.IsPositive() {
		return fmt.Errorf("%w: non-positive synthetic price tick", ErrInvalidInstrument)
	}
	if len(s.Components) < 2 {
		return fmt.Errorf("%w: synthetic instrument requires at least two components", ErrInvalidInstrument)
	}
	for _, component := range s.Components {
		if err := component.Validate(); err != nil {
			return err
		}
	}
	if s.Formula == "" {
		return fmt.Errorf("%w: missing synthetic formula", ErrInvalidInstrument)
	}
	return nil
}

func (i Instrument) Validate() error {
	if err := i.ID.Validate(); err != nil {
		return err
	}
	if i.RawSymbol == "" || i.Type == "" {
		return fmt.Errorf("%w: missing identity fields", ErrInvalidInstrument)
	}
	if err := i.Type.Validate(); err != nil {
		return err
	}
	if i.Type.requiresBaseQuote() && (i.Base == "" || i.Quote == "") {
		return fmt.Errorf("%w: missing currency fields", ErrInvalidInstrument)
	}
	if i.Type.requiresSettle() && i.Settle == "" {
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
