package model

import (
	"fmt"
	"time"
)

type AccountType string

const (
	AccountTypeCash    AccountType = "cash"
	AccountTypeMargin  AccountType = "margin"
	AccountTypeBetting AccountType = "betting"
)

type AccountState struct {
	AccountID    AccountID
	Venue        Venue
	Type         AccountType
	BaseCurrency Currency
	Reported     bool
	Balances     []AccountBalance
	Margins      []MarginBalance
	Positions    []PositionStatusReport
	Metadata     Metadata
	EventID      string
	EventTime    time.Time
	InitTime     time.Time
}

type AccountBalance struct {
	Total  Money
	Locked Money
	Free   Money
}

type MarginBalance struct {
	Initial     Money
	Maintenance Money
	Instrument  *InstrumentID
}

func NewBalance(total, locked, free Money) (AccountBalance, error) {
	if err := total.RequireSameCurrency(locked); err != nil {
		return AccountBalance{}, err
	}
	if err := total.RequireSameCurrency(free); err != nil {
		return AccountBalance{}, err
	}
	if !locked.Amount.Add(free.Amount).Equal(total.Amount) {
		return AccountBalance{}, fmt.Errorf("%w: total must equal locked plus free", ErrInvalidAccountState)
	}
	return AccountBalance{Total: total, Locked: locked, Free: free}, nil
}

func BalanceFromTotalAndFree(total, free Money) (AccountBalance, error) {
	if err := total.RequireSameCurrency(free); err != nil {
		return AccountBalance{}, err
	}
	locked := Money{Amount: total.Amount.Sub(free.Amount), Currency: total.Currency}
	return NewBalance(total, locked, free)
}

func BalanceFromTotalAndLocked(total, locked Money) (AccountBalance, error) {
	if err := total.RequireSameCurrency(locked); err != nil {
		return AccountBalance{}, err
	}
	free := Money{Amount: total.Amount.Sub(locked.Amount), Currency: total.Currency}
	return NewBalance(total, locked, free)
}
