package testsuite

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDataEngineTesterReportsParityCases(t *testing.T) {
	report := NewDataEngineTester(DataEngineTesterConfig{}).Run(context.Background(), t)

	require.Equal(t, "data-engine", report.Suite)
	require.True(t, report.AllPassed(), "data engine tester failed: %#v", report.Cases)
	requireCasePassed(t, report, "TC-DE01", "Engine replays idempotent subscriptions on restart")
	requireCasePassed(t, report, "TC-DE02", "Engine forwards and caches live stream events")
	requireCasePassed(t, report, "TC-DE03", "Catalog requests preserve correlation metadata")
	requireCasePassed(t, report, "TC-DE04", "Trade ticks aggregate into bars")
	requireCasePassed(t, report, "TC-DE05", "Health reports clients subscriptions events and errors")
	requireCasePassed(t, report, "TC-DE06", "Engine reconnects closed streams and replays subscriptions")
	requireCasePassed(t, report, "TC-DE07", "Health marks stale clients")
}
