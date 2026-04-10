package bybit

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/bybit/sdk"
	"github.com/shopspring/decimal"
)

type Adapter struct {
	*exchanges.BaseAdapter
	client    marketClient
	publicWS  publicWSClient
	privateWS privateWSClient
	tradeWS   tradeWSClient
	markets   *marketCache
	quote     exchanges.QuoteCurrency
	cancel    context.CancelFunc
	cancels   map[string]context.CancelFunc
	mu        sync.RWMutex
}

func NewAdapter(ctx context.Context, opts Options) (*Adapter, error) {
	quote, err := opts.quoteCurrency()
	if err != nil {
		return nil, err
	}

	lifecycleCtx, cancel := context.WithCancel(ctx)
	adp, err := newPerpAdapterWithClient(lifecycleCtx, cancel, opts, quote, sdk.NewClient().WithCredentials(opts.APIKey, opts.SecretKey))
	if err != nil {
		cancel()
		return nil, err
	}
	return adp, nil
}

func newPerpAdapterWithClient(ctx context.Context, cancel context.CancelFunc, opts Options, quote exchanges.QuoteCurrency, client marketClient) (*Adapter, error) {
	if hasAnyCredentials(opts) && !hasFullCredentials(opts) {
		return nil, authError("bybit: api_key and secret_key must both be set together")
	}

	base := exchanges.NewBaseAdapter(exchangeName, exchanges.MarketTypePerp, opts.logger())
	instruments, err := client.GetInstruments(ctx, categoryLinear)
	if err != nil {
		return nil, err
	}

	markets := buildMarketCache(instruments, quote)
	base.SetSymbolDetails(buildSymbolDetails(instruments, quote))

	return &Adapter{
		BaseAdapter: base,
		client:      client,
		publicWS:    sdk.NewPublicWSClient(categoryLinear),
		privateWS:   sdk.NewPrivateWSClient().WithCredentials(opts.APIKey, opts.SecretKey),
		tradeWS:     sdk.NewTradeWSClient().WithCredentials(opts.APIKey, opts.SecretKey),
		markets:     markets,
		quote:       quote,
		cancel:      cancel,
		cancels:     make(map[string]context.CancelFunc),
	}, nil
}

func (a *Adapter) Close() error {
	if a.cancel != nil {
		a.cancel()
	}
	if a.publicWS != nil {
		_ = a.publicWS.Close()
	}
	if a.privateWS != nil {
		_ = a.privateWS.Close()
	}
	if a.tradeWS != nil {
		_ = a.tradeWS.Close()
	}
	return nil
}

func (a *Adapter) FormatSymbol(symbol string) string {
	upper := strings.ToUpper(symbol)
	if inst, ok := a.markets.byBase[upper]; ok {
		return inst.Symbol
	}
	return upper
}

func (a *Adapter) ExtractSymbol(symbol string) string {
	upper := strings.ToUpper(symbol)
	if inst, ok := a.markets.bySymbol[upper]; ok && inst.BaseCoin != "" {
		return strings.ToUpper(inst.BaseCoin)
	}
	return upper
}

func (a *Adapter) FetchTicker(ctx context.Context, symbol string) (*exchanges.Ticker, error) {
	raw, err := a.client.GetTicker(ctx, categoryLinear, a.FormatSymbol(symbol))
	if err != nil {
		return nil, err
	}
	return toTicker(symbol, raw), nil
}

func (a *Adapter) FetchOrderBook(ctx context.Context, symbol string, limit int) (*exchanges.OrderBook, error) {
	raw, err := a.client.GetOrderBook(ctx, categoryLinear, a.FormatSymbol(symbol), limit)
	if err != nil {
		return nil, err
	}
	return toOrderBook(symbol, raw), nil
}

func (a *Adapter) FetchTrades(ctx context.Context, symbol string, limit int) ([]exchanges.Trade, error) {
	raw, err := a.client.GetRecentTrades(ctx, categoryLinear, a.FormatSymbol(symbol), limit)
	if err != nil {
		return nil, err
	}
	return mapTrades(symbol, raw), nil
}

func (a *Adapter) FetchKlines(ctx context.Context, symbol string, interval exchanges.Interval, opts *exchanges.KlineOpts) ([]exchanges.Kline, error) {
	rawInterval, err := klineIntervalString(interval)
	if err != nil {
		return nil, err
	}
	start, end, limit, err := klineTimeRange(interval, opts)
	if err != nil {
		return nil, err
	}
	raw, err := a.client.GetKlines(ctx, categoryLinear, a.FormatSymbol(symbol), rawInterval, start, end, limit)
	if err != nil {
		return nil, err
	}
	return mapKlines(symbol, interval, raw), nil
}

