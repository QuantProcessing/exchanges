package okx

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

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

	posMode string // long_short_mode or net_mode

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
		if strings.Contains(inst.InstId, a.quoteCurrency) && inst.State == "live" {
			a.symbolMap[inst.CtValCcy] = inst.InstId
			a.idMap[inst.InstId] = inst.CtValCcy
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

func (a *Adapter) PlaceOrder(ctx context.Context, params *exchanges.OrderParams) (*exchanges.Order, error) {
	// OKX supports native market orders — skip slippage conversion entirely.
	// OKX's ~1% price band limit rejects LIMIT+IOC with large slippage.
	// 1. Validation & Formatting
	details, err := a.FetchSymbolDetails(ctx, params.Symbol)
	if err == nil {
		if err := exchanges.ValidateAndFormatParams(params, details); err != nil {
			return nil, err
		}
	}

	instId := a.FormatSymbol(params.Symbol)

	side := "buy"
	if params.Side == exchanges.OrderSideSell {
		side = "sell"
	}

	// Map Order Type & TIF
	ordType := a.mapOrderType(params)

	ctVal := a.getCtVal(ctx, instId)
	if ctVal.IsZero() {
		return nil, fmt.Errorf("invalid contract value for %s", instId)
	}

	// Calculate sz (Contracts)
	// params.Quantity (Coins) / CtVal (Coins/Contract) = Contracts
	szVal := params.Quantity.Div(ctVal)

	// Format to LotSz precision
	// We need LotSz.
	a.mu.RLock()
	inst, ok := a.instruments[instId]
	a.mu.RUnlock()

	sz := ""
	if ok {
		prec := exchanges.CountDecimalPlaces(inst.LotSz)
		sz = szVal.StringFixed(prec)
	} else {
		// Fallback to integer
		szVal = szVal.Floor()
		sz = szVal.String()
	}

	var clOrdId *string
	if params.ClientID != "" {
		clOrdId = &params.ClientID
	}

	// Determine PosSide
	a.mu.RLock()
	pm := a.posMode
	a.mu.RUnlock()

	var posSide *string
	if pm == "long_short_mode" {
		val := "long"
		if params.Side == exchanges.OrderSideBuy {
			if params.ReduceOnly {
				val = "short"
			} else {
				val = "long"
			}
		} else { // Sell
			if params.ReduceOnly {
				val = "long"
			} else {
				val = "short"
			}
		}
		posSide = &val
	} else {
		// net_mode: posSide optional, defaults to net
		// If we set it, use "net"? Or dont set it.
		// API says "In net mode... default net".
		// Leave nil.
	}

	// Format Price
	var px *string
	if params.Price.IsPositive() {
		if ok {
			prec := exchanges.CountDecimalPlaces(inst.TickSz)
			s := params.Price.StringFixed(prec)
			px = &s
		} else {
			s := fmt.Sprintf("%v", params.Price)
			px = &s
		}
	}

	req := &okx.OrderRequest{
		InstId:  instId,
		TdMode:  "isolated",
		Side:    side,
		PosSide: posSide,
		OrdType: ordType,
		Sz:      sz,
		Px:      px,
		ClOrdId: clOrdId,
	}

	if params.ReduceOnly {
		ro := true
		req.ReduceOnly = &ro
	}

	ids, err := a.client.PlaceOrder(ctx, req)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("no response")
	}
	return &exchanges.Order{
		OrderID:       ids[0].OrdId,
		ClientOrderID: ids[0].ClOrdId,
		Symbol:        params.Symbol,
		Side:          params.Side,
		Type:          params.Type,
		Quantity:      params.Quantity,
		Price:         params.Price,
		Status:        exchanges.OrderStatusPending,
		Timestamp:     time.Now().UnixMilli(),
	}, nil
}

