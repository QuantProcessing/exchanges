package exchanges

import "sync"

// ============================================================================
// EventBus — generic fan-out pub/sub for streaming events
// ============================================================================

// Subscription represents a single subscriber's channel for receiving events.
// Call Unsubscribe() to stop receiving events and release resources.
type Subscription[T any] struct {
	C  <-chan *T     // Read-only channel for the consumer
	ch chan *T       // Internal writable channel
	id uint64       // Unique subscriber ID
	bus *EventBus[T] // Back-reference for unsubscribe
}

// Unsubscribe removes this subscription and closes the channel.
func (s *Subscription[T]) Unsubscribe() {
	s.bus.unsubscribe(s.id)
}

// EventBus provides fan-out event distribution.
// Multiple subscribers can listen concurrently; each receives all published events.
type EventBus[T any] struct {
	mu          sync.RWMutex
	subscribers map[uint64]*Subscription[T]
	nextID      uint64
}

// NewEventBus creates a new EventBus.
func NewEventBus[T any]() *EventBus[T] {
	return &EventBus[T]{
		subscribers: make(map[uint64]*Subscription[T]),
	}
}

// Subscribe creates a new subscription that receives all published events.
// The returned channel is buffered (capacity 64).
func (b *EventBus[T]) Subscribe() *Subscription[T] {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.nextID++
	ch := make(chan *T, 64)
	sub := &Subscription[T]{
		C:   ch,
		ch:  ch,
		id:  b.nextID,
		bus: b,
	}
	b.subscribers[sub.id] = sub
	return sub
}

// Publish sends an event to all current subscribers (non-blocking).
// If a subscriber's channel is full, the event is dropped for that subscriber.
func (b *EventBus[T]) Publish(event *T) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, sub := range b.subscribers {
		select {
		case sub.ch <- event:
		default:
			// Channel full — drop to avoid blocking the publisher
		}
	}
}

// unsubscribe removes a subscriber and closes its channel.
func (b *EventBus[T]) unsubscribe(id uint64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if sub, ok := b.subscribers[id]; ok {
		delete(b.subscribers, id)
		close(sub.ch)
	}
}

// Close removes all subscribers and closes all channels.
func (b *EventBus[T]) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	for id, sub := range b.subscribers {
		close(sub.ch)
		delete(b.subscribers, id)
	}
}
