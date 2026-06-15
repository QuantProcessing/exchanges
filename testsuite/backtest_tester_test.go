package testsuite

import (
	"context"
	"testing"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/stretchr/testify/require"
)

func TestBacktestTesterReportsParityCaseResults(t *testing.T) {
	tester := NewBacktestTester(BacktestTesterConfig{
		InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
	})

	report := tester.Run(context.Background(), t)

	require.Equal(t, "backtest", report.Suite)
	require.True(t, report.AllPassed(), "all cases should pass: %#v", report)
	requireCasePassed(t, report, "TC-B01", "Replay market data into strategy")
	requireCasePassed(t, report, "TC-B02", "Match existing orders before strategy callback")
	requireCasePassed(t, report, "TC-B03", "Market fills update portfolio")
	requireCasePassed(t, report, "TC-B04", "Order book liquidity consumption")
	requireCasePassed(t, report, "TC-B05", "Strategy command metadata propagation")
	requireCasePassed(t, report, "TC-B06", "Reusable matching core")
	requireCasePassed(t, report, "TC-B07", "Post-only limit rests before filling")
	requireCasePassed(t, report, "TC-B08", "Reduce-only cannot open a position")
	requireCasePassed(t, report, "TC-B09", "Market-if-touched triggers on favorable touch")
	requireCasePassed(t, report, "TC-B10", "Limit-if-touched triggers then rests as limit")
	requireCasePassed(t, report, "TC-B11", "OUO partial fill reduces linked sibling quantity")
	requireCasePassed(t, report, "TC-B12", "OTO child releases and resizes on partial parent fills")
	requireCasePassed(t, report, "TC-B13", "Deterministic result summary JSON")
	requireCasePassed(t, report, "TC-B14", "Multi-account result summary defaults")
	requireCasePassed(t, report, "TC-B15", "Catalog-backed engine run")
	requireCasePassed(t, report, "TC-B16", "Multi-strategy run preserves strategy metadata")
}

func TestParityScoreboardAggregatesBacktestGate(t *testing.T) {
	report := NewBacktestTester(BacktestTesterConfig{}).Run(context.Background(), t)
	board := NewParityScoreboard(report)

	require.True(t, board.Passed(RequiredCases{
		Suite: "backtest",
		IDs:   []string{"TC-B01", "TC-B02", "TC-B03", "TC-B04", "TC-B05", "TC-B06", "TC-B07", "TC-B08", "TC-B09", "TC-B10", "TC-B11", "TC-B12", "TC-B13", "TC-B14", "TC-B15", "TC-B16"},
	}))
	require.Equal(t, 16, board.Summary(RequiredCases{
		Suite: "backtest",
		IDs:   []string{"TC-B01", "TC-B02", "TC-B03", "TC-B04", "TC-B05", "TC-B06", "TC-B07", "TC-B08", "TC-B09", "TC-B10", "TC-B11", "TC-B12", "TC-B13", "TC-B14", "TC-B15", "TC-B16"},
	}).RequiredPassed)
}
