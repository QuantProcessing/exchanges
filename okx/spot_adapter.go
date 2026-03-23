package okx

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/okx/sdk"

	"github.com/shopspring/decimal"
)

// SpotAdapter OKX 现货适配器
type SpotAdapter struct {
	*exchanges.BaseAdapter

	client    *okx.Client
	wsPublic  *okx.WsClient
	wsPrivate *okx.WsClient

	// Symbol mapping: Token -> InstId (e.g. BTC -> BTC-USDT)
	symbolMap     map[string]string         // BTC -> BTC-USDT
	idMap         map[string]string         // BTC-USDT -> BTC
	instruments   map[string]okx.Instrument // InstId -> Instrument
	quoteCurrency string                    // "USDT" or "USDC"

	mu sync.RWMutex
}

// NewSpotAdapter creates a new OKX spot adapter
func NewSpotAdapter(ctx context.Context, opts Options) (*SpotAdapter, error) {
	quote, err := opts.quoteCurrency()
	if err != nil {
		return nil, err
	}

	client := okx.NewClient()
	wsPublic := okx.NewWsClient(ctx)
	wsPrivate := okx.NewWsClient(ctx)

	if opts.APIKey != "" {
		client.WithCredentials(opts.APIKey, opts.SecretKey, opts.Passphrase)
		wsPrivate.WithCredentials(opts.APIKey, opts.SecretKey, opts.Passphrase)
	}

	a := &SpotAdapter{
		BaseAdapter:   exchanges.NewBaseAdapter("OKX", exchanges.MarketTypeSpot, opts.logger()),
		client:        client,
		wsPublic:      wsPublic,
		wsPrivate:     wsPrivate,
		symbolMap:     make(map[string]string),
		idMap:         make(map[string]string),
		instruments:   make(map[string]okx.Instrument),
		quoteCurrency: string(quote),
	}

	// Load Instruments
	if err := a.RefreshSymbolDetails(context.Background()); err != nil {
		// TODO: logger.Error("Failed to load instruments", zap.Error(err))
	}

	// TODO: logger.Info("Initialized OKX Spot Adapter")
	return a, nil
}

func (a *SpotAdapter) WithCredentials(apiKey, secretKey string) exchanges.Exchange {
	// OKX requires 3 credentials, this method is not ideal
	// Use NewSpotAdapter with config instead
	// TODO: logger.Warn("WithCredentials not fully supported for OKX, use NewSpotAdapter with config")
	return a
}

func (a *SpotAdapter) Close() error {
	if a.wsPublic.Conn != nil {
		a.wsPublic.Conn.Close()
	}
	if a.wsPrivate.Conn != nil {
		a.wsPrivate.Conn.Close()
	}
	return nil
}

func (a *SpotAdapter) WsAccountConnected(ctx context.Context) error {
	if a.wsPrivate.Conn == nil {
		if err := a.wsPrivate.Connect(); err != nil {
			return err
		}
	}
	return nil
}

func (a *SpotAdapter) WsMarketConnected(ctx context.Context) error {
	if a.wsPublic.Conn == nil {
		if err := a.wsPublic.Connect(); err != nil {
			return err
		}
	}
	return nil
}

func (a *SpotAdapter) WsOrderConnected(ctx context.Context) error {
	if a.wsPrivate.Conn == nil {
		if err := a.wsPrivate.Connect(); err != nil {
			return err
		}
	}
	return nil
}

func (a *SpotAdapter) fetchInstruments(ctx context.Context) error {
	insts, err := a.client.GetInstruments(ctx, "SPOT")
	if err != nil {
		return err
	}

	a.symbolMap = make(map[string]string)
	a.idMap = make(map[string]string)
	a.instruments = make(map[string]okx.Instrument)

	quoteSuffix := "-" + a.quoteCurrency
	for _, inst := range insts {
		// For spot: BTC-USDT, ETH-USDT, etc.
		if strings.Contains(inst.InstId, quoteSuffix) && inst.State == "live" {
			// BaseCcy is the base currency (e.g., BTC in BTC-USDT)
			a.symbolMap[inst.BaseCcy] = inst.InstId
			a.idMap[inst.InstId] = inst.BaseCcy
			a.instruments[inst.InstId] = inst
		}
	}
	return nil
}

