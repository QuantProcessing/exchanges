package standx

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/standx/sdk"

	"github.com/shopspring/decimal"
)

type Adapter struct {
	*exchanges.BaseAdapter
	// Lifecycle Context (for background tasks like WS)
	lifecycleCtx context.Context
	cancel       context.CancelFunc

	client    *standx.Client
	wsMarket  *standx.WsMarketClient
	wsAccount *standx.WsAccountClient
	wsApi     *standx.WsApiClient

	cancelMu sync.Mutex
	cancels  map[string]context.CancelFunc

	// Cached fee rates (per-symbol)
	feeCache         sync.Map // symbol -> *exchanges.FeeRate
	hasPrivateAccess bool
}

func NewAdapter(ctx context.Context, opts Options) (exchanges.Exchange, error) {
	if _, err := opts.quoteCurrency(); err != nil {
		return nil, err
	}
	// 1. Create lifecycle context for the adapter and its background services
	// This context is used to shut down the exchanges.
	aCtx, cancel := context.WithCancel(ctx)

	// 2. Initialize Clients
	client := standx.NewClient()
	// StandX allows public-market usage without credentials; a private key enables account and order flows.
	if opts.PrivateKey != "" {
		if _, err := client.WithCredentials(opts.PrivateKey); err != nil {
			cancel()
			return nil, exchanges.NewExchangeError("STANDX", "", fmt.Sprintf("invalid private_key: %v", err), exchanges.ErrAuthFailed)
		}
	}

	// 3. Perform Initial Login (Propagate Lifecycle Context, or use a short timeout?)
	// Login is a "start-up" action. If it fails, we fail to create the exchanges.
	if opts.PrivateKey != "" {
		loginCtx, loginCancel := context.WithTimeout(aCtx, 10*time.Second)
		defer loginCancel()
		if err := client.Login(loginCtx); err != nil {
			cancel()
			return nil, exchanges.NewExchangeError("STANDX", "", fmt.Sprintf("login failed: %v", err), exchanges.ErrAuthFailed)
		}
	}

	// 4. Initialize WS Clients
	// They need the lifecycle context to manage their run loops.
	wsMarket := standx.NewWsMarketClient(aCtx)
	wsAccount := standx.NewWsAccountClient(aCtx, client)
	wsApi := standx.NewWsApiClient(aCtx, client)

	base := exchanges.NewBaseAdapter("STANDX", exchanges.MarketTypePerp, opts.logger())
	// StandX is a controlled hybrid adapter: place/cancel switch on OrderMode, while some private setup stays WS-backed.

	a := &Adapter{
		BaseAdapter:      base,
		lifecycleCtx:     aCtx,
		cancel:           cancel,
		client:           client,
		wsMarket:         wsMarket,
		wsAccount:        wsAccount,
		wsApi:            wsApi,
		cancels:          make(map[string]context.CancelFunc),
		hasPrivateAccess: opts.PrivateKey != "",
	}

	// 5. Preload symbol details cache
	if err := a.RefreshSymbolDetails(aCtx); err != nil {
		a.Logger.Warnw("Failed to preload symbol details, will fetch on demand", "error", err)
	}

	return a, nil
}

func (a *Adapter) Close() error {
	a.cancel()
	if a.wsMarket != nil {
		a.wsMarket.Close()
	}
	if a.wsAccount != nil {
		a.wsAccount.Close()
	}
	if a.wsApi != nil {
		a.wsApi.Close()
	}
	return nil
}

// ================= Connection Hooks =================

func (a *Adapter) WsMarketConnected(ctx context.Context) error {
	// Connect is idempotent in our SDKs usually, but valid to call.
	// It uses internal constraints. We trigger it here.
	return a.wsMarket.Connect()
}

func (a *Adapter) WsAccountConnected(ctx context.Context) error {
	if err := a.requirePrivateAccess(); err != nil {
		return err
	}
	if err := a.wsAccount.Connect(); err != nil {
		return err
	}
	// Attempt Auth immediately
	return a.wsAccount.Auth()
}

func (a *Adapter) WsOrderConnected(ctx context.Context) error {
	if err := a.requirePrivateAccess(); err != nil {
		return err
	}
	if err := a.wsApi.Connect(); err != nil {
		return err
	}
	return a.wsApi.Auth()
}

