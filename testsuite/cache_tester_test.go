package testsuite

import (
	"context"
	"testing"

	"github.com/QuantProcessing/exchanges/cache"
	"github.com/stretchr/testify/require"
)

func TestCacheTesterReportsRuntimeStateCases(t *testing.T) {
	report := NewCacheTester(CacheTesterConfig{Cache: cache.New()}).Run(context.Background(), t)

	require.Equal(t, "cache", report.Suite)
	require.True(t, report.AllPassed(), "cache tester failed: %#v", report.Cases)
	requireCasePassed(t, report, "TC-C01", "Order runtime indexes")
	requireCasePassed(t, report, "TC-C02", "Fill runtime indexes")
	requireCasePassed(t, report, "TC-C03", "Deferred fill storage")
	requireCasePassed(t, report, "TC-C04", "Position runtime indexes")
	requireCasePassed(t, report, "TC-C05", "Account snapshot history")
	requireCasePassed(t, report, "TC-C06", "Market data snapshots")
	requireCasePassed(t, report, "TC-C07", "Residual summary")
	requireCasePassed(t, report, "TC-C08", "Snapshot and purge")
}
