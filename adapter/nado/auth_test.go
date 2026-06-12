package nado

import (
	"context"
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/stretchr/testify/require"
)

func TestPerpPrivateAccessWithoutCredentialsReturnsAuthSentinel(t *testing.T) {
	adp := &Adapter{}

	require.ErrorIs(t, adp.WsOrderConnected(context.Background()), exchanges.ErrAuthFailed)
	_, err := adp.FetchAccount(context.Background())
	require.ErrorIs(t, err, exchanges.ErrAuthFailed)
}

func TestSpotPrivateAccessWithoutCredentialsReturnsAuthSentinel(t *testing.T) {
	adp := &SpotAdapter{}

	require.ErrorIs(t, adp.WsOrderConnected(context.Background()), exchanges.ErrAuthFailed)
	_, err := adp.FetchAccount(context.Background())
	require.ErrorIs(t, err, exchanges.ErrAuthFailed)
}