func (a *Adapter) PlaceOrder(ctx context.Context, params *exchanges.OrderParams) (*exchanges.Order, error) {
	if err := requirePrivateClientAccess(a.client); err != nil {
		return nil, err
	}
	if err := a.BaseAdapter.ValidateOrder(params); err != nil {
		return nil, err
	}
	req, err := toPlaceOrderRequest(ctx, a, categoryLinear, params)
	if err != nil {
		return nil, err
	}
	raw, err := a.client.PlaceOrder(ctx, *req)
	if err != nil {
		return nil, err
	}
	return &exchanges.Order{
		OrderID:       raw.OrderID,
		ClientOrderID: raw.OrderLinkID,
		Symbol:        strings.ToUpper(params.Symbol),
		Side:          params.Side,
		Type:          params.Type,
		Quantity:      params.Quantity,
		Price:         params.Price,
		Status:        exchanges.OrderStatusNew,
		Timestamp:     time.Now().UnixMilli(),
		ReduceOnly:    params.ReduceOnly,
		TimeInForce:   params.TimeInForce,
	}, nil
}

func (a *Adapter) PlaceOrderWS(ctx context.Context, params *exchanges.OrderParams) error {
	if err := requirePrivateClientAccess(a.client); err != nil {
		return err
	}
	if err := a.BaseAdapter.ValidateOrder(params); err != nil {
		return err
	}
	req, err := toPlaceOrderRequest(ctx, a, categoryLinear, params)
	if err != nil {
		return err
	}
	if a.tradeWS == nil {
		return exchanges.ErrNotSupported
	}
	return a.tradeWS.PlaceOrder(ctx, *req)
}

func (a *Adapter) CancelOrder(ctx context.Context, orderID, symbol string) error {
	if err := requirePrivateClientAccess(a.client); err != nil {
		return err
	}
	_, err := a.client.CancelOrder(ctx, sdk.CancelOrderRequest{
		Category: categoryLinear,
		Symbol:   a.FormatSymbol(symbol),
		OrderID:  orderID,
	})
	return err
}

func (a *Adapter) CancelOrderWS(ctx context.Context, orderID, symbol string) error {
	if err := requirePrivateClientAccess(a.client); err != nil {
		return err
	}
	if a.tradeWS == nil {
		return exchanges.ErrNotSupported
	}
	return a.tradeWS.CancelOrder(ctx, sdk.CancelOrderRequest{
		Category: categoryLinear,
		Symbol:   a.FormatSymbol(symbol),
		OrderID:  orderID,
	})
}

func (a *Adapter) CancelAllOrders(ctx context.Context, symbol string) error {
	if err := requirePrivateClientAccess(a.client); err != nil {
		return err
	}
	return a.client.CancelAllOrders(ctx, sdk.CancelAllOrdersRequest{
		Category: categoryLinear,
		Symbol:   a.FormatSymbol(symbol),
	})
}

func (a *Adapter) FetchOrderByID(ctx context.Context, orderID, symbol string) (*exchanges.Order, error) {
	if err := requirePrivateClientAccess(a.client); err != nil {
		return nil, err
	}
	formatted := a.FormatSymbol(symbol)
	realtime, err := a.client.GetRealtimeOrders(ctx, categoryLinear, formatted, "", orderID, "", -1)
	if err != nil {
		return nil, err
	}
	for _, order := range realtime {
		if order.OrderID == orderID || order.OrderLinkID == orderID {
			return mapOrder(a.ExtractSymbol(order.Symbol), order), nil
		}
	}

	history, err := a.client.GetOrderHistoryFiltered(ctx, categoryLinear, formatted, orderID, "")
	if err != nil {
		return nil, err
	}
	for _, order := range history {
		if order.OrderID == orderID || order.OrderLinkID == orderID {
			return mapOrder(a.ExtractSymbol(order.Symbol), order), nil
		}
	}
	return nil, exchanges.ErrOrderNotFound
}

