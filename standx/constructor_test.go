package standx

import (
	"context"
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/stretchr/testify/require"
)

func TestNewAdapterRejectsInvalidPrivateKey(t *testing.T) {
	_, err := NewAdapter(context.Background(), Options{PrivateKey: "invalid"})
	require.ErrorIs(t, err, exchanges.ErrAuthFailed)
}