// ================= Account & Trading =================

func (a *Adapter) FetchAccount(ctx context.Context) (*exchanges.Account, error) {
	if err := a.requirePrivateAccess(); err != nil {
		return nil, err
	}
	// Balance and Positions are separate calls in Standx SDK
	// We run them sequentially for simplicity.
	balance, err := a.client.QueryBalances(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query balance: %w", err)
	}

	positions, err := a.client.QueryPositions(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("failed to query positions: %w", err)
	}

	// We can also fetch open orders if we want a full snapshot,
	// but exchanges.Account typically focuses on Balance/Positions.
	// The interface includes Orders []Order, so we should fetch them.
	// QueryUserAllOpenOrders typically takes symbol. If empty, returns all?
	// SDK docs might require symbol or "" for all. Assuming "" works for all.
	orders, err := a.client.QueryUserAllOpenOrders(ctx, "")
	if err != nil {
		// Log error but don't fail entire account fetch?
		// Or fail. Strict implementation prefers failing.
		// However, some exchanges make this heavy. Standx should be fine.
		return nil, fmt.Errorf("failed to query open orders: %w", err)
	}

	acc := &exchanges.Account{
		Positions: []exchanges.Position{},
		Orders:    []exchanges.Order{},
	}

	// Standx Balance Mapping
	acc.TotalBalance = parseDecimal(balance.Balance)
	acc.AvailableBalance = parseDecimal(balance.CrossAvailable)
	acc.UnrealizedPnL = parseDecimal(balance.Upnl)

	// Positions Mapping
	for _, p := range positions {
		size := parseDecimal(p.Qty)
		if size.IsZero() {
			continue
		}
		side := exchanges.PositionSideLong
		if size.IsNegative() {
			side = exchanges.PositionSideShort
		}
		acc.Positions = append(acc.Positions, exchanges.Position{
			Symbol:           a.toAdapterSymbol(p.Symbol),
			Side:             side,
			Quantity:         size,
			EntryPrice:       parseDecimal(p.EntryPrice),
			UnrealizedPnL:    parseDecimal(p.Upnl), // Ensure accessing correct field
			MarginType:       p.MarginMode,
			Leverage:         parseDecimal(p.Leverage),
			LiquidationPrice: parseDecimal(p.LiqPrice),
		})
	}

	// Orders Mapping
	for _, o := range orders {
		acc.Orders = append(acc.Orders, a.mapSDKOrderToAdapterOrder(o))
	}

	return acc, nil
}

func (a *Adapter) FetchBalance(ctx context.Context) (decimal.Decimal, error) {
	if err := a.requirePrivateAccess(); err != nil {
		return decimal.Zero, err
	}
	bal, err := a.client.QueryBalances(ctx)
	if err != nil {
		return decimal.Zero, err
	}
	return parseDecimal(bal.CrossAvailable), nil
}

func (a *Adapter) FetchPositions(ctx context.Context) ([]exchanges.Position, error) {
	if err := a.requirePrivateAccess(); err != nil {
		return nil, err
	}
	rawPositions, err := a.client.QueryPositions(ctx, "")
	if err != nil {
		return nil, err
	}
	var res []exchanges.Position
	for _, p := range rawPositions {
		size := parseDecimal(p.Qty)
		if size.IsZero() {
			continue
		}
		side := exchanges.PositionSideLong
		if size.IsNegative() {
			side = exchanges.PositionSideShort
		}
		res = append(res, exchanges.Position{
			Symbol:           a.toAdapterSymbol(p.Symbol),
			Side:             side,
			Quantity:         size,
			EntryPrice:       parseDecimal(p.EntryPrice),
			UnrealizedPnL:    parseDecimal(p.Upnl),
			MarginType:       p.MarginMode,
			Leverage:         parseDecimal(p.Leverage),
			LiquidationPrice: parseDecimal(p.LiqPrice),
		})
	}
	return res, nil
}

