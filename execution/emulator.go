package execution

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/QuantProcessing/exchanges/cache"
	"github.com/QuantProcessing/exchanges/model"
	"github.com/shopspring/decimal"
)

type EmulatorConfig struct {
	Cache   *cache.Cache
	Manager *Manager
}

type Emulator struct {
	mu       sync.Mutex
	manager  *Manager
	cache    *cache.Cache
	orders   map[emulatedOrderKey]model.SubmitOrder
	trailing map[emulatedOrderKey]decimal.Decimal
}

type EmulatedRelease struct {
	Order     model.SubmitOrder
	Triggered model.OrderStatusReport
	Released  model.OrderStatusReport
}

type emulatedOrderKey struct {
	accountID     model.AccountID
	clientOrderID model.ClientOrderID
}

func NewEmulator(cfg EmulatorConfig) *Emulator {
	manager := cfg.Manager
	if manager == nil {
		manager = NewManager(Config{Cache: cfg.Cache})
	}
	c := cfg.Cache
	if c == nil && manager != nil {
		c = manager.cache
	}
	return &Emulator{
		manager:  manager,
		cache:    c,
		orders:   make(map[emulatedOrderKey]model.SubmitOrder),
		trailing: make(map[emulatedOrderKey]decimal.Decimal),
	}
}

func (e *Emulator) bind(manager *Manager) {
	if e == nil || manager == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.manager = manager
	e.cache = manager.cache
	if e.orders == nil {
		e.orders = make(map[emulatedOrderKey]model.SubmitOrder)
	}
	if e.trailing == nil {
		e.trailing = make(map[emulatedOrderKey]decimal.Decimal)
	}
}

func (e *Emulator) SubmitOrder(order model.SubmitOrder) (model.OrderStatusReport, *EmulatedRelease, error) {
	if e == nil {
		return model.OrderStatusReport{}, nil, fmt.Errorf("%w: execution emulator is required", model.ErrInvalidOrder)
	}
	if !order.EmulationTrigger.IsActive() {
		return model.OrderStatusReport{}, nil, fmt.Errorf("%w: order has no emulation trigger", model.ErrInvalidOrder)
	}
	if err := order.Validate(); err != nil {
		return model.OrderStatusReport{}, nil, err
	}
	if err := e.manager.CacheSubmitCommand(order); err != nil {
		return model.OrderStatusReport{}, nil, err
	}
	report := orderStatusReportFromSubmit(order, model.OrderStatusEmulated)
	key := emulatedOrderKey{accountID: order.AccountID, clientOrderID: order.ClientOrderID}
	var release *EmulatedRelease
	e.mu.Lock()
	if e.orders == nil {
		e.orders = make(map[emulatedOrderKey]model.SubmitOrder)
	}
	if e.trailing == nil {
		e.trailing = make(map[emulatedOrderKey]decimal.Decimal)
	}
	nextOrder := order
	if event, ok := e.latestMarketEvent(order); ok {
		triggered := false
		updated := false
		nextOrder, triggered, updated = e.evaluateOrder(key, order, event)
		if triggered {
			delete(e.trailing, key)
			nextOrder.EmulationTrigger = model.TriggerTypeNoTrigger
			nextRelease := emulatedReleaseFromSubmit(nextOrder)
			release = &nextRelease
		} else {
			e.orders[key] = nextOrder
			if updated {
				report = orderStatusReportFromSubmit(nextOrder, model.OrderStatusEmulated)
			}
		}
	} else {
		e.orders[key] = order
	}
	e.mu.Unlock()
	if release != nil {
		if err := e.manager.ApplyOrderReport(release.Triggered); err != nil {
			return model.OrderStatusReport{}, release, err
		}
		if err := e.manager.ApplyOrderReport(release.Released); err != nil {
			return model.OrderStatusReport{}, release, err
		}
		return release.Released, release, nil
	}
	if err := e.manager.ApplyOrderReport(report); err != nil {
		return model.OrderStatusReport{}, nil, err
	}
	return report, nil, nil
}

