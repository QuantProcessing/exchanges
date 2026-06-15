package model

import (
	"fmt"
	"time"
)

type SubmitOrderList struct {
	Metadata  CommandMetadata
	AccountID AccountID
	List      OrderList
}

func (c SubmitOrderList) WithCommandMetadataDefaults() SubmitOrderList {
	c.Metadata = c.Metadata.Clone()
	c.List.Metadata = c.List.Metadata.WithDefaults(c.Metadata)
	c.List = c.List.WithCommandMetadataDefaults()
	return c
}

func (c SubmitOrderList) Validate() error {
	if c.AccountID == "" {
		return ErrInvalidAccount
	}
	if err := c.List.Validate(); err != nil {
		return err
	}
	for i, order := range c.List.Orders {
		if order.AccountID != c.AccountID {
			return fmt.Errorf("%w: order %d account mismatch", ErrInvalidOrder, i)
		}
	}
	return nil
}

type GenerateOrderStatusReports struct {
	Metadata      CommandMetadata
	AccountID     AccountID
	InstrumentID  InstrumentID
	OrderID       OrderID
	VenueOrderID  VenueOrderID
	ClientOrderID ClientOrderID
}

func (c GenerateOrderStatusReports) Validate() error {
	return validateReportCommandScope(c.AccountID, c.InstrumentID)
}

type GenerateFillReports struct {
	Metadata      CommandMetadata
	AccountID     AccountID
	InstrumentID  InstrumentID
	OrderID       OrderID
	VenueOrderID  VenueOrderID
	ClientOrderID ClientOrderID
	StartTradeID  TradeID
}

func (c GenerateFillReports) Validate() error {
	return validateReportCommandScope(c.AccountID, c.InstrumentID)
}

type GeneratePositionStatusReports struct {
	Metadata        CommandMetadata
	AccountID       AccountID
	InstrumentID    InstrumentID
	PositionID      PositionID
	VenuePositionID VenuePositionID
}

func (c GeneratePositionStatusReports) Validate() error {
	return validateReportCommandScope(c.AccountID, c.InstrumentID)
}

type GenerateExecutionMassStatus struct {
	Metadata        CommandMetadata
	AccountID       AccountID
	InstrumentID    InstrumentID
	OrderID         OrderID
	VenueOrderID    VenueOrderID
	ClientOrderID   ClientOrderID
	PositionID      PositionID
	VenuePositionID VenuePositionID
}

func (c GenerateExecutionMassStatus) Validate() error {
	return validateReportCommandScope(c.AccountID, c.InstrumentID)
}

type ExecutionMassStatus struct {
	Metadata  CommandMetadata
	AccountID AccountID
	Venue     Venue
	Accounts  []AccountSnapshot
	Orders    []OrderStatusReport
	Fills     []FillReport
	Positions []PositionStatusReport
	Timestamp time.Time
}

func (s ExecutionMassStatus) Validate() error {
	if s.AccountID == "" {
		return ErrInvalidAccount
	}
	for _, account := range s.Accounts {
		if err := account.Validate(); err != nil {
			return err
		}
		if account.AccountID != s.AccountID {
			return fmt.Errorf("%w: account report account mismatch", ErrInvalidAccount)
		}
		if s.Venue != "" && account.Venue != "" && account.Venue != s.Venue {
			return fmt.Errorf("%w: account report venue mismatch", ErrInvalidAccount)
		}
	}
	for _, order := range s.Orders {
		if err := order.Validate(); err != nil {
			return err
		}
		if order.AccountID != s.AccountID {
			return fmt.Errorf("%w: order report account mismatch", ErrInvalidOrder)
		}
	}
	for _, fill := range s.Fills {
		if err := fill.Validate(); err != nil {
			return err
		}
		if fill.AccountID != s.AccountID {
			return fmt.Errorf("%w: fill report account mismatch", ErrInvalidOrder)
		}
	}
	for _, position := range s.Positions {
		if err := position.Validate(); err != nil {
			return err
		}
		if position.AccountID != s.AccountID {
			return fmt.Errorf("%w: position report account mismatch", ErrInvalidOrder)
		}
	}
	return nil
}

func validateReportCommandScope(accountID AccountID, instrumentID InstrumentID) error {
	if accountID == "" {
		return ErrInvalidAccount
	}
	return instrumentID.Validate()
}
