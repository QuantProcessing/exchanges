package okx

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/okx/sdk"

	"github.com/shopspring/decimal"
)

// Adapter OKX 适配器
type Adapter struct {
	*exchanges.BaseAdapter

	client    *okx.Client
	wsPublic  *okx.WSClient
	wsPrivate *okx.WSClient

	// Symbol mapping: Token -> InstId (e.g. BTC -> BTC-USDT-SWAP)
	symbolMap     map[string]string         // BTC -> BTC-USDT-SWAP
	idMap         map[string]string         // BTC-USDT-SWAP -> BTC
	instruments   map[string]okx.Instrument // InstId -> Instrument
	quoteCurrency string                    // "USDT" or "USDC"

	accountLevel okx.AccountLevel
	posMode      string // long_short_mode or net_mode

	mu sync.RWMutex

	// Cached fee rate (account-level, not per-symbol)
	feeOnce       sync.Once
	cachedFeeRate *exchanges.FeeRate
	cachedFeeErr  error

	privateOrderStream okxPrivateOrderStreamState
}

// NewAdapter creates a new OKX adapter
func NewAdapter(ctx context.Context, opts Options) (*Adapter, error) {
	quote, err := opts.quoteCurrency()
	if err != nil {
		return nil, err
	}
	return newPerpAdapterWithClient(ctx, opts, quote, okx.NewClient())
}

func newPerpAdapterWithClient(ctx context.Context, opts Options, quote exchanges.QuoteCurrency, client *okx.Client) (*Adapter, error) {
	if err := opts.validateCredentials(); err != nil {
		return nil, err
	}
	wsPublic := okx.NewWSClient(ctx)
	wsPrivate := okx.NewWSClient(ctx)

	if opts.hasFullCredentials() {
		client.WithCredentials(opts.APIKey, opts.SecretKey, opts.Passphrase)
		wsPrivate.WithCredentials(opts.APIKey, opts.SecretKey, opts.Passphrase)
	}

	base := exchanges.NewBaseAdapter("OKX", exchanges.MarketTypePerp, opts.logger())

	a := &Adapter{
		BaseAdapter:   base,
		client:        client,
		wsPublic:      wsPublic,
		wsPrivate:     wsPrivate,
		symbolMap:     make(map[string]string),
		idMap:         make(map[string]string),
		instruments:   make(map[string]okx.Instrument),
		quoteCurrency: string(quote),
		accountLevel:  okx.AccountLevelUnknown,
		posMode:       "net_mode", // default
	}

	// Load Instruments
	if err := a.RefreshSymbolDetails(context.Background()); err != nil {
		// TODO: logger.Error("Failed to load instruments", zap.Error(err))
	}

	// Load Position Mode
	if opts.hasFullCredentials() {
		if err := a.RefreshPositionMode(context.Background()); err != nil {
			// TODO: logger.Warn("Failed to load position mode, using default", zap.Error(err))
		}
	}

	// TODO: logger.Info("Initialized OKX Adapter", zap.String("posMode", a.posMode))
	return a, nil
}

func (a *Adapter) WsAccountConnected(ctx context.Context) error {
	if a.wsPrivate.Conn == nil {
		if err := a.wsPrivate.Connect(); err != nil {
			return err
		}
	}

	return nil
}

func (a *Adapter) WsMarketConnected(ctx context.Context) error {
	if a.wsPublic.Conn == nil {
		if err := a.wsPublic.Connect(); err != nil {
			return err
		}
	}
	return nil
}

func (a *Adapter) WsOrderConnected(ctx context.Context) error {
	if a.wsPrivate.Conn == nil {
		if err := a.wsPrivate.Connect(); err != nil {
			return err
		}
	}

	return nil
}

