package aster

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/aster/sdk/spot"

	"github.com/shopspring/decimal"
)

type SpotAdapter struct {
	*exchanges.BaseAdapter
	client        *spot.Client
	wsMarket      *spot.WsMarketClient
	wsAccount     *spot.WsAccountClient
	apiKey        string
	secretKey     string
	quoteCurrency string // "USDT" or "USDC"

	// OrderBook management cancellations
	cancelMu sync.Mutex
	cancels  map[string]context.CancelFunc
}

// NewSpotAdapter creates a new Aster Spot Adapter
func NewSpotAdapter(ctx context.Context, opts Options) (*SpotAdapter, error) {
	quote, err := opts.quoteCurrency()
	if err != nil {
		return nil, err
	}

	client := spot.NewClient(opts.APIKey, opts.SecretKey)
	wsMarket := spot.NewWsMarketClient(ctx)
	wsAccount := spot.NewWsAccountClient(ctx, opts.APIKey, opts.SecretKey)

	a := &SpotAdapter{
		BaseAdapter:   exchanges.NewBaseAdapter("ASTER", exchanges.MarketTypeSpot, opts.logger()),
		client:        client,
		wsMarket:      wsMarket,
		wsAccount:     wsAccount,
		apiKey:        opts.APIKey,
		secretKey:     opts.SecretKey,
		quoteCurrency: string(quote),
		cancels:       make(map[string]context.CancelFunc),
	}

	if err := a.RefreshSymbolDetails(context.Background()); err != nil {
		// TODO: logger.Error("Failed to refresh symbol details", "error", err)
	}

	// TODO: logger.Info("Initialized Aster Spot Adapter")
	return a, nil
}

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
	return nil
}

func (a *SpotAdapter) Close() error {
	a.wsMarket.Close()
	a.wsAccount.Close()
	return nil
}

// ================= Account & Trading =================

func (a *SpotAdapter) FetchAccount(ctx context.Context) (*exchanges.Account, error) {
	res, err := a.client.GetAccount(ctx)
	if err != nil {
		return nil, fmt.Errorf("aster spot get account failed: %w", err)
	}

	account := &exchanges.Account{
		Positions: []exchanges.Position{},
		Orders:    []exchanges.Order{},
	}

	// Spot has balances, not positions (conceptually) or positions are treated as assets.
	// Adapter.Account struct:
	// TotalBalance: usually total estimated value in USD or BTC
	// AvailableBalance: usually quote currency available
	// But Spot Account has many assets.
	// We'll try to find the configured quote currency for AvailableBalance.

	for _, b := range res.Balances {
		if b.Asset == a.quoteCurrency {
			free := parseDecimal(b.Free)
			locked := parseDecimal(b.Locked)
			account.AvailableBalance = free
			account.TotalBalance = free.Add(locked)
		}
	}
	// Note: TotalBalance in generic Account struct is often ambiguous for Spot.
	// Use GetSpotBalances for detailed view.

	return account, nil
}

func (a *SpotAdapter) FetchBalance(ctx context.Context) (decimal.Decimal, error) {
	acc, err := a.FetchAccount(ctx)
	if err != nil {
		return decimal.Zero, err
	}
	return acc.AvailableBalance, nil
}

func (a *SpotAdapter) FetchPositions(ctx context.Context) ([]exchanges.Position, error) {
	// Spot has no positions in PERP sense
	return []exchanges.Position{}, nil
}

func (a *SpotAdapter) PlaceOrder(ctx context.Context, params *exchanges.OrderParams) (*exchanges.Order, error) {
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

	p := spot.PlaceOrderParams{
		Symbol:   a.FormatSymbol(params.Symbol),
		Side:     string(params.Side),
		Type:     string(params.Type),
		Quantity: params.Quantity.String(),
	}

	if params.Price.IsPositive() {
		p.Price = params.Price.String()
	}

	if params.ClientID != "" {
		p.NewClientOrderID = params.ClientID
	}

	// TimeInForce mapping
	if params.TimeInForce != "" {
		p.TimeInForce = string(params.TimeInForce)
	} else if params.Type == exchanges.OrderTypeLimit {
		p.TimeInForce = "GTC"
	}

	resp, err := a.client.PlaceOrder(ctx, p)
	if err != nil {
		return nil, err
	}

	return a.normalizeOrderResponse(resp)
}