func (a *Adapter) PlaceOrder(ctx context.Context, params *exchanges.OrderParams) (*exchanges.Order, error) {
	// Apply slippage logic: converts MARKET+Slippage to LIMIT+IOC
	if err := a.BaseAdapter.ApplySlippage(ctx, params, a.FetchTicker); err != nil {
		return nil, err
	}
	if err := a.ensureAPIReady(ctx); err != nil {
		return nil, err
	}

	// 1. Validate and normalize price/quantity according to exchange tick size
	if err := a.normalizeOrderParams(ctx, params); err != nil {
		return nil, fmt.Errorf("invalid order params: %w", err)
	}

	// 2. Prepare Request
	if params.ClientID == "" {
		params.ClientID = standx.GenRequestID()
	}

	req := standx.CreateOrderRequest{
		Symbol:      a.toExchangeSymbol(params.Symbol),
		Side:        mapAdapterSideToSDKSide(params.Side),
		OrderType:   mapAdapterTypeToSDKType(params.Type),
		Qty:         a.formatQuantity(params.Quantity, params.Symbol),
		TimeInForce: mapAdapterTIFToSDKTIF(params.TimeInForce),
		ClientOrdID: params.ClientID,
		ReduceOnly:  params.ReduceOnly,
	}
	if params.Type == exchanges.OrderTypeLimit {
		req.Price = a.formatPrice(params.Price, params.Symbol)
	}

	// REST mode: use HTTP client directly
	if a.IsRESTMode() {
		resp, err := a.client.CreateOrder(ctx, req, nil)
		if err != nil {
			return nil, err
		}
		if resp.Code != 0 {
			return nil, fmt.Errorf("create order failed: code=%d msg=%s", resp.Code, resp.Message)
		}
		return &exchanges.Order{
			OrderID:       params.ClientID,
			Symbol:        params.Symbol,
			Side:          params.Side,
			Type:          params.Type,
			Quantity:      params.Quantity,
			Price:         params.Price,
			Status:        exchanges.OrderStatusNew,
			ClientOrderID: params.ClientID,
			Timestamp:     time.Now().UnixMilli(),
		}, nil
	}

	// WS mode: Send Request via WS API
	resp, err := a.wsApi.CreateOrder(ctx, &req)
	if err != nil {
		return nil, err
	}

	if resp.Code != 0 {
		return nil, fmt.Errorf("create order failed: code=%d msg=%s", resp.Code, resp.Message)
	}

	// Success
	return &exchanges.Order{
		OrderID:       params.ClientID, // server not return order id, use client order id instead
		Symbol:        params.Symbol,
		Side:          params.Side,
		Type:          params.Type,
		Quantity:      params.Quantity,
		Price:         params.Price,
		Status:        exchanges.OrderStatusNew,
		ClientOrderID: params.ClientID,
		Timestamp:     time.Now().UnixMilli(),
	}, nil
}

func (a *Adapter) CancelOrder(ctx context.Context, orderID, symbol string) error {
	if err := a.ensureAPIReady(ctx); err != nil {
		return err
	}

	req := standx.CancelOrderRequest{
		Symbol: a.toExchangeSymbol(symbol),
	}
	// Check if orderID is numeric (Exchange ID) or alphanumeric (Client ID)
	if _, err := strconv.ParseInt(orderID, 10, 64); err == nil {
		req.OrderID = orderID
	} else {
		req.ClOrdID = orderID
	}

	// REST mode
	if a.IsRESTMode() {
		resp, err := a.client.CancelOrder(ctx, req)
		if err != nil {
			return err
		}
		if resp.Code != 0 {
			return fmt.Errorf("cancel order failed: code=%d msg=%s", resp.Code, resp.Message)
		}
		return nil
	}

	// WS mode
	resp, err := a.wsApi.CancelOrder(ctx, &req)
	if err != nil {
		return err
	}
	if resp.Code != 0 {
		return fmt.Errorf("cancel order failed: code=%d msg=%s", resp.Code, resp.Message)
	}
	return nil
}

func (a *Adapter) CancelAllOrders(ctx context.Context, symbol string) error {
	if err := a.ensureAPIReady(ctx); err != nil {
		return err
	}

	// CancelMultipleOrders takes OrderIDs.
	// To Cancel ALL, we first query all open orders.
	exchSymbol := a.toExchangeSymbol(symbol)
	orders, err := a.client.QueryUserAllOpenOrders(ctx, exchSymbol)
	if err != nil {
		return err
	}
	if len(orders) == 0 {
		return nil
	}

	var ids []interface{}
	for _, o := range orders {
		ids = append(ids, o.ID)
	}

	req := standx.CancelOrdersRequest{
		Symbol:   exchSymbol,
		OrderIDs: ids,
	}
	_, err = a.client.CancelMultipleOrders(ctx, req)
	return err
}

