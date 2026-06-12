package testsuite

import (
	"context"
	"testing"

	"github.com/QuantProcessing/exchanges/platform"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/stretchr/testify/require"
)

type PlatformContractSuiteConfig struct {
	DataClient      venue.DataClient
	ExecutionClient venue.ExecutionClient
}

func RunPlatformContractSuite(t *testing.T, cfg PlatformContractSuiteConfig) {
	t.Helper()
	node := platform.NewNode(platform.Config{})
	if cfg.DataClient != nil {
		require.NoError(t, node.AddDataClient(cfg.DataClient.ClientID(), cfg.DataClient))
	}
	if cfg.ExecutionClient != nil {
		require.NoError(t, node.AddExecutionClient(string(cfg.ExecutionClient.AccountID()), cfg.ExecutionClient))
	}
	require.NoError(t, node.Stop(context.Background()))
}
