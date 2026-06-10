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

func (b *baseTradeClient) resetHealthForStart() {
	b.healthMu.Lock()
	defer b.healthMu.Unlock()

	b.snapshotLoaded = false
	b.lastSnapshotAt = time.Time{}
	b.streams = initialStreamHealth()
	for name, health := range b.streams {
		health.Status = StreamStatusStarting
		b.streams[name] = health
	}
}

func (b *baseTradeClient) markSnapshotLoaded() {
	b.healthMu.Lock()
	defer b.healthMu.Unlock()
	b.snapshotLoaded = true
	b.lastSnapshotAt = time.Now()
}

func (b *baseTradeClient) markStreamReady(name StreamName) {
	b.updateStreamHealth(name, func(health StreamHealth) StreamHealth {
		health.Status = StreamStatusReady
		health.Supported = true
		health.Ready = true
		health.LastError = ""
		health.LastErrorAt = time.Time{}
		return health
	})
}

func (b *baseTradeClient) markStreamUnsupported(name StreamName, err error) {
	b.updateStreamHealth(name, func(health StreamHealth) StreamHealth {
		health.Status = StreamStatusUnsupported
		health.Supported = false
		health.Ready = false
		if err != nil {
			health.LastError = err.Error()
			health.LastErrorAt = time.Now()
		}
		return health
	})
}

func (b *baseTradeClient) markStreamError(name StreamName, err error) {
	b.updateStreamHealth(name, func(health StreamHealth) StreamHealth {
		health.Status = StreamStatusError
		health.Supported = true
		health.Ready = false
		if err != nil {
			health.LastError = err.Error()
			health.LastErrorAt = time.Now()
		}
		return health
	})
}

func (b *baseTradeClient) markStreamsStopped() {
	b.healthMu.Lock()
	defer b.healthMu.Unlock()
	if b.streams == nil {
		b.streams = initialStreamHealth()
	}
	for name, health := range b.streams {
		health.Status = StreamStatusStopped
		health.Ready = false
		b.streams[name] = health
	}
}

func (b *baseTradeClient) markStreamEvent(name StreamName, dropped uint64) {
	b.updateStreamHealth(name, func(health StreamHealth) StreamHealth {
		health.Events++
		health.DroppedEvents += dropped
		health.LastEventAt = time.Now()
		if health.Status == StreamStatusStarting || health.Status == StreamStatusUnknown {
			health.Status = StreamStatusReady
			health.Ready = true
		}
		return health
	})
}

func (b *baseTradeClient) updateStreamHealth(name StreamName, update func(StreamHealth) StreamHealth) {
	b.healthMu.Lock()
	defer b.healthMu.Unlock()
	if b.streams == nil {
		b.streams = initialStreamHealth()
	}
	health, ok := b.streams[name]
	if !ok {
		health = StreamHealth{Name: name, Status: StreamStatusUnknown, Supported: true}
	}
	b.streams[name] = update(health)
}

func (b *baseTradeClient) healthSnapshot() TradingAccountHealth {
	b.runMu.RLock()
	started := b.started
	starting := b.starting
	closing := b.closing
	b.runMu.RUnlock()

	b.healthMu.RLock()
	defer b.healthMu.RUnlock()
	streams := b.streams
	if streams == nil {
		streams = initialStreamHealth()
	}
	return TradingAccountHealth{
		Started:        started,
		Starting:       starting,
		Closing:        closing,
		SnapshotLoaded: b.snapshotLoaded,
		LastSnapshotAt: b.lastSnapshotAt,
		Streams:        copyStreamHealthMap(streams),
	}
}