func (e *Emulator) ProcessMarketEvent(event model.MarketEvent) ([]EmulatedRelease, error) {
	if e == nil {
		return nil, nil
	}
	if err := event.Validate(); err != nil {
		return nil, err
	}
	instrumentID := event.InstrumentID()
	e.mu.Lock()
	keys := make([]emulatedOrderKey, 0, len(e.orders))
	for key, order := range e.orders {
		if order.TriggerInstrument() == instrumentID {
			keys = append(keys, key)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].accountID == keys[j].accountID {
			return keys[i].clientOrderID < keys[j].clientOrderID
		}
		return keys[i].accountID < keys[j].accountID
	})
	releases := make([]EmulatedRelease, 0, len(keys))
	updates := make([]model.OrderStatusReport, 0)
	for _, key := range keys {
		order := e.orders[key]
		nextOrder, triggered, updated := e.evaluateOrder(key, order, event)
		if !triggered {
			if updated {
				e.orders[key] = nextOrder
				updates = append(updates, orderStatusReportFromSubmit(nextOrder, model.OrderStatusEmulated))
			}
			continue
		}
		delete(e.orders, key)
		delete(e.trailing, key)
		nextOrder.EmulationTrigger = model.TriggerTypeNoTrigger
		releases = append(releases, emulatedReleaseFromSubmit(nextOrder))
	}
	e.mu.Unlock()
	for _, update := range updates {
		if err := e.manager.ApplyOrderReport(update); err != nil {
			return releases, err
		}
	}
	for _, release := range releases {
		if err := e.manager.ApplyOrderReport(release.Triggered); err != nil {
			return releases, err
		}
		if err := e.manager.ApplyOrderReport(release.Released); err != nil {
			return releases, err
		}
	}
	return releases, nil
}

func (e *Emulator) ModifyOrder(modify model.ModifyOrder) (model.OrderStatusReport, *EmulatedRelease, bool, error) {
	if e == nil {
		return model.OrderStatusReport{}, nil, false, nil
	}
	if err := modify.Validate(); err != nil {
		return model.OrderStatusReport{}, nil, false, err
	}
	report, ok := e.emulatedOrderReportForModify(modify)
	if !ok {
		return model.OrderStatusReport{}, nil, false, nil
	}
	key := emulatedOrderKey{accountID: report.AccountID, clientOrderID: report.ClientOrderID}
	var release *EmulatedRelease
	e.mu.Lock()
	order, ok := e.orders[key]
	if !ok {
		e.mu.Unlock()
		return model.OrderStatusReport{}, nil, false, nil
	}
	nextOrder, nextReport, err := applyEmulatedOrderModification(order, report, modify)
	if err != nil {
		e.mu.Unlock()
		return model.OrderStatusReport{}, nil, true, err
	}
	if modify.TriggerPrice.IsPositive() || modify.ActivationPrice.IsPositive() || modify.TrailingOffset.IsPositive() || modify.TrailingOffsetType != "" {
		delete(e.trailing, key)
	}
	e.orders[key] = nextOrder
	if event, ok := e.latestMarketEvent(nextOrder); ok {
		evaluatedOrder, triggered, updated := e.evaluateOrder(key, nextOrder, event)
		if triggered {
			delete(e.orders, key)
			delete(e.trailing, key)
			evaluatedOrder.EmulationTrigger = model.TriggerTypeNoTrigger
			nextRelease := emulatedReleaseFromSubmit(evaluatedOrder)
			release = &nextRelease
		} else if updated {
			e.orders[key] = evaluatedOrder
			nextReport = orderStatusReportFromSubmit(evaluatedOrder, model.OrderStatusEmulated)
			nextReport.Metadata = modify.Metadata.WithDefaults(report.Metadata)
		}
	}
	e.mu.Unlock()
	if err := e.manager.ApplyOrderReport(nextReport); err != nil {
		return model.OrderStatusReport{}, release, true, err
	}
	if release != nil {
		if err := e.manager.ApplyOrderReport(release.Triggered); err != nil {
			return model.OrderStatusReport{}, release, true, err
		}
		if err := e.manager.ApplyOrderReport(release.Released); err != nil {
			return model.OrderStatusReport{}, release, true, err
		}
	}
	return nextReport, release, true, nil
}

func (e *Emulator) CancelOrder(cancel model.CancelOrder) (model.OrderStatusReport, bool, error) {
	if e == nil {
		return model.OrderStatusReport{}, false, nil
	}
	if err := cancel.Validate(); err != nil {
		return model.OrderStatusReport{}, false, err
	}
	report, ok := e.emulatedOrderReport(cancel)
	if !ok {
		return model.OrderStatusReport{}, false, nil
	}
	key := emulatedOrderKey{accountID: report.AccountID, clientOrderID: report.ClientOrderID}
	e.mu.Lock()
	if _, ok := e.orders[key]; !ok {
		e.mu.Unlock()
		return model.OrderStatusReport{}, false, nil
	}
	delete(e.orders, key)
	delete(e.trailing, key)
	e.mu.Unlock()
	_, _ = e.manager.PopSubmitCommand(report.ClientOrderID)
	report.Metadata = cancel.Metadata.WithDefaults(report.Metadata)
	report.Status = model.OrderStatusCanceled
	report.LeavesQuantity = decimal.Zero
	if err := e.manager.ApplyOrderReport(report); err != nil {
		return model.OrderStatusReport{}, true, err
	}
	return report, true, nil
}

