package lighter

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/lighter/sdk"

	"github.com/shopspring/decimal"
)

// Adapter Lighter 适配器
type Adapter struct {
	*exchanges.BaseAdapter
	client          *lighter.Client
	wsClient        *lighter.WebsocketClient
	tokenManager    *TokenManager
	hasAccountIndex bool
	hasReadAccess   bool
	hasWriteAccess  bool

	// Symbol <-> MarketID
	symbolToID map[string]int
	idToSymbol map[int]string
	marketMeta map[int]*lighter.OrderBookDetail

	metaMu      sync.RWMutex
	isConnected bool

	cancelMu sync.Mutex
	cancels  map[string]context.CancelFunc

	// Cached fee rate (account-wide, not per-symbol)
	feeOnce       sync.Once
	cachedFeeRate *exchanges.FeeRate
	cachedFeeErr  error
}

// NewAdapter 创建 Lighter 适配器
func NewAdapter(ctx context.Context, opts Options) (*Adapter, error) {
	if _, err := opts.quoteCurrency(); err != nil {
		return nil, err
	}
	if err := opts.validateCredentials(); err != nil {
		return nil, err
	}

	client := lighter.NewClient()
	wsClient := lighter.NewWebsocketClient(ctx)
	hasAccountIndex := opts.AccountIndex != ""

	if hasAccountIndex {
		accIndex, err := strconv.ParseInt(opts.AccountIndex, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse account index: %w", err)
		}
		client.AccountIndex = accIndex
	}
	if opts.KeyIndex != "" {
		ki, err := strconv.ParseUint(opts.KeyIndex, 10, 8)
		if err != nil {
			return nil, fmt.Errorf("parse key index: %w", err)
		}
		client.KeyIndex = uint8(ki)
	}

	// Only set up credentials if private key is provided
	if opts.PrivateKey != "" {
		client.WithCredentials(opts.PrivateKey, client.AccountIndex, client.KeyIndex)
		if client.KeyManager == nil {
			return nil, exchanges.NewExchangeError("LIGHTER", "", "invalid private_key", exchanges.ErrAuthFailed)
		}
	}

	base := exchanges.NewBaseAdapter("LIGHTER", exchanges.MarketTypePerp, opts.logger())
	// Lighter perp is a controlled hybrid adapter: order placement and cancellation can switch between WS and REST.

	a := &Adapter{
		BaseAdapter:     base,
		client:          client,
		wsClient:        wsClient,
		tokenManager:    NewTokenManager(client, opts.RoToken),
		hasAccountIndex: hasAccountIndex,
		hasReadAccess:   opts.RoToken != "" || opts.PrivateKey != "",
		hasWriteAccess:  opts.PrivateKey != "",
		symbolToID:      make(map[string]int),
		idToSymbol:      make(map[int]string),
		marketMeta:      make(map[int]*lighter.OrderBookDetail),
		cancels:         make(map[string]context.CancelFunc),
	}

	// Start TokenManager only if credentials are available
	if opts.PrivateKey != "" {
		if err := a.tokenManager.Start(ctx); err != nil {
			return nil, fmt.Errorf("start token manager: %w", err)
		}
	}

	// Init metadata
	if err := a.RefreshSymbolDetails(context.Background()); err != nil {
		return nil, fmt.Errorf("refresh symbol details: %w", err)
	}

	// TODO: logger.Info("Initialized Lighter Adapter")
	return a, nil
}

func (a *Adapter) WsAccountConnected(ctx context.Context) error {
	if err := a.requireReadAccess(); err != nil {
		return err
	}
	if a.wsClient.Conn == nil {
		if err := a.wsClient.Connect(); err != nil {
			return err
		}
	}

	return nil
}

func (a *Adapter) WsMarketConnected(ctx context.Context) error {
	if a.wsClient.Conn == nil {
		if err := a.wsClient.Connect(); err != nil {
			return err
		}
	}
	return nil
}

func (a *Adapter) WsOrderConnected(ctx context.Context) error {
	if err := a.requireWriteAccess(); err != nil {
		return err
	}
	if a.wsClient.Conn == nil {
		if err := a.wsClient.Connect(); err != nil {
			return err
		}
	}

	return nil
}