func (a *Adapter) fetchInstruments(ctx context.Context) error {
	insts, err := a.client.GetInstruments(ctx, "SWAP")
	if err != nil {
		return err
	}

	a.symbolMap = make(map[string]string)
	a.idMap = make(map[string]string)
	a.instruments = make(map[string]okx.Instrument)

	for _, inst := range insts {
		market := exchanges.ParseMarketRef(inst.InstId, exchanges.QuoteCurrency(a.quoteCurrency), exchanges.MarketTypePerp)
		if inst.State == "live" && market.Base != "" && isSupportedOKXQuote(market.Quote) {
			a.symbolMap[market.Symbol()] = inst.InstId
			if string(market.Quote) == a.quoteCurrency {
				a.symbolMap[market.Base] = inst.InstId
			}
			a.idMap[inst.InstId] = market.Symbol()
			a.instruments[inst.InstId] = inst
		}
	}

	// Populate BaseAdapter symbol details so ListSymbols() works
	details := make(map[string]*exchanges.SymbolDetails)
	for token, instId := range a.symbolMap {
		inst := a.instruments[instId]
		details[token] = &exchanges.SymbolDetails{
			Symbol:            token,
			MinQuantity:       parseString(inst.MinSz).Mul(parseString(inst.CtVal)),
			PricePrecision:    exchanges.CountDecimalPlaces(inst.TickSz),
			QuantityPrecision: exchanges.CountDecimalPlaces(inst.LotSz),
		}
	}
	a.SetSymbolDetails(details)

	return nil
}

func (a *Adapter) IsConnected(ctx context.Context) (bool, error) {
	return a.IsMarketConnected() && a.IsAccountConnected(), nil
}

func (a *Adapter) Close() error {
	if a.wsPublic.Conn != nil {
		a.wsPublic.Conn.Close()
	}
	if a.wsPrivate.Conn != nil {
		a.wsPrivate.Conn.Close()
	}
	return nil
}

func (a *Adapter) FormatSymbol(symbol string) string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if strings.Contains(symbol, "-") {
		return symbol
	}
	if instId, ok := a.symbolMap[symbol]; ok {
		return instId
	}
	return FormatSymbolWithQuote(symbol, a.quoteCurrency, "SWAP")
}

func (a *Adapter) ExtractSymbol(instId string) string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if sym, ok := a.idMap[instId]; ok {
		return sym
	}
	return instId
}

func (a *Adapter) RefreshSymbolDetails(ctx context.Context) error {
	return a.fetchInstruments(ctx)
}

func (a *Adapter) RefreshPositionMode(ctx context.Context) error {
	res, err := a.client.GetAccountConfig(ctx)
	if err != nil {
		return err
	}
	if len(res) > 0 {
		a.mu.Lock()
		a.accountLevel = res[0].AccountLevel()
		a.posMode = res[0].PosMode
		a.mu.Unlock()
	}
	return nil
}

func (a *Adapter) getCtVal(ctx context.Context, symbol string) decimal.Decimal {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if inst, ok := a.instruments[symbol]; ok {
		return parseString(inst.CtVal)
	}
	return decimal.NewFromFloat(0.01) // fallback
}

func (a *Adapter) formatPerpOrderSize(ctx context.Context, instId string, quantity decimal.Decimal) (string, error) {
	ctVal := a.getCtVal(ctx, instId)
	if ctVal.IsZero() {
		return "", fmt.Errorf("invalid contract value for %s", instId)
	}

	contracts := quantity.Div(ctVal)

	a.mu.RLock()
	inst, ok := a.instruments[instId]
	a.mu.RUnlock()
	if ok {
		prec := exchanges.CountDecimalPlaces(inst.LotSz)
		return contracts.StringFixed(prec), nil
	}

	return contracts.Floor().String(), nil
}

// ================= Account & Trading =================

func (a *Adapter) FetchAccount(ctx context.Context) (*exchanges.Account, error) {
	bals, err := a.client.GetAccountBalance(ctx, nil)
	if err != nil {
		return nil, err
	}

	totalEq := decimal.Zero
	availEq := decimal.Zero
	if len(bals) > 0 {
		totalEq = parseString(bals[0].TotalEq)
		if len(bals[0].Details) > 0 {
			availEq = parseString(bals[0].Details[0].AvailEq)
		}
	}

	positions, err := a.FetchPositions(ctx)
	if err != nil {
		return nil, err
	}

	// open orders
	orders, err := a.client.GetOrders(ctx, nil, nil)
	if err != nil {
		return nil, err
	}
	var orderList []exchanges.Order
	for _, o := range orders {
		orderList = append(orderList, *a.mapOrderRest(&o))
	}

	return &exchanges.Account{
		TotalBalance:     totalEq,
		AvailableBalance: availEq,
		Positions:        positions,
		Orders:           orderList,
	}, nil
}

