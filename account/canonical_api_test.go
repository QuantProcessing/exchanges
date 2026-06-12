package account

import (
	"testing"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/stretchr/testify/require"
)

func TestCanonicalAccountAPIUsesPrimaryNames(t *testing.T) {
	cache := NewCache()
	reconciler := NewReconciler(cache)
	acct, err := NewTradingAccount(newFakeExecution(), TradingAccountConfig{
		Cache:      cache,
		Reconciler: reconciler,
	})
	require.NoError(t, err)
	require.NotNil(t, acct)
	require.IsType(t, &OrderTracker{}, reconciler.EnsureFlowForClientID(model.ClientOrderID("cli")))
}