func (a *Adapter) refreshMetaInternal(ctx context.Context) error {
	// Must be called with lock held or ensure thread safety
	filter := "perp"
	res, err := a.client.GetOrderBookDetails(ctx, nil, &filter)
	if err != nil {
		return err
	}
	if res.Code != 200 {
		return fmt.Errorf("failed to get order book details: %d", res.Code)
	}

	a.symbolToID = make(map[string]int)
	a.idToSymbol = make(map[int]string)
	a.marketMeta = make(map[int]*lighter.OrderBookDetail)
	symbols := make(map[string]*exchanges.SymbolDetails)

	for _, detail := range res.OrderBookDetails {
		if detail.MarketType != "perp" {
			continue
		}
		mid := int(detail.MarketId)
		a.symbolToID[detail.Symbol] = mid
		a.idToSymbol[mid] = detail.Symbol
		a.marketMeta[mid] = detail

		details := &exchanges.SymbolDetails{
			Symbol:            detail.Symbol,
			PricePrecision:    int32(detail.PriceDecimals),
			QuantityPrecision: int32(detail.SizeDecimals),
			MinQuantity:       parseString(detail.MinBaseAmount),
		}
		symbols[detail.Symbol] = details
	}
	a.SetSymbolDetails(symbols)
	return nil
}

func (a *Adapter) IsConnected(ctx context.Context) (bool, error) {
	// Simple check
	_, err := a.client.GetL1Metadata(ctx)
	return err == nil, err
}

func (a *Adapter) Close() error {
	a.wsClient.Close()
	return nil
}

func (a *Adapter) RefreshSymbolDetails(ctx context.Context) error {
	a.metaMu.Lock()
	defer a.metaMu.Unlock()
	return a.refreshMetaInternal(ctx)
}

func (a *Adapter) FormatSymbol(symbol string) string {
	return strings.ToUpper(symbol)
}

func (a *Adapter) ExtractSymbol(symbol string) string {
	return strings.ToUpper(symbol)
}

// ================= Account & Trading =================

func (a *Adapter) FetchAccount(ctx context.Context) (*exchanges.Account, error) {
	if err := a.requireAccountAccess(); err != nil {
		return nil, err
	}
	res, err := a.client.GetAccount(ctx)
	if err != nil {
		return nil, err
	}
	if len(res.Accounts) == 0 {
		return nil, fmt.Errorf("no account found")
	}

	accData := res.Accounts[0]
	account := &exchanges.Account{
		Positions: []exchanges.Position{},
		Orders:    []exchanges.Order{},
	}

	account.AvailableBalance = parseString(accData.AvailableBalance)
	account.TotalBalance = parseString(accData.TotalAssetValue)

	marketIds := make([]int, 0, len(accData.Positions))
	for _, p := range accData.Positions {
		if p == nil {
			continue
		}
		marketIds = append(marketIds, p.MarketId)
	}

	for _, p := range accData.Positions {
		if p == nil {
			continue
		}
		qty := parseString(p.Position)
		if qty.IsZero() {
			continue
		}

		side := exchanges.PositionSideLong
		if qty.IsNegative() {
			side = exchanges.PositionSideShort
		}

		marginType := "CROSSED"
		if p.MarginMode == 1 {
			marginType = "ISOLATED"
		}

		account.Positions = append(account.Positions, exchanges.Position{
			Symbol:           p.Symbol,
			Side:             side,
			Quantity:         qty,
			EntryPrice:       parseString(p.AvgEntryPrice),
			UnrealizedPnL:    parseString(p.UnrealizedPnl),
			LiquidationPrice: parseString(p.LiquidationPrice),
			MarginType:       marginType,
		})
	}

	// Active-order queries require write credentials because the Lighter endpoint
	// depends on a signed auth token rather than the read-only token path.
	// Read-only accounts therefore return balances and positions here, but leave
	// Account.Orders empty by design.
	if a.hasWriteAccess {
		for _, marketId := range marketIds {
			orderRes, err := a.client.GetAccountActiveOrders(ctx, marketId)
			if err != nil {
				return nil, err
			}
			for _, o := range orderRes.Orders {
				if o == nil {
					continue
				}
				account.Orders = append(account.Orders, exchanges.Order{
					OrderID:        o.OrderId,
					ClientOrderID:  o.ClientOrderId,
					Symbol:         a.idToSymbol[o.MarketIndex],
					Side:           exchanges.OrderSide(o.Side),
					Quantity:       parseString(o.InitialBaseAmount),
					FilledQuantity: parseString(o.FilledBaseAmount),
					Price:          parseString(o.Price),
					Status:         exchanges.OrderStatus(o.Status),
					Type:           exchanges.OrderType(o.OrderType),
					TimeInForce:    exchanges.TimeInForce(o.TimeInForce),
				})
			}
		}
	}

	return account, nil
}

func (a *Adapter) FetchBalance(ctx context.Context) (decimal.Decimal, error) {
	acc, err := a.FetchAccount(ctx)
	if err != nil {
		return decimal.Zero, err
	}
	return acc.TotalBalance, nil
}

func (a *Adapter) FetchPositions(ctx context.Context) ([]exchanges.Position, error) {
	acc, err := a.FetchAccount(ctx)
	if err != nil {
		return nil, err
	}
	return acc.Positions, nil
}

