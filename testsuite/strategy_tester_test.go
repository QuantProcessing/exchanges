package testsuite

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStrategyTesterReportsRuntimeCases(t *testing.T) {
	report := NewStrategyTester(StrategyTesterConfig{}).Run(context.Background(), t)

	require.Equal(t, "strategy", report.Suite)
	require.True(t, report.AllPassed(), "strategy tester failed: %#v", report.Cases)
	requireCasePassed(t, report, "TC-S01", "Typed market callbacks")
	requireCasePassed(t, report, "TC-S02", "Typed execution callbacks")
	requireCasePassed(t, report, "TC-S03", "Typed timer callbacks")
	requireCasePassed(t, report, "TC-S04", "Typed error callbacks")
	requireCasePassed(t, report, "TC-S05", "Async engine errors are observable")
	requireCasePassed(t, report, "TC-S06", "Strategy config validates and freezes runtime identity")
	requireCasePassed(t, report, "TC-S07", "Built-in indicators initialize and update")
	requireCasePassed(t, report, "TC-S08", "Runtime helpers include data request metadata and strategy-scoped logging")
	requireCasePassed(t, report, "TC-S09", "Strategy actor faults do not skip peers")
}
