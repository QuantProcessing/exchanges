package model

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

type Venue string

const (
	VenueBinance Venue = "BINANCE"
	VenueOKX     Venue = "OKX"
	VenueBybit   Venue = "BYBIT"
)

type AccountID string
type ClientOrderID string
type OrderID string
type PositionID string
type TradeID string

var clientOrderIDCounter atomic.Uint64

type InstrumentID struct {
	Symbol string
	Venue  Venue
}

func ParseInstrumentID(s string) (InstrumentID, error) {
	s = strings.TrimSpace(s)
	parts := strings.Split(s, ".")
	if len(parts) != 2 {
		return InstrumentID{}, fmt.Errorf("%w: %q", ErrInvalidInstrumentID, s)
	}
	id := InstrumentID{
		Symbol: strings.ToUpper(strings.TrimSpace(parts[0])),
		Venue:  Venue(strings.ToUpper(strings.TrimSpace(parts[1]))),
	}
	if err := id.Validate(); err != nil {
		return InstrumentID{}, err
	}
	return id, nil
}

func MustInstrumentID(s string) InstrumentID {
	id, err := ParseInstrumentID(s)
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
		return fmt.Errorf("%w: symbol contains venue separator: %q", ErrInvalidInstrumentID, id.String())
	}
	return nil
}

func NewClientOrderID() ClientOrderID {
	n := clientOrderIDCounter.Add(1)
	return ClientOrderID(fmt.Sprintf("cli_%x_%x", time.Now().UnixNano(), n))
}