func (a *Adapter) PlaceOrder(ctx context.Context, params *exchanges.OrderParams) (*exchanges.Order, error) {
	// Lighter doesn't support true market orders (price=0 is rejected).
	// Auto-apply default slippage to convert MARKET to aggressive LIMIT+IOC.
	if params.Type == exchanges.OrderTypeMarket && params.Slippage.IsZero() {
		params.Slippage = decimal.NewFromFloat(0.02) // 2% default
	}

	// Apply slippage logic: converts MARKET+Slippage to LIMIT+IOC
	if err := a.BaseAdapter.ApplySlippage(ctx, params, a.FetchTicker); err != nil {
		return nil, err
	}
	// 1. Validation & Formatting
	details, err := a.FetchSymbolDetails(ctx, params.Symbol)
	if err == nil {
		if params.Type == exchanges.OrderTypeLimit || params.Price.IsPositive() {
			params.Price = exchanges.RoundToPrecision(params.Price, details.PricePrecision)
		}
		params.Quantity = exchanges.FloorToPrecision(params.Quantity, details.QuantityPrecision)

		if params.Quantity.LessThan(details.MinQuantity) {
			return nil, fmt.Errorf("quantity %v less than min quantity %v", params.Quantity, details.MinQuantity)
		}
		if params.Type == exchanges.OrderTypeLimit && details.MinNotional.IsPositive() && params.Price.Mul(params.Quantity).LessThan(details.MinNotional) {
			return nil, fmt.Errorf("notional %v less than min notional %v", params.Price.Mul(params.Quantity), details.MinNotional)
		}
	}

	if err := a.WsOrderConnected(ctx); err != nil {
		return nil, err
	}

	a.metaMu.RLock()
	mid, ok := a.symbolToID[a.FormatSymbol(params.Symbol)]
	meta, hasMeta := a.marketMeta[mid]
	a.metaMu.RUnlock()

	if !ok || !hasMeta {
		return nil, fmt.Errorf("unknown symbol or missing meta: %s", params.Symbol)
	}

	isAsk := uint32(0)
	if params.Side == exchanges.OrderSideSell {
		isAsk = 1
	}

	orderType := uint32(lighter.OrderTypeLimit)
	tif := uint32(lighter.OrderTimeInForceGoodTillTime)

	switch params.Type {
	case exchanges.OrderTypeLimit:
		orderType = lighter.OrderTypeLimit
	case exchanges.OrderTypeMarket:
		orderType = lighter.OrderTypeMarket
	case exchanges.OrderTypePostOnly:
		orderType = lighter.OrderTypeLimit
		tif = lighter.OrderTimeInForcePostOnly
	}

	switch params.TimeInForce {
	case exchanges.TimeInForceIOC:
		tif = lighter.OrderTimeInForceImmediateOrCancel
	case exchanges.TimeInForcePO:
		tif = lighter.OrderTimeInForcePostOnly
	}

	reduceOnly := uint32(0)
	if params.ReduceOnly {
		reduceOnly = 1
	}

	// Set Expiry
	// IOC orders don't need expiry (set to 0)
	// All other orders (GoodTillTime, PostOnly) need expiry
	expiry := int64(0)
	if tif != lighter.OrderTimeInForceImmediateOrCancel {
		// Lighter expects Milliseconds (verified via failure with Unix/Seconds)
		expiry = time.Now().Add(time.Hour).UnixMilli()
	}

	priceVal := a.scalePrice(params.Price.InexactFloat64(), meta)
	sizeVal := a.scaleSize(params.Quantity.InexactFloat64(), meta)

	// Generate ClientOrderID (must be int64)
	var clientOidInt int64
	if params.ClientID != "" {
		parsed, err := strconv.ParseInt(params.ClientID, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("client order id is not int64: %s", params.ClientID)
		}
		clientOidInt = parsed
	} else {
		clientOidInt = time.Now().UnixMilli()
	}

	req := lighter.CreateOrderRequest{
		MarketId:      mid,
		Price:         uint32(priceVal),
		BaseAmount:    int64(sizeVal),
		IsAsk:         isAsk,
		OrderType:     orderType,
		TimeInForce:   tif,
		ReduceOnly:    reduceOnly,
		OrderExpiry:   expiry,
		ClientOrderId: clientOidInt,
	}

	// REST mode: use HTTP client directly
	if a.IsRESTMode() {
		_, err = a.client.PlaceOrder(ctx, req)
		if err != nil {
			return nil, err
		}
		return newSubmittedOrder(params, strconv.FormatInt(clientOidInt, 10), time.Now()), nil
	}

	// WS mode
	if err := a.WsOrderConnected(ctx); err != nil {
		return nil, err
	}
	_, err = a.wsClient.PlaceOrder(ctx, a.client, req)
	if err != nil {
		return nil, err
	}

	return newSubmittedOrder(params, strconv.FormatInt(clientOidInt, 10), time.Now()), nil
}

