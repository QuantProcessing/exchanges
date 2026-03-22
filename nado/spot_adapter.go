package nado

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/nado/sdk"

	"github.com/shopspring/decimal"
)

// Adapter Nado 适配器
type SpotAdapter struct {
	*exchanges.BaseAdapter
	httpClient *nado.Client
	apiClient  *nado.WsApiClient
	wsMarket   *nado.WsMarketClient
	wsAccount  *nado.WsAccountClient

	privateKey string
	subaccount string

	productMap map[string]int64 // Symbol (e.g. BTC) -> ProductID
	idMap      map[int64]string // ProductID -> Symbol
	symbolInfo map[string]nado.Symbol

	isConnected bool
	mu          sync.RWMutex

	cancelMu sync.Mutex
	cancels  map[string]context.CancelFunc

	sender string

	// Order state tracking for "remaining" amount conversion
	// Digest -> OriginalQuantity
	orderMap sync.Map
}

// NewAdapter 创建 Nado 适配器
func NewSpotAdapter(ctx context.Context, opts Options) (*SpotAdapter, error) {
	subaccount := opts.SubAccountName
	if subaccount == "" {
		subaccount = "default"
	}
	privateKey := opts.PrivateKey

	httpClient := nado.NewClient()
	var apiClient *nado.WsApiClient
	var sender string

	if privateKey != "" {
		var err error
		httpClient, err = httpClient.WithCredentials(privateKey, subaccount)
		if err != nil {
			return nil, fmt.Errorf("nado spot init: %w", err)
		}
		apiClient, err = nado.NewWsApiClient(ctx, privateKey)
		if err != nil {
			return nil, fmt.Errorf("nado spot init: %w", err)
		}
		apiClient.SetSubaccount(subaccount)
		sender = nado.BuildSender(apiClient.Signer.GetAddress(), subaccount)
	}

	wsMarket := nado.NewWsMarketClient(ctx)
	wsAccount := nado.NewWsAccountClient(ctx)
	if privateKey != "" {
		wsAccount.WithCredentials(privateKey)
	}

	a := &SpotAdapter{
		BaseAdapter: exchanges.NewBaseAdapter("NADO", exchanges.MarketTypeSpot, opts.logger()),
		httpClient:  httpClient,
		apiClient:   apiClient,
		wsMarket:    wsMarket,
		wsAccount:   wsAccount,
		privateKey:  privateKey,
		subaccount:  subaccount,
		productMap:  make(map[string]int64),
		idMap:       make(map[int64]string),
		symbolInfo:  make(map[string]nado.Symbol),
		cancels:     make(map[string]context.CancelFunc),
		sender:      sender,
	}

	if err := a.RefreshSymbolDetails(context.Background()); err != nil {
		return nil, fmt.Errorf("nado spot init: %w", err)
	}

	// TODO: logger.Info("Initialized Nado Adapter")
	return a, nil
}

func (a *SpotAdapter) WsAccountConnected(ctx context.Context) error {
	if !a.wsAccount.IsConnected() {
		if err := a.wsAccount.Connect(); err != nil {
			return err
		}
	}

	return nil
}

func (a *SpotAdapter) WsMarketConnected(ctx context.Context) error {
	if !a.wsMarket.IsConnected() {
		if err := a.wsMarket.Connect(); err != nil {
			return err
		}
	}
	return nil
}

func (a *SpotAdapter) WsOrderConnected(ctx context.Context) error {
	if !a.apiClient.IsConnected() {
		if err := a.apiClient.Connect(); err != nil {
			return err
		}
	}

	return nil
}

func (a *SpotAdapter) fetchSymbols(ctx context.Context) error {
	productType := "spot"
	resp, err := a.httpClient.GetSymbols(ctx, &productType)
	if err != nil {
		return err
	}

	a.productMap = make(map[string]int64)
	a.idMap = make(map[int64]string)
	a.symbolInfo = make(map[string]nado.Symbol)

	symbols := make(map[string]*exchanges.SymbolDetails)

	for _, s := range resp.Symbols {
		if s.Type == "spot" {
			// symbol is usually "kBTC wETH SOL"
			a.productMap[s.Symbol] = int64(s.ProductID)
			a.idMap[int64(s.ProductID)] = s.Symbol

			// Parse precision from increments
			priceInc := parseX18(s.PriceIncrementX18)
			sizeInc := parseX18(s.SizeIncrement)
			// minSize := parseX18(s.MinSize)

			symbols[s.Symbol] = &exchanges.SymbolDetails{
				Symbol:            s.Symbol,
				MinNotional:       decimal.Zero, // not provided in json
				MinQuantity:       sizeInc,      // eth use size inc as min quantity
				PricePrecision:    exchanges.CountDecimalPlaces(priceInc.String()),
				QuantityPrecision: exchanges.CountDecimalPlaces(sizeInc.String()),
			}
			a.symbolInfo[s.Symbol] = s
		}
	}
	a.SetSymbolDetails(symbols)
	return nil
}

