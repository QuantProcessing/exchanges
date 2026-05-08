package account

import (
	"time"
)

// StreamName identifies a TradingAccount runtime stream.
type StreamName string

const (
	StreamOrders    StreamName = "orders"
	StreamFills     StreamName = "fills"
	StreamPositions StreamName = "positions"
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
		StreamOrders: {
			Name:      StreamOrders,
			Status:    StreamStatusUnknown,
			Supported: true,
		},
		StreamFills: {
			Name:      StreamFills,
			Status:    StreamStatusUnknown,
			Supported: true,
		},
		StreamPositions: {
			Name:      StreamPositions,
			Status:    StreamStatusUnknown,
			Supported: true,
		},
	}
}

func copyStreamHealthMap(in map[StreamName]StreamHealth) map[StreamName]StreamHealth {
	out := make(map[StreamName]StreamHealth, len(in))
	for name, health := range in {
		out[name] = health
	}
	return out
}

func (a *TradingAccount) resetHealthForStart() {
	a.healthMu.Lock()
	defer a.healthMu.Unlock()

	a.snapshotLoaded = false
	a.lastSnapshotAt = time.Time{}
	a.streams = initialStreamHealth()
	for name, health := range a.streams {
		health.Status = StreamStatusStarting
		a.streams[name] = health
	}
}

func (a *TradingAccount) markSnapshotLoaded() {
	a.healthMu.Lock()
	defer a.healthMu.Unlock()
	a.snapshotLoaded = true
	a.lastSnapshotAt = time.Now()
}

func (a *TradingAccount) markStreamReady(name StreamName) {
	a.updateStreamHealth(name, func(health StreamHealth) StreamHealth {
		health.Status = StreamStatusReady
		health.Supported = true
		health.Ready = true
		health.LastError = ""
		health.LastErrorAt = time.Time{}
		return health
	})
}

func (a *TradingAccount) markStreamUnsupported(name StreamName, err error) {
	a.updateStreamHealth(name, func(health StreamHealth) StreamHealth {
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

func (a *TradingAccount) markStreamError(name StreamName, err error) {
	a.updateStreamHealth(name, func(health StreamHealth) StreamHealth {
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

func (a *TradingAccount) markStreamsStopped() {
	a.healthMu.Lock()
	defer a.healthMu.Unlock()
	if a.streams == nil {
		a.streams = initialStreamHealth()
	}
	for name, health := range a.streams {
		health.Status = StreamStatusStopped
		health.Ready = false
		a.streams[name] = health
	}
}

func (a *TradingAccount) markStreamEvent(name StreamName, dropped uint64) {
	a.updateStreamHealth(name, func(health StreamHealth) StreamHealth {
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

func (a *TradingAccount) updateStreamHealth(name StreamName, update func(StreamHealth) StreamHealth) {
	a.healthMu.Lock()
	defer a.healthMu.Unlock()
	if a.streams == nil {
		a.streams = initialStreamHealth()
	}
	health, ok := a.streams[name]
	if !ok {
		health = StreamHealth{Name: name, Status: StreamStatusUnknown, Supported: true}
	}
	a.streams[name] = update(health)
}

// Health returns a copy of the current account runtime health snapshot.
func (a *TradingAccount) Health() TradingAccountHealth {
	a.runMu.RLock()
	started := a.started
	starting := a.starting
	closing := a.closing
	a.runMu.RUnlock()

	a.healthMu.RLock()
	defer a.healthMu.RUnlock()
	streams := a.streams
	if streams == nil {
		streams = initialStreamHealth()
	}
	return TradingAccountHealth{
		Started:        started,
		Starting:       starting,
		Closing:        closing,
		SnapshotLoaded: a.snapshotLoaded,
		LastSnapshotAt: a.lastSnapshotAt,
		Streams:        copyStreamHealthMap(streams),
	}
}