func (a *Adapter) PlaceOrderWS(ctx context.Context, params *exchanges.OrderParams) error {
	if strings.TrimSpace(params.ClientID) == "" {
		return fmt.Errorf("client id required for PlaceOrderWS")
	}
	instId := a.FormatSymbol(params.Symbol)
	side := "buy"
	if params.Side == exchanges.OrderSideSell {
		side = "sell"
	}
	ordType := a.mapOrderType(params)
	sz := fmt.Sprintf("%v", params.Quantity)

	a.mu.RLock()
	pm := a.posMode
	a.mu.RUnlock()

	var posSide *string
	if pm == "long_short_mode" {
		val := "long"
		if params.Side == exchanges.OrderSideBuy {
			if params.ReduceOnly {
				val = "short"
			} else {
				val = "long"
			}
		} else if params.ReduceOnly {
			val = "long"
		} else {
			val = "short"
		}
		posSide = &val
	}

	var clOrdId *string
	if params.ClientID != "" {
		clOrdId = &params.ClientID
	}

	var px *string
	if params.Type != exchanges.OrderTypeMarket && params.Price.IsPositive() {
		if ctVal := a.getCtVal(ctx, instId); !ctVal.IsZero() {
			contracts := params.Quantity.Div(ctVal)
			sz = fmt.Sprintf("%v", contracts)
		}
		if inst, ok := a.instruments[instId]; ok {
			prec := exchanges.CountDecimalPlaces(inst.TickSz)
			s := params.Price.StringFixed(prec)
			px = &s
		} else {
			s := fmt.Sprintf("%v", params.Price)
			px = &s
		}
	}

	req := &okx.OrderRequest{
		InstId:  instId,
		TdMode:  "isolated",
		Side:    side,
		PosSide: posSide,
		OrdType: ordType,
		Sz:      sz,
		Px:      px,
		ClOrdId: clOrdId,
	}

	if params.ReduceOnly {
		ro := true
		req.ReduceOnly = &ro
	}

	if err := a.WsOrderConnected(ctx); err != nil {
		return err
	}
	_, err := a.wsPrivate.PlaceOrderWS(req)
	return err
}

func (a *Adapter) mapOrderType(params *exchanges.OrderParams) string {
	switch params.Type {
	case exchanges.OrderTypeMarket:
		return "market"
	case exchanges.OrderTypePostOnly:
		return "post_only"
	case exchanges.OrderTypeLimit:
		if params.TimeInForce == exchanges.TimeInForceIOC {
			return "ioc"
		} else if params.TimeInForce == exchanges.TimeInForceFOK {
			return "fok"
		}
		return "limit"
	default:
		return "limit"
	}
}

func (a *Adapter) CancelOrder(ctx context.Context, orderID, symbol string) error {
	instId := a.FormatSymbol(symbol)
	_, err := a.client.CancelOrder(ctx, instId, orderID, "")
	return err
}

func (a *Adapter) CancelOrderWS(ctx context.Context, orderID, symbol string) error {
	instId := a.FormatSymbol(symbol)
	if err := a.WsOrderConnected(ctx); err != nil {
		return err
	}
	_, err := a.wsPrivate.CancelOrderWS(instId, &orderID, nil)
	return err
}

func (a *Adapter) ModifyOrder(ctx context.Context, orderID, symbol string, params *exchanges.ModifyOrderParams) (*exchanges.Order, error) {
	instId := a.FormatSymbol(symbol)

	req := &okx.ModifyOrderRequest{
		InstId: instId,
		OrdId:  &orderID,
	}
	if params.Quantity.IsPositive() {
		sz := fmt.Sprintf("%v", params.Quantity)
		req.NewSz = &sz
	}
	if params.Price.IsPositive() {
		px := fmt.Sprintf("%v", params.Price)
		req.NewPx = &px
	}

	resp, err := a.client.ModifyOrder(ctx, req)
	if err != nil {
		return nil, err
	}
	if len(resp) == 0 {
		return nil, fmt.Errorf("no response")
	}

	return &exchanges.Order{
		OrderID:       resp[0].OrdId,
		ClientOrderID: resp[0].ClOrdId,
		Symbol:        symbol,
		Status:        exchanges.OrderStatusPending,
	}, nil
}

