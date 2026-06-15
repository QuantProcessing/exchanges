package portfolio

import (
	"fmt"
	"sync"

	"github.com/QuantProcessing/exchanges/cache"
	"github.com/QuantProcessing/exchanges/model"
	"github.com/shopspring/decimal"
)

type Portfolio struct {
	mu              sync.RWMutex
	cache           *cache.Cache
	marks           map[model.AccountID]map[model.InstrumentID]decimal.Decimal
	realized        map[model.AccountID]map[model.InstrumentID]decimal.Decimal
	commissions     map[model.AccountID]map[model.Currency]decimal.Decimal
	conversions     map[model.Currency]map[model.Currency]decimal.Decimal
	unrealizedCache map[model.AccountID]map[model.InstrumentID]decimal.Decimal
	appliedFills    map[model.AccountID]map[model.OrderID]map[model.TradeID]struct{}
	analyzer        Analyzer
}

type Analyzer interface {
	RecordTrade(TradeRecord)
}

type AnalyzerFunc func(TradeRecord)

func (f AnalyzerFunc) RecordTrade(record TradeRecord) {
	f(record)
}

type TradeRecord struct {
	AccountID          model.AccountID
	InstrumentID       model.InstrumentID
	PositionID         model.PositionID
	Currency           model.Currency
	RealizedPnL        decimal.Decimal
	AccountCurrency    model.Currency
	AccountCurrencyPnL decimal.Decimal
}

func New(c *cache.Cache) *Portfolio {
	if c == nil {
		c = cache.New()
	}
	return &Portfolio{
		cache:           c,
		marks:           make(map[model.AccountID]map[model.InstrumentID]decimal.Decimal),
		realized:        make(map[model.AccountID]map[model.InstrumentID]decimal.Decimal),
		commissions:     make(map[model.AccountID]map[model.Currency]decimal.Decimal),
		conversions:     make(map[model.Currency]map[model.Currency]decimal.Decimal),
		unrealizedCache: make(map[model.AccountID]map[model.InstrumentID]decimal.Decimal),
		appliedFills:    make(map[model.AccountID]map[model.OrderID]map[model.TradeID]struct{}),
	}
}

func (p *Portfolio) SetAnalyzer(analyzer Analyzer) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.analyzer = analyzer
}

func (p *Portfolio) UpdateAccount(account model.AccountSnapshot) error {
	if err := account.Validate(); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cache.PutAccount(account)
	p.invalidateAccountPnLLocked(account.AccountID)
	return nil
}

func (p *Portfolio) HandleExecutionEvent(event model.ExecutionEvent) error {
	if err := event.Validate(); err != nil {
		return err
	}
	switch {
	case event.Account != nil:
		return p.UpdateAccount(*event.Account)
	case event.Order != nil:
		return p.putOrder(*event.Order)
	case event.Lifecycle != nil:
		return p.handleOrderLifecycleEvent(*event.Lifecycle)
	case event.Fill != nil:
		return p.ApplyFill(*event.Fill)
	case event.Position != nil:
		return p.putPosition(*event.Position)
	case event.PositionLifecycle != nil:
		return p.handlePositionLifecycleEvent(*event.PositionLifecycle)
	default:
		return model.ErrInvalidExecutionEvent
	}
}

func (p *Portfolio) handleOrderLifecycleEvent(event model.OrderLifecycleEvent) error {
	if event.Report != nil {
		return p.putOrder(*event.Report)
	}
	if event.OrderID == "" {
		return nil
	}
	report := model.OrderStatusReport{
		Metadata:      event.Metadata,
		AccountID:     event.AccountID,
		InstrumentID:  event.InstrumentID,
		OrderID:       event.OrderID,
		ClientOrderID: event.ClientOrderID,
		VenueOrderID:  event.VenueOrderID,
		Status:        event.Status,
	}
	return p.putOrder(report)
}

func (p *Portfolio) handlePositionLifecycleEvent(event model.PositionLifecycleEvent) error {
	if event.Report != nil {
		return p.putPosition(*event.Report)
	}
	position := model.PositionStatusReport{
		AccountID:    event.AccountID,
		InstrumentID: event.InstrumentID,
		PositionID:   event.PositionID,
		Side:         event.Side,
		Quantity:     event.Quantity,
	}
	return p.putPosition(position)
}

func (p *Portfolio) putOrder(order model.OrderStatusReport) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.cache.PutOrder(order); err != nil {
		return err
	}
	p.invalidatePnLLocked(order.AccountID, order.InstrumentID)
	return nil
}

