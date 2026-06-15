package model

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestInstrumentTaxonomyCoversCoreAndExtensionTypes(t *testing.T) {
	expected := []InstrumentType{
		InstrumentTypeSpot,
		InstrumentTypePerp,
		InstrumentTypeFuture,
		InstrumentTypeOption,
		InstrumentTypeSpread,
		InstrumentTypeSynthetic,
		InstrumentTypeIndex,
		InstrumentTypeEquity,
		InstrumentTypeBetting,
	}

	for _, typ := range expected {
		require.NoError(t, typ.Validate(), typ)
	}
	require.ErrorIs(t, InstrumentType("unknown").Validate(), ErrInvalidInstrument)
}

func TestInstrumentValidateAllowsFutureSafeNonCurrencyProducts(t *testing.T) {
	tests := []Instrument{
		{
			ID:        MustInstrumentID("SPX.IDX"),
			RawSymbol: "SPX",
			Type:      InstrumentTypeIndex,
			PriceTick: decimal.RequireFromString("0.01"),
			SizeTick:  decimal.RequireFromString("1"),
			Status:    InstrumentStatusTrading,
		},
		{
			ID:        MustInstrumentID("AAPL.NASDAQ"),
			RawSymbol: "AAPL",
			Type:      InstrumentTypeEquity,
			Base:      "AAPL",
			Quote:     "USD",
			PriceTick: decimal.RequireFromString("0.01"),
			SizeTick:  decimal.RequireFromString("1"),
			Status:    InstrumentStatusTrading,
		},
		{
			ID:        MustInstrumentID("ELECTION-2028.POLYMARKET"),
			RawSymbol: "ELECTION-2028",
			Type:      InstrumentTypeBetting,
			Quote:     "USDC",
			PriceTick: decimal.RequireFromString("0.001"),
			SizeTick:  decimal.RequireFromString("1"),
			Status:    InstrumentStatusTrading,
		},
		{
			ID:        MustInstrumentID("BTC-ETH-SPREAD.SIM"),
			RawSymbol: "BTC-ETH-SPREAD",
			Type:      InstrumentTypeSpread,
			Quote:     "USDT",
			PriceTick: decimal.RequireFromString("0.01"),
			SizeTick:  decimal.RequireFromString("0.001"),
			Status:    InstrumentStatusTrading,
		},
		{
			ID:        MustInstrumentID("BTC-INDEX.SIM"),
			RawSymbol: "BTC-INDEX",
			Type:      InstrumentTypeSynthetic,
			Quote:     "USDT",
			PriceTick: decimal.RequireFromString("0.01"),
			SizeTick:  decimal.RequireFromString("0.001"),
			Status:    InstrumentStatusTrading,
		},
	}

	for _, inst := range tests {
		require.NoError(t, inst.Validate(), inst.ID)
	}
}