func (e *Emulator) emulatedOrderReport(cancel model.CancelOrder) (model.OrderStatusReport, bool) {
	if e.manager == nil || e.manager.cache == nil {
		return model.OrderStatusReport{}, false
	}
	if cancel.OrderID != "" {
		if report, ok := e.manager.cache.Order(cancel.AccountID, cancel.OrderID); ok && report.Status == model.OrderStatusEmulated {
			return report, true
		}
	}
	if cancel.ClientOrderID != "" {
		if report, ok := e.manager.cache.OrderByClientID(cancel.AccountID, cancel.ClientOrderID); ok && report.Status == model.OrderStatusEmulated {
			return report, true
		}
	}
	return model.OrderStatusReport{}, false
}

func (e *Emulator) emulatedOrderReportForModify(modify model.ModifyOrder) (model.OrderStatusReport, bool) {
	if e.manager == nil || e.manager.cache == nil {
		return model.OrderStatusReport{}, false
	}
	if modify.OrderID != "" {
		if report, ok := e.manager.cache.Order(modify.AccountID, modify.OrderID); ok && report.Status == model.OrderStatusEmulated {
			return report, true
		}
	}
	if modify.ClientOrderID != "" {
		if report, ok := e.manager.cache.OrderByClientID(modify.AccountID, modify.ClientOrderID); ok && report.Status == model.OrderStatusEmulated {
			return report, true
		}
	}
	return model.OrderStatusReport{}, false
}

func applyEmulatedOrderModification(order model.SubmitOrder, report model.OrderStatusReport, modify model.ModifyOrder) (model.SubmitOrder, model.OrderStatusReport, error) {
	updatedReport, _, err := model.ApplyOrderModification(report, modify)
	if err != nil {
		return model.SubmitOrder{}, model.OrderStatusReport{}, err
	}
	updatedReport.Status = model.OrderStatusEmulated
	updatedReport.Metadata = modify.Metadata.WithDefaults(report.Metadata)
	updatedOrder := order
	updatedOrder.Metadata = modify.Metadata.WithDefaults(order.Metadata)
	updatedOrder.Quantity = updatedReport.Quantity
	updatedOrder.Price = updatedReport.Price
	updatedOrder.TriggerPrice = updatedReport.TriggerPrice
	updatedOrder.ActivationPrice = updatedReport.ActivationPrice
	updatedOrder.TrailingOffset = updatedReport.TrailingOffset
	updatedOrder.TrailingOffsetType = updatedReport.TrailingOffsetType
	updatedOrder.TimeInForce = updatedReport.TimeInForce
	return updatedOrder, updatedReport, nil
}

func emulatedReleaseFromSubmit(order model.SubmitOrder) EmulatedRelease {
	return EmulatedRelease{
		Order:     transformReleasedOrder(order),
		Triggered: orderStatusReportFromSubmit(order, model.OrderStatusTriggered),
		Released:  orderStatusReportFromSubmit(order, model.OrderStatusReleased),
	}
}

func (e *Emulator) evaluateOrder(key emulatedOrderKey, order model.SubmitOrder, event model.MarketEvent) (model.SubmitOrder, bool, bool) {
	if isTrailingStop(order.Type) {
		return e.evaluateTrailingOrder(key, order, event)
	}
	return order, emulatedOrderTriggered(order, event), false
}

func (e *Emulator) latestMarketEvent(order model.SubmitOrder) (model.MarketEvent, bool) {
	if e.cache == nil {
		return model.MarketEvent{}, false
	}
	instrumentID := order.TriggerInstrument()
	switch order.EmulationTrigger {
	case model.TriggerTypeBidAsk, model.TriggerTypeDefault:
		return e.latestBidAskMarketEvent(instrumentID)
	case model.TriggerTypeLastPrice:
		if trade, ok := e.cache.TradeTick(instrumentID); ok {
			return model.MarketEvent{Trade: &trade}, true
		}
	}
	return model.MarketEvent{}, false
}

func (e *Emulator) latestBidAskMarketEvent(instrumentID model.InstrumentID) (model.MarketEvent, bool) {
	var event model.MarketEvent
	var ts time.Time
	var seen bool
	if quote, ok := e.cache.QuoteTick(instrumentID); ok {
		event = model.MarketEvent{Quote: &quote}
		ts = quote.Timestamp
		seen = true
	}
	if book, ok := e.cache.OrderBook(instrumentID); ok {
		if !seen || book.Timestamp.After(ts) {
			event = model.MarketEvent{OrderBook: &book}
			seen = true
		}
	}
	return event, seen
}

