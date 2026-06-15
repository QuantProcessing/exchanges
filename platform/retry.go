package platform

import (
	"context"
	"time"
)

type RetryPolicy struct {
	MaxAttempts int
	Backoff     time.Duration
}

func (p RetryPolicy) attempts() int {
	if p.MaxAttempts <= 0 {
		return 1
	}
	return p.MaxAttempts
}

func (n *Node) retryReconnect(ctx context.Context, operation func(context.Context) error) error {
	attempts := n.reconnectPolicy.attempts()
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		if err := operation(ctx); err != nil {
			lastErr = err
			if attempt == attempts-1 {
				return lastErr
			}
			if err := n.waitReconnectBackoff(ctx); err != nil {
				return err
			}
			continue
		}
		return nil
	}
	return lastErr
}

func (n *Node) waitReconnectBackoff(ctx context.Context) error {
	backoff := n.reconnectPolicy.Backoff
	if backoff <= 0 {
		return nil
	}
	timer := time.NewTimer(backoff)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