func (a *Adapter) FetchOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	if err := requirePrivateClientAccess(a.client); err != nil {
		return nil, err
	}
	formatted := a.FormatSymbol(symbol)
	recentClosed, err := a.client.GetRealtimeOrders(ctx, categoryLinear, formatted, "", "", "", 1)
	if err != nil {
		return nil, err
	}
	openOrders, err := a.client.GetRealtimeOrders(ctx, categoryLinear, formatted, "", "", "", 0)
	if err != nil {
		return nil, err
	}
	history, err := a.client.GetOrderHistory(ctx, categoryLinear, formatted)
	if err != nil {
		return nil, err
	}
	out := make([]exchanges.Order, 0, len(history)+len(recentClosed)+len(openOrders))
	for _, order := range history {
		out = append(out, *mapOrder(a.ExtractSymbol(order.Symbol), order))
	}
	for _, order := range recentClosed {
		out = append(out, *mapOrder(a.ExtractSymbol(order.Symbol), order))
	}
	for _, order := range openOrders {
		out = append(out, *mapOrder(a.ExtractSymbol(order.Symbol), order))
	}
	return dedupeOrders(out), nil
}

func (a *Adapter) FetchOpenOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	if err := requirePrivateClientAccess(a.client); err != nil {
		return nil, err
	}
	formatted := a.FormatSymbol(symbol)
	settleCoin := ""
	if formatted == "" {
		settleCoin = string(a.quote)
	}
	raw, err := a.client.GetRealtimeOrders(ctx, categoryLinear, formatted, settleCoin, "", "", 0)
	if err != nil {
		return nil, err
	}
	out := make([]exchanges.Order, 0, len(raw))
	for _, order := range raw {
		if !containsActiveOrder(order) {
			continue
		}
		out = append(out, *mapOrder(a.ExtractSymbol(order.Symbol), order))
	}
	return out, nil
}

func (a *Adapter) FetchAccount(ctx context.Context) (*exchanges.Account, error) {
	if err := requirePrivateClientAccess(a.client); err != nil {
		return nil, err
	}
	wallet, err := a.client.GetWalletBalance(ctx, "UNIFIED", string(a.quote))
	if err != nil {
		return nil, err
	}
	positions, err := a.FetchPositions(ctx)
	if err != nil {
		return nil, err
	}
	openOrders, err := a.FetchOpenOrders(ctx, "")
	if err != nil {
		return nil, err
	}

	account := &exchanges.Account{
		Positions: positions,
		Orders:    openOrders,
	}
	if len(wallet.List) > 0 {
		account.TotalBalance = parseDecimal(wallet.List[0].TotalEquity)
		account.AvailableBalance = parseDecimal(wallet.List[0].TotalAvailableBalance)
		account.UnrealizedPnL = parseDecimal(wallet.List[0].TotalPerpUPL)
	}
	for _, position := range positions {
		account.RealizedPnL = account.RealizedPnL.Add(position.RealizedPnL)
	}
	return account, nil
}

func (a *Adapter) FetchBalance(ctx context.Context) (decimal.Decimal, error) {
	if err := requirePrivateClientAccess(a.client); err != nil {
		return decimal.Zero, err
	}
	balance, err := a.client.GetWalletBalance(ctx, "UNIFIED", string(a.quote))
	if err != nil {
		return decimal.Zero, err
	}
	if len(balance.List) == 0 {
		return decimal.Zero, nil
	}
	return parseDecimal(balance.List[0].TotalAvailableBalance), nil
}

func (a *Adapter) FetchSymbolDetails(ctx context.Context, symbol string) (*exchanges.SymbolDetails, error) {
	_ = ctx
	return a.GetSymbolDetail(strings.ToUpper(symbol))
}

func (a *Adapter) FetchFeeRate(ctx context.Context, symbol string) (*exchanges.FeeRate, error) {
	if err := requirePrivateClientAccess(a.client); err != nil {
		return nil, err
	}
	raw, err := a.client.GetFeeRates(ctx, categoryLinear, a.FormatSymbol(symbol))
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, exchanges.ErrNotSupported
	}
	return &exchanges.FeeRate{
		Maker: parseDecimal(raw[0].MakerFeeRate),
		Taker: parseDecimal(raw[0].TakerFeeRate),
	}, nil
}

func (a *Adapter) GetLocalOrderBook(symbol string, depth int) *exchanges.OrderBook {
	ob := a.BaseAdapter.GetLocalOrderBook(a.FormatSymbol(symbol), depth)
	if ob != nil {
		ob.Symbol = strings.ToUpper(symbol)
	}
	return ob
}

