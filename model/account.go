package model

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

type AccountType string

const (
	AccountTypeCash   AccountType = "cash"
	AccountTypeMargin AccountType = "margin"
)

type Balance struct {
	Currency Currency
	Free     string
	Locked   string
	Total    string
}

func (b Balance) Validate() error {
	if b.Currency == "" {
		return fmt.Errorf("%w: missing balance currency", ErrInvalidAccount)
	}
	total, locked, free, err := b.Amounts()
	if err != nil {
		return err
	}
	if total.IsNegative() || locked.IsNegative() || free.IsNegative() {
		return fmt.Errorf("%w: negative balance amount", ErrInvalidAccount)
	}
	if !total.Sub(locked).Equal(free) {
		return fmt.Errorf("%w: total - locked must equal free", ErrInvalidAccount)
	}
	return nil
}

func (b Balance) Amounts() (total decimal.Decimal, locked decimal.Decimal, free decimal.Decimal, err error) {
	total, err = parseOptionalDecimal(b.Total, "total")
	if err != nil {
		return decimal.Zero, decimal.Zero, decimal.Zero, err
	}
	locked, err = parseOptionalDecimal(b.Locked, "locked")
	if err != nil {
		return decimal.Zero, decimal.Zero, decimal.Zero, err
	}
	free, err = parseOptionalDecimal(b.Free, "free")
	if err != nil {
		return decimal.Zero, decimal.Zero, decimal.Zero, err
	}
	if b.Total == "" && (b.Free != "" || b.Locked != "") {
		total = free.Add(locked)
	}
	if b.Locked == "" && b.Total != "" && b.Free != "" {
		locked = total.Sub(free)
	}
	if b.Free == "" && b.Total != "" {
		free = total.Sub(locked)
	}
	return total, locked, free, nil
}

func (b Balance) TotalAmount() (decimal.Decimal, error) {
	total, _, _, err := b.Amounts()
	return total, err
}

func (b Balance) LockedAmount() (decimal.Decimal, error) {
	_, locked, _, err := b.Amounts()
	return locked, err
}

func (b Balance) FreeAmount() (decimal.Decimal, error) {
	_, _, free, err := b.Amounts()
	return free, err
}

type MarginBalance struct {
	Currency     Currency
	InstrumentID InstrumentID
	Initial      string
	Maintenance  string
}

func (m MarginBalance) Validate() error {
	if m.Currency == "" {
		return fmt.Errorf("%w: missing margin currency", ErrInvalidAccount)
	}
	initial, maintenance, err := m.Amounts()
	if err != nil {
		return err
	}
	if initial.IsNegative() || maintenance.IsNegative() {
		return fmt.Errorf("%w: negative margin amount", ErrInvalidAccount)
	}
	return nil
}

func (m MarginBalance) Amounts() (initial decimal.Decimal, maintenance decimal.Decimal, err error) {
	initial, err = parseOptionalDecimal(m.Initial, "initial")
	if err != nil {
		return decimal.Zero, decimal.Zero, err
	}
	maintenance, err = parseOptionalDecimal(m.Maintenance, "maintenance")
	if err != nil {
		return decimal.Zero, decimal.Zero, err
	}
	return initial, maintenance, nil
}

type AccountSnapshot struct {
	AccountID    AccountID
	Venue        Venue
	Type         AccountType
	BaseCurrency Currency
	Balances     []Balance
	Margins      []MarginBalance
	Timestamp    time.Time
}

func (a AccountSnapshot) Validate() error {
	if a.AccountID == "" {
		return ErrInvalidAccount
	}
	switch a.Type {
	case "", AccountTypeCash, AccountTypeMargin:
	default:
		return fmt.Errorf("%w: invalid account type %q", ErrInvalidAccount, a.Type)
	}
	for _, balance := range a.Balances {
		if err := balance.Validate(); err != nil {
			return err
		}
	}
	for _, margin := range a.Margins {
		if err := margin.Validate(); err != nil {
			return err
		}
	}
	return nil
}

type QueryAccount struct {
	AccountID AccountID
}

func (q QueryAccount) Validate() error {
	if q.AccountID == "" {
		return ErrInvalidAccount
	}
	return nil
}

type ExecutionEvent struct {
	Account           *AccountSnapshot
	Order             *OrderStatusReport
	Lifecycle         *OrderLifecycleEvent
	Fill              *FillReport
	Position          *PositionStatusReport
	PositionLifecycle *PositionLifecycleEvent
}

func (e ExecutionEvent) Validate() error {
	count := 0
	if e.Account != nil {
		count++
		if err := e.Account.Validate(); err != nil {
			return err
		}
	}
	if e.Order != nil {
		count++
		if err := e.Order.Validate(); err != nil {
			return err
		}
	}
	if e.Lifecycle != nil {
		count++
		if err := e.Lifecycle.Validate(); err != nil {
			return err
		}
	}
	if e.Fill != nil {
		count++
		if err := e.Fill.Validate(); err != nil {
			return err
		}
	}
	if e.Position != nil {
		count++
		if err := e.Position.Validate(); err != nil {
			return err
		}
	}
	if e.PositionLifecycle != nil {
		count++
		if err := e.PositionLifecycle.Validate(); err != nil {
			return err
		}
	}
	if count != 1 {
		return ErrInvalidExecutionEvent
	}
	return nil
}

func parseOptionalDecimal(value string, name string) (decimal.Decimal, error) {
	if value == "" {
		return decimal.Zero, nil
	}
	amount, err := decimal.NewFromString(value)
	if err != nil {
		return decimal.Zero, fmt.Errorf("%w: invalid %s amount %q", ErrInvalidAccount, name, value)
	}
	return amount, nil
}
