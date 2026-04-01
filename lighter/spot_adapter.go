package lighter

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/lighter/sdk"

	"github.com/shopspring/decimal"
)

// SpotAdapter Lighter 现货适配器
type SpotAdapter struct {
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

// NewSpotAdapter 创建 Lighter 现货适配器
func NewSpotAdapter(ctx context.Context, opts Options) (*SpotAdapter, error) {
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
			return nil, fmt.Errorf("lighter spot init: %w", err)
		}
		client.AccountIndex = accIndex
	}
	if opts.KeyIndex != "" {
		ki, err := strconv.ParseUint(opts.KeyIndex, 10, 8)
		if err != nil {
			return nil, fmt.Errorf("lighter spot init: %w", err)
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

	base := exchanges.NewBaseAdapter("LIGHTER", exchanges.MarketTypeSpot, opts.logger())
	// Lighter spot currently routes private order placement and cancellation through WS only.

	a := &SpotAdapter{
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
			return nil, fmt.Errorf("lighter spot init: %w", err)
		}
	}

	// Init metadata
	if err := a.RefreshSymbolDetails(context.Background()); err != nil {
		return nil, fmt.Errorf("lighter spot init: %w", err)
	}

	// TODO: logger.Info("Initialized Lighter Spot Adapter")
	return a, nil
}

func (a *SpotAdapter) WithCredentials(apiKey, secretKey string) exchanges.Exchange {
	// Lighter uses private key, not API key/secret pattern
	// TODO: logger.Warn("WithCredentials not supported for Lighter, use NewSpotAdapter with config")
	return a
}

func (a *SpotAdapter) Close() error {
	a.wsClient.Close()
	return nil
}