func (a *Adapter) CancelOrder(ctx context.Context, orderID, symbol string) error {
	a.metaMu.RLock()
	mid, ok := a.symbolToID[a.FormatSymbol(symbol)]
	a.metaMu.RUnlock()
	if !ok {
		return fmt.Errorf("unknown symbol: %s", symbol)
	}

	oid, err := strconv.ParseInt(orderID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid order id: %s", orderID)
	}

	req := lighter.CancelOrderRequest{
		MarketId: mid,
		OrderId:  oid,
	}

	// REST mode
	if a.IsRESTMode() {
		_, err = a.client.CancelOrder(ctx, req)
		return err
	}

	// WS mode
	if err := a.WsOrderConnected(ctx); err != nil {
		return err
	}
	_, err = a.wsClient.CancelOrder(ctx, a.client, req)
	return err
}

func (a *Adapter) ModifyOrder(ctx context.Context, orderID, symbol string, params *exchanges.ModifyOrderParams) (*exchanges.Order, error) {
	if err := a.WsOrderConnected(ctx); err != nil {
		return nil, err
	}
	a.metaMu.RLock()
	mid, ok := a.symbolToID[a.FormatSymbol(symbol)]
	meta, hasMeta := a.marketMeta[mid]
	a.metaMu.RUnlock()
	if !ok || !hasMeta {
		return nil, fmt.Errorf("unknown symbol: %s", symbol)
	}

	oid, err := strconv.ParseInt(orderID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid order id: %s", orderID)
	}

	// Fetch existing order first?
	// Simplified: Assume we have params or fetch.
	// Lighter ModifyOrder accepts new Price/Size.

	req := lighter.ModifyOrderRequest{
		MarketId:   mid,
		OrderIndex: oid,
	}

	if params.Quantity.IsPositive() {
		req.BaseAmount = int64(a.scaleSize(params.Quantity.InexactFloat64(), meta))
	}
	if params.Price.IsPositive() {
		req.Price = uint32(a.scalePrice(params.Price.InexactFloat64(), meta))
	}

	_, err = a.client.ModifyOrder(ctx, req)
	if err != nil {
		return nil, err
	}

	return &exchanges.Order{
		OrderID:   orderID,
		Symbol:    symbol,
		Status:    exchanges.OrderStatusPending,
		Timestamp: time.Now().UnixMilli(),
	}, nil
}

func (a *Adapter) FetchOrderByID(ctx context.Context, orderID, symbol string) (*exchanges.Order, error) {
	_ = ctx
	_ = orderID
	_ = symbol
	return nil, exchanges.ErrNotSupported
}

func (a *Adapter) FetchOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	_ = ctx
	_ = symbol
	return nil, exchanges.ErrNotSupported
}

func (a *Adapter) FetchOpenOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	if err := a.requireWriteAccess(); err != nil {
		return nil, err
	}
	a.metaMu.RLock()
	mid, ok := a.symbolToID[a.FormatSymbol(symbol)]
	a.metaMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown symbol: %s", symbol)
	}

	res, err := a.client.GetAccountActiveOrders(ctx, mid)
	if err != nil {
		return nil, err
	}

	orders := []exchanges.Order{}
	for _, o := range res.Orders {
		orders = append(orders, *a.mapOrder(o))
	}
	return orders, nil
}

func (a *Adapter) CancelAllOrders(ctx context.Context, symbol string) error {
	// REST mode: loop-cancel via FetchOpenOrders + CancelOrder (no SDK batch cancel)
	if a.IsRESTMode() {
		orders, err := a.FetchOpenOrders(ctx, symbol)
		if err != nil {
			return err
		}
		for _, o := range orders {
			if err := a.CancelOrder(ctx, o.OrderID, symbol); err != nil {
				return err
			}
		}
		return nil
	}

	// WS mode
	if err := a.WsOrderConnected(ctx); err != nil {
		return err
	}
	req := lighter.CancelAllOrdersRequest{}
	_, err := a.wsClient.CancelAllOrders(ctx, a.client, req)
	return err
}

func (a *Adapter) SetLeverage(ctx context.Context, symbol string, leverage int) error {
	return exchanges.ErrNotSupported
}

