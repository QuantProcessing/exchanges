package testsuite

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNautilusMasterRequirementsDefineCompleteScorecard(t *testing.T) {
	requirements := NautilusMasterRequirements()
	require.NotEmpty(t, requirements)

	requiredSuites := []string{
		"model",
		"cache",
		"command",
		"strategy",
		"data",
		"execution",
		"reconciliation",
		"risk",
		"portfolio",
		"backtest",
		"live",
		"adapter",
		"documentation",
	}
	seenSuites := make(map[string]NautilusMasterRequirement, len(requirements))
	coveredScenarios := make(map[string]bool)
	totalPoints := 0

	for _, req := range requirements {
		require.NotEmpty(t, req.Suite, "suite")
		require.NotEmpty(t, req.Domain, req.Suite)
		require.NotEmpty(t, req.GoPackage, req.Suite)
		require.NotEmpty(t, req.ReportSource, req.Suite)
		require.NotEmpty(t, req.CaseIDs, req.Suite)
		require.NotContains(t, seenSuites, req.Suite, "duplicate suite %s", req.Suite)
		seenSuites[req.Suite] = req
		totalPoints += req.Points
		for _, id := range req.ScenarioIDs {
			coveredScenarios[id] = true
		}
	}

	require.Equal(t, 1000, totalPoints)
	for _, suite := range requiredSuites {
		req, ok := seenSuites[suite]
		require.True(t, ok, "missing suite %s", suite)
		require.NotEmpty(t, req.CaseIDs, "suite %s has no required case IDs", suite)
		require.NotEmpty(t, req.ReportSource, "suite %s has no report source", suite)
	}
	for _, scenario := range []string{"A", "B", "C", "D", "E"} {
		require.True(t, coveredScenarios[scenario], "missing golden scenario %s coverage", scenario)
	}
}

func TestNautilusMasterTesterReportsGateCases(t *testing.T) {
	report := NewNautilusMasterTester().Run(t)

	require.Equal(t, "nautilus-master", report.Suite)
	require.True(t, report.AllPassed(), "master gate report failed: %#v", report.Cases)
	requireCasePassed(t, report, "TC-MASTER01", "Master scorecard totals 1000 points")
	requireCasePassed(t, report, "TC-MASTER02", "Required suites define case IDs")
	requireCasePassed(t, report, "TC-MASTER03", "Required suites define report sources")
	requireCasePassed(t, report, "TC-MASTER04", "Golden scenarios are mapped")
	requireCasePassed(t, report, "TC-MASTER05", "Required case IDs are unique within suites")
	requireCasePassed(t, report, "TC-MASTER06", "Master gate rejects missing failed or skipped required cases")
}

func TestNautilusMasterGateFailsMissingFailedOrSkippedRequiredCases(t *testing.T) {
	required := []RequiredCases{{Suite: "reconciliation", IDs: []string{"TC-REC01", "TC-REC02", "TC-REC03", "TC-REC04"}}}
	report := ContractReport{Suite: "reconciliation", Cases: []CaseResult{
		{ID: "TC-REC01", Status: CasePassed},
		{ID: "TC-REC02", Status: CaseFailed, Error: "boom"},
		{ID: "TC-REC03", Status: CaseSkipped, Error: "unsupported"},
	}}

	err := NautilusMasterGateWithRequirements(required, report)
	require.Error(t, err)
	require.Contains(t, err.Error(), "reconciliation:TC-REC02")
	require.Contains(t, err.Error(), "reconciliation:TC-REC03")
	require.Contains(t, err.Error(), "reconciliation:TC-REC04")
}

func TestNautilusMasterGatePassesWhenRequiredCasesPass(t *testing.T) {
	required := []RequiredCases{{Suite: "reconciliation", IDs: []string{"TC-REC01", "TC-REC02"}}}
	report := ContractReport{Suite: "reconciliation", Cases: []CaseResult{
		{ID: "TC-REC01", Status: CasePassed},
		{ID: "TC-REC02", Status: CasePassed},
	}}

	require.NoError(t, NautilusMasterGateWithRequirements(required, report))
}

func TestNautilusMasterDocsHaveMatrixOwners(t *testing.T) {
	featureMatrix := mustReadMatrix(t, "../docs/parity/complete-feature-matrix.md")
	adapterMatrix := mustReadMatrix(t, "../docs/parity/adapter-capability-matrix.md")

	for _, content := range []string{featureMatrix, adapterMatrix} {
		lower := strings.ToLower(content)
		require.NotContains(t, lower, "unknown owner")
		require.NotContains(t, lower, "tbd")
	}

	for _, surface := range []string{
		"model",
		"execution",
		"live",
		"portfolio",
		"adapters/binance",
		"adapters/interactive_brokers",
	} {
		require.Contains(t, featureMatrix, surface)
	}

	for _, adapter := range []string{
		"Binance Spot",
		"Binance Perp",
		"Aster Spot",
		"Aster Perp",
		"OKX",
		"Bybit",
		"Bitget",
		"Hyperliquid Spot",
		"Hyperliquid Perp",
		"Lighter",
		"Nado",
		"EdgeX",
		"GRVT",
		"StandX",
		"Backpack",
	} {
		require.Contains(t, adapterMatrix, adapter)
	}
}

func mustReadMatrix(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(content)
}
