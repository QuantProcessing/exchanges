package testsuite

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNautilusDocumentationArtifactsCoverEpic13(t *testing.T) {
	docs := map[string][]string{
		"../docs/README.md": {
			"# Documentation",
			"Getting Started",
			"Module Guide",
			"Reliable Quant Program Guide",
		},
		"../docs/getting-started.md": {
			"# Getting Started",
			"Choose The Right Layer",
			"Run The Strategy In A Backtest",
		},
		"../docs/module-guide.md": {
			"# Module Guide",
			"`model`",
			"`platform`",
			"`testsuite`",
		},
		"../docs/runtime-flow.md": {
			"# Runtime Flow",
			"Market Data Flow",
			"Order Flow",
			"Reconciliation Flow",
		},
		"../docs/architecture.md": {
			"# Project Architecture",
			"Strategy Order Flow",
			"Reconciliation Flow",
			"`platform.Node`",
		},
		"../docs/guides/master-scorecard.md": {
			"# Master Scorecard",
			"Complete Feature Matrix",
			"Complete Quality Gate",
		},
		"../docs/guides/quant-use-cases.md": {
			"# Quant Developer Use Cases",
			"Order Book Imbalance Strategy",
			"Deterministic Backtest",
			"Adapter Capability-Aware Live Setup",
		},
		"../docs/guides/reliable-quant-program.md": {
			"# Reliable Quant Program Guide",
			"Create Orders Through `OrderFactory`",
			"Production Rollout Checklist",
		},
		"../docs/guides/strategy-authoring-bracket.md": {
			"# Strategy Authoring With Brackets",
			"SubmitOrderList",
			"Runtime Semantics",
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
			"resubscribe",
			"live write",
		},
		"../docs/guides/workflow-recipes.md": {
			"# Workflow Recipes",
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