func (p *Portfolio) ApplyFill(fill model.FillReport) error {
	if err := fill.Validate(); err != nil {
		return err
	}
	if fill.Side == "" {
		fill.Side = model.OrderSideBuy
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	var previous *model.PositionStatusReport
	if existing, ok := p.cache.PositionByInstrument(fill.AccountID, fill.InstrumentID); ok {
		previous = &existing
	}
	position := p.nextPosition(fill)
	return p.applyFillAccounting(fill, previous, position)
}

func (p *Portfolio) ApplyFillWithPosition(fill model.FillReport, previous *model.PositionStatusReport, position model.PositionStatusReport) error {
	if err := fill.Validate(); err != nil {
		return err
	}
	if fill.Side == "" {
		fill.Side = model.OrderSideBuy
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.applyFillAccounting(fill, previous, position)
}

func (p *Portfolio) applyFillAccounting(fill model.FillReport, previous *model.PositionStatusReport, position model.PositionStatusReport) error {
	if p.hasAppliedFill(fill) {
		return nil
	}
	if _, err := p.cache.PutFill(fill); err != nil {
		return err
	}
	p.markAppliedFill(fill)
	p.applyCommission(fill)
	realizedDelta := p.applyRealizedPnLFromPosition(fill, previous)
	if err := p.cache.PutPosition(position); err != nil {
		return err
	}
	if err := p.applyFillBalanceDeltas(fill, realizedDelta); err != nil {
		return err
	}
	p.recordClosedTradeLocked(fill, previous, position, realizedDelta)
	p.setMarkLocked(fill.AccountID, fill.InstrumentID, fill.Price)
	return nil
}

func (p *Portfolio) ApplyMarketEvent(event model.MarketEvent) error {
	if err := event.Validate(); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.cache.PutMarketEvent(event); err != nil {
		return err
	}
	instrumentID := event.InstrumentID()
	for _, position := range p.cache.PositionsForInstrument(instrumentID) {
		if position.Side == model.PositionSideFlat || position.Quantity.IsZero() {
			continue
		}
		mark := marketEventMark(event, position.Side)
		if mark.IsPositive() {
			p.setMarkLocked(position.AccountID, instrumentID, mark)
		}
	}
	return nil
}

func (p *Portfolio) putPosition(position model.PositionStatusReport) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.cache.PutPosition(position); err != nil {
		return err
	}
	p.invalidatePnLLocked(position.AccountID, position.InstrumentID)
	if position.EntryPrice.IsPositive() && !p.marks[position.AccountID][position.InstrumentID].IsPositive() {
		p.setMarkLocked(position.AccountID, position.InstrumentID, position.EntryPrice)
	}
	return nil
}

func (p *Portfolio) hasAppliedFill(fill model.FillReport) bool {
	if p.appliedFills[fill.AccountID] == nil || p.appliedFills[fill.AccountID][fill.OrderID] == nil {
		return false
	}
	_, ok := p.appliedFills[fill.AccountID][fill.OrderID][fill.TradeID]
	return ok
}

func (p *Portfolio) markAppliedFill(fill model.FillReport) {
	if p.appliedFills[fill.AccountID] == nil {
		p.appliedFills[fill.AccountID] = make(map[model.OrderID]map[model.TradeID]struct{})
	}
	if p.appliedFills[fill.AccountID][fill.OrderID] == nil {
		p.appliedFills[fill.AccountID][fill.OrderID] = make(map[model.TradeID]struct{})
	}
	p.appliedFills[fill.AccountID][fill.OrderID][fill.TradeID] = struct{}{}
}

func (p *Portfolio) SetMark(accountID model.AccountID, instrumentID model.InstrumentID, price decimal.Decimal) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.setMarkLocked(accountID, instrumentID, price)
}