func (a *Adapter) WatchOrderBook(ctx context.Context, symbol string, depth int, cb exchanges.OrderBookCallback) error {
	formatted := a.FormatSymbol(symbol)
	_ = a.StopWatchOrderBook(context.Background(), symbol)

	ob := NewOrderBook(formatted)
	if snapshot, err := a.client.GetOrderBook(ctx, categoryLinear, formatted, 50); err == nil {
		ob.LoadSnapshot(snapshot)
	}
	a.SetLocalOrderBook(formatted, ob)

	watchCtx, cancel := context.WithCancel(context.Background())
	a.mu.Lock()
	a.cancels[formatted] = cancel
	a.mu.Unlock()

	if a.publicWS == nil {
		cancel()
		a.RemoveLocalOrderBook(formatted)
		return exchanges.ErrNotSupported
	}

	err := a.publicWS.Subscribe(ctx, orderBookTopic(formatted), func(payload json.RawMessage) {
		select {
		case <-watchCtx.Done():
			return
		default:
		}

		msg, err := sdk.DecodeOrderBookMessage(payload)
		if err != nil {
			return
		}
		switch msg.Type {
		case "snapshot":
			ob.ProcessSnapshot(&msg.Data)
		case "delta":
			ob.ProcessDelta(&msg.Data)
		default:
			return
		}
		if cb != nil {
			cb(a.GetLocalOrderBook(symbol, depth))
		}
	})
	if err != nil {
		cancel()
		a.RemoveLocalOrderBook(formatted)
		return err
	}

	a.MarkMarketConnected()
	return a.BaseAdapter.WaitOrderBookReady(ctx, formatted)
}

func (a *Adapter) StopWatchOrderBook(ctx context.Context, symbol string) error {
	formatted := a.FormatSymbol(symbol)

	a.mu.Lock()
	if cancel, ok := a.cancels[formatted]; ok {
		cancel()
		delete(a.cancels, formatted)
	}
	a.mu.Unlock()

	a.RemoveLocalOrderBook(formatted)
	if a.publicWS == nil {
		return exchanges.ErrNotSupported
	}
	return a.publicWS.Unsubscribe(ctx, orderBookTopic(formatted))
}

func (a *Adapter) WatchOrders(ctx context.Context, cb exchanges.OrderUpdateCallback) error {
	if err := requirePrivateClientAccess(a.client); err != nil {
		return err
	}
	if a.privateWS == nil {
		return exchanges.ErrNotSupported
	}
	return a.privateWS.Subscribe(ctx, "order.linear", func(payload json.RawMessage) {
		msg, err := sdk.DecodeOrderMessage(payload)
		if err != nil {
			return
		}
		for _, order := range msg.Data {
			cb(mapOrder(a.ExtractSymbol(order.Symbol), order))
		}
	})
}

func (a *Adapter) WatchFills(ctx context.Context, cb exchanges.FillCallback) error {
	if err := requirePrivateClientAccess(a.client); err != nil {
		return err
	}
	if a.privateWS == nil {
		return exchanges.ErrNotSupported
	}
	return a.privateWS.Subscribe(ctx, "execution.linear", func(payload json.RawMessage) {
		msg, err := sdk.DecodeExecutionMessage(payload)
		if err != nil {
			return
		}
		for _, fill := range msg.Data {
			cb(mapExecutionFill(a.ExtractSymbol(fill.Symbol), fill))
		}
	})
}

func (a *Adapter) WatchPositions(ctx context.Context, cb exchanges.PositionUpdateCallback) error {
	if err := requirePrivateClientAccess(a.client); err != nil {
		return err
	}
	if a.privateWS == nil {
		return exchanges.ErrNotSupported
	}
	return a.privateWS.Subscribe(ctx, "position.linear", func(payload json.RawMessage) {
		msg, err := sdk.DecodePositionMessage(payload)
		if err != nil {
			return
		}
		for _, position := range msg.Data {
			update := mapPosition(a.ExtractSymbol(position.Symbol), position)
			cb(&update)
		}
	})
}

func (a *Adapter) WatchTicker(ctx context.Context, symbol string, cb exchanges.TickerCallback) error {
	_ = ctx
	_ = symbol
	_ = cb
	return exchanges.ErrNotSupported
}

func (a *Adapter) WatchTrades(ctx context.Context, symbol string, cb exchanges.TradeCallback) error {
	_ = ctx
	_ = symbol
	_ = cb
	return exchanges.ErrNotSupported
}

