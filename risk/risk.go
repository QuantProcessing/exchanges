package risk

import (
	"errors"
	"fmt"

	"github.com/QuantProcessing/exchanges/cache"
	"github.com/QuantProcessing/exchanges/model"
	"github.com/shopspring/decimal"
)

var ErrRiskRejected = errors.New("risk rejected")

type TradingState string

const (
	TradingStateActive   TradingState = "active"
	TradingStateHalted   TradingState = "halted"
	TradingStateReducing TradingState = "reducing"
)

type Config struct {
	MaxOrderNotional    decimal.Decimal
	MaxPositionNotional decimal.Decimal
	MaxAccountExposure  decimal.Decimal
	ExposureCurrency    model.Currency
	TradingState        TradingState
}

type Engine struct {
	cache *cache.Cache
	cfg   Config
}

func NewEngine(c *cache.Cache, cfg Config) *Engine {
	if c == nil {
		c = cache.New()
	}
	return &Engine{cache: c, cfg: cfg}
}

func (e *Engine) Check(order model.SubmitOrder) error {
	if err := order.Validate(); err != nil {
		return err
	}
	inst, ok := e.cache.Instrument(order.InstrumentID)
	if !ok {
		return fmt.Errorf("%w: %s", model.ErrInstrumentNotFound, order.InstrumentID.String())
	}
	if inst.Status != model.InstrumentStatusTrading {
		return fmt.Errorf("%w: instrument is not trading", ErrRiskRejected)
	}
	switch e.cfg.TradingState {
	case "", TradingStateActive:
	case TradingStateHalted:
		return fmt.Errorf("%w: trading state halted", ErrRiskRejected)
	case TradingStateReducing:
		if e.increasesCurrentExposure(order) {
			return fmt.Errorf("%w: reducing state rejects exposure increase", ErrRiskRejected)
		}
	default:
		return fmt.Errorf("%w: invalid trading state %q", ErrRiskRejected, e.cfg.TradingState)
	}
	if err := inst.ValidateSize(order.Quantity); err != nil {
		return err
	}
	if requiresLimitPrice(order.Type) {
		if err := inst.ValidatePrice(order.Price); err != nil {
			return err
		}
	}
	if order.TriggerPrice.IsPositive() {
		if err := inst.ValidatePrice(order.TriggerPrice); err != nil {
			return err
		}
	}
	if order.ActivationPrice.IsPositive() {
		if err := inst.ValidatePrice(order.ActivationPrice); err != nil {
			return err
		}
	}
	price := e.estimatedPrice(order)
	if e.cfg.MaxOrderNotional.IsPositive() {
		if !price.IsPositive() {
			return fmt.Errorf("%w: cannot estimate order notional", ErrRiskRejected)
		}
		if price.Mul(order.Quantity).GreaterThan(e.cfg.MaxOrderNotional) {
			return fmt.Errorf("%w: max order notional exceeded", ErrRiskRejected)
		}
	}
	if e.cfg.MaxPositionNotional.IsPositive() {
		if !price.IsPositive() {
			return fmt.Errorf("%w: cannot estimate position notional", ErrRiskRejected)
		}
		if e.projectedPositionNotional(order, price).GreaterThan(e.cfg.MaxPositionNotional) {
			return fmt.Errorf("%w: max position notional exceeded", ErrRiskRejected)
		}
	}
	if e.cfg.MaxAccountExposure.IsPositive() {
		if !price.IsPositive() {
			return fmt.Errorf("%w: cannot estimate account exposure", ErrRiskRejected)
		}
		exposure, ok := e.projectedAccountExposure(order, price)
		if !ok {
			return fmt.Errorf("%w: cannot estimate account exposure", ErrRiskRejected)
		}
		if exposure.GreaterThan(e.cfg.MaxAccountExposure) {
			return fmt.Errorf("%w: max account exposure exceeded", ErrRiskRejected)
		}
	}
	if err := e.checkAvailableInitialMargin(order, inst, price); err != nil {
		return err
	}
	if order.ReduceOnly && e.increasesCurrentExposure(order) {
		return fmt.Errorf("%w: reduce-only would increase exposure", ErrRiskRejected)
	}
	return nil
}