func (a *Adapter) ModifyOrder(ctx context.Context, orderID, symbol string, params *exchanges.ModifyOrderParams) (*exchanges.Order, error) {
	return nil, exchanges.ErrNotSupported
}

func (a *Adapter) FetchOrderByID(ctx context.Context, orderID, symbol string) (*exchanges.Order, error) {
	return nil, exchanges.ErrNotSupported
}

func (a *Adapter) FetchOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	return nil, exchanges.ErrNotSupported
}

func (a *Adapter) FetchOpenOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	if err := a.requirePrivateAccess(); err != nil {
		return nil, err
	}
	orders, err := a.client.QueryUserAllOpenOrders(ctx, a.toExchangeSymbol(symbol))
	if err != nil {
		return nil, err
	}
	var res []exchanges.Order
	for _, o := range orders {
		res = append(res, a.mapSDKOrderToAdapterOrder(o))
	}
	return res, nil
}

func (a *Adapter) SetLeverage(ctx context.Context, symbol string, leverage int) error {
	if err := a.requirePrivateAccess(); err != nil {
		return err
	}
	// ChangeLeverage uses HTTP client, so no ensureAPIReady needed for wsApi
	req := standx.ChangeLeverageRequest{
		Symbol:   a.toExchangeSymbol(symbol),
		Leverage: leverage,
	}
	_, err := a.client.ChangeLeverage(ctx, req)
	return err
}

func (a *Adapter) FetchFeeRate(ctx context.Context, symbol string) (*exchanges.FeeRate, error) {
	if v, ok := a.feeCache.Load(symbol); ok {
		return v.(*exchanges.FeeRate), nil
	}
	// Query Symbol info
	exchSymbol := a.toExchangeSymbol(symbol)
	infoList, err := a.client.QuerySymbolInfo(ctx, exchSymbol)
	if err != nil {
		return nil, err
	}
	for _, info := range infoList {
		if strings.EqualFold(info.Symbol, exchSymbol) {
			res := &exchanges.FeeRate{
				Maker: parseDecimal(info.MakerFee),
				Taker: parseDecimal(info.TakerFee),
			}
			a.feeCache.Store(symbol, res)
			return res, nil
		}
	}
	return nil, fmt.Errorf("symbol info not found")
}

// ================= Market Data =================

func (a *Adapter) FetchTicker(ctx context.Context, symbol string) (*exchanges.Ticker, error) {
	t, err := a.client.QuerySymbolMarket(ctx, a.toExchangeSymbol(symbol))
	if err != nil {
		return nil, err
	}
	return &exchanges.Ticker{
		Symbol:    a.toAdapterSymbol(t.Symbol),
		LastPrice: parseDecimal(t.LastPrice),
		Bid:       parseDecimal(t.Spread[0]),
		Ask:       parseDecimal(t.Spread[1]),
		Volume24h: decimal.NewFromFloat(t.Volume24h),
		QuoteVol:  decimal.NewFromFloat(t.VolumeQuote24h),
		High24h:   decimal.NewFromFloat(t.HighPrice24h),
		Low24h:    decimal.NewFromFloat(t.LowPrice24h),
		Timestamp: time.Now().UnixMilli(),
	}, nil
}

func (a *Adapter) FetchOrderBook(ctx context.Context, symbol string, limit int) (*exchanges.OrderBook, error) {
	res, err := a.client.QueryDepthBook(ctx, a.toExchangeSymbol(symbol), limit)
	if err != nil {
		return nil, err
	}
	bids := make([]exchanges.Level, len(res.Bids))
	for i, b := range res.Bids {
		if len(b) >= 2 {
			bids[i] = exchanges.Level{Price: parseDecimal(b[0]), Quantity: parseDecimal(b[1])}
		}
	}
	asks := make([]exchanges.Level, len(res.Asks))
	for i, item := range res.Asks {
		if len(item) >= 2 {
			asks[i] = exchanges.Level{Price: parseDecimal(item[0]), Quantity: parseDecimal(item[1])}
		}
	}
	return &exchanges.OrderBook{
		Symbol:    a.toAdapterSymbol(res.Symbol),
		Bids:      bids,
		Asks:      asks,
		Timestamp: time.Now().UnixMilli(),
	}, nil
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
	return nil, exchanges.ErrNotSupported
}

