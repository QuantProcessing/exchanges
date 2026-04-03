package aster

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/aster/sdk/perp"

	"github.com/shopspring/decimal"
)

// Adapter Aster 适配器
type Adapter struct {
	*exchanges.BaseAdapter
	client        *perp.Client
	wsMarket      *perp.WsMarketClient
	wsAccount     *perp.WsAccountClient
	apiKey        string
	secretKey     string
	quoteCurrency string // "USDT" or "USDC"

	// OrderBook management cancellations
	cancelMu sync.Mutex
	cancels  map[string]context.CancelFunc

	// Cached fee rates (per-symbol)
	feeCache sync.Map // symbol -> *exchanges.FeeRate

	privateOrderStreamsOnce sync.Once
	privateOrderStreams     *privateOrderStreams[perp.OrderUpdateEvent]
}

// NewAdapter 创建 Aster 适配器
func NewAdapter(ctx context.Context, opts Options) (*Adapter, error) {
	quote, err := opts.quoteCurrency()
	if err != nil {
		return nil, err
	}
	if err := opts.validateCredentials(); err != nil {
		return nil, err
	}

	client := perp.NewClient().WithCredentials(opts.APIKey, opts.SecretKey)
	wsMarket := perp.NewWsMarketClient(ctx)
	wsAccount := perp.NewWsAccountClient(ctx, opts.APIKey, opts.SecretKey)

	base := exchanges.NewBaseAdapter("ASTER", exchanges.MarketTypePerp, opts.logger())

	a := &Adapter{
		BaseAdapter:   base,
		client:        client,
		wsMarket:      wsMarket,
		wsAccount:     wsAccount,
		apiKey:        opts.APIKey,
		secretKey:     opts.SecretKey,
		quoteCurrency: string(quote),
		cancels:       make(map[string]context.CancelFunc),
	}

	if err := a.RefreshSymbolDetails(context.Background()); err != nil {
		return nil, fmt.Errorf("aster init: %w", err)
	}

	// TODO: logger.Info("Initialized Aster Adapter")

	return a, nil
}

func (a *Adapter) WsAccountConnected(ctx context.Context) error {
	if err := a.requirePrivateAccess(); err != nil {
		return err
	}
	if a.wsAccount.Conn == nil {
		if err := a.wsAccount.Connect(); err != nil {
			return err
		}
	}

	return nil
}

func (a *Adapter) WsMarketConnected(ctx context.Context) error {
	if a.wsMarket.Conn == nil {
		if err := a.wsMarket.Connect(); err != nil {
			return err
		}
	}

	return nil
}

// Aster does not support WS private order placement in this adapter.
func (a *Adapter) WsOrderConnected(ctx context.Context) error {
	return nil
}

func (a *Adapter) Close() error {
	a.wsMarket.Close()
	a.wsAccount.Close()
	return nil
}

// ================= Account & Trading =================

