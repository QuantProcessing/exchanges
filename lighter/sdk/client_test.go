package lighter

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClientInvalidateNonceResetsCache(t *testing.T) {
	client := &Client{
		nonce:     123,
		nonceInit: true,
	}

	client.InvalidateNonce()

	require.False(t, client.nonceInit)
	require.Equal(t, int64(123), client.nonce)
}
