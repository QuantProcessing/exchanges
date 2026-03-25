package decibel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	decibelaptos "github.com/QuantProcessing/exchanges/decibel/sdk/aptos"
	decibelrest "github.com/QuantProcessing/exchanges/decibel/sdk/rest"
	decibelws "github.com/QuantProcessing/exchanges/decibel/sdk/ws"
	aptossdkapi "github.com/aptos-labs/aptos-go-sdk/api"
	"github.com/shopspring/decimal"
)

const (
	defaultOrderBookAggregation = "1"
	defaultOpenOrdersLimit      = 100
)

var (
	newDecibelRESTClient = func(apiKey string) decibelRESTClient {
		return decibelrest.NewClient(apiKey)
	}
	newDecibelWSClient = func(ctx context.Context, apiKey string) decibelWSClient {
		return decibelws.NewClient(ctx, apiKey)
	}
	newDecibelAptosClient = func(privateKey string) (decibelAptosClient, error) {
		return decibelaptos.NewClient(privateKey)
	}
	defaultMarketSlippage               = decimal.RequireFromString("0.05")
	orderReconcileTimeout               = 12 * time.Second
	orderPollInterval                   = 100 * time.Millisecond
	lookupDecibelTransactionOrderEvents = fetchTransactionOrderEvents
)

var bootstrapMetadata = defaultBootstrapMetadata

type decibelRESTClient interface {
	GetMarkets(ctx context.Context) ([]decibelrest.Market, error)
	GetTicker(ctx context.Context, market string) (*decibelrest.Ticker, error)
	GetOrderBook(ctx context.Context, market string, limit int) (*decibelrest.OrderBookSnapshot, error)
	GetAccountOverview(ctx context.Context, account string) (*decibelrest.AccountOverview, error)
	GetAccountPositions(ctx context.Context, account string) ([]decibelrest.AccountPosition, error)
	GetOpenOrders(ctx context.Context, account string, limit int, offset int) (*decibelrest.OpenOrdersResponse, error)
	GetOrderHistory(ctx context.Context, account string, limit int, offset int) (*decibelrest.OpenOrdersResponse, error)
	GetOrder(ctx context.Context, account, market, orderID, clientOrderID string) (*decibelrest.OrderResponse, error)
	GetOrderByID(ctx context.Context, account, orderID string) (*decibelrest.OpenOrder, error)
}

type decibelWSClient interface {
	Connect() error
	Close() error
	Subscribe(topic string, handler func(decibelws.MarketDepthMessage)) error
	SubscribeUserOrderHistory(userAddr string, handler func(decibelws.UserOrderHistoryMessage)) error
	SubscribeOrderUpdates(userAddr string, handler func(decibelws.OrderUpdateMessage)) error
	SubscribeUserPositions(userAddr string, handler func(decibelws.UserPositionsMessage)) error
}

type decibelAptosClient interface {
	AccountAddress() string
	PlaceOrder(req decibelaptos.PlaceOrderRequest) (*aptossdkapi.SubmitTransactionResponse, error)
	CancelOrder(req decibelaptos.CancelOrderRequest) (*aptossdkapi.SubmitTransactionResponse, error)
}

type orderBookWatchState struct {
	subscribed bool
	ctx        context.Context
	cancel     context.CancelFunc
	callback   exchanges.OrderBookCallback
}

type ordersWatchState struct {
	subscribed bool
	ctx        context.Context
	cancel     context.CancelFunc
	callback   exchanges.OrderUpdateCallback
}

// Adapter is the Decibel perpetual futures adapter.
type Adapter struct {
	*exchanges.BaseAdapter

	apiKey         string
	privateKey     string
	subaccountAddr string
	accountAddr    string
	quoteCurrency  exchanges.QuoteCurrency

	lifecycleCtx context.Context
	cancel       context.CancelFunc

	rest    decibelRESTClient
	ws      decibelWSClient
	aptos   decibelAptosClient
	markets *marketMetadataCache

	orderBookMu      sync.Mutex
	orderBookWatches map[string]*orderBookWatchState

	ordersMu    sync.Mutex
	ordersWatch ordersWatchState

	pendingMu     sync.Mutex
	pendingOrders map[string]chan *exchanges.Order
	orderAliases  map[string]string
}

func NewAdapter(ctx context.Context, opts Options) (*Adapter, error) {
	if err := opts.validateCredentials(); err != nil {
		return nil, err
	}

	if ctx == nil {
		ctx = context.Background()
	}
	lifecycleCtx, cancel := context.WithCancel(ctx)

	adp := &Adapter{
		BaseAdapter:    exchanges.NewBaseAdapter("DECIBEL", exchanges.MarketTypePerp, opts.logger()),
		apiKey:         opts.APIKey,
		privateKey:     opts.PrivateKey,
		subaccountAddr: opts.SubaccountAddr,
		quoteCurrency:  opts.quoteCurrency(),
		lifecycleCtx:   lifecycleCtx,
		cancel:         cancel,
	}
	adp.initRuntimeState()

	if err := bootstrapMetadata(ctx, adp); err != nil {
		cancel()
		return nil, fmt.Errorf("decibel init: %w", err)
	}

	return adp, nil
}

func defaultBootstrapMetadata(ctx context.Context, adp *Adapter) error {
	adp.initRuntimeState()

	restClient := newDecibelRESTClient(adp.apiKey)
	wsClient := newDecibelWSClient(adp.lifecycleCtx, adp.apiKey)
	aptosClient, err := newDecibelAptosClient(adp.privateKey)
	if err != nil {
		return exchanges.NewExchangeError("DECIBEL", "", fmt.Sprintf("invalid private_key: %v", err), exchanges.ErrAuthFailed)
	}

	markets, err := restClient.GetMarkets(ctx)
	if err != nil {
		return err
	}
	cache, err := newMarketMetadataCache(markets)
	if err != nil {
		return err
	}

	adp.rest = restClient
	adp.ws = wsClient
	adp.aptos = aptosClient
	adp.accountAddr = aptosClient.AccountAddress()
	adp.markets = cache
	adp.SetSymbolDetails(symbolDetailsFromMetadataCache(cache))
	return nil
}

