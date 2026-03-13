package binance

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"


	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/binance/sdk/spot"

	"github.com/shopspring/decimal"
)

// SpotAdapter implements exchanges.Exchange for Binance Spot markets
type SpotAdapter struct {
	*exchanges.BaseAdapter
	client        *spot.Client
	wsMarket      *spot.WsMarketClient
	wsAccount     *spot.WsAccountClient
	wsAPI         *spot.WsAPIClient

	apiKey        string
	secretKey     string
	quoteCurrency string // "USDT" or "USDC"

	// OrderBook management cancellations
	cancelMu sync.Mutex
	cancels  map[string]context.CancelFunc
}

// NewSpotAdapter creates a new Binance Spot adapter instance
func NewSpotAdapter(ctx context.Context, opts Options) (*SpotAdapter, error) {
	quote, err := opts.quoteCurrency()
	if err != nil {
		return nil, err
	}

	client := spot.NewClient().WithCredentials(opts.APIKey, opts.SecretKey)
	wsMarket := spot.NewWsMarketClient(ctx)
	wsAPI := spot.NewWsAPIClient(ctx)
	wsAccount := spot.NewWsAccountClient(wsAPI, opts.APIKey, opts.SecretKey)

	a := &SpotAdapter{
		BaseAdapter:   exchanges.NewBaseAdapter("BINANCE", exchanges.MarketTypeSpot, opts.logger()),
		client:        client,
		wsMarket:      wsMarket,
		wsAccount:     wsAccount,
		wsAPI:         wsAPI,
		apiKey:        opts.APIKey,
		secretKey:     opts.SecretKey,
		quoteCurrency: string(quote),
		cancels:       make(map[string]context.CancelFunc),
	}

	// Initialize metadata
	if err := a.RefreshSymbolDetails(context.Background()); err != nil {
		return nil, fmt.Errorf("binance spot init: %w", err)
	}
	// TODO: logger.Info("Initialized Binance Spot Adapter")

	return a, nil
}

// ================= Core Interface Methods =================

func (a *SpotAdapter) WithCredentials(apiKey, secretKey string) exchanges.Exchange {
	a.apiKey = apiKey
	a.secretKey = secretKey
	a.client = spot.NewClient().WithCredentials(apiKey, secretKey)
	return a
}

func (a *SpotAdapter) Close() error {
	if a.wsMarket != nil {
		a.wsMarket.Close()
	}
	if a.wsAccount != nil {
		a.wsAccount.Close()
	}
	if a.wsAPI != nil {
		a.wsAPI.Close()
	}
	return nil
}

// ================= Account Management =================

func (a *SpotAdapter) FetchAccount(ctx context.Context) (*exchanges.Account, error) {
	resp, err := a.client.GetAccount(ctx)
	if err != nil {
		return nil, err
	}

	balances := make([]exchanges.SpotBalance, 0, len(resp.Balances))
	totalEquity := decimal.Zero

	for _, b := range resp.Balances {
		free := parseDecimal(b.Free)
		locked := parseDecimal(b.Locked)
		total := free.Add(locked)

		// Only include non-zero balances
		if total.IsZero() {
			continue
		}

		balances = append(balances, exchanges.SpotBalance{
			Asset:  b.Asset,
			Free:   free,
			Locked: locked,
			Total:  total,
		})

		// Simple quote currency equity calculation (would need price conversion for accuracy)
		if b.Asset == a.quoteCurrency {
			totalEquity = totalEquity.Add(total)
		}
	}

	return &exchanges.Account{
		TotalBalance:     totalEquity,
		AvailableBalance: totalEquity,
	}, nil
}

func (a *SpotAdapter) FetchBalance(ctx context.Context) (decimal.Decimal, error) {
	account, err := a.FetchAccount(ctx)
	if err != nil {
		return decimal.Zero, err
	}
	return account.AvailableBalance, nil
}

