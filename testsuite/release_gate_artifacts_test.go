package testsuite

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNautilusQualityGateDefinesReviewRequirements(t *testing.T) {
	content, err := os.ReadFile("../docs/parity/complete-quality-gate.json")
	require.NoError(t, err)

	var gate struct {
		GoalID                   string   `json:"goalId"`
		RequiredReviewApprovals  []string `json:"requiredReviewApprovals"`
		RequiredVerification     []string `json:"requiredVerification"`
		RequiredReleaseSections  []string `json:"requiredReleaseSections"`
		BlockingResidualRiskTags []string `json:"blockingResidualRiskTags"`
	}
	require.NoError(t, json.Unmarshal(content, &gate))

	require.Equal(t, "go-trading-platform-complete", gate.GoalID)
	require.Contains(t, gate.RequiredReviewApprovals, "code-reviewer:APPROVE")
	require.Contains(t, gate.RequiredReviewApprovals, "architect:CLEAR")
	require.Contains(t, gate.RequiredVerification, "go vet ./...")
	require.Contains(t, gate.RequiredVerification, "git diff --check")
	require.Contains(t, gate.RequiredReleaseSections, "Completed Score")
	require.Contains(t, gate.RequiredReleaseSections, "Known Unsupported Extension Adapters")
	require.Contains(t, gate.RequiredReleaseSections, "Verification Evidence")
	require.Contains(t, gate.BlockingResidualRiskTags, "critical")
}

func TestNautilusReleaseNotesTemplateHasEvidenceSections(t *testing.T) {
	contentBytes, err := os.ReadFile("../docs/parity/release-notes-template.md")
	require.NoError(t, err)
	content := string(contentBytes)

	for _, heading := range []string{
		"# Trading Platform Release Notes",
		"## Completed Score",
		"## Verification Evidence",
		"## Benchmark Evidence",
		"## Known Unsupported Extension Adapters",
		"## Adapter Capability Changes",
		"## Residual Risks",
	} {
		require.Contains(t, content, heading)
	}
	require.Contains(t, content, "go vet ./...")
	require.Contains(t, content, "git diff --check")
}
