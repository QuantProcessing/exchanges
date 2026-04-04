package grvt

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/grvt/sdk"

	"github.com/shopspring/decimal"
)

// Adapter GRVT 适配器
type Adapter struct {
	*exchanges.BaseAdapter
	client        *grvt.Client
	wsMarket      *grvt.WebsocketClient
	wsAccount     *grvt.WebsocketClient
	wsTradeRpc    grvtTradeRPCClient
	apiKey        string
	privateKey    string
	subAccountID  uint64
	quoteCurrency string // "USDT" (currently only supported)

	isConnected bool

	instruments map[string]grvt.Instrument
	mu          sync.RWMutex

	cancelMu sync.Mutex
	cancels  map[string]context.CancelFunc

	// Cached fee rate (account-level, not per-symbol)
	feeOnce       sync.Once
	cachedFeeRate *exchanges.FeeRate
	cachedFeeErr  error
}

type grvtTradeRPCClient interface {
	Connect() error
	Close()
	PlaceOrder(ctx context.Context, req *grvt.OrderRequest) (*grvt.CreateOrderResponse, error)
	CancelOrder(ctx context.Context, req *grvt.CancelOrderRequest) (*grvt.CancelOrderResponse, error)
}

// NewAdapter creates a new GRVT adapter
func NewAdapter(ctx context.Context, opts Options) (*Adapter, error) {
	quote, err := opts.quoteCurrency()
	if err != nil {
		return nil, err
	}
	if err := opts.validateCredentials(); err != nil {
		return nil, err
	}

	pk := strings.TrimPrefix(opts.PrivateKey, "0x")
	client := grvt.NewClient()
	var saId uint64

	if opts.APIKey != "" {
		client.WithCredentials(opts.APIKey, opts.SubAccountID, pk)
		var err error
		saId, err = strconv.ParseUint(opts.SubAccountID, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("grvt init: invalid sub account id: %w", err)
		}
	}

	marketWs := grvt.NewMarketWebsocketClient(ctx, client)
	accountWs := grvt.NewAccountWebsocketClient(ctx, client)
	tradeRpcWs := grvt.NewAccountRpcWebsocketClient(ctx, client)

	base := exchanges.NewBaseAdapter("GRVT", exchanges.MarketTypePerp, opts.logger())

	a := &Adapter{
		BaseAdapter:   base,
		client:        client,
		wsMarket:      marketWs,
		wsAccount:     accountWs,
		wsTradeRpc:    tradeRpcWs,
		apiKey:        opts.APIKey,
		privateKey:    opts.PrivateKey,
		subAccountID:  saId,
		quoteCurrency: string(quote),
		instruments:   make(map[string]grvt.Instrument),
		cancels:       make(map[string]context.CancelFunc),
	}

	// Load Instruments
	if err := a.RefreshSymbolDetails(context.Background()); err != nil {
		return nil, fmt.Errorf("grvt init: %w", err)
	}

	// TODO: logger.Info("Initialized GRVT Adapter", zap.String("subAccountID", opts.SubAccountID))
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

// grvt not support ws order place
func (a *Adapter) WsOrderConnected(ctx context.Context) error {
	if err := a.requirePrivateAccess(); err != nil {
		return err
	}
	if a.wsTradeRpc == nil {
		return fmt.Errorf("grvt: trade rpc client not configured")
	}
	if err := a.wsTradeRpc.Connect(); err != nil {
		return err
	}
	return nil
}

func (a *Adapter) IsConnected(ctx context.Context) (bool, error) {
	return a.isConnected, nil
}

func (a *Adapter) Close() error {
	a.wsMarket.Close()
	a.wsAccount.Close()
	if a.wsTradeRpc != nil {
		a.wsTradeRpc.Close()
	}
	return nil
}

// ================= Account & Trading =================

func (a *Adapter) FetchAccount(ctx context.Context) (*exchanges.Account, error) {
	if err := a.requirePrivateAccess(); err != nil {
		return nil, err
	}
	res, err := a.client.GetAccountSummary(ctx)
	if err != nil {
		return nil, err
	}

	avail := parseGrvtFloat(res.Result.AvailableBalance)
	total := parseGrvtFloat(res.Result.TotalEquity)

	account := &exchanges.Account{
		TotalBalance:     total,
		AvailableBalance: avail,
		UnrealizedPnL:    parseGrvtFloat(res.Result.UnrealizedPnl),
		Positions:        []exchanges.Position{},
		Orders:           []exchanges.Order{},
	}

	for _, p := range res.Result.Position {
		size := parseGrvtFloat(p.Size)
		if size.IsZero() {
			continue
		}

		side := exchanges.PositionSideLong
		if size.IsNegative() {
			side = exchanges.PositionSideShort
			size = size.Abs()
		}

		account.Positions = append(account.Positions, exchanges.Position{
			Symbol:           a.ExtractSymbol(p.Instrument),
			Side:             side,
			Quantity:         size,
			EntryPrice:       parseGrvtFloat(p.EntryPrice),
			UnrealizedPnL:    parseGrvtFloat(p.UnrealizedPnl),
			RealizedPnL:      parseGrvtFloat(p.RealizedPnl),
			LiquidationPrice: parseGrvtFloat(p.EstLiquidationPrice),
			MarginType:       "CROSSED",
			Leverage:         parseGrvtFloat(p.Leverage),
		})
	}

	// open orders
	orders, err := a.client.GetOpenOrders(ctx, "")
	if err != nil {
		return nil, err
	}
	for _, o := range orders {
		account.Orders = append(account.Orders, exchanges.Order{
			OrderID:       o.OrderID,
			ClientOrderID: o.OrderID,
			Symbol:        a.ExtractSymbol(o.Legs[0].Instrument),
			Side:          exchanges.OrderSideBuy,
			Type:          exchanges.OrderTypeLimit,
			Quantity:      parseGrvtFloat(o.Legs[0].Size),
			Price:         parseGrvtFloat(o.Legs[0].LimitPrice),
			TimeInForce:   exchanges.TimeInForce(o.TimeInForce),
		})
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

// PlaceOrder places an order on GRVT.
// NOTE: GRVT supports native market orders (IsMarket=true, LimitPrice="0"),
// so we skip ApplySlippage entirely. GRVT has strict price protection bands
// that reject LIMIT+IOC orders exceeding the band (~1-2%).
func (a *Adapter) PlaceOrder(ctx context.Context, params *exchanges.OrderParams) (*exchanges.Order, error) {
	if err := a.WsOrderConnected(ctx); err != nil {
		return nil, err
	}
	req, err := a.buildOrderRequest(ctx, params)
	if err != nil {
		return nil, err
	}
	res, err := a.wsTradeRpc.PlaceOrder(ctx, req)
	if err != nil {
		return nil, err
	}
	return a.mapGrvtOrder(&res.Result), nil
}

func (a *Adapter) PlaceOrderWS(ctx context.Context, params *exchanges.OrderParams) error {
	if err := a.requirePrivateAccess(); err != nil {
		return err
	}
	if strings.TrimSpace(params.ClientID) == "" {
		return fmt.Errorf("client id required for PlaceOrderWS")
	}

	if err := a.WsOrderConnected(ctx); err != nil {
		return err
	}
	req, err := a.buildOrderRequest(ctx, params)
	if err != nil {
		return err
	}
	_, err = a.wsTradeRpc.PlaceOrder(ctx, req)
	return err
}

func (a *Adapter) buildOrderRequest(ctx context.Context, params *exchanges.OrderParams) (*grvt.OrderRequest, error) {
	details, err := a.FetchSymbolDetails(ctx, params.Symbol)
	if err == nil {
		if err := exchanges.ValidateAndFormatParams(params, details); err != nil {
			return nil, err
		}
	}

	if params.ClientID == "" {
		params.ClientID = fmt.Sprintf("%d", rand.Int63())
	}

	tif := grvt.GTT
	switch params.TimeInForce {
	case exchanges.TimeInForceIOC:
		tif = grvt.IOC
	case exchanges.TimeInForceFOK:
		tif = grvt.FOK
	}

	return &grvt.OrderRequest{
		SubAccountID: a.subAccountID,
		IsMarket:     params.Type == exchanges.OrderTypeMarket,
		TimeInForce:  tif,
		PostOnly:     params.Type == exchanges.OrderTypePostOnly,
		ReduceOnly:   params.ReduceOnly,
		Legs: []grvt.OrderLeg{{
			Instrument:    a.FormatSymbol(params.Symbol),
			Size:          params.Quantity.String(),
			LimitPrice:    params.Price.String(),
			IsBuyintAsset: params.Side == exchanges.OrderSideBuy,
		}},
		Metadata: grvt.OrderMetadata{
			ClientOrderID: params.ClientID,
		},
	}, nil
}

func (a *Adapter) CancelOrder(ctx context.Context, orderID, symbol string) error {
	if err := a.requirePrivateAccess(); err != nil {
		return err
	}
	if err := a.WsOrderConnected(ctx); err != nil {
		return err
	}
	return a.cancelOrderRPC(ctx, orderID)
}

func (a *Adapter) CancelOrderWS(ctx context.Context, orderID, symbol string) error {
	if err := a.requirePrivateAccess(); err != nil {
		return err
	}
	if err := a.WsOrderConnected(ctx); err != nil {
		return err
	}
	return a.cancelOrderRPC(ctx, orderID)
}

func (a *Adapter) cancelOrderRPC(ctx context.Context, orderID string) error {
	saID := strconv.FormatUint(a.subAccountID, 10)
	req := &grvt.CancelOrderRequest{
		SubAccountID: saID,
		OrderID:      &orderID,
	}
	_, err := a.wsTradeRpc.CancelOrder(ctx, req)
	return err
}

func (a *Adapter) ModifyOrder(ctx context.Context, orderID, symbol string, params *exchanges.ModifyOrderParams) (*exchanges.Order, error) {
	return nil, exchanges.ErrNotSupported
}

func (a *Adapter) ModifyOrderWS(ctx context.Context, orderID, symbol string, params *exchanges.ModifyOrderParams) error {
	return exchanges.ErrNotSupported
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
	res, err := a.client.GetOpenOrders(ctx, a.FormatSymbol(symbol))
	if err != nil {
		return nil, err
	}
	var orders []exchanges.Order
	for _, o := range res {
		orders = append(orders, *a.mapOpenOrder(&o))
	}
	return orders, nil
}

func (a *Adapter) CancelAllOrders(ctx context.Context, symbol string) error {
	if err := a.requirePrivateAccess(); err != nil {
		return err
	}
	return a.client.CancelAllOrders(ctx)
}

func (a *Adapter) SetLeverage(ctx context.Context, symbol string, leverage int) error {
	if err := a.requirePrivateAccess(); err != nil {
		return err
	}
	res, err := a.client.SetLeverage(ctx, a.FormatSymbol(symbol), leverage)
	if err != nil {
		return err
	}
	if res.Success != "true" {
		return fmt.Errorf("set leverage failed: %s", res.Success)
	}
	return nil
}

func (a *Adapter) FetchFeeRate(ctx context.Context, symbol string) (*exchanges.FeeRate, error) {
	if err := a.requirePrivateAccess(); err != nil {
		return nil, err
	}
	a.feeOnce.Do(func() {
		account, err := a.client.GetFundingAccountSummary(ctx)
		if err != nil {
			a.cachedFeeErr = err
			return
		}
		a.cachedFeeRate = &exchanges.FeeRate{
			Maker: decimal.NewFromFloat(float64(account.Tier.FuturesMakerFee) / 1_000_000),
			Taker: decimal.NewFromFloat(float64(account.Tier.FuturesTakerFee) / 1_000_000),
		}
	})
	if a.cachedFeeErr != nil {
		return nil, a.cachedFeeErr
	}
	return a.cachedFeeRate, nil
}

// ================= Market Data =================

func (a *Adapter) FetchTicker(ctx context.Context, symbol string) (*exchanges.Ticker, error) {
	res, err := a.client.GetTicker(ctx, a.FormatSymbol(symbol))
	if err != nil {
		return nil, err
	}

	// Calc vols
	buyVolB := parseGrvtFloat(res.Result.BuyVolume24hB)
	sellVolB := parseGrvtFloat(res.Result.SellVolume24hB)

	buyVolQ := parseGrvtFloat(res.Result.BuyVolume24hQ)
	sellVolQ := parseGrvtFloat(res.Result.SellVolume24hQ)

	return &exchanges.Ticker{
		Symbol:    symbol,
		LastPrice: parseGrvtFloat(res.Result.LastPrice),
		Bid:       parseGrvtFloat(res.Result.BestBidPrice),
		Ask:       parseGrvtFloat(res.Result.BestAskPrice),
		High24h:   parseGrvtFloat(res.Result.HighPrice),
		Low24h:    parseGrvtFloat(res.Result.LowPrice),
		Volume24h: buyVolB.Add(sellVolB),
		QuoteVol:  buyVolQ.Add(sellVolQ),
		Timestamp: time.Now().UnixMilli(),
	}, nil
}

func (a *Adapter) FetchOrderBook(ctx context.Context, symbol string, limit int) (*exchanges.OrderBook, error) {
	// GRVT only supports depths of 10 or 50.
	reqLimit := 50
	if limit > 0 && limit <= 10 {
		reqLimit = 10
	}
	res, err := a.client.GetOrderBook(ctx, a.FormatSymbol(symbol), reqLimit)
	if err != nil {
		return nil, err
	}

	ts, _ := strconv.ParseInt(res.Result.EventTime, 10, 64)
	ob := &exchanges.OrderBook{
		Symbol:    symbol,
		Timestamp: ts / 1000000,
		Bids:      make([]exchanges.Level, len(res.Result.Bids)),
		Asks:      make([]exchanges.Level, len(res.Result.Asks)),
	}

	for i, b := range res.Result.Bids {
		ob.Bids[i] = exchanges.Level{Price: parseGrvtFloat(b.Price), Quantity: parseGrvtFloat(b.Size)}
	}
	for i, as := range res.Result.Asks {
		ob.Asks[i] = exchanges.Level{Price: parseGrvtFloat(as.Price), Quantity: parseGrvtFloat(as.Size)}
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
	formattedSymbol := a.FormatSymbol(symbol)
	exInterval := "1m"
	switch interval {
	case exchanges.Interval1m:
		exInterval = "1m"
	case exchanges.Interval5m:
		exInterval = "5m"
	case exchanges.Interval15m:
		exInterval = "15m"
	case exchanges.Interval1h:
		exInterval = "1h"
	case exchanges.Interval4h:
		exInterval = "4h"
	case exchanges.Interval1d:
		exInterval = "1d"
	}

	var startTime, endTime *string
	limit64 := int64(limit)
	if start != nil {
		ts := fmt.Sprintf("%d", start.UnixNano())
		startTime = &ts
	}
	if end != nil {
		ts := fmt.Sprintf("%d", end.UnixNano())
		endTime = &ts
	}

	res, err := a.client.GetKLine(ctx, formattedSymbol, exInterval, "TRADE", startTime, endTime, &limit64, nil)
	if err != nil {
		return nil, err
	}

	klines := make([]exchanges.Kline, len(res.Result))
	for i, k := range res.Result {
		ts, _ := strconv.ParseInt(k.OpenTime, 10, 64)
		klines[i] = exchanges.Kline{
			Symbol:    symbol,
			Interval:  interval,
			Timestamp: ts / 1000000,
			Open:      parseGrvtFloat(k.Open),
			High:      parseGrvtFloat(k.High),
			Low:       parseGrvtFloat(k.Low),
			Close:     parseGrvtFloat(k.Close),
			Volume:    parseGrvtFloat(k.VolumeB),
			QuoteVol:  parseGrvtFloat(k.VolumeQ),
		}
	}
	return klines, nil
}

func (a *Adapter) FetchTrades(ctx context.Context, symbol string, limit int) ([]exchanges.Trade, error) {
	res, err := a.client.GetTrade(ctx, a.FormatSymbol(symbol), limit)
	if err != nil {
		return nil, err
	}
	trades := make([]exchanges.Trade, len(res.Result))
	for i, t := range res.Result {
		side := exchanges.TradeSideSell
		if t.IsTakerBuyer {
			side = exchanges.TradeSideBuy
		}
		ts, _ := strconv.ParseInt(t.EventTime, 10, 64)
		trades[i] = exchanges.Trade{
			ID:        t.TradeId,
			Symbol:    symbol,
			Price:     parseGrvtFloat(t.Price),
			Quantity:  parseGrvtFloat(t.Size),
			Side:      side,
			Timestamp: ts / 1000000,
		}
	}
	return trades, nil
}

// ================= WebSocket =================

func (a *Adapter) WatchOrders(ctx context.Context, callback exchanges.OrderUpdateCallback) error {
	if err := a.WsAccountConnected(ctx); err != nil {
		return err
	}
	return a.wsAccount.SubscribeOrderUpdate("all", func(d grvt.WsFeeData[grvt.Order]) error {
		callback(a.mapGrvtOrderStream(&d.Feed))
		return nil
	})
}

func (a *Adapter) WatchPositions(ctx context.Context, callback exchanges.PositionUpdateCallback) error {
	if err := a.WsAccountConnected(ctx); err != nil {
		return err
	}
	return a.wsAccount.SubscribePositions("all", func(d grvt.WsFeeData[grvt.Position]) error {
		size := parseGrvtFloat(d.Feed.Size)
		side := exchanges.PositionSideLong
		if size.IsNegative() {
			side = exchanges.PositionSideShort
			size = size.Abs()
		}
		pos := &exchanges.Position{
			Symbol:           a.ExtractSymbol(d.Feed.Instrument),
			Side:             side,
			Quantity:         size,
			EntryPrice:       parseGrvtFloat(d.Feed.EntryPrice),
			UnrealizedPnL:    parseGrvtFloat(d.Feed.UnrealizedPnl),
			LiquidationPrice: parseGrvtFloat(d.Feed.EstLiquidationPrice),
			Leverage:         parseGrvtFloat(d.Feed.Leverage),
			MarginType:       "CROSSED",
		}
		callback(pos)
		return nil
	})
}

func (a *Adapter) WatchTicker(ctx context.Context, symbol string, callback exchanges.TickerCallback) error {
	if err := a.WsMarketConnected(ctx); err != nil {
		return err
	}
	instrument := a.FormatSymbol(symbol)
	return a.wsMarket.SubscribeTickerDelta(instrument, grvt.TickerDeltaRate100, func(d grvt.WsFeeData[grvt.Ticker]) error {
		callback(&exchanges.Ticker{
			Symbol:    symbol,
			LastPrice: parseGrvtFloat(d.Feed.LastPrice),
			Bid:       parseGrvtFloat(d.Feed.BestBidPrice),
			Ask:       parseGrvtFloat(d.Feed.BestAskPrice),
		})
		return nil
	})
}

// WatchOrderBook subscribes to orderbook updates and waits for the book to be ready.
func (a *Adapter) WatchOrderBook(ctx context.Context, symbol string, depth int, callback exchanges.OrderBookCallback) error {
	if err := a.WsMarketConnected(ctx); err != nil {
		return err
	}
	instrument := a.FormatSymbol(symbol)

	a.cancelMu.Lock()
	if a.cancels == nil {
		a.cancels = make(map[string]context.CancelFunc)
	}
	if cancel, ok := a.cancels[symbol]; ok {
		cancel()
	}

	ob := NewOrderBook(instrument)
	a.SetLocalOrderBook(symbol, ob)

	_, cancel := context.WithCancel(context.Background())
	a.cancels[symbol] = cancel
	a.cancelMu.Unlock()

	err := a.wsMarket.SubscribeOrderbookDelta(instrument, grvt.OrderBookDeltaRate50, func(e grvt.WsFeeData[grvt.OrderBook]) error {
		ob.ProcessUpdate(&e.Feed)
		if callback != nil {
			callback(ob.ToAdapterOrderBook(depth))
		}
		return nil
	})
	if err != nil {
		return err
	}
	return a.BaseAdapter.WaitOrderBookReady(ctx, symbol)
}

func (a *Adapter) WatchKlines(ctx context.Context, symbol string, interval exchanges.Interval, callback exchanges.KlineCallback) error {
	if err := a.WsMarketConnected(ctx); err != nil {
		return err
	}
	formattedSymbol := a.FormatSymbol(symbol)
	var kInterval grvt.KlineInterval
	switch interval {
	case exchanges.Interval1m:
		kInterval = grvt.KlineInterval1m
	case exchanges.Interval5m:
		kInterval = grvt.KlineInterval5m
	case exchanges.Interval15m:
		kInterval = grvt.KlineInterval15m
	case exchanges.Interval1h:
		kInterval = grvt.KlineInterval1h
	case exchanges.Interval4h:
		kInterval = grvt.KlineInterval4h
	case exchanges.Interval1d:
		kInterval = grvt.KlineInterval1d
	default:
		return fmt.Errorf("unsupported interval")
	}

	return a.wsMarket.SubscribeKline(formattedSymbol, kInterval, grvt.KlineTypeTrade, func(e grvt.WsFeeData[grvt.KLine]) error {
		d := e.Feed
		ts, _ := strconv.ParseInt(d.OpenTime, 10, 64)
		callback(&exchanges.Kline{
			Symbol:    symbol,
			Interval:  interval,
			Timestamp: ts / 1000000,
			Open:      parseGrvtFloat(d.Open),
			High:      parseGrvtFloat(d.High),
			Low:       parseGrvtFloat(d.Low),
			Close:     parseGrvtFloat(d.Close),
			Volume:    parseGrvtFloat(d.VolumeB),
			QuoteVol:  parseGrvtFloat(d.VolumeQ),
		})
		return nil
	})
}

func (a *Adapter) WatchTrades(ctx context.Context, symbol string, callback exchanges.TradeCallback) error {
	if err := a.WsMarketConnected(ctx); err != nil {
		return err
	}
	formattedSymbol := a.FormatSymbol(symbol)
	return a.wsMarket.SubscribeTrade(formattedSymbol, grvt.TradeLimit50, func(e grvt.WsFeeData[grvt.Trade]) error {
		d := e.Feed
		side := exchanges.TradeSideSell
		if d.IsTakerBuyer {
			side = exchanges.TradeSideBuy
		}

		ts, _ := strconv.ParseInt(d.EventTime, 10, 64)

		callback(&exchanges.Trade{
			ID:        d.TradeId,
			Symbol:    symbol,
			Price:     parseGrvtFloat(d.Price),
			Quantity:  parseGrvtFloat(d.Size),
			Side:      side,
			Timestamp: ts / 1000000,
		})
		return nil
	})
}

func (a *Adapter) StopWatchOrders(ctx context.Context) error { return nil }
func (a *Adapter) WatchFills(ctx context.Context, callback exchanges.FillCallback) error {
	if err := a.WsAccountConnected(ctx); err != nil {
		return err
	}
	return a.wsAccount.SubscribeFill("all", func(d grvt.WsFeeData[grvt.WsFill]) error {
		callback(a.mapGrvtFill(&d.Feed))
		return nil
	})
}
func (a *Adapter) StopWatchFills(ctx context.Context) error {
	_ = ctx
	if a.wsAccount == nil || a.subAccountID == 0 {
		return nil
	}
	return a.wsAccount.Unsubscribe(grvt.StreamFill, fmt.Sprintf("%d", a.subAccountID))
}
func (a *Adapter) StopWatchPositions(ctx context.Context) error             { return nil }
func (a *Adapter) StopWatchTicker(ctx context.Context, symbol string) error { return nil }
func (a *Adapter) StopWatchOrderBook(ctx context.Context, symbol string) error {
	formattedSymbol := a.FormatSymbol(symbol)
	a.cancelMu.Lock()
	if cancel, ok := a.cancels[symbol]; ok {
		cancel()
		delete(a.cancels, symbol)
	}
	a.cancelMu.Unlock()
	a.RemoveLocalOrderBook(symbol)
	// Use UnsubscribeOrderbookDelta matching SubscribeOrderBook
	return a.wsMarket.UnsubscribeOrderbookDelta(formattedSymbol, grvt.OrderBookDeltaRate50)
}
func (a *Adapter) StopWatchKlines(ctx context.Context, symbol string, interval exchanges.Interval) error {
	formattedSymbol := a.FormatSymbol(symbol)
	var kInterval grvt.KlineInterval
	switch interval {
	case exchanges.Interval1m:
		kInterval = grvt.KlineInterval1m
	case exchanges.Interval5m:
		kInterval = grvt.KlineInterval5m
	case exchanges.Interval15m:
		kInterval = grvt.KlineInterval15m
	case exchanges.Interval1h:
		kInterval = grvt.KlineInterval1h
	case exchanges.Interval4h:
		kInterval = grvt.KlineInterval4h
	case exchanges.Interval1d:
		kInterval = grvt.KlineInterval1d
	default:
		return fmt.Errorf("unsupported interval")
	}
	return a.wsMarket.UnsubscribeKline(formattedSymbol, kInterval, grvt.KlineTypeTrade)
}
func (a *Adapter) StopWatchTrades(ctx context.Context, symbol string) error {
	formattedSymbol := a.FormatSymbol(symbol)
	return a.wsMarket.UnsubscribeTrade(formattedSymbol, grvt.TradeLimit50)
}

func (a *Adapter) FetchSymbolDetails(ctx context.Context, symbol string) (*exchanges.SymbolDetails, error) {
	details, err := a.GetSymbolDetail(symbol)
	if err != nil {
		return nil, fmt.Errorf("symbol not found in cache: %s", symbol)
	}
	return details, nil
}

func (a *Adapter) RefreshSymbolDetails(ctx context.Context) error {
	instruments, err := a.client.GetInstruments(ctx)
	if err != nil {
		return err
	}

	a.mu.Lock()
	a.instruments = make(map[string]grvt.Instrument)
	symbols := make(map[string]*exchanges.SymbolDetails)
	for _, instrument := range instruments {
		if instrument.Quote != a.quoteCurrency || instrument.Kind != "PERPETUAL" {
			continue
		}
		a.instruments[instrument.Instrument] = instrument

		details := &exchanges.SymbolDetails{
			Symbol:            instrument.Instrument,
			PricePrecision:    exchanges.CountDecimalPlaces(instrument.TickSize),
			QuantityPrecision: exchanges.CountDecimalPlaces(instrument.MinSize),
			MinQuantity:       parseGrvtFloat(instrument.MinSize),
		}
		symbols[instrument.Base] = details
	}
	a.mu.Unlock()
	a.SetSymbolDetails(symbols)
	return nil
}

func (a *Adapter) FormatSymbol(symbol string) string {
	suffix := "_" + a.quoteCurrency + "_Perp"
	if strings.HasSuffix(symbol, suffix) {
		return symbol
	}
	return symbol + suffix
}

func (a *Adapter) ExtractSymbol(instrument string) string {
	return strings.TrimSuffix(instrument, "_"+a.quoteCurrency+"_Perp")
}

// Helpers
func (a *Adapter) mapGrvtOrder(o *grvt.Order) *exchanges.Order {
	side := exchanges.OrderSideSell
	var instrument, qty string
	orderID := o.OrderID
	if orderID == "0x00" {
		orderID = ""
	}

	// size is state.tradedSize[-1]
	if len(o.Legs) > 0 {
		if o.Legs[0].IsBuyintAsset {
			side = exchanges.OrderSideBuy
		}
		instrument = o.Legs[0].Instrument
		qty = o.Legs[0].Size
	}

	// price is state.avgFillPrice[-1]
	avgPrice := decimal.Zero
	for _, fill := range o.State.AvgFillPrice {
		fillPrice, _ := decimal.NewFromString(fill)
		avgPrice = avgPrice.Add(fillPrice)
	}
	if len(o.State.AvgFillPrice) > 0 {
		avgPrice = avgPrice.Div(decimal.NewFromInt(int64(len(o.State.AvgFillPrice))))
	}

	filledQuantity := decimal.Zero
	for _, fill := range o.State.TradedSize {
		fillSize, _ := decimal.NewFromString(fill)
		filledQuantity = filledQuantity.Add(fillSize)
	}

	status := exchanges.OrderStatusUnknown
	switch o.State.Status {
	case grvt.OrderStatusPending, grvt.OrderStatusOpen:
		status = exchanges.OrderStatusNew
	case grvt.OrderStatusFilled:
		status = exchanges.OrderStatusFilled
	case grvt.OrderStatusCancelled:
		status = exchanges.OrderStatusCancelled
	case grvt.OrderStatusRejected:
		status = exchanges.OrderStatusRejected
	}

	order := &exchanges.Order{
		OrderID:          orderID,
		ClientOrderID:    o.Metadata.ClientOrderID,
		Symbol:           a.ExtractSymbol(instrument),
		Side:             side,
		Quantity:         parseGrvtFloat(qty),
		FilledQuantity:   filledQuantity,
		Price:            avgPrice,
		OrderPrice:       parseGrvtFloat(firstGrvtLimitPrice(o.Legs)),
		AverageFillPrice: avgPrice,
		Status:           status,
		Timestamp:        parseGrvtTimestamp(o.Metadata.CreatedTime),
	}
	exchanges.DerivePartialFillStatus(order)
	return order
}

func firstGrvtLimitPrice(legs []grvt.OrderLeg) string {
	if len(legs) == 0 {
		return ""
	}
	return legs[0].LimitPrice
}

func (a *Adapter) mapOpenOrder(o *grvt.Order) *exchanges.Order {
	return a.mapGrvtOrder(o) // same struct
}

func (a *Adapter) mapGrvtOrderStream(o *grvt.Order) *exchanges.Order {
	order := a.mapGrvtOrder(o)
	order.Price = order.OrderPrice
	order.AverageFillPrice = decimal.Zero
	order.LastFillPrice = decimal.Zero
	order.LastFillQuantity = decimal.Zero
	order.Fee = decimal.Zero
	return order
}

// WaitOrderBookReady waits for orderbook to be ready
func (a *Adapter) WaitOrderBookReady(ctx context.Context, symbol string) error {
	return a.BaseAdapter.WaitOrderBookReady(ctx, symbol)
}

// GetLocalOrderBook get local orderbook
func (a *Adapter) GetLocalOrderBook(symbol string, depth int) *exchanges.OrderBook {
	ob, ok := a.GetLocalOrderBookImplementation(symbol)
	if !ok {
		return nil
	}

	grvtOb := ob.(*OrderBook)
	if !grvtOb.IsInitialized() {
		return nil
	}

	bids, asks := grvtOb.GetDepth(depth)
	return &exchanges.OrderBook{
		Symbol:    symbol,
		Timestamp: grvtOb.Timestamp(),
		Bids:      bids,
		Asks:      asks,
	}
}

func (a *Adapter) mapGrvtFill(f *grvt.WsFill) *exchanges.Fill {
	side := exchanges.OrderSideBuy
	if !f.IsBuyer {
		side = exchanges.OrderSideSell
	}

	ts, _ := strconv.ParseInt(f.EventTime, 10, 64)

	return &exchanges.Fill{
		TradeID:       f.TradeID,
		OrderID:       f.OrderID,
		ClientOrderID: f.ClientOrderID,
		Symbol:        a.ExtractSymbol(f.Instrument),
		Side:          side,
		Price:         parseGrvtFloat(f.Price),
		Quantity:      parseGrvtFloat(f.Size),
		Fee:           parseGrvtFloat(f.Fee),
		IsMaker:       !f.IsTaker,
		Timestamp:     ts / 1000000,
	}
}

func (a *Adapter) requirePrivateAccess() error {
	if a.apiKey == "" || a.privateKey == "" || a.subAccountID == 0 {
		return exchanges.NewExchangeError("GRVT", "", "private API not available (no credentials configured)", exchanges.ErrAuthFailed)
	}
	return nil
}
