package model

import (
	"fmt"
	"strings"

	"github.com/shopspring/decimal"
)

type Currency string

const (
	USDT Currency = "USDT"
	USDC Currency = "USDC"
	DUSD Currency = "DUSD"
	USD  Currency = "USD"
	BTC  Currency = "BTC"
	ETH  Currency = "ETH"
)

type Money struct {
	Amount   decimal.Decimal
	Currency Currency
}

func NewMoney(amount decimal.Decimal, currency Currency) (Money, error) {
	m := Money{Amount: amount, Currency: Currency(strings.ToUpper(strings.TrimSpace(string(currency))))}
	if err := m.Validate(); err != nil {
		return Money{}, err
	}
	return m, nil
}

func (m Money) Validate() error {
	if strings.TrimSpace(string(m.Currency)) == "" {
		return fmt.Errorf("%w: missing currency", ErrInvalidMoney)
	}
	return nil
}

func (m Money) RequireSameCurrency(other Money) error {
	if err := m.Validate(); err != nil {
		return err
	}
	if err := other.Validate(); err != nil {
		return err
	}
	if m.Currency != other.Currency {
		return fmt.Errorf("%w: %s != %s", ErrInvalidMoney, m.Currency, other.Currency)
	}
	return nil
}
