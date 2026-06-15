package kernel

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrBackpressure     = errors.New("msgbus backpressure")
	ErrEndpointNotFound = errors.New("endpoint not found")
)

type Envelope struct {
	Topic         string
	Message       any
	Timestamp     time.Time
	CorrelationID string
}

type Request struct {
	Endpoint      string
	Payload       any
	CorrelationID string
	Timestamp     time.Time
}

type Response struct {
	CorrelationID string
	Payload       any
}

type EndpointHandler func(context.Context, Request) (Response, error)

type MsgBusConfig struct {
	Clock         Clock
	DefaultBuffer int
}

type MsgBus struct {
	mu            sync.RWMutex
	clock         Clock
	defaultBuffer int
	subs          map[string]map[*msgSubscription]struct{}
	endpoints     map[string]EndpointHandler
	published     atomic.Int64
	dropped       atomic.Int64
	nextID        atomic.Int64
}

func NewMsgBus(cfg MsgBusConfig) *MsgBus {
	clock := cfg.Clock
	if clock == nil {
		clock = LiveClock{}
	}
	if cfg.DefaultBuffer < 0 {
		cfg.DefaultBuffer = 0
	}
	return &MsgBus{
		clock:         clock,
		defaultBuffer: cfg.DefaultBuffer,
		subs:          make(map[string]map[*msgSubscription]struct{}),
		endpoints:     make(map[string]EndpointHandler),
	}
}

func (b *MsgBus) Subscribe(topic string, buffer int) Subscription {
	if buffer < 0 {
		buffer = b.defaultBuffer
	}
	sub := &msgSubscription{
		bus:   b,
		topic: topic,
		ch:    make(chan Envelope, buffer),
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.subs[topic] == nil {
		b.subs[topic] = make(map[*msgSubscription]struct{})
	}
	b.subs[topic][sub] = struct{}{}
	return sub
}

func (b *MsgBus) Publish(ctx context.Context, topic string, message any) error {
	env := Envelope{Topic: topic, Message: message, Timestamp: b.clock.Now()}
	b.mu.RLock()
	subs := make([]*msgSubscription, 0, len(b.subs[topic]))
	for sub := range b.subs[topic] {
		subs = append(subs, sub)
	}
	b.mu.RUnlock()

	var publishErr error
	for _, sub := range subs {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case sub.ch <- env:
			b.published.Add(1)
		default:
			b.dropped.Add(1)
			publishErr = ErrBackpressure
		}
	}
	return publishErr
}

func (b *MsgBus) RegisterEndpoint(endpoint string, handler EndpointHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.endpoints[endpoint] = handler
}

func (b *MsgBus) Request(ctx context.Context, endpoint string, payload any) (Response, error) {
	b.mu.RLock()
	handler := b.endpoints[endpoint]
	b.mu.RUnlock()
	if handler == nil {
		return Response{}, fmt.Errorf("%w: %s", ErrEndpointNotFound, endpoint)
	}
	correlationID := fmt.Sprintf("req-%d", b.nextID.Add(1))
	response, err := handler(ctx, Request{
		Endpoint:      endpoint,
		Payload:       payload,
		CorrelationID: correlationID,
		Timestamp:     b.clock.Now(),
	})
	if err != nil {
		return Response{}, err
	}
	if response.CorrelationID == "" {
		response.CorrelationID = correlationID
	}
	return response, nil
}

func (b *MsgBus) Stats() MsgBusStats {
	return MsgBusStats{
		Published: b.published.Load(),
		Dropped:   b.dropped.Load(),
	}
}

func (b *MsgBus) remove(sub *msgSubscription) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.subs[sub.topic], sub)
	if len(b.subs[sub.topic]) == 0 {
		delete(b.subs, sub.topic)
	}
}

type Subscription interface {
	C() <-chan Envelope
	Close() error
}

type msgSubscription struct {
	once  sync.Once
	bus   *MsgBus
	topic string
	ch    chan Envelope
}

func (s *msgSubscription) C() <-chan Envelope {
	return s.ch
}

func (s *msgSubscription) Close() error {
	s.once.Do(func() {
		s.bus.remove(s)
		close(s.ch)
	})
	return nil
}