func (a *SpotAdapter) IsConnected(ctx context.Context) (bool, error) {
	return a.IsMarketConnected() && a.IsAccountConnected(), nil
}

// FormatSymbol converts internal symbol (e.g., BTC) to OKX spot InstId (e.g., BTC-USDT)
func (a *SpotAdapter) FormatSymbol(symbol string) string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if strings.Contains(symbol, "-") {
		return symbol
	}
	if instId, ok := a.symbolMap[symbol]; ok {
		return instId
	}
	return FormatSpotSymbolWithQuote(symbol, a.quoteCurrency)
}

// ExtractSymbol converts OKX InstId (e.g., BTC-USDT) to internal symbol (e.g., BTC)
func (a *SpotAdapter) ExtractSymbol(instId string) string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if sym, ok := a.idMap[instId]; ok {
		return sym
	}
	return instId
}

func (a *SpotAdapter) RefreshSymbolDetails(ctx context.Context) error {
	return a.fetchInstruments(ctx)
}

// ================= Account & Trading =================

func (a *SpotAdapter) FetchAccount(ctx context.Context) (*exchanges.Account, error) {
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

	// Get open orders
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
		Positions:        nil, // Spot has no positions
		Orders:           orderList,
	}, nil
}

func (a *SpotAdapter) FetchBalance(ctx context.Context) (decimal.Decimal, error) {
	acc, err := a.FetchAccount(ctx)
	if err != nil {
		return decimal.Zero, err
	}
	return acc.TotalBalance, nil
}

func (a *SpotAdapter) FetchSpotBalances(ctx context.Context) ([]exchanges.SpotBalance, error) {
	bals, err := a.client.GetAccountBalance(ctx, nil)
	if err != nil {
		return nil, err
	}

	var spotBalances []exchanges.SpotBalance
	if len(bals) > 0 {
		for _, detail := range bals[0].Details {
			free := parseString(detail.AvailBal)
			locked := parseString(detail.FrozenBal)
			// CashBal is usually the total for spot/cash accounts
			total := parseString(detail.CashBal)
			if total.IsZero() {
				total = free.Add(locked)
			}

			spotBalances = append(spotBalances, exchanges.SpotBalance{
				Asset:  detail.Ccy,
				Free:   free,
				Locked: locked,
				Total:  total,
			})
		}
	}

	return spotBalances, nil
}

func (a *SpotAdapter) TransferAsset(ctx context.Context, params *exchanges.TransferParams) error {
	return exchanges.ErrNotSupported
}

func (a *SpotAdapter) FetchFeeRate(ctx context.Context, symbol string) (*exchanges.FeeRate, error) {
	instId := a.FormatSymbol(symbol)

	res, err := a.client.GetTradeFee(ctx, "SPOT", &instId)
	if err != nil {
		return nil, err
	}
	if len(res) == 0 || len(res[0].FeeGroups) == 0 {
		return nil, fmt.Errorf("no fee data")
	}

	// Default spot fees
	return &exchanges.FeeRate{Maker: decimal.NewFromFloat(0.0008), Taker: decimal.NewFromFloat(0.001)}, nil
}

// ================= Order Management =================