func (a *Adapter) FetchBalance(ctx context.Context) (decimal.Decimal, error) {
	acc, err := a.FetchAccount(ctx)
	if err != nil {
		return decimal.Zero, err
	}
	return acc.TotalBalance, nil
}

func (a *Adapter) FetchPositions(ctx context.Context) ([]exchanges.Position, error) {
	instType := "SWAP"
	ps, err := a.client.GetPositions(ctx, &instType, nil)
	if err != nil {
		return nil, err
	}

	var positions []exchanges.Position
	for _, p := range ps {
		sz := parseString(p.Pos)
		side := exchanges.PositionSideLong
		if p.PosSide == okx.PosSideShort {
			side = exchanges.PositionSideShort
		} else if p.PosSide == okx.PosSideNet && sz.IsNegative() {
			side = exchanges.PositionSideShort
			sz = sz.Abs()
		}

		marginType := exchanges.MarginTypeCrossed
		if p.MgnMode == okx.MgnModeIsolated {
			marginType = exchanges.MarginTypeIsolated
		}

		ctVal := a.getCtVal(ctx, p.InstId)
		// OKX Pos is in contracts for swaps/futures
		// Quantity (Coins) = Contracts * CtVal
		qty := sz.Mul(ctVal)

		positions = append(positions, exchanges.Position{
			InstrumentType:   exchanges.InstrumentTypePerp,
			Symbol:           a.ExtractSymbol(p.InstId),
			Side:             side,
			Quantity:         qty,
			EntryPrice:       parseString(p.AvgPx),
			UnrealizedPnL:    parseString(p.Upl),
			LiquidationPrice: parseString(p.LiqPx),
			Leverage:         parseString(p.Lever),
			MarginType:       string(marginType),
		})
	}
	return positions, nil
}

func (a *Adapter) SetLeverage(ctx context.Context, symbol string, leverage int) error {
	instId := a.FormatSymbol(symbol)
	_, err := a.client.SetLeverage(ctx, okx.SetLeverage{
		InstId:  instId,
		Lever:   leverage,
		MgnMode: "isolated",
	})
	return err
}

func (a *Adapter) FetchFeeRate(ctx context.Context, symbol string) (*exchanges.FeeRate, error) {
	a.feeOnce.Do(func() {
		// OKX GetTradeFee with instType only (no instId) to avoid mismatch error
		res, err := a.client.GetTradeFee(ctx, "SWAP", nil)
		if err != nil {
			a.cachedFeeErr = err
			return
		}
		if len(res) == 0 {
			a.cachedFeeErr = fmt.Errorf("no fee data")
			return
		}

		fee := &exchanges.FeeRate{
			Maker: decimal.NewFromFloat(0.0002),
			Taker: decimal.NewFromFloat(0.0005),
		}
		if len(res[0].FeeGroups) > 0 {
			g := res[0].FeeGroups[0]
			if m, err := decimal.NewFromString(g.Maker); err == nil {
				fee.Maker = m.Abs()
			}
			if t, err := decimal.NewFromString(g.Taker); err == nil {
				fee.Taker = t.Abs()
			}
		}
		a.cachedFeeRate = fee
	})
	if a.cachedFeeErr != nil {
		return nil, a.cachedFeeErr
	}
	return a.cachedFeeRate, nil
}

// ================= WebSocket =================

func (a *Adapter) WatchOrders(ctx context.Context, callback exchanges.OrderUpdateCallback) error {
	return a.watchPrivateOrders(ctx, callback, nil)
}

func (a *Adapter) WatchPositions(ctx context.Context, callback exchanges.PositionUpdateCallback) error {
	if err := a.WsAccountConnected(ctx); err != nil {
		return err
	}
	return a.wsPrivate.SubscribePositions("SWAP", func(p *okx.Position) {
		side := exchanges.PositionSideLong
		if p.PosSide == "short" {
			side = exchanges.PositionSideShort
		}

		ctVal := a.getCtVal(ctx, p.InstId)
		qty := parseString(p.Pos).Mul(ctVal)

		callback(&exchanges.Position{
			InstrumentType: exchanges.InstrumentTypePerp,
			Symbol:         a.ExtractSymbol(p.InstId),
			Side:           side,
			Quantity:       qty,
			EntryPrice:     parseString(p.AvgPx),
		})
	})
}