func (e *Engine) estimatedPrice(order model.SubmitOrder) decimal.Decimal {
	if order.Price.IsPositive() {
		return order.Price
	}
	if quote, ok := e.cache.QuoteTick(order.InstrumentID); ok {
		if order.Side == model.OrderSideBuy && quote.AskPrice.IsPositive() {
			return quote.AskPrice
		}
		if order.Side == model.OrderSideSell && quote.BidPrice.IsPositive() {
			return quote.BidPrice
		}
	}
	if ticker, ok := e.cache.Ticker(order.InstrumentID); ok {
		if order.Side == model.OrderSideBuy && ticker.Ask.IsPositive() {
			return ticker.Ask
		}
		if order.Side == model.OrderSideSell && ticker.Bid.IsPositive() {
			return ticker.Bid
		}
		if ticker.Last.IsPositive() {
			return ticker.Last
		}
	}
	if book, ok := e.cache.OrderBook(order.InstrumentID); ok {
		if order.Side == model.OrderSideBuy && len(book.Asks) > 0 {
			return book.Asks[0].Price
		}
		if order.Side == model.OrderSideSell && len(book.Bids) > 0 {
			return book.Bids[0].Price
		}
	}
	if trade, ok := e.cache.TradeTick(order.InstrumentID); ok && trade.Price.IsPositive() {
		return trade.Price
	}
	if bar, ok := e.cache.LatestBar(order.InstrumentID); ok && bar.Close.IsPositive() {
		return bar.Close
	}
	return decimal.Zero
}

func (e *Engine) projectedPositionNotional(order model.SubmitOrder, price decimal.Decimal) decimal.Decimal {
	return e.projectedSignedPosition(order).Abs().Mul(price)
}

func (e *Engine) projectedAccountExposure(order model.SubmitOrder, orderPrice decimal.Decimal) (decimal.Decimal, bool) {
	total := decimal.Zero
	target := e.accountExposureCurrency(order.AccountID)
	for _, inst := range e.cache.Instruments() {
		projected := e.signedPositionWithOpenOrders(order.AccountID, inst.ID)
		if inst.ID == order.InstrumentID {
			projected = applySignedOrder(projected, order.Side, order.Quantity)
		}
		if projected.IsZero() {
			continue
		}
		price := orderPrice
		if inst.ID != order.InstrumentID {
			price = e.exposurePrice(order.AccountID, inst.ID, projected)
		}
		if price.IsPositive() {
			value := projected.Abs().Mul(price)
			if target != "" {
				converted, ok := e.convertAmount(value, marginCurrency(inst), target)
				if !ok {
					return decimal.Zero, false
				}
				value = converted
			}
			total = total.Add(value)
		}
	}
	return total, true
}

func (e *Engine) accountExposureCurrency(accountID model.AccountID) model.Currency {
	if e.cfg.ExposureCurrency != "" {
		return e.cfg.ExposureCurrency
	}
	account, ok := e.cache.Account(accountID)
	if !ok {
		return ""
	}
	return account.BaseCurrency
}

func (e *Engine) checkAvailableInitialMargin(order model.SubmitOrder, inst model.Instrument, price decimal.Decimal) error {
	if !inst.MarginInit.IsPositive() {
		return nil
	}
	if !e.increasesProjectedExposure(order) {
		return nil
	}
	if !price.IsPositive() {
		return fmt.Errorf("%w: cannot estimate initial margin", ErrRiskRejected)
	}
	account, ok := e.cache.Account(order.AccountID)
	if !ok {
		return nil
	}
	if account.Type != "" && account.Type != model.AccountTypeMargin {
		return nil
	}
	currency := marginCurrency(inst)
	if currency == "" {
		return nil
	}
	available, ok := e.availableMargin(account, currency)
	if !ok {
		return fmt.Errorf("%w: missing margin balance for %s", ErrRiskRejected, currency)
	}
	available = available.Sub(e.openOrderInitialMarginReservation(order.AccountID, currency))
	if available.IsNegative() {
		available = decimal.Zero
	}
	required := e.incrementalInitialMargin(order, price, inst.MarginInit)
	if required.GreaterThan(available) {
		return fmt.Errorf("%w: available initial margin exceeded", ErrRiskRejected)
	}
	return nil
}

