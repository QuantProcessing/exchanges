package testsuite

import (
	"context"
	"testing"

	"github.com/QuantProcessing/exchanges/cache"
	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/risk"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestRiskTesterReportsOrderSafetyCases(t *testing.T) {
	c := cache.New()
	inst := fakeInstrument(model.MustInstrumentID("BTC-USDT-PERP.FAKE"))
	inst.Type = model.InstrumentTypePerp
	inst.Settle = "USDT"
	eth := fakeInstrument(model.MustInstrumentID("ETH-USDT-PERP.FAKE"))
	eth.Type = model.InstrumentTypePerp
	eth.Base = "ETH"
	eth.RawSymbol = "ETHUSDT"
	eth.Settle = "USDT"
	sol := fakeInstrument(model.MustInstrumentID("SOL-USDT-PERP.FAKE"))
	sol.Type = model.InstrumentTypePerp
	sol.Base = "SOL"
	sol.RawSymbol = "SOLUSDT"
	sol.Settle = "USDT"
	xrp := fakeInstrument(model.MustInstrumentID("XRP-USDT-PERP.FAKE"))
	xrp.Type = model.InstrumentTypePerp
	xrp.Base = "XRP"
	xrp.RawSymbol = "XRPUSDT"
	xrp.Settle = "USDT"
	ada := fakeInstrument(model.MustInstrumentID("ADA-USDT-PERP.FAKE"))
	ada.Type = model.InstrumentTypePerp
	ada.Base = "ADA"
	ada.RawSymbol = "ADAUSDT"
	ada.Settle = "USDT"
	ada.MarginInit = decimal.RequireFromString("0.10")
	ada.MarginMaint = decimal.RequireFromString("0.05")
	dot := fakeInstrument(model.MustInstrumentID("DOT-USDT-PERP.FAKE"))
	dot.Type = model.InstrumentTypePerp
	dot.Base = "DOT"
	dot.RawSymbol = "DOTUSDT"
	dot.Settle = "USDT"
	dot.MarginInit = decimal.RequireFromString("0.10")
	require.NoError(t, c.PutInstrument(inst))
	require.NoError(t, c.PutInstrument(eth))
	require.NoError(t, c.PutInstrument(sol))
	require.NoError(t, c.PutInstrument(xrp))
	marginCache := cache.New()
	require.NoError(t, marginCache.PutInstrument(ada))
	openOrderCache := cache.New()
	require.NoError(t, openOrderCache.PutInstrument(dot))
	baseCurrencyCache := cache.New()
	btcEUR := fakeInstrument(model.MustInstrumentID("BTC-EUR-PERP.FAKE"))
	btcEUR.RawSymbol = "BTCEUR"
	btcEUR.Type = model.InstrumentTypePerp
	btcEUR.Quote = "EUR"
	btcEUR.Settle = "EUR"
	eurUSD := fakeInstrument(model.MustInstrumentID("EUR-USD-SPOT.FAKE"))
	eurUSD.RawSymbol = "EURUSD"
	eurUSD.Type = model.InstrumentTypeSpot
	eurUSD.Base = "EUR"
	eurUSD.Quote = "USD"
	require.NoError(t, baseCurrencyCache.PutInstrument(btcEUR))
	require.NoError(t, baseCurrencyCache.PutInstrument(eurUSD))
	marginCache.PutAccount(model.AccountSnapshot{
		AccountID: "acct",
		Venue:     "FAKE",
		Type:      model.AccountTypeMargin,
		Balances: []model.Balance{{
			Currency: "USDT",
			Free:     "90",
			Locked:   "10",
			Total:    "100",
		}},
		Margins: []model.MarginBalance{{
			Currency:    "USDT",
			Initial:     "20",
			Maintenance: "10",
		}},
	})
	openOrderCache.PutAccount(model.AccountSnapshot{
		AccountID: "acct",
		Venue:     "FAKE",
		Type:      model.AccountTypeMargin,
		Balances: []model.Balance{{
			Currency: "USDT",
			Free:     "100",
			Total:    "100",
		}},
	})
	baseCurrencyCache.PutAccount(model.AccountSnapshot{
		AccountID:    "acct",
		Venue:        "FAKE",
		Type:         model.AccountTypeMargin,
		BaseCurrency: "USD",
	})
	require.NoError(t, openOrderCache.PutOrder(model.OrderStatusReport{
		AccountID:      "acct",
		InstrumentID:   dot.ID,
		OrderID:        "open-dot-buy",
		ClientOrderID:  "open-dot-buy-client",
		Status:         model.OrderStatusAccepted,
		Side:           model.OrderSideBuy,
		Type:           model.OrderTypeLimit,
		Quantity:       decimal.RequireFromString("9"),
		LeavesQuantity: decimal.RequireFromString("9"),
		Price:          decimal.RequireFromString("100"),
	}))
	require.NoError(t, c.PutMarketEvent(model.MarketEvent{Ticker: &model.Ticker{
		InstrumentID: inst.ID,
		Bid:          decimal.RequireFromString("100"),
		Ask:          decimal.RequireFromString("101"),
		Last:         decimal.RequireFromString("100.5"),
	}}))
	require.NoError(t, c.PutMarketEvent(model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: sol.ID,
		BidPrice:     decimal.RequireFromString("100"),
		AskPrice:     decimal.RequireFromString("101"),
		BidSize:      decimal.RequireFromString("2"),
		AskSize:      decimal.RequireFromString("2"),
	}}))
	require.NoError(t, c.PutPosition(model.PositionStatusReport{
		AccountID:    "acct",
		InstrumentID: inst.ID,
		PositionID:   model.PositionID(inst.ID.String()),
		Side:         model.PositionSideLong,
		Quantity:     decimal.RequireFromString("1"),
		EntryPrice:   decimal.RequireFromString("100"),
	}))
	require.NoError(t, c.PutPosition(model.PositionStatusReport{
		AccountID:    "acct",
		InstrumentID: eth.ID,
		PositionID:   model.PositionID(eth.ID.String()),
		Side:         model.PositionSideLong,
		Quantity:     decimal.RequireFromString("1"),
		EntryPrice:   decimal.RequireFromString("200"),
	}))
	require.NoError(t, baseCurrencyCache.PutPosition(model.PositionStatusReport{
		AccountID:    "acct",
		InstrumentID: btcEUR.ID,
		PositionID:   model.PositionID(btcEUR.ID.String()),
		Side:         model.PositionSideLong,
		Quantity:     decimal.RequireFromString("9"),
		EntryPrice:   decimal.RequireFromString("10"),
	}))
	require.NoError(t, baseCurrencyCache.PutMarketEvent(model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: eurUSD.ID,
		BidPrice:     decimal.RequireFromString("1.20"),
		AskPrice:     decimal.RequireFromString("1.20"),
		BidSize:      decimal.RequireFromString("1000"),
		AskSize:      decimal.RequireFromString("1000"),
	}}))

	report := NewRiskTester(RiskTesterConfig{
		Engine: risk.NewEngine(c, risk.Config{
			MaxOrderNotional:    decimal.RequireFromString("1000"),
			MaxPositionNotional: decimal.RequireFromString("200"),
			MaxAccountExposure:  decimal.RequireFromString("350"),
		}),
		OrderNotionalOnlyEngine: risk.NewEngine(c, risk.Config{
			MaxOrderNotional: decimal.RequireFromString("1000"),
		}),
		MarginEngine:             risk.NewEngine(marginCache, risk.Config{}),
		OpenOrderEngine:          risk.NewEngine(openOrderCache, risk.Config{MaxPositionNotional: decimal.RequireFromString("1000")}),
		OpenOrderMarginEngine:    risk.NewEngine(openOrderCache, risk.Config{}),
		BaseCurrencyEngine:       risk.NewEngine(baseCurrencyCache, risk.Config{MaxAccountExposure: decimal.RequireFromString("115")}),
		AccountID:                "acct",
		InstrumentID:             inst.ID,
		QuoteOnlyInstrumentID:    sol.ID,
		UnpricedInstrumentID:     xrp.ID,
		MarginInstrumentID:       ada.ID,
		OpenOrderInstrumentID:    dot.ID,
		BaseCurrencyInstrumentID: btcEUR.ID,
	}).Run(context.Background(), t)

	require.Equal(t, "risk", report.Suite)
	require.True(t, report.Passed(), "all cases should pass: %#v", report)
	requireCasePassed(t, report, "TC-R01", "Precision rejection")
	requireCasePassed(t, report, "TC-R02", "Market notional rejection")
	requireCasePassed(t, report, "TC-R03", "Reduce-only exposure rejection")
	requireCasePassed(t, report, "TC-R04", "Invalid time-in-force rejection")
	requireCasePassed(t, report, "TC-R05", "Projected position notional rejection")
	requireCasePassed(t, report, "TC-R06", "Projected account exposure rejection")
	requireCasePassed(t, report, "TC-R07", "Reduce-only flip rejection")
	requireCasePassed(t, report, "TC-R08", "Quote tick market notional rejection")
	requireCasePassed(t, report, "TC-R09", "Missing market price notional rejection")
	requireCasePassed(t, report, "TC-R10", "Available initial margin rejection")
	requireCasePassed(t, report, "TC-R11", "Open order projected position rejection")
	requireCasePassed(t, report, "TC-R12", "Open order initial margin rejection")
	requireCasePassed(t, report, "TC-R13", "Base currency account exposure rejection")
}
