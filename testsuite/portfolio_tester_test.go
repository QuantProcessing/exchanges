package testsuite

import (
	"context"
	"testing"

	"github.com/QuantProcessing/exchanges/cache"
	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/portfolio"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestPortfolioTesterReportsPnLAndCommissionCases(t *testing.T) {
	c := cache.New()
	inst := fakeInstrument(model.MustInstrumentID("BTC-USDT-PERP.FAKE"))
	inst.Type = model.InstrumentTypePerp
	inst.Settle = "USDT"
	eth := fakeInstrument(model.MustInstrumentID("ETH-USDT-PERP.FAKE"))
	eth.Type = model.InstrumentTypePerp
	eth.Base = "ETH"
	eth.RawSymbol = "ETHUSDT"
	eth.Settle = "USDT"
	xrate := fakeInstrument(model.MustInstrumentID("USDT-USD-SPOT.FAKE"))
	xrate.RawSymbol = "USDTUSD"
	xrate.Type = model.InstrumentTypeSpot
	xrate.Base = "USDT"
	xrate.Quote = "USD"
	require.NoError(t, c.PutInstrument(inst))
	require.NoError(t, c.PutInstrument(eth))
	require.NoError(t, c.PutInstrument(xrate))
	p := portfolio.New(c)

	report := NewPortfolioTester(PortfolioTesterConfig{
		Portfolio:         p,
		Cache:             c,
		AccountID:         "acct",
		InstrumentID:      inst.ID,
		ShortInstrumentID: eth.ID,
		XRateInstrumentID: xrate.ID,
		MarkPrice:         decimal.RequireFromString("120"),
	}).Run(context.Background(), t)

	require.Equal(t, "portfolio", report.Suite)
	require.True(t, report.Passed(), "all cases should pass: %#v", report)
	requireCasePassed(t, report, "TC-P01", "Apply fills and position")
	requireCasePassed(t, report, "TC-P02", "Realized and unrealized PnL")
	requireCasePassed(t, report, "TC-P03", "Commission by currency")
	requireCasePassed(t, report, "TC-P04", "Market data mark update")
	requireCasePassed(t, report, "TC-P05", "Signed mark values")
	requireCasePassed(t, report, "TC-P06", "Account equity and margins")
	requireCasePassed(t, report, "TC-P07", "Account base currency conversion")
	requireCasePassed(t, report, "TC-P08", "Event-driven execution updates")
	requireCasePassed(t, report, "TC-P09", "Cash and margin accounting paths")
	requireCasePassed(t, report, "TC-P10", "Fill balance deltas")
	requireCasePassed(t, report, "TC-P11", "Unrealized PnL selected prices and marks")
	requireCasePassed(t, report, "TC-P12", "Net position and exposure aggregation")
	requireCasePassed(t, report, "TC-P13", "Explicit conversion hooks")
	requireCasePassed(t, report, "TC-P14", "PnL cache invalidation")
	requireCasePassed(t, report, "TC-P15", "Analyzer closed-trade records")
	requireCasePassed(t, report, "TC-P16", "Leg fills use explicit position IDs")
}
