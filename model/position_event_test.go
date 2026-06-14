package model

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestPositionLifecycleEventValidate(t *testing.T) {
	event := PositionLifecycleEvent{
		AccountID:    "acct",
		InstrumentID: MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
		PositionID:   "BTC-USDT-SPOT.BINANCE",
		Kind:         PositionEventOpened,
		PreviousSide: PositionSideFlat,
		Side:         PositionSideLong,
		Quantity:     decimal.RequireFromString("1"),
	}

	require.NoError(t, event.Validate())

	event.Kind = PositionEventClosed
	require.ErrorIs(t, event.Validate(), ErrInvalidOrder)
}

func TestNewPositionLifecycleEventClassifiesPositionChanges(t *testing.T) {
	instID := MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	opened, ok := NewPositionLifecycleEvent(nil, PositionStatusReport{
		AccountID:    "acct",
		InstrumentID: instID,
		PositionID:   "pos-1",
		Side:         PositionSideLong,
		Quantity:     decimal.RequireFromString("1"),
	})
	require.True(t, ok)
	require.Equal(t, PositionEventOpened, opened.Kind)

	changed, ok := NewPositionLifecycleEvent(&PositionStatusReport{
		AccountID:    "acct",
		InstrumentID: instID,
		PositionID:   "pos-1",
		Side:         PositionSideLong,
		Quantity:     decimal.RequireFromString("1"),
	}, PositionStatusReport{
		AccountID:    "acct",
		InstrumentID: instID,
		PositionID:   "pos-1",
		Side:         PositionSideLong,
		Quantity:     decimal.RequireFromString("1.5"),
	})
	require.True(t, ok)
	require.Equal(t, PositionEventChanged, changed.Kind)

	closed, ok := NewPositionLifecycleEvent(&PositionStatusReport{
		AccountID:    "acct",
		InstrumentID: instID,
		PositionID:   "pos-1",
		Side:         PositionSideLong,
		Quantity:     decimal.RequireFromString("1"),
	}, PositionStatusReport{
		AccountID:    "acct",
		InstrumentID: instID,
		PositionID:   "pos-1",
		Side:         PositionSideFlat,
		Quantity:     decimal.Zero,
	})
	require.True(t, ok)
	require.Equal(t, PositionEventClosed, closed.Kind)
}