func (a *SpotAdapter) IsConnected(ctx context.Context) (bool, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.isConnected, nil // simplified
}

func (a *SpotAdapter) Close() error {
	a.apiClient.Close()
	a.wsAccount.Close()
	a.wsMarket.Close()
	return nil
}

func (a *SpotAdapter) RefreshSymbolDetails(ctx context.Context) error {
	return a.fetchSymbols(ctx)
}

func (a *SpotAdapter) FormatSymbol(symbol string) string {
	if strings.HasSuffix(symbol, "_USDT0") {
		return symbol
	}
	tickerID := fmt.Sprintf("%s_USDT0", symbol)
	return strings.ToUpper(tickerID)
}

func (a *SpotAdapter) ExtractSymbol(symbol string) string {
	if strings.HasSuffix(symbol, "_USDT0") {
		return symbol[:len(symbol)-6]
	}
	return strings.ToUpper(symbol)
}

// ================= Account & Trading =================

func (a *SpotAdapter) FetchAccount(ctx context.Context) (*exchanges.Account, error) {
	// Use subaccount_info API from Gateway to get balances and positions
	resp, err := a.httpClient.GetAccount(ctx)
	if err != nil {
		return nil, err
	}

	var totalBalance, availableBalance decimal.Decimal

	// Total Balance: Spot Balance for Product 0 (USDT0)
	// Contains un-realized PnL
	for _, b := range resp.SpotBalances {
		if b.ProductID == 0 {
			totalBalance = parseX18(b.Balance.Amount)
			break
		}
	}

	// Available Balance: From healths[0].health (initial health)
	// healths[0] represents initial health weighted by initial weights
	if len(resp.Healths) > 0 {
		availableBalance = parseX18(resp.Healths[0].Health)
	}

	account := &exchanges.Account{
		TotalBalance:     totalBalance,
		AvailableBalance: availableBalance,
		Positions:        []exchanges.Position{},
		Orders:           []exchanges.Order{},
	}

	// Collect Product IDs with non-zero positions to fetch matches
	var activeProductIds []int64
	for _, b := range resp.PerpBalances {
		amount := parseX18(b.Balance.Amount)
		if !amount.IsZero() {
			activeProductIds = append(activeProductIds, b.ProductID)
		}
	}

	// Fetch Matches from Archive Indexer for net execution price
	var matchesResp *nado.ArchiveMatchesResponse
	if len(activeProductIds) > 0 {
		sender := nado.BuildSender(a.apiClient.Signer.GetAddress(), a.subaccount)
		matchesResp, err = a.httpClient.GetMatches(ctx, sender, activeProductIds, 100)
		if err != nil {
			// 			// TODO: logger.Warn("Failed to fetch matches for entry price calculation", zap.Error(err))
		}
	}

	// Fetch Tickers for Mark Price to compute unrealized PnL
	tickers, err := a.httpClient.GetTickers(ctx, nado.MarketTypeSpot, nil)
	if err != nil {
		// 		// TODO: logger.Warn("Failed to fetch tickers for mark price", zap.Error(err))
	}

	// Process Perp Balances as Positions
	for _, b := range resp.PerpBalances {
		signedAmount := parseX18(b.Balance.Amount)
		if signedAmount.IsZero() {
			continue
		}

		symbol := a.getSymbol(b.ProductID)
		side := exchanges.PositionSideLong
		quantity := signedAmount
		if signedAmount.IsNegative() {
			side = exchanges.PositionSideShort
			quantity = signedAmount.Abs() // Adapter expects positive quantity
		}

		var unrealizedPnl decimal.Decimal

		// Calculate Entry Price from Matches using net execution price
		entryPrice, realizedPnL, err := a.GetNetEntryPriceAndRealizedPnL(b.ProductID, quantity.InexactFloat64(), side, matchesResp)
		if err != nil {
			// 			// TODO: logger.Warn("Failed to calculate entry price", zap.Error(err))
			return nil, err
		}

		// Calculate Unrealized PnL from Mark Price
		ticker, ok := tickers[a.FormatSymbol(symbol)]
		if !ok {
			// 			// TODO: logger.Warn("Ticker not found for symbol", zap.String("symbol", symbol))
			return nil, fmt.Errorf("ticker not found for symbol %s", symbol)
		}
		unrealizedPnl = decimal.NewFromFloat(ticker.LastPrice - entryPrice).Mul(quantity)
		if side == exchanges.PositionSideShort {
			unrealizedPnl = unrealizedPnl.Neg()
		}

		account.Positions = append(account.Positions, exchanges.Position{
			Symbol:        symbol,
			Side:          side,
			Quantity:      quantity,
			EntryPrice:    decimal.NewFromFloat(entryPrice),
			UnrealizedPnL: unrealizedPnl,
			RealizedPnL:   decimal.NewFromFloat(realizedPnL),
			MarginType:    "CROSS",
		})
	}

	return account, nil
}