func (a *Adapter) FetchTrades(ctx context.Context, symbol string, limit int) ([]exchanges.Trade, error) {
	trades, err := a.client.QueryRecentTrades(ctx, a.toExchangeSymbol(symbol), limit)
	if err != nil {
		return nil, err
	}
	var res []exchanges.Trade
	for _, t := range trades {
		side := exchanges.TradeSideSell
		if t.IsBuyerTaker {
			side = exchanges.TradeSideBuy
		}
		res = append(res, exchanges.Trade{
			ID:        "",
			Price:     parseDecimal(t.Price),
			Quantity:  parseDecimal(t.Qty),
			Timestamp: 0,
			Side:      side,
		})
	}
	return res, nil
}

// normalizeOrderParams validates and rounds price/quantity to exchange tick size
func (a *Adapter) normalizeOrderParams(ctx context.Context, params *exchanges.OrderParams) error {
	details, err := a.FetchSymbolDetails(ctx, params.Symbol)
	if err != nil {
		return fmt.Errorf("failed to get symbol details: %w", err)
	}

	// Round quantity to precision
	params.Quantity = exchanges.FloorToPrecision(params.Quantity, details.QuantityPrecision)

	// Validate minimum quantity
	if params.Quantity.LessThan(details.MinQuantity) {
		return fmt.Errorf("quantity %s below minimum %s", params.Quantity, details.MinQuantity)
	}

	// Round price to precision (for limit orders)
	if params.Type == exchanges.OrderTypeLimit {
		params.Price = exchanges.RoundToPrecision(params.Price, details.PricePrecision)

		if !params.Price.IsPositive() {
			return fmt.Errorf("invalid price %s", params.Price)
		}
	}

	return nil
}

// formatPrice formats price according to symbol precision
func (a *Adapter) formatPrice(price decimal.Decimal, symbol string) string {
	ctx, cancel := context.WithTimeout(a.lifecycleCtx, 2*time.Second)
	defer cancel()
	details, err := a.FetchSymbolDetails(ctx, symbol)
	if err != nil {
		return price.StringFixed(6)
	}
	return price.StringFixed(details.PricePrecision)
}

// formatQuantity formats quantity according to symbol precision
func (a *Adapter) formatQuantity(qty decimal.Decimal, symbol string) string {
	ctx, cancel := context.WithTimeout(a.lifecycleCtx, 2*time.Second)
	defer cancel()
	details, err := a.FetchSymbolDetails(ctx, symbol)
	if err != nil {
		return qty.StringFixed(6)
	}
	return qty.StringFixed(details.QuantityPrecision)
}

func (a *Adapter) FetchSymbolDetails(ctx context.Context, symbol string) (*exchanges.SymbolDetails, error) {
	adapterSymbol := a.toAdapterSymbol(symbol)
	details, err := a.GetSymbolDetail(adapterSymbol)
	if err == nil {
		return details, nil
	}

	exchSymbol := a.toExchangeSymbol(symbol)

	// Cache miss, fetch from API
	infoList, err := a.client.QuerySymbolInfo(ctx, exchSymbol)
	if err != nil {
		return nil, err
	}
	for _, info := range infoList {
		if strings.EqualFold(info.Symbol, exchSymbol) {
			details := &exchanges.SymbolDetails{
				Symbol:            a.toAdapterSymbol(info.Symbol),
				PricePrecision:    int32(info.PriceTickDecimals),
				QuantityPrecision: int32(info.QtyTickDecimals),
				MinQuantity:       parseDecimal(info.MinOrderQty),
			}
			// Cache the result
			symbols := map[string]*exchanges.SymbolDetails{
				adapterSymbol: details,
			}
			a.SetSymbolDetails(symbols)
			return details, nil
		}
	}
	return nil, fmt.Errorf("symbol not found")
}

