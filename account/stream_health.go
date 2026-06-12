package account

import "time"

// StreamName identifies an account runtime stream or event source.
type StreamName string

const (
	StreamOrders    StreamName = "orders"
	StreamFills     StreamName = "fills"
	StreamPositions StreamName = "positions"
	StreamBalances  StreamName = "balances"
)

// StreamStatus describes the current support/health state of a stream.
type StreamStatus string

const (
	StreamStatusUnknown     StreamStatus = "unknown"
	StreamStatusStarting    StreamStatus = "starting"
	StreamStatusReady       StreamStatus = "ready"
	StreamStatusUnsupported StreamStatus = "unsupported"
	StreamStatusError       StreamStatus = "error"
	StreamStatusStopped     StreamStatus = "stopped"
)

// StreamHealth is a copyable snapshot of one runtime stream's state.
type StreamHealth struct {
	Name          StreamName   `json:"name"`
	Status        StreamStatus `json:"status"`
	Supported     bool         `json:"supported"`
	Ready         bool         `json:"ready"`
	Events        uint64       `json:"events"`
	DroppedEvents uint64       `json:"dropped_events"`
	LastEventAt   time.Time    `json:"last_event_at,omitempty"`
	LastError     string       `json:"last_error,omitempty"`
	LastErrorAt   time.Time    `json:"last_error_at,omitempty"`
}

// TradingAccountHealth is a copyable snapshot of account runtime health.
type TradingAccountHealth struct {
	Started        bool                        `json:"started"`
	Starting       bool                        `json:"starting"`
	Closing        bool                        `json:"closing"`
	SnapshotLoaded bool                        `json:"snapshot_loaded"`
	LastSnapshotAt time.Time                   `json:"last_snapshot_at,omitempty"`
	Streams        map[StreamName]StreamHealth `json:"streams"`
}

func initialStreamHealth() map[StreamName]StreamHealth {
	return map[StreamName]StreamHealth{
		StreamOrders:    {Name: StreamOrders, Status: StreamStatusUnknown, Supported: true},
		StreamFills:     {Name: StreamFills, Status: StreamStatusUnknown, Supported: true},
		StreamPositions: {Name: StreamPositions, Status: StreamStatusUnknown, Supported: true},
		StreamBalances:  {Name: StreamBalances, Status: StreamStatusUnknown, Supported: true},
	}
}

func copyStreamHealthMap(in map[StreamName]StreamHealth) map[StreamName]StreamHealth {
	out := make(map[StreamName]StreamHealth, len(in))
	for name, health := range in {
		out[name] = health
	}
	return out
}
