package bus

import (
	"context"
	"sync"
	"time"
)

type Envelope struct {
	Topic     string
	Message   any
	Timestamp time.Time
}

type Subscription interface {
	C() <-chan Envelope
	Close() error
}

type Bus struct {
	mu   sync.RWMutex
	subs map[string]map[*subscription]struct{}
}

func New() *Bus {
	return &Bus{subs: make(map[string]map[*subscription]struct{})}
}

func (b *Bus) Subscribe(topic string, buffer int) Subscription {
	sub := &subscription{
		bus:   b,
		topic: topic,
		ch:    make(chan Envelope, buffer),
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.subs[topic] == nil {
		b.subs[topic] = make(map[*subscription]struct{})
	}
	b.subs[topic][sub] = struct{}{}
	return sub
}

func (b *Bus) Publish(ctx context.Context, topic string, message any) error {
	env := Envelope{Topic: topic, Message: message, Timestamp: time.Now()}
	b.mu.RLock()
	defer b.mu.RUnlock()
	for sub := range b.subs[topic] {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case sub.ch <- env:
		}
	}
	return nil
}

func (b *Bus) remove(sub *subscription) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.subs[sub.topic], sub)
	if len(b.subs[sub.topic]) == 0 {
		delete(b.subs, sub.topic)
	}
}

type subscription struct {
	once  sync.Once
	bus   *Bus
	topic string
	ch    chan Envelope
}

func (s *subscription) C() <-chan Envelope {
	return s.ch
}

func (s *subscription) Close() error {
	s.once.Do(func() {
		s.bus.remove(s)
		close(s.ch)
	})
	return nil
}
