package cache

import (
	"sync"

	"github.com/QuantProcessing/exchanges/model"
)

type Cache struct {
	mu                 sync.RWMutex
	instruments        map[model.InstrumentID]model.Instrument
	synthetics         map[model.InstrumentID]model.SyntheticInstrument
	accounts           map[model.AccountID]model.AccountSnapshot
	accountHistory     map[model.AccountID][]model.AccountSnapshot
	orders             map[model.AccountID]map[model.OrderID]model.OrderStatusReport
	orderClients       map[model.AccountID]map[model.ClientOrderID]model.OrderID
	orderVenues        map[model.AccountID]map[model.VenueOrderID]model.OrderID
	orderStrategies    map[model.AccountID]map[model.StrategyID]map[model.OrderID]struct{}
	orderPositions     map[model.AccountID]map[model.PositionID]map[model.OrderID]struct{}
	orderLists         map[model.AccountID]map[model.OrderListID]map[model.OrderID]struct{}
	orderExecSpawns    map[model.AccountID]map[model.ExecSpawnID]map[model.OrderID]struct{}
	fills              map[model.AccountID]map[model.OrderID]map[model.TradeID]model.FillReport
	fillTrades         map[model.AccountID]map[model.TradeID]model.FillReport
	fillVenues         map[model.AccountID]map[model.VenueOrderID]map[model.TradeID]model.FillReport
	deferredFills      map[model.AccountID]map[model.OrderID]map[model.TradeID]model.FillReport
	positions          map[model.AccountID]map[model.PositionID]model.PositionStatusReport
	positionInst       map[model.AccountID]map[model.InstrumentID]model.PositionID
	positionVenues     map[model.AccountID]map[model.VenuePositionID]model.PositionID
	positionStrategies map[model.AccountID]map[model.StrategyID]map[model.PositionID]struct{}
	tickers            map[model.InstrumentID]model.Ticker
	orderBooks         map[model.InstrumentID]model.OrderBook
	trades             map[model.InstrumentID]model.TradeTick
	quotes             map[model.InstrumentID]model.QuoteTick
	bars               map[model.BarType]model.Bar
	latestBars         map[model.InstrumentID]model.Bar
	customData         map[model.InstrumentID]map[string]model.CustomData
}

func New() *Cache {
	return &Cache{
		instruments:        make(map[model.InstrumentID]model.Instrument),
		synthetics:         make(map[model.InstrumentID]model.SyntheticInstrument),
		accounts:           make(map[model.AccountID]model.AccountSnapshot),
		accountHistory:     make(map[model.AccountID][]model.AccountSnapshot),
		orders:             make(map[model.AccountID]map[model.OrderID]model.OrderStatusReport),
		orderClients:       make(map[model.AccountID]map[model.ClientOrderID]model.OrderID),
		orderVenues:        make(map[model.AccountID]map[model.VenueOrderID]model.OrderID),
		orderStrategies:    make(map[model.AccountID]map[model.StrategyID]map[model.OrderID]struct{}),
		orderPositions:     make(map[model.AccountID]map[model.PositionID]map[model.OrderID]struct{}),
		orderLists:         make(map[model.AccountID]map[model.OrderListID]map[model.OrderID]struct{}),
		orderExecSpawns:    make(map[model.AccountID]map[model.ExecSpawnID]map[model.OrderID]struct{}),
		fills:              make(map[model.AccountID]map[model.OrderID]map[model.TradeID]model.FillReport),
		fillTrades:         make(map[model.AccountID]map[model.TradeID]model.FillReport),
		fillVenues:         make(map[model.AccountID]map[model.VenueOrderID]map[model.TradeID]model.FillReport),
		deferredFills:      make(map[model.AccountID]map[model.OrderID]map[model.TradeID]model.FillReport),
		positions:          make(map[model.AccountID]map[model.PositionID]model.PositionStatusReport),
		positionInst:       make(map[model.AccountID]map[model.InstrumentID]model.PositionID),
		positionVenues:     make(map[model.AccountID]map[model.VenuePositionID]model.PositionID),
		positionStrategies: make(map[model.AccountID]map[model.StrategyID]map[model.PositionID]struct{}),
		tickers:            make(map[model.InstrumentID]model.Ticker),
		orderBooks:         make(map[model.InstrumentID]model.OrderBook),
		trades:             make(map[model.InstrumentID]model.TradeTick),
		quotes:             make(map[model.InstrumentID]model.QuoteTick),
		bars:               make(map[model.BarType]model.Bar),
		latestBars:         make(map[model.InstrumentID]model.Bar),
		customData:         make(map[model.InstrumentID]map[string]model.CustomData),
	}
}
