package platform

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"time"

	"github.com/QuantProcessing/exchanges/venue"
)

var ErrEndpointNotFound = errors.New("platform endpoint not found")

type Handler func(context.Context, Event) error

type Event struct {
	Endpoint string
	Topic    string
	Message  any
	Time     time.Time
}

type BusHealth struct {
	Closed          bool
	Dropped         uint64
	ClosedPublishes uint64
	LastError       error
}

type Bus struct {
	mu          sync.RWMutex
	handlers    map[string]Handler
	subscribers map[string]map[string]*busSubscription
	closed      bool
	seq         uint64
	health      BusHealth
}

func NewBus() *Bus {
	return &Bus{
		handlers:    make(map[string]Handler),
		subscribers: make(map[string]map[string]*busSubscription),
	}
}

func (b *Bus) Register(endpoint string, handler Handler) venue.Subscription {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.seq++
	id := endpoint + "#" + strconv.FormatUint(b.seq, 10)
	if handler != nil && !b.closed {
		b.handlers[endpoint] = handler
	}
	return &endpointSubscription{
		id:       id,
		endpoint: endpoint,
		bus:      b,
		done:     make(chan struct{}),
	}
}

func (b *Bus) Send(ctx context.Context, endpoint string, msg any) error {
	b.mu.RLock()
	if b.closed {
		b.mu.RUnlock()
		return nil
	}
	handler, ok := b.handlers[endpoint]
	b.mu.RUnlock()
	if !ok {
		b.recordError(ErrEndpointNotFound)
		return ErrEndpointNotFound
	}
	return handler(ctx, Event{Endpoint: endpoint, Message: msg, Time: time.Now()})
}

func (b *Bus) Subscribe(topic string, buffer int) (venue.Subscription, <-chan Event) {
	if buffer < 0 {
		buffer = 0
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.seq++
	id := topic + "#" + strconv.FormatUint(b.seq, 10)
	sub := &busSubscription{
		id:    id,
		topic: topic,
		ch:    make(chan Event, buffer),
		done:  make(chan struct{}),
		bus:   b,
	}
	if !b.closed {
		if b.subscribers[topic] == nil {
			b.subscribers[topic] = make(map[string]*busSubscription)
		}
		b.subscribers[topic][id] = sub
	} else {
		close(sub.done)
		close(sub.ch)
	}
	return sub, sub.ch
}

func (b *Bus) Publish(topic string, msg any) error {
	ev := Event{Topic: topic, Message: msg, Time: time.Now()}

	b.mu.RLock()
	if b.closed {
		b.mu.RUnlock()
		b.recordClosedPublish()
		return nil
	}
	subs := make([]*busSubscription, 0, len(b.subscribers[topic]))
	for _, sub := range b.subscribers[topic] {
		subs = append(subs, sub)
	}
	b.mu.RUnlock()

	for _, sub := range subs {
		select {
		case sub.ch <- ev:
		default:
			b.recordDrop()
		}
	}
	return nil
}

func (b *Bus) Close() error {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil
	}
	b.closed = true
	b.health.Closed = true
	subscribers := b.subscribers
	b.subscribers = make(map[string]map[string]*busSubscription)
	b.handlers = make(map[string]Handler)
	b.mu.Unlock()

	for _, byID := range subscribers {
		for _, sub := range byID {
			sub.closeFromBus()
		}
	}
	return nil
}

func (b *Bus) Health() BusHealth {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.health
}

func (b *Bus) recordError(err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.health.LastError = err
}

func (b *Bus) recordDrop() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.health.Dropped++
}

func (b *Bus) recordClosedPublish() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.health.ClosedPublishes++
}

type busSubscription struct {
	id        string
	topic     string
	ch        chan Event
	done      chan struct{}
	bus       *Bus
	closeOnce sync.Once
	err       error
}

func (s *busSubscription) ID() string { return s.id }

func (s *busSubscription) Close() error {
	s.closeOnce.Do(func() {
		s.bus.mu.Lock()
		if subs := s.bus.subscribers[s.topic]; subs != nil {
			delete(subs, s.id)
			if len(subs) == 0 {
				delete(s.bus.subscribers, s.topic)
			}
		}
		s.bus.mu.Unlock()
		close(s.done)
		close(s.ch)
	})
	return s.err
}

func (s *busSubscription) Done() <-chan struct{} { return s.done }

func (s *busSubscription) Err() error { return s.err }

func (s *busSubscription) closeFromBus() {
	s.closeOnce.Do(func() {
		close(s.done)
		close(s.ch)
	})
}

type endpointSubscription struct {
	id        string
	endpoint  string
	bus       *Bus
	done      chan struct{}
	closeOnce sync.Once
	err       error
}

func (s *endpointSubscription) ID() string { return s.id }

func (s *endpointSubscription) Close() error {
	s.closeOnce.Do(func() {
		s.bus.mu.Lock()
		delete(s.bus.handlers, s.endpoint)
		s.bus.mu.Unlock()
		close(s.done)
	})
	return s.err
}

func (s *endpointSubscription) Done() <-chan struct{} { return s.done }

func (s *endpointSubscription) Err() error { return s.err }