func (a *Adapter) ModifyOrderWS(ctx context.Context, orderID, symbol string, params *exchanges.ModifyOrderParams) error {
	instId := a.FormatSymbol(symbol)

	req := &okx.ModifyOrderRequest{
		InstId: instId,
		OrdId:  &orderID,
	}
	if params.Quantity.IsPositive() {
		sz := fmt.Sprintf("%v", params.Quantity)
		req.NewSz = &sz
	}
	if params.Price.IsPositive() {
		px := fmt.Sprintf("%v", params.Price)
		req.NewPx = &px
	}

	if err := a.WsOrderConnected(ctx); err != nil {
		return err
	}
	resp, err := a.wsPrivate.ModifyOrderWS(req)
	if err != nil {
		return err
	}
	if resp.SCode != "0" {
		return fmt.Errorf("modify error: %s", resp.SMsg)
	}
	return nil
}

func (a *Adapter) FetchOrderByID(ctx context.Context, orderID, symbol string) (*exchanges.Order, error) {
	instId := a.FormatSymbol(symbol)

	res, err := a.client.GetOrder(ctx, instId, orderID, "")
	if err != nil {
		if isOKXOrderLookupMiss(err) {
			return nil, exchanges.ErrOrderNotFound
		}
		return nil, err
	}
	if len(res) == 0 {
		return nil, exchanges.ErrOrderNotFound
	}

	return a.mapOrderRest(&res[0]), nil
}

func (a *Adapter) FetchOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	return nil, exchanges.ErrNotSupported
}

func (a *Adapter) FetchOpenOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	instId := a.FormatSymbol(symbol)

	var ids *string
	if instId != "" {
		ids = &instId
	}
	res, err := a.client.GetOrders(ctx, nil, ids)
	if err != nil {
		return nil, err
	}

	var orders []exchanges.Order
	for _, o := range res {
		orders = append(orders, *a.mapOrderRest(&o))
	}
	return orders, nil
}

func isOKXOrderLookupMiss(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "order") &&
		(strings.Contains(msg, "not exist") || strings.Contains(msg, "not found"))
}