func (a *SpotAdapter) PlaceOrder(ctx context.Context, params *exchanges.OrderParams) (*exchanges.Order, error) {
	// OKX supports native market orders — skip slippage conversion entirely.
	// OKX's ~1% price band limit rejects LIMIT+IOC with large slippage.
	// Validation & Formatting
	details, err := a.FetchSymbolDetails(ctx, params.Symbol)
	if err == nil {
		if err := exchanges.ValidateAndFormatParams(params, details); err != nil {
			return nil, err
		}
	}

	if err := a.WsOrderConnected(ctx); err != nil {
		return nil, err
	}
	instId := a.FormatSymbol(params.Symbol)

	side := "buy"
	if params.Side == exchanges.OrderSideSell {
		side = "sell"
	}

	// Map Order Type
	ordType := a.mapOrderType(params)

	// For spot: sz is in base currency (e.g., BTC amount)
	// No need to convert like perp
	sz := fmt.Sprintf("%v", params.Quantity)

	var clOrdId *string
	if params.ClientID != "" {
		clOrdId = &params.ClientID
	}

	// Format Price
	var px *string
	if params.Price.IsPositive() {
		a.mu.RLock()
		inst, ok := a.instruments[instId]
		a.mu.RUnlock()

		if ok {
			prec := exchanges.CountDecimalPlaces(inst.TickSz)
			s := params.Price.StringFixed(prec)
			px = &s
		} else {
			s := fmt.Sprintf("%v", params.Price)
			px = &s
		}
	}

	ccy := a.quoteCurrency
	var tgtCcy *string
	if ordType == "market" {
		t := "base_ccy"
		tgtCcy = &t
	}

	req := &okx.OrderRequest{
		InstId:  instId,
		TdMode:  "cash", // relation about account mode, default cross
		Side:    side,
		PosSide: nil, // Spot doesn't use PosSide
		OrdType: ordType,
		Sz:      sz,
		Px:      px,
		ClOrdId: clOrdId,
		Ccy:     &ccy,   // 保证金币种
		TgtCcy:  tgtCcy, // 计价货币
	}

	resp, err := a.wsPrivate.PlaceOrderWS(req)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, fmt.Errorf("no response")
	}

	return &exchanges.Order{
		OrderID:       resp.OrdId,
		ClientOrderID: resp.ClOrdId,
		Symbol:        params.Symbol,
		Side:          params.Side,
		Type:          params.Type,
		Quantity:      params.Quantity,
		Price:         params.Price,
		Status:        exchanges.OrderStatusPending,
		Timestamp:     time.Now().UnixMilli(),
	}, nil
}

