package testsuite

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNautilusParityVerificationScriptRunsRequiredGates(t *testing.T) {
	info, err := os.Stat("../scripts/verify_nautilus_parity.sh")
	require.NoError(t, err)
	require.True(t, info.Mode().Perm()&0o111 != 0, "verification script must be executable")

	contentBytes, err := os.ReadFile("../scripts/verify_nautilus_parity.sh")
	require.NoError(t, err)
	content := string(contentBytes)

	for _, needle := range []string{
		"#!/usr/bin/env bash",
		"set -euo pipefail",
		"go test -count=1 ./testsuite -run 'TestNautilusMaster",
		"go test -count=1 $(env GOCACHE=\"$GOCACHE\" go list ./... | rg -v '/sdk/')",
		"go test -race -count=1 ./model ./cache ./account ./execution ./risk ./portfolio ./platform ./strategy ./backtest ./live ./testsuite",
		"go test -run '^$' -count=1 ./sdk/...",
		"go vet ./...",
		"git diff --check",
	} {
		require.Contains(t, content, needle)
	}
	require.True(t, strings.Contains(content, "GOCACHE=${GOCACHE:-/private/tmp/go-build-exchanges}") ||
		strings.Contains(content, "GOCACHE=\"${GOCACHE:-/private/tmp/go-build-exchanges}\""))
}