func (a *SpotAdapter) FetchSpotBalances(ctx context.Context) ([]exchanges.SpotBalance, error) {
	resp, err := a.httpClient.GetAccount(ctx)
	if err != nil {
		return nil, err
	}

	var balances []exchanges.SpotBalance
	for _, b := range resp.SpotBalances {
		// Nado SpotBalance: product_id, balance (amount)
		// Usually ProductID 0 is USDT
		// Mapping needs productMap reverse lookup or similar?
		// Nado GetAccount resp has SpotBalances. ProductID 0 is Settlement/Quote?
		// Let's try to map generic symbol using idMap

		asset := fmt.Sprintf("PRODUCT-%d", b.ProductID)
		name := a.getSymbol(b.ProductID)
		if name != fmt.Sprintf("UNKNOWN-%d", b.ProductID) {
			asset = name
		} else if b.ProductID == 0 {
			asset = "USDT" // Assumption for Nado
		}

		total := parseX18(b.Balance.Amount)

		balances = append(balances, exchanges.SpotBalance{
			Asset:  asset,
			Free:   total,        // Nado doesn't explicitly separate locked in spot balance struct here easily without orders?
			Locked: decimal.Zero, // TODO: subtract open order costs if needed
			Total:  total,
		})
	}
	return balances, nil
}

func (a *SpotAdapter) TransferAsset(ctx context.Context, params *exchanges.TransferParams) error {
	return fmt.Errorf("transfer asset not supported by nado adapter")
}

func (a *SpotAdapter) FetchBalance(ctx context.Context) (decimal.Decimal, error) {
	acc, err := a.FetchAccount(ctx)
	if err != nil {
		return decimal.Zero, err
	}
	return acc.AvailableBalance, nil
}

func (a *SpotAdapter) FetchPositions(ctx context.Context) ([]exchanges.Position, error) {
	acc, err := a.FetchAccount(ctx)
	if err != nil {
		return nil, err
	}
	return acc.Positions, nil
}

