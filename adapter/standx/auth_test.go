package standx

import (
	"context"
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/stretchr/testify/require"
)

func TestPrivatePathsWithoutCredentialsReturnAuthSentinel(t *testing.T) {
	adp := &Adapter{}

	require.ErrorIs(t, adp.WsAccountConnected(context.Background()), exchanges.ErrAuthFailed)
	require.ErrorIs(t, adp.WsOrderConnected(context.Background()), exchanges.ErrAuthFailed)
	require.ErrorIs(t, adp.WatchFills(context.Background(), nil), exchanges.ErrAuthFailed)
	_, err := adp.FetchAccount(context.Background())
	require.ErrorIs(t, err, exchanges.ErrAuthFailed)
	_, err = adp.FetchOpenOrders(context.Background(), "BTC")
	require.ErrorIs(t, err, exchanges.ErrAuthFailed)
	require.ErrorIs(t, adp.SetLeverage(context.Background(), "BTC", 2), exchanges.ErrAuthFailed)
}