func (a *Adapter) WatchTicker(ctx context.Context, symbol string, callback exchanges.TickerCallback) error {
	if err := a.WsMarketConnected(ctx); err != nil {
		return err
	}
	instId := a.FormatSymbol(symbol)

	return a.wsPublic.SubscribeTicker(instId, func(t *okx.Ticker) {
		callback(&exchanges.Ticker{
			Symbol:    symbol,
			LastPrice: parseString(t.Last),
			Timestamp: parseTime(t.Ts),
		})
	})
}

func (a *Adapter) WatchKlines(ctx context.Context, symbol string, interval exchanges.Interval, callback exchanges.KlineCallback) error {
	return exchanges.ErrNotSupported
}

func (a *Adapter) WatchTrades(ctx context.Context, symbol string, callback exchanges.TradeCallback) error {
	return exchanges.ErrNotSupported
}

func (a *Adapter) StopWatchOrders(ctx context.Context) error {
	return a.stopPrivateOrders(ctx, true, false)
}
func (a *Adapter) WatchFills(ctx context.Context, callback exchanges.FillCallback) error {
	return a.watchPrivateOrders(ctx, nil, callback)
}
func (a *Adapter) StopWatchFills(ctx context.Context) error {
	return a.stopPrivateOrders(ctx, false, true)
}
func (a *Adapter) StopWatchPositions(ctx context.Context) error { return nil }

func (a *Adapter) StopWatchTicker(ctx context.Context, symbol string) error {
	instId := a.FormatSymbol(symbol)
	channel := okx.WsSubscribeArgs{
		Channel: "tickers",
		InstId:  instId,
	}
	return a.wsPublic.Unsubscribe(channel)
}

func (a *Adapter) WatchOrderBook(ctx context.Context, symbol string, depth int, cb exchanges.OrderBookCallback) error {
	if err := a.WsMarketConnected(ctx); err != nil {
		return err
	}
	instId := a.FormatSymbol(symbol)
	ctVal := a.getCtVal(ctx, instId)
	if ctVal.IsZero() {
		ctVal = decimal.NewFromInt(1)
	}

	ob := NewOrderBook(symbol, ctVal)
	a.SetLocalOrderBook(instId, ob) // Store under instId to match GetLocalOrderBook override

	if err := a.wsPublic.SubscribeOrderBook(instId, func(data *okx.OrderBook, action string) {
		ob.ProcessUpdate(data, action)
		if cb != nil {
			cb(ob.ToAdapterOrderBook(depth))
		}
	}); err != nil {
		return err
	}
	return a.BaseAdapter.WaitOrderBookReady(ctx, instId)
}

func (a *Adapter) StopWatchOrderBook(ctx context.Context, symbol string) error {
	instId := a.FormatSymbol(symbol)
	channel := okx.WsSubscribeArgs{
		Channel: "books",
		InstId:  instId,
	}
	return a.wsPublic.Unsubscribe(channel)
}

func (a *Adapter) StopWatchKlines(ctx context.Context, symbol string, interval exchanges.Interval) error {
	return exchanges.ErrNotSupported
}

func (a *Adapter) StopWatchTrades(ctx context.Context, symbol string) error {
	return exchanges.ErrNotSupported
}

func (a *Adapter) FetchSymbolDetails(ctx context.Context, symbol string) (*exchanges.SymbolDetails, error) {
	instId := a.FormatSymbol(symbol)

	a.mu.RLock()
	inst, ok := a.instruments[instId]
	a.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("not found")
	}

	return &exchanges.SymbolDetails{
		Symbol:            symbol,
		MinQuantity:       parseString(inst.MinSz).Mul(parseString(inst.CtVal)), // Convert contracts to coins
		PricePrecision:    exchanges.CountDecimalPlaces(inst.TickSz),
		QuantityPrecision: exchanges.CountDecimalPlaces(inst.LotSz), // LotSz is usually 1 for contracts, but just in case
	}, nil
}

// Helpers

