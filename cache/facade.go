package cache

import "github.com/QuantProcessing/exchanges/model"

type Facade struct {
	cache *Cache
}

func (c *Cache) Facade() Facade {
	return Facade{cache: c}
}

func (f Facade) Instruments() []model.Instrument {
	if f.cache == nil {
		return nil
	}
	return f.cache.Instruments()
}

func (f Facade) AccountState(v model.Venue, id model.AccountID) (model.AccountState, bool) {
	if f.cache == nil {
		return model.AccountState{}, false
	}
	return f.cache.AccountState(v, id)
}

func (f Facade) OrderByOrderID(accountID model.AccountID, orderID model.OrderID) (model.OrderStatusReport, bool) {
	if f.cache == nil {
		return model.OrderStatusReport{}, false
	}
	return f.cache.OrderByOrderID(accountID, orderID)
}

func (f Facade) OrderByClientID(accountID model.AccountID, clientID model.ClientOrderID) (model.OrderStatusReport, bool) {
	if f.cache == nil {
		return model.OrderStatusReport{}, false
	}
	return f.cache.OrderByClientID(accountID, clientID)
}

func (f Facade) FillsByOrderID(accountID model.AccountID, orderID model.OrderID) []model.FillReport {
	if f.cache == nil {
		return nil
	}
	return f.cache.FillsByOrderID(accountID, orderID)
}

func (f Facade) Positions(accountID model.AccountID) []model.PositionStatusReport {
	if f.cache == nil {
		return nil
	}
	return f.cache.Positions(accountID)
}
