package model

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

type Metadata map[string]string

type InstrumentType string

const (
	InstrumentTypeCurrencyPair InstrumentType = "currency_pair"
	InstrumentTypeCryptoPerp   InstrumentType = "crypto_perp"
	InstrumentTypeCryptoFuture InstrumentType = "crypto_future"
	InstrumentTypeCryptoOption InstrumentType = "crypto_option"
	InstrumentTypeOptionSpread InstrumentType = "option_spread"
	InstrumentTypeBinaryOption InstrumentType = "binary_option"
	InstrumentTypeSynthetic    InstrumentType = "synthetic"
)

type OptionKind string

const (
	OptionKindCall OptionKind = "CALL"
	OptionKindPut  OptionKind = "PUT"
)

type ExerciseStyle string

const (
	ExerciseStyleEuropean ExerciseStyle = "european"
	ExerciseStyleAmerican ExerciseStyle = "american"
)

type SettlementStyle string

const (
	SettlementStyleCash     SettlementStyle = "cash"
	SettlementStylePhysical SettlementStyle = "physical"
)

type Instrument struct {
	ID          InstrumentID
	RawSymbol   string
	Type        InstrumentType
	Base        Currency
	Quote       Currency
	Settle      Currency
	PriceStep   decimal.Decimal
	SizeStep    decimal.Decimal
	PricePrec   int32
	SizePrec    int32
	Multiplier  decimal.Decimal
	MinQty      decimal.Decimal
	MaxQty      decimal.Decimal
	MinNotional Money
	MaxNotional Money
	MakerFee    decimal.Decimal
	TakerFee    decimal.Decimal
	MarginInit  decimal.Decimal
	MarginMaint decimal.Decimal
	Option      *OptionSpec
	Spread      *SpreadSpec
	Metadata    Metadata
	EventTime   time.Time
	InitTime    time.Time
}

type OptionSpec struct {
	Underlying InstrumentID
	Strike     decimal.Decimal
	Kind       OptionKind
	Expiration time.Time
	Exercise   ExerciseStyle
	Settlement SettlementStyle
	IsInverse  bool
	IsQuanto   bool
}

type OptionSeriesID struct {
	Venue      Venue
	Underlying InstrumentID
	Expiration time.Time
	Settle     Currency
}

type SpreadLeg struct {
	InstrumentID InstrumentID
	Ratio        decimal.Decimal
}

type SpreadSpec struct {
	Legs []SpreadLeg
}

func (i Instrument) Validate() error {
	if err := i.ID.Validate(); err != nil {
		return err
	}
	if i.Type == "" {
		return fmt.Errorf("%w: missing type", ErrInvalidInstrument)
	}
	if i.PriceStep.LessThanOrEqual(decimal.Zero) {
		return fmt.Errorf("%w: invalid price step", ErrInvalidInstrument)
	}
	if i.SizeStep.LessThanOrEqual(decimal.Zero) {
		return fmt.Errorf("%w: invalid size step", ErrInvalidInstrument)
	}
	if i.Type == InstrumentTypeCryptoOption && i.Option == nil {
		return fmt.Errorf("%w: crypto option requires option spec", ErrInvalidInstrument)
	}
	if i.Option != nil {
		if err := i.Option.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (o OptionSpec) Validate() error {
	if err := o.Underlying.Validate(); err != nil {
		return err
	}
	if o.Strike.LessThanOrEqual(decimal.Zero) {
		return fmt.Errorf("%w: invalid option strike", ErrInvalidInstrument)
	}
	if o.Kind != OptionKindCall && o.Kind != OptionKindPut {
		return fmt.Errorf("%w: invalid option kind", ErrInvalidInstrument)
	}
	if o.Expiration.IsZero() {
		return fmt.Errorf("%w: missing option expiration", ErrInvalidInstrument)
	}
	return nil
}

func (i Instrument) MakePrice(v decimal.Decimal) (decimal.Decimal, error) {
	if i.PriceStep.LessThanOrEqual(decimal.Zero) || !v.Mod(i.PriceStep).IsZero() {
		return decimal.Zero, fmt.Errorf("%w: invalid price step", ErrInvalidInstrument)
	}
	return v, nil
}

func (i Instrument) MakeQty(v decimal.Decimal) (decimal.Decimal, error) {
	if i.SizeStep.LessThanOrEqual(decimal.Zero) || !v.Mod(i.SizeStep).IsZero() {
		return decimal.Zero, fmt.Errorf("%w: invalid size step", ErrInvalidInstrument)
	}
	return v, nil
}
