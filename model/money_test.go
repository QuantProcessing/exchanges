package model

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestNewMoneyRejectsMissingCurrency(t *testing.T) {
	_, err := NewMoney(decimal.NewFromInt(1), "")
	require.ErrorIs(t, err, ErrInvalidMoney)
}

func TestMoneySameCurrency(t *testing.T) {
	a := Money{Amount: decimal.NewFromInt(1), Currency: USDT}
	b := Money{Amount: decimal.NewFromInt(2), Currency: USDT}
	require.NoError(t, a.RequireSameCurrency(b))
}

func TestMoneySameCurrencyRejectsMismatch(t *testing.T) {
	a := Money{Amount: decimal.NewFromInt(1), Currency: USDT}
	b := Money{Amount: decimal.NewFromInt(2), Currency: USDC}
	require.ErrorIs(t, a.RequireSameCurrency(b), ErrInvalidMoney)
}
