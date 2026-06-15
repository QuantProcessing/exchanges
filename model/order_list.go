package model

import "fmt"

type OrderListKind string

const (
	OrderListKindBulk    OrderListKind = "bulk"
	OrderListKindOTO     OrderListKind = "oto"
	OrderListKindOCO     OrderListKind = "oco"
	OrderListKindOUO     OrderListKind = "ouo"
	OrderListKindBracket OrderListKind = "bracket"
)

type OrderList struct {
	Metadata CommandMetadata
	ID       OrderListID
	Orders   []SubmitOrder
}

func (l OrderList) WithCommandMetadataDefaults() OrderList {
	l.Metadata = l.Metadata.Clone()
	l.Orders = append([]SubmitOrder(nil), l.Orders...)
	for i := range l.Orders {
		l.Orders[i].Metadata = l.Orders[i].Metadata.WithDefaults(l.Metadata)
	}
	return l
}

func (l OrderList) Validate() error {
	if l.ID == "" || len(l.Orders) == 0 {
		return fmt.Errorf("%w: invalid order list", ErrInvalidOrder)
	}
	var venue Venue
	for i, order := range l.Orders {
		if err := order.Validate(); err != nil {
			return err
		}
		if order.OrderListID != l.ID {
			return fmt.Errorf("%w: order %d list id mismatch", ErrInvalidOrder, i)
		}
		if i == 0 {
			venue = order.InstrumentID.Venue
			continue
		}
		if order.InstrumentID.Venue != venue {
			return fmt.Errorf("%w: mixed venue order list", ErrInvalidOrder)
		}
	}
	return nil
}

func (l OrderList) Kind() OrderListKind {
	if l.IsBracket() {
		return OrderListKindBracket
	}
	if len(l.Orders) == 0 {
		return ""
	}
	var kind OrderListKind
	for _, order := range l.Orders {
		if order.Contingency == "" {
			continue
		}
		current := OrderListKind(order.Contingency)
		if kind == "" {
			kind = current
			continue
		}
		if kind != current {
			return OrderListKindBulk
		}
	}
	if kind == "" {
		return OrderListKindBulk
	}
	return kind
}

func (l OrderList) IsBracket() bool {
	if len(l.Orders) != 3 {
		return false
	}
	entry := l.Orders[0]
	if entry.ClientOrderID == "" || entry.ParentClientOrderID != "" || entry.Side.Opposite() == "" {
		return false
	}
	if entry.Contingency != "" && entry.Contingency != ContingencyTypeOTO {
		return false
	}
	exitSide := entry.Side.Opposite()
	for _, child := range l.Orders[1:] {
		if child.ParentClientOrderID != entry.ClientOrderID {
			return false
		}
		if child.Side != exitSide || !child.ReduceOnly {
			return false
		}
		if !child.Quantity.Equal(entry.Quantity) {
			return false
		}
		if child.Contingency != ContingencyTypeOCO && child.Contingency != ContingencyTypeOUO {
			return false
		}
	}
	return true
}

func (l OrderList) UniformInstrument() (InstrumentID, bool) {
	if len(l.Orders) == 0 {
		return InstrumentID{}, false
	}
	instrumentID := l.Orders[0].InstrumentID
	for _, order := range l.Orders[1:] {
		if order.InstrumentID != instrumentID {
			return InstrumentID{}, false
		}
	}
	return instrumentID, true
}

func (l OrderList) Venue() (Venue, bool) {
	if len(l.Orders) == 0 {
		return "", false
	}
	venue := l.Orders[0].InstrumentID.Venue
	if venue == "" {
		return "", false
	}
	for _, order := range l.Orders[1:] {
		if order.InstrumentID.Venue != venue {
			return "", false
		}
	}
	return venue, true
}
