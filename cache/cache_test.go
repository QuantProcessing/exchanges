package cache

import (
	"testing"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestCacheStoresInstrumentAndFacadeReturnsSnapshot(t *testing.T) {
	c := New()
	inst := testInstrument()

	require.NoError(t, c.PutInstrument(inst))

	got, ok := c.Instrument(inst.ID)
	require.True(t, ok)
	require.Equal(t, inst.ID, got.ID)

	snapshot := c.Facade().Instruments()
	require.Len(t, snapshot, 1)
	snapshot[0].RawSymbol = "MUTATED"

	got, ok = c.Instrument(inst.ID)
	require.True(t, ok)
	require.Equal(t, "BTCUSDT", got.RawSymbol)
}

func TestCacheStoresReportsByAccountAndOrderKeys(t *testing.T) {
	c := New()
	inst := testInstrument()
	require.NoError(t, c.PutInstrument(inst))

	order := model.OrderStatusReport{
		AccountID:    "acct-1",
		InstrumentID: inst.ID,
		OrderID:      "venue-1",
		ClientID:     "client-1",
		Status:       model.OrderStatusAccepted,
		Side:         model.OrderSideBuy,
		Type:         model.OrderTypeLimit,
		Quantity:     decimal.RequireFromString("0.5"),
		EventTime:    time.Now(),
	}
	require.NoError(t, c.PutOrderStatus(order))

	got, ok := c.OrderByClientID("acct-1", "client-1")
	require.True(t, ok)
	require.Equal(t, order.OrderID, got.OrderID)

	got, ok = c.OrderByOrderID("acct-1", "venue-1")
	require.True(t, ok)
	require.Equal(t, order.ClientID, got.ClientID)
}

func TestCacheStoresFillReportsByTradeID(t *testing.T) {
	c := New()
	fill := model.FillReport{
		AccountID:    "acct-1",
		InstrumentID: testInstrument().ID,
		OrderID:      "venue-1",
		ClientID:     "client-1",
		TradeID:      "trade-1",
		Side:         model.OrderSideBuy,
		Quantity:     decimal.RequireFromString("0.1"),
		Price:        decimal.RequireFromString("65000"),
		EventTime:    time.Now(),
	}
	require.NoError(t, c.PutFill(fill))
	require.NoError(t, c.PutFill(fill))

	fills := c.FillsByOrderID("acct-1", "venue-1")
	require.Len(t, fills, 1)
	require.Equal(t, fill.TradeID, fills[0].TradeID)
}

func TestCacheStoresPositionStatus(t *testing.T) {
	c := New()
	pos := model.PositionStatusReport{
		AccountID:    "acct-1",
		InstrumentID: testInstrument().ID,
		PositionID:   "pos-1",
		Side:         model.PositionSideLong,
		Quantity:     decimal.RequireFromString("0.25"),
		AvgPrice:     decimal.RequireFromString("64000"),
		EventTime:    time.Now(),
	}
	require.NoError(t, c.PutPosition(pos))

	positions := c.Positions("acct-1")
	require.Len(t, positions, 1)
	require.Equal(t, pos.PositionID, positions[0].PositionID)
}

func TestCacheAppliesAccountStateReplacement(t *testing.T) {
	c := New()
	total := model.Money{Amount: decimal.NewFromInt(10), Currency: model.USDT}
	free := model.Money{Amount: decimal.NewFromInt(8), Currency: model.USDT}
	bal, err := model.BalanceFromTotalAndFree(total, free)
	require.NoError(t, err)

	state := model.AccountState{
		AccountID: "acct",
		Venue:     model.VenueBinance,
		Type:      model.AccountTypeMargin,
		Balances:  []model.AccountBalance{bal},
	}
	require.NoError(t, c.ApplyAccountState(state))

	got, ok := c.AccountState(model.VenueBinance, "acct")
	require.True(t, ok)
	require.Len(t, got.Balances, 1)
	require.True(t, got.Balances[0].Free.Amount.Equal(decimal.NewFromInt(8)))
}

func testInstrument() model.Instrument {
	return model.Instrument{
		ID:        model.MustInstrumentID("BTC-USDT-PERP.BINANCE"),
		RawSymbol: "BTCUSDT",
		Type:      model.InstrumentTypeCryptoPerp,
		Base:      model.BTC,
		Quote:     model.USDT,
		Settle:    model.USDT,
		PriceStep: decimal.RequireFromString("0.1"),
		SizeStep:  decimal.RequireFromString("0.001"),
	}
}
