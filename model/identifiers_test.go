package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseInstrumentID(t *testing.T) {
	got, err := ParseInstrumentID("BTC-USDT-PERP.BINANCE")
	require.NoError(t, err)
	require.Equal(t, InstrumentID{Symbol: "BTC-USDT-PERP", Venue: VenueBinance}, got)
	require.Equal(t, "BTC-USDT-PERP.BINANCE", got.String())
}

func TestParseInstrumentIDRejectsMissingVenue(t *testing.T) {
	_, err := ParseInstrumentID("BTC-USDT-PERP")
	require.ErrorIs(t, err, ErrInvalidInstrumentID)
}

func TestParseInstrumentIDRejectsEmptySymbol(t *testing.T) {
	_, err := ParseInstrumentID(".BINANCE")
	require.ErrorIs(t, err, ErrInvalidInstrumentID)
}

func TestInstrumentIDValidateRejectsEmptyVenue(t *testing.T) {
	err := (InstrumentID{Symbol: "BTC-USDT-PERP"}).Validate()
	require.ErrorIs(t, err, ErrInvalidInstrumentID)
}