func (a *SpotAdapter) mapOrderType(params *exchanges.OrderParams) string {
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

func (a *SpotAdapter) CancelOrder(ctx context.Context, orderID, symbol string) error {
	if err := a.WsOrderConnected(ctx); err != nil {
		return err
	}
	instId := a.FormatSymbol(symbol)
	_, err := a.wsPrivate.CancelOrderWS(instId, &orderID, nil)
	return err
}

func (a *SpotAdapter) ModifyOrder(ctx context.Context, orderID, symbol string, params *exchanges.ModifyOrderParams) (*exchanges.Order, error) {
	if err := a.WsOrderConnected(ctx); err != nil {
		return nil, err
	}
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

	resp, err := a.wsPrivate.ModifyOrderWS(req)
	if err != nil {
		return nil, err
	}
	if resp.SCode != "0" {
		return nil, fmt.Errorf("modify error: %s", resp.SMsg)
	}

	return &exchanges.Order{
		OrderID: resp.OrdId,
		Symbol:  symbol,
		Status:  exchanges.OrderStatusPending,
	}, nil
}

func (a *SpotAdapter) FetchOrderByID(ctx context.Context, orderID, symbol string) (*exchanges.Order, error) {
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

func (a *SpotAdapter) FetchOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	return nil, exchanges.ErrNotSupported
}

func (a *SpotAdapter) FetchOpenOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
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

func (a *SpotAdapter) CancelAllOrders(ctx context.Context, symbol string) error {
	if err := a.WsOrderConnected(ctx); err != nil {
		return err
	}
	orders, err := a.FetchOpenOrders(ctx, symbol)
	if err != nil {
		return err
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
		_, err := a.wsPrivate.CancelOrdersWS(reqs[i:end])
		if err != nil {
			return err
		}
	}
	return nil
}

// ================= Market Data =================

func (a *SpotAdapter) FetchTicker(ctx context.Context, symbol string) (*exchanges.Ticker, error) {
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

func (a *SpotAdapter) FetchOrderBook(ctx context.Context, symbol string, limit int) (*exchanges.OrderBook, error) {
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

func (a *SpotAdapter) FetchKlines(ctx context.Context, symbol string, interval exchanges.Interval, opts *exchanges.KlineOpts) ([]exchanges.Kline, error) {
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

func (a *SpotAdapter) FetchTrades(ctx context.Context, symbol string, limit int) ([]exchanges.Trade, error) {
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

func (a *SpotAdapter) FetchSymbolDetails(ctx context.Context, symbol string) (*exchanges.SymbolDetails, error) {
	instId := a.FormatSymbol(symbol)

	a.mu.RLock()
	inst, ok := a.instruments[instId]
	a.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("symbol not found: %s", symbol)
	}

	return &exchanges.SymbolDetails{
		Symbol:            symbol,
		PricePrecision:    exchanges.CountDecimalPlaces(inst.TickSz),
		QuantityPrecision: exchanges.CountDecimalPlaces(inst.LotSz),
		MinQuantity:       parseString(inst.MinSz),
		MinNotional:       decimal.Zero, // OKX doesn't provide this directly
	}, nil
}

// ================= WebSocket =================

func (a *SpotAdapter) WatchOrders(ctx context.Context, callback exchanges.OrderUpdateCallback) error {
	if err := a.WsAccountConnected(ctx); err != nil {
		return err
	}
	return a.wsPrivate.SubscribeOrders("SPOT", nil, func(o *okx.Order) {
		callback(a.mapOrderRest(o))
	})
}

func (a *SpotAdapter) WatchTicker(ctx context.Context, symbol string, callback exchanges.TickerCallback) error {
	if err := a.WsMarketConnected(ctx); err != nil {
		return err
	}

	instId := a.FormatSymbol(symbol)
	return a.wsPublic.SubscribeTicker(instId, func(t *okx.Ticker) {
		callback(&exchanges.Ticker{
			Symbol:    symbol,
			LastPrice: parseString(t.Last),
			Bid:       parseString(t.BidPx),
			Ask:       parseString(t.AskPx),
			High24h:   parseString(t.High24h),
			Low24h:    parseString(t.Low24h),
			Volume24h: parseString(t.Vol24h),
			Timestamp: parseTime(t.Ts),
		})
	})
}

func (a *SpotAdapter) WatchKlines(ctx context.Context, symbol string, interval exchanges.Interval, callback exchanges.KlineCallback) error {
	return exchanges.ErrNotSupported
}

func (a *SpotAdapter) WatchTrades(ctx context.Context, symbol string, callback exchanges.TradeCallback) error {
	return exchanges.ErrNotSupported
}

// Unsubscribe methods
func (a *SpotAdapter) StopWatchOrders(ctx context.Context) error {
	return nil
}

func (a *SpotAdapter) StopWatchTicker(ctx context.Context, symbol string) error {
	instId := a.FormatSymbol(symbol)
	channel := okx.WsSubscribeArgs{
		Channel: "tickers",
		InstId:  instId,
	}
	return a.wsPublic.Unsubscribe(channel)
}

func (a *SpotAdapter) StopWatchKlines(ctx context.Context, symbol string, interval exchanges.Interval) error {
	return exchanges.ErrNotSupported
}

func (a *SpotAdapter) StopWatchTrades(ctx context.Context, symbol string) error {
	return exchanges.ErrNotSupported
}

// WaitOrderBookReady waits for orderbook to be ready
func (a *SpotAdapter) WaitOrderBookReady(ctx context.Context, symbol string) error {
	instId := a.FormatSymbol(symbol)
	return a.BaseAdapter.WaitOrderBookReady(ctx, instId)
}

// GetLocalOrderBook retrieves locally maintained order book
func (a *SpotAdapter) GetLocalOrderBook(symbol string, depth int) *exchanges.OrderBook {
	instId := a.FormatSymbol(symbol)

	ob, ok := a.GetLocalOrderBookImplementation(instId)
	if !ok {
		return nil
	}

	bids, asks := ob.GetDepth(depth)
	return &exchanges.OrderBook{
		Symbol:    symbol,
		Timestamp: ob.Timestamp(),
		Bids:      bids,
		Asks:      asks,
	}
}

// ================= Helper Functions =================

func (a *SpotAdapter) mapOrderRest(o *okx.Order) *exchanges.Order {
	status := exchanges.OrderStatusPending
	switch o.State {
	case okx.OrderStatusLive:
		status = exchanges.OrderStatusNew
	case okx.OrderStatusPartiallyFilled:
		status = exchanges.OrderStatusPartiallyFilled
	case okx.OrderStatusFilled:
		status = exchanges.OrderStatusFilled
	case okx.OrderStatusCanceled, okx.OrderStatusMmpCanceled:
		status = exchanges.OrderStatusCancelled
	}

	side := exchanges.OrderSideBuy
	if o.Side == okx.SideSell {
		side = exchanges.OrderSideSell
	}

	qty := parseString(o.Sz)
	filledQty := parseString(o.AccFillSz)

	return &exchanges.Order{
		OrderID:        o.OrdId,
		ClientOrderID:  o.ClOrdId,
		Symbol:         a.ExtractSymbol(o.InstId),
		Side:           side,
		Type:           mapOKXOrderType(o.OrdType),
		Quantity:       qty,
		Price:          parseString(o.Px),
		Status:         status,
		FilledQuantity: filledQty,
		Timestamp:      parseTime(o.CTime),
		Fee:            parseString(o.Fee),
	}
}

func mapOKXOrderType(t okx.OrderType) exchanges.OrderType {
	switch t {
	case okx.OrderTypeMarket:
		return exchanges.OrderTypeMarket
	case okx.OrderTypeLimit:
		return exchanges.OrderTypeLimit
	case okx.OrderTypePostOnly:
		return exchanges.OrderTypePostOnly
	case okx.OrderTypeFok:
		return exchanges.OrderTypeLimit
	case okx.OrderTypeIoc:
		return exchanges.OrderTypeLimit
	default:
		return exchanges.OrderTypeUnknown
	}
}

func (a *SpotAdapter) WatchPositions(ctx context.Context, cb exchanges.PositionUpdateCallback) error {
	return exchanges.ErrNotSupported
}

func (a *SpotAdapter) StopWatchPositions(ctx context.Context) error {
	return exchanges.ErrNotSupported
}

func (a *SpotAdapter) StopWatchOrderBook(ctx context.Context, symbol string) error {
	instId := a.FormatSymbol(symbol)
	channel := okx.WsSubscribeArgs{
		Channel: "books",
		InstId:  instId,
	}
	a.RemoveLocalOrderBook(instId)
	return a.wsPublic.Unsubscribe(channel)
}

func (a *SpotAdapter) WatchOrderBook(ctx context.Context, symbol string, callback exchanges.OrderBookCallback) error {
	if err := a.WsMarketConnected(ctx); err != nil {
		return err
	}
	instId := a.FormatSymbol(symbol)

	// Spot orderbook: sizes are in base currency (no ctVal conversion needed)
	ob := NewOrderBook(symbol, decimal.NewFromInt(1))
	a.SetLocalOrderBook(instId, ob) // Store under instId to match GetLocalOrderBook override

	if err := a.wsPublic.SubscribeOrderBook(instId, func(data *okx.OrderBook, action string) {
		ob.ProcessUpdate(data, action)
		if callback != nil {
			callback(ob.ToAdapterOrderBook(20))
		}
	}); err != nil {
		return err
	}
	return a.WaitOrderBookReady(ctx, symbol)
}