func (a *SpotAdapter) WsAccountConnected(ctx context.Context) error {
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

func (a *SpotAdapter) WsMarketConnected(ctx context.Context) error {
	if a.wsClient.Conn == nil {
		if err := a.wsClient.Connect(); err != nil {
			return err
		}
	}
	return nil
}

func (a *SpotAdapter) WsOrderConnected(ctx context.Context) error {
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

// refreshMetaInternal loads spot markets from spot_order_book_details
func (a *SpotAdapter) refreshMetaInternal(ctx context.Context) error {
	res, err := a.client.GetOrderBookDetails(ctx, nil, nil)
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

	for _, detail := range res.SpotOrderBookDetails {
		mid := int(detail.MarketId)
		// Use base symbol (e.g. "ETH" from "ETH/USDC")
		sym := a.ExtractSymbol(detail.Symbol)
		a.symbolToID[sym] = mid
		a.idToSymbol[mid] = sym
		a.marketMeta[mid] = detail

		details := &exchanges.SymbolDetails{
			Symbol:            sym,
			PricePrecision:    int32(detail.PriceDecimals),
			QuantityPrecision: int32(detail.SizeDecimals),
			MinQuantity:       parseString(detail.MinBaseAmount),
		}
		symbols[sym] = details
	}
	a.SetSymbolDetails(symbols)
	// TODO: logger.Info("Loaded Lighter spot markets", zap.Int("count", len(a.idToSymbol)))
	return nil
}

func (a *SpotAdapter) IsConnected(ctx context.Context) (bool, error) {
	// Simple check
	_, err := a.client.GetL1Metadata(ctx)
	return err == nil, err
}

func (a *SpotAdapter) RefreshSymbolDetails(ctx context.Context) error {
	a.metaMu.Lock()
	defer a.metaMu.Unlock()
	return a.refreshMetaInternal(ctx)
}

func (a *SpotAdapter) FormatSymbol(symbol string) string {
	return strings.ToUpper(symbol)
}

func (a *SpotAdapter) ExtractSymbol(symbol string) string {
	symbol = strings.ToUpper(symbol)
	if idx := strings.Index(symbol, "/"); idx > 0 {
		return symbol[:idx]
	}
	return symbol
}

// ================= Account & Trading =================

func (a *SpotAdapter) FetchAccount(ctx context.Context) (*exchanges.Account, error) {
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
		Orders: []exchanges.Order{},
	}

	// Map spot token holdings from account assets as positions
	// Non-quote assets (e.g. ETH, BTC) with positive balance = Long position
	var positions []exchanges.Position
	for _, asset := range accData.Assets {
		// Skip quote currencies
		sym := strings.ToUpper(asset.Symbol)
		if sym == "USDC" || sym == "USDT" || sym == "USDE" {
			continue
		}
		qty := parseString(asset.Balance)
		if qty.IsZero() {
			continue
		}
		// Only include assets that have a matching spot market
		a.metaMu.RLock()
		_, hasMarket := a.symbolToID[sym]
		a.metaMu.RUnlock()
		if !hasMarket {
			continue
		}
		positions = append(positions, exchanges.Position{
			Symbol:   sym,
			Side:     exchanges.PositionSideLong,
			Quantity: qty,
		})
	}
	account.Positions = positions

	account.AvailableBalance = parseString(accData.AvailableBalance)
	account.TotalBalance = parseString(accData.TotalAssetValue)

	// Get active orders for spot markets
	a.metaMu.RLock()
	marketIds := make([]int, 0, len(a.symbolToID))
	for _, mid := range a.symbolToID {
		marketIds = append(marketIds, mid)
	}
	a.metaMu.RUnlock()

	// Active-order queries require write credentials because the Lighter endpoint
	// depends on a signed auth token rather than the read-only token path.
	// Read-only accounts therefore return balances and holdings here, but leave
	// Account.Orders empty by design.
	if a.hasWriteAccess {
		for _, marketId := range marketIds {
			orderRes, err := a.client.GetAccountActiveOrders(ctx, marketId)
			if err != nil {
				continue // Skip on error
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

func (a *SpotAdapter) FetchBalance(ctx context.Context) (decimal.Decimal, error) {
	acc, err := a.FetchAccount(ctx)
	if err != nil {
		return decimal.Zero, err
	}
	return acc.TotalBalance, nil
}

func (a *SpotAdapter) FetchFeeRate(ctx context.Context, symbol string) (*exchanges.FeeRate, error) {
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

// PlaceOrder places a spot order using market_index
func (a *SpotAdapter) PlaceOrder(ctx context.Context, params *exchanges.OrderParams) (*exchanges.Order, error) {
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
	// Spot doesn't use reduceOnly typically, but we'll keep the parameter

	// Set Expiry
	expiry := int64(0)
	if tif != lighter.OrderTimeInForceImmediateOrCancel {
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

	_, err = a.wsClient.PlaceOrder(ctx, a.client, req)
	if err != nil {
		return nil, err
	}

	return &exchanges.Order{
		Symbol:        params.Symbol,
		Side:          params.Side,
		Type:          params.Type,
		Quantity:      params.Quantity,
		Price:         params.Price,
		Status:        exchanges.OrderStatusPending,
		Timestamp:     time.Now().UnixMilli(),
		OrderID:       strconv.FormatInt(clientOidInt, 10),
		ClientOrderID: strconv.FormatInt(clientOidInt, 10),
	}, nil
}

func (a *SpotAdapter) CancelOrder(ctx context.Context, orderID, symbol string) error {
	if err := a.WsOrderConnected(ctx); err != nil {
		return err
	}
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
	_, err = a.wsClient.CancelOrder(ctx, a.client, req)
	return err
}

func (a *SpotAdapter) ModifyOrder(ctx context.Context, orderID, symbol string, params *exchanges.ModifyOrderParams) (*exchanges.Order, error) {
	// Fetch existing order to get side
	existing, err := a.fetchActiveOrderByID(ctx, orderID, symbol)
	if err != nil {
		return nil, err
	}

	if err := a.CancelOrder(ctx, orderID, symbol); err != nil {
		return nil, err
	}

	orderParams := &exchanges.OrderParams{
		Symbol:   symbol,
		Side:     existing.Side,
		Type:     exchanges.OrderTypeLimit,
		Quantity: params.Quantity,
		Price:    params.Price,
	}
	return a.PlaceOrder(ctx, orderParams)
}

func (a *SpotAdapter) fetchActiveOrderByID(ctx context.Context, orderID, symbol string) (*exchanges.Order, error) {
	if err := a.requireWriteAccess(); err != nil {
		return nil, err
	}
	a.metaMu.RLock()
	mid, ok := a.symbolToID[a.FormatSymbol(symbol)]
	a.metaMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown symbol: %s", symbol)
	}

	oid, err := strconv.ParseInt(orderID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid order id: %s", orderID)
	}

	res, err := a.client.GetAccountActiveOrders(ctx, mid)
	if err != nil {
		return nil, err
	}
	for _, o := range res.Orders {
		if o.OrderIndex == oid {
			return a.mapOrder(o), nil
		}
	}
	return nil, exchanges.ErrOrderNotFound
}

func (a *SpotAdapter) FetchOrderByID(ctx context.Context, orderID, symbol string) (*exchanges.Order, error) {
	_ = ctx
	_ = orderID
	_ = symbol
	return nil, exchanges.ErrNotSupported
}

func (a *SpotAdapter) FetchOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	_ = ctx
	_ = symbol
	return nil, exchanges.ErrNotSupported
}

func (a *SpotAdapter) mapOrder(o *lighter.Order) *exchanges.Order {
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
	case lighter.OrderStatusOpen, lighter.OrderStatusPending:
		status = exchanges.OrderStatusNew
		if o.Status == lighter.OrderStatusPending {
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
		lighter.OrderStatusCanceledChild, lighter.OrderStatusCanceledLiquidation:
		status = exchanges.OrderStatusCancelled
	case lighter.OrderStatusRejected:
		status = exchanges.OrderStatusRejected
	}

	a.metaMu.RLock()
	symbol := a.idToSymbol[o.MarketIndex]
	a.metaMu.RUnlock()

	return &exchanges.Order{
		OrderID:        o.OrderId,
		ClientOrderID:  o.ClientOrderId,
		Symbol:         symbol,
		Side:           side,
		Type:           oType,
		Quantity:       parseString(o.InitialBaseAmount),
		FilledQuantity: parseString(o.FilledBaseAmount),
		Price:          parseString(o.Price),
		Status:         status,
		Timestamp:      o.Timestamp,
	}
}

func (a *SpotAdapter) FetchOpenOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
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

	orders := make([]exchanges.Order, 0, len(res.Orders))
	for _, o := range res.Orders {
		if o == nil {
			continue
		}
		orders = append(orders, exchanges.Order{
			OrderID:        o.OrderId,
			ClientOrderID:  o.ClientOrderId,
			Symbol:         symbol,
			Side:           exchanges.OrderSide(o.Side),
			Type:           exchanges.OrderType(o.OrderType),
			Quantity:       parseString(o.InitialBaseAmount),
			FilledQuantity: parseString(o.FilledBaseAmount),
			Price:          parseString(o.Price),
			Status:         exchanges.OrderStatus(o.Status),
			TimeInForce:    exchanges.TimeInForce(o.TimeInForce),
		})
	}
	return orders, nil
}

func (a *SpotAdapter) CancelAllOrders(ctx context.Context, symbol string) error {
	orders, err := a.FetchOpenOrders(ctx, symbol)
	if err != nil {
		return err
	}

	for _, order := range orders {
		if err := a.CancelOrder(ctx, order.OrderID, symbol); err != nil {
			// TODO: logger.Warn("Failed to cancel order", "orderID", order.OrderID, "error", err)
		}
	}
	return nil
}

// ================= Market Data =================

func (a *SpotAdapter) FetchTicker(ctx context.Context, symbol string) (*exchanges.Ticker, error) {
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
	// Spot markets are in SpotOrderBookDetails
	details := res.SpotOrderBookDetails
	if len(details) == 0 {
		details = res.OrderBookDetails // fallback
	}
	if len(details) == 0 {
		return nil, fmt.Errorf("no details")
	}

	d := details[0]
	return &exchanges.Ticker{
		Symbol:    symbol,
		LastPrice: decimal.NewFromFloat(d.LastTradePrice),
		Volume24h: decimal.NewFromFloat(d.DailyBaseTokenVolume),
		High24h:   decimal.NewFromFloat(d.DailyPriceHigh),
		Low24h:    decimal.NewFromFloat(d.DailyPriceLow),
		Timestamp: time.Now().UnixMilli(),
	}, nil
}

func (a *SpotAdapter) FetchOrderBook(ctx context.Context, symbol string, limit int) (*exchanges.OrderBook, error) {
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
	return nil, exchanges.ErrNotSupported
}

func (a *SpotAdapter) FetchTrades(ctx context.Context, symbol string, limit int) ([]exchanges.Trade, error) {
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

func (a *SpotAdapter) FetchSymbolDetails(ctx context.Context, symbol string) (*exchanges.SymbolDetails, error) {
	details, err := a.GetSymbolDetail(a.FormatSymbol(symbol))
	if err != nil {
		return nil, fmt.Errorf("symbol not found: %s", symbol)
	}
	return details, nil
}

// ================= WebSocket =================

func (a *SpotAdapter) WatchOrders(ctx context.Context, callback exchanges.OrderUpdateCallback) error {
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

func (a *SpotAdapter) WatchTicker(ctx context.Context, symbol string, callback exchanges.TickerCallback) error {
	return exchanges.ErrNotSupported
}

// WatchOrderBook subscribes to orderbook updates and waits for the book to be ready.
func (a *SpotAdapter) WatchOrderBook(ctx context.Context, symbol string, depth int, callback exchanges.OrderBookCallback) error {
	if err := a.WsMarketConnected(ctx); err != nil {
		return err
	}
	formattedSymbol := a.FormatSymbol(symbol)

	a.cancelMu.Lock()
	if a.cancels == nil {
		a.cancels = make(map[string]context.CancelFunc)
	}
	if cancel, ok := a.cancels[formattedSymbol]; ok {
		cancel()
	}

	ob := NewOrderBook(formattedSymbol)
	a.SetLocalOrderBook(formattedSymbol, ob)

	_, cancel := context.WithCancel(context.Background())
	a.cancels[formattedSymbol] = cancel
	a.cancelMu.Unlock()

	mid, ok := a.symbolToID[formattedSymbol]
	if !ok {
		return fmt.Errorf("unknown symbol: %s", symbol)
	}

	err := a.wsClient.SubscribeOrderBook(mid, func(msg []byte) {
		ob.ProcessUpdate(msg)
		if callback != nil {
			callback(ob.ToAdapterOrderBook(depth))
		}
	})
	if err != nil {
		return err
	}
	return a.BaseAdapter.WaitOrderBookReady(ctx, formattedSymbol)
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
	return exchanges.ErrNotSupported
}

func (a *SpotAdapter) StopWatchKlines(ctx context.Context, symbol string, interval exchanges.Interval) error {
	return exchanges.ErrNotSupported
}

func (a *SpotAdapter) StopWatchTrades(ctx context.Context, symbol string) error {
	return exchanges.ErrNotSupported
}

func (a *SpotAdapter) WaitOrderBookReady(ctx context.Context, symbol string) error {
	return a.BaseAdapter.WaitOrderBookReady(ctx, a.FormatSymbol(symbol))
}

func (a *SpotAdapter) GetLocalOrderBook(symbol string, depth int) *exchanges.OrderBook {
	ob, ok := a.GetLocalOrderBookImplementation(a.FormatSymbol(symbol))
	if !ok {
		return nil
	}

	lighterOb := ob.(*OrderBook)
	if !lighterOb.initialized {
		return nil
	}

	return lighterOb.ToAdapterOrderBook(depth)
}

// ================= Helper Functions =================

// scalePrice converts human price to scaled uint32
func (a *SpotAdapter) scalePrice(price float64, meta *lighter.OrderBookDetail) int64 {
	priceDecimals := int(meta.PriceDecimals)
	multiplier := 1.0
	for i := 0; i < priceDecimals; i++ {
		multiplier *= 10
	}
	return int64(price * multiplier)
}

// unscalePrice converts scaled uint32 back to human price
func (a *SpotAdapter) unscalePrice(scaledPrice uint32, meta *lighter.OrderBookDetail) float64 {
	priceDecimals := int(meta.PriceDecimals)
	divisor := 1.0
	for i := 0; i < priceDecimals; i++ {
		divisor *= 10
	}
	return float64(scaledPrice) / divisor
}

// scaleSize converts human size to scaled int64
func (a *SpotAdapter) scaleSize(size float64, meta *lighter.OrderBookDetail) int64 {
	sizeDecimals := int(meta.SizeDecimals)
	multiplier := 1.0
	for i := 0; i < sizeDecimals; i++ {
		multiplier *= 10
	}
	return int64(size * multiplier)
}

// unscaleSize converts scaled int64 back to human size
func (a *SpotAdapter) unscaleSize(scaledSize int64, meta *lighter.OrderBookDetail) float64 {
	sizeDecimals := int(meta.SizeDecimals)
	divisor := 1.0
	for i := 0; i < sizeDecimals; i++ {
		divisor *= 10
	}
	return float64(scaledSize) / divisor
}

func (a *SpotAdapter) WatchPositions(ctx context.Context, cb exchanges.PositionUpdateCallback) error {
	return exchanges.ErrNotSupported
}

func (a *SpotAdapter) StopWatchPositions(ctx context.Context) error {
	return exchanges.ErrNotSupported
}

func (a *SpotAdapter) StopWatchOrderBook(ctx context.Context, symbol string) error {
	formattedSymbol := a.FormatSymbol(symbol)
	a.cancelMu.Lock()
	if cancel, ok := a.cancels[formattedSymbol]; ok {
		cancel()
		delete(a.cancels, formattedSymbol)
	}
	a.cancelMu.Unlock()
	a.RemoveLocalOrderBook(formattedSymbol)
	return nil
}

func (a *SpotAdapter) requireAccountAccess() error {
	if !a.hasAccountIndex {
		return exchanges.NewExchangeError("LIGHTER", "", "account_index is required for account access", exchanges.ErrAuthFailed)
	}
	return nil
}

func (a *SpotAdapter) requireReadAccess() error {
	if err := a.requireAccountAccess(); err != nil {
		return err
	}
	if !a.hasReadAccess {
		return exchanges.NewExchangeError("LIGHTER", "", "ro_token or private_key is required for account streams", exchanges.ErrAuthFailed)
	}
	return nil
}

func (a *SpotAdapter) requireWriteAccess() error {
	if err := a.requireAccountAccess(); err != nil {
		return err
	}
	if !a.hasWriteAccess {
		return exchanges.NewExchangeError("LIGHTER", "", "private_key is required for trading operations", exchanges.ErrAuthFailed)
	}
	return nil
}