func (e *Emulator) evaluateTrailingOrder(key emulatedOrderKey, order model.SubmitOrder, event model.MarketEvent) (model.SubmitOrder, bool, bool) {
	if !order.TrailingOffset.IsPositive() {
		return order, false, false
	}
	favorable, adverse, ok := trailingRange(order, event)
	if !ok {
		return order, false, false
	}
	extreme, active := e.trailing[key]
	if !active {
		if order.ActivationPrice.IsPositive() {
			if order.Side == model.OrderSideSell && favorable.LessThan(order.ActivationPrice) {
				return order, false, false
			}
			if order.Side == model.OrderSideBuy && favorable.GreaterThan(order.ActivationPrice) {
				return order, false, false
			}
		}
		e.trailing[key] = favorable
		extreme = favorable
	}
	if order.Side == model.OrderSideSell {
		if favorable.GreaterThan(extreme) {
			e.trailing[key] = favorable
			extreme = favorable
		}
		offset, ok := e.trailingOffsetPrice(order, extreme)
		if !ok {
			return order, false, false
		}
		triggerPrice := extreme.Sub(offset)
		updated := !order.TriggerPrice.Equal(triggerPrice)
		order.TriggerPrice = triggerPrice
		return order, adverse.LessThanOrEqual(triggerPrice), updated
	}
	if order.Side == model.OrderSideBuy {
		if favorable.LessThan(extreme) {
			e.trailing[key] = favorable
			extreme = favorable
		}
		offset, ok := e.trailingOffsetPrice(order, extreme)
		if !ok {
			return order, false, false
		}
		triggerPrice := extreme.Add(offset)
		updated := !order.TriggerPrice.Equal(triggerPrice)
		order.TriggerPrice = triggerPrice
		return order, adverse.GreaterThanOrEqual(triggerPrice), updated
	}
	return order, false, false
}

func (e *Emulator) trailingOffsetPrice(order model.SubmitOrder, reference decimal.Decimal) (decimal.Decimal, bool) {
	switch order.TrailingOffsetType.Canonical() {
	case model.TrailingOffsetTypePrice:
		return order.TrailingOffset, true
	case model.TrailingOffsetTypeBasisPoints:
		return reference.Mul(order.TrailingOffset).Div(decimal.NewFromInt(10000)), true
	case model.TrailingOffsetTypeTicks:
		if e.cache == nil {
			return decimal.Zero, false
		}
		triggerInstrumentID := order.TriggerInstrument()
		if inst, ok := e.cache.Instrument(triggerInstrumentID); ok && inst.PriceTick.IsPositive() {
			return order.TrailingOffset.Mul(inst.PriceTick), true
		}
		if synth, ok := e.cache.SyntheticInstrument(triggerInstrumentID); ok && synth.PriceTick.IsPositive() {
			return order.TrailingOffset.Mul(synth.PriceTick), true
		}
		return decimal.Zero, false
	default:
		return decimal.Zero, false
	}
}

func orderStatusReportFromSubmit(order model.SubmitOrder, status model.OrderStatus) model.OrderStatusReport {
	orderID := model.OrderID("emulated-" + string(order.ClientOrderID))
	return model.OrderStatusReport{
		Metadata:            order.Metadata,
		AccountID:           order.AccountID,
		InstrumentID:        order.InstrumentID,
		TriggerInstrumentID: order.TriggerInstrumentID,
		OrderListID:         order.OrderListID,
		ParentClientOrderID: order.ParentClientOrderID,
		ClientOrderID:       order.ClientOrderID,
		OrderID:             orderID,
		Status:              status,
		Side:                order.Side,
		Type:                order.Type,
		Contingency:         order.Contingency,
		Quantity:            order.Quantity,
		LeavesQuantity:      order.Quantity,
		Price:               order.Price,
		TriggerPrice:        order.TriggerPrice,
		ActivationPrice:     order.ActivationPrice,
		TrailingOffset:      order.TrailingOffset,
		TrailingOffsetType:  order.TrailingOffsetType,
		PostOnly:            order.PostOnly,
		ReduceOnly:          order.ReduceOnly,
		TimeInForce:         order.TimeInForce,
		ExpireTime:          order.ExpireTime,
	}
}

