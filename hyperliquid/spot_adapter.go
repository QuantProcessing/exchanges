package hyperliquid

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	hyperliquid "github.com/QuantProcessing/exchanges/hyperliquid/sdk"
	"github.com/QuantProcessing/exchanges/hyperliquid/sdk/spot"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/shopspring/decimal"
)

// SpotAdapter Hyperliquid 现货适配器
type SpotAdapter struct {
	*exchanges.BaseAdapter
	client   *spot.Client
	wsClient *spot.WebsocketClient

	accountAddr string
	privateKey  string

	// Symbol <-> AssetID 映射
	symbolToID map[string]int
	idToSymbol map[int]string
	metaMu     sync.RWMutex

	isConnected bool

	cancelMu sync.Mutex
	cancels  map[string]context.CancelFunc
}

// NewSpotAdapter 创建 Hyperliquid 现货适配器
func NewSpotAdapter(ctx context.Context, opts Options) (*SpotAdapter, error) {
	if _, err := opts.quoteCurrency(); err != nil {
		return nil, err
	}
	if err := opts.validateCredentials(); err != nil {
		return nil, err
	}

	accountAddr := opts.accountAddr()
	baseClient := hyperliquid.NewClient().WithCredentials(opts.PrivateKey, nil)
	if accountAddr != "" {
		baseClient = baseClient.WithAccount(accountAddr)
	}
	client := spot.NewClient(baseClient)

	// Create lifecycle context
	baseWsClient := hyperliquid.NewWebsocketClient(ctx).WithCredentials(opts.PrivateKey, nil)
	baseWsClient.AccountAddr = accountAddr

	wsClient := spot.NewWebsocketClient(baseWsClient)

	a := &SpotAdapter{
		BaseAdapter: exchanges.NewBaseAdapter("HYPERLIQUID", exchanges.MarketTypeSpot, opts.logger()),
		client:      client,
		wsClient:    wsClient,
		accountAddr: accountAddr,
		privateKey:  opts.PrivateKey,
		symbolToID:  make(map[string]int),
		idToSymbol:  make(map[int]string),
		cancels:     make(map[string]context.CancelFunc),
	}
	// Hyperliquid spot uses WS-only private order transport in this adapter.
	// Init metadata
	if err := a.RefreshSymbolDetails(context.Background()); err != nil {
		return nil, fmt.Errorf("hyperliquid spot init: %w", err)
	}

	// TODO: logger.Info("Initialized Hyperliquid Spot Adapter", zap.String("accountAddr", opts.AccountAddr))
	return a, nil
}

func (a *SpotAdapter) WithCredentials(apiKey, secretKey string) exchanges.Exchange {
	// TODO: logger.Warn("WithCredentials not supported for Hyperliquid, use NewSpotAdapter with config")
	return a
}

func (a *SpotAdapter) Close() error {
	a.wsClient.Close()
	return nil
}