func (a *SpotAdapter) CancelOrder(ctx context.Context, orderID, symbol string) error {
	formattedSymbol := a.FormatSymbol(symbol)
	oid, _ := strconv.ParseInt(orderID, 10, 64)
	_, err := a.client.CancelOrder(ctx, formattedSymbol, oid, "")
	return err
}

func (a *SpotAdapter) ModifyOrder(ctx context.Context, orderID, symbol string, params *exchanges.ModifyOrderParams) (*exchanges.Order, error) {
	return nil, fmt.Errorf("modify order not supported by aster spot")
}

func (a *SpotAdapter) FetchOrder(ctx context.Context, orderID, symbol string) (*exchanges.Order, error) {
	formattedSymbol := a.FormatSymbol(symbol)
	oid, err := strconv.ParseInt(orderID, 10, 64)
	if err != nil {
		return nil, err
	}
	resp, err := a.client.GetOrder(ctx, formattedSymbol, oid, "")
	if err != nil {
		return nil, err
	}
	return a.normalizeOrderResponse(resp)
}

func (a *SpotAdapter) FetchOpenOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	formattedSymbol := a.FormatSymbol(symbol)
	resp, err := a.client.GetOpenOrders(ctx, formattedSymbol)
	if err != nil {
		return nil, err
	}

	orders := make([]exchanges.Order, 0, len(resp))
	for _, o := range resp {
		ord, _ := a.normalizeOrderResponse(&o)
		if ord != nil {
			orders = append(orders, *ord)
		}
	}
	return orders, nil
}

func (a *SpotAdapter) CancelAllOrders(ctx context.Context, symbol string) error {
	// Not supported natively in spot SDK usually, need to verify
	// SDK has no CancelAllOrders method visible in order.go view?
	// It has CancelOrder.
	// Aster Spot SDK client.go view didn't show CancelAllOrders
	// We might need to iterate OpenOrders.
	orders, err := a.FetchOpenOrders(ctx, symbol)
	if err != nil {
		return err
	}
	for _, o := range orders {
		if err := a.CancelOrder(ctx, o.OrderID, symbol); err != nil {
			// TODO: logger.Warn("Failed to cancel order", "orderID", o.OrderID, "err", err)
		}
	}
	return nil
}

func (a *SpotAdapter) SetLeverage(ctx context.Context, symbol string, leverage int) error {
	return fmt.Errorf("set leverage not supported for spot")
}

func (a *SpotAdapter) FetchFeeRate(ctx context.Context, symbol string) (*exchanges.FeeRate, error) {
	// Spot doesn't always show fee rate easily via API?
	// Ticker info might have it? No.
	// Account info has commissions.
	acc, err := a.client.GetAccount(ctx)
	if err != nil {
		return nil, err
	}
	// Commissions in Account response are basis points or similar?
	// Docs say "makerCommission": 10 (which means 0.1% usually)
	return &exchanges.FeeRate{
		Maker: decimal.NewFromInt(int64(acc.MakerCommission)).Div(decimal.NewFromInt(10000)),
		Taker: decimal.NewFromInt(int64(acc.TakerCommission)).Div(decimal.NewFromInt(10000)),
	}, nil
}

// ================= Market Data =================

func (a *SpotAdapter) FetchTicker(ctx context.Context, symbol string) (*exchanges.Ticker, error) {
	formattedSymbol := a.FormatSymbol(symbol)
	t, err := a.client.Ticker(ctx, formattedSymbol)
	if err != nil {
		return nil, err
	}
	// Also get book ticker for bid/ask if 24hr ticker doesn't have it (it usually does)
	// TickerResponse struct has BidPrice, AskPrice.

	return &exchanges.Ticker{
		Symbol:    symbol,
		LastPrice: parseDecimal(t.LastPrice),
		High24h:   parseDecimal(t.HighPrice),
		Low24h:    parseDecimal(t.LowPrice),
		Volume24h: parseDecimal(t.Volume),
		QuoteVol:  parseDecimal(t.QuoteVolume),
		Bid:       parseDecimal(t.BidPrice),
		Ask:       parseDecimal(t.AskPrice),
		Timestamp: t.CloseTime,
	}, nil
}

func (a *SpotAdapter) FetchOrderBook(ctx context.Context, symbol string, limit int) (*exchanges.OrderBook, error) {
	formattedSymbol := a.FormatSymbol(symbol)
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
	for _, row := range res {
		// row is []interface{}
		if len(row) < 6 {
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
			// QuoteVol is at index 7?
			// QuoteVol:  parseDecimalInterface(row[7]),
		}
		if len(row) > 7 {
			k.QuoteVol = parseDecimalInterface(row[7])
		}
		klines = append(klines, k)
	}
	return klines, nil
}