func (a *Adapter) FetchFeeRate(ctx context.Context, symbol string) (*exchanges.FeeRate, error) {
	if err := a.requireWriteAccess(); err != nil {
		return nil, err
	}
	a.feeOnce.Do(func() {
		limits, err := a.client.GetAccountLimits(ctx)
		if err != nil {
			a.cachedFeeErr = fmt.Errorf("fetch account limits: %w", err)
			return
		}
		// 1 tick = 0.0001% = 1e-6
		divisor := decimal.NewFromInt(1_000_000)
		a.cachedFeeRate = &exchanges.FeeRate{
			Maker: decimal.NewFromInt32(limits.CurrentMakerFeeTick).Div(divisor),
			Taker: decimal.NewFromInt32(limits.CurrentTakerFeeTick).Div(divisor),
		}
	})
	if a.cachedFeeErr != nil {
		return nil, a.cachedFeeErr
	}
	return a.cachedFeeRate, nil
}

// ================= Market Data =================

func (a *Adapter) FetchTicker(ctx context.Context, symbol string) (*exchanges.Ticker, error) {
	a.metaMu.RLock()
	mid, ok := a.symbolToID[a.FormatSymbol(symbol)]
	a.metaMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown symbol: %s", symbol)
	}

	res, err := a.client.GetOrderBookDetails(ctx, &mid, nil)
	if err != nil {
		return nil, err
	}
	if len(res.OrderBookDetails) == 0 {
		return nil, fmt.Errorf("no details")
	}

	d := res.OrderBookDetails[0]
	return &exchanges.Ticker{
		Symbol:    symbol,
		LastPrice: decimal.NewFromFloat(d.LastTradePrice),
		Volume24h: decimal.NewFromFloat(d.DailyBaseTokenVolume),
		High24h:   decimal.NewFromFloat(d.DailyPriceHigh),
		Low24h:    decimal.NewFromFloat(d.DailyPriceLow),
		Timestamp: time.Now().UnixMilli(),
	}, nil
}

func (a *Adapter) FetchOrderBook(ctx context.Context, symbol string, limit int) (*exchanges.OrderBook, error) {
	a.metaMu.RLock()
	mid, ok := a.symbolToID[a.FormatSymbol(symbol)]
	a.metaMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown symbol: %s", symbol)
	}

	res, err := a.client.GetOrderBookOrders(ctx, mid, int64(limit))
	if err != nil {
		return nil, err
	}

	ob := &exchanges.OrderBook{
		Symbol:    symbol,
		Timestamp: time.Now().UnixMilli(),
		Bids:      make([]exchanges.Level, 0, len(res.Bids)),
		Asks:      make([]exchanges.Level, 0, len(res.Asks)),
	}
	for _, b := range res.Bids {
		ob.Bids = append(ob.Bids, exchanges.Level{Price: parseString(b.Price), Quantity: parseString(b.RemainingBaseAmount)})
	}
	for _, as := range res.Asks {
		ob.Asks = append(ob.Asks, exchanges.Level{Price: parseString(as.Price), Quantity: parseString(as.RemainingBaseAmount)})
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
	a.metaMu.RLock()
	mid, ok := a.symbolToID[a.FormatSymbol(symbol)]
	a.metaMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown symbol: %s", symbol)
	}

	resolution := "1m"
	switch interval {
	case exchanges.Interval1m:
		resolution = "1m"
	case exchanges.Interval5m:
		resolution = "5m"
	case exchanges.Interval15m:
		resolution = "15m"
	case exchanges.Interval1h:
		resolution = "1h"
	case exchanges.Interval4h:
		resolution = "4h"
	case exchanges.Interval1d:
		resolution = "1d"
	}

	var endTime, startTime int64
	if end != nil {
		endTime = end.Unix()
	} else {
		endTime = time.Now().Unix()
	}
	if start != nil {
		startTime = start.Unix()
	} else {
		startTime = endTime - int64(limit)*intervalToSeconds(interval)
	}

	res, err := a.client.GetCandlesticks(ctx, mid, resolution, startTime, endTime)
	if err != nil {
		return nil, err
	}

	klines := make([]exchanges.Kline, len(res.Candlesticks))
	for i, k := range res.Candlesticks {
		klines[i] = exchanges.Kline{
			Symbol:    symbol,
			Interval:  interval,
			Timestamp: k.Timestamp * 1000,
			Open:      parseLighterFloat(k.Open),
			High:      parseLighterFloat(k.High),
			Low:       parseLighterFloat(k.Low),
			Close:     parseLighterFloat(k.Close),
			Volume:    parseLighterFloat(k.Volume),
		}
	}
	return klines, nil
}

