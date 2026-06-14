package model

import "fmt"

type OrderList struct {
	ID     OrderListID
	Orders []SubmitOrder
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