// ================= WebSocket =================

func (a *Adapter) WatchOrders(ctx context.Context, callback exchanges.OrderUpdateCallback) error {
	if err := a.WsAccountConnected(ctx); err != nil {
		return err
	}

	// Eagerly connect WS API here to avoid first-order timeout
	// This is non-blocking for the subscription, but ensures the connection
	// is ready when PlaceOrder is called
	go func() {
		warmCtx, cancel := context.WithTimeout(a.lifecycleCtx, 15*time.Second)
		defer cancel()
		if err := a.WsOrderConnected(warmCtx); err != nil {
			a.Logger.Warnw("Failed to pre-connect WS API (will retry on first order)", "error", err)
		} else {
			a.Logger.Infow("WS API pre-connected successfully")
		}
	}()

	return a.wsAccount.SubscribeOrderUpdate(func(o *standx.Order) {
		callback(a.mapSDKOrderToAdapterOrderPTR(o))
	})
}

func (a *Adapter) WatchPositions(ctx context.Context, callback exchanges.PositionUpdateCallback) error {
	if err := a.WsAccountConnected(ctx); err != nil {
		return err
	}
	return a.wsAccount.SubscribePositionUpdate(func(p *standx.Position) {
		qty := parseDecimal(p.Qty)
		side := exchanges.PositionSideLong
		if qty.IsNegative() {
			side = exchanges.PositionSideShort
		}
		// Map Position
		pos := exchanges.Position{
			Symbol:           a.toAdapterSymbol(p.Symbol),
			Side:             side,
			Quantity:         qty,
			EntryPrice:       parseDecimal(p.EntryPrice),
			UnrealizedPnL:    parseDecimal(p.Upnl),
			MarginType:       p.MarginMode,
			Leverage:         parseDecimal(p.Leverage),
			LiquidationPrice: parseDecimal(p.LiqPrice),
		}
		callback(&pos)
	})
}

func (a *Adapter) WatchTicker(ctx context.Context, symbol string, callback exchanges.TickerCallback) error {
	if err := a.WsMarketConnected(ctx); err != nil {
		return err
	}

	return a.wsMarket.SubscribePrice(a.toExchangeSymbol(symbol), func(data []byte) error {
		var priceData standx.WSPriceData
		if err := json.Unmarshal(data, &priceData); err != nil {
			return err
		}
		// priceData.Time is 2025-08-11T07:23:50.923602474Z
		time, err := time.Parse(time.RFC3339Nano, priceData.Time)
		if err != nil {
			return err
		}
		callback(&exchanges.Ticker{
			Symbol:     a.toAdapterSymbol(priceData.Symbol),
			LastPrice:  parseDecimal(priceData.LastPrice),
			MarkPrice:  parseDecimal(priceData.MarkPrice),
			IndexPrice: parseDecimal(priceData.IndexPrice),
			MidPrice:   parseDecimal(priceData.MidPrice),
			Bid:        parseDecimal(priceData.Spread[0]),
			Ask:        parseDecimal(priceData.Spread[1]),
			Timestamp:  time.UnixMilli(), // fact is nano, but return ms standard
		})
		return nil
	})
}

// WatchOrderBook subscribes to orderbook updates and waits for the book to be ready.
func (a *Adapter) WatchOrderBook(ctx context.Context, symbol string, callback exchanges.OrderBookCallback) error {
	if err := a.WsMarketConnected(ctx); err != nil {
		return err
	}
	exchSymbol := a.toExchangeSymbol(symbol)

	a.cancelMu.Lock()
	if cancel, ok := a.cancels[symbol]; ok {
		cancel()
	}
	ob := NewOrderBook(exchSymbol)
	a.SetLocalOrderBook(symbol, ob)
	_, cancel := context.WithCancel(context.Background())
	a.cancels[symbol] = cancel
	a.cancelMu.Unlock()

	err := a.wsMarket.SubscribeDepthBook(exchSymbol, func(data []byte) error {
		var depthData standx.WSDepthData
		if err := json.Unmarshal(data, &depthData); err != nil {
			return err
		}
		ob.UpdateSnapshot(depthData)
		if callback != nil {
			callback(ob.Snapshot())
		}
		return nil
	})
	if err != nil {
		return err
	}
	return a.BaseAdapter.WaitOrderBookReady(ctx, symbol)
}

