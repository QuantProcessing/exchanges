package aster

import (
	"sync"

	exchanges "github.com/QuantProcessing/exchanges"
)

type privateOrderStreams[E any] struct {
	mu sync.Mutex

	started bool

	subscribe func(func(*E))
	mapOrder  func(*E) *exchanges.Order
	mapFill   func(*E) *exchanges.Fill

	orderCallback exchanges.OrderUpdateCallback
	fillCallback  exchanges.FillCallback
}

func newPrivateOrderStreams[E any](
	subscribe func(func(*E)),
	mapOrder func(*E) *exchanges.Order,
	mapFill func(*E) *exchanges.Fill,
) *privateOrderStreams[E] {
	return &privateOrderStreams[E]{
		subscribe: subscribe,
		mapOrder:  mapOrder,
		mapFill:   mapFill,
	}
}

func (s *privateOrderStreams[E]) watchOrders(connect func() error, callback exchanges.OrderUpdateCallback) error {
	if err := s.ensureStarted(connect); err != nil {
		return err
	}

	s.mu.Lock()
	s.orderCallback = callback
	s.mu.Unlock()
	return nil
}

func (s *privateOrderStreams[E]) watchFills(connect func() error, callback exchanges.FillCallback) error {
	if err := s.ensureStarted(connect); err != nil {
		return err
	}

	s.mu.Lock()
	s.fillCallback = callback
	s.mu.Unlock()
	return nil
}

func (s *privateOrderStreams[E]) stopOrders() {
	s.mu.Lock()
	s.orderCallback = nil
	s.mu.Unlock()
}

func (s *privateOrderStreams[E]) stopFills() {
	s.mu.Lock()
	s.fillCallback = nil
	s.mu.Unlock()
}

func (s *privateOrderStreams[E]) ensureStarted(connect func() error) error {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()

	if err := connect(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return nil
	}

	s.subscribe(s.handleEvent)
	s.started = true
	return nil
}

func (s *privateOrderStreams[E]) handleEvent(event *E) {
	s.mu.Lock()
	orderCallback := s.orderCallback
	fillCallback := s.fillCallback
	mapOrder := s.mapOrder
	mapFill := s.mapFill
	s.mu.Unlock()

	if orderCallback != nil && mapOrder != nil {
		if order := mapOrder(event); order != nil {
			orderCallback(order)
		}
	}

	if fillCallback != nil && mapFill != nil {
		if fill := mapFill(event); fill != nil {
			fillCallback(fill)
		}
	}
}