func (a *SpotAdapter) PlaceOrder(ctx context.Context, params *exchanges.OrderParams) (*exchanges.Order, error) {
	// Apply slippage logic: converts MARKET+Slippage to LIMIT+IOC
	if err := a.BaseAdapter.ApplySlippage(ctx, params, a.FetchTicker); err != nil {
		return nil, err
	}
	if err := a.WsOrderConnected(ctx); err != nil {
		return nil, err
	}
	productID, err := a.getProductId(params.Symbol)
	if err != nil {
		return nil, err
	}

	// 1. Validation & Formatting
	details, err := a.FetchSymbolDetails(ctx, params.Symbol)
	if err == nil {
		if params.Type == exchanges.OrderTypeMarket && params.Price.IsZero() {
			ticker, err := a.FetchTicker(ctx, params.Symbol)
			if err != nil {
				return nil, fmt.Errorf("failed to get ticker for market order price: %w", err)
			}
			if params.Side == exchanges.OrderSideBuy {
				params.Price = ticker.LastPrice.Mul(decimal.NewFromFloat(1.05))
			} else {
				params.Price = ticker.LastPrice.Mul(decimal.NewFromFloat(0.95))
			}
		}

		if err := exchanges.ValidateAndFormatParams(params, details); err != nil {
			return nil, err
		}
	}

	side := nado.OrderSideBuy
	if params.Side == exchanges.OrderSideSell {
		side = nado.OrderSideSell
	}

	ot := nado.OrderTypeLimit
	switch params.Type {
	case exchanges.OrderTypeMarket:
		ot = nado.OrderTypeMarket
	case exchanges.OrderTypePostOnly:
		ot = nado.OrderTypeLimit
	}

	input := nado.ClientOrderInput{
		ProductId:  int64(productID),
		Side:       side,
		Price:      params.Price.StringFixed(details.PricePrecision),
		Amount:     params.Quantity.StringFixed(details.QuantityPrecision),
		OrderType:  ot,
		PostOnly:   params.Type == exchanges.OrderTypePostOnly,
		ReduceOnly: params.ReduceOnly,
	}

	switch params.TimeInForce {
	case exchanges.TimeInForceIOC:
		input.OrderType = nado.OrderTypeIOC
	case exchanges.TimeInForceFOK:
		input.OrderType = nado.OrderTypeFOK
	}

	// Prepare order (sign and calculate hash) locally
	prepared, err := a.apiClient.PrepareOrder(ctx, input)
	if err != nil {
		return nil, err
	}

	// Cache original quantity for WS update calculation - BEFORE execution to avoid race
	a.orderMap.Store(prepared.Digest, params.Quantity)

	// Execute
	resp, err := a.apiClient.ExecutePreparedOrder(ctx, prepared)
	if err != nil {
		// Cleanup on failure
		a.orderMap.Delete(prepared.Digest)
		return nil, err
	}

	return &exchanges.Order{
		OrderID:   resp.Digest,
		Symbol:    params.Symbol,
		Side:      params.Side,
		Type:      params.Type,
		Quantity:  params.Quantity,
		Price:     params.Price,
		Status:    exchanges.OrderStatusPending,
		Timestamp: time.Now().UnixMilli(),
	}, nil
}

func (a *SpotAdapter) CancelOrder(ctx context.Context, orderID, symbol string) error {
	if err := a.WsOrderConnected(ctx); err != nil {
		return err
	}
	productID, err := a.getProductId(symbol)
	if err != nil {
		return err
	}

	input := nado.CancelOrdersInput{
		ProductIds: []int64{productID},
		Digests:    []string{orderID},
	}
	_, err = a.apiClient.CancelOrders(ctx, input)
	return err
}

func (a *SpotAdapter) ModifyOrder(ctx context.Context, orderID, symbol string, params *exchanges.ModifyOrderParams) (*exchanges.Order, error) {
	return nil, fmt.Errorf("modify order not supported by nado")
}

func (a *SpotAdapter) FetchOrderByID(ctx context.Context, orderID, symbol string) (*exchanges.Order, error) {
	productID, err := a.getProductId(symbol)
	if err != nil {
		return nil, err
	}

	resp, err := a.httpClient.GetOrder(ctx, productID, orderID)
	if err != nil {
		if isNadoOrderLookupMiss(err) {
			return nil, exchanges.ErrOrderNotFound
		}
		return nil, err
	}
	if resp == nil || resp.Digest == "" {
		return nil, exchanges.ErrOrderNotFound
	}
	return a.mapOrder(resp), nil
}

func (a *SpotAdapter) FetchOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	return nil, exchanges.ErrNotSupported
}

func (a *SpotAdapter) FetchOpenOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	productID, err := a.getProductId(symbol)
	if err != nil {
		return nil, err
	}

	sender := nado.BuildSender(a.apiClient.Signer.GetAddress(), a.subaccount)
	resp, err := a.httpClient.GetAccountProductOrders(ctx, productID, sender)
	if err != nil {
		return nil, err
	}

	var orders []exchanges.Order
	for _, o := range resp.Orders {
		orders = append(orders, *a.mapOrder(&o))
	}
	return orders, nil
}