func (a *Adapter) mapOrderRest(o *okx.Order) *exchanges.Order {
	status := exchanges.OrderStatusUnknown
	switch o.State {
	case okx.OrderStatusLive:
		status = exchanges.OrderStatusNew
	case okx.OrderStatusPartiallyFilled:
		status = exchanges.OrderStatusPartiallyFilled
	case okx.OrderStatusFilled:
		status = exchanges.OrderStatusFilled
	case okx.OrderStatusCanceled:
		status = exchanges.OrderStatusCancelled
	case okx.OrderStatusMmpCanceled:
		status = exchanges.OrderStatusCancelled
	}

	side := exchanges.OrderSideBuy
	if o.Side == okx.SideSell {
		side = exchanges.OrderSideSell
	}

	ts, _ := strconv.ParseInt(o.UTime, 10, 64)

	return &exchanges.Order{
		OrderID:          o.OrdId,
		Symbol:           a.ExtractSymbol(o.InstId),
		Side:             side,
		Status:           status,
		Quantity:         parseString(o.Sz).Mul(a.getCtVal(context.Background(), o.InstId)), // 张数 * 每张代表的数量
		FilledQuantity:   parseString(o.AccFillSz).Mul(a.getCtVal(context.Background(), o.InstId)),
		Price:            parseString(o.Px),
		OrderPrice:       parseString(o.Px),
		AverageFillPrice: parseString(o.AvgPx),
		LastFillPrice:    parseString(o.FillPx),
		LastFillQuantity: parseString(o.FillSz).Mul(a.getCtVal(context.Background(), o.InstId)),
		Fee:              parseString(o.Fee),
		Timestamp:        ts,
	}
}

func (a *Adapter) mapOrderStream(o *okx.Order) *exchanges.Order {
	order := a.mapOrderRest(o)
	order.ClientOrderID = o.ClOrdId
	order.Type = mapOKXOrderType(o.OrdType)
	order.Price = order.OrderPrice
	order.AverageFillPrice = decimal.Zero
	order.LastFillPrice = decimal.Zero
	order.LastFillQuantity = decimal.Zero
	order.Fee = decimal.Zero
	return order
}

func (a *Adapter) mapOrderFill(o *okx.Order) *exchanges.Fill {
	qty := parseString(o.FillSz).Mul(a.getCtVal(context.Background(), o.InstId))
	if qty.IsZero() {
		return nil
	}

	side := exchanges.OrderSideBuy
	if o.Side == okx.SideSell {
		side = exchanges.OrderSideSell
	}

	ts := parseTime(o.FillTime)
	if ts == 0 {
		ts = parseTime(o.UTime)
	}
	if ts == 0 {
		ts = parseTime(o.CTime)
	}

	return &exchanges.Fill{
		TradeID:       o.TradeId,
		OrderID:       o.OrdId,
		ClientOrderID: o.ClOrdId,
		Symbol:        a.ExtractSymbol(o.InstId),
		Side:          side,
		Price:         parseString(o.FillPx),
		Quantity:      qty,
		Fee:           parseString(o.Fee),
		FeeAsset:      o.FeeCcy,
		IsMaker:       strings.EqualFold(o.ExecType, "M"),
		Timestamp:     ts,
	}
}

func parseTime(s string) int64 {
	val, _ := strconv.ParseInt(s, 10, 64)
	return val
}

// WaitOrderBookReady waits for orderbook to be ready
func (a *Adapter) WaitOrderBookReady(ctx context.Context, symbol string) error {
	instId := a.FormatSymbol(symbol)
	return a.BaseAdapter.WaitOrderBookReady(ctx, instId)
}

// GetLocalOrderBook get local orderbook
func (a *Adapter) GetLocalOrderBook(symbol string, depth int) *exchanges.OrderBook {
	instId := a.FormatSymbol(symbol)

	ob, ok := a.GetLocalOrderBookImplementation(instId)
	if !ok {
		return nil
	}

	okxOb := ob.(*OrderBook)
	if !okxOb.IsInitialized() {
		return nil
	}

	bids, asks := ob.GetDepth(depth)
	return &exchanges.OrderBook{
		// To map perfectly, okx perp adds coins multiplier
		// In OKX, bids and asks in ToAdapterOrderBook are ALREADY converted by ctVal. So just returning GetDepth is correct.
		Symbol:    symbol,
		Timestamp: ob.Timestamp(),
		Bids:      bids,
		Asks:      asks,
	}
}