func (e *Engine) availableMargin(account model.AccountSnapshot, currency model.Currency) (decimal.Decimal, bool) {
	equity := decimal.Zero
	locked := decimal.Zero
	foundBalance := false
	for _, balance := range account.Balances {
		if balance.Currency != currency {
			continue
		}
		total, balanceLocked, _, err := balance.Amounts()
		if err != nil {
			return decimal.Zero, false
		}
		equity = equity.Add(total)
		locked = locked.Add(balanceLocked)
		foundBalance = true
	}
	if !foundBalance {
		return decimal.Zero, false
	}
	for _, inst := range e.cache.Instruments() {
		if marginCurrency(inst) != currency {
			continue
		}
		position, ok := e.cache.PositionByInstrument(account.AccountID, inst.ID)
		if !ok || position.Side == model.PositionSideFlat || position.Quantity.IsZero() {
			continue
		}
		equity = equity.Add(unrealizedPnL(position, e.positionMark(position)))
	}
	initialMargin := decimal.Zero
	for _, margin := range account.Margins {
		if margin.Currency != currency {
			continue
		}
		initial, _, err := margin.Amounts()
		if err != nil {
			return decimal.Zero, false
		}
		initialMargin = initialMargin.Add(initial)
	}
	available := equity.Sub(locked).Sub(initialMargin)
	if available.IsNegative() {
		return decimal.Zero, true
	}
	return available, true
}

func (e *Engine) incrementalInitialMargin(order model.SubmitOrder, price decimal.Decimal, marginRate decimal.Decimal) decimal.Decimal {
	current := e.signedPositionWithOpenOrders(order.AccountID, order.InstrumentID)
	projected := e.projectedSignedPosition(order)
	currentNotional := current.Abs().Mul(price)
	projectedNotional := projected.Abs().Mul(price)
	incremental := projectedNotional.Sub(currentNotional)
	if incremental.IsNegative() {
		return decimal.Zero
	}
	return incremental.Mul(marginRate)
}

func (e *Engine) projectedSignedPosition(order model.SubmitOrder) decimal.Decimal {
	current := e.signedPositionWithOpenOrders(order.AccountID, order.InstrumentID)
	return applySignedOrder(current, order.Side, order.Quantity)
}

func (e *Engine) signedPositionWithOpenOrders(accountID model.AccountID, instrumentID model.InstrumentID) decimal.Decimal {
	current := decimal.Zero
	if position, ok := e.cache.PositionByInstrument(accountID, instrumentID); ok {
		current = signedPosition(position)
	}
	for _, order := range e.cache.OpenOrders(accountID) {
		if order.InstrumentID != instrumentID || order.ReduceOnly {
			continue
		}
		leaves := openOrderLeaves(order)
		if !leaves.IsPositive() {
			continue
		}
		current = applySignedOrder(current, order.Side, leaves)
	}
	return current
}

func (e *Engine) exposurePrice(accountID model.AccountID, instrumentID model.InstrumentID, signed decimal.Decimal) decimal.Decimal {
	side := model.PositionSideLong
	if signed.IsNegative() {
		side = model.PositionSideShort
	}
	if mark := e.markPriceForSide(instrumentID, side); mark.IsPositive() {
		return mark
	}
	if position, ok := e.cache.PositionByInstrument(accountID, instrumentID); ok && position.EntryPrice.IsPositive() {
		return position.EntryPrice
	}
	for _, order := range e.cache.OpenOrders(accountID) {
		if order.InstrumentID == instrumentID && order.Price.IsPositive() {
			return order.Price
		}
	}
	return decimal.Zero
}

