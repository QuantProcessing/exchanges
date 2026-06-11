package model

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestInstrumentValidateRequiresOptionSpecForCryptoOption(t *testing.T) {
	inst := Instrument{
		ID:        MustInstrumentID("BTC-20260626-100000-C.BYBIT"),
		RawSymbol: "BTC-26JUN26-100000-C",
		Type:      InstrumentTypeCryptoOption,
		Base:      BTC,
		Quote:     USDC,
		Settle:    USDC,
		PriceStep: decimal.RequireFromString("0.1"),
		SizeStep:  decimal.RequireFromString("0.01"),
	}
	require.ErrorIs(t, inst.Validate(), ErrInvalidInstrument)
}

func TestInstrumentMakeQtyRejectsBadStep(t *testing.T) {
	inst := Instrument{
		ID:        MustInstrumentID("BTC-USDT-PERP.BINANCE"),
		RawSymbol: "BTCUSDT",
		Type:      InstrumentTypeCryptoPerp,
		Base:      BTC,
		Quote:     USDT,
		Settle:    USDT,
		PriceStep: decimal.RequireFromString("0.1"),
		SizeStep:  decimal.RequireFromString("0.001"),
	}
	_, err := inst.MakeQty(decimal.RequireFromString("0.0005"))
	require.ErrorIs(t, err, ErrInvalidInstrument)
}

func TestCryptoOptionInstrumentValidates(t *testing.T) {
	inst := Instrument{
		ID:        MustInstrumentID("BTC-20260626-100000-C.BYBIT"),
		RawSymbol: "BTC-26JUN26-100000-C",
		Type:      InstrumentTypeCryptoOption,
		Base:      BTC,
		Quote:     USDC,
		Settle:    USDC,
		PriceStep: decimal.RequireFromString("0.1"),
		SizeStep:  decimal.RequireFromString("0.01"),
		Option: &OptionSpec{
			Underlying: MustInstrumentID("BTC-USDC-PERP.BYBIT"),
			Strike:     decimal.RequireFromString("100000"),
			Kind:       OptionKindCall,
			Expiration: time.Date(2026, 6, 26, 8, 0, 0, 0, time.UTC),
			Exercise:   ExerciseStyleEuropean,
			Settlement: SettlementStyleCash,
		},
	}
	require.NoError(t, inst.Validate())
}