func (a *SpotAdapter) FetchFeeRate(ctx context.Context, symbol string) (*exchanges.FeeRate, error) {
	// Binance spot doesn't have a dedicated fee rate endpoint typically
	// Default spot fees are usually 0.1% maker/taker
	// Can be retrieved from account commission rates
	return &exchanges.FeeRate{
		Maker: decimal.NewFromFloat(0.001), // 0.1%
		Taker: decimal.NewFromFloat(0.001), // 0.1%
	}, nil
}

// ================= Order Management =================

func (a *SpotAdapter) PlaceOrder(ctx context.Context, params *exchanges.OrderParams) (*exchanges.Order, error) {
	// Apply slippage logic: converts MARKET+Slippage to LIMIT+IOC
	if err := a.BaseAdapter.ApplySlippage(ctx, params, a.FetchTicker); err != nil {
		return nil, err
	}
	if err := a.WsOrderConnected(ctx); err != nil {
		return nil, err
	}

	formattedSymbol := strings.ToUpper(a.FormatSymbol(params.Symbol))

	side := "BUY"
	if params.Side == exchanges.OrderSideSell {
		side = "SELL"
	}

	orderType := strings.ToUpper(string(params.Type))

	p := spot.PlaceOrderParams{
		Symbol:           formattedSymbol,
		Side:             side,
		Type:             orderType,
		Quantity:         params.Quantity.String(),
		NewClientOrderID: params.ClientID,
	}

	// Add price for limit orders
	if params.Type == exchanges.OrderTypeLimit || params.Type == exchanges.OrderTypePostOnly {
		p.Price = params.Price.String()
		p.TimeInForce = "GTC" // Good Till Cancel

		// PostOnly needs special handling
		if params.Type == exchanges.OrderTypePostOnly {
			p.TimeInForce = "GTC"
			// Consider changing type to LIMIT_MAKER if needed, but keeping existing logic for now
		}
	}

	reqID := fmt.Sprintf("order-%d", time.Now().UnixNano())
	resp, err := a.wsAPI.PlaceOrderWS(a.apiKey, a.secretKey, p, reqID)
	if err != nil {
		return nil, err
	}

	return a.normalizeOrderResponse(resp)
}

func (a *SpotAdapter) CancelOrder(ctx context.Context, orderID, symbol string) error {
	if err := a.WsOrderConnected(ctx); err != nil {
		return err
	}

	formattedSymbol := strings.ToUpper(a.FormatSymbol(symbol))
	oid, err := strconv.ParseInt(orderID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid order id: %w", err)
	}

	reqID := fmt.Sprintf("cancel-%d", time.Now().UnixNano())
	_, err = a.wsAPI.CancelOrderWS(a.apiKey, a.secretKey, formattedSymbol, oid, "", reqID)
	return err
}

func (a *SpotAdapter) ModifyOrder(ctx context.Context, orderID, symbol string, params *exchanges.ModifyOrderParams) (*exchanges.Order, error) {
	if err := a.WsOrderConnected(ctx); err != nil {
		return nil, err
	}

	// Binance spot supports cancel-replace
	formattedSymbol := a.FormatSymbol(symbol)
	oid, err := strconv.ParseInt(orderID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid order id: %w", err)
	}

	// Get existing order to determine side
	existingOrder, err := a.FetchOrder(ctx, orderID, symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get existing order: %w", err)
	}

	side := "BUY"
	if existingOrder.Side == exchanges.OrderSideSell {
		side = "SELL"
	}

	p := spot.CancelReplaceOrderParams{
		Symbol:            formattedSymbol,
		Side:              side,
		Type:              "LIMIT",
		CancelReplaceMode: "STOP_ON_FAILURE",
		TimeInForce:       "GTC",
		Quantity:          params.Quantity.String(),
		Price:             params.Price.String(),
		CancelOrderID:     oid,
	}

	reqID := fmt.Sprintf("modify-%d", time.Now().UnixNano())
	resp, err := a.wsAPI.ModifyOrderWS(a.apiKey, a.secretKey, p, reqID)
	if err != nil {
		return nil, err
	}

	if resp.NewOrderResponse == nil {
		return nil, fmt.Errorf("modify order failed: %s", resp.NewOrderResult)
	}

	return a.normalizeOrderResponse(resp.NewOrderResponse)
}