func (e *Engine) openOrderInitialMarginReservation(accountID model.AccountID, currency model.Currency) decimal.Decimal {
	reserved := decimal.Zero
	signedByInstrument := make(map[model.InstrumentID]decimal.Decimal)
	for _, inst := range e.cache.Instruments() {
		if marginCurrency(inst) != currency || !inst.MarginInit.IsPositive() {
			continue
		}
		if position, ok := e.cache.PositionByInstrument(accountID, inst.ID); ok {
			signedByInstrument[inst.ID] = signedPosition(position)
		}
	}
	for _, order := range e.cache.OpenOrders(accountID) {
		if order.ReduceOnly {
			continue
		}
		inst, ok := e.cache.Instrument(order.InstrumentID)
		if !ok || marginCurrency(inst) != currency || !inst.MarginInit.IsPositive() {
			continue
		}
		leaves := openOrderLeaves(order)
		if !leaves.IsPositive() {
			continue
		}
		price := order.Price
		if !price.IsPositive() {
			price = e.exposurePrice(accountID, order.InstrumentID, signedByInstrument[order.InstrumentID])
		}
		if !price.IsPositive() {
			continue
		}
		current := signedByInstrument[order.InstrumentID]
		projected := applySignedOrder(current, order.Side, leaves)
		incremental := projected.Abs().Sub(current.Abs())
		if incremental.IsPositive() {
			reserved = reserved.Add(incremental.Mul(price).Mul(inst.MarginInit))
		}
		signedByInstrument[order.InstrumentID] = projected
	}
	return reserved
}

func (e *Engine) positionMark(position model.PositionStatusReport) decimal.Decimal {
	if mark := e.markPrice(position); mark.IsPositive() {
		return mark
	}
	return position.EntryPrice
}

func (e *Engine) markPrice(position model.PositionStatusReport) decimal.Decimal {
	return e.markPriceForSide(position.InstrumentID, position.Side)
}

func (e *Engine) markPriceForSide(instrumentID model.InstrumentID, side model.PositionSide) decimal.Decimal {
	if quote, ok := e.cache.QuoteTick(instrumentID); ok {
		if side == model.PositionSideLong && quote.BidPrice.IsPositive() {
			return quote.BidPrice
		}
		if side == model.PositionSideShort && quote.AskPrice.IsPositive() {
			return quote.AskPrice
		}
	}
	if ticker, ok := e.cache.Ticker(instrumentID); ok {
		if ticker.Last.IsPositive() {
			return ticker.Last
		}
	}
	if book, ok := e.cache.OrderBook(instrumentID); ok {
		if side == model.PositionSideLong && len(book.Bids) > 0 {
			return book.Bids[0].Price
		}
		if side == model.PositionSideShort && len(book.Asks) > 0 {
			return book.Asks[0].Price
		}
	}
	if trade, ok := e.cache.TradeTick(instrumentID); ok && trade.Price.IsPositive() {
		return trade.Price
	}
	if bar, ok := e.cache.LatestBar(instrumentID); ok && bar.Close.IsPositive() {
		return bar.Close
	}
	return decimal.Zero
}

func unrealizedPnL(position model.PositionStatusReport, mark decimal.Decimal) decimal.Decimal {
	if !mark.IsPositive() || position.Side == model.PositionSideFlat || position.Quantity.IsZero() {
		return decimal.Zero
	}
	if position.Side == model.PositionSideShort {
		return position.EntryPrice.Sub(mark).Mul(position.Quantity)
	}
	return mark.Sub(position.EntryPrice).Mul(position.Quantity)
}

func marginCurrency(inst model.Instrument) model.Currency {
	if inst.Settle != "" {
		return inst.Settle
	}
	return inst.Quote
}

