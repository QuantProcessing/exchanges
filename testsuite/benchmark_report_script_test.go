package testsuite

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNautilusBenchmarkReportScriptCoversRequiredBaselines(t *testing.T) {
	info, err := os.Stat("../scripts/generate_nautilus_benchmark_report.sh")
	require.NoError(t, err)
	require.True(t, info.Mode().Perm()&0o111 != 0, "benchmark report script must be executable")

	contentBytes, err := os.ReadFile("../scripts/generate_nautilus_benchmark_report.sh")
	require.NoError(t, err)
	content := string(contentBytes)

	for _, needle := range []string{
		"#!/usr/bin/env bash",
		"set -euo pipefail",
		".omx/reports/nautilus-benchmark-report.md",
		"BenchmarkMatchingCoreOrderBookDepth",
		"BenchmarkBusPublishFanout",
		"BenchmarkReconcilerMassStatus",
		"Test.*ClientsPassVenueContractSuite",
		"-benchmem",
		"rg -q",
	} {
		require.Contains(t, content, needle)
	}
}
