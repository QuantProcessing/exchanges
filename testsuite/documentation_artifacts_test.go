package testsuite

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNautilusDocumentationArtifactsCoverEpic13(t *testing.T) {
	docs := map[string][]string{
		"../docs/guides/master-parity-scorecard.md": {
			"# Master Parity Scorecard",
			"scripts/verify_nautilus_parity.sh",
			"NautilusMasterRequirements",
		},
		"../docs/guides/strategy-authoring-bracket.md": {
			"# Strategy Authoring With Brackets",
			"examples/nautilus_style",
			"SubmitOrderList",
		},
		"../docs/guides/live-node-configuration.md": {
			"# Live Node Configuration",
			"NodeBuilder",
			"shutdown",
		},
		"../docs/guides/reconciliation-states.md": {
			"# Reconciliation States",
			"unresolved discrepancy",
			"AuditTrail",
		},
		"../docs/guides/adapter-capability-policy.md": {
			"# Adapter Capability Policy",
			"Resubscribe",
			"live write",
		},
		"../docs/guides/side-by-side-nautilus-go-examples.md": {
			"# Side-By-Side Nautilus And Go Examples",
			"Bracket Strategy",
			"Portfolio Query",
			"Risk Rejection",
			"Backtest Run",
			"Live Node Assembly",
		},
	}
	for path, needles := range docs {
		contentBytes, err := os.ReadFile(path)
		require.NoError(t, err, path)
		content := string(contentBytes)
		for _, needle := range needles {
			require.Contains(t, content, needle, path)
		}
	}
}