func (a *Adapter) CancelAllOrders(ctx context.Context, symbol string) error {
	orders, err := a.FetchOpenOrders(ctx, symbol)
	if err != nil {
		return err
	}
	if len(orders) == 0 {
		return nil
	}

	instId := a.FormatSymbol(symbol)
	var reqs []okx.CancelOrderRequest
	for _, o := range orders {
		oid := o.OrderID
		reqs = append(reqs, okx.CancelOrderRequest{InstId: instId, OrdId: &oid})
	}

	chunkSize := 20
	for i := 0; i < len(reqs); i += chunkSize {
		end := i + chunkSize
		if end > len(reqs) {
			end = len(reqs)
		}
		_, err := a.client.CancelOrders(ctx, reqs[i:end])
		if err != nil {
			return err
		}
	}
	return nil
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

// ================= Market Data =================

func (a *Adapter) FetchTicker(ctx context.Context, symbol string) (*exchanges.Ticker, error) {
	instId := a.FormatSymbol(symbol)

	res, err := a.client.GetTicker(ctx, instId)
	if err != nil {
		return nil, err
	}
	if len(res) == 0 {
		return nil, fmt.Errorf("no ticker")
	}

	t := res[0]
	return &exchanges.Ticker{
		Symbol:    symbol,
		LastPrice: parseString(t.Last),
		Bid:       parseString(t.BidPx),
		Ask:       parseString(t.AskPx),
		High24h:   parseString(t.High24h),
		Low24h:    parseString(t.Low24h),
		Volume24h: parseString(t.Vol24h),
		Timestamp: parseTime(t.Ts),
	}, nil
}

func (a *Adapter) FetchOrderBook(ctx context.Context, symbol string, limit int) (*exchanges.OrderBook, error) {
	instId := a.FormatSymbol(symbol)

	sz := 400
	if limit > 0 && limit < 400 {
		sz = limit
	}

	res, err := a.client.GetOrderBook(ctx, instId, &sz)
	if err != nil {
		return nil, err
	}
	if len(res) == 0 {
		return nil, fmt.Errorf("no orderbook")
	}

	book := res[0]
	ob := &exchanges.OrderBook{
		Symbol:    symbol,
		Timestamp: parseTime(book.Ts),
		Bids:      make([]exchanges.Level, 0, len(book.Bids)),
		Asks:      make([]exchanges.Level, 0, len(book.Asks)),
	}
	for _, b := range book.Bids {
		if len(b) >= 2 {
			ob.Bids = append(ob.Bids, exchanges.Level{Price: parseString(b[0]), Quantity: parseString(b[1])})
		}
	}
	for _, as := range book.Asks {
		if len(as) >= 2 {
			ob.Asks = append(ob.Asks, exchanges.Level{Price: parseString(as[0]), Quantity: parseString(as[1])})
		}
	}
	return ob, nil
}

func (a *Adapter) FetchKlines(ctx context.Context, symbol string, interval exchanges.Interval, opts *exchanges.KlineOpts) ([]exchanges.Kline, error) {
	var start, end *time.Time
	var limit int
	if opts != nil {
		start = opts.Start
		end = opts.End
		limit = opts.Limit
	}
	_ = start
	_ = end
	_ = limit
	instId := a.FormatSymbol(symbol)

	bar := "1m"
	switch interval {
	case exchanges.Interval1m:
		bar = "1m"
	case exchanges.Interval5m:
		bar = "5m"
	case exchanges.Interval15m:
		bar = "15m"
	case exchanges.Interval1h:
		bar = "1H"
	case exchanges.Interval4h:
		bar = "4H"
	case exchanges.Interval1d:
		bar = "1D"
	}

	var after *string
	if end != nil {
		s := fmt.Sprintf("%d", end.UnixMilli())
		after = &s
	}

	res, err := a.client.GetCandles(ctx, instId, &bar, after, nil, &limit)
	if err != nil {
		return nil, err
	}

	klines := make([]exchanges.Kline, len(res))
	for i, k := range res {
		idx := len(res) - 1 - i
		klines[idx] = exchanges.Kline{
			Symbol:    symbol,
			Interval:  interval,
			Timestamp: parseTime(k[0]),
			Open:      parseString(k[1]),
			High:      parseString(k[2]),
			Low:       parseString(k[3]),
			Close:     parseString(k[4]),
			Volume:    parseString(k[5]),
		}
	}
	return klines, nil
}

func (a *Adapter) FetchTrades(ctx context.Context, symbol string, limit int) ([]exchanges.Trade, error) {
	instId := a.FormatSymbol(symbol)

	params := url.Values{}
	params.Add("instId", instId)
	if limit > 0 {
		params.Add("limit", fmt.Sprintf("%d", limit))
	}
	path := "/api/v5/market/trades?" + params.Encode()

	type OkxTrade struct {
		TradeId string `json:"tradeId"`
		Px      string `json:"px"`
		Sz      string `json:"sz"`
		Side    string `json:"side"`
		Ts      string `json:"ts"`
	}
	trades, err := okx.Request[OkxTrade](a.client, ctx, okx.MethodGet, path, nil, false)
	if err != nil {
		return nil, err
	}

	res := make([]exchanges.Trade, len(trades))
	for i, t := range trades {
		side := exchanges.TradeSideBuy
		if t.Side == "sell" {
			side = exchanges.TradeSideSell
		}
		res[i] = exchanges.Trade{
			ID:        t.TradeId,
			Symbol:    symbol,
			Price:     parseString(t.Px),
			Quantity:  parseString(t.Sz),
			Side:      side,
			Timestamp: parseTime(t.Ts),
		}
	}
	return res, nil
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
			Symbol:     a.ExtractSymbol(p.InstId),
			Side:       side,
			Quantity:   qty,
			EntryPrice: parseString(p.AvgPx),
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
