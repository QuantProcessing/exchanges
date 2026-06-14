package testsuite

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLifecycleTesterReportsNautilusOrderLifecycleCases(t *testing.T) {
	report := NewLifecycleTester(LifecycleTesterConfig{}).Run(context.Background(), t)

	require.Equal(t, "lifecycle", report.Suite)
	require.True(t, report.Passed(), "all cases should pass: %#v", report)
	requireCasePassed(t, report, "TC-L01", "Order event vocabulary")
	requireCasePassed(t, report, "TC-L02", "Allowed order transitions")
	requireCasePassed(t, report, "TC-L03", "Rejected terminal and backward transitions")
	requireCasePassed(t, report, "TC-L04", "Position lifecycle classification")
}