func (a *Adapter) initRuntimeState() {
	if a.lifecycleCtx == nil {
		a.lifecycleCtx = context.Background()
	}
	if a.cancel == nil {
		a.lifecycleCtx, a.cancel = context.WithCancel(a.lifecycleCtx)
	}
	if a.orderBookWatches == nil {
		a.orderBookWatches = make(map[string]*orderBookWatchState)
	}
	if a.pendingOrders == nil {
		a.pendingOrders = make(map[string]chan *exchanges.Order)
	}
	if a.orderAliases == nil {
		a.orderAliases = make(map[string]string)
	}
}

func (a *Adapter) Close() error {
	if a.cancel != nil {
		a.cancel()
	}
	return closeWS(a.ws)
}

func (a *Adapter) FormatSymbol(symbol string) string {
	if a.markets == nil {
		return strings.ToUpper(strings.TrimSpace(symbol))
	}
	marketAddr, err := a.markets.marketAddress(symbol)
	if err != nil {
		return strings.ToUpper(strings.TrimSpace(symbol))
	}
	return marketAddr
}

func (a *Adapter) ExtractSymbol(symbol string) string {
	if a.markets == nil {
		return strings.ToUpper(strings.TrimSpace(symbol))
	}
	base, err := a.markets.symbolForMarket(symbol)
	if err != nil {
		return strings.ToUpper(strings.TrimSpace(symbol))
	}
	return base
}

func (a *Adapter) FetchTicker(ctx context.Context, symbol string) (*exchanges.Ticker, error) {
	if err := a.requireREST(); err != nil {
		return nil, err
	}
	meta, err := a.metadataForSymbol(symbol)
	if err != nil {
		return nil, err
	}
	raw, err := a.rest.GetTicker(ctx, meta.MarketAddr)
	if err != nil {
		return nil, err
	}
	return a.mapTicker(meta.BaseSymbol, raw), nil
}

func (a *Adapter) FetchOrderBook(ctx context.Context, symbol string, limit int) (*exchanges.OrderBook, error) {
	if err := a.requireREST(); err != nil {
		return nil, err
	}
	meta, err := a.metadataForSymbol(symbol)
	if err != nil {
		return nil, err
	}
	raw, err := a.rest.GetOrderBook(ctx, meta.MarketAddr, limit)
	if err == nil && raw != nil {
		return a.mapOrderBook(meta.BaseSymbol, raw), nil
	}
	return a.syntheticOrderBookFromTicker(ctx, meta, limit)
}

func (a *Adapter) FetchTrades(context.Context, string, int) ([]exchanges.Trade, error) {
	return nil, unsupported("FetchTrades")
}

func (a *Adapter) FetchKlines(context.Context, string, exchanges.Interval, *exchanges.KlineOpts) ([]exchanges.Kline, error) {
	return nil, unsupported("FetchKlines")
}

func (a *Adapter) PlaceOrder(ctx context.Context, params *exchanges.OrderParams) (*exchanges.Order, error) {
	if params == nil {
		return nil, fmt.Errorf("order params are required")
	}
	if err := a.requireTrading(); err != nil {
		return nil, err
	}

	reqParams := *params
	originalType := reqParams.Type
	if reqParams.Type == exchanges.OrderTypeMarket && !reqParams.Slippage.IsPositive() {
		reqParams.Slippage = defaultMarketSlippage
	}
	if err := a.BaseAdapter.ApplySlippage(ctx, &reqParams, a.FetchTicker); err != nil {
		return nil, err
	}
	if err := a.BaseAdapter.ValidateOrder(&reqParams); err != nil {
		return nil, err
	}

	meta, err := a.metadataForSymbol(reqParams.Symbol)
	if err != nil {
		return nil, err
	}
	quantizedSize, err := meta.quantizeSize(reqParams.Quantity)
	if err != nil {
		return nil, err
	}
	reqParams.Quantity = quantizedSize
	if reqParams.Type == exchanges.OrderTypeMarket && reqParams.Price.IsZero() {
		price, err := a.marketOrderPrice(ctx, meta, reqParams.Side)
		if err != nil {
			return nil, err
		}
		reqParams.Price = price
		reqParams.Type = exchanges.OrderTypeLimit
		if reqParams.TimeInForce == "" {
			reqParams.TimeInForce = exchanges.TimeInForceIOC
		}
	}
	if reqParams.Type == exchanges.OrderTypeLimit && !reqParams.Price.IsPositive() {
		return nil, newPrecisionError("limit price must be positive")
	}
	if !reqParams.Price.IsZero() {
		quantizedPrice, err := meta.quantizePrice(reqParams.Price)
		if err != nil {
			return nil, err
		}
		reqParams.Price = quantizedPrice
		if _, err := meta.EncodePrice(reqParams.Price); err != nil {
			return nil, err
		}
	}

	clientID := strings.TrimSpace(reqParams.ClientID)
	if clientID == "" {
		clientID = fmt.Sprintf("decibel-%d", time.Now().UnixNano())
	}

	pending := a.registerPendingOrder(clientID)
	defer a.unregisterPendingOrder(clientID, pending)

	tif, err := toAptosTimeInForce(reqParams.Type, reqParams.TimeInForce)
	if err != nil {
		return nil, err
	}
	isBuy := reqParams.Side == exchanges.OrderSideBuy
	aptosReq := decibelaptos.PlaceOrderRequest{
		SubaccountAddr: a.subaccountAddr,
		MarketAddr:     meta.MarketAddr,
		Price:          reqParams.Price,
		Size:           reqParams.Quantity,
		Encoder:        meta,
		IsBuy:          isBuy,
		TimeInForce:    tif,
		ReduceOnly:     reqParams.ReduceOnly,
		ClientOrderID:  &clientID,
	}

	submitResp, err := a.aptos.PlaceOrder(aptosReq)
	if err != nil {
		return nil, err
	}

	fallbackID := ""
	if submitResp != nil {
		fallbackID = submitResp.Hash
	}
	if order, ok, alreadyPublished := a.awaitReconciledOrder(ctx, originalType, meta.MarketAddr, clientID, fallbackID, pending); ok {
		if !alreadyPublished {
			a.publishOrderUpdate(order)
		}
		return order, nil
	}

	status := exchanges.OrderStatusPending
	if fallbackID == "" {
		status = exchanges.OrderStatusNew
	}
	order := &exchanges.Order{
		OrderID:       "",
		ClientOrderID: clientID,
		Symbol:        meta.BaseSymbol,
		Side:          reqParams.Side,
		Type:          originalType,
		Quantity:      reqParams.Quantity,
		Price:         reqParams.Price,
		Status:        status,
		ReduceOnly:    reqParams.ReduceOnly,
		TimeInForce:   reqParams.TimeInForce,
		Timestamp:     time.Now().UnixMilli(),
	}
	a.publishOrderUpdate(order)
	return order, nil
}

