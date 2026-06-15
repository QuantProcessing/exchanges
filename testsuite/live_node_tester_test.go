package testsuite

import (
	"context"
	"testing"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/stretchr/testify/require"
)

func TestLiveNodeTesterReportsParityCaseResults(t *testing.T) {
	tester := NewLiveNodeTester(LiveNodeTesterConfig{
		InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
	})

	report := tester.Run(context.Background(), t)

	require.Equal(t, "live", report.Suite)
	require.True(t, report.AllPassed(), "all cases should pass: %#v", report)
	requireCasePassed(t, report, "TC-LIVE01", "Trading node assembles live runtime dependencies")
	requireCasePassed(t, report, "TC-LIVE02", "Live node start and stop update lifecycle health")
	requireCasePassed(t, report, "TC-LIVE03", "Startup applies strategy market-data subscriptions")
	requireCasePassed(t, report, "TC-LIVE04", "Strategy runtime submits orders through execution client")
	requireCasePassed(t, report, "TC-LIVE05", "Live market-data events reach typed strategy callbacks")
	requireCasePassed(t, report, "TC-LIVE06", "Startup phases complete before strategy start")
	requireCasePassed(t, report, "TC-LIVE07", "Health snapshots include clients and strategies")
	requireCasePassed(t, report, "TC-LIVE08", "Reconnect policy retries data and execution recovery")
	requireCasePassed(t, report, "TC-LIVE09", "Fatal runtime exception triggers graceful shutdown")
}

func TestParityScoreboardAggregatesLiveNodeGate(t *testing.T) {
	report := NewLiveNodeTester(LiveNodeTesterConfig{}).Run(context.Background(), t)
	board := NewParityScoreboard(report)

	require.True(t, board.Passed(RequiredCases{
		Suite: "live",
		IDs:   []string{"TC-LIVE01", "TC-LIVE02", "TC-LIVE03", "TC-LIVE04", "TC-LIVE05", "TC-LIVE06", "TC-LIVE07", "TC-LIVE08", "TC-LIVE09"},
	}))
	require.Equal(t, 9, board.Summary(RequiredCases{
		Suite: "live",
		IDs:   []string{"TC-LIVE01", "TC-LIVE02", "TC-LIVE03", "TC-LIVE04", "TC-LIVE05", "TC-LIVE06", "TC-LIVE07", "TC-LIVE08", "TC-LIVE09"},
	}).RequiredPassed)
}
