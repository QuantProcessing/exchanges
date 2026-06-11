package edgex

import (
	"context"
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/stretchr/testify/require"
)

func TestEdgeXMissingCredentialsUseAuthSentinel(t *testing.T) {
	adp := &Adapter{}

	require.ErrorIs(t, adp.WsAccountConnected(context.Background()), exchanges.ErrAuthFailed)
}

func TestEdgeXUnsupportedPathsUseSentinelErrors(t *testing.T) {
	adp := &Adapter{}

	_, err := adp.ModifyOrder(context.Background(), "1", "BTC", &exchanges.ModifyOrderParams{})
	require.ErrorIs(t, err, exchanges.ErrNotSupported)
}
