package cache

import (
	"fmt"
	"sync"

	"github.com/QuantProcessing/exchanges/model"
)

type Cache struct {
	mu sync.RWMutex

	instruments map[model.InstrumentID]model.Instrument
	accounts    map[accountKey]model.AccountState

	ordersByOrderID  map[orderIDKey]model.OrderStatusReport
	ordersByClientID map[clientIDKey]model.OrderStatusReport

	fillsByTradeID map[tradeIDKey]model.FillReport
	fillsByOrderID map[orderIDKey]map[model.TradeID]model.FillReport

	positions map[positionKey]model.PositionStatusReport
}

type accountKey struct {
	venue model.Venue
	id    model.AccountID
}

type orderIDKey struct {
	accountID model.AccountID
	orderID   model.OrderID
}

type clientIDKey struct {
	accountID model.AccountID
	clientID  model.ClientOrderID
}

type tradeIDKey struct {
	accountID model.AccountID
	tradeID   model.TradeID
}

type positionKey struct {
	accountID    model.AccountID
	instrumentID model.InstrumentID
	positionID   model.PositionID
	side         model.PositionSide
}

func New() *Cache {
	return &Cache{
		instruments:      make(map[model.InstrumentID]model.Instrument),
		accounts:         make(map[accountKey]model.AccountState),
		ordersByOrderID:  make(map[orderIDKey]model.OrderStatusReport),
		ordersByClientID: make(map[clientIDKey]model.OrderStatusReport),
		fillsByTradeID:   make(map[tradeIDKey]model.FillReport),
		fillsByOrderID:   make(map[orderIDKey]map[model.TradeID]model.FillReport),
		positions:        make(map[positionKey]model.PositionStatusReport),
	}
}

func (c *Cache) PutInstrument(inst model.Instrument) error {
	if err := inst.Validate(); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.instruments[inst.ID] = inst
	return nil
}

func (c *Cache) Instrument(id model.InstrumentID) (model.Instrument, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	inst, ok := c.instruments[id]
	return inst, ok
}

func (c *Cache) Instruments() []model.Instrument {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]model.Instrument, 0, len(c.instruments))
	for _, inst := range c.instruments {
		out = append(out, inst)
	}
	return out
}

func (c *Cache) ApplyAccountState(state model.AccountState) error {
	if state.Venue == "" || state.AccountID == "" {
		return fmt.Errorf("%w: missing account state key", model.ErrInvalidAccountState)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.accounts[accountKey{venue: state.Venue, id: state.AccountID}] = state
	return nil
}

func (c *Cache) AccountState(v model.Venue, id model.AccountID) (model.AccountState, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	state, ok := c.accounts[accountKey{venue: v, id: id}]
	return state, ok
}

func (c *Cache) PutOrderStatus(report model.OrderStatusReport) error {
	if report.AccountID == "" {
		return fmt.Errorf("%w: missing order account id", model.ErrInvalidAccountState)
	}
	if err := report.InstrumentID.Validate(); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if report.OrderID != "" {
		c.ordersByOrderID[orderIDKey{accountID: report.AccountID, orderID: report.OrderID}] = report
	}
	if report.ClientID != "" {
		c.ordersByClientID[clientIDKey{accountID: report.AccountID, clientID: report.ClientID}] = report
	}
	return nil
}

func (c *Cache) OrderByOrderID(accountID model.AccountID, orderID model.OrderID) (model.OrderStatusReport, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	report, ok := c.ordersByOrderID[orderIDKey{accountID: accountID, orderID: orderID}]
	return report, ok
}

func (c *Cache) OrderByClientID(accountID model.AccountID, clientID model.ClientOrderID) (model.OrderStatusReport, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	report, ok := c.ordersByClientID[clientIDKey{accountID: accountID, clientID: clientID}]
	return report, ok
}

func (c *Cache) PutFill(report model.FillReport) error {
	if report.AccountID == "" {
		return fmt.Errorf("%w: missing fill account id", model.ErrInvalidAccountState)
	}
	if report.TradeID == "" {
		return fmt.Errorf("%w: missing fill trade id", model.ErrInvalidAccountState)
	}
	if err := report.InstrumentID.Validate(); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	tradeKey := tradeIDKey{accountID: report.AccountID, tradeID: report.TradeID}
	c.fillsByTradeID[tradeKey] = report
	if report.OrderID != "" {
		key := orderIDKey{accountID: report.AccountID, orderID: report.OrderID}
		if c.fillsByOrderID[key] == nil {
			c.fillsByOrderID[key] = make(map[model.TradeID]model.FillReport)
		}
		c.fillsByOrderID[key][report.TradeID] = report
	}
	return nil
}

func (c *Cache) FillsByOrderID(accountID model.AccountID, orderID model.OrderID) []model.FillReport {
	c.mu.RLock()
	defer c.mu.RUnlock()
	reports := c.fillsByOrderID[orderIDKey{accountID: accountID, orderID: orderID}]
	out := make([]model.FillReport, 0, len(reports))
	for _, report := range reports {
		out = append(out, report)
	}
	return out
}

func (c *Cache) PutPosition(report model.PositionStatusReport) error {
	if report.AccountID == "" {
		return fmt.Errorf("%w: missing position account id", model.ErrInvalidAccountState)
	}
	if err := report.InstrumentID.Validate(); err != nil {
		return err
	}
	key := positionKey{
		accountID:    report.AccountID,
		instrumentID: report.InstrumentID,
		positionID:   report.PositionID,
		side:         report.Side,
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.positions[key] = report
	return nil
}

func (c *Cache) Positions(accountID model.AccountID) []model.PositionStatusReport {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]model.PositionStatusReport, 0, len(c.positions))
	for key, report := range c.positions {
		if key.accountID == accountID {
			out = append(out, report)
		}
	}
	return out
}
