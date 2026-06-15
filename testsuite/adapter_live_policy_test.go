package testsuite

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdapterLiveTestPolicyDocumentsReadAndWriteGates(t *testing.T) {
	contentBytes, err := os.ReadFile("../docs/parity/adapter-live-test-policy.md")
	require.NoError(t, err)
	content := string(contentBytes)

	for _, needle := range []string{
		"# Adapter Live Test Policy",
		"Public live read tests",
		"Live write tests",
		"internal/testenv.RequireLiveCredentials",
		"internal/testenv.RequireLiveWrite",
		"BINANCE_ENABLE_LIVE_WRITE_TESTS",
		"BINANCE_PERP_ENABLE_LIVE_WRITE_TESTS",
		"BYBIT_ENABLE_LIVE_WRITE_TESTS",
		"BITGET_ENABLE_LIVE_WRITE_TESTS",
		"OKX_ENABLE_LIVE_WRITE_TESTS",
		"HYPERLIQUID_ENABLE_LIVE_WRITE_TESTS",
		"LIGHTER_ENABLE_LIVE_WRITE_TESTS",
		"Binance",
		"OKX",
		"Bybit",
		"Bitget",
		"Hyperliquid",
		"Backpack",
		"Aster",
		"Lighter",
		"Nado",
		"EdgeX",
		"GRVT",
		"StandX",
	} {
		require.Contains(t, content, needle)
	}
}