func (a *SpotAdapter) FetchTrades(ctx context.Context, symbol string, limit int) ([]exchanges.Trade, error) {
	formattedSymbol := a.FormatSymbol(symbol)
	res, err := a.client.MyTrades(ctx, formattedSymbol, limit, 0, 0, 0)
	if err != nil {
		return nil, err
	}

	trades := make([]exchanges.Trade, 0, len(res))
	for _, t := range res {
		side := exchanges.TradeSideBuy
		if t.IsBuyer {
			side = exchanges.TradeSideBuy
		} else {
			side = exchanges.TradeSideSell
		}

		trades = append(trades, exchanges.Trade{
			ID:        fmt.Sprintf("%d", t.ID),
			Symbol:    symbol,
			Price:     parseDecimal(t.Price),
			Quantity:  parseDecimal(t.Qty),
			Side:      side,
			Timestamp: t.Time,
		})
	}
	return trades, nil
}

func (a *SpotAdapter) FetchSymbolDetails(ctx context.Context, symbol string) (*exchanges.SymbolDetails, error) {
	normalized := a.ExtractSymbol(symbol)
	return a.GetSymbolDetail(normalized)
}

// ================= Spot-Specific =================

func (a *SpotAdapter) FetchSpotBalances(ctx context.Context) ([]exchanges.SpotBalance, error) {
	res, err := a.client.GetAccount(ctx)
	if err != nil {
		return nil, err
	}

	balances := make([]exchanges.SpotBalance, 0, len(res.Balances))
	for _, b := range res.Balances {
		free := parseDecimal(b.Free)
		locked := parseDecimal(b.Locked)
		if free.IsZero() && locked.IsZero() {
			continue
		}
		balances = append(balances, exchanges.SpotBalance{
			Asset:  b.Asset,
			Free:   free,
			Locked: locked,
			Total:  free.Add(locked),
		})
	}
	return balances, nil
}

func (a *SpotAdapter) TransferAsset(ctx context.Context, params *exchanges.TransferParams) error {
	return fmt.Errorf("transfer asset not supported")
}

// ================= WebSocket =================

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
			Symbol:         report.Symbol,
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
			Symbol: symbol,
			Bid:    parseDecimal(e.BestBidPrice),
			Ask:    parseDecimal(e.BestAskPrice),
			// Approximate timestamp if not in event, but BookTicker usually has "u" (updateId) not time
			// We can use current time or 0
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
	if a.cancels == nil {
		a.cancels = make(map[string]context.CancelFunc)
	}

	// 如果已经存在订阅，先取消
	if cancel, ok := a.cancels[formattedSymbol]; ok {
		cancel()
	}

	// 创建新的 OrderBook 实例
	ob := NewSpotOrderBook(formattedSymbol)
	a.SetLocalOrderBook(formattedSymbol, ob)

	ctxCancel, cancel := context.WithCancel(context.Background())
	a.cancels[formattedSymbol] = cancel
	a.cancelMu.Unlock()

	// 用于触发快照同步的通道
	// 大小设为 1，避免阻塞
	snapshotTrigger := make(chan struct{}, 1)

	// 启动快照同步协程
	go func() {
		for {
			select {
			case <-ctxCancel.Done():
				return
			case <-snapshotTrigger:
				// 如果已经初始化完成了，就不再请求快照
				if ob.IsInitialized() {
					continue
				}

				// 获取1000档深度快照
				// limit=1000 compliant with explicit instruction "Step 3"
				snapshotDepth, err := a.client.Depth(ctx, formattedSymbol, 1000)
				if err != nil {
					// TODO: logger.Error("Failed to fetch snapshot", "symbol", symbol, "error", err)
					// 失败后延时重试
					select {
					case <-time.After(time.Second):
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
					// TODO: logger.Warn("Failed to apply snapshot", "symbol", symbol, "error", err)
					// 重新触发
					select {
					case <-time.After(time.Second):
						select {
						case snapshotTrigger <- struct{}{}:
						default:
						}
					case <-ctxCancel.Done():
						return
					}
				} else {
					// TODO: logger.Info("Orderbook initialized with snapshot", "symbol", symbol, "lastUpdateId", snapshotDepth.LastUpdateID)
				}
			}
		}
	}()

	// 订阅 WS 增量更新 (100ms)
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
	return fmt.Errorf("subscribe kline not implemented")
}

