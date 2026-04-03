package edgex

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/edgex/sdk/perp"

	"github.com/shopspring/decimal"
)

type Adapter struct {
	*exchanges.BaseAdapter
	client    *perp.Client
	wsMarket  *perp.WsMarketClient
	wsAccount *perp.WsAccountClient

	starkPrivateKey string
	accountID       string

	isConnected bool
	mu          sync.RWMutex

	// Symbol mapping
	symbolToContract map[string]*perp.Contract // "BTCUSD" -> contract obj
	contractToSymbol map[string]string         // contractId -> "BTCUSD"
	coinIdToCoin     map[string]*perp.Coin     // coinId -> coin

	cancelMu sync.Mutex
	cancels  map[string]context.CancelFunc

	// Cached fee rates (fetched once via GetAccount)
	feeOnce      sync.Once
	feeCache     map[string]*exchanges.FeeRate // symbol -> fee rate (per-contract overrides)
	defaultFee   *exchanges.FeeRate
	cachedFeeErr error
}

// NewAdapter creates a new EdgeX adapter
// APIKey -> AccountID
// SecretKey -> StarkPrivateKey
func NewAdapter(ctx context.Context, opts Options) (*Adapter, error) {
	if _, err := opts.quoteCurrency(); err != nil {
		return nil, err
	}
	if err := opts.validateCredentials(); err != nil {
		return nil, err
	}

	client := perp.NewClient()
	if opts.PrivateKey != "" {
		client.WithCredentials(opts.PrivateKey, opts.AccountID)
	}
	wsMarket := perp.NewWsMarketClient(ctx)
	var wsAccount *perp.WsAccountClient
	if opts.PrivateKey != "" {
		wsAccount = perp.NewWsAccountClient(ctx, opts.PrivateKey, opts.AccountID)
	}

	base := exchanges.NewBaseAdapter("EDGEX", exchanges.MarketTypePerp, opts.logger())

	a := &Adapter{
		BaseAdapter:      base,
		client:           client,
		wsMarket:         wsMarket,
		wsAccount:        wsAccount,
		starkPrivateKey:  opts.PrivateKey,
		accountID:        opts.AccountID,
		symbolToContract: make(map[string]*perp.Contract),
		contractToSymbol: make(map[string]string),
		coinIdToCoin:     make(map[string]*perp.Coin),
		cancels:          make(map[string]context.CancelFunc),
	}

	// Load Exchange Info
	if err := a.RefreshSymbolDetails(context.Background()); err != nil {
		return nil, fmt.Errorf("edgex init: %w", err)
	}

	// TODO: logger.Info("Initialized Edgex Adapter", zap.String("accountID", opts.AccountID))
	return a, nil
}

