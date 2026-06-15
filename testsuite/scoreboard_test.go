package testsuite

import (
	"testing"

	"github.com/QuantProcessing/exchanges/venue"
	"github.com/stretchr/testify/require"
)

func TestParityScoreboardCountsAndRequiresCases(t *testing.T) {
	board := NewParityScoreboard(
		ContractReport{Suite: "execution", Cases: []CaseResult{
			{ID: "TC-E01", Status: CasePassed},
			{ID: "TC-E04", Status: CaseSkipped, Error: "not supported"},
		}},
		ContractReport{Suite: "risk", Cases: []CaseResult{
			{ID: "TC-R01", Status: CasePassed},
		}},
	)

	require.False(t, board.Passed(RequiredCases{Suite: "execution", IDs: []string{"TC-E01", "TC-E04"}}))
	require.Equal(t, []string{"execution:TC-E04"}, board.MissingRequired(
		RequiredCases{Suite: "execution", IDs: []string{"TC-E01", "TC-E04"}},
	))

	summary := board.Summary(RequiredCases{Suite: "execution", IDs: []string{"TC-E01", "TC-E04"}})
	require.Equal(t, 2, summary.Suites)
	require.Equal(t, 3, summary.Cases)
	require.Equal(t, 2, summary.Passed)
	require.Equal(t, 1, summary.Skipped)
	require.Equal(t, 2, summary.Required)
	require.Equal(t, 1, summary.RequiredPassed)
}

func TestParityScoreboardPassesWhenRequiredCasesPass(t *testing.T) {
	board := NewParityScoreboard(ContractReport{Suite: "lifecycle", Cases: []CaseResult{
		{ID: "TC-L01", Status: CasePassed},
		{ID: "TC-L02", Status: CasePassed},
	}})

	require.True(t, board.Passed(RequiredCases{Suite: "lifecycle", IDs: []string{"TC-L01", "TC-L02"}}))
	require.Empty(t, board.MissingRequired(RequiredCases{Suite: "lifecycle", IDs: []string{"TC-L01"}}))
}

func TestNautilusParityRequirementsCoverTargetSuites(t *testing.T) {
	requirements := NautilusParityRequirements()
	suites := make(map[string]int, len(requirements))
	ids := make(map[string][]string, len(requirements))
	for _, req := range requirements {
		suites[req.Suite] = len(req.IDs)
		ids[req.Suite] = req.IDs
	}

	require.Equal(t, 8, suites["data"])
	require.Equal(t, 7, suites["data-engine"])
	require.Equal(t, 9, suites["execution"])
	require.Equal(t, 33, suites["execution-engine"])
	require.Equal(t, 9, suites["reconciliation"])
	require.Equal(t, 9, suites["strategy"])
	require.Equal(t, 7, suites["adapter"])
	require.Equal(t, 4, suites["lifecycle"])
	require.Equal(t, 16, suites["backtest"])
	require.Equal(t, 16, suites["portfolio"])
	require.Equal(t, 13, suites["risk"])
	require.Equal(t, 9, suites["live"])
	require.Equal(t, requiredDataCaseIDs(venue.DeclaredCapabilities{}), ids["data"])
	require.Equal(t, []string{"TC-DE01", "TC-DE02", "TC-DE03", "TC-DE04", "TC-DE05", "TC-DE06", "TC-DE07"}, ids["data-engine"])
	require.Equal(t, requiredExecutionCaseIDs(venue.DeclaredCapabilities{}, true), ids["execution"])
	require.Equal(t, []string{"TC-EXENG01", "TC-EXENG02", "TC-EXENG03", "TC-EXENG04", "TC-EXENG05", "TC-EXENG06", "TC-EXENG07", "TC-EXENG08", "TC-EXENG09", "TC-EXENG10", "TC-EXENG11", "TC-EXENG12", "TC-EXENG13", "TC-EXENG14", "TC-EXENG15", "TC-EXENG16", "TC-EXENG17", "TC-EXENG18", "TC-EXENG19", "TC-EXENG20", "TC-EXENG21", "TC-EXENG22", "TC-EXENG23", "TC-EXENG24", "TC-EXENG25", "TC-EXENG26", "TC-EXENG27", "TC-EXENG28", "TC-EXENG29", "TC-EXENG30", "TC-EXENG31", "TC-EXENG32", "TC-EXENG33"}, ids["execution-engine"])
	require.Equal(t, []string{"TC-REC01", "TC-REC02", "TC-REC03", "TC-REC04", "TC-REC05", "TC-REC06", "TC-REC07", "TC-REC08", "TC-REC09"}, ids["reconciliation"])
	require.Equal(t, []string{"TC-S01", "TC-S02", "TC-S03", "TC-S04", "TC-S05", "TC-S06", "TC-S07", "TC-S08", "TC-S09"}, ids["strategy"])
	require.Equal(t, []string{"TC-LIVE01", "TC-LIVE02", "TC-LIVE03", "TC-LIVE04", "TC-LIVE05", "TC-LIVE06", "TC-LIVE07", "TC-LIVE08", "TC-LIVE09"}, ids["live"])
}