func (a *Adapter) FetchTrades(ctx context.Context, symbol string, limit int) ([]exchanges.Trade, error) {
	a.metaMu.RLock()
	mid, ok := a.symbolToID[a.FormatSymbol(symbol)]
	a.metaMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown symbol: %s", symbol)
	}

	res, err := a.client.GetRecentTrades(ctx, mid, int64(limit))
	if err != nil {
		return nil, err
	}

	trades := make([]exchanges.Trade, len(res.Trades))
	for i, t := range res.Trades {
		side := exchanges.TradeSideSell
		if !t.IsMakerAsk {
			side = exchanges.TradeSideBuy
		}
		trades[i] = exchanges.Trade{
			ID:        fmt.Sprintf("%d", t.TradeId),
			Symbol:    symbol,
			Price:     parseLighterFloat(t.Price),
			Quantity:  parseLighterFloat(t.Size),
			Side:      side,
			Timestamp: t.Timestamp * 1000,
		}
	}
	return trades, nil
}

// ================= WebSocket =================

func (a *Adapter) WatchOrders(ctx context.Context, callback exchanges.OrderUpdateCallback) error {
	if err := a.WsAccountConnected(ctx); err != nil {
		return err
	}
	token, err := a.tokenManager.GetReadToken()
	if err != nil {
		return err
	}
	return a.wsClient.SubscribeAccountAllOrders(a.client.AccountIndex, token, func(msg []byte) {
		var res struct {
			Orders map[string][]*lighter.Order `json:"orders"`
		}
		if err := json.Unmarshal(msg, &res); err != nil {
			return
		}
		for _, orders := range res.Orders {
			for _, o := range orders {
				callback(a.mapOrder(o))
			}
		}
	})
}

func (a *Adapter) WatchPositions(ctx context.Context, callback exchanges.PositionUpdateCallback) error {
	if err := a.WsAccountConnected(ctx); err != nil {
		return err
	}
	token, err := a.tokenManager.GetReadToken()
	if err != nil {
		return err
	}
	return a.wsClient.SubscribeAccountAllPositions(a.client.AccountIndex, token, func(msg []byte) {
		var res struct {
			Positions map[string]*lighter.Position `json:"positions"`
		}
		if err := json.Unmarshal(msg, &res); err != nil {
			return
		}
		for _, p := range res.Positions {
			callback(a.mapPosition(p))
		}
	})
}

func (a *Adapter) WatchTicker(ctx context.Context, symbol string, callback exchanges.TickerCallback) error {
	if err := a.WsMarketConnected(ctx); err != nil {
		return err
	}
	return a.watchTickerWithWS(ctx, a.wsClient, symbol, callback)
}

func (a *Adapter) WatchKlines(ctx context.Context, symbol string, interval exchanges.Interval, callback exchanges.KlineCallback) error {
	return exchanges.ErrNotSupported
}

func (a *Adapter) WatchTrades(ctx context.Context, symbol string, callback exchanges.TradeCallback) error {
	if err := a.WsMarketConnected(ctx); err != nil {
		return err
	}
	return a.watchTradesWithWS(ctx, a.wsClient, symbol, callback)
}

func (a *Adapter) StopWatchOrders(ctx context.Context) error { return nil }
func (a *Adapter) WatchFills(ctx context.Context, callback exchanges.FillCallback) error {
	if err := a.WsAccountConnected(ctx); err != nil {
		return err
	}
	token, err := a.tokenManager.GetReadToken()
	if err != nil {
		return err
	}
	return a.wsClient.SubscribeAccountAllTrades(a.client.AccountIndex, token, func(msg []byte) {
		var event lighter.WsAccountAllTradesEvent
		if err := json.Unmarshal(msg, &event); err != nil {
			return
		}
		for _, trades := range event.Trades {
			for _, trade := range trades {
				fill := mapLighterTradeToFill(trade, a.idToSymbol, a.client.AccountIndex)
				if fill != nil {
					callback(fill)
				}
			}
		}
	})
}
func (a *Adapter) StopWatchFills(ctx context.Context) error {
	_ = ctx
	if !a.hasAccountIndex {
		return nil
	}
	return a.wsClient.Unsubscribe(fmt.Sprintf("account_all_trades/%d", a.client.AccountIndex))
}
func (a *Adapter) StopWatchPositions(ctx context.Context) error { return nil }
func (a *Adapter) StopWatchTicker(ctx context.Context, symbol string) error {
	_ = ctx
	a.metaMu.RLock()
	mid, ok := a.symbolToID[a.FormatSymbol(symbol)]
	a.metaMu.RUnlock()
	if !ok {
		return fmt.Errorf("unknown symbol: %s", symbol)
	}
	if err := a.wsClient.UnsubscribeTicker(mid); err != nil {
		return err
	}
	return a.wsClient.UnsubscribeMarketStats(mid)
}
func (a *Adapter) WatchOrderBook(ctx context.Context, symbol string, depth int, cb exchanges.OrderBookCallback) error {
	if err := a.WsMarketConnected(ctx); err != nil {
		return err
	}

	return a.watchOrderBookWithWS(ctx, a.wsClient, symbol, depth, cb)
}