func (a *Adapter) CancelOrder(ctx context.Context, orderID, symbol string) error {
	if err := a.requireTrading(); err != nil {
		return err
	}
	meta, err := a.metadataForSymbol(symbol)
	if err != nil {
		return err
	}
	_, err = a.aptos.CancelOrder(decibelaptos.CancelOrderRequest{
		SubaccountAddr: a.subaccountAddr,
		OrderID:        a.resolveOrderID(orderID),
		MarketAddr:     meta.MarketAddr,
	})
	return err
}

func (a *Adapter) CancelAllOrders(context.Context, string) error {
	return unsupported("CancelAllOrders")
}

func (a *Adapter) FetchOrderByID(ctx context.Context, orderID, symbol string) (*exchanges.Order, error) {
	if err := a.requireREST(); err != nil {
		return nil, err
	}
	resolvedID := a.resolveOrderID(orderID)
	if strings.TrimSpace(symbol) != "" {
		meta, err := a.metadataForSymbol(symbol)
		if err == nil {
			resp, err := a.rest.GetOrder(ctx, a.subaccountAddr, meta.MarketAddr, resolvedID, "")
			if err == nil {
				order, mapErr := a.mapOrderResponse(resp)
				if mapErr == nil && order != nil {
					return order, nil
				}
			}
		}
	}

	raw, err := a.rest.GetOrderByID(ctx, a.subaccountAddr, resolvedID)
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, exchanges.NewExchangeError("DECIBEL", "", fmt.Sprintf("order not found: %s", orderID), exchanges.ErrOrderNotFound)
	}
	order, err := a.mapOpenOrder(raw)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(symbol) != "" && order.Symbol != strings.ToUpper(strings.TrimSpace(symbol)) {
		return nil, exchanges.NewExchangeError("DECIBEL", "", fmt.Sprintf("order not found: %s", orderID), exchanges.ErrOrderNotFound)
	}
	return order, nil
}

func (a *Adapter) FetchOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	if err := a.requireREST(); err != nil {
		return nil, err
	}
	rawOrders, err := a.fetchOrderHistory(ctx)
	if err != nil {
		return nil, err
	}
	return a.mapOpenOrders(rawOrders, symbol)
}

func (a *Adapter) FetchOpenOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	if err := a.requireREST(); err != nil {
		return nil, err
	}
	rawOrders, err := a.fetchOpenOrders(ctx)
	if err != nil {
		return nil, err
	}
	return a.mapOpenOrders(rawOrders, symbol)
}

func (a *Adapter) FetchAccount(ctx context.Context) (*exchanges.Account, error) {
	if err := a.requireREST(); err != nil {
		return nil, err
	}
	overview, err := a.rest.GetAccountOverview(ctx, a.subaccountAddr)
	if err != nil {
		return nil, err
	}
	positions, err := a.fetchPositions(ctx)
	if err != nil {
		return nil, err
	}
	openOrders, err := a.fetchOpenOrders(ctx)
	if err != nil {
		return nil, err
	}
	mappedOrders, err := a.mapOpenOrders(openOrders, "")
	if err != nil {
		return nil, err
	}
	return &exchanges.Account{
		TotalBalance:     overview.TotalBalance,
		AvailableBalance: overview.AvailableBalance,
		UnrealizedPnL:    overview.UnrealizedPnL,
		Positions:        positions,
		Orders:           mappedOrders,
	}, nil
}

func (a *Adapter) FetchBalance(ctx context.Context) (decimal.Decimal, error) {
	if err := a.requireREST(); err != nil {
		return decimal.Zero, err
	}
	overview, err := a.rest.GetAccountOverview(ctx, a.subaccountAddr)
	if err != nil {
		return decimal.Zero, err
	}
	return overview.AvailableBalance, nil
}

func (a *Adapter) FetchSymbolDetails(_ context.Context, symbol string) (*exchanges.SymbolDetails, error) {
	if a.markets == nil {
		return nil, exchanges.NewExchangeError("DECIBEL", "", fmt.Sprintf("symbol not found: %s", symbol), exchanges.ErrSymbolNotFound)
	}
	return a.markets.symbolDetails(symbol)
}

func (a *Adapter) FetchFeeRate(_ context.Context, symbol string) (*exchanges.FeeRate, error) {
	if _, err := a.metadataForSymbol(symbol); err != nil {
		return nil, err
	}
	// Decibel fee tiers are not surfaced by the current metadata bootstrap, so v1
	// returns a deterministic zero-fee fallback to satisfy repository reads.
	return &exchanges.FeeRate{Maker: decimal.Zero, Taker: decimal.Zero}, nil
}

func (a *Adapter) WatchOrderBook(ctx context.Context, symbol string, cb exchanges.OrderBookCallback) error {
	if err := a.requireWS(); err != nil {
		return err
	}
	meta, err := a.metadataForSymbol(symbol)
	if err != nil {
		return err
	}
	baseSymbol := meta.BaseSymbol
	orderBook := NewOrderBook(baseSymbol)
	a.SetLocalOrderBook(baseSymbol, orderBook)

	a.orderBookMu.Lock()
	state := a.orderBookWatches[baseSymbol]
	if state == nil {
		state = &orderBookWatchState{}
		a.orderBookWatches[baseSymbol] = state
	}
	if state.cancel != nil {
		state.cancel()
	}
	watchCtx, cancel := context.WithCancel(context.Background())
	state.ctx = watchCtx
	state.cancel = cancel
	state.callback = cb
	shouldSubscribe := !state.subscribed
	state.subscribed = true
	a.orderBookMu.Unlock()

	if shouldSubscribe {
		topic := depthTopic(meta.MarketAddr)
		if err := a.ws.Subscribe(topic, func(msg decibelws.MarketDepthMessage) {
			a.handleDepthUpdate(baseSymbol, msg)
		}); err != nil {
			return err
		}
	}

	if err := a.ws.Connect(); err != nil {
		return err
	}

	waitCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	if err := a.BaseAdapter.WaitOrderBookReady(waitCtx, baseSymbol); err == nil {
		return nil
	}

	if seeded, err := a.seedOrderBookFromFallback(ctx, meta, orderBook); err == nil && seeded {
		if cb != nil {
			cb(orderBook.ToAdapterOrderBook(0))
		}
	}
	return a.BaseAdapter.WaitOrderBookReady(ctx, baseSymbol)
}