func emulatedOrderTriggered(order model.SubmitOrder, event model.MarketEvent) bool {
	if order.Type == model.OrderTypeLimit || order.Type == model.OrderTypeMarketToLimit {
		return emulatedLimitOrderMarketable(order, event)
	}
	price, ok := emulationPrice(order, event)
	if !ok || !price.IsPositive() || !order.TriggerPrice.IsPositive() {
		return false
	}
	switch order.Type {
	case model.OrderTypeMarketIfTouched, model.OrderTypeLimitIfTouched:
		if order.Side == model.OrderSideBuy {
			return price.LessThanOrEqual(order.TriggerPrice)
		}
		return price.GreaterThanOrEqual(order.TriggerPrice)
	default:
		if order.Side == model.OrderSideBuy {
			return price.GreaterThanOrEqual(order.TriggerPrice)
		}
		return price.LessThanOrEqual(order.TriggerPrice)
	}
}

func emulatedLimitOrderMarketable(order model.SubmitOrder, event model.MarketEvent) bool {
	if !order.Price.IsPositive() {
		return false
	}
	price, ok := emulationPrice(order, event)
	if !ok || !price.IsPositive() {
		return false
	}
	if order.Side == model.OrderSideBuy {
		return price.LessThanOrEqual(order.Price)
	}
	if order.Side == model.OrderSideSell {
		return price.GreaterThanOrEqual(order.Price)
	}
	return false
}

func emulationPrice(order model.SubmitOrder, event model.MarketEvent) (decimal.Decimal, bool) {
	switch order.EmulationTrigger {
	case model.TriggerTypeBidAsk:
		return bidAskTriggerPrice(order, event)
	case model.TriggerTypeLastPrice:
		if event.Trade == nil {
			return decimal.Zero, false
		}
		return event.Trade.Price, true
	case model.TriggerTypeDefault:
		return bidAskTriggerPrice(order, event)
	}
	return decimal.Zero, false
}

func isTrailingStop(orderType model.OrderType) bool {
	return orderType == model.OrderTypeTrailingStopMarket || orderType == model.OrderTypeTrailingStopLimit
}

func trailingRange(order model.SubmitOrder, event model.MarketEvent) (decimal.Decimal, decimal.Decimal, bool) {
	switch order.EmulationTrigger {
	case model.TriggerTypeBidAsk:
		return bidAskTrailingRange(order, event)
	case model.TriggerTypeLastPrice:
		return tradeTrailingRange(event)
	case model.TriggerTypeDefault:
		return bidAskTrailingRange(order, event)
	default:
		return decimal.Zero, decimal.Zero, false
	}
}

func bidAskTrailingRange(order model.SubmitOrder, event model.MarketEvent) (decimal.Decimal, decimal.Decimal, bool) {
	if price, ok := bidAskTriggerPrice(order, event); ok {
		return price, price, true
	}
	return decimal.Zero, decimal.Zero, false
}

func tradeTrailingRange(event model.MarketEvent) (decimal.Decimal, decimal.Decimal, bool) {
	if event.Trade == nil {
		return decimal.Zero, decimal.Zero, false
	}
	return event.Trade.Price, event.Trade.Price, true
}

func transformReleasedOrder(order model.SubmitOrder) model.SubmitOrder {
	order.EmulationTrigger = model.TriggerTypeNoTrigger
	order.TriggerInstrumentID = model.InstrumentID{}
	switch order.Type {
	case model.OrderTypeStopMarket, model.OrderTypeMarketIfTouched, model.OrderTypeTrailingStopMarket:
		order.Type = model.OrderTypeMarket
		order.Price = decimal.Zero
		order.TriggerPrice = decimal.Zero
		order.ActivationPrice = decimal.Zero
		order.TrailingOffset = decimal.Zero
		order.TrailingOffsetType = ""
	case model.OrderTypeStopLimit, model.OrderTypeLimitIfTouched, model.OrderTypeTrailingStopLimit:
		order.Type = model.OrderTypeLimit
		order.TriggerPrice = decimal.Zero
		order.ActivationPrice = decimal.Zero
		order.TrailingOffset = decimal.Zero
		order.TrailingOffsetType = ""
	}
	return order
}

func bidAskTriggerPrice(order model.SubmitOrder, event model.MarketEvent) (decimal.Decimal, bool) {
	if event.Quote != nil {
		if order.Side == model.OrderSideBuy {
			return event.Quote.AskPrice, true
		}
		return event.Quote.BidPrice, true
	}
	if event.OrderBook == nil {
		return decimal.Zero, false
	}
	if order.Side == model.OrderSideBuy && len(event.OrderBook.Asks) > 0 {
		return event.OrderBook.Asks[0].Price, true
	}
	if order.Side == model.OrderSideSell && len(event.OrderBook.Bids) > 0 {
		return event.OrderBook.Bids[0].Price, true
	}
	return decimal.Zero, false
}