func (a *SpotAdapter) FetchOrder(ctx context.Context, orderID, symbol string) (*exchanges.Order, error) {
	formattedSymbol := strings.ToUpper(a.FormatSymbol(symbol))
	oid, err := strconv.ParseInt(orderID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid order id: %w", err)
	}

	resp, err := a.client.GetOrder(ctx, formattedSymbol, oid, "")
	if err != nil {
		return nil, err
	}

	return a.normalizeOrderResponse(resp)
}

func (a *SpotAdapter) FetchOpenOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	formattedSymbol := strings.ToUpper(a.FormatSymbol(symbol))
	resp, err := a.client.GetOpenOrders(ctx, formattedSymbol)
	if err != nil {
		return nil, err
	}

	orders := make([]exchanges.Order, 0, len(resp))
	for _, r := range resp {
		o, err := a.normalizeOrderResponse(&r)
		if err != nil {
			continue
		}
		orders = append(orders, *o)
	}

	return orders, nil
}

func (a *SpotAdapter) CancelAllOrders(ctx context.Context, symbol string) error {
	// Binance spot doesn't have a single cancel-all endpoint
	// We need to get all open orders and cancel them individually
	orders, err := a.FetchOpenOrders(ctx, symbol)
	if err != nil {
		return err
	}

	for _, order := range orders {
		if err := a.CancelOrder(ctx, order.OrderID, symbol); err != nil {
			a.Logger.Warnw("Failed to cancel order", "orderID", order.OrderID, "error", err)
		}
	}

	return nil
}

// ================= Spot-Specific =================

func (a *SpotAdapter) FetchSpotBalances(ctx context.Context) ([]exchanges.SpotBalance, error) {
	resp, err := a.client.GetAccount(ctx)
	if err != nil {
		return nil, err
	}

	balances := make([]exchanges.SpotBalance, 0, len(resp.Balances))
	for _, b := range resp.Balances {
		free := parseDecimal(b.Free)
		locked := parseDecimal(b.Locked)
		total := free.Add(locked)

		if total.IsZero() {
			continue
		}

		balances = append(balances, exchanges.SpotBalance{
			Asset:  b.Asset,
			Free:   free,
			Locked: locked,
			Total:  total,
		})
	}
	return balances, nil
}

func (a *SpotAdapter) TransferAsset(ctx context.Context, params *exchanges.TransferParams) error {
	return fmt.Errorf("TransferAsset not supported by Binance Spot Adapter yet (requires Universal Transfer endpoint)")
}

// ================= Market Data =================

func (a *SpotAdapter) FetchTicker(ctx context.Context, symbol string) (*exchanges.Ticker, error) {
	// Try 24hr Ticker for volume/high/low
	formattedSymbol := strings.ToUpper(a.FormatSymbol(symbol))
	bookTicker, err := a.client.BookTicker(ctx, formattedSymbol)
	if err != nil {
		return nil, err
	}

	// Try 24hr Ticker for volume/high/low
	ticker, err := a.client.Ticker(ctx, formattedSymbol)
	// If 24hr ticker fails, we just use book ticker data where possible

	res := &exchanges.Ticker{
		Symbol:    symbol,
		Bid:       parseDecimal(bookTicker.BidPrice),
		Ask:       parseDecimal(bookTicker.AskPrice),
		Timestamp: time.Now().UnixMilli(),
	}

	if err == nil && ticker != nil {
		res.LastPrice = parseDecimal(ticker.LastPrice)
		res.High24h = parseDecimal(ticker.HighPrice)
		res.Low24h = parseDecimal(ticker.LowPrice)
		res.Volume24h = parseDecimal(ticker.Volume)
		res.QuoteVol = parseDecimal(ticker.QuoteVolume)
		res.Timestamp = ticker.CloseTime
	} else {
		// Fallback for LastPrice
		res.LastPrice = res.Bid.Add(res.Ask).Div(decimal.NewFromInt(2))
	}

	return res, nil
}