func (a *Adapter) WatchKlines(ctx context.Context, symbol string, interval exchanges.Interval, cb exchanges.KlineCallback) error {
	_ = ctx
	_ = symbol
	_ = interval
	_ = cb
	return exchanges.ErrNotSupported
}

func (a *Adapter) StopWatchOrders(ctx context.Context) error {
	if a.privateWS == nil {
		return exchanges.ErrNotSupported
	}
	return a.privateWS.Unsubscribe(ctx, "order.linear")
}

func (a *Adapter) StopWatchFills(ctx context.Context) error {
	if a.privateWS == nil {
		return exchanges.ErrNotSupported
	}
	return a.privateWS.Unsubscribe(ctx, "execution.linear")
}

func (a *Adapter) StopWatchPositions(ctx context.Context) error {
	if a.privateWS == nil {
		return exchanges.ErrNotSupported
	}
	return a.privateWS.Unsubscribe(ctx, "position.linear")
}

func (a *Adapter) StopWatchTicker(ctx context.Context, symbol string) error {
	_ = ctx
	_ = symbol
	return exchanges.ErrNotSupported
}

func (a *Adapter) StopWatchTrades(ctx context.Context, symbol string) error {
	_ = ctx
	_ = symbol
	return exchanges.ErrNotSupported
}

func (a *Adapter) StopWatchKlines(ctx context.Context, symbol string, interval exchanges.Interval) error {
	_ = ctx
	_ = symbol
	_ = interval
	return exchanges.ErrNotSupported
}

func (a *Adapter) FetchPositions(ctx context.Context) ([]exchanges.Position, error) {
	if err := requirePrivateClientAccess(a.client); err != nil {
		return nil, err
	}
	raw, err := a.client.GetPositions(ctx, categoryLinear, "", string(a.quote))
	if err != nil {
		return nil, err
	}
	out := make([]exchanges.Position, 0, len(raw))
	for _, position := range raw {
		out = append(out, mapPosition(a.ExtractSymbol(position.Symbol), position))
	}
	return out, nil
}

func (a *Adapter) SetLeverage(ctx context.Context, symbol string, leverage int) error {
	if err := requirePrivateClientAccess(a.client); err != nil {
		return err
	}
	value := strconv.Itoa(leverage)
	return a.client.SetLeverage(ctx, sdk.SetLeverageRequest{
		Category:     categoryLinear,
		Symbol:       a.FormatSymbol(symbol),
		BuyLeverage:  value,
		SellLeverage: value,
	})
}

func (a *Adapter) FetchFundingRate(ctx context.Context, symbol string) (*exchanges.FundingRate, error) {
	_ = ctx
	_ = symbol
	return nil, exchanges.ErrNotSupported
}

func (a *Adapter) FetchAllFundingRates(ctx context.Context) ([]exchanges.FundingRate, error) {
	_ = ctx
	return nil, exchanges.ErrNotSupported
}

func (a *Adapter) ModifyOrder(ctx context.Context, orderID, symbol string, params *exchanges.ModifyOrderParams) (*exchanges.Order, error) {
	if err := requirePrivateClientAccess(a.client); err != nil {
		return nil, err
	}
	req := sdk.AmendOrderRequest{
		Category: categoryLinear,
		Symbol:   a.FormatSymbol(symbol),
		OrderID:  orderID,
	}
	if params.Quantity.IsPositive() {
		req.Qty = params.Quantity.String()
	}
	if params.Price.IsPositive() {
		req.Price = params.Price.String()
	}
	if _, err := a.client.AmendOrder(ctx, req); err != nil {
		return nil, err
	}
	return a.FetchOrderByID(ctx, orderID, symbol)
}

func (a *Adapter) ModifyOrderWS(ctx context.Context, orderID, symbol string, params *exchanges.ModifyOrderParams) error {
	if err := requirePrivateClientAccess(a.client); err != nil {
		return err
	}
	if a.tradeWS == nil {
		return exchanges.ErrNotSupported
	}
	req := sdk.AmendOrderRequest{
		Category: categoryLinear,
		Symbol:   a.FormatSymbol(symbol),
		OrderID:  orderID,
	}
	if params.Quantity.IsPositive() {
		req.Qty = params.Quantity.String()
	}
	if params.Price.IsPositive() {
		req.Price = params.Price.String()
	}
	return a.tradeWS.AmendOrder(ctx, req)
}
