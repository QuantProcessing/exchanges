package testsuite

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReconciliationTesterReportsMassStatusCase(t *testing.T) {
	report := NewReconciliationTester(ReconciliationTesterConfig{}).Run(context.Background(), t)

	require.Equal(t, "reconciliation", report.Suite)
	require.True(t, report.AllPassed(), "reconciliation tester failed: %#v", report.Cases)
	requireCasePassed(t, report, "TC-REC01", "Reconciler applies mass status reports with audit counters")
	requireCasePassed(t, report, "TC-REC02", "Reconciler applies only missing fills inside lookback")
	requireCasePassed(t, report, "TC-REC03", "Reconciler detects order state and filled quantity discrepancies")
	requireCasePassed(t, report, "TC-REC04", "Engine query falls back to venue-order-id reports")
	requireCasePassed(t, report, "TC-REC05", "Reconciler defers fill-before-order reports and replays them")
	requireCasePassed(t, report, "TC-REC06", "TradingAccount skips recent missing open order repair until threshold")
	requireCasePassed(t, report, "TC-REC07", "Reconciler stops position repair after retry limit with unresolved discrepancies")
	requireCasePassed(t, report, "TC-REC08", "Reconciler imports or explicitly rejects external orders")
	requireCasePassed(t, report, "TC-REC09", "Reconciler audit trail tracks success error and unresolved state")
}
