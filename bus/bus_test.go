package bus

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBusPublishesToAllSubscribers(t *testing.T) {
	b := New()
	subA := b.Subscribe("orders", 1)
	subB := b.Subscribe("orders", 1)

	require.NoError(t, b.Publish(context.Background(), "orders", "accepted"))

	require.Equal(t, "accepted", (<-subA.C()).Message)
	require.Equal(t, "accepted", (<-subB.C()).Message)
	require.NoError(t, subA.Close())
	require.NoError(t, subB.Close())
}

func TestBusPublishHonorsContext(t *testing.T) {
	b := New()
	sub := b.Subscribe("orders", 0)
	defer sub.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	<-ctx.Done()

	require.ErrorIs(t, b.Publish(ctx, "orders", "blocked"), context.DeadlineExceeded)
}