func (a *Adapter) watchOrderBookWithWS(ctx context.Context, ws lighterOrderBookWS, symbol string, depth int, cb exchanges.OrderBookCallback) error {
	formattedSymbol := a.FormatSymbol(symbol)
	mid, ok := a.symbolToID[formattedSymbol]
	if !ok {
		return fmt.Errorf("unknown symbol: %s", symbol)
	}

	return startLighterOrderBookWatch(ctx, a.BaseAdapter, &a.cancelMu, a.cancels, ws, formattedSymbol, mid, depth, cb)
}

func (a *Adapter) StopWatchOrderBook(ctx context.Context, symbol string) error {
	formattedSymbol := a.FormatSymbol(symbol)
	a.cancelMu.Lock()
	if cancel, ok := a.cancels[formattedSymbol]; ok {
		cancel()
		delete(a.cancels, formattedSymbol)
	}
	a.cancelMu.Unlock()
	a.RemoveLocalOrderBook(formattedSymbol)
	a.metaMu.RLock()
	mid, ok := a.symbolToID[formattedSymbol]
	a.metaMu.RUnlock()
	if ok {
		return a.wsClient.UnsubscribeOrderBook(mid)
	}
	return nil
}
func (a *Adapter) StopWatchKlines(ctx context.Context, symbol string, interval exchanges.Interval) error {
	return exchanges.ErrNotSupported
}
func (a *Adapter) StopWatchTrades(ctx context.Context, symbol string) error {
	a.metaMu.RLock()
	mid, ok := a.symbolToID[a.FormatSymbol(symbol)]
	a.metaMu.RUnlock()
	if !ok {
		return fmt.Errorf("unknown symbol: %s", symbol)
	}
	return a.wsClient.UnsubscribeTrades(mid)
}

func (a *Adapter) FetchSymbolDetails(ctx context.Context, symbol string) (*exchanges.SymbolDetails, error) {
	details, err := a.GetSymbolDetail(a.FormatSymbol(symbol))
	if err != nil {
		return nil, fmt.Errorf("symbol not found: %s", symbol)
	}
	return details, nil
}

// Helpers

func (a *Adapter) mapOrder(o *lighter.Order) *exchanges.Order {
	side := exchanges.OrderSideBuy
	if o.IsAsk {
		side = exchanges.OrderSideSell
	}
	oType := exchanges.OrderTypeLimit
	if o.OrderType == lighter.OrderTypeRespMarket {
		oType = exchanges.OrderTypeMarket
	}

	status := exchanges.OrderStatusUnknown
	switch o.Status {
	case lighter.OrderStatusInProgress, lighter.OrderStatusOpen, lighter.OrderStatusPending:
		status = exchanges.OrderStatusNew // or Open
		if o.Status == lighter.OrderStatusPending || o.Status == lighter.OrderStatusInProgress {
			status = exchanges.OrderStatusPending
		}
	case lighter.OrderStatusFilled:
		status = exchanges.OrderStatusFilled
	case lighter.OrderStatusPartiallyFilled:
		status = exchanges.OrderStatusPartiallyFilled
	case lighter.OrderStatusCanceled, lighter.OrderStatusCanceledPostOnly, lighter.OrderStatusCanceledReduceOnly,
		lighter.OrderStatusCanceledPositionNotAllowed, lighter.OrderStatusCanceledMarginNotAllowed,
		lighter.OrderStatusCanceledTooMuchSlippage, lighter.OrderStatusCanceledNotEnoughLiquidity,
		lighter.OrderStatusCanceledSelfTrade, lighter.OrderStatusCanceledExpired, lighter.OrderStatusCanceledOco,
		lighter.OrderStatusCanceledChild, lighter.OrderStatusCanceledLiquidation,
		lighter.OrderStatusCanceledInvalidBalance:
		// Map all cancel/reject reasons to Cancelled or Rejected
		status = exchanges.OrderStatusCancelled
	case lighter.OrderStatusRejected:
		status = exchanges.OrderStatusRejected
	}

	ord := &exchanges.Order{
		Symbol:         a.idToSymbol[int(o.MarketIndex)],
		OrderID:        o.OrderId,
		ClientOrderID:  o.ClientOrderId,
		Side:           side,
		Type:           oType,
		Price:          parseString(o.Price),
		OrderPrice:     parseString(o.Price),
		Quantity:       parseString(o.InitialBaseAmount),
		FilledQuantity: parseString(o.FilledBaseAmount),
		Status:         status,
		Timestamp:      o.Timestamp,
	}
	return ord
}