func (a *Adapter) WatchKlines(ctx context.Context, symbol string, interval exchanges.Interval, callback exchanges.KlineCallback) error {
	return exchanges.ErrNotSupported
}

func (a *Adapter) WatchTrades(ctx context.Context, symbol string, callback exchanges.TradeCallback) error {
	return a.wsMarket.SubscribePublicTrade(a.toExchangeSymbol(symbol), func(data []byte) error {
		return nil
	})
}

// Unsubscribes
func (a *Adapter) StopWatchOrders(ctx context.Context) error                { return nil }
func (a *Adapter) StopWatchPositions(ctx context.Context) error             { return nil }
func (a *Adapter) StopWatchTicker(ctx context.Context, symbol string) error { return nil }
func (a *Adapter) StopWatchOrderBook(ctx context.Context, symbol string) error {
	a.cancelMu.Lock()
	if cancel, ok := a.cancels[symbol]; ok {
		cancel()
		delete(a.cancels, symbol)
	}
	a.cancelMu.Unlock()
	a.RemoveLocalOrderBook(symbol)
	return nil
}
func (a *Adapter) StopWatchKlines(ctx context.Context, symbol string, interval exchanges.Interval) error {
	return exchanges.ErrNotSupported
}
func (a *Adapter) StopWatchTrades(ctx context.Context, symbol string) error { return nil }

// ================= Helpers & Internals =================

func (a *Adapter) ensureAPIReady(ctx context.Context) error {
	if err := a.requirePrivateAccess(); err != nil {
		return err
	}
	return a.WsOrderConnected(ctx)
}

func (a *Adapter) requirePrivateAccess() error {
	if !a.hasPrivateAccess {
		return exchanges.NewExchangeError("STANDX", "", "private API not available (no credentials configured)", exchanges.ErrAuthFailed)
	}
	return nil
}

func (a *Adapter) WaitOrderBookReady(ctx context.Context, symbol string) error {
	return a.BaseAdapter.WaitOrderBookReady(ctx, symbol)
}

func (a *Adapter) GetLocalOrderBook(symbol string, depth int) *exchanges.OrderBook {
	ob, ok := a.GetLocalOrderBookImplementation(symbol)
	if !ok {
		return nil
	}
	snapshot := ob.(*OrderBook).Snapshot()
	if depth > 0 {
		bids, asks := ob.(*OrderBook).GetDepth(depth)
		snapshot.Bids = bids
		snapshot.Asks = asks
	}
	return snapshot
}

func (a *Adapter) RefreshSymbolDetails(ctx context.Context) error {
	// Query all symbols info
	infoList, err := a.client.QuerySymbolInfo(ctx, "")
	if err != nil {
		return err
	}

	symbols := make(map[string]*exchanges.SymbolDetails)

	for _, info := range infoList {
		adapterSymbol := a.toAdapterSymbol(info.Symbol)
		symbols[adapterSymbol] = &exchanges.SymbolDetails{
			Symbol:            adapterSymbol,
			PricePrecision:    int32(info.PriceTickDecimals),
			QuantityPrecision: int32(info.QtyTickDecimals),
			MinQuantity:       parseDecimal(info.MinOrderQty),
		}
	}

	a.SetSymbolDetails(symbols)

	a.Logger.Infow("Symbol details cache refreshed", "count", len(symbols))
	return nil
}
func (a *Adapter) FormatSymbol(symbol string) string  { return a.toExchangeSymbol(symbol) }
func (a *Adapter) ExtractSymbol(symbol string) string { return a.toAdapterSymbol(symbol) }

func (a *Adapter) toExchangeSymbol(symbol string) string {
	symbol = strings.ToUpper(symbol)
	if !strings.HasSuffix(symbol, "-USD") {
		return fmt.Sprintf("%s-USD", symbol)
	}
	return symbol
}

func (a *Adapter) toAdapterSymbol(symbol string) string {
	symbol = strings.ToUpper(symbol)
	return strings.TrimSuffix(symbol, "-USD")
}

// Utils