func (a *Adapter) StopWatchOrderBook(_ context.Context, symbol string) error {
	baseSymbol := strings.ToUpper(strings.TrimSpace(symbol))
	a.orderBookMu.Lock()
	if state := a.orderBookWatches[baseSymbol]; state != nil {
		if state.cancel != nil {
			state.cancel()
		}
		state.ctx = nil
		state.cancel = nil
		state.callback = nil
	}
	a.orderBookMu.Unlock()
	a.RemoveLocalOrderBook(baseSymbol)
	return nil
}

func (a *Adapter) WatchOrders(_ context.Context, cb exchanges.OrderUpdateCallback) error {
	if err := a.requireWS(); err != nil {
		return err
	}

	a.ordersMu.Lock()
	if a.ordersWatch.cancel != nil {
		a.ordersWatch.cancel()
	}
	watchCtx, cancel := context.WithCancel(context.Background())
	a.ordersWatch.ctx = watchCtx
	a.ordersWatch.cancel = cancel
	a.ordersWatch.callback = cb
	shouldSubscribe := !a.ordersWatch.subscribed
	a.ordersWatch.subscribed = true
	a.ordersMu.Unlock()

	if shouldSubscribe {
		userAddr := a.accountAddr
		if strings.TrimSpace(userAddr) == "" {
			userAddr = a.subaccountAddr
		}
		if err := a.ws.SubscribeOrderUpdates(userAddr, a.handleOrderUpdate); err != nil {
			return err
		}
		if err := a.ws.SubscribeUserOrderHistory(userAddr, a.handleUserOrderHistory); err != nil {
			return err
		}
	}
	return a.ws.Connect()
}

func (a *Adapter) WatchPositions(context.Context, exchanges.PositionUpdateCallback) error {
	return unsupported("WatchPositions")
}

func (a *Adapter) WatchTicker(context.Context, string, exchanges.TickerCallback) error {
	return unsupported("WatchTicker")
}

func (a *Adapter) WatchTrades(context.Context, string, exchanges.TradeCallback) error {
	return unsupported("WatchTrades")
}

func (a *Adapter) WatchKlines(context.Context, string, exchanges.Interval, exchanges.KlineCallback) error {
	return unsupported("WatchKlines")
}

func (a *Adapter) StopWatchOrders(context.Context) error {
	a.ordersMu.Lock()
	defer a.ordersMu.Unlock()
	if a.ordersWatch.cancel != nil {
		a.ordersWatch.cancel()
	}
	a.ordersWatch.ctx = nil
	a.ordersWatch.cancel = nil
	a.ordersWatch.callback = nil
	return nil
}

func (a *Adapter) StopWatchPositions(context.Context) error {
	return unsupported("StopWatchPositions")
}

func (a *Adapter) StopWatchTicker(context.Context, string) error {
	return unsupported("StopWatchTicker")
}

func (a *Adapter) StopWatchTrades(context.Context, string) error {
	return unsupported("StopWatchTrades")
}

func (a *Adapter) StopWatchKlines(context.Context, string, exchanges.Interval) error {
	return unsupported("StopWatchKlines")
}

func (a *Adapter) FetchPositions(ctx context.Context) ([]exchanges.Position, error) {
	if err := a.requireREST(); err != nil {
		return nil, err
	}
	return a.fetchPositions(ctx)
}

func (a *Adapter) SetLeverage(context.Context, string, int) error {
	return unsupported("SetLeverage")
}

func (a *Adapter) FetchFundingRate(context.Context, string) (*exchanges.FundingRate, error) {
	return nil, unsupported("FetchFundingRate")
}

func (a *Adapter) FetchAllFundingRates(context.Context) ([]exchanges.FundingRate, error) {
	return nil, unsupported("FetchAllFundingRates")
}

func (a *Adapter) ModifyOrder(context.Context, string, string, *exchanges.ModifyOrderParams) (*exchanges.Order, error) {
	return nil, unsupported("ModifyOrder")
}

func (a *Adapter) handleDepthUpdate(symbol string, msg decibelws.MarketDepthMessage) {
	a.orderBookMu.Lock()
	state := a.orderBookWatches[symbol]
	a.orderBookMu.Unlock()
	if state == nil || state.ctx == nil || state.ctx.Err() != nil {
		return
	}

	impl, ok := a.GetLocalOrderBookImplementation(symbol)
	if !ok {
		return
	}
	book, ok := impl.(*OrderBook)
	if !ok {
		return
	}
	book.ProcessDepth(msg)
	if state.callback != nil {
		state.callback(book.ToAdapterOrderBook(0))
	}
}

func (a *Adapter) handleUserOrderHistory(msg decibelws.UserOrderHistoryMessage) {
	a.ordersMu.Lock()
	state := a.ordersWatch
	a.ordersMu.Unlock()
	if state.ctx == nil || state.ctx.Err() != nil {
		return
	}

	for _, item := range msg.Orders {
		order := a.mapWSOrder(msg.Market, item)
		if order == nil {
			continue
		}
		if order.ClientOrderID != "" && order.OrderID != "" {
			a.storeOrderAlias(order.ClientOrderID, order.OrderID)
		}
		a.publishOrderUpdate(order)
	}
}

func (a *Adapter) handleOrderUpdate(msg decibelws.OrderUpdateMessage) {
	a.ordersMu.Lock()
	state := a.ordersWatch
	a.ordersMu.Unlock()
	if state.ctx == nil || state.ctx.Err() != nil {
		return
	}

	order := a.mapOrderUpdate(msg)
	if order == nil {
		return
	}
	if order.ClientOrderID != "" && order.OrderID != "" {
		a.storeOrderAlias(order.ClientOrderID, order.OrderID)
	}
	a.publishOrderUpdate(order)
}