func (e *Engine) convertAmount(amount decimal.Decimal, from model.Currency, to model.Currency) (decimal.Decimal, bool) {
	if from == "" || to == "" {
		return decimal.Zero, false
	}
	if from == to {
		return amount, true
	}
	rate, ok := e.exchangeRate(from, to)
	if !ok {
		return decimal.Zero, false
	}
	return amount.Mul(rate), true
}

func (e *Engine) exchangeRate(from model.Currency, to model.Currency) (decimal.Decimal, bool) {
	for _, inst := range e.cache.Instruments() {
		price, ok := e.xratePrice(inst.ID)
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

func (e *Engine) xratePrice(instrumentID model.InstrumentID) (decimal.Decimal, bool) {
	if quote, ok := e.cache.QuoteTick(instrumentID); ok {
		switch {
		case quote.BidPrice.IsPositive() && quote.AskPrice.IsPositive():
			return quote.BidPrice.Add(quote.AskPrice).Div(decimal.NewFromInt(2)), true
		case quote.BidPrice.IsPositive():
			return quote.BidPrice, true
		case quote.AskPrice.IsPositive():
			return quote.AskPrice, true
		}
	}
	if ticker, ok := e.cache.Ticker(instrumentID); ok {
		if ticker.Bid.IsPositive() && ticker.Ask.IsPositive() {
			return ticker.Bid.Add(ticker.Ask).Div(decimal.NewFromInt(2)), true
		}
		if ticker.Last.IsPositive() {
			return ticker.Last, true
		}
	}
	if trade, ok := e.cache.TradeTick(instrumentID); ok && trade.Price.IsPositive() {
		return trade.Price, true
	}
	if bar, ok := e.cache.LatestBar(instrumentID); ok && bar.Close.IsPositive() {
		return bar.Close, true
	}
	return decimal.Zero, false
}

func requiresLimitPrice(t model.OrderType) bool {
	return t == model.OrderTypeLimit ||
		t == model.OrderTypeStopLimit ||
		t == model.OrderTypeLimitIfTouched ||
		t == model.OrderTypeTrailingStopLimit
}

func signedPosition(position model.PositionStatusReport) decimal.Decimal {
	if position.Side == model.PositionSideShort {
		return position.Quantity.Neg()
	}
	return position.Quantity
}

func openOrderLeaves(order model.OrderStatusReport) decimal.Decimal {
	if order.LeavesQuantity.IsPositive() {
		return order.LeavesQuantity
	}
	if order.Quantity.IsPositive() {
		leaves := order.Quantity.Sub(order.FilledQuantity)
		if leaves.IsPositive() {
			return leaves
		}
	}
	return decimal.Zero
}

func applySignedOrder(current decimal.Decimal, side model.OrderSide, quantity decimal.Decimal) decimal.Decimal {
	if side == model.OrderSideSell {
		return current.Sub(quantity)
	}
	return current.Add(quantity)
}

func (e *Engine) increasesCurrentExposure(order model.SubmitOrder) bool {
	current := decimal.Zero
	if position, ok := e.cache.PositionByInstrument(order.AccountID, order.InstrumentID); ok {
		current = signedPosition(position)
	}
	projected := applySignedOrder(current, order.Side, order.Quantity)
	return exposureIncreased(current, projected)
}

func (e *Engine) increasesProjectedExposure(order model.SubmitOrder) bool {
	current := e.signedPositionWithOpenOrders(order.AccountID, order.InstrumentID)
	projected := applySignedOrder(current, order.Side, order.Quantity)
	return exposureIncreased(current, projected)
}

func exposureIncreased(current, projected decimal.Decimal) bool {
	if current.IsZero() {
		return !projected.IsZero()
	}
	if projected.IsZero() {
		return false
	}
	if current.Sign()*projected.Sign() < 0 {
		return true
	}
	return projected.Abs().GreaterThan(current.Abs())
}