func (a *SpotAdapter) WatchTrades(ctx context.Context, symbol string, callback exchanges.TradeCallback) error {
	return fmt.Errorf("subscribe trades not implemented")
}

func (a *SpotAdapter) StopWatchOrders(ctx context.Context) error {
	return nil
}

func (a *SpotAdapter) StopWatchTicker(ctx context.Context, symbol string) error {
	return nil // TODO
}

func (a *SpotAdapter) StopWatchOrderBook(ctx context.Context, symbol string) error {
	return nil // TODO
}

func (a *SpotAdapter) StopWatchKlines(ctx context.Context, symbol string, interval exchanges.Interval) error {
	return nil
}

func (a *SpotAdapter) StopWatchTrades(ctx context.Context, symbol string) error {
	return nil
}

// ================= Internal Methods =================

func (a *SpotAdapter) RefreshSymbolDetails(ctx context.Context) error {
	res, err := a.client.ExchangeInfo(ctx)
	if err != nil {
		return err
	}

	symbols := make(map[string]*exchanges.SymbolDetails)

	for _, s := range res.Symbols {
		if !strings.HasSuffix(s.Symbol, a.quoteCurrency) {
			continue
		}

		details := &exchanges.SymbolDetails{
			Symbol:            s.Symbol,
			PricePrecision:    int32(s.PricePrecision),
			QuantityPrecision: int32(s.QuantityPrecision),
		}

		for _, f := range s.Filters {
			filterType, ok := f["filterType"].(string)
			if !ok {
				continue
			}
			switch filterType {
			case "LOT_SIZE":
				if minQtyStr, ok := f["minQty"].(string); ok {
					details.MinQuantity = parseDecimal(minQtyStr)
				}
			case "MIN_NOTIONAL":
				if minNotionalStr, ok := f["minNotional"].(string); ok {
					details.MinNotional = parseDecimal(minNotionalStr)
				}
			case "NOTIONAL":
				if minNotionalStr, ok := f["minNotional"].(string); ok {
					details.MinNotional = parseDecimal(minNotionalStr)
				}
			}
		}

		symbols[a.ExtractSymbol(s.Symbol)] = details
	}

	a.SetSymbolDetails(symbols)
	return nil
}

func (a *SpotAdapter) FormatSymbol(symbol string) string {
	s := strings.ToUpper(symbol)
	q := strings.ToUpper(a.quoteCurrency)
	ql := strings.ToLower(a.quoteCurrency)
	if !strings.HasSuffix(symbol, q) && !strings.HasSuffix(symbol, ql) {
		symbol = s + q
	}
	return strings.ToLower(symbol)
}

func (a *SpotAdapter) ExtractSymbol(symbol string) string {
	s := strings.ToUpper(symbol)
	q := strings.ToUpper(a.quoteCurrency)
	if strings.HasSuffix(s, q) {
		s = s[:len(s)-len(q)]
	}
	return s
}

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

// WatchOrderBook subscribes to orderbook updates and waits for the book to be ready.
func (a *SpotAdapter) WatchOrderBook(ctx context.Context, symbol string, cb exchanges.OrderBookCallback) error {
	if err := a.subscribeOrderBookInternal(ctx, symbol, cb); err != nil {
		return err
	}
	formattedSymbol := a.FormatSymbol(symbol)
	return a.BaseAdapter.WaitOrderBookReady(ctx, formattedSymbol)
}

// Helpers

func (a *SpotAdapter) normalizeOrderResponse(r *spot.OrderResponse) (*exchanges.Order, error) {
	status := exchanges.OrderStatusUnknown
	switch r.Status {
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

	return &exchanges.Order{
		OrderID:        fmt.Sprintf("%d", r.OrderID),
		Symbol:         a.ExtractSymbol(r.Symbol),
		Side:           exchanges.OrderSide(r.Side),
		Type:           exchanges.OrderType(r.Type),
		Quantity:       parseDecimal(r.OrigQty),
		Price:          parseDecimal(r.Price),
		Status:         status,
		FilledQuantity: parseDecimal(r.ExecutedQty),
		Timestamp:      r.TransactTime,
		ClientOrderID:  r.ClientOrderID,
		TimeInForce:    exchanges.TimeInForce(r.TimeInForce),
	}, nil
}

func (a *SpotAdapter) WatchPositions(ctx context.Context, cb exchanges.PositionUpdateCallback) error {
	return fmt.Errorf("not implemented")
}

func (a *SpotAdapter) StopWatchPositions(ctx context.Context) error {
	return nil
}
