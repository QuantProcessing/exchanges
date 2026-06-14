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
	require.Equal(t, 9, suites["execution"])
	require.Equal(t, 7, suites["adapter"])
	require.Equal(t, 4, suites["lifecycle"])
	require.Equal(t, 5, suites["backtest"])
	require.Equal(t, 7, suites["portfolio"])
	require.Equal(t, 13, suites["risk"])
	require.Equal(t, requiredDataCaseIDs(venue.DeclaredCapabilities{}), ids["data"])
	require.Equal(t, requiredExecutionCaseIDs(venue.DeclaredCapabilities{}, true), ids["execution"])
}