func (a *Adapter) WsAccountConnected(ctx context.Context) error {
	if a.wsAccount == nil {
		return exchanges.NewExchangeError("EDGEX", "", "ws account not available (no credentials configured)", exchanges.ErrAuthFailed)
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

// EdgeX does not support WS private order placement in this adapter.
func (a *Adapter) WsOrderConnected(ctx context.Context) error {
	return nil
}

func (a *Adapter) RefreshSymbolDetails(ctx context.Context) error {
	info, err := a.client.GetExchangeInfo(ctx)
	if err != nil {
		return err
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	a.symbolToContract = make(map[string]*perp.Contract)
	a.contractToSymbol = make(map[string]string)
	symbols := make(map[string]*exchanges.SymbolDetails)

	for i := range info.ContractList {
		c := &info.ContractList[i]
		name := a.ExtractSymbol(c.ContractName)
		a.symbolToContract[name] = c
		a.contractToSymbol[c.ContractId] = name

		details := &exchanges.SymbolDetails{
			Symbol:            name,
			PricePrecision:    countStringDecimalPlaces(c.TickSize),
			QuantityPrecision: countStringDecimalPlaces(c.StepSize),
			MinQuantity:       parseEdgexFloat(c.MinOrderSize),
		}

		symbols[name] = details
	}
	a.SetSymbolDetails(symbols)

	a.coinIdToCoin = make(map[string]*perp.Coin)
	for i := range info.CoinList {
		c := &info.CoinList[i]
		a.coinIdToCoin[c.CoinId] = c
	}
	return nil
}

func (a *Adapter) FormatSymbol(symbol string) string {
	if strings.HasSuffix(symbol, "USD") {
		return symbol
	}
	return symbol + "USD"
}

func (a *Adapter) ExtractSymbol(symbol string) string {
	return strings.TrimSuffix(symbol, "USD")
}

func (a *Adapter) IsConnected(ctx context.Context) (bool, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.isConnected, nil
}

func (a *Adapter) Close() error {
	if a.wsMarket != nil {
		a.wsMarket.Close()
	}
	if a.wsAccount != nil {
		a.wsAccount.Close()
	}
	return nil
}

// ================= Account & Trading =================

func (a *Adapter) FetchAccount(ctx context.Context) (*exchanges.Account, error) {
	res, err := a.client.GetAccountAsset(ctx)
	if err != nil {
		return nil, err
	}

	account := &exchanges.Account{
		Positions: []exchanges.Position{},
		Orders:    []exchanges.Order{},
	}

	availBal := decimal.Zero
	totalBal := decimal.Zero

	for _, c := range res.CollateralAssetModelList {
		avail := parseEdgexFloat(c.AvailableAmount)
		total := parseEdgexFloat(c.TotalEquity)

		availBal = availBal.Add(avail)
		totalBal = totalBal.Add(total)
	}
	account.AvailableBalance = availBal
	account.TotalBalance = totalBal

	for _, p := range res.PositionList {
		size := parseEdgexFloat(p.OpenSize)
		if size.IsZero() {
			continue
		}

		side := exchanges.PositionSideLong
		if size.IsNegative() {
			side = exchanges.PositionSideShort
			size = size.Abs()
		}

		lev := decimal.Zero
		entryPrice := decimal.Zero
		liqPrice := decimal.Zero
		unPnl := decimal.Zero

		for _, pa := range res.PositionAssetList {
			if pa.ContractId == p.ContractId {
				lev = parseEdgexFloat(pa.MaxLeverage)
				entryPrice = parseEdgexFloat(pa.AvgEntryPrice)
				liqPrice = parseEdgexFloat(pa.LiquidatePrice)
				unPnl = parseEdgexFloat(pa.UnrealizePnl)

				if liqPrice.IsPositive() && entryPrice.IsPositive() {
					calcLev := decimal.Zero
					if side == exchanges.PositionSideLong {
						if entryPrice.GreaterThan(liqPrice) {
							calcLev = entryPrice.Div(entryPrice.Sub(liqPrice))
						}
					} else {
						if liqPrice.GreaterThan(entryPrice) {
							calcLev = entryPrice.Div(liqPrice.Sub(entryPrice))
						}
					}
					if calcLev.IsPositive() {
						lev = calcLev
					}
				}
				break
			}
		}

		symbol := p.ContractId
		if name, ok := a.contractToSymbol[p.ContractId]; ok {
			symbol = name
		}

		account.Positions = append(account.Positions, exchanges.Position{
			Symbol:           symbol,
			Side:             side,
			Quantity:         size,
			EntryPrice:       entryPrice,
			UnrealizedPnL:    unPnl,
			LiquidationPrice: liqPrice,
			MarginType:       "CROSSED",
			Leverage:         lev,
		})
	}

	// open orders
	orders, err := a.client.GetOpenOrders(ctx, nil)
	if err != nil {
		return nil, err
	}
	for _, o := range orders {
		order := a.mapOrder(&o)
		account.Orders = append(account.Orders, *order)
	}
	return account, nil
}

func (a *Adapter) FetchBalance(ctx context.Context) (decimal.Decimal, error) {
	acc, err := a.FetchAccount(ctx)
	if err != nil {
		return decimal.Zero, err
	}
	return acc.AvailableBalance, nil
}

func (a *Adapter) FetchPositions(ctx context.Context) ([]exchanges.Position, error) {
	acc, err := a.FetchAccount(ctx)
	if err != nil {
		return nil, err
	}
	return acc.Positions, nil
}

func (a *Adapter) PlaceOrder(ctx context.Context, params *exchanges.OrderParams) (*exchanges.Order, error) {
	// Auto-slippage: EdgeX rejects market orders with price=0
	if params.Type == exchanges.OrderTypeMarket && params.Slippage.IsZero() {
		params.Slippage = decimal.NewFromFloat(0.02)
	}
	// Apply slippage logic: converts MARKET+Slippage to LIMIT+IOC
	if err := a.BaseAdapter.ApplySlippage(ctx, params, a.FetchTicker); err != nil {
		return nil, err
	}
	// 1. Validation & Formatting
	details, err := a.FetchSymbolDetails(ctx, params.Symbol)
	if err == nil {
		if err := exchanges.ValidateAndFormatParams(params, details); err != nil {
			return nil, err
		}
	}

	side := "BUY"
	if params.Side == exchanges.OrderSideSell {
		side = "SELL"
	}

	c, ok := a.symbolToContract[params.Symbol]
	if !ok {
		return nil, fmt.Errorf("symbol not found: %s", params.Symbol)
	}

	req := perp.PlaceOrderParams{
		ContractId:    c.ContractId,
		Side:          side,
		Type:          string(params.Type),
		Quantity:      params.Quantity.String(),
		ClientOrderId: params.ClientID,
	}

	req.Type = a.mapOrderType(params.Type)

	if params.Price.IsPositive() {
		req.Price = params.Price.String()
	}

	if params.ReduceOnly {
		req.ReduceOnly = true
	}
	req.TimeInForce = a.mapTimeInForce(params.TimeInForce)

	q, ok := a.coinIdToCoin[c.QuoteCoinId]
	if !ok {
		return nil, fmt.Errorf("quote coin not found: %s", c.QuoteCoinId)
	}

	res, err := a.client.PlaceOrder(ctx, req, c, q)
	if err != nil {
		return nil, err
	}

	return &exchanges.Order{
		OrderID:       res.OrderId,
		ClientOrderID: params.ClientID,
		Symbol:        params.Symbol,
		Status:        exchanges.OrderStatusPending,
		Timestamp:     time.Now().UnixMilli(),
	}, nil
}

func (a *Adapter) PlaceOrderWS(context.Context, *exchanges.OrderParams) error {
	return exchanges.ErrNotSupported
}

func (a *Adapter) CancelOrder(ctx context.Context, orderID, symbol string) error {
	_, err := a.client.CancelOrder(ctx, orderID)
	return err
}

func (a *Adapter) CancelOrderWS(context.Context, string, string) error {
	return exchanges.ErrNotSupported
}

func (a *Adapter) ModifyOrder(ctx context.Context, orderID, symbol string, params *exchanges.ModifyOrderParams) (*exchanges.Order, error) {
	return nil, exchanges.ErrNotSupported
}

func (a *Adapter) ModifyOrderWS(context.Context, string, string, *exchanges.ModifyOrderParams) error {
	return exchanges.ErrNotSupported
}

func (a *Adapter) FetchOrderByID(ctx context.Context, orderID, symbol string) (*exchanges.Order, error) {
	orders, err := a.client.GetOrdersByIds(ctx, []string{orderID})
	if err != nil {
		return nil, err
	}
	if len(orders) > 0 {
		order := a.mapOrder(&orders[0])
		if symbol != "" && order.Symbol != symbol {
			return nil, exchanges.ErrOrderNotFound
		}
		return order, nil
	}
	return nil, exchanges.ErrOrderNotFound
}

func (a *Adapter) FetchOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	return nil, exchanges.ErrNotSupported
}

func (a *Adapter) FetchOpenOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	c, ok := a.symbolToContract[symbol]
	if !ok {
		return nil, fmt.Errorf("symbol not found: %s", symbol)
	}
	contractId := c.ContractId
	res, err := a.client.GetOpenOrders(ctx, &contractId)
	if err != nil {
		return nil, err
	}
	orders := make([]exchanges.Order, len(res))
	for i, r := range res {
		orders[i] = *a.mapOrder(&r)
	}
	return orders, nil
}

func (a *Adapter) CancelAllOrders(ctx context.Context, symbol string) error {
	return a.client.CancelAllOrders(ctx)
}

func (a *Adapter) SetLeverage(ctx context.Context, symbol string, leverage int) error {
	c, ok := a.symbolToContract[symbol]
	if !ok {
		return fmt.Errorf("symbol not found: %s", symbol)
	}
	return a.client.UpdateLeverageSetting(ctx, c.ContractId, leverage)
}

func (a *Adapter) FetchFeeRate(ctx context.Context, symbol string) (*exchanges.FeeRate, error) {
	a.feeOnce.Do(func() {
		account, err := a.client.GetAccount(ctx)
		if err != nil {
			a.cachedFeeErr = err
			return
		}
		// Account-level default
		a.defaultFee = &exchanges.FeeRate{
			Maker: parseEdgexFloat(account.DefaultTradeSetting.MakerFeeRate),
			Taker: parseEdgexFloat(account.DefaultTradeSetting.TakerFeeRate),
		}
		// Per-contract overrides
		a.feeCache = make(map[string]*exchanges.FeeRate)
		for contractId, cSetting := range account.ContractIdToTradeSetting {
			if !cSetting.IsSetFeeRate {
				continue
			}
			sym, ok := a.contractToSymbol[contractId]
			if !ok {
				continue
			}
			fee := &exchanges.FeeRate{
				Maker: a.defaultFee.Maker,
				Taker: a.defaultFee.Taker,
			}
			if cSetting.MakerFeeRate != "" {
				fee.Maker = parseEdgexFloat(cSetting.MakerFeeRate)
			}
			if cSetting.TakerFeeRate != "" {
				fee.Taker = parseEdgexFloat(cSetting.TakerFeeRate)
			}
			a.feeCache[sym] = fee
		}
	})
	if a.cachedFeeErr != nil {
		return nil, a.cachedFeeErr
	}
	// Check per-contract override first
	if fee, ok := a.feeCache[symbol]; ok {
		return fee, nil
	}
	return a.defaultFee, nil
}

// ================= Market Data =================

func (a *Adapter) FetchTicker(ctx context.Context, symbol string) (*exchanges.Ticker, error) {
	formattedSymbol := symbol
	c, ok := a.symbolToContract[formattedSymbol]
	if !ok {
		return nil, fmt.Errorf("symbol not found: %s", symbol)
	}

	t, err := a.client.GetTicker(ctx, c.ContractId)
	if err != nil {
		return nil, err
	}

	last := parseDecimal(t.LastPrice)
	high := parseDecimal(t.High)
	low := parseDecimal(t.Low)
	vol := parseDecimal(t.Size)

	return &exchanges.Ticker{
		Symbol:    symbol,
		LastPrice: last,
		High24h:   high,
		Low24h:    low,
		Volume24h: vol,
		Timestamp: time.Now().UnixMilli(),
	}, nil
}

func (a *Adapter) FetchOrderBook(ctx context.Context, symbol string, limit int) (*exchanges.OrderBook, error) {
	formattedSymbol := symbol
	c, ok := a.symbolToContract[formattedSymbol]
	if !ok {
		return nil, fmt.Errorf("symbol not found: %s", symbol)
	}

	level := 15
	if limit > 15 {
		level = 200
	}

	res, err := a.client.GetOrderBook(ctx, c.ContractId, level)
	if err != nil {
		return nil, err
	}

	bids := make([]exchanges.Level, len(res.Bids))
	asks := make([]exchanges.Level, len(res.Asks))

	for i, b := range res.Bids {
		bids[i] = exchanges.Level{Price: parseEdgexFloat(b.Price), Quantity: parseEdgexFloat(b.Size)}
	}
	for i, as := range res.Asks {
		asks[i] = exchanges.Level{Price: parseEdgexFloat(as.Price), Quantity: parseEdgexFloat(as.Size)}
	}

	return &exchanges.OrderBook{
		Symbol:    symbol,
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
	formattedSymbol := symbol
	c, ok := a.symbolToContract[formattedSymbol]
	if !ok {
		return nil, fmt.Errorf("symbol not found: %s", symbol)
	}

	exInterval := "MINUTE_1"
	switch interval {
	case exchanges.Interval1m:
		exInterval = "MINUTE_1"
	case exchanges.Interval5m:
		exInterval = "MINUTE_5"
	case exchanges.Interval15m:
		exInterval = "MINUTE_15"
	case exchanges.Interval1h:
		exInterval = "HOUR_1"
	case exchanges.Interval4h:
		exInterval = "HOUR_4"
	case exchanges.Interval1d:
		exInterval = "DAY_1"
	}

	var startTime, endTime string
	if start != nil {
		startTime = fmt.Sprintf("%d", start.UnixMilli())
	}
	if end != nil {
		endTime = fmt.Sprintf("%d", end.UnixMilli())
	}

	res, err := a.client.GetKline(ctx, c.ContractId, "LAST_PRICE", exInterval, limit, startTime, endTime, "")
	if err != nil {
		return nil, err
	}

	klines := make([]exchanges.Kline, len(res.DataList))
	for i, k := range res.DataList {
		ts, _ := strconv.ParseInt(k.KlineTime, 10, 64)
		klines[i] = exchanges.Kline{
			Symbol:    symbol,
			Interval:  interval,
			Timestamp: ts,
			Open:      parseEdgexFloat(k.Open),
			High:      parseEdgexFloat(k.High),
			Low:       parseEdgexFloat(k.Low),
			Close:     parseEdgexFloat(k.Close),
			Volume:    parseEdgexFloat(k.Size),
			QuoteVol:  parseEdgexFloat(k.Value),
		}
	}
	return klines, nil
}

func (a *Adapter) FetchTrades(ctx context.Context, symbol string, limit int) ([]exchanges.Trade, error) {
	return []exchanges.Trade{}, nil
}

// ================= WebSocket =================

func (a *Adapter) WatchOrders(ctx context.Context, callback exchanges.OrderUpdateCallback) error {
	if err := a.WsAccountConnected(ctx); err != nil {
		return err
	}
	a.wsAccount.SubscribeOrderUpdate(func(orders []perp.Order) {
		for _, o := range orders {
			callback(a.mapOrder(&o))
		}
	})
	return nil
}

func (a *Adapter) WatchPositions(ctx context.Context, callback exchanges.PositionUpdateCallback) error {
	if err := a.WsAccountConnected(ctx); err != nil {
		return err
	}
	a.wsAccount.SubscribePositionUpdate(func(positions []perp.PositionInfo) {
		for _, p := range positions {
			size := parseEdgexFloat(p.OpenSize)
			if size.IsZero() {
				continue
			}
			side := exchanges.PositionSideLong
			if size.IsNegative() {
				side = exchanges.PositionSideShort
				size = size.Abs()
			}
			openValue := parseEdgexFloat(p.OpenValue)
			entryPrice := openValue.Div(size)
			if entryPrice.IsNegative() {
				entryPrice = entryPrice.Abs()
			}

			symbol := p.ContractId
			if name, ok := a.contractToSymbol[p.ContractId]; ok {
				symbol = name
			}

			callback(&exchanges.Position{
				Symbol:     symbol,
				Side:       side,
				Quantity:   size,
				EntryPrice: entryPrice,
				MarginType: "CROSSED",
			})
		}
	})
	return nil
}

func (a *Adapter) WatchTicker(ctx context.Context, symbol string, callback exchanges.TickerCallback) error {
	formattedSymbol := symbol
	if err := a.WsMarketConnected(ctx); err != nil {
		return err
	}
	c, ok := a.symbolToContract[formattedSymbol]
	if !ok {
		return fmt.Errorf("symbol not found: %s", symbol)
	}

	return a.wsMarket.SubscribeTicker(c.ContractId, func(e *perp.WsTickerEvent) {
		if len(e.Content.Data) > 0 {
			d := e.Content.Data[0]
			callback(&exchanges.Ticker{
				Symbol:    symbol,
				LastPrice: parseEdgexFloat(d.LastPrice),
				High24h:   parseEdgexFloat(d.High),
				Low24h:    parseEdgexFloat(d.Low),
				Volume24h: parseEdgexFloat(d.Size),
				Timestamp: time.Now().UnixMilli(),
			})
		}
	})
}

func (a *Adapter) WatchOrderBook(ctx context.Context, symbol string, depth int, callback exchanges.OrderBookCallback) error {
	if err := a.WsMarketConnected(ctx); err != nil {
		return err
	}
	c, ok := a.symbolToContract[symbol]
	if !ok {
		return fmt.Errorf("symbol not found: %s", symbol)
	}

	a.cancelMu.Lock()
	if a.cancels == nil {
		a.cancels = make(map[string]context.CancelFunc)
	}
	if cancel, ok := a.cancels[symbol]; ok {
		cancel()
	}

	ob := NewOrderBook(symbol)
	a.SetLocalOrderBook(symbol, ob)

	_, cancel := context.WithCancel(context.Background())
	a.cancels[symbol] = cancel
	a.cancelMu.Unlock()

	err := a.wsMarket.SubscribeOrderBook(c.ContractId, perp.OrderBookDepth200, func(e *perp.WsDepthEvent) {
		ob.ProcessPerpUpdate(e)
		if callback != nil {
			callback(ob.ToAdapterOrderBook(depth))
		}
	})
	if err != nil {
		return err
	}
	return a.BaseAdapter.WaitOrderBookReady(ctx, symbol)
}

func (a *Adapter) WatchKlines(ctx context.Context, symbol string, interval exchanges.Interval, callback exchanges.KlineCallback) error {
	formattedSymbol := symbol
	if err := a.WsMarketConnected(ctx); err != nil {
		return err
	}
	c, ok := a.symbolToContract[formattedSymbol]
	if !ok {
		return fmt.Errorf("symbol not found: %s", symbol)
	}

	exInterval := perp.KlineInterval1m
	// map interval... same as above
	switch interval {
	case exchanges.Interval1m:
		exInterval = perp.KlineInterval1m
	case exchanges.Interval5m:
		exInterval = perp.KlineInterval5m
	case exchanges.Interval15m:
		exInterval = perp.KlineInterval15m
	case exchanges.Interval1h:
		exInterval = perp.KlineInterval1h
	case exchanges.Interval4h:
		exInterval = perp.KlineInterval4h
	case exchanges.Interval1d:
		exInterval = perp.KlineInterval1d
	}

	return a.wsMarket.SubscribeKline(c.ContractId, perp.PriceTypeLastPrice, exInterval, func(e *perp.WsKlineEvent) {
		if len(e.Content.Data) > 0 {
			d := e.Content.Data[0]
			ts, _ := strconv.ParseInt(d.KlineTime, 10, 64)
			callback(&exchanges.Kline{
				Symbol:    symbol,
				Interval:  interval,
				Timestamp: ts,
				Open:      parseEdgexFloat(d.Open),
				High:      parseEdgexFloat(d.High),
				Low:       parseEdgexFloat(d.Low),
				Close:     parseEdgexFloat(d.Close),
				Volume:    parseEdgexFloat(d.Size),
				QuoteVol:  parseEdgexFloat(d.Value),
			})
		}
	})
}

func (a *Adapter) WatchTrades(ctx context.Context, symbol string, callback exchanges.TradeCallback) error {
	formattedSymbol := symbol
	if err := a.WsMarketConnected(ctx); err != nil {
		return err
	}
	c, ok := a.symbolToContract[formattedSymbol]
	if !ok {
		return fmt.Errorf("symbol not found: %s", symbol)
	}

	return a.wsMarket.SubscribeTrades(c.ContractId, func(e *perp.WsTradeEvent) {
		for _, d := range e.Content.Data {
			side := exchanges.TradeSideBuy
			if d.IsBuyerMaker {
				side = exchanges.TradeSideSell
			}
			ts, _ := strconv.ParseInt(d.Time, 10, 64)
			callback(&exchanges.Trade{
				ID:        d.TicketId,
				Symbol:    symbol,
				Price:     parseEdgexFloat(d.Price),
				Quantity:  parseEdgexFloat(d.Size),
				Side:      side,
				Timestamp: ts,
			})
		}
	})
}

func (a *Adapter) StopWatchOrders(ctx context.Context) error { return nil }
func (a *Adapter) WatchFills(ctx context.Context, callback exchanges.FillCallback) error {
	if err := a.WsAccountConnected(ctx); err != nil {
		return err
	}
	a.wsAccount.SubscribeOrderFillUpdate(func(fills []perp.OrderFillTransaction) {
		for i := range fills {
			callback(a.mapOrderFill(&fills[i]))
		}
	})
	return nil
}
func (a *Adapter) StopWatchFills(ctx context.Context) error {
	_ = ctx
	if a.wsAccount == nil {
		return nil
	}
	a.wsAccount.Unsubscribe(perp.EventOrderFillFee)
	return nil
}
func (a *Adapter) StopWatchPositions(ctx context.Context) error             { return nil }
func (a *Adapter) StopWatchTicker(ctx context.Context, symbol string) error { return nil }
func (a *Adapter) StopWatchOrderBook(ctx context.Context, symbol string) error {
	formattedSymbol := symbol
	c, ok := a.symbolToContract[formattedSymbol]
	if !ok {
		return fmt.Errorf("symbol not found: %s", symbol)
	}

	a.cancelMu.Lock()
	if cancel, ok := a.cancels[formattedSymbol]; ok {
		cancel()
		delete(a.cancels, formattedSymbol)
	}
	a.cancelMu.Unlock()
	a.RemoveLocalOrderBook(formattedSymbol)

	// Use OrderBookDepth15 as used in SubscribeOrderBook
	return a.wsMarket.UnsubscribeOrderBook(c.ContractId, perp.OrderBookDepth15)
}
func (a *Adapter) StopWatchKlines(ctx context.Context, symbol string, interval exchanges.Interval) error {
	formattedSymbol := symbol
	c, ok := a.symbolToContract[formattedSymbol]
	if !ok {
		return fmt.Errorf("symbol not found: %s", symbol)
	}

	exInterval := perp.KlineInterval1m
	switch interval {
	case exchanges.Interval1m:
		exInterval = perp.KlineInterval1m
	case exchanges.Interval5m:
		exInterval = perp.KlineInterval5m
	case exchanges.Interval15m:
		exInterval = perp.KlineInterval15m
	case exchanges.Interval1h:
		exInterval = perp.KlineInterval1h
	case exchanges.Interval4h:
		exInterval = perp.KlineInterval4h
	case exchanges.Interval1d:
		exInterval = perp.KlineInterval1d
	}
	return a.wsMarket.UnsubscribeKline(c.ContractId, perp.PriceTypeLastPrice, exInterval)
}
func (a *Adapter) StopWatchTrades(ctx context.Context, symbol string) error {
	formattedSymbol := symbol
	c, ok := a.symbolToContract[formattedSymbol]
	if !ok {
		return fmt.Errorf("symbol not found: %s", symbol)
	}
	return a.wsMarket.UnsubscribeTrades(c.ContractId)
}

func (a *Adapter) FetchSymbolDetails(ctx context.Context, symbol string) (*exchanges.SymbolDetails, error) {
	details, err := a.GetSymbolDetail(symbol)
	if err != nil {
		return nil, fmt.Errorf("symbol not found in cache: %s", symbol)
	}
	return details, nil
}

// Helpers

func (a *Adapter) mapOrder(o *perp.Order) *exchanges.Order {
	p := parseEdgexFloat(o.Price)
	q := parseEdgexFloat(o.Size)
	ts, _ := strconv.ParseInt(o.CreatedTime, 10, 64)

	side := exchanges.OrderSideBuy
	if o.Side == perp.SideSell {
		side = exchanges.OrderSideSell
	}

	status := exchanges.OrderStatusUnknown
	// Need to map status string if possible, edgex returns strings like WORKING, FILLED
	switch o.Status {
	case perp.OrderStatusPending, perp.OrderStatusOpen:
		status = exchanges.OrderStatusNew
	case perp.OrderStatusFilled:
		status = exchanges.OrderStatusFilled
	case perp.OrderStatusCanceled, perp.OrderStatusCanceling:
		status = exchanges.OrderStatusCancelled
	}

	orderType := exchanges.OrderTypeUnknown
	switch o.Type {
	case perp.OrderTypeLimit:
		orderType = exchanges.OrderTypeLimit
	case perp.OrderTypeMarket:
		orderType = exchanges.OrderTypeMarket
	case perp.OrderTypeStopLimit:
		orderType = exchanges.OrderTypeStopLossLimit
	case perp.OrderTypeStopMarket:
		orderType = exchanges.OrderTypeStopLossMarket
	case perp.OrderTypeTakeProfitLimit:
		orderType = exchanges.OrderTypeTakeProfitLimit
	case perp.OrderTypeTakeProfitMarket:
		orderType = exchanges.OrderTypeTakeProfitMarket
	}

	// EdgeX has two-phase order updates:
	// 1. Match: cumMatchSize populated, cumFillSize = 0
	// 2. Fill: cumFillSize populated
	// Use max(cumMatchSize, cumFillSize) to ensure we capture the latest matched quantity
	// even if the fill (settlement) event lags behind.
	fillSize := parseEdgexFloat(o.CumFillSize)
	matchSize := parseEdgexFloat(o.CumMatchSize)

	filledQty := fillSize
	if matchSize.GreaterThan(fillSize) {
		filledQty = matchSize
	}

	order := &exchanges.Order{
		OrderID:        o.Id,
		ClientOrderID:  o.ClientOrderId,
		Symbol:         a.contractToSymbol[o.ContractId],
		Side:           side,
		Type:           orderType,
		Price:          p,
		Quantity:       q,
		FilledQuantity: filledQty,
		Status:         status,
		Timestamp:      ts,
	}
	exchanges.DerivePartialFillStatus(order)
	return order
}

func (a *Adapter) mapOrderFill(fill *perp.OrderFillTransaction) *exchanges.Fill {
	side := exchanges.OrderSideBuy
	if fill.OrderSide == string(perp.SideSell) {
		side = exchanges.OrderSideSell
	}

	timestamp, _ := strconv.ParseInt(fill.MatchTime, 10, 64)
	if timestamp == 0 {
		timestamp, _ = strconv.ParseInt(fill.CreatedTime, 10, 64)
	}

	feeAsset := ""
	if coin, ok := a.coinIdToCoin[fill.CoinId]; ok && coin != nil {
		feeAsset = coin.CoinName
	}

	return &exchanges.Fill{
		TradeID:   fill.MatchFillId,
		OrderID:   fill.OrderId,
		Symbol:    a.contractToSymbol[fill.ContractId],
		Side:      side,
		Price:     parseEdgexFloat(fill.FillPrice),
		Quantity:  parseEdgexFloat(fill.FillSize),
		Fee:       parseEdgexFloat(fill.FillFee),
		FeeAsset:  feeAsset,
		Timestamp: timestamp,
	}
}

func (a *Adapter) mapOrderType(t exchanges.OrderType) string {
	if t == exchanges.OrderTypeMarket {
		return "MARKET"
	}
	return "LIMIT"
}

func (a *Adapter) mapTimeInForce(tif exchanges.TimeInForce) string {
	if tif == "" {
		return ""
	}
	switch tif {
	case exchanges.TimeInForceIOC:
		return "IMMEDIATE_OR_CANCEL"
	case exchanges.TimeInForceFOK:
		return "FILL_OR_KILL"
	case exchanges.TimeInForcePO:
		return "POST_ONLY"
	case exchanges.TimeInForceGTC:
		return "GOOD_TIL_CANCEL"
	default:
		return string(tif)
	}
}

// GetLocalOrderBook get local orderbook
func (a *Adapter) GetLocalOrderBook(symbol string, depth int) *exchanges.OrderBook {
	ob, ok := a.GetLocalOrderBookImplementation(symbol)
	if !ok {
		return nil
	}

	edgexOb := ob.(*OrderBook)
	if !edgexOb.initialized {
		return nil
	}

	bids, asks := edgexOb.GetDepth(depth)
	return &exchanges.OrderBook{
		Symbol:    symbol,
		Timestamp: edgexOb.Timestamp(),
		Bids:      bids,
		Asks:      asks,
	}
}