func parseDecimal(s string) decimal.Decimal {
	d, _ := decimal.NewFromString(s)
	return d
}

func parsePositionSide(s string) exchanges.PositionSide {
	s = strings.ToLower(s)
	if s == "long" || s == "buy" {
		return exchanges.PositionSideLong
	}
	if s == "short" || s == "sell" {
		return exchanges.PositionSideShort
	}
	return exchanges.PositionSideBoth
}

func parseOrderSide(s standx.OrderSide) exchanges.OrderSide {
	if s == standx.SideBuy {
		return exchanges.OrderSideBuy
	}
	if s == standx.SideSell {
		return exchanges.OrderSideSell
	}
	return exchanges.OrderSideBuy // default?
}

func (a *Adapter) mapSDKOrderToAdapterOrder(o standx.Order) exchanges.Order {
	// Need to map standx side string to OrderSide
	var side exchanges.OrderSide
	s := strings.ToLower(o.Side)
	if s == "buy" {
		side = exchanges.OrderSideBuy
	} else {
		side = exchanges.OrderSideSell
	}

	orderID := fmt.Sprintf("%d", o.ID)
	if o.ClOrdID != "" {
		orderID = o.ClOrdID
	}

	order := exchanges.Order{
		OrderID:        orderID,
		Symbol:         a.toAdapterSymbol(o.Symbol),
		Side:           side,
		Type:           exchanges.OrderType(strings.ToUpper(o.OrderType)), // rough mapping
		Quantity:       parseDecimal(o.Qty),
		FilledQuantity: parseDecimal(o.FillQty),
		Price:          parseDecimal(o.Price),
		Status:         mapSDKStatus(o.Status),
		TimeInForce:    mapSDKTIF(o.TimeInForce),
		ReduceOnly:     o.ReduceOnly,
		ClientOrderID:  o.ClOrdID,
		Timestamp:      0,
	}
	exchanges.DerivePartialFillStatus(&order)
	return order
}

func (a *Adapter) mapSDKOrderToAdapterOrderPTR(o *standx.Order) *exchanges.Order {
	ord := a.mapSDKOrderToAdapterOrder(*o)
	return &ord
}

func mapSDKStatus(s string) exchanges.OrderStatus {
	switch strings.ToLower(s) {
	case standx.OrderStatusOpen, standx.OrderStatusNew:
		return exchanges.OrderStatusNew
	case standx.OrderStatusFilled:
		return exchanges.OrderStatusFilled
	case standx.OrderStatusCancelled, standx.OrderStatusCanceled:
		return exchanges.OrderStatusCancelled
	case standx.OrderStatusRejected:
		return exchanges.OrderStatusRejected
	case standx.OrderStatusUntriggered:
		return exchanges.OrderStatusNew // untriggered conditional orders mapped as New
	default:
		return exchanges.OrderStatusUnknown
	}
}

func mapSDKTIF(s string) exchanges.TimeInForce {
	switch strings.ToLower(s) {
	case "gtc":
		return exchanges.TimeInForceGTC
	case "ioc":
		return exchanges.TimeInForceIOC
	case "fok": // SDK might return rok/fok
		return exchanges.TimeInForceFOK
	case "alo":
		return exchanges.TimeInForcePO // Post Only
	default:
		return exchanges.TimeInForceGTC
	}
}

func mapAdapterSideToSDKSide(s exchanges.OrderSide) standx.OrderSide {
	if s == exchanges.OrderSideBuy {
		return standx.SideBuy
	}
	return standx.SideSell
}

func mapAdapterTypeToSDKType(t exchanges.OrderType) standx.OrderType {
	if t == exchanges.OrderTypeMarket {
		return standx.OrderTypeMarket
	}
	return standx.OrderTypeLimit
}

func mapAdapterTIFToSDKTIF(t exchanges.TimeInForce) standx.TimeInForce {
	switch t {
	case exchanges.TimeInForceIOC:
		return standx.TimeInForceIOC
	case exchanges.TimeInForceFOK:
		return standx.TimeInForceFOK
	case exchanges.TimeInForcePO:
		// Standx likely uses ALO (Add or Limit Only) or similar for PO
		return standx.TimeInForceALO
	default:
		return standx.TimeInForceGTC
	}
}