func (a *SpotAdapter) WsAccountConnected(ctx context.Context) error {
	if err := a.requireAccountAccess(); err != nil {
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

func (a *SpotAdapter) refreshMeta(ctx context.Context) error {
	meta, err := a.client.GetSpotMeta(ctx)
	if err != nil {
		return err
	}

	a.metaMu.Lock()
	a.symbolToID = make(map[string]int)
	a.idToSymbol = make(map[int]string)

	symbols := make(map[string]*exchanges.SymbolDetails)
	tokens := meta.Tokens
	for _, u := range meta.Universe {
		// Only keep USDC pairs (tokens[1] == 0 means quote is USDC)
		// Skip USDE and other quote currencies to prevent overwriting
		if len(u.Tokens) < 2 || u.Tokens[1] != 0 {
			continue
		}

		tokenIndex := u.Tokens[0]   // token index
		token := tokens[tokenIndex] // token obj
		details := &exchanges.SymbolDetails{
			Symbol:            token.Name,
			PricePrecision:    int32(8 - token.SzDecimals), // Spot uses 8 - szDecimals
			QuantityPrecision: int32(token.SzDecimals),
			MinQuantity:       decimal.New(1, -int32(token.SzDecimals)),
			MinNotional:       decimal.NewFromInt(10), // Hyperliquid spot minimum is 10 USDC
		}
		symbols[token.Name] = details

		assetID := 10000 + u.Index // spot asset id = 10000 + universe index
		a.symbolToID[token.Name] = assetID
		a.idToSymbol[assetID] = token.Name
	}
	a.metaMu.Unlock()
	a.SetSymbolDetails(symbols)
	return nil
}

func (a *SpotAdapter) IsConnected(ctx context.Context) (bool, error) {
	return a.isConnected, nil
}

func (a *SpotAdapter) RefreshSymbolDetails(ctx context.Context) error {
	return a.refreshMeta(ctx)
}

func (a *SpotAdapter) FormatSymbol(symbol string) string {
	symbol = strings.ToUpper(symbol)
	if strings.HasSuffix(symbol, "/USDC") {
		return symbol[:len(symbol)-5]
	}
	return symbol
}

func (a *SpotAdapter) ExtractSymbol(symbol string) string {
	symbol = strings.ToUpper(symbol)
	if strings.HasSuffix(symbol, "/USDC") {
		return symbol[:len(symbol)-5]
	}
	// Hyperliquid WS sends spot coins as @{index} (e.g. "@107")
	if strings.HasPrefix(symbol, "@") {
		indexStr := symbol[1:]
		if idx, err := strconv.Atoi(indexStr); err == nil {
			assetID := 10000 + idx
			a.metaMu.RLock()
			name, ok := a.idToSymbol[assetID]
			a.metaMu.RUnlock()
			if ok {
				return name
			}
		}
	}
	return symbol
}

// ================= Account & Trading =================

func (a *SpotAdapter) FetchAccount(ctx context.Context) (*exchanges.Account, error) {
	if err := a.requireAccountAccess(); err != nil {
		return nil, err
	}
	account := &exchanges.Account{
		Positions: nil, // Spot has no positions
		Orders:    []exchanges.Order{},
	}

	// Get real spot balances
	balances, err := a.FetchSpotBalances(ctx)
	if err != nil {
		return nil, err
	}

	// Sum USD-like balances as total/available
	for _, b := range balances {
		if b.Asset == "USDC" || b.Asset == "USDE" || b.Asset == "USDCE" {
			account.TotalBalance = account.TotalBalance.Add(b.Total)
			account.AvailableBalance = account.AvailableBalance.Add(b.Free)
		}
	}

	return account, nil
}

func (a *SpotAdapter) FetchSpotBalances(ctx context.Context) ([]exchanges.SpotBalance, error) {
	if err := a.requireAccountAccess(); err != nil {
		return nil, err
	}
	res, err := a.client.GetBalance()
	if err != nil {
		return nil, err
	}

	var balances []exchanges.SpotBalance
	for _, b := range res.Balances {
		total := parseDecimal(b.Total)
		hold := parseDecimal(b.Hold)
		balances = append(balances, exchanges.SpotBalance{
			Asset:  b.Coin,
			Total:  total,
			Locked: hold,
			Free:   total.Sub(hold),
		})
	}
	return balances, nil
}

func (a *SpotAdapter) TransferAsset(ctx context.Context, params *exchanges.TransferParams) error {
	if err := a.requireWriteAccess(); err != nil {
		return err
	}
	if params.Asset != "USDC" && params.Asset != "USD" {
		return fmt.Errorf("only USDC transfer supported")
	}

	var toPerp bool
	if params.FromAccount == exchanges.AccountTypeSpot && params.ToAccount == exchanges.AccountTypePerp {
		toPerp = true
	} else if params.FromAccount == exchanges.AccountTypePerp && params.ToAccount == exchanges.AccountTypeSpot {
		toPerp = false
	} else {
		return fmt.Errorf("unsupported transfer direction: from %s to %s", params.FromAccount, params.ToAccount)
	}

	// Amount for UsdClassTransferAction is float64
	action := hyperliquid.UsdClassTransferAction{
		Type:   "usdClassTransfer",
		Amount: params.Amount.InexactFloat64(),
		ToPerp: toPerp,
	}

	timestamp := time.Now().UnixMilli()

	// The constructor validates configured private keys, so this should only fail
	// if the adapter was built manually without going through NewSpotAdapter.
	pk, err := crypto.HexToECDSA(a.privateKey)
	if err != nil {
		return exchanges.NewExchangeError("HYPERLIQUID", "", "invalid private_key", exchanges.ErrAuthFailed)
	}

	isMainnet := a.client.BaseURL == hyperliquid.MainnetAPIURL

	sig, err := hyperliquid.SignL1Action(pk, action, "", timestamp, nil, isMainnet)
	if err != nil {
		return fmt.Errorf("failed to sign transfer action: %w", err)
	}

	// Post action
	// Note: PostAction expects nonce to match timestamp used in signing
	_, err = a.client.PostAction(ctx, action, sig, timestamp)
	if err != nil {
		return fmt.Errorf("failed to post transfer action: %w", err)
	}

	return nil
}

func (a *SpotAdapter) FetchBalance(ctx context.Context) (decimal.Decimal, error) {
	acc, err := a.FetchAccount(ctx)
	if err != nil {
		return decimal.Zero, err
	}
	return acc.AvailableBalance, nil
}

func (a *SpotAdapter) FetchFeeRate(ctx context.Context, symbol string) (*exchanges.FeeRate, error) {
	// Placeholder fees for spot
	return &exchanges.FeeRate{Maker: decimal.NewFromFloat(0.0002), Taker: decimal.NewFromFloat(0.0005)}, nil
}

func (a *SpotAdapter) PlaceOrder(ctx context.Context, params *exchanges.OrderParams) (*exchanges.Order, error) {
	if err := a.requireWriteAccess(); err != nil {
		return nil, err
	}
	// Hyperliquid doesn't support true market orders (price=0 is rejected).
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
		if err := exchanges.ValidateAndFormatParams(params, details); err != nil {
			return nil, err
		}
		// Hyperliquid spot uses decimal precision AND max 5 significant figures
		if params.Type == exchanges.OrderTypeLimit || params.Price.IsPositive() {
			params.Price = a.formatPrice(params.Price, details.PricePrecision)
		}
	}

	assetID, ok := a.getAssetID(params.Symbol)
	if !ok {
		return nil, fmt.Errorf("unknown symbol: %s", params.Symbol)
	}

	isBuy := params.Side == exchanges.OrderSideBuy

	var cloid *string
	if params.ClientID != "" {
		cloid = &params.ClientID
	}

	req := spot.PlaceOrderRequest{
		AssetID:       assetID,
		IsBuy:         isBuy,
		Price:         params.Price.InexactFloat64(),
		Size:          params.Quantity.InexactFloat64(),
		ClientOrderID: cloid,
		OrderType:     a.mapOrderType(params),
	}

	status, err := a.client.PlaceOrder(ctx, req)
	if err != nil {
		return nil, err
	}
	return &exchanges.Order{
		OrderID:       extractSpotOrderID(status),
		ClientOrderID: params.ClientID,
		Symbol:        params.Symbol,
		Side:          params.Side,
		Type:          params.Type,
		Quantity:      params.Quantity,
		Price:         params.Price,
		Status:        exchanges.OrderStatusPending,
		Timestamp:     time.Now().UnixMilli(),
	}, nil
}

func (a *SpotAdapter) PlaceOrderWS(ctx context.Context, params *exchanges.OrderParams) error {
	if err := a.requireWriteAccess(); err != nil {
		return err
	}
	if strings.TrimSpace(params.ClientID) == "" {
		return fmt.Errorf("client id required for PlaceOrderWS")
	}
	if params.Type == exchanges.OrderTypeMarket && params.Slippage.IsZero() {
		params.Slippage = decimal.NewFromFloat(0.02)
	}
	if err := a.BaseAdapter.ApplySlippage(ctx, params, a.FetchTicker); err != nil {
		return err
	}
	details, err := a.FetchSymbolDetails(ctx, params.Symbol)
	if err == nil {
		if err := exchanges.ValidateAndFormatParams(params, details); err != nil {
			return err
		}
		if params.Type == exchanges.OrderTypeLimit || params.Price.IsPositive() {
			params.Price = a.formatPrice(params.Price, details.PricePrecision)
		}
	}
	if err := a.WsOrderConnected(ctx); err != nil {
		return err
	}
	assetID, ok := a.getAssetID(params.Symbol)
	if !ok {
		return fmt.Errorf("unknown symbol: %s", params.Symbol)
	}
	cloid := &params.ClientID
	req := spot.PlaceOrderRequest{
		AssetID:       assetID,
		IsBuy:         params.Side == exchanges.OrderSideBuy,
		Price:         params.Price.InexactFloat64(),
		Size:          params.Quantity.InexactFloat64(),
		ClientOrderID: cloid,
		OrderType:     a.mapOrderType(params),
	}
	ch, err := a.wsClient.PlaceOrder(ctx, req)
	if err != nil {
		return fmt.Errorf("hyperliquid place order failed: %w", err)
	}
	res := <-ch
	if res.Error != nil {
		return fmt.Errorf("place order error: %v", res.Error)
	}
	return nil
}

func (a *SpotAdapter) CancelOrder(ctx context.Context, orderID, symbol string) error {
	if err := a.requireWriteAccess(); err != nil {
		return err
	}
	assetID, ok := a.getAssetID(symbol)
	if !ok {
		return fmt.Errorf("unknown symbol: %s", symbol)
	}

	oid, err := strconv.ParseInt(orderID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid order id: %s", orderID)
	}

	req := spot.CancelOrderRequest{
		AssetID: assetID,
		OrderID: oid,
	}
	_, err = a.client.CancelOrder(ctx, req)
	return err
}

func (a *SpotAdapter) CancelOrderWS(ctx context.Context, orderID, symbol string) error {
	if err := a.requireWriteAccess(); err != nil {
		return err
	}
	if err := a.WsOrderConnected(ctx); err != nil {
		return err
	}
	assetID, ok := a.getAssetID(symbol)
	if !ok {
		return fmt.Errorf("unknown symbol: %s", symbol)
	}
	oid, err := strconv.ParseInt(orderID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid order id: %s", orderID)
	}
	req := spot.CancelOrderRequest{
		AssetID: assetID,
		OrderID: oid,
	}
	ch, err := a.wsClient.CancelOrder(ctx, req)
	if err != nil {
		return err
	}
	res := <-ch
	if res.Error != nil {
		return res.Error
	}
	return nil
}

func (a *SpotAdapter) ModifyOrder(ctx context.Context, orderID, symbol string, params *exchanges.ModifyOrderParams) (*exchanges.Order, error) {
	_ = ctx
	_ = orderID
	_ = symbol
	_ = params
	return nil, exchanges.ErrNotSupported
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

func (a *SpotAdapter) FetchOpenOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	_ = ctx
	_ = symbol
	return nil, exchanges.ErrNotSupported
}

func (a *SpotAdapter) CancelAllOrders(ctx context.Context, symbol string) error {
	orders, err := a.FetchOpenOrders(ctx, symbol)
	if err != nil {
		return err
	}
	for _, o := range orders {
		if err := a.CancelOrder(ctx, o.OrderID, o.Symbol); err != nil {
			return err
		}
	}
	return nil
}

// ================= Market Data =================

func (a *SpotAdapter) FetchTicker(ctx context.Context, symbol string) (*exchanges.Ticker, error) {
	// Use L2 book as ticker source (similar to perp)
	ob, err := a.FetchOrderBook(ctx, symbol, 1)
	if err != nil {
		return nil, err
	}

	ticker := &exchanges.Ticker{
		Symbol:    symbol,
		Timestamp: ob.Timestamp,
	}

	if len(ob.Bids) > 0 {
		ticker.Bid = ob.Bids[0].Price
	}
	if len(ob.Asks) > 0 {
		ticker.Ask = ob.Asks[0].Price
	}

	if ticker.Bid.IsPositive() && ticker.Ask.IsPositive() {
		ticker.LastPrice = ticker.Bid.Add(ticker.Ask).Div(decimal.NewFromInt(2))
	} else if ticker.Bid.IsPositive() {
		ticker.LastPrice = ticker.Bid
	} else {
		ticker.LastPrice = ticker.Ask
	}

	return ticker, nil
}

func (a *SpotAdapter) FetchOrderBook(ctx context.Context, symbol string, limit int) (*exchanges.OrderBook, error) {
	l2, err := a.client.L2Book(ctx, symbol)
	if err != nil {
		return nil, err
	}

	ob := &exchanges.OrderBook{
		Symbol:    symbol,
		Timestamp: l2.Time,
		Bids:      make([]exchanges.Level, 0, len(l2.Levels[0])),
		Asks:      make([]exchanges.Level, 0, len(l2.Levels[1])),
	}

	for _, b := range l2.Levels[0] {
		px := parseDecimal(b.Px)
		sz := parseDecimal(b.Sz)
		ob.Bids = append(ob.Bids, exchanges.Level{Price: px, Quantity: sz})
	}

	for _, as := range l2.Levels[1] {
		px := parseDecimal(as.Px)
		sz := parseDecimal(as.Sz)
		ob.Asks = append(ob.Asks, exchanges.Level{Price: px, Quantity: sz})
	}

	// Sort Bids descending
	sort.Slice(ob.Bids, func(i, j int) bool {
		return ob.Bids[i].Price.GreaterThan(ob.Bids[j].Price)
	})

	// Sort Asks ascending
	sort.Slice(ob.Asks, func(i, j int) bool {
		return ob.Asks[i].Price.LessThan(ob.Asks[j].Price)
	})

	// Apply limit after sorting
	if limit > 0 {
		if len(ob.Bids) > limit {
			ob.Bids = ob.Bids[:limit]
		}
		if len(ob.Asks) > limit {
			ob.Asks = ob.Asks[:limit]
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

	var startTime, endTime int64
	if end != nil {
		endTime = end.UnixMilli()
	} else {
		endTime = time.Now().UnixMilli()
	}
	if start != nil {
		startTime = start.UnixMilli()
	} else {
		startTime = endTime - int64(limit)*intervalToMillis(interval)
	}

	res, err := a.client.CandleSnapshot(ctx, symbol, exInterval, startTime, endTime)
	if err != nil {
		return nil, err
	}

	klines := make([]exchanges.Kline, len(res))
	for i, k := range res {
		klines[i] = exchanges.Kline{
			Symbol:    symbol,
			Interval:  interval,
			Timestamp: k.T,
			Open:      parseHlFloat(k.O),
			High:      parseHlFloat(k.H),
			Low:       parseHlFloat(k.L),
			Close:     parseHlFloat(k.C),
			Volume:    parseHlFloat(k.V),
		}
	}
	return klines, nil
}

func (a *SpotAdapter) FetchTrades(ctx context.Context, symbol string, limit int) ([]exchanges.Trade, error) {
	return []exchanges.Trade{}, nil
}

func (a *SpotAdapter) FetchSymbolDetails(ctx context.Context, symbol string) (*exchanges.SymbolDetails, error) {
	details, err := a.GetSymbolDetail(symbol)
	if err != nil {
		return nil, fmt.Errorf("symbol not found: %s", symbol)
	}
	return details, nil
}

func (a *SpotAdapter) formatPrice(price decimal.Decimal, precision int32) decimal.Decimal {
	// 1. Round to max 5 significant figures
	price = exchanges.RoundToSignificantFigures(price, 5)
	// 2. Round to max decimals (precision)
	price = exchanges.RoundToPrecision(price, precision)
	return price
}

// ================= WebSocket =================

func (a *SpotAdapter) WatchOrders(ctx context.Context, callback exchanges.OrderUpdateCallback) error {
	if err := a.WsAccountConnected(ctx); err != nil {
		return err
	}

	// orderUpdates provides complete order state (status + origSz + remaining sz)
	// for all lifecycle events: open, filled, partially filled, canceled, rejected, etc.
	// userFills is NOT subscribed here — it's a trade execution event (individual fill),
	// not an order status update. Each WsUserFill only has the fill sz for that specific trade
	// with no origSz to determine if the order is fully filled.
	return a.wsClient.SubscribeOrderUpdates(a.accountAddr, func(updates []hyperliquid.WsOrderUpdate) {
		for _, u := range updates {
			symbol := a.ExtractSymbol(u.Order.Coin)

			_, err := a.GetSymbolDetail(symbol)
			if err != nil {
				continue
			}

			// Map status using SDK enums
			status := exchanges.OrderStatusUnknown
			switch u.Status {
			case hyperliquid.StatusOpen:
				status = exchanges.OrderStatusNew
			case hyperliquid.StatusFilled:
				status = exchanges.OrderStatusFilled
			case hyperliquid.StatusCanceled,
				hyperliquid.StatusMarginCanceled,
				hyperliquid.StatusVaultWithdrawalCanceled,
				hyperliquid.StatusOpenInterestCapCanceled,
				hyperliquid.StatusSelfTradeCanceled,
				hyperliquid.StatusReduceOnlyCanceled,
				hyperliquid.StatusSiblingFilledCanceled,
				hyperliquid.StatusDelistedCanceled,
				hyperliquid.StatusLiquidatedCanceled,
				hyperliquid.StatusScheduledCancel:
				status = exchanges.OrderStatusCancelled
			case hyperliquid.StatusTriggered:
				status = exchanges.OrderStatusNew
			case hyperliquid.StatusRejected,
				hyperliquid.StatusTickRejected,
				hyperliquid.StatusMinTradeNtlRejected:
				status = exchanges.OrderStatusRejected
			}

			side := exchanges.OrderSideBuy
			if u.Order.Side == "A" {
				side = exchanges.OrderSideSell
			}

			// Calculate filled quantity
			origSz := parseHlFloat(u.Order.OrigSz)
			sz := parseHlFloat(u.Order.Sz)
			filledQty := origSz.Sub(sz)

			if status == exchanges.OrderStatusNew && filledQty.IsPositive() {
				status = exchanges.OrderStatusPartiallyFilled
			}

			callback(&exchanges.Order{
				OrderID:        fmt.Sprintf("%d", u.Order.Oid),
				ClientOrderID:  u.Order.Cliod,
				Symbol:         symbol,
				Side:           side,
				Price:          parseHlFloat(u.Order.LimitPx),
				OrderPrice:     parseHlFloat(u.Order.LimitPx),
				Quantity:       origSz,
				FilledQuantity: filledQty,
				Status:         status,
				Timestamp:      u.StatusTimestamp,
			})
		}
	})
}

func (a *SpotAdapter) WatchTicker(ctx context.Context, symbol string, callback exchanges.TickerCallback) error {
	if err := a.WsMarketConnected(ctx); err != nil {
		return err
	}
	return a.wsClient.SubscribeBbo(symbol, func(data hyperliquid.WsBbo) {
		if len(data.Bbo) < 2 {
			return
		}
		bid := parseHlFloat(data.Bbo[0].Px)
		ask := parseHlFloat(data.Bbo[1].Px)
		last := bid.Add(ask).Div(decimal.NewFromInt(2))

		callback(&exchanges.Ticker{
			Symbol:    data.Coin,
			Bid:       bid,
			Ask:       ask,
			LastPrice: last,
			Timestamp: data.Time,
		})
	})
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

	err := a.wsClient.SubscribeL2Book(formattedSymbol, func(data hyperliquid.WsL2Book) {
		ob.ProcessSnapshot(data)
		if callback != nil {
			callback(ob.ToAdapterOrderBook(depth))
		}
	})
	if err != nil {
		return err
	}
	formattedSymbol2 := a.FormatSymbol(symbol)
	return a.BaseAdapter.WaitOrderBookReady(ctx, formattedSymbol2)
}

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

// ================= Helper Functions =================

func (a *SpotAdapter) getAssetID(symbol string) (int, bool) {
	a.metaMu.RLock()
	defer a.metaMu.RUnlock()
	id, ok := a.symbolToID[a.FormatSymbol(symbol)]
	return id, ok
}

func (a *SpotAdapter) mapOrderType(params *exchanges.OrderParams) spot.OrderType {
	orderType := spot.OrderType{}

	switch params.Type {
	case exchanges.OrderTypeLimit, exchanges.OrderTypePostOnly:
		tif := hyperliquid.TifGtc
		if params.TimeInForce == exchanges.TimeInForceIOC {
			tif = hyperliquid.TifIoc
		}
		// PostOnly uses Gtc (Hyperliquid spot doesn't have specific PostOnly Tif)
		orderType.Limit = &spot.OrderTypeLimit{Tif: tif}
	case exchanges.OrderTypeMarket:
		// Use IOC limit order
		orderType.Limit = &spot.OrderTypeLimit{Tif: hyperliquid.TifIoc}
	}

	return orderType
}

func (a *SpotAdapter) WatchPositions(ctx context.Context, cb exchanges.PositionUpdateCallback) error {
	_ = ctx
	_ = cb
	return exchanges.ErrNotSupported
}

func (a *SpotAdapter) WatchKlines(ctx context.Context, symbol string, interval exchanges.Interval, callback exchanges.KlineCallback) error {
	_ = ctx
	_ = symbol
	_ = interval
	_ = callback
	return exchanges.ErrNotSupported
}

func (a *SpotAdapter) WatchTrades(ctx context.Context, symbol string, callback exchanges.TradeCallback) error {
	if err := a.WsMarketConnected(ctx); err != nil {
		return err
	}
	return a.wsClient.SubscribeTrades(symbol, func(events []hyperliquid.WsTrade) {
		for _, t := range events {
			side := exchanges.TradeSideBuy
			if t.Side != "B" {
				side = exchanges.TradeSideSell
			}
			callback(&exchanges.Trade{
				ID:        fmt.Sprintf("%d", t.Tid),
				Symbol:    symbol,
				Price:     parseHlFloat(t.Px),
				Quantity:  parseHlFloat(t.Sz),
				Side:      side,
				Timestamp: t.Time,
			})
		}
	})
}

func (a *SpotAdapter) StopWatchOrders(ctx context.Context) error { return nil }
func (a *SpotAdapter) WatchFills(ctx context.Context, callback exchanges.FillCallback) error {
	if err := a.WsAccountConnected(ctx); err != nil {
		return err
	}
	return a.wsClient.SubscribeUserFills(a.accountAddr, func(data hyperliquid.WsUserFills) {
		for _, fill := range data.Fills {
			callback(a.mapWsUserFill(fill))
		}
	})
}
func (a *SpotAdapter) StopWatchFills(ctx context.Context) error {
	_ = ctx
	if a.accountAddr == "" {
		return nil
	}
	return a.wsClient.Unsubscribe("userFills", map[string]string{
		"type": "userFills",
		"user": a.accountAddr,
	})
}
func (a *SpotAdapter) StopWatchPositions(ctx context.Context) error {
	_ = ctx
	return exchanges.ErrNotSupported
}
func (a *SpotAdapter) StopWatchTicker(ctx context.Context, symbol string) error {
	return a.wsClient.UnsubscribeBbo(symbol)
}
func (a *SpotAdapter) StopWatchKlines(ctx context.Context, symbol string, interval exchanges.Interval) error {
	_ = ctx
	_ = symbol
	_ = interval
	return exchanges.ErrNotSupported
}
func (a *SpotAdapter) StopWatchTrades(ctx context.Context, symbol string) error {
	return a.wsClient.UnsubscribeTrades(symbol)
}

func (a *SpotAdapter) WaitOrderBookReady(ctx context.Context, symbol string) error {
	formattedSymbol := a.FormatSymbol(symbol)
	return a.BaseAdapter.WaitOrderBookReady(ctx, formattedSymbol)
}

func (a *SpotAdapter) requireAccountAccess() error {
	if a.accountAddr == "" {
		return exchanges.NewExchangeError("HYPERLIQUID", "", "account_addr or private_key is required for account access", exchanges.ErrAuthFailed)
	}
	return nil
}

func (a *SpotAdapter) requireWriteAccess() error {
	if err := a.requireAccountAccess(); err != nil {
		return err
	}
	if a.privateKey == "" {
		return exchanges.NewExchangeError("HYPERLIQUID", "", "private_key is required for trading operations", exchanges.ErrAuthFailed)
	}
	return nil
}

func (a *SpotAdapter) mapWsUserFill(fill hyperliquid.WsUserFill) *exchanges.Fill {
	side := exchanges.OrderSideBuy
	if fill.Side != "B" {
		side = exchanges.OrderSideSell
	}

	return &exchanges.Fill{
		TradeID:   strconv.FormatInt(fill.Tid, 10),
		OrderID:   strconv.FormatInt(fill.Oid, 10),
		Symbol:    a.ExtractSymbol(fill.Coin),
		Side:      side,
		Price:     parseHlFloat(fill.Px),
		Quantity:  parseHlFloat(fill.Sz),
		Fee:       parseHlFloat(fill.Fee),
		FeeAsset:  fill.FeeToken,
		IsMaker:   !fill.Crossed,
		Timestamp: fill.Time,
	}
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
	return a.wsClient.UnsubscribeL2Book(formattedSymbol)
}
