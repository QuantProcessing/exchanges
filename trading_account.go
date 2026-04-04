package exchanges

import (
	"context"
	"sync"

	"github.com/shopspring/decimal"
)

type TradingAccount struct {
	mu          sync.RWMutex
	lifecycleMu sync.Mutex
	runMu       sync.RWMutex

	adp    Exchange
	logger Logger

	orders    map[string]*Order
	positions map[string]*Position
	balance   decimal.Decimal

	orderBus    *EventBus[Order]
	positionBus *EventBus[Position]
	flows       *orderFlowRegistry

	started   bool
	starting  bool
	closing   bool
	runCancel context.CancelFunc
	runGen    uint64
}

type TradingAccountOption func(*TradingAccount)

func NewTradingAccount(adp Exchange, logger Logger, _ ...TradingAccountOption) *TradingAccount {
	if logger == nil {
		logger = NopLogger
	}
	return &TradingAccount{
		adp:         adp,
		logger:      logger,
		orders:      make(map[string]*Order),
		positions:   make(map[string]*Position),
		orderBus:    NewEventBus[Order](),
		positionBus: NewEventBus[Position](),
		flows:       newOrderFlowRegistry(),
	}
}