func mapLighterTradeToFill(trade lighter.Trade, idToSymbol map[int]string, accountIndex int64) *exchanges.Fill {
	side := exchanges.OrderSideBuy
	orderID := int64(0)
	isMaker := false

	switch {
	case trade.AskAccountId == accountIndex:
		side = exchanges.OrderSideSell
		orderID = trade.AskId
		isMaker = trade.IsMakerAsk
	case trade.BidAccountId == accountIndex:
		side = exchanges.OrderSideBuy
		orderID = trade.BidId
		isMaker = !trade.IsMakerAsk
	default:
		return nil
	}

	fill := &exchanges.Fill{
		TradeID:   fmt.Sprintf("%d", trade.TradeId),
		Symbol:    idToSymbol[trade.MarketId],
		Side:      side,
		Price:     parseLighterFloat(trade.Price),
		Quantity:  parseLighterFloat(trade.Size),
		IsMaker:   isMaker,
		Timestamp: trade.Timestamp,
	}
	if orderID > 0 {
		fill.OrderID = fmt.Sprintf("%d", orderID)
	}
	return fill
}

func (a *Adapter) mapPosition(p *lighter.Position) *exchanges.Position {
	qty := parseString(p.Position)
	side := exchanges.PositionSideLong
	if qty.IsNegative() {
		side = exchanges.PositionSideShort
	}
	marginType := "CROSSED"
	if p.MarginMode == lighter.IsolatedMarginMode {
		marginType = "ISOLATED"
	}
	return &exchanges.Position{
		Symbol:           p.Symbol,
		Side:             side,
		Quantity:         qty,
		EntryPrice:       parseString(p.AvgEntryPrice),
		UnrealizedPnL:    parseString(p.UnrealizedPnl),
		LiquidationPrice: parseString(p.LiquidationPrice),
		MarginType:       marginType,
	}
}

func (a *Adapter) scalePrice(price float64, meta *lighter.OrderBookDetail) float64 {
	pow := float64(1)
	for i := 0; i < int(meta.PriceDecimals); i++ {
		pow *= 10
	}
	return price * pow
}

func (a *Adapter) scaleSize(size float64, meta *lighter.OrderBookDetail) float64 {
	pow := float64(1)
	for i := 0; i < int(meta.SizeDecimals); i++ {
		pow *= 10
	}
	// 使用 Round 避免 float64 精度问题 (0.0006 * 100000 = 59.999... -> 60)
	return math.Round(size * pow)
}

func intervalToSeconds(i exchanges.Interval) int64 {
	switch i {
	case exchanges.Interval1m:
		return 60
	case exchanges.Interval5m:
		return 5 * 60
	case exchanges.Interval15m:
		return 15 * 60
	case exchanges.Interval1h:
		return 60 * 60
	case exchanges.Interval4h:
		return 4 * 60 * 60
	case exchanges.Interval1d:
		return 24 * 60 * 60
	}
	return 60
}

func parseString(s string) decimal.Decimal {
	d, _ := decimal.NewFromString(s)
	return d
}

func parseLighterFloat(s string) decimal.Decimal {
	return parseString(s) // alias
}

func (a *Adapter) WaitOrderBookReady(ctx context.Context, symbol string) error {
	return a.BaseAdapter.WaitOrderBookReady(ctx, a.FormatSymbol(symbol))
}

// GetLocalOrderBook get local orderbook
func (a *Adapter) GetLocalOrderBook(symbol string, depth int) *exchanges.OrderBook {
	ob, ok := a.GetLocalOrderBookImplementation(a.FormatSymbol(symbol))
	if !ok {
		return nil
	}

	lighterOb := ob.(*OrderBook)
	if !lighterOb.initialized {
		return nil
	}

	bids, asks := lighterOb.GetDepth(depth)
	return &exchanges.OrderBook{
		Symbol:    symbol,
		Timestamp: lighterOb.Timestamp(),
		Bids:      bids,
		Asks:      asks,
	}
}

func (a *Adapter) requireAccountAccess() error {
	if !a.hasAccountIndex {
		return exchanges.NewExchangeError("LIGHTER", "", "account_index is required for account access", exchanges.ErrAuthFailed)
	}
	return nil
}

func (a *Adapter) requireReadAccess() error {
	if err := a.requireAccountAccess(); err != nil {
		return err
	}
	if !a.hasReadAccess {
		return exchanges.NewExchangeError("LIGHTER", "", "ro_token or private_key is required for account streams", exchanges.ErrAuthFailed)
	}
	return nil
}

func (a *Adapter) requireWriteAccess() error {
	if err := a.requireAccountAccess(); err != nil {
		return err
	}
	if !a.hasWriteAccess {
		return exchanges.NewExchangeError("LIGHTER", "", "private_key is required for trading operations", exchanges.ErrAuthFailed)
	}
	return nil
}