func (a *SpotAdapter) CancelAllOrders(ctx context.Context, symbol string) error {
	if err := a.WsOrderConnected(ctx); err != nil {
		return err
	}
	productID, err := a.getProductId(symbol)
	if err != nil {
		return err
	}

	tx := nado.TxCancelProductOrders{
		Sender:     nado.BuildSender(a.apiClient.Signer.GetAddress(), a.subaccount),
		ProductIds: []int64{productID},
		Nonce:      strconv.FormatInt(nado.GetNonce(), 10),
	}
	_, err = a.apiClient.WsCancelProductOrders(tx)
	return err
}

func (a *SpotAdapter) SetLeverage(ctx context.Context, symbol string, leverage int) error {
	return fmt.Errorf("set leverage not supported")
}

func (a *SpotAdapter) FetchFeeRate(ctx context.Context, symbol string) (*exchanges.FeeRate, error) {
	fees, err := a.httpClient.GetFeeRates(ctx)
	if err != nil {
		return nil, err
	}
	info, ok := a.symbolInfo[symbol]
	if !ok {
		return nil, fmt.Errorf("symbol info not found")
	}
	productID := info.ProductID
	maker := parseX18(fees.MakerFeeRateX18[productID])
	taker := parseX18(fees.TakerFeeRateX18[productID])
	return &exchanges.FeeRate{Maker: maker, Taker: taker}, nil
}

// ================= Market Data =================

func (a *SpotAdapter) FetchTicker(ctx context.Context, symbol string) (*exchanges.Ticker, error) {
	tickers, err := a.httpClient.GetTickers(ctx, nado.MarketTypeSpot, nil)
	if err != nil {
		return nil, err
	}

	tickerID := a.FormatSymbol(symbol)
	t, ok := tickers[tickerID]
	if !ok {
		// Try fallback if format differs?
		return nil, fmt.Errorf("ticker not found: %s", tickerID)
	}

	return &exchanges.Ticker{
		Symbol:    symbol,
		LastPrice: decimal.NewFromFloat(t.LastPrice),
		Volume24h: decimal.NewFromFloat(t.BaseVolume),
		QuoteVol:  decimal.NewFromFloat(t.QuoteVolume),
		Timestamp: time.Now().UnixMilli(),
	}, nil
}

