package testsuite

import (
	"context"
	"testing"
	"time"

	"github.com/QuantProcessing/exchanges/account"
	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/stretchr/testify/require"
)

type V2LifecycleSuiteConfig struct {
	Execution   venue.ExecutionClient
	Instruments []model.InstrumentID
}

func RunV2LifecycleSuite(t *testing.T, cfg V2LifecycleSuiteConfig) {
	t.Helper()
	require.NotNil(t, cfg.Execution, "Execution is required")

	acct, err := account.NewV2TradingAccount(cfg.Execution, account.V2TradingAccountConfig{
		Instruments: cfg.Instruments,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		require.NoError(t, acct.Stop(stopCtx))
	})

	t.Run("StartupReconciliation", func(t *testing.T) {
		require.NoError(t, acct.Start(context.Background()))
		require.True(t, acct.Ready())
		health := acct.Health()
		require.True(t, health.SnapshotLoaded)
		require.Equal(t, account.StreamStatusReady, health.Streams[account.StreamBalances].Status)
	})
}

func AssertV2OrderFlowTerminal(t *testing.T, flow *account.V2OrderFlow, want model.OrderStatus) {
	t.Helper()
	require.NotNil(t, flow)
	require.Eventually(t, func() bool {
		got, ok := flow.Latest()
		return ok && got.Status == want
	}, time.Second, 10*time.Millisecond)
}