func (a *Adapter) awaitReconciledOrder(
	ctx context.Context,
	originalType exchanges.OrderType,
	marketAddr string,
	clientID string,
	fallbackID string,
	pending chan *exchanges.Order,
) (*exchanges.Order, bool, bool) {
	waitCtx, cancel := context.WithTimeout(ctx, orderReconcileTimeout)
	defer cancel()

	if clientID == "" || a.rest == nil {
		if fallbackID == "" {
			return nil, false, false
		}
		if order, err := a.lookupOrderFromTransaction(waitCtx, fallbackID, clientID); err == nil && order != nil {
			return order, true, false
		}
		return nil, false, false
	}

	ticker := time.NewTicker(orderPollInterval)
	defer ticker.Stop()

	for {
		if pending != nil {
			select {
			case order := <-pending:
				if order != nil && fallbackID != "" && order.OrderID != "" {
					a.storeOrderAlias(fallbackID, order.OrderID)
				}
				if order != nil {
					return order, true, true
				}
			default:
			}
		}

		if order, err := a.lookupOrder(waitCtx, marketAddr, fallbackID, clientID); err == nil && order != nil {
			if shouldAcceptReconciledOrder(order, originalType) {
				return order, true, false
			}
		}

		rawOrders, err := a.fetchOpenOrders(waitCtx)
		if err == nil {
			for _, raw := range rawOrders {
				if raw.ClientOrderID != clientID {
					continue
				}
				order, mapErr := a.mapOpenOrder(&raw)
				if mapErr != nil {
					continue
				}
				if fallbackID != "" && order.OrderID != "" {
					a.storeOrderAlias(fallbackID, order.OrderID)
				}
				if shouldAcceptReconciledOrder(order, originalType) {
					return order, true, false
				}
			}
		}
		if fallbackID != "" {
			if order, txErr := a.lookupOrderFromTransaction(waitCtx, fallbackID, clientID); txErr == nil && order != nil {
				if fallbackID != "" && order.OrderID != "" {
					a.storeOrderAlias(fallbackID, order.OrderID)
				}
				if shouldAcceptReconciledOrder(order, originalType) {
					return order, true, false
				}
			}
		}

		select {
		case <-waitCtx.Done():
			return nil, false, false
		case <-ticker.C:
		}
	}
}

func shouldAcceptReconciledOrder(order *exchanges.Order, originalType exchanges.OrderType) bool {
	if order == nil {
		return false
	}
	switch order.Status {
	case exchanges.OrderStatusUnknown, exchanges.OrderStatusPending:
		return false
	}
	if originalType != exchanges.OrderTypeMarket {
		return true
	}
	switch order.Status {
	case exchanges.OrderStatusFilled, exchanges.OrderStatusCancelled, exchanges.OrderStatusRejected, exchanges.OrderStatusPartiallyFilled:
		return true
	default:
		return false
	}
}

func (a *Adapter) registerPendingOrder(clientID string) chan *exchanges.Order {
	if clientID == "" {
		return nil
	}
	ch := make(chan *exchanges.Order, 1)
	a.pendingMu.Lock()
	a.pendingOrders[clientID] = ch
	a.pendingMu.Unlock()
	return ch
}

func (a *Adapter) unregisterPendingOrder(clientID string, ch chan *exchanges.Order) {
	if clientID == "" || ch == nil {
		return
	}
	a.pendingMu.Lock()
	if current, ok := a.pendingOrders[clientID]; ok && current == ch {
		delete(a.pendingOrders, clientID)
	}
	a.pendingMu.Unlock()
}

func (a *Adapter) resolvePendingOrder(order *exchanges.Order) {
	if order == nil || order.ClientOrderID == "" {
		return
	}
	a.pendingMu.Lock()
	ch := a.pendingOrders[order.ClientOrderID]
	a.pendingMu.Unlock()
	if ch == nil {
		return
	}
	select {
	case ch <- order:
	default:
	}
}

func (a *Adapter) publishOrderUpdate(order *exchanges.Order) {
	if order == nil {
		return
	}
	if order.ClientOrderID != "" && order.OrderID != "" {
		a.storeOrderAlias(order.ClientOrderID, order.OrderID)
	}
	a.resolvePendingOrder(order)

	a.ordersMu.Lock()
	state := a.ordersWatch
	a.ordersMu.Unlock()
	if state.ctx == nil || state.ctx.Err() != nil || state.callback == nil {
		return
	}
	state.callback(order)
}

func (a *Adapter) storeOrderAlias(fromID, toID string) {
	fromID = strings.TrimSpace(fromID)
	toID = strings.TrimSpace(toID)
	if fromID == "" || toID == "" || fromID == toID {
		return
	}
	a.pendingMu.Lock()
	a.orderAliases[fromID] = toID
	a.pendingMu.Unlock()
}

func (a *Adapter) resolveOrderID(orderID string) string {
	resolved := strings.TrimSpace(orderID)
	if resolved == "" {
		return resolved
	}

	seen := map[string]struct{}{}
	for {
		a.pendingMu.Lock()
		next, ok := a.orderAliases[resolved]
		a.pendingMu.Unlock()
		if !ok || next == "" {
			return resolved
		}
		if _, exists := seen[next]; exists {
			return next
		}
		seen[resolved] = struct{}{}
		resolved = next
	}
}

func (a *Adapter) fetchPositions(ctx context.Context) ([]exchanges.Position, error) {
	raw, err := a.rest.GetAccountPositions(ctx, a.subaccountAddr)
	if err != nil {
		return nil, err
	}

	positions := make([]exchanges.Position, 0, len(raw))
	for _, position := range raw {
		mapped, ok := a.mapAccountPosition(position)
		if !ok {
			continue
		}
		positions = append(positions, mapped)
	}
	return positions, nil
}

func (a *Adapter) fetchOpenOrders(ctx context.Context) ([]decibelrest.OpenOrder, error) {
	offset := 0
	var orders []decibelrest.OpenOrder

	for {
		resp, err := a.rest.GetOpenOrders(ctx, a.subaccountAddr, defaultOpenOrdersLimit, offset)
		if err != nil {
			return nil, err
		}
		if resp == nil {
			return orders, nil
		}
		orders = append(orders, resp.Items...)
		offset += len(resp.Items)
		if len(resp.Items) == 0 || offset >= resp.TotalCount {
			return orders, nil
		}
	}
}