func (a *SpotAdapter) FetchOrderBook(ctx context.Context, symbol string, limit int) (*exchanges.OrderBook, error) {
	formattedSymbol := strings.ToUpper(a.FormatSymbol(symbol))
	res, err := a.client.Depth(ctx, formattedSymbol, limit)
	if err != nil {
		return nil, err
	}

	ob := &exchanges.OrderBook{
		Symbol:    symbol,
		Timestamp: time.Now().UnixMilli(), // DepthResponse doesn't always have T?
		Bids:      make([]exchanges.Level, 0, len(res.Bids)),
		Asks:      make([]exchanges.Level, 0, len(res.Asks)),
	}

	for _, item := range res.Bids {
		if len(item) >= 2 {
			ob.Bids = append(ob.Bids, exchanges.Level{Price: parseDecimal(item[0]), Quantity: parseDecimal(item[1])})
		}
	}
	for _, item := range res.Asks {
		if len(item) >= 2 {
			ob.Asks = append(ob.Asks, exchanges.Level{Price: parseDecimal(item[0]), Quantity: parseDecimal(item[1])})
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
	formattedSymbol := strings.ToUpper(a.FormatSymbol(symbol))
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
	for _, row := range res {
		// row is KlineResponse which is []interface{}
		// [openTime, open, high, low, close, volume, closeTime, quoteAssetVolume, numberOfTrades, takerBuyBaseAssetVolume, takerBuyQuoteAssetVolume, ignore]
		rowSlice := []interface{}(row)
		if len(rowSlice) < 6 {
			continue
		}

		k := exchanges.Kline{
			Symbol:    symbol,
			Interval:  interval,
			Timestamp: parseInt64(rowSlice[0]),
			Open:      parseDecimalInterface(rowSlice[1]),
			High:      parseDecimalInterface(rowSlice[2]),
			Low:       parseDecimalInterface(rowSlice[3]),
			Close:     parseDecimalInterface(rowSlice[4]),
			Volume:    parseDecimalInterface(rowSlice[5]),
		}
		if len(rowSlice) > 7 {
			k.QuoteVol = parseDecimalInterface(rowSlice[7])
		}
		klines = append(klines, k)
	}
	return klines, nil
}

func (a *SpotAdapter) FetchTrades(ctx context.Context, symbol string, limit int) ([]exchanges.Trade, error) {
	resp, err := a.client.MyTrades(ctx, strings.ToUpper(a.FormatSymbol(symbol)), limit, 0, 0, 0)
	if err != nil {
		return nil, err
	}

	trades := make([]exchanges.Trade, 0, len(resp))
	for _, t := range resp {
		side := exchanges.TradeSideBuy
		if !t.IsBuyer {
			side = exchanges.TradeSideSell
		}

		trades = append(trades, exchanges.Trade{
			ID:        fmt.Sprintf("%d", t.ID),
			Symbol:    a.ExtractSymbol(t.Symbol),
			Price:     parseDecimal(t.Price),
			Quantity:  parseDecimal(t.Qty),
			Side:      side,
			Timestamp: t.Time,
		})
	}

	return trades, nil
}

func (a *SpotAdapter) FetchSymbolDetails(ctx context.Context, symbol string) (*exchanges.SymbolDetails, error) {
	details, err := a.GetSymbolDetail(symbol)
	if err != nil {
		return nil, fmt.Errorf("symbol not found in cache: %s", symbol)
	}
	return details, nil
}

// ================= WebSocket Methods =================

func (a *SpotAdapter) WsAccountConnected(ctx context.Context) error {
	if a.wsAccount.IsConnected() {
		return nil
	}
	return a.wsAccount.Connect()
}

func (a *SpotAdapter) WsMarketConnected(ctx context.Context) error {
	if a.wsMarket.IsConnected() {
		return nil
	}
	return a.wsMarket.Connect()
}

func (a *SpotAdapter) WsOrderConnected(ctx context.Context) error {
	if a.wsAPI.IsConnected() {
		return nil
	}
	return a.wsAPI.Connect()
}

func (a *SpotAdapter) WatchOrders(ctx context.Context, callback exchanges.OrderUpdateCallback) error {
	if err := a.WsAccountConnected(ctx); err != nil {
		return err
	}

	a.wsAccount.SubscribeExecutionReport(func(report *spot.ExecutionReportEvent) {
		status := exchanges.OrderStatusPending
		switch report.OrderStatus {
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
		if report.Side == "SELL" {
			side = exchanges.OrderSideSell
		}

		order := &exchanges.Order{
			OrderID:        fmt.Sprintf("%d", report.OrderID),
			Symbol:         a.ExtractSymbol(report.Symbol),
			Side:           side,
			Type:           exchanges.OrderType(report.OrderType),
			Quantity:       parseDecimal(report.Quantity),
			Price:          parseDecimal(report.Price),
			Status:         status,
			FilledQuantity: parseDecimal(report.CumulativeFilledQuantity),
			Timestamp:      report.TransactionTime,
			ClientOrderID:  report.ClientOrderID,
		}
		callback(order)
	})
	return nil
}

func (a *SpotAdapter) WatchTicker(ctx context.Context, symbol string, callback exchanges.TickerCallback) error {
	if err := a.WsMarketConnected(ctx); err != nil {
		return err
	}

	handler := func(e *spot.BookTickerEvent) error {
		ticker := &exchanges.Ticker{
			Symbol:    symbol,
			Bid:       parseDecimal(e.BestBidPrice),
			Ask:       parseDecimal(e.BestAskPrice),
			Timestamp: time.Now().UnixMilli(),
		}
		callback(ticker)
		return nil
	}

	return a.wsMarket.SubscribeBookTicker(a.ExtractSymbol(symbol), handler)
}

func (a *SpotAdapter) subscribeOrderBookInternal(ctx context.Context, symbol string, callback exchanges.OrderBookCallback) error {
	formattedSymbol := a.FormatSymbol(symbol)
	if err := a.WsMarketConnected(ctx); err != nil {
		return err
	}

	a.cancelMu.Lock()
	// 如果已经存在订阅，先取消
	if cancel, ok := a.cancels[formattedSymbol]; ok {
		cancel()
	}

	// 创建新的 OrderBook 实例
	ob := NewSpotOrderBook(symbol)
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

				a.Logger.Infow("Fetching orderbook snapshot", "symbol", symbol)
				// 获取1000档深度快照
				snapshotDepth, err := a.client.Depth(ctx, strings.ToUpper(formattedSymbol), 1000)
				if err != nil {
					a.Logger.Errorw("Failed to fetch snapshot", "symbol", symbol, "error", err, "retryIn", retryDelay)
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
					case <-ctx.Done():
						return
					}
					continue
				}

				// 应用快照
				if err := ob.ApplySnapshot(snapshotDepth); err != nil {
					a.Logger.Warnw("Failed to apply snapshot", "symbol", symbol, "error", err, "retryIn", retryDelay)
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
					a.Logger.Infow("Orderbook initialized with snapshot", "symbol", symbol, "lastUpdateId", snapshotDepth.LastUpdateID)
				}
			}
		}
	}()

	// 订阅 WS 增量更新 (Diff Depth)
	err := a.wsMarket.SubscribeIncrementOrderBook(formattedSymbol, "100ms", func(e *spot.WsDepthEvent) error {
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
			if ob.IsInitialized() {
				a.Logger.Warnw("Orderbook sync detected gap/error", "symbol", symbol, "error", err)
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
			select {
			case snapshotTrigger <- struct{}{}:
			default:
			}
			return nil
		}

		// 如果提供了 callback，则推送数据
		if callback != nil {
			bids, asks := ob.GetDepth(20)

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

func (a *SpotAdapter) WatchKlines(ctx context.Context, symbol string, interval exchanges.Interval, callback exchanges.KlineCallback) error {
	if err := a.WsMarketConnected(ctx); err != nil {
		return err
	}

	handler := func(e *spot.KlineEvent) error {
		k := exchanges.Kline{
			Symbol:    symbol,
			Interval:  interval,
			Timestamp: e.Kline.StartTime,
			Open:      parseDecimal(e.Kline.OpenPrice),
			High:      parseDecimal(e.Kline.HighPrice),
			Low:       parseDecimal(e.Kline.LowPrice),
			Close:     parseDecimal(e.Kline.ClosePrice),
			Volume:    parseDecimal(e.Kline.Volume),
			QuoteVol:  parseDecimal(e.Kline.QuoteVolume),
		}
		callback(&k)
		return nil
	}

	return a.wsMarket.SubscribeKline(a.ExtractSymbol(symbol), string(interval), handler)
}

func (a *SpotAdapter) WatchTrades(ctx context.Context, symbol string, callback exchanges.TradeCallback) error {
	if err := a.WsMarketConnected(ctx); err != nil {
		return err
	}

	handler := func(e *spot.AggTradeEvent) error {
		side := exchanges.TradeSideBuy
		if !e.IsBuyerMaker {
			side = exchanges.TradeSideBuy // Buyer is Taker -> Buy
		} else {
			side = exchanges.TradeSideSell // Buyer is Maker -> Sell
		}

		t := exchanges.Trade{
			ID:        fmt.Sprintf("%d", e.AggTradeID),
			Symbol:    symbol,
			Price:     parseDecimal(e.Price),
			Quantity:  parseDecimal(e.Quantity),
			Side:      side,
			Timestamp: e.TradeTime,
		}
		callback(&t)
		return nil
	}

	return a.wsMarket.SubscribeAggTrade(a.ExtractSymbol(symbol), handler)
}

// Unsubscribe methods
func (a *SpotAdapter) StopWatchOrders(ctx context.Context) error {
	// Spot User Stream is one connection per listenKey.
	// Can't easily unsubscribe just one callback without logic in WsAccountClient
	return nil // no-op
}

func (a *SpotAdapter) StopWatchTicker(ctx context.Context, symbol string) error {
	if a.wsMarket == nil {
		return nil
	}
	return a.wsMarket.UnsubscribeBookTicker(a.ExtractSymbol(symbol))
}

func (a *SpotAdapter) StopWatchOrderBook(ctx context.Context, symbol string) error {
	// Need to match depth/interval used in Subscribe?
	// We assumed 20 / 100ms in SubscribeLimitOrderBook default?
	// WsMarketClient.UnsubscribeLimitOrderBook(symbol, levels, interval)
	// We'll try to unsubscribe what we likely subscribed.
	// But without tracking state it's hard.
	// For now, assume the user handles it or we pass default.
	// Let's assume depth 20 for generic Unsubscribe if we don't have params.
	if a.wsMarket == nil {
		return nil
	}
	// Try standard depth unsubscribe
	return a.wsMarket.UnsubscribeLimitOrderBook(a.ExtractSymbol(symbol), 20, "100ms")
}

func (a *SpotAdapter) StopWatchKlines(ctx context.Context, symbol string, interval exchanges.Interval) error {
	if a.wsMarket == nil {
		return nil
	}
	return a.wsMarket.UnsubscribeKline(a.ExtractSymbol(symbol), string(interval))
}

func (a *SpotAdapter) StopWatchTrades(ctx context.Context, symbol string) error {
	if a.wsMarket == nil {
		return nil
	}
	return a.wsMarket.UnsubscribeAggTrade(a.ExtractSymbol(symbol))
}

// ================= Internal Helpers =================

func (a *SpotAdapter) normalizeOrderResponse(resp *spot.OrderResponse) (*exchanges.Order, error) {
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
		Timestamp:      resp.TransactTime,
		ClientOrderID:  resp.ClientOrderID,
	}, nil
}

func (a *SpotAdapter) RefreshSymbolDetails(ctx context.Context) error {
	info, err := a.client.ExchangeInfo(ctx)
	if err != nil {
		return fmt.Errorf("failed to get exchange info: %w", err)
	}

	symbols := make(map[string]*exchanges.SymbolDetails)

	for _, s := range info.Symbols {
		// Only pairs matching configured quote currency
		if !strings.HasSuffix(s.Symbol, a.quoteCurrency) {
			continue
		}

		details := &exchanges.SymbolDetails{
			Symbol:            a.ExtractSymbol(s.Symbol),
			PricePrecision:    int32(s.QuotePrecision),
			QuantityPrecision: int32(s.BaseAssetPrecision),
		}

		// Parse filters
		for _, f := range s.Filters {
			filterType, ok := f["filterType"].(string)
			if !ok {
				continue
			}
			switch filterType {
			case "PRICE_FILTER":
				if tickSizeStr, ok := f["tickSize"].(string); ok {
					details.PricePrecision = getPrecision(tickSizeStr)
				}
			case "LOT_SIZE":
				if stepSizeStr, ok := f["stepSize"].(string); ok {
					details.QuantityPrecision = getPrecision(stepSizeStr)
				}
				if minQtyStr, ok := f["minQty"].(string); ok {
					details.MinQuantity = parseDecimal(minQtyStr)
				}
			case "NOTIONAL":
				if minNotionalStr, ok := f["minNotional"].(string); ok {
					details.MinNotional = parseDecimal(minNotionalStr)
				}
			}
		}

		symbols[details.Symbol] = details
	}

	a.SetSymbolDetails(symbols)
	return nil
}

func getPrecision(s string) int32 {
	f, _ := strconv.ParseFloat(s, 64)
	if f == 0 {
		return 0
	}
	var count int32
	for f < 1 {
		f *= 10
		count++
	}
	return count
}

func (a *SpotAdapter) FormatSymbol(symbol string) string {
	return FormatSymbolWithQuote(symbol, a.quoteCurrency)
}

func (a *SpotAdapter) ExtractSymbol(symbol string) string {
	return ExtractSymbolWithQuote(symbol, a.quoteCurrency)
}

// GetLocalOrderBook retrieves locally maintained order book
func (a *SpotAdapter) GetLocalOrderBook(symbol string, depth int) *exchanges.OrderBook {
	formattedSymbol := a.FormatSymbol(symbol)

	ob, ok := a.GetLocalOrderBookImplementation(formattedSymbol)
	if !ok {
		return nil
	}

	spotOb := ob.(*SpotOrderBook)
	if !spotOb.IsInitialized() {
		return nil
	}

	bids, asks := spotOb.GetDepth(depth)
	return &exchanges.OrderBook{
		Symbol:    symbol,
		Timestamp: spotOb.Timestamp(),
		Bids:      bids,
		Asks:      asks,
	}
}

func (a *SpotAdapter) SubscribeAllMiniTicker(ctx context.Context, callback func([]*spot.WsMiniTickerEvent)) error {
	if err := a.WsMarketConnected(ctx); err != nil {
		return err
	}
	return a.wsMarket.SubscribeAllMiniTicker(func(events []*spot.WsMiniTickerEvent) error {
		callback(events)
		return nil
	})
}

// WatchOrderBook subscribes to orderbook updates and waits for the book to be ready.
func (a *SpotAdapter) WatchOrderBook(ctx context.Context, symbol string, cb exchanges.OrderBookCallback) error {
	if err := a.subscribeOrderBookInternal(ctx, symbol, cb); err != nil {
		return err
	}
	formattedSymbol := a.FormatSymbol(symbol)
	return a.BaseAdapter.WaitOrderBookReady(ctx, formattedSymbol)
}

func (a *SpotAdapter) WatchPositions(ctx context.Context, cb exchanges.PositionUpdateCallback) error {
	return fmt.Errorf("not implemented")
}

func (a *SpotAdapter) StopWatchPositions(ctx context.Context) error {
	return nil
}