func (a *Adapter) FetchAccount(ctx context.Context) (_ *exchanges.Account, retErr error) {
	if err := a.requirePrivateAccess(); err != nil {
		return nil, err
	}
	res, err := a.client.GetAccount(ctx)
	if err != nil {
		return nil, fmt.Errorf("aster get account failed: %w", err)
	}

	account := &exchanges.Account{
		Positions: []exchanges.Position{},
		Orders:    []exchanges.Order{},
	}

	var availBalance decimal.Decimal
	for _, asset := range res.Assets {
		if asset.Asset == a.quoteCurrency {
			availBalance = parseDecimal(asset.AvailableBalance)
			break
		}
	}

	totalWallet := parseDecimal(res.TotalWalletBalance)
	totalUnrealized := parseDecimal(res.TotalUnrealizedProfit)

	account.TotalBalance = totalWallet.Add(totalUnrealized)
	account.UnrealizedPnL = totalUnrealized
	account.AvailableBalance = availBalance

	for _, p := range res.Positions {
		amt := parseDecimal(p.PositionAmt)
		if amt.IsZero() {
			continue
		}

		entryPrice := parseDecimal(p.EntryPrice)
		unrealizedPnL := parseDecimal(p.UnrealizedProfit)
		leverage := parseDecimal(p.Leverage)
		maintMargin := parseDecimal(p.MaintMargin)

		side := exchanges.PositionSideLong
		if amt.IsNegative() {
			side = exchanges.PositionSideShort
		}

		account.Positions = append(account.Positions, exchanges.Position{
			Symbol:            a.ExtractSymbol(p.Symbol),
			Side:              side,
			Quantity:          amt,
			EntryPrice:        entryPrice,
			UnrealizedPnL:     unrealizedPnL,
			Leverage:          leverage,
			MaintenanceMargin: maintMargin,
			MarginType:        boolToMarginType(p.Isolated),
		})
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

func (a *Adapter) PlaceOrder(ctx context.Context, params *exchanges.OrderParams) (_ *exchanges.Order, retErr error) {
	if err := a.requirePrivateAccess(); err != nil {
		return nil, err
	}
	// Apply slippage logic: converts MARKET+Slippage to LIMIT+IOC
	if err := a.BaseAdapter.ApplySlippage(ctx, params, a.FetchTicker); err != nil {
		return nil, err
	}
	// 1. Validation & Formatting
	details, err := a.FetchSymbolDetails(ctx, params.Symbol)
	if err != nil {
		return nil, err
	}
	if err := exchanges.ValidateAndFormatParams(params, details); err != nil {
		return nil, err
	}

	// Map Order Type
	orderType, err := a.mapOrderType(params.Type)
	if err != nil {
		return nil, err
	}

	p := perp.PlaceOrderParams{
		Symbol:   a.FormatSymbol(params.Symbol),
		Side:     string(params.Side),
		Type:     orderType,
		Quantity: params.Quantity.String(),
	}

	if params.Price.IsPositive() {
		p.Price = params.Price.String()
	}

	// Map TimeInForce (market orders don't accept it)
	if tif := a.mapTimeInForce(params); tif != "" {
		p.TimeInForce = tif
	}

	if params.ClientID != "" {
		p.NewClientOrderID = params.ClientID
	}
	if params.ReduceOnly {
		p.ReduceOnly = true
	}

	resp, err := a.client.PlaceOrder(ctx, p)
	if err != nil {
		return nil, err
	}

	return a.normalizeOrderResponse(resp)
}

func (a *Adapter) PlaceOrderWS(context.Context, *exchanges.OrderParams) error {
	return exchanges.ErrNotSupported
}

func (a *Adapter) mapOrderType(t exchanges.OrderType) (perp.OrderType, error) {
	switch t {
	case exchanges.OrderTypeLimit:
		return perp.OrderType_LIMIT, nil
	case exchanges.OrderTypeMarket:
		return perp.OrderType_MARKET, nil
	case exchanges.OrderTypeStopLossLimit:
		return perp.OrderType_STOP_LIMIT, nil
	case exchanges.OrderTypeStopLossMarket:
		return perp.OrderType_STOP_MARKET, nil
	case exchanges.OrderTypeTakeProfitLimit:
		return perp.OrderType_TAKE_PROFIT_LIMIT, nil
	case exchanges.OrderTypeTakeProfitMarket:
		return perp.OrderType_TAKE_PROFIT_MARKET, nil
	case exchanges.OrderTypePostOnly:
		return perp.OrderType_LIMIT, nil // PostOnly is Limit with specialized TIF
	default:
		return "", fmt.Errorf("unsupported order type: %s", t)
	}
}

func (a *Adapter) mapTimeInForce(params *exchanges.OrderParams) perp.TimeInForce {
	// Market orders must NOT send timeInForce
	if params.Type == exchanges.OrderTypeMarket {
		return ""
	}

	if params.Type == exchanges.OrderTypePostOnly {
		return perp.TimeInForce_GTX
	}

	if params.TimeInForce != "" {
		switch params.TimeInForce {
		case exchanges.TimeInForceGTC:
			return perp.TimeInForce_GTC
		case exchanges.TimeInForceIOC:
			return perp.TimeInForce_IOC
		case exchanges.TimeInForceFOK:
			return perp.TimeInForce_FOK
		case exchanges.TimeInForcePO:
			return perp.TimeInForce_GTX
		default:
			return perp.TimeInForce_GTC
		}
	}

	return perp.TimeInForce_GTC
}

func (a *Adapter) CancelOrder(ctx context.Context, orderID, symbol string) (retErr error) {
	if err := a.requirePrivateAccess(); err != nil {
		return err
	}
	formattedSymbol := a.FormatSymbol(symbol)
	p := perp.CancelOrderParams{
		Symbol:  formattedSymbol,
		OrderID: orderID,
	}

	_, err := a.client.CancelOrder(ctx, p)
	return err
}

func (a *Adapter) CancelOrderWS(context.Context, string, string) error {
	return exchanges.ErrNotSupported
}

func (a *Adapter) ModifyOrder(ctx context.Context, orderID, symbol string, params *exchanges.ModifyOrderParams) (_ *exchanges.Order, retErr error) {
	if err := a.requirePrivateAccess(); err != nil {
		return nil, err
	}
	formattedSymbol := a.FormatSymbol(symbol)
	oid, _ := strconv.ParseInt(orderID, 10, 64)
	p := perp.ModifyOrderParams{
		Symbol:   formattedSymbol,
		OrderID:  oid,
		Quantity: params.Quantity.String(),
		Price:    params.Price.String(),
	}

	resp, err := a.client.ModifyOrder(ctx, p)
	if err != nil {
		return nil, err
	}

	return a.normalizeOrderResponse(resp)
}

func (a *Adapter) ModifyOrderWS(context.Context, string, string, *exchanges.ModifyOrderParams) error {
	return exchanges.ErrNotSupported
}

func (a *Adapter) FetchOrderByID(ctx context.Context, orderID, symbol string) (_ *exchanges.Order, retErr error) {
	if err := a.requirePrivateAccess(); err != nil {
		return nil, err
	}
	formattedSymbol := a.FormatSymbol(symbol)
	oid, err := strconv.ParseInt(orderID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid order id: %w", err)
	}

	res, err := a.client.GetOrder(ctx, formattedSymbol, oid, "")
	if err != nil {
		if isAsterOrderLookupMiss(err) {
			return nil, exchanges.ErrOrderNotFound
		}
		return nil, err
	}

	return a.normalizeOrderResponse(res)
}

func (a *Adapter) FetchOrders(ctx context.Context, symbol string) (_ []exchanges.Order, retErr error) {
	return nil, exchanges.ErrNotSupported
}

func (a *Adapter) FetchOpenOrders(ctx context.Context, symbol string) (_ []exchanges.Order, retErr error) {
	if err := a.requirePrivateAccess(); err != nil {
		return nil, err
	}
	formattedSymbol := a.FormatSymbol(symbol)
	res, err := a.client.GetOpenOrders(ctx, formattedSymbol)
	if err != nil {
		return nil, err
	}

	orders := make([]exchanges.Order, 0, len(res))
	for _, r := range res {
		o, err := a.normalizeOrderResponse(&r)
		if err != nil {
			continue
		}
		orders = append(orders, *o)
	}

	return orders, nil
}

func isAsterOrderLookupMiss(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "order") &&
		(strings.Contains(msg, "does not exist") ||
			strings.Contains(msg, "unknown order") ||
			strings.Contains(msg, "not found"))
}

func (a *Adapter) CancelAllOrders(ctx context.Context, symbol string) (retErr error) {
	if err := a.requirePrivateAccess(); err != nil {
		return err
	}
	formattedSymbol := a.FormatSymbol(symbol)
	p := perp.CancelAllOrdersParams{
		Symbol: formattedSymbol,
	}
	return a.client.CancelAllOpenOrders(ctx, p)
}

func (a *Adapter) SetLeverage(ctx context.Context, symbol string, leverage int) (retErr error) {
	if err := a.requirePrivateAccess(); err != nil {
		return err
	}
	formattedSymbol := a.FormatSymbol(symbol)
	_, err := a.client.ChangeLeverage(ctx, formattedSymbol, leverage)
	return err
}

func (a *Adapter) FetchFeeRate(ctx context.Context, symbol string) (_ *exchanges.FeeRate, retErr error) {
	if err := a.requirePrivateAccess(); err != nil {
		return nil, err
	}
	if v, ok := a.feeCache.Load(symbol); ok {
		return v.(*exchanges.FeeRate), nil
	}
	formattedSymbol := a.FormatSymbol(symbol)
	feeRate, err := a.client.GetFeeRate(ctx, formattedSymbol)
	if err != nil {
		return nil, err
	}
	res := &exchanges.FeeRate{
		Maker: parseDecimal(feeRate.MakerCommissionRate),
		Taker: parseDecimal(feeRate.TakerCommissionRate),
	}
	a.feeCache.Store(symbol, res)
	return res, nil
}

// ================= Market Data =================

func (a *Adapter) FetchTicker(ctx context.Context, symbol string) (_ *exchanges.Ticker, retErr error) {
	formattedSymbol := a.FormatSymbol(symbol)
	t, err := a.client.Ticker(ctx, formattedSymbol)
	if err != nil {
		return nil, err
	}

	depth, err := a.client.Depth(ctx, strings.ToUpper(formattedSymbol), 5)
	if err != nil {
		return nil, err
	}

	ticker := &exchanges.Ticker{
		Symbol:    symbol,
		LastPrice: parseDecimal(t.LastPrice),
		High24h:   parseDecimal(t.HighPrice),
		Low24h:    parseDecimal(t.LowPrice),
		Volume24h: parseDecimal(t.Volume),
		QuoteVol:  parseDecimal(t.QuoteVolume),
		Timestamp: t.CloseTime,
	}

	if len(depth.Bids) > 0 {
		ticker.Bid = parseDecimal(depth.Bids[0][0])
	}
	if len(depth.Asks) > 0 {
		ticker.Ask = parseDecimal(depth.Asks[0][0])
	}

	return ticker, nil
}

func (a *Adapter) FetchOrderBook(ctx context.Context, symbol string, limit int) (_ *exchanges.OrderBook, retErr error) {
	formattedSymbol := a.FormatSymbol(symbol)
	res, err := a.client.Depth(ctx, strings.ToUpper(formattedSymbol), limit)
	if err != nil {
		return nil, err
	}

	ob := &exchanges.OrderBook{
		Symbol:    symbol,
		Timestamp: res.T,
		Bids:      make([]exchanges.Level, 0, len(res.Bids)),
		Asks:      make([]exchanges.Level, 0, len(res.Asks)),
	}

	for _, item := range res.Bids {
		ob.Bids = append(ob.Bids, exchanges.Level{Price: parseDecimal(item[0]), Quantity: parseDecimal(item[1])})
	}
	for _, item := range res.Asks {
		ob.Asks = append(ob.Asks, exchanges.Level{Price: parseDecimal(item[0]), Quantity: parseDecimal(item[1])})
	}

	return ob, nil
}

func (a *Adapter) FetchKlines(ctx context.Context, symbol string, interval exchanges.Interval, opts *exchanges.KlineOpts) (_ []exchanges.Kline, retErr error) {
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
	formattedSymbol := a.FormatSymbol(symbol)
	var startTime, endTime int64
	if start != nil {
		startTime = start.UnixMilli()
	}
	if end != nil {
		endTime = end.UnixMilli()
	}
	res, err := a.client.Klines(ctx, formattedSymbol, string(interval), limit, startTime, endTime)
	if err != nil {
		return nil, err
	}

	klines := make([]exchanges.Kline, 0, len(res))
	for _, item := range res {
		row := item
		if len(row) < 8 {
			continue
		}

		k := exchanges.Kline{
			Symbol:    symbol,
			Interval:  interval,
			Timestamp: parseInt64(row[0]),
			Open:      parseDecimalInterface(row[1]),
			High:      parseDecimalInterface(row[2]),
			Low:       parseDecimalInterface(row[3]),
			Close:     parseDecimalInterface(row[4]),
			Volume:    parseDecimalInterface(row[5]),
			QuoteVol:  parseDecimalInterface(row[7]),
		}
		klines = append(klines, k)
	}

	return klines, nil
}

func (a *Adapter) FetchTrades(ctx context.Context, symbol string, limit int) (_ []exchanges.Trade, retErr error) {
	formattedSymbol := a.FormatSymbol(symbol)
	res, err := a.client.GetAggTrades(ctx, formattedSymbol, limit)
	if err != nil {
		return nil, err
	}

	trades := make([]exchanges.Trade, 0, len(res))
	for _, r := range res {
		side := exchanges.TradeSideBuy
		if r.IsBuyerMaker {
			side = exchanges.TradeSideSell
		}

		trades = append(trades, exchanges.Trade{
			ID:        fmt.Sprintf("%d", r.ID),
			Symbol:    symbol,
			Price:     parseDecimal(r.Price),
			Quantity:  parseDecimal(r.Quantity),
			Side:      side,
			Timestamp: r.Timestamp,
		})
	}
	return trades, nil
}

func (a *Adapter) FetchSymbolDetails(ctx context.Context, symbol string) (*exchanges.SymbolDetails, error) {
	// Normalize symbol to match cache key (Base Symbol)
	normalized := a.ExtractSymbol(symbol)
	return a.GetSymbolDetail(normalized)
}

// ================= WebSocket =================

func (a *Adapter) WatchOrders(ctx context.Context, callback exchanges.OrderUpdateCallback) error {
	return a.privateStreams().watchOrders(func() error {
		return a.WsAccountConnected(ctx)
	}, callback)
}

func (a *Adapter) WatchPositions(ctx context.Context, callback exchanges.PositionUpdateCallback) error {
	if err := a.WsAccountConnected(ctx); err != nil {
		return err
	}

	a.wsAccount.SubscribeAccountUpdate(func(e *perp.AccountUpdateEvent) {
		for _, p := range e.UpdateData.Positions {
			pos := a.normalizePositionUpdate(p)
			callback(pos)
		}
	})
	return nil
}

func (a *Adapter) WatchTicker(ctx context.Context, symbol string, callback exchanges.TickerCallback) error {
	formattedSymbol := a.FormatSymbol(symbol)
	if err := a.WsMarketConnected(ctx); err != nil {
		return err
	}
	return a.wsMarket.SubscribeBookTicker(formattedSymbol, func(e *perp.WsBookTickerEvent) error {
		t := &exchanges.Ticker{
			Symbol:    symbol,
			Bid:       parseDecimal(e.BestBidPrice),
			Ask:       parseDecimal(e.BestAskPrice),
			Timestamp: time.Now().UnixMilli(),
		}
		if e.EventTime > 0 {
			t.Timestamp = e.EventTime
		}
		callback(t)
		return nil
	})
}

// WatchOrderBook subscribes to orderbook updates, maintains a local copy,
// and optionally calls the callback on each update. Pass nil callback for pull-only mode.
// This method blocks until the initial snapshot is synced.
func (a *Adapter) WatchOrderBook(ctx context.Context, symbol string, depth int, cb exchanges.OrderBookCallback) error {
	if err := a.subscribeOrderBookInternal(ctx, symbol, depth, cb); err != nil {
		return err
	}
	formattedSymbol := a.FormatSymbol(symbol)
	return a.BaseAdapter.WaitOrderBookReady(ctx, formattedSymbol)
}

// subscribeOrderBookInternal 订阅订单薄的内部实现
// depth 为 nil 时表示不需要推送回调
// callback 为 nil 时表示不触发回调（WC 模式）
func (a *Adapter) subscribeOrderBookInternal(ctx context.Context, symbol string, depth int, callback exchanges.OrderBookCallback) error {
	formattedSymbol := a.FormatSymbol(symbol)
	if err := a.WsMarketConnected(ctx); err != nil {
		return err
	}

	a.cancelMu.Lock()
	if a.cancels == nil {
		a.cancels = make(map[string]context.CancelFunc)
	}

	// 如果已经存在订阅，先取消
	if cancel, ok := a.cancels[formattedSymbol]; ok {
		cancel()
	}

	// 创建新的 OrderBook 实例
	ob := NewOrderBook(formattedSymbol)
	a.SetLocalOrderBook(formattedSymbol, ob)

	ctxCancel, cancel := context.WithCancel(context.Background())
	a.cancels[formattedSymbol] = cancel
	a.cancelMu.Unlock()

	// 用于触发快照同步的通道
	// 大小设为 1，避免阻塞
	snapshotTrigger := make(chan struct{}, 1)

	// 启动快照同步协程
	go func() {
		retryDelay := time.Second // exponential backoff: 1s → 2s → 4s → ... → 30s cap
		const maxDelay = 30 * time.Second

		for {
			select {
			case <-ctxCancel.Done():
				return
			case <-snapshotTrigger:
				// 如果已经初始化完成了，就不再请求快照
				if ob.IsInitialized() {
					retryDelay = time.Second
					continue
				}

				// TODO: logger.Info("Fetching orderbook snapshot", "symbol", symbol)
				// 获取1000档深度快照
				// limit=1000 compliant with explicit instruction "Step 3"
				snapshotDepth, err := a.client.Depth(ctxCancel, strings.ToUpper(formattedSymbol), 1000)
				if err != nil {
					// TODO: logger.Error("Failed to fetch snapshot", "symbol", symbol, "error", err, "retryIn", retryDelay)
					// 失败后指数退避重试
					select {
					case <-time.After(retryDelay):
						if retryDelay < maxDelay {
							retryDelay *= 2
							if retryDelay > maxDelay {
								retryDelay = maxDelay
							}
						}
						select {
						case snapshotTrigger <- struct{}{}:
						default:
						}
					case <-ctxCancel.Done():
						return
					}
					continue
				}

				// 应用快照
				// 将深度快照中的内容更新到本地orderbook副本中
				if err := ob.ApplySnapshot(snapshotDepth); err != nil {
					// TODO: logger.Warn("Failed to apply snapshot", "symbol", symbol, "error", err, "retryIn", retryDelay)
					// 重新触发
					select {
					case <-time.After(retryDelay):
						if retryDelay < maxDelay {
							retryDelay *= 2
							if retryDelay > maxDelay {
								retryDelay = maxDelay
							}
						}
						select {
						case snapshotTrigger <- struct{}{}:
						default:
						}
					case <-ctxCancel.Done():
						return
					}
				} else {
					retryDelay = time.Second
					// TODO: logger.Info("Orderbook initialized with snapshot", "symbol", symbol, "lastUpdateId", snapshotDepth.LastUpdateID)
				}
			}
		}
	}()

	// 订阅 WS 增量更新 (100ms)
	err := a.wsMarket.SubscribeIncrementOrderBook(formattedSymbol, "100ms", func(e *perp.WsDepthEvent) error {
		// 检查上下文是否已取消
		select {
		case <-ctxCancel.Done():
			return nil
		default:
		}

		// 处理更新
		err := ob.ProcessUpdate(e)
		if err != nil {
			// 只有初始化后出现的错误（如丢包 gap）才需要警告
			// 未初始化时的 buffer 行为在 ProcessUpdate 内部处理
			if ob.IsInitialized() {
				// TODO: logger.Warn("Orderbook sync detected gap/error", "symbol", symbol, "error", err)
			}

			// 触发重新同步/初始化
			select {
			case snapshotTrigger <- struct{}{}:
			default:
			}
			return nil
		}

		// 如果尚未初始化（还在 buffer 阶段），则不推送回调
		if !ob.IsInitialized() {
			// 再次确保触发快照（针对刚订阅时的状态）
			// 虽然 go routine 刚启动时触发了，但为了稳健性
			select {
			case snapshotTrigger <- struct{}{}:
			default:
			}
			return nil
		}

		// 如果提供了 callback，则推送数据
		if callback != nil {
			bids, asks := ob.GetDepth(depth)

			res := &exchanges.OrderBook{
				Symbol:    symbol,
				Timestamp: e.EventTime,
				Bids:      bids,
				Asks:      asks,
			}

			callback(res)
		}

		return nil
	})

	if err != nil {
		cancel() // 订阅失败，清理资源
		a.cancelMu.Lock()
		a.RemoveLocalOrderBook(formattedSymbol)
		delete(a.cancels, formattedSymbol)
		a.cancelMu.Unlock()
		return err
	}

	return nil
}

func (a *Adapter) WatchKlines(ctx context.Context, symbol string, interval exchanges.Interval, callback exchanges.KlineCallback) error {
	formattedSymbol := a.FormatSymbol(symbol)
	if err := a.WsMarketConnected(ctx); err != nil {
		return err
	}
	return a.wsMarket.SubscribeKline(formattedSymbol, string(interval), func(e *perp.WsKlineEvent) error {
		k := &exchanges.Kline{
			Symbol:    symbol,
			Interval:  exchanges.Interval(e.Kline.Interval),
			Timestamp: e.Kline.StartTime,
			Open:      parseDecimal(e.Kline.OpenPrice),
			High:      parseDecimal(e.Kline.HighPrice),
			Low:       parseDecimal(e.Kline.LowPrice),
			Close:     parseDecimal(e.Kline.ClosePrice),
			Volume:    parseDecimal(e.Kline.Volume),
			QuoteVol:  parseDecimal(e.Kline.QuoteVolume),
		}
		callback(k)
		return nil
	})
}

func (a *Adapter) WatchTrades(ctx context.Context, symbol string, callback exchanges.TradeCallback) error {
	formattedSymbol := a.FormatSymbol(symbol)
	if err := a.WsMarketConnected(ctx); err != nil {
		return err
	}
	return a.wsMarket.SubscribeAggTrade(formattedSymbol, func(e *perp.WsAggTradeEvent) error {
		side := exchanges.TradeSideBuy
		if e.IsBuyerMaker {
			side = exchanges.TradeSideSell
		}
		t := &exchanges.Trade{
			ID:        fmt.Sprintf("%d", e.AggTradeID),
			Symbol:    symbol,
			Price:     parseDecimal(e.Price),
			Quantity:  parseDecimal(e.Quantity),
			Side:      side,
			Timestamp: e.EventTime,
		}
		callback(t)
		return nil
	})
}

func (a *Adapter) StopWatchOrders(ctx context.Context) error {
	_ = ctx
	if a.privateOrderStreams != nil {
		a.privateOrderStreams.stopOrders()
	}
	return nil
}
func (a *Adapter) WatchFills(ctx context.Context, callback exchanges.FillCallback) error {
	return a.privateStreams().watchFills(func() error {
		return a.WsAccountConnected(ctx)
	}, callback)
}
func (a *Adapter) StopWatchFills(ctx context.Context) error {
	_ = ctx
	if a.privateOrderStreams != nil {
		a.privateOrderStreams.stopFills()
	}
	return nil
}
func (a *Adapter) StopWatchPositions(ctx context.Context) error { return nil }
func (a *Adapter) StopWatchTicker(ctx context.Context, symbol string) error {
	formattedSymbol := a.FormatSymbol(symbol)
	return a.wsMarket.UnsubscribeBookTicker(formattedSymbol)
}
func (a *Adapter) StopWatchOrderBook(ctx context.Context, symbol string) error {
	formattedSymbol := a.FormatSymbol(symbol)

	a.cancelMu.Lock()
	if cancel, ok := a.cancels[formattedSymbol]; ok {
		cancel()
		delete(a.cancels, formattedSymbol)
	}
	a.RemoveLocalOrderBook(formattedSymbol)
	a.cancelMu.Unlock()

	// Assuming depth 20, 100ms like others
	return a.wsMarket.UnsubscribeIncrementOrderBook(formattedSymbol, "100ms")
}
func (a *Adapter) StopWatchKlines(ctx context.Context, symbol string, interval exchanges.Interval) error {
	formattedSymbol := a.FormatSymbol(symbol)
	return a.wsMarket.UnsubscribeKline(formattedSymbol, string(interval))
}
func (a *Adapter) StopWatchTrades(ctx context.Context, symbol string) error {
	formattedSymbol := a.FormatSymbol(symbol)
	return a.wsMarket.UnsubscribeAggTrade(formattedSymbol)
}

// ================= Internal Methods =================

func (a *Adapter) RefreshSymbolDetails(ctx context.Context) error {
	info, err := a.client.ExchangeInfo(ctx)
	if err != nil {
		return err
	}

	symbols := make(map[string]*exchanges.SymbolDetails)

	for _, s := range info.Symbols {
		details := &exchanges.SymbolDetails{
			Symbol:            s.Symbol,
			PricePrecision:    int32(s.PricePrecision),
			QuantityPrecision: int32(s.QuantityPrecision),
		}

		for _, filter := range s.Filters {
			ftype, ok := filter["filterType"].(string)
			if !ok {
				continue
			}

			switch ftype {
			case "LOT_SIZE":
				if minQtyStr, ok := filter["minQty"].(string); ok {
					details.MinQuantity = parseDecimal(minQtyStr)
				}
			case "MIN_NOTIONAL":
				if notionalStr, ok := filter["notional"].(string); ok {
					details.MinNotional = parseDecimal(notionalStr)
				}
			case "PRICE_FILTER":
				if tickSizeStr, ok := filter["tickSize"].(string); ok {
					tickSize := parseDecimal(tickSizeStr)
					if tickSize.IsPositive() {
						precision := exchanges.CountDecimalPlaces(tickSizeStr)
						details.PricePrecision = precision
					}
				}
			}
		}
		symbols[strings.TrimSuffix(s.Symbol, a.quoteCurrency)] = details
	}

	a.SetSymbolDetails(symbols)

	return nil
}

func (a *Adapter) normalizeOrderResponse(resp *perp.OrderResponse) (*exchanges.Order, error) {
	price := parseDecimal(resp.Price)
	qty := parseDecimal(resp.OrigQty)
	filled := parseDecimal(resp.ExecutedQty)

	status := exchanges.OrderStatusPending
	switch resp.Status {
	case "NEW":
		status = exchanges.OrderStatusNew
	case "FILLED":
		status = exchanges.OrderStatusFilled
	case "PARTIALLY_FILLED":
		status = exchanges.OrderStatusPartiallyFilled
	case "CANCELED":
		status = exchanges.OrderStatusCancelled
	case "REJECTED":
		status = exchanges.OrderStatusRejected
	}

	side := exchanges.OrderSideBuy
	if resp.Side == "SELL" {
		side = exchanges.OrderSideSell
	}

	return &exchanges.Order{
		OrderID:        fmt.Sprintf("%d", resp.OrderID),
		Symbol:         a.ExtractSymbol(resp.Symbol),
		Side:           side,
		Type:           exchanges.OrderType(resp.Type),
		Quantity:       qty,
		Price:          price,
		Status:         status,
		FilledQuantity: filled,
		Timestamp:      resp.UpdateTime,
		ClientOrderID:  resp.ClientOrderID,
	}, nil
}

func (a *Adapter) normalizeOrderUpdate(e *perp.OrderUpdateEvent) *exchanges.Order {
	price := parseDecimal(e.Order.OriginalPrice)
	qty := parseDecimal(e.Order.OriginalQty)
	filled := parseDecimal(e.Order.AccumulatedFilledQty)

	status := exchanges.OrderStatusPending
	switch e.Order.OrderStatus {
	case "NEW":
		status = exchanges.OrderStatusNew
	case "FILLED":
		status = exchanges.OrderStatusFilled
	case "PARTIALLY_FILLED":
		status = exchanges.OrderStatusPartiallyFilled
	case "CANCELED":
		status = exchanges.OrderStatusCancelled
	case "REJECTED":
		status = exchanges.OrderStatusRejected
	case "EXPIRED":
		status = exchanges.OrderStatusCancelled
	}

	side := exchanges.OrderSideBuy
	if e.Order.Side == "SELL" {
		side = exchanges.OrderSideSell
	}

	return &exchanges.Order{
		OrderID:        fmt.Sprintf("%d", e.Order.OrderID),
		Symbol:         a.ExtractSymbol(e.Order.Symbol),
		Side:           side,
		Type:           exchanges.OrderType(e.Order.OrderType),
		Quantity:       qty,
		Price:          price,
		OrderPrice:     price,
		Status:         status,
		FilledQuantity: filled,
		Timestamp:      e.Order.TradeTime,
		ClientOrderID:  e.Order.ClientOrderID,
	}
}

func (a *Adapter) privateStreams() *privateOrderStreams[perp.OrderUpdateEvent] {
	a.privateOrderStreamsOnce.Do(func() {
		a.privateOrderStreams = newPrivateOrderStreams(
			func(handler func(*perp.OrderUpdateEvent)) {
				a.wsAccount.SubscribeOrderUpdate(handler)
			},
			a.normalizeOrderUpdate,
			a.mapOrderFill,
		)
	})
	return a.privateOrderStreams
}

func (a *Adapter) mapOrderFill(e *perp.OrderUpdateEvent) *exchanges.Fill {
	if e == nil || e.Order.ExecutionType != "TRADE" {
		return nil
	}

	qty := parseDecimal(e.Order.LastFilledQty)
	if qty.IsZero() {
		return nil
	}

	side := exchanges.OrderSideBuy
	if e.Order.Side == "SELL" {
		side = exchanges.OrderSideSell
	}

	tradeID := ""
	if e.Order.TradeID > 0 {
		tradeID = fmt.Sprintf("%d", e.Order.TradeID)
	}

	ts := e.Order.TradeTime
	if ts == 0 {
		ts = e.TransactionTime
	}
	if ts == 0 {
		ts = e.EventTime
	}

	return &exchanges.Fill{
		TradeID:       tradeID,
		OrderID:       fmt.Sprintf("%d", e.Order.OrderID),
		ClientOrderID: e.Order.ClientOrderID,
		Symbol:        a.ExtractSymbol(e.Order.Symbol),
		Side:          side,
		Price:         parseDecimal(e.Order.LastFilledPrice),
		Quantity:      qty,
		Fee:           parseDecimal(e.Order.Commission),
		FeeAsset:      e.Order.CommissionAsset,
		IsMaker:       e.Order.IsMaker,
		Timestamp:     ts,
	}
}

func (a *Adapter) normalizePositionUpdate(p struct {
	Symbol              string `json:"s"`
	PositionAmount      string `json:"pa"`
	EntryPrice          string `json:"ep"`
	AccumulatedRealized string `json:"cr"`
	UnrealizedPnL       string `json:"up"`
	MarginType          string `json:"mt"`
	IsolatedWallet      string `json:"iw"`
	PositionSide        string `json:"ps"`
}) *exchanges.Position {
	amt := parseDecimal(p.PositionAmount)
	entry := parseDecimal(p.EntryPrice)
	unPnL := parseDecimal(p.UnrealizedPnL)

	side := exchanges.PositionSideLong
	if amt.IsNegative() {
		side = exchanges.PositionSideShort
	}

	return &exchanges.Position{
		Symbol:        a.ExtractSymbol(p.Symbol),
		Side:          side,
		Quantity:      amt,
		EntryPrice:    entry,
		UnrealizedPnL: unPnL,
		MarginType:    p.MarginType,
	}
}

// Sorting helpers

func (a *Adapter) FormatSymbol(symbol string) string {
	s := strings.ToUpper(symbol)
	q := strings.ToUpper(a.quoteCurrency)
	if !strings.HasSuffix(s, q) {
		s += q
	}
	return strings.ToLower(s)
}

func (a *Adapter) ExtractSymbol(symbol string) string {
	return strings.ToUpper(strings.TrimSuffix(strings.ToUpper(symbol), strings.ToUpper(a.quoteCurrency)))
}

// GetLocalOrderBook get local orderbook
func (a *Adapter) GetLocalOrderBook(symbol string, depth int) *exchanges.OrderBook {
	formattedSymbol := a.FormatSymbol(symbol)

	ob, ok := a.GetLocalOrderBookImplementation(formattedSymbol)
	if !ok {
		return nil
	}

	perpOb := ob.(*OrderBook)
	if !perpOb.IsInitialized() {
		return nil
	}

	bids, asks := perpOb.GetDepth(depth)
	return &exchanges.OrderBook{
		Symbol:    symbol,
		Timestamp: perpOb.Timestamp(),
		Bids:      bids,
		Asks:      asks,
	}
}

func (a *Adapter) requirePrivateAccess() error {
	if a.apiKey == "" || a.secretKey == "" {
		return exchanges.NewExchangeError("ASTER", "", "private API not available (no credentials configured)", exchanges.ErrAuthFailed)
	}
	return nil
}