func (a *Adapter) fetchOrderHistory(ctx context.Context) ([]decibelrest.OpenOrder, error) {
	offset := 0
	var orders []decibelrest.OpenOrder

	for {
		resp, err := a.rest.GetOrderHistory(ctx, a.subaccountAddr, defaultOpenOrdersLimit, offset)
		if err != nil {
			return nil, err
		}
		if resp == nil {
			return orders, nil
		}
		orders = append(orders, resp.Items...)
		offset += len(resp.Items)
		if len(resp.Items) == 0 || offset >= resp.TotalCount {
			return orders, nil
		}
	}
}

func (a *Adapter) mapTicker(symbol string, raw *decibelrest.Ticker) *exchanges.Ticker {
	if raw == nil {
		return nil
	}
	return &exchanges.Ticker{
		Symbol:    symbol,
		LastPrice: raw.LastPrice,
		MarkPrice: raw.MarkPrice,
		Bid:       raw.BidPrice,
		Ask:       raw.AskPrice,
		Timestamp: raw.Timestamp,
	}
}

func (a *Adapter) mapOrderBook(symbol string, raw *decibelrest.OrderBookSnapshot) *exchanges.OrderBook {
	if raw == nil {
		return nil
	}

	book := &exchanges.OrderBook{
		Symbol:    symbol,
		Timestamp: raw.Timestamp,
		Bids:      make([]exchanges.Level, 0, len(raw.Bids)),
		Asks:      make([]exchanges.Level, 0, len(raw.Asks)),
	}
	for _, level := range raw.Bids {
		book.Bids = append(book.Bids, exchanges.Level{Price: level.Price, Quantity: level.Size})
	}
	for _, level := range raw.Asks {
		book.Asks = append(book.Asks, exchanges.Level{Price: level.Price, Quantity: level.Size})
	}
	return book
}

func (a *Adapter) syntheticOrderBookFromTicker(ctx context.Context, meta marketMetadata, limit int) (*exchanges.OrderBook, error) {
	ticker, err := a.FetchTicker(ctx, meta.BaseSymbol)
	if err != nil {
		return nil, err
	}
	bid, ask := syntheticBidAsk(ticker, meta.TickSize)
	book := &exchanges.OrderBook{
		Symbol:    meta.BaseSymbol,
		Timestamp: ticker.Timestamp,
	}
	if bid.IsPositive() {
		book.Bids = []exchanges.Level{{Price: bid, Quantity: meta.MinSize}}
	}
	if ask.IsPositive() {
		book.Asks = []exchanges.Level{{Price: ask, Quantity: meta.MinSize}}
	}
	if limit > 0 {
		if len(book.Bids) > limit {
			book.Bids = book.Bids[:limit]
		}
		if len(book.Asks) > limit {
			book.Asks = book.Asks[:limit]
		}
	}
	return book, nil
}

func (a *Adapter) seedOrderBookFromFallback(ctx context.Context, meta marketMetadata, book *OrderBook) (bool, error) {
	snapshot, err := a.syntheticOrderBookFromTicker(ctx, meta, 1)
	if err != nil {
		return false, err
	}

	msg := decibelws.MarketDepthMessage{
		Topic:      depthTopic(meta.MarketAddr),
		Market:     meta.MarketAddr,
		UpdateType: decibelws.DepthUpdateSnapshot,
		Timestamp:  snapshot.Timestamp,
		Bids:       make([]decibelws.DepthLevel, 0, len(snapshot.Bids)),
		Asks:       make([]decibelws.DepthLevel, 0, len(snapshot.Asks)),
	}
	for _, level := range snapshot.Bids {
		msg.Bids = append(msg.Bids, decibelws.DepthLevel{Price: level.Price, Size: level.Quantity})
	}
	for _, level := range snapshot.Asks {
		msg.Asks = append(msg.Asks, decibelws.DepthLevel{Price: level.Price, Size: level.Quantity})
	}
	book.ProcessDepth(msg)
	return true, nil
}

func (a *Adapter) mapAccountPosition(raw decibelrest.AccountPosition) (exchanges.Position, bool) {
	symbol := a.ExtractSymbol(raw.Market)
	quantity := raw.Size
	if quantity.IsZero() {
		return exchanges.Position{}, false
	}

	side := exchanges.PositionSideLong
	switch strings.ToUpper(strings.TrimSpace(raw.Side)) {
	case "SHORT", "SELL":
		side = exchanges.PositionSideShort
		quantity = quantity.Abs()
	case "LONG", "BUY":
		quantity = quantity.Abs()
	default:
		if quantity.IsNegative() {
			side = exchanges.PositionSideShort
			quantity = quantity.Abs()
		}
	}

	return exchanges.Position{
		Symbol:           symbol,
		Side:             side,
		Quantity:         quantity,
		EntryPrice:       raw.EntryPrice,
		Leverage:         raw.UserLeverage,
		LiquidationPrice: raw.EstimatedLiquidationPrice,
	}, true
}

func (a *Adapter) mapOpenOrders(rawOrders []decibelrest.OpenOrder, symbol string) ([]exchanges.Order, error) {
	orders := make([]exchanges.Order, 0, len(rawOrders))
	filter := strings.ToUpper(strings.TrimSpace(symbol))

	for _, raw := range rawOrders {
		order, err := a.mapOpenOrder(&raw)
		if err != nil {
			return nil, err
		}
		if filter != "" && order.Symbol != filter {
			continue
		}
		orders = append(orders, *order)
	}
	return orders, nil
}

func (a *Adapter) mapOpenOrder(raw *decibelrest.OpenOrder) (*exchanges.Order, error) {
	if raw == nil {
		return nil, nil
	}
	symbol := a.ExtractSymbol(raw.Market)
	filled := raw.OrigSize.Sub(raw.RemainingSize)
	if filled.IsNegative() {
		filled = decimal.Zero
	}

	order := &exchanges.Order{
		OrderID:        raw.OrderID,
		ClientOrderID:  raw.ClientOrderID,
		Symbol:         symbol,
		Side:           normalizeRESTOrderSide(raw),
		Type:           normalizeOrderType(raw.OrderType),
		Quantity:       raw.OrigSize,
		Price:          raw.Price,
		Status:         normalizeOrderStatus(raw.Status),
		FilledQuantity: filled,
		Timestamp:      raw.UnixMS,
	}
	exchanges.DerivePartialFillStatus(order)
	return order, nil
}

