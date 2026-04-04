package exchanges

import (
	"context"
	"time"
)

// LocalOrderBook interface standardizes the output of locally maintained orderbooks.
// This allows the BaseAdapter and upper logic layers to consume depth data
// uniformly, regardless of whether the internal sync uses delta snapshots,
// buffer timestamps, or gapless polling.
//
// Each exchange implements this interface in its own orderbook.go file because
// synchronization protocols differ (Binance: diff+snapshot, Nado: gap detection,
// OKX: checksum validation, etc.).
type LocalOrderBook interface {
	// GetDepth returns the sorted top `limit` depth levels.
	// Bids are sorted descending (highest price first).
	// Asks are sorted ascending (lowest price first).
	GetDepth(limit int) ([]Level, []Level)

	// WaitReady blocks until the orderbook is initialized or the timeout expires.
	WaitReady(ctx context.Context, timeout time.Duration) bool

	// Timestamp returns the Unix millisecond timestamp of the last update.
	Timestamp() int64
}
