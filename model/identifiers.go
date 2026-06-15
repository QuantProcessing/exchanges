package model

import (
	"fmt"
	"strings"
)

type Venue string
type Currency string
type AccountID string
type ClientOrderID string
type OrderListID string
type OrderID string
type VenueOrderID string
type TradeID string
type PositionID string
type VenuePositionID string
type ExecAlgorithmID string
type ExecSpawnID string
type ComponentID string

type InstrumentID struct {
	Symbol string
	Venue  Venue
}

func ParseInstrumentID(raw string) (InstrumentID, error) {
	raw = strings.TrimSpace(raw)
	symbol, venue, ok := strings.Cut(raw, ".")
	if !ok {
		return InstrumentID{}, fmt.Errorf("%w: %q", ErrInvalidInstrumentID, raw)
	}
	id := InstrumentID{
		Symbol: strings.ToUpper(strings.TrimSpace(symbol)),
		Venue:  Venue(strings.ToUpper(strings.TrimSpace(venue))),
	}
	if err := id.Validate(); err != nil {
		return InstrumentID{}, err
	}
	return id, nil
}

func MustInstrumentID(raw string) InstrumentID {
	id, err := ParseInstrumentID(raw)
	if err != nil {
		panic(err)
	}
	return id
}

func (id InstrumentID) String() string {
	if id.Symbol == "" && id.Venue == "" {
		return ""
	}
	return id.Symbol + "." + string(id.Venue)
}

func (id InstrumentID) Validate() error {
	if strings.TrimSpace(id.Symbol) == "" || strings.TrimSpace(string(id.Venue)) == "" {
		return fmt.Errorf("%w: %q", ErrInvalidInstrumentID, id.String())
	}
	if strings.Contains(id.Symbol, ".") {
		return fmt.Errorf("%w: %q", ErrInvalidInstrumentID, id.String())
	}
	return nil
}

func (id InstrumentID) IsSynthetic() bool {
	return id.Venue == Venue("SYNTH")
}