func (a *Adapter) mapOrderResponse(resp *decibelrest.OrderResponse) (*exchanges.Order, error) {
	if resp == nil {
		return nil, nil
	}
	order, err := a.mapOpenOrder(&resp.Order)
	if err != nil || order == nil {
		return order, err
	}
	if strings.TrimSpace(resp.Status) != "" {
		order.Status = normalizeOrderStatus(resp.Status)
	}
	exchanges.DerivePartialFillStatus(order)
	return order, nil
}

func (a *Adapter) lookupOrder(
	ctx context.Context,
	marketAddr string,
	orderID string,
	clientID string,
) (*exchanges.Order, error) {
	if a.rest == nil || strings.TrimSpace(marketAddr) == "" {
		return nil, nil
	}
	lookupOrderID := a.resolveOrderID(orderID)
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(lookupOrderID)), "0x") {
		lookupOrderID = ""
	}
	lookupClientID := clientID
	if lookupOrderID != "" {
		lookupClientID = ""
	}
	resp, err := a.rest.GetOrder(ctx, a.subaccountAddr, marketAddr, lookupOrderID, lookupClientID)
	if err != nil {
		return nil, err
	}
	order, err := a.mapOrderResponse(resp)
	if err != nil || order == nil {
		return order, err
	}
	if orderID != "" && order.OrderID != "" {
		a.storeOrderAlias(orderID, order.OrderID)
	}
	return order, nil
}

func (a *Adapter) mapWSOrder(market string, raw decibelws.OrderHistoryItem) *exchanges.Order {
	status := raw.NormalizedStatus
	if status == exchanges.OrderStatusUnknown {
		status = normalizeOrderStatus(raw.Status)
	}

	filled := raw.OrigSize.Sub(raw.RemainingSize)
	if filled.IsNegative() {
		filled = decimal.Zero
	}

	order := &exchanges.Order{
		OrderID:        raw.OrderID,
		ClientOrderID:  raw.ClientOrderID,
		Symbol:         a.ExtractSymbol(market),
		Side:           normalizeOrderSide(raw.Side),
		Type:           normalizeOrderType(raw.OrderType),
		Quantity:       raw.OrigSize,
		Price:          raw.Price,
		Status:         status,
		FilledQuantity: filled,
		Timestamp:      raw.UnixMS,
	}
	exchanges.DerivePartialFillStatus(order)
	return order
}

func (a *Adapter) mapOrderUpdate(msg decibelws.OrderUpdateMessage) *exchanges.Order {
	raw := msg.Order.Order
	status := msg.Order.NormalizedStatus
	if status == exchanges.OrderStatusUnknown {
		status = normalizeOrderStatus(msg.Order.Status)
	}
	if status == exchanges.OrderStatusUnknown {
		status = normalizeOrderStatus(raw.Status)
	}

	filled := raw.OrigSize.Sub(raw.RemainingSize)
	if filled.IsNegative() {
		filled = decimal.Zero
	}

	order := &exchanges.Order{
		OrderID:        raw.OrderID,
		ClientOrderID:  raw.ClientOrderID,
		Symbol:         a.ExtractSymbol(raw.Market),
		Side:           normalizeRESTOrderSide(&decibelrest.OpenOrder{Side: raw.Side, IsBuy: raw.IsBuy}),
		Type:           normalizeOrderType(raw.OrderType),
		Quantity:       raw.OrigSize,
		Price:          raw.Price,
		Status:         status,
		FilledQuantity: filled,
		ReduceOnly:     raw.IsReduceOnly,
		Timestamp:      raw.UnixMS,
	}
	exchanges.DerivePartialFillStatus(order)
	return order
}

func (a *Adapter) lookupOrderFromTransaction(ctx context.Context, txHash string, clientID string) (*exchanges.Order, error) {
	events, err := lookupDecibelTransactionOrderEvents(ctx, txHash)
	if err != nil {
		return nil, err
	}

	var matched *exchanges.Order
	for _, event := range events {
		order, ok := a.mapTransactionOrderEvent(event)
		if !ok {
			continue
		}
		if clientID != "" && order.ClientOrderID != clientID {
			continue
		}
		matched = order
	}
	return matched, nil
}

func (a *Adapter) metadataForSymbol(symbol string) (marketMetadata, error) {
	if a.markets == nil {
		return marketMetadata{}, exchanges.NewExchangeError("DECIBEL", "", fmt.Sprintf("symbol not found: %s", symbol), exchanges.ErrSymbolNotFound)
	}
	return a.markets.metadata(symbol)
}

func (a *Adapter) requireREST() error {
	if a.rest == nil {
		return fmt.Errorf("decibel rest client not configured")
	}
	return nil
}

func (a *Adapter) requireWS() error {
	if a.ws == nil {
		return fmt.Errorf("decibel websocket client not configured")
	}
	return nil
}

func (a *Adapter) requireTrading() error {
	if a.aptos == nil {
		return fmt.Errorf("decibel aptos client not configured")
	}
	return nil
}

func normalizeOrderSide(side string) exchanges.OrderSide {
	switch strings.ToUpper(strings.TrimSpace(side)) {
	case "SELL", "SHORT", "ASK":
		return exchanges.OrderSideSell
	default:
		return exchanges.OrderSideBuy
	}
}

func normalizeOrderType(orderType string) exchanges.OrderType {
	switch strings.ToUpper(strings.TrimSpace(orderType)) {
	case "LIMIT":
		return exchanges.OrderTypeLimit
	case "MARKET":
		return exchanges.OrderTypeMarket
	case "POST_ONLY", "POSTONLY":
		return exchanges.OrderTypePostOnly
	default:
		return exchanges.OrderTypeUnknown
	}
}

func normalizeRESTOrderSide(order *decibelrest.OpenOrder) exchanges.OrderSide {
	if order == nil {
		return exchanges.OrderSideBuy
	}
	if strings.TrimSpace(order.Side) != "" {
		return normalizeOrderSide(order.Side)
	}
	if order.IsBuy {
		return exchanges.OrderSideBuy
	}
	return exchanges.OrderSideSell
}

