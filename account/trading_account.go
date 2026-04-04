package account

import (
	"context"
	"sync"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/shopspring/decimal"
)

type TradingAccount struct {
	mu          sync.RWMutex
	lifecycleMu sync.Mutex
	runMu       sync.RWMutex

	adp    exchanges.Exchange
	logger exchanges.Logger

	orders    map[string]*exchanges.Order
	positions map[string]*exchanges.Position
	balance   decimal.Decimal

	orderBus    *eventBus[exchanges.Order]
	positionBus *eventBus[exchanges.Position]
	flows       *orderFlowRegistry

	started   bool
	starting  bool
	closing   bool
	runCancel context.CancelFunc
	runGen    uint64
}

type TradingAccountOption func(*TradingAccount)

func NewTradingAccount(adp exchanges.Exchange, logger exchanges.Logger, _ ...TradingAccountOption) *TradingAccount {
	if logger == nil {
		logger = exchanges.NopLogger
	}
	return &TradingAccount{
		adp:         adp,
		logger:      logger,
		orders:      make(map[string]*exchanges.Order),
		positions:   make(map[string]*exchanges.Position),
		orderBus:    newEventBus[exchanges.Order](),
		positionBus: newEventBus[exchanges.Position](),
		flows:       newOrderFlowRegistry(),
	}
}