func (p *Portfolio) SetConversionRate(from model.Currency, to model.Currency, rate decimal.Decimal) error {
	if from == "" || to == "" || !rate.IsPositive() {
		return fmt.Errorf("%w: invalid conversion rate", model.ErrInvalidAccount)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.conversions[from] == nil {
		p.conversions[from] = make(map[model.Currency]decimal.Decimal)
	}
	p.conversions[from][to] = rate
	return nil
}

func (p *Portfolio) setMarkLocked(accountID model.AccountID, instrumentID model.InstrumentID, price decimal.Decimal) {
	if p.marks[accountID] == nil {
		p.marks[accountID] = make(map[model.InstrumentID]decimal.Decimal)
	}
	p.marks[accountID][instrumentID] = price
	p.invalidatePnLLocked(accountID, instrumentID)
}

func (p *Portfolio) invalidatePnLLocked(accountID model.AccountID, instrumentID model.InstrumentID) {
	if p.unrealizedCache[accountID] == nil {
		return
	}
	delete(p.unrealizedCache[accountID], instrumentID)
	if len(p.unrealizedCache[accountID]) == 0 {
		delete(p.unrealizedCache, accountID)
	}
}

func (p *Portfolio) invalidateAccountPnLLocked(accountID model.AccountID) {
	delete(p.unrealizedCache, accountID)
}

func (p *Portfolio) RealizedPnL(accountID model.AccountID, instrumentID model.InstrumentID) decimal.Decimal {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.realized[accountID] == nil {
		return decimal.Zero
	}
	return p.realized[accountID][instrumentID]
}

func (p *Portfolio) UnrealizedPnL(accountID model.AccountID, instrumentID model.InstrumentID) decimal.Decimal {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.unrealizedPnLLocked(accountID, instrumentID)
}

func (p *Portfolio) unrealizedPnLLocked(accountID model.AccountID, instrumentID model.InstrumentID) decimal.Decimal {
	if p.unrealizedCache[accountID] != nil {
		if value, ok := p.unrealizedCache[accountID][instrumentID]; ok {
			return value
		}
	}
	value := p.computeUnrealizedPnLLocked(accountID, instrumentID)
	if p.unrealizedCache[accountID] == nil {
		p.unrealizedCache[accountID] = make(map[model.InstrumentID]decimal.Decimal)
	}
	p.unrealizedCache[accountID][instrumentID] = value
	return value
}

func (p *Portfolio) computeUnrealizedPnLLocked(accountID model.AccountID, instrumentID model.InstrumentID) decimal.Decimal {
	position, ok := p.cache.PositionByInstrument(accountID, instrumentID)
	if !ok || position.Side == model.PositionSideFlat || position.Quantity.IsZero() {
		return decimal.Zero
	}
	mark := p.marks[accountID][instrumentID]
	if !mark.IsPositive() {
		return decimal.Zero
	}
	if position.Side == model.PositionSideShort {
		return position.EntryPrice.Sub(mark).Mul(position.Quantity)
	}
	return mark.Sub(position.EntryPrice).Mul(position.Quantity)
}

func (p *Portfolio) Commission(accountID model.AccountID, currency model.Currency) decimal.Decimal {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.commissions[accountID] == nil {
		return decimal.Zero
	}
	return p.commissions[accountID][currency]
}

func (p *Portfolio) BalancesLocked(accountID model.AccountID) map[model.Currency]decimal.Decimal {
	account, ok := p.cache.Account(accountID)
	if !ok {
		return map[model.Currency]decimal.Decimal{}
	}
	return p.balancesLockedLocked(account)
}

func (p *Portfolio) balancesLockedLocked(account model.AccountSnapshot) map[model.Currency]decimal.Decimal {
	locked := make(map[model.Currency]decimal.Decimal)
	for _, balance := range account.Balances {
		amount, err := balance.LockedAmount()
		if err != nil {
			continue
		}
		locked[balance.Currency] = locked[balance.Currency].Add(amount)
	}
	return locked
}

func (p *Portfolio) MarginsInit(accountID model.AccountID) map[model.InstrumentID]decimal.Decimal {
	return p.marginsByInstrument(accountID, true)
}

func (p *Portfolio) MarginsMaint(accountID model.AccountID) map[model.InstrumentID]decimal.Decimal {
	return p.marginsByInstrument(accountID, false)
}

func (p *Portfolio) RealizedPnLs(accountID model.AccountID) map[model.Currency]decimal.Decimal {
	p.mu.RLock()
	defer p.mu.RUnlock()
	values := make(map[model.Currency]decimal.Decimal)
	for instrumentID, amount := range p.realized[accountID] {
		currency := p.pnlCurrency(instrumentID)
		if currency == "" {
			continue
		}
		values[currency] = values[currency].Add(amount)
	}
	return p.convertForAccountBaseLocked(accountID, values)
}

func (p *Portfolio) UnrealizedPnLs(accountID model.AccountID) map[model.Currency]decimal.Decimal {
	p.mu.RLock()
	defer p.mu.RUnlock()
	values := p.unrealizedPnLsLocked(accountID)
	return p.convertForAccountBaseLocked(accountID, values)
}

func (p *Portfolio) unrealizedPnLsLocked(accountID model.AccountID) map[model.Currency]decimal.Decimal {
	values := make(map[model.Currency]decimal.Decimal)
	for _, inst := range p.cache.Instruments() {
		amount := p.unrealizedPnLLocked(accountID, inst.ID)
		if amount.IsZero() {
			continue
		}
		currency := settlementCurrency(inst)
		if currency == "" {
			continue
		}
		values[currency] = values[currency].Add(amount)
	}
	return values
}

func (p *Portfolio) TotalPnLs(accountID model.AccountID) map[model.Currency]decimal.Decimal {
	values := p.RealizedPnLs(accountID)
	for currency, amount := range p.UnrealizedPnLs(accountID) {
		values[currency] = values[currency].Add(amount)
	}
	return values
}

func (p *Portfolio) Equity(accountID model.AccountID) map[model.Currency]decimal.Decimal {
	p.mu.RLock()
	defer p.mu.RUnlock()
	account, ok := p.cache.Account(accountID)
	if !ok {
		return map[model.Currency]decimal.Decimal{}
	}
	return p.convertForAccountLocked(account, p.equityNativeLocked(account))
}

func (p *Portfolio) equityNativeLocked(account model.AccountSnapshot) map[model.Currency]decimal.Decimal {
	values := p.balanceTotals(account)
	if account.Type == model.AccountTypeCash {
		for currency, amount := range p.markValuesLocked(account.AccountID) {
			values[currency] = values[currency].Add(amount)
		}
		return values
	}
	for currency, amount := range p.unrealizedPnLsLocked(account.AccountID) {
		values[currency] = values[currency].Add(amount)
	}
	return values
}

func (p *Portfolio) AvailableEquity(accountID model.AccountID) map[model.Currency]decimal.Decimal {
	p.mu.RLock()
	defer p.mu.RUnlock()
	account, ok := p.cache.Account(accountID)
	if !ok {
		return map[model.Currency]decimal.Decimal{}
	}
	values := p.equityNativeLocked(account)
	for currency, amount := range p.balancesLockedLocked(account) {
		values[currency] = values[currency].Sub(amount)
	}
	for currency, amount := range p.marginTotalsFromAccount(account, true) {
		values[currency] = values[currency].Sub(amount)
	}
	return p.convertForAccountLocked(account, values)
}

func (p *Portfolio) Exposure(accountID model.AccountID, quote model.Currency) decimal.Decimal {
	p.mu.RLock()
	defer p.mu.RUnlock()
	total := decimal.Zero
	for _, inst := range p.cache.Instruments() {
		position, ok := p.cache.PositionByInstrument(accountID, inst.ID)
		if !ok || position.Side == model.PositionSideFlat || position.Quantity.IsZero() {
			continue
		}
		price := position.EntryPrice
		if mark := p.marks[accountID][inst.ID]; mark.IsPositive() {
			price = mark
		}
		value := position.Quantity.Abs().Mul(price)
		currency := settlementCurrency(inst)
		if currency != quote {
			converted, ok := p.convertAmountLocked(value, currency, quote)
			if !ok {
				continue
			}
			value = converted
		}
		total = total.Add(value)
	}
	return total
}

func (p *Portfolio) NetPosition(accountID model.AccountID, instrumentID model.InstrumentID) decimal.Decimal {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if accountID != "" {
		position, ok := p.cache.PositionByInstrument(accountID, instrumentID)
		if !ok || position.Side == model.PositionSideFlat || position.Quantity.IsZero() {
			return decimal.Zero
		}
		return signedPosition(position)
	}
	total := decimal.Zero
	for _, position := range p.cache.PositionsForInstrument(instrumentID) {
		if position.Side == model.PositionSideFlat || position.Quantity.IsZero() {
			continue
		}
		total = total.Add(signedPosition(position))
	}
	return total
}

func (p *Portfolio) NetPositionsByInstrument(accountID model.AccountID) map[model.InstrumentID]decimal.Decimal {
	p.mu.RLock()
	defer p.mu.RUnlock()
	values := make(map[model.InstrumentID]decimal.Decimal)
	for _, inst := range p.cache.Instruments() {
		total := decimal.Zero
		if accountID != "" {
			position, ok := p.cache.PositionByInstrument(accountID, inst.ID)
			if ok && position.Side != model.PositionSideFlat && !position.Quantity.IsZero() {
				total = signedPosition(position)
			}
		} else {
			for _, position := range p.cache.PositionsForInstrument(inst.ID) {
				if position.Side == model.PositionSideFlat || position.Quantity.IsZero() {
					continue
				}
				total = total.Add(signedPosition(position))
			}
		}
		if !total.IsZero() {
			values[inst.ID] = total
		}
	}
	return values
}

func (p *Portfolio) NetExposuresByInstrument(accountID model.AccountID, target model.Currency) map[model.InstrumentID]decimal.Decimal {
	p.mu.RLock()
	defer p.mu.RUnlock()
	values := make(map[model.InstrumentID]decimal.Decimal)
	for _, inst := range p.cache.Instruments() {
		total := decimal.Zero
		for _, position := range p.positionsForInstrumentLocked(accountID, inst.ID) {
			value, ok := p.signedExposureLocked(position, inst, target)
			if !ok {
				continue
			}
			total = total.Add(value)
		}
		if !total.IsZero() {
			values[inst.ID] = total
		}
	}
	return values
}

func (p *Portfolio) NetExposuresByAccount(target model.Currency) map[model.AccountID]decimal.Decimal {
	p.mu.RLock()
	defer p.mu.RUnlock()
	values := make(map[model.AccountID]decimal.Decimal)
	for _, inst := range p.cache.Instruments() {
		for _, position := range p.positionsForInstrumentLocked("", inst.ID) {
			value, ok := p.signedExposureLocked(position, inst, target)
			if !ok {
				continue
			}
			values[position.AccountID] = values[position.AccountID].Add(value)
		}
	}
	return values
}

func (p *Portfolio) NetExposuresByVenue(target model.Currency) map[model.Venue]decimal.Decimal {
	p.mu.RLock()
	defer p.mu.RUnlock()
	values := make(map[model.Venue]decimal.Decimal)
	for _, inst := range p.cache.Instruments() {
		for _, position := range p.positionsForInstrumentLocked("", inst.ID) {
			value, ok := p.signedExposureLocked(position, inst, target)
			if !ok {
				continue
			}
			values[inst.ID.Venue] = values[inst.ID.Venue].Add(value)
		}
	}
	return values
}

func (p *Portfolio) positionsForInstrumentLocked(accountID model.AccountID, instrumentID model.InstrumentID) []model.PositionStatusReport {
	if accountID != "" {
		position, ok := p.cache.PositionByInstrument(accountID, instrumentID)
		if !ok {
			return nil
		}
		return []model.PositionStatusReport{position}
	}
	return p.cache.PositionsForInstrument(instrumentID)
}

func (p *Portfolio) signedExposureLocked(position model.PositionStatusReport, inst model.Instrument, target model.Currency) (decimal.Decimal, bool) {
	if position.Side == model.PositionSideFlat || position.Quantity.IsZero() {
		return decimal.Zero, false
	}
	price := position.EntryPrice
	if mark := p.marks[position.AccountID][position.InstrumentID]; mark.IsPositive() {
		price = mark
	}
	if !price.IsPositive() {
		return decimal.Zero, false
	}
	value := signedPosition(position).Mul(price)
	currency := settlementCurrency(inst)
	if target == "" || currency == target {
		return value, true
	}
	converted, ok := p.convertAmountLocked(value, currency, target)
	if !ok {
		return decimal.Zero, false
	}
	return converted, true
}

func (p *Portfolio) MarkValue(accountID model.AccountID, instrumentID model.InstrumentID) decimal.Decimal {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.markValueLocked(accountID, instrumentID)
}

func (p *Portfolio) markValueLocked(accountID model.AccountID, instrumentID model.InstrumentID) decimal.Decimal {
	position, ok := p.cache.PositionByInstrument(accountID, instrumentID)
	if !ok || position.Side == model.PositionSideFlat || position.Quantity.IsZero() {
		return decimal.Zero
	}
	price := position.EntryPrice
	if mark := p.marks[accountID][instrumentID]; mark.IsPositive() {
		price = mark
	}
	value := position.Quantity.Abs().Mul(price)
	if position.Side == model.PositionSideShort {
		return value.Neg()
	}
	return value
}

func (p *Portfolio) MarkValues(accountID model.AccountID) map[model.Currency]decimal.Decimal {
	p.mu.RLock()
	defer p.mu.RUnlock()
	values := p.markValuesLocked(accountID)
	return p.convertForAccountBaseLocked(accountID, values)
}

func (p *Portfolio) markValuesLocked(accountID model.AccountID) map[model.Currency]decimal.Decimal {
	values := make(map[model.Currency]decimal.Decimal)
	for _, inst := range p.cache.Instruments() {
		value := p.markValueLocked(accountID, inst.ID)
		if value.IsZero() {
			continue
		}
		currency := inst.Settle
		if currency == "" {
			currency = inst.Quote
		}
		if currency == "" {
			continue
		}
		values[currency] = values[currency].Add(value)
	}
	return values
}

func (p *Portfolio) balanceTotals(account model.AccountSnapshot) map[model.Currency]decimal.Decimal {
	values := make(map[model.Currency]decimal.Decimal)
	for _, balance := range account.Balances {
		amount, err := balance.TotalAmount()
		if err != nil {
			continue
		}
		values[balance.Currency] = values[balance.Currency].Add(amount)
	}
	return values
}

func (p *Portfolio) marginsByInstrument(accountID model.AccountID, initial bool) map[model.InstrumentID]decimal.Decimal {
	account, ok := p.cache.Account(accountID)
	if !ok {
		return map[model.InstrumentID]decimal.Decimal{}
	}
	values := make(map[model.InstrumentID]decimal.Decimal)
	for _, margin := range account.Margins {
		if margin.InstrumentID == (model.InstrumentID{}) {
			continue
		}
		initAmount, maintAmount, err := margin.Amounts()
		if err != nil {
			continue
		}
		amount := maintAmount
		if initial {
			amount = initAmount
		}
		values[margin.InstrumentID] = values[margin.InstrumentID].Add(amount)
	}
	return values
}

func (p *Portfolio) marginTotals(accountID model.AccountID, initial bool) map[model.Currency]decimal.Decimal {
	account, ok := p.cache.Account(accountID)
	if !ok {
		return map[model.Currency]decimal.Decimal{}
	}
	return p.marginTotalsFromAccount(account, initial)
}

func (p *Portfolio) marginTotalsFromAccount(account model.AccountSnapshot, initial bool) map[model.Currency]decimal.Decimal {
	values := make(map[model.Currency]decimal.Decimal)
	for _, margin := range account.Margins {
		initAmount, maintAmount, err := margin.Amounts()
		if err != nil {
			continue
		}
		amount := maintAmount
		if initial {
			amount = initAmount
		}
		values[margin.Currency] = values[margin.Currency].Add(amount)
	}
	return values
}

func (p *Portfolio) convertForAccountBaseLocked(accountID model.AccountID, values map[model.Currency]decimal.Decimal) map[model.Currency]decimal.Decimal {
	account, ok := p.cache.Account(accountID)
	if !ok {
		return values
	}
	return p.convertForAccountLocked(account, values)
}

func (p *Portfolio) convertForAccountLocked(account model.AccountSnapshot, values map[model.Currency]decimal.Decimal) map[model.Currency]decimal.Decimal {
	if account.BaseCurrency == "" {
		return values
	}
	return p.convertValuesLocked(values, account.BaseCurrency)
}

func (p *Portfolio) convertValuesLocked(values map[model.Currency]decimal.Decimal, target model.Currency) map[model.Currency]decimal.Decimal {
	converted := make(map[model.Currency]decimal.Decimal)
	for currency, amount := range values {
		value, ok := p.convertAmountLocked(amount, currency, target)
		if !ok {
			continue
		}
		converted[target] = converted[target].Add(value)
	}
	return converted
}

func (p *Portfolio) convertAmountLocked(amount decimal.Decimal, from model.Currency, to model.Currency) (decimal.Decimal, bool) {
	if from == "" || to == "" {
		return decimal.Zero, false
	}
	if from == to {
		return amount, true
	}
	rate, ok := p.exchangeRateLocked(from, to)
	if !ok {
		return decimal.Zero, false
	}
	return amount.Mul(rate), true
}

func (p *Portfolio) exchangeRateLocked(from model.Currency, to model.Currency) (decimal.Decimal, bool) {
	if rate, ok := p.conversions[from][to]; ok && rate.IsPositive() {
		return rate, true
	}
	if rate, ok := p.conversions[to][from]; ok && rate.IsPositive() {
		return decimal.NewFromInt(1).Div(rate), true
	}
	for _, inst := range p.cache.Instruments() {
		price, ok := p.xratePriceLocked(inst.ID)
		if !ok || !price.IsPositive() {
			continue
		}
		if inst.Base == from && inst.Quote == to {
			return price, true
		}
		if inst.Base == to && inst.Quote == from {
			return decimal.NewFromInt(1).Div(price), true
		}
	}
	return decimal.Zero, false
}

func (p *Portfolio) xratePriceLocked(instrumentID model.InstrumentID) (decimal.Decimal, bool) {
	if quote, ok := p.cache.QuoteTick(instrumentID); ok {
		switch {
		case quote.BidPrice.IsPositive() && quote.AskPrice.IsPositive():
			return quote.BidPrice.Add(quote.AskPrice).Div(decimal.NewFromInt(2)), true
		case quote.BidPrice.IsPositive():
			return quote.BidPrice, true
		case quote.AskPrice.IsPositive():
			return quote.AskPrice, true
		}
	}
	if ticker, ok := p.cache.Ticker(instrumentID); ok {
		if ticker.Bid.IsPositive() && ticker.Ask.IsPositive() {
			return ticker.Bid.Add(ticker.Ask).Div(decimal.NewFromInt(2)), true
		}
		if ticker.Last.IsPositive() {
			return ticker.Last, true
		}
	}
	if trade, ok := p.cache.TradeTick(instrumentID); ok && trade.Price.IsPositive() {
		return trade.Price, true
	}
	if bar, ok := p.cache.LatestBar(instrumentID); ok && bar.Close.IsPositive() {
		return bar.Close, true
	}
	return decimal.Zero, false
}

func (p *Portfolio) pnlCurrency(instrumentID model.InstrumentID) model.Currency {
	inst, ok := p.cache.Instrument(instrumentID)
	if !ok {
		return ""
	}
	return settlementCurrency(inst)
}

func settlementCurrency(inst model.Instrument) model.Currency {
	if inst.Settle != "" {
		return inst.Settle
	}
	return inst.Quote
}

func (p *Portfolio) nextPosition(fill model.FillReport) model.PositionStatusReport {
	positionID := model.PositionID(fill.InstrumentID.String())
	if fill.PositionID != "" {
		positionID = fill.PositionID
	}
	existing, ok := p.cache.Position(fill.AccountID, positionID)
	if !ok {
		return model.PositionStatusReport{
			AccountID:    fill.AccountID,
			InstrumentID: fill.InstrumentID,
			PositionID:   positionID,
			Side:         sideFromSigned(fillSignedQuantity(fill)),
			Quantity:     fill.Quantity,
			EntryPrice:   fill.Price,
			Timestamp:    fill.Timestamp,
		}
	}
	signed := signedPosition(existing).Add(fillSignedQuantity(fill))
	side := sideFromSigned(signed)
	absQty := signed.Abs()
	entry := fill.Price
	if sameDirection(existing, fill) {
		totalQty := existing.Quantity.Add(fill.Quantity)
		if totalQty.IsPositive() {
			entry = existing.EntryPrice.Mul(existing.Quantity).Add(fill.Price.Mul(fill.Quantity)).Div(totalQty)
		}
	} else if existing.Quantity.GreaterThan(fill.Quantity) {
		entry = existing.EntryPrice
	}
	if side == model.PositionSideFlat {
		entry = decimal.Zero
	}
	return model.PositionStatusReport{
		AccountID:    fill.AccountID,
		InstrumentID: fill.InstrumentID,
		PositionID:   positionID,
		Side:         side,
		Quantity:     absQty,
		EntryPrice:   entry,
		Timestamp:    fill.Timestamp,
	}
}

func (p *Portfolio) applyCommission(fill model.FillReport) {
	if !fill.Fee.IsPositive() || fill.FeeCurrency == "" {
		return
	}
	if p.commissions[fill.AccountID] == nil {
		p.commissions[fill.AccountID] = make(map[model.Currency]decimal.Decimal)
	}
	p.commissions[fill.AccountID][fill.FeeCurrency] = p.commissions[fill.AccountID][fill.FeeCurrency].Add(fill.Fee)
}

func (p *Portfolio) applyRealizedPnL(fill model.FillReport) {
	position, ok := p.cache.PositionByInstrument(fill.AccountID, fill.InstrumentID)
	if !ok || position.Side == model.PositionSideFlat || position.Quantity.IsZero() {
		return
	}
	p.applyRealizedPnLFromPosition(fill, &position)
}

func (p *Portfolio) applyRealizedPnLFromPosition(fill model.FillReport, position *model.PositionStatusReport) decimal.Decimal {
	if position == nil || position.Side == model.PositionSideFlat || position.Quantity.IsZero() {
		return decimal.Zero
	}
	if sameDirection(*position, fill) {
		return decimal.Zero
	}
	closeQty := decimal.Min(position.Quantity, fill.Quantity)
	if !closeQty.IsPositive() {
		return decimal.Zero
	}
	pnl := fill.Price.Sub(position.EntryPrice).Mul(closeQty)
	if position.Side == model.PositionSideShort {
		pnl = position.EntryPrice.Sub(fill.Price).Mul(closeQty)
	}
	if p.realized[fill.AccountID] == nil {
		p.realized[fill.AccountID] = make(map[model.InstrumentID]decimal.Decimal)
	}
	p.realized[fill.AccountID][fill.InstrumentID] = p.realized[fill.AccountID][fill.InstrumentID].Add(pnl)
	return pnl
}

func (p *Portfolio) applyFillBalanceDeltas(fill model.FillReport, realizedDelta decimal.Decimal) error {
	deltas := make(map[model.Currency]decimal.Decimal)
	if !realizedDelta.IsZero() {
		if currency := p.pnlCurrency(fill.InstrumentID); currency != "" {
			deltas[currency] = deltas[currency].Add(realizedDelta)
		}
	}
	if fill.Fee.IsPositive() && fill.FeeCurrency != "" {
		deltas[fill.FeeCurrency] = deltas[fill.FeeCurrency].Sub(fill.Fee)
	}
	if len(deltas) == 0 {
		return nil
	}
	account, ok := p.cache.Account(fill.AccountID)
	if !ok {
		return nil
	}
	updated, err := accountWithBalanceDeltas(account, deltas)
	if err != nil {
		return err
	}
	p.cache.PutAccount(updated)
	return nil
}

func (p *Portfolio) recordClosedTradeLocked(fill model.FillReport, previous *model.PositionStatusReport, position model.PositionStatusReport, realizedDelta decimal.Decimal) {
	if p.analyzer == nil || previous == nil || realizedDelta.IsZero() {
		return
	}
	if previous.Side == model.PositionSideFlat || previous.Quantity.IsZero() {
		return
	}
	closed := position.Side == model.PositionSideFlat || position.Side != previous.Side
	if !closed {
		return
	}
	currency := p.pnlCurrency(fill.InstrumentID)
	if currency == "" {
		return
	}
	accountCurrency := currency
	accountCurrencyPnL := realizedDelta
	if account, ok := p.cache.Account(fill.AccountID); ok && account.BaseCurrency != "" {
		if converted, ok := p.convertAmountLocked(realizedDelta, currency, account.BaseCurrency); ok {
			accountCurrency = account.BaseCurrency
			accountCurrencyPnL = converted
		}
	}
	p.analyzer.RecordTrade(TradeRecord{
		AccountID:          fill.AccountID,
		InstrumentID:       fill.InstrumentID,
		PositionID:         previous.PositionID,
		Currency:           currency,
		RealizedPnL:        realizedDelta,
		AccountCurrency:    accountCurrency,
		AccountCurrencyPnL: accountCurrencyPnL,
	})
}

func accountWithBalanceDeltas(account model.AccountSnapshot, deltas map[model.Currency]decimal.Decimal) (model.AccountSnapshot, error) {
	updated := account
	updated.Balances = append([]model.Balance(nil), account.Balances...)
	for currency, delta := range deltas {
		if delta.IsZero() {
			continue
		}
		idx := -1
		for i, balance := range updated.Balances {
			if balance.Currency == currency {
				idx = i
				break
			}
		}
		if idx == -1 {
			if delta.IsNegative() {
				return model.AccountSnapshot{}, fmt.Errorf("%w: missing balance for %s", model.ErrInvalidAccount, currency)
			}
			updated.Balances = append(updated.Balances, model.Balance{
				Currency: currency,
				Free:     delta.String(),
				Total:    delta.String(),
			})
			continue
		}
		total, locked, free, err := updated.Balances[idx].Amounts()
		if err != nil {
			return model.AccountSnapshot{}, err
		}
		total = total.Add(delta)
		free = free.Add(delta)
		if total.IsNegative() || free.IsNegative() {
			return model.AccountSnapshot{}, fmt.Errorf("%w: balance delta makes %s negative", model.ErrInvalidAccount, currency)
		}
		updated.Balances[idx] = model.Balance{
			Currency: currency,
			Free:     free.String(),
			Locked:   locked.String(),
			Total:    total.String(),
		}
	}
	if err := updated.Validate(); err != nil {
		return model.AccountSnapshot{}, err
	}
	return updated, nil
}

func sameDirection(position model.PositionStatusReport, fill model.FillReport) bool {
	return (position.Side == model.PositionSideLong && fill.Side == model.OrderSideBuy) ||
		(position.Side == model.PositionSideShort && fill.Side == model.OrderSideSell)
}

func signedPosition(position model.PositionStatusReport) decimal.Decimal {
	if position.Side == model.PositionSideShort {
		return position.Quantity.Neg()
	}
	return position.Quantity
}

func fillSignedQuantity(fill model.FillReport) decimal.Decimal {
	if fill.Side == model.OrderSideSell {
		return fill.Quantity.Neg()
	}
	return fill.Quantity
}

func sideFromSigned(qty decimal.Decimal) model.PositionSide {
	if qty.IsPositive() {
		return model.PositionSideLong
	}
	if qty.IsNegative() {
		return model.PositionSideShort
	}
	return model.PositionSideFlat
}

func marketEventMark(event model.MarketEvent, side model.PositionSide) decimal.Decimal {
	switch {
	case event.Quote != nil:
		if side == model.PositionSideShort {
			return event.Quote.AskPrice
		}
		return event.Quote.BidPrice
	case event.OrderBook != nil:
		if side == model.PositionSideShort && len(event.OrderBook.Asks) > 0 {
			return event.OrderBook.Asks[0].Price
		}
		if side == model.PositionSideLong && len(event.OrderBook.Bids) > 0 {
			return event.OrderBook.Bids[0].Price
		}
	case event.Ticker != nil:
		if event.Ticker.Last.IsPositive() {
			return event.Ticker.Last
		}
		if side == model.PositionSideShort {
			return event.Ticker.Ask
		}
		return event.Ticker.Bid
	case event.Trade != nil:
		return event.Trade.Price
	case event.Bar != nil:
		return event.Bar.Close
	}
	return decimal.Zero
}