func syntheticBidAsk(ticker *exchanges.Ticker, tickSize decimal.Decimal) (decimal.Decimal, decimal.Decimal) {
	if ticker == nil {
		return decimal.Zero, decimal.Zero
	}
	bid := ticker.Bid
	ask := ticker.Ask
	ref := ticker.LastPrice
	if ref.IsZero() {
		ref = ticker.MarkPrice
	}
	if ref.IsZero() {
		ref = ticker.IndexPrice
	}
	if ref.IsZero() {
		return bid, ask
	}

	spread := tickSize
	if !spread.IsPositive() {
		spread = ref.Mul(decimal.RequireFromString("0.0001"))
	}
	if !spread.IsPositive() {
		spread = decimal.NewFromInt(1)
	}

	if !bid.IsPositive() {
		bid = ref.Sub(spread)
	}
	if !ask.IsPositive() {
		ask = ref.Add(spread)
	}
	if !bid.IsPositive() {
		bid = ref
	}
	if ask.LessThanOrEqual(bid) {
		ask = bid.Add(spread)
	}
	return bid, ask
}

func (a *Adapter) marketOrderPrice(ctx context.Context, meta marketMetadata, side exchanges.OrderSide) (decimal.Decimal, error) {
	ticker, err := a.FetchTicker(ctx, meta.BaseSymbol)
	if err != nil {
		return decimal.Zero, err
	}
	bid, ask := syntheticBidAsk(ticker, meta.TickSize)
	if side == exchanges.OrderSideBuy {
		return meta.quantizePrice(ask)
	}
	return meta.quantizePrice(bid)
}

type aptosTxEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type aptosTxResponse struct {
	Success  bool           `json:"success"`
	VmStatus string         `json:"vm_status"`
	Events   []aptosTxEvent `json:"events"`
}

type decibelOrderEventData struct {
	Market        string `json:"market"`
	OrderID       string `json:"order_id"`
	IsBid         bool   `json:"is_bid"`
	OrigSize      string `json:"orig_size"`
	RemainingSize string `json:"remaining_size"`
	Price         string `json:"price"`
	ClientOrderID struct {
		Vec []string `json:"vec"`
	} `json:"client_order_id"`
	Status struct {
		Variant string `json:"__variant__"`
	} `json:"status"`
	TimeInForce struct {
		Variant string `json:"__variant__"`
	} `json:"time_in_force"`
}

func fetchTransactionOrderEvents(ctx context.Context, txHash string) ([]aptosTxEvent, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.mainnet.aptoslabs.com/v1/transactions/by_hash/"+txHash, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("lookup transaction %s: %s", txHash, strings.TrimSpace(string(body)))
	}
	var tx aptosTxResponse
	if err := json.Unmarshal(body, &tx); err != nil {
		return nil, err
	}
	if !tx.Success {
		return nil, fmt.Errorf("transaction %s failed: %s", txHash, tx.VmStatus)
	}
	return tx.Events, nil
}

func (a *Adapter) mapTransactionOrderEvent(event aptosTxEvent) (*exchanges.Order, bool) {
	if !strings.HasSuffix(event.Type, "::market_types::OrderEvent") {
		return nil, false
	}
	var data decibelOrderEventData
	if err := json.Unmarshal(event.Data, &data); err != nil {
		return nil, false
	}
	meta, err := a.metadataForSymbol(a.ExtractSymbol(data.Market))
	if err != nil {
		return nil, false
	}

	priceUnits, err := decimal.NewFromString(data.Price)
	if err != nil {
		return nil, false
	}
	origUnits, err := decimal.NewFromString(data.OrigSize)
	if err != nil {
		return nil, false
	}
	remainingUnits, err := decimal.NewFromString(data.RemainingSize)
	if err != nil {
		return nil, false
	}
	price := priceUnits.Shift(-meta.PriceDecimals)
	qty := origUnits.Shift(-meta.SizeDecimals)
	remaining := remainingUnits.Shift(-meta.SizeDecimals)
	filled := qty.Sub(remaining)
	if filled.IsNegative() {
		filled = decimal.Zero
	}

	clientID := ""
	if len(data.ClientOrderID.Vec) > 0 {
		clientID = data.ClientOrderID.Vec[0]
	}
	order := &exchanges.Order{
		OrderID:        data.OrderID,
		ClientOrderID:  clientID,
		Symbol:         meta.BaseSymbol,
		Side:           boolToOrderSide(data.IsBid),
		Type:           normalizeOrderType(data.TimeInForce.Variant),
		Quantity:       qty,
		Price:          price,
		Status:         normalizeOrderStatus(data.Status.Variant),
		FilledQuantity: filled,
		Timestamp:      time.Now().UnixMilli(),
	}
	exchanges.DerivePartialFillStatus(order)
	return order, true
}

func boolToOrderSide(isBid bool) exchanges.OrderSide {
	if isBid {
		return exchanges.OrderSideBuy
	}
	return exchanges.OrderSideSell
}

func toAptosTimeInForce(orderType exchanges.OrderType, tif exchanges.TimeInForce) (decibelaptos.TimeInForce, error) {
	if orderType == exchanges.OrderTypeMarket {
		return decibelaptos.TimeInForceImmediateOrCancel, nil
	}

	switch tif {
	case "", exchanges.TimeInForceGTC:
		return decibelaptos.TimeInForceGoodTillCancelled, nil
	case exchanges.TimeInForcePO:
		return decibelaptos.TimeInForcePostOnly, nil
	case exchanges.TimeInForceIOC:
		return decibelaptos.TimeInForceImmediateOrCancel, nil
	case exchanges.TimeInForceFOK:
		return 0, unsupported("PlaceOrder")
	default:
		return 0, unsupported("PlaceOrder")
	}
}

func depthTopic(marketAddr string) string {
	return fmt.Sprintf("depth:%s:%s", marketAddr, defaultOrderBookAggregation)
}

func closeWS(client decibelWSClient) error {
	if client == nil {
		return nil
	}
	return client.Close()
}

func unsupported(method string) error {
	return exchanges.NewExchangeError("DECIBEL", "", method+" not supported", exchanges.ErrNotSupported)
}
