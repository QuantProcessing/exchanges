package model

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestNewBalanceRequiresSameCurrency(t *testing.T) {
	total := Money{Amount: decimal.NewFromInt(10), Currency: USDT}
	locked := Money{Amount: decimal.NewFromInt(1), Currency: USDC}
	free := Money{Amount: decimal.NewFromInt(9), Currency: USDT}
	_, err := NewBalance(total, locked, free)
	require.ErrorIs(t, err, ErrInvalidMoney)
}

func TestNewBalanceRequiresInvariant(t *testing.T) {
	total := Money{Amount: decimal.NewFromInt(10), Currency: USDT}
	locked := Money{Amount: decimal.NewFromInt(2), Currency: USDT}
	free := Money{Amount: decimal.NewFromInt(7), Currency: USDT}
	_, err := NewBalance(total, locked, free)
	require.ErrorIs(t, err, ErrInvalidAccountState)
}

func TestBalanceFromTotalAndFreeDerivesLocked(t *testing.T) {
	total := Money{Amount: decimal.NewFromInt(10), Currency: USDT}
	free := Money{Amount: decimal.NewFromInt(8), Currency: USDT}
	got, err := BalanceFromTotalAndFree(total, free)
	require.NoError(t, err)
	require.True(t, got.Locked.Amount.Equal(decimal.NewFromInt(2)))
}
