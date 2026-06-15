package testsuite

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNautilusQualityGateDefinesReviewRequirements(t *testing.T) {
	content, err := os.ReadFile("../docs/parity/nautilus-complete-quality-gate.json")
	require.NoError(t, err)

	var gate struct {
		GoalID                   string   `json:"goalId"`
		RequiredReviewApprovals  []string `json:"requiredReviewApprovals"`
		RequiredVerification     []string `json:"requiredVerification"`
		RequiredReleaseSections  []string `json:"requiredReleaseSections"`
		BlockingResidualRiskTags []string `json:"blockingResidualRiskTags"`
	}
	require.NoError(t, json.Unmarshal(content, &gate))

	require.Equal(t, "go-nautilustrader-complete-replica", gate.GoalID)
	require.Contains(t, gate.RequiredReviewApprovals, "code-reviewer:APPROVE")
	require.Contains(t, gate.RequiredReviewApprovals, "architect:CLEAR")
	require.Contains(t, gate.RequiredVerification, "bash scripts/verify_nautilus_parity.sh")
	require.Contains(t, gate.RequiredVerification, "bash scripts/generate_nautilus_benchmark_report.sh")
	require.Contains(t, gate.RequiredReleaseSections, "Completed Score")
	require.Contains(t, gate.RequiredReleaseSections, "Known Unsupported External Adapters")
	require.Contains(t, gate.RequiredReleaseSections, "Verification Evidence")
	require.Contains(t, gate.BlockingResidualRiskTags, "critical")
}

func TestNautilusReleaseNotesTemplateHasEvidenceSections(t *testing.T) {
	contentBytes, err := os.ReadFile("../docs/parity/nautilus-release-notes-template.md")
	require.NoError(t, err)
	content := string(contentBytes)

	for _, heading := range []string{
		"# Go NautilusTrader Release Notes",
		"## Completed Score",
		"## Verification Evidence",
		"## Benchmark Evidence",
		"## Known Unsupported External Adapters",
		"## Adapter Capability Changes",
		"## Residual Risks",
	} {
		require.Contains(t, content, heading)
	}
	require.Contains(t, content, "scripts/verify_nautilus_parity.sh")
	require.Contains(t, content, "scripts/generate_nautilus_benchmark_report.sh")
}