func (a *SpotAdapter) FetchOrderBook(ctx context.Context, symbol string, limit int) (*exchanges.OrderBook, error) {
	tickerID := a.FormatSymbol(symbol)
	res, err := a.httpClient.GetOrderBook(ctx, tickerID, limit)
	if err != nil {
		return nil, err
	}

	ob := &exchanges.OrderBook{
		Symbol:    symbol,
		Timestamp: res.Timestamp,
		Bids:      make([]exchanges.Level, len(res.Bids)),
		Asks:      make([]exchanges.Level, len(res.Asks)),
	}

	for i, b := range res.Bids {
		if len(b) >= 2 {
			ob.Bids[i] = exchanges.Level{Price: smartScale(b[0]), Quantity: smartScale(b[1])}
		}
	}
	for i, a := range res.Asks {
		if len(a) >= 2 {
			ob.Asks[i] = exchanges.Level{Price: smartScale(a[0]), Quantity: smartScale(a[1])}
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
	productID, err := a.getProductId(symbol)
	if err != nil {
		return nil, err
	}

	granularity := int64(60)
	switch interval {
	case exchanges.Interval1m:
		granularity = 60
	case exchanges.Interval5m:
		granularity = 300
	case exchanges.Interval15m:
		granularity = 900
	case exchanges.Interval1h:
		granularity = 3600
	case exchanges.Interval4h:
		granularity = 14400
	case exchanges.Interval1d:
		granularity = 86400
	}

	req := nado.CandlestickRequest{
		Candlesticks: nado.Candlesticks{
			ProductID:   productID,
			Granularity: granularity,
			Limit:       limit,
		},
	}
	if end != nil {
		req.Candlesticks.MaxTime = end.Unix()
	} else {
		req.Candlesticks.MaxTime = time.Now().Unix()
	}

	candles, err := a.httpClient.GetCandlesticks(ctx, req)
	if err != nil {
		return nil, err
	}

	klines := make([]exchanges.Kline, len(candles))
	for i, c := range candles {
		ts, _ := strconv.ParseInt(c.Timestamp, 10, 64)
		klines[i] = exchanges.Kline{
			Symbol:    symbol,
			Interval:  interval,
			Timestamp: ts * 1000,
			Open:      parseX18(c.OpenX18),
			High:      parseX18(c.HighX18),
			Low:       parseX18(c.LowX18),
			Close:     parseX18(c.CloseX18),
			Volume:    parseX18(c.Volume),
		}
	}
	// Reverse to ascending
	for i, j := 0, len(klines)-1; i < j; i, j = i+1, j-1 {
		klines[i], klines[j] = klines[j], klines[i]
	}
	return klines, nil
}

func (a *SpotAdapter) FetchTrades(ctx context.Context, symbol string, limit int) ([]exchanges.Trade, error) {
	tickerID := a.FormatSymbol(symbol)
	res, err := a.httpClient.GetTrades(ctx, tickerID, &limit, nil)
	if err != nil {
		return nil, err
	}

	trades := make([]exchanges.Trade, len(res))
	for i, t := range res {
		side := exchanges.TradeSideBuy
		if t.TradeType == "sell" {
			side = exchanges.TradeSideSell
		}
		trades[i] = exchanges.Trade{
			ID:        fmt.Sprintf("%d", t.TradeID),
			Symbol:    symbol,
			Price:     decimal.NewFromFloat(t.Price),
			Quantity:  decimal.NewFromFloat(t.BaseFilled),
			Side:      side,
			Timestamp: t.Timestamp / 1000,
		}
	}
	return trades, nil
}

// ================= WebSocket =================

func (a *SpotAdapter) WatchOrders(ctx context.Context, callback exchanges.OrderUpdateCallback) error {
	if err := a.WsAccountConnected(ctx); err != nil {
		return err
	}
	return a.wsAccount.SubscribeOrders(nil, func(d *nado.OrderUpdate) {
		symbol := a.getSymbol(d.ProductId)
		status := exchanges.OrderStatusUnknown
		switch nado.OrderUpdateReason(d.Reason) {
		case nado.OrderReasonFilled:
			status = exchanges.OrderStatusFilled
		case nado.OrderReasonCanceled:
			status = exchanges.OrderStatusCancelled
		case nado.OrderReasonPlaced:
			status = exchanges.OrderStatusNew
		}

		ts, _ := strconv.ParseInt(d.Timestamp, 10, 64)
		remaining := parseX18(d.Amount)
		var filled decimal.Decimal

		if nado.OrderUpdateReason(d.Reason) == nado.OrderReasonPlaced {
			// Cache total amount
			a.orderMap.Store(d.Digest, remaining)
			filled = decimal.Zero
		} else {
			if val, ok := a.orderMap.Load(d.Digest); ok {
				original := val.(decimal.Decimal)
				filled = original.Sub(remaining)
				if filled.IsNegative() {
					filled = decimal.Zero
				}
				// "filled" with remaining > 0 is actually partial fill
				if nado.OrderUpdateReason(d.Reason) == nado.OrderReasonFilled && remaining.IsPositive() {
					status = exchanges.OrderStatusPartiallyFilled
				}
				// Cleanup on terminal state
				if status == exchanges.OrderStatusFilled || status == exchanges.OrderStatusCancelled {
					a.orderMap.Delete(d.Digest)
				}
			}
		}

		callback(&exchanges.Order{
			OrderID:        d.Digest,
			Symbol:         symbol,
			Status:         status,
			Timestamp:      ts,
			FilledQuantity: filled,
		})
	})
}

func (a *SpotAdapter) WatchPositions(ctx context.Context, callback exchanges.PositionUpdateCallback) error {
	if err := a.WsAccountConnected(ctx); err != nil {
		return err
	}
	return a.wsAccount.SubscribePositions(nil, func(d *nado.PositionChange) {
		amount := parseX18(d.Amount)
		entry := parseX18(d.EntryPrice)
		side := exchanges.PositionSideLong
		if d.Side == "short" {
			side = exchanges.PositionSideShort
		}

		callback(&exchanges.Position{
			Symbol:     a.getSymbol(d.ProductId),
			Side:       side,
			Quantity:   amount,
			EntryPrice: entry,
		})
	})
}

func (a *SpotAdapter) WatchTicker(ctx context.Context, symbol string, callback exchanges.TickerCallback) error {
	if err := a.WsMarketConnected(ctx); err != nil {
		return err
	}
	id, err := a.getProductId(symbol)
	if err != nil {
		return err
	}
	return a.wsMarket.SubscribeTicker(id, func(d *nado.Ticker) {
		bid := parseX18(d.BidPrice)
		ask := parseX18(d.AskPrice)
		ts, _ := strconv.ParseInt(d.Timestamp, 10, 64)

		callback(&exchanges.Ticker{
			Symbol:    symbol,
			LastPrice: bid.Add(ask).Div(decimal.NewFromInt(2)), // approximate
			Bid:       bid,
			Ask:       ask,
			Timestamp: ts / 1e6,
		})
	})
}

// SubscribeOrderBook is a wrapper for SubscribeOrderBookInternal

// SubscribeOrderBookWC is a wrapper for SubscribeOrderBookInternal

func (a *SpotAdapter) SubscribeOrderBookInternal(ctx context.Context, symbol string, depth *int, callback exchanges.OrderBookCallback) error {
	if err := a.WsMarketConnected(ctx); err != nil {
		return err
	}
	id, err := a.getProductId(symbol)
	if err != nil {
		return err
	}

	formattedSymbol := a.FormatSymbol(symbol)

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

	_, cancel := context.WithCancel(context.Background())
	a.cancels[formattedSymbol] = cancel
	a.cancelMu.Unlock()

	// Snapshot fetching state management (local to this subscription)
	state := &struct {
		fetching bool
		mu       sync.Mutex
	}{}

	return a.wsMarket.SubscribeOrderBook(id, func(e *nado.OrderBook) {
		// Process update and check for errors (gap detection)
		err := ob.ProcessUpdate(e)
		if err != nil {
			// Gap detected, need to resync
			// TODO: logger.Warn("Orderbook gap detected, triggering resync",
			// 				// zap.String("symbol", symbol),
			// 				// zap.Error(err))

			state.mu.Lock()
			if !state.fetching {
				state.fetching = true
				go func() {
					// Fetch new snapshot
					ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
					defer cancel()

					snap, err := a.httpClient.GetMarketLiquidity(ctx, id, 100)
					if err != nil {
						// 						// TODO: logger.Error("Failed to fetch Nado orderbook snapshot", zap.String("symbol", symbol), zap.Error(err))
						state.mu.Lock()
						state.fetching = false
						state.mu.Unlock()
						return
					}

					// Apply snapshot and check for errors
					if err := ob.ApplySnapshot(snap); err != nil {
						// 						// TODO: logger.Error("Failed to apply Nado orderbook snapshot", zap.String("symbol", symbol), zap.Error(err))
						state.mu.Lock()
						state.fetching = false
						state.mu.Unlock()
						return
					}

					state.mu.Lock()
					state.fetching = false
					state.mu.Unlock()
					// 					// TODO: logger.Info("Nado OrderBook Snapshot initialized", zap.String("symbol", symbol))

					if depth != nil && callback != nil {
						callback(ob.ToAdapterOrderBook(*depth))
					}
				}()
			}
			state.mu.Unlock()
			return
		}

		if !ob.IsInitialized() {
			state.mu.Lock()
			if !state.fetching {
				state.fetching = true
				go func() {
					// Fetch initial snapshot
					ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
					defer cancel()

					snap, err := a.httpClient.GetMarketLiquidity(ctx, id, 100)
					if err != nil {
						// 						// TODO: logger.Error("Failed to fetch Nado orderbook snapshot", zap.String("symbol", symbol), zap.Error(err))
						state.mu.Lock()
						state.fetching = false
						state.mu.Unlock()
						return
					}

					// Apply snapshot and check for errors
					if err := ob.ApplySnapshot(snap); err != nil {
						// 						// TODO: logger.Error("Failed to apply Nado orderbook snapshot", zap.String("symbol", symbol), zap.Error(err))
						state.mu.Lock()
						state.fetching = false
						state.mu.Unlock()
						return
					}

					state.mu.Lock()
					state.fetching = false
					state.mu.Unlock()
					// 					// TODO: logger.Info("Nado OrderBook Snapshot initialized", zap.String("symbol", symbol))

					if depth != nil && callback != nil {
						callback(ob.ToAdapterOrderBook(*depth))
					}
				}()
			}
			state.mu.Unlock()
			return
		}

		if depth != nil && callback != nil {
			callback(ob.ToAdapterOrderBook(*depth))
		}
	})
}

func (a *SpotAdapter) StopWatchKlines(ctx context.Context, symbol string, interval exchanges.Interval) error {
	return nil
}

func (a *SpotAdapter) WatchTrades(ctx context.Context, symbol string, callback exchanges.TradeCallback) error {
	if err := a.WsMarketConnected(ctx); err != nil {
		return err
	}
	id, err := a.getProductId(symbol)
	if err != nil {
		return err
	}
	return a.wsMarket.SubscribeTrades(id, func(t *nado.Trade) {
		side := exchanges.TradeSideSell
		if t.IsTakerBuyer {
			side = exchanges.TradeSideBuy
		}
		price := parseX18(t.Price)
		qty := parseX18(t.TakerQty)
		ts, _ := strconv.ParseInt(t.Timestamp, 10, 64)

		callback(&exchanges.Trade{
			ID:        fmt.Sprintf("%d", t.ProductId),
			Symbol:    symbol,
			Price:     price,
			Quantity:  qty,
			Side:      side,
			Timestamp: ts / 1000,
		})
	})
}

func (a *SpotAdapter) mapOrder(o *nado.Order) *exchanges.Order {
	amount := parseX18(o.Amount)
	price := parseX18(o.PriceX18)

	side := exchanges.OrderSideBuy
	if amount.IsNegative() {
		side = exchanges.OrderSideSell
		amount = amount.Abs()
	}

	order := &exchanges.Order{
		OrderID:   o.Digest,
		Symbol:    a.idMap[o.ProductID],
		Side:      side,
		Quantity:  amount,
		Price:     price,
		Status:    exchanges.OrderStatusNew, // Nado REST open orders are active
		Timestamp: o.PlacedAt,
	}
	exchanges.DerivePartialFillStatus(order)
	return order
}

// WaitOrderBookReady waits for orderbook to be ready
func (a *SpotAdapter) WaitOrderBookReady(ctx context.Context, symbol string) error {
	formattedSymbol := a.FormatSymbol(symbol)
	return a.BaseAdapter.WaitOrderBookReady(ctx, formattedSymbol)
}

// GetLocalOrderBook get local orderbook
func (a *SpotAdapter) GetLocalOrderBook(symbol string, depth int) *exchanges.OrderBook {
	formattedSymbol := a.FormatSymbol(symbol)

	ob, ok := a.GetLocalOrderBookImplementation(formattedSymbol)
	if !ok {
		return nil
	}

	nadoOb := ob.(*OrderBook)
	if !nadoOb.IsInitialized() {
		return nil
	}

	bids, asks := nadoOb.GetDepth(depth)
	return &exchanges.OrderBook{
		Symbol:    symbol,
		Timestamp: nadoOb.Timestamp(),
		Bids:      bids,
		Asks:      asks,
	}
}

func (a *SpotAdapter) WatchOrderBook(ctx context.Context, symbol string, cb exchanges.OrderBookCallback) error {
	depth := 20
	if err := a.SubscribeOrderBookInternal(ctx, symbol, &depth, cb); err != nil {
		return err
	}
	formattedSymbol := a.FormatSymbol(symbol)
	return a.BaseAdapter.WaitOrderBookReady(ctx, formattedSymbol)
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

func (a *SpotAdapter) StopWatchOrders(ctx context.Context) error                { return nil }
func (a *SpotAdapter) StopWatchPositions(ctx context.Context) error             { return nil }
func (a *SpotAdapter) StopWatchTicker(ctx context.Context, symbol string) error { return nil }
func (a *SpotAdapter) StopWatchTrades(ctx context.Context, symbol string) error { return nil }

func (a *SpotAdapter) WatchKlines(ctx context.Context, symbol string, interval exchanges.Interval, callback exchanges.KlineCallback) error {
	return fmt.Errorf("not implemented")
}

func (a *SpotAdapter) FetchSymbolDetails(ctx context.Context, symbol string) (*exchanges.SymbolDetails, error) {
	details, err := a.GetSymbolDetail(symbol)
	if err != nil {
		return nil, fmt.Errorf("symbol not found: %s", symbol)
	}
	return details, nil
}

func (a *SpotAdapter) getProductId(symbol string) (int64, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if id, ok := a.productMap[symbol]; ok {
		return id, nil
	}
	return 0, fmt.Errorf("symbol not found: %s", symbol)
}

func (a *SpotAdapter) getSymbol(productID int64) string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if sym, ok := a.idMap[productID]; ok {
		return sym
	}
	return fmt.Sprintf("UNKNOWN-%d", productID)
}
