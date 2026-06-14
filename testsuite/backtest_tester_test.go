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
}

func TestParityScoreboardAggregatesBacktestGate(t *testing.T) {
	report := NewBacktestTester(BacktestTesterConfig{}).Run(context.Background(), t)
	board := NewParityScoreboard(report)

	require.True(t, board.Passed(RequiredCases{
		Suite: "backtest",
		IDs:   []string{"TC-B01", "TC-B02", "TC-B03", "TC-B04", "TC-B05"},
	}))
	require.Equal(t, 5, board.Summary(RequiredCases{
		Suite: "backtest",
		IDs:   []string{"TC-B01", "TC-B02", "TC-B03", "TC-B04", "TC-B05"},
	}).RequiredPassed)
}
