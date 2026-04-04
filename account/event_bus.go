package account

import "sync"

// ============================================================================
// EventBus — generic fan-out pub/sub for streaming events
// ============================================================================

// Subscription represents a single subscriber's channel for receiving events.
// Call Unsubscribe() to stop receiving events and release resources.
type Subscription[T any] struct {
	C   <-chan *T    // Read-only channel for the consumer
	ch  chan *T      // Internal writable channel
	id  uint64       // Unique subscriber ID
	bus *eventBus[T] // Back-reference for unsubscribe
}

// Unsubscribe removes this subscription and closes the channel.
func (s *Subscription[T]) Unsubscribe() {
	s.bus.unsubscribe(s.id)
}

// eventBus provides fan-out event distribution for the account runtime.
type eventBus[T any] struct {
	mu          sync.RWMutex
	subscribers map[uint64]*Subscription[T]
	nextID      uint64
}

func newEventBus[T any]() *eventBus[T] {
	return &eventBus[T]{
		subscribers: make(map[uint64]*Subscription[T]),
	}
}

// Subscribe creates a new subscription that receives all published events.
// The returned channel is buffered (capacity 64).
func (b *eventBus[T]) Subscribe() *Subscription[T] {
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
func (b *eventBus[T]) Publish(event *T) {
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
func (b *eventBus[T]) unsubscribe(id uint64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if sub, ok := b.subscribers[id]; ok {
		delete(b.subscribers, id)
		close(sub.ch)
	}
}

// Close removes all subscribers and closes all channels.
func (b *eventBus[T]) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	for id, sub := range b.subscribers {
		close(sub.ch)
		delete(b.subscribers, id)
	}
}
