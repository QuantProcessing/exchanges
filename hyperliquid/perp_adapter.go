package hyperliquid

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	hyperliquid "github.com/QuantProcessing/exchanges/hyperliquid/sdk"
	"github.com/QuantProcessing/exchanges/hyperliquid/sdk/perp"

	"github.com/shopspring/decimal"
)

// Adapter Hyperliquid 适配器
type Adapter struct {
	*exchanges.BaseAdapter
	client   *perp.Client
	wsClient *perp.WebsocketClient

	accountAddr string
	privateKey  string

	// Symbol <-> AssetID 映射
	symbolToID map[string]int
	idToSymbol map[int]string
	metaMu     sync.RWMutex

	isConnected bool

	cancelMu sync.Mutex
	cancels  map[string]context.CancelFunc

	// Cached fee rate (account-level, not per-symbol)
	feeOnce       sync.Once
	cachedFeeRate *exchanges.FeeRate
	cachedFeeErr  error
}

// NewAdapter 创建 Hyperliquid 适配器
func NewAdapter(ctx context.Context, opts Options) (*Adapter, error) {
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
	client := perp.NewClient(baseClient)

	// Create lifecycle context
	baseWsClient := hyperliquid.NewWebsocketClient(ctx).WithCredentials(opts.PrivateKey, nil)
	baseWsClient.AccountAddr = accountAddr

	wsClient := perp.NewWebsocketClient(baseWsClient)

	base := exchanges.NewBaseAdapter("HYPERLIQUID", exchanges.MarketTypePerp, opts.logger())

	a := &Adapter{
		BaseAdapter: base,
		client:      client,
		wsClient:    wsClient,
		accountAddr: accountAddr,
		privateKey:  opts.PrivateKey,
		symbolToID:  make(map[string]int),
		idToSymbol:  make(map[int]string),
		cancels:     make(map[string]context.CancelFunc),
	}
	// Hyperliquid perp is a controlled hybrid: REST and WS can both carry orders.
	// Init metadata
	if err := a.RefreshSymbolDetails(context.Background()); err != nil {
		return nil, fmt.Errorf("hyperliquid init: %w", err)
	}

	// TODO: logger.Info("Initialized Hyperliquid Adapter")
	return a, nil
}

func (a *Adapter) WsAccountConnected(ctx context.Context) error {
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

func (a *Adapter) WsMarketConnected(ctx context.Context) error {
	if a.wsClient.Conn == nil {
		if err := a.wsClient.Connect(); err != nil {
			return err
		}
	}
	return nil
}

func (a *Adapter) WsOrderConnected(ctx context.Context) error {
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

func (a *Adapter) refreshMeta(ctx context.Context) error {
	meta, err := a.client.GetPrepMeta(ctx)
	if err != nil {
		return err
	}

	a.metaMu.Lock()
	a.symbolToID = make(map[string]int)
	a.idToSymbol = make(map[int]string)

	symbols := make(map[string]*exchanges.SymbolDetails)
	for i, asset := range meta.Universe {
		a.symbolToID[asset.Name] = i
		a.idToSymbol[i] = asset.Name

		details := &exchanges.SymbolDetails{
			Symbol:            asset.Name,
			PricePrecision:    5,
			QuantityPrecision: int32(asset.SzDecimals),
			MinQuantity:       decimal.New(1, -int32(asset.SzDecimals)),
			MinNotional:       decimal.NewFromInt(10),
		}
		symbols[asset.Name] = details
	}
	a.metaMu.Unlock()
	a.SetSymbolDetails(symbols)
	return nil
}

func (a *Adapter) IsConnected(ctx context.Context) (bool, error) {
	return a.isConnected, nil // Simple check
}

func (a *Adapter) Close() error {
	a.wsClient.Close()
	return nil
}

func (a *Adapter) RefreshSymbolDetails(ctx context.Context) error {
	return a.refreshMeta(ctx)
}

func (a *Adapter) FormatSymbol(symbol string) string {
	return strings.ToUpper(symbol)
}

func (a *Adapter) ExtractSymbol(symbol string) string {
	return strings.ToUpper(symbol)
}

// ================= Account & Trading =================

func (a *Adapter) FetchAccount(ctx context.Context) (*exchanges.Account, error) {
	if err := a.requireAccountAccess(); err != nil {
		return nil, err
	}
	perpPos, err := a.client.GetPerpPosition(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get perp position: %w", err)
	}

	openOrders, err := a.client.UserOpenOrders(ctx, a.accountAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get open orders: %w", err)
	}

	account := &exchanges.Account{
		Positions: []exchanges.Position{},
		Orders:    []exchanges.Order{},
	}

	accountValue, _ := strconv.ParseFloat(perpPos.MarginSummary.AccountValue, 64)
	withdrawable, _ := strconv.ParseFloat(perpPos.Withdrawable, 64)

	account.TotalBalance = decimal.NewFromFloat(accountValue)
	account.AvailableBalance = decimal.NewFromFloat(withdrawable)

	for _, ap := range perpPos.AssetPositions {
		pos := ap.Position
		if pos.Coin == "" {
			continue
		}

		quantity, _ := strconv.ParseFloat(pos.Szi, 64)
		if quantity == 0 {
			continue
		}

		entryPrice, _ := strconv.ParseFloat(pos.EntryPx, 64)
		unrealizedPnL, _ := strconv.ParseFloat(pos.UnrealizedPnl, 64)
		liquidationPrice, _ := strconv.ParseFloat(pos.LiquidationPx, 64)
		leverageVal := float64(pos.Leverage.Value)
		// marginUsed, _ := strconv.ParseFloat(pos.MarginUsed, 64)

		side := exchanges.PositionSideLong
		if quantity < 0 {
			side = exchanges.PositionSideShort
		}

		account.Positions = append(account.Positions, exchanges.Position{
			Symbol:           pos.Coin,
			Side:             side,
			Quantity:         decimal.NewFromFloat(quantity),
			EntryPrice:       decimal.NewFromFloat(entryPrice),
			UnrealizedPnL:    decimal.NewFromFloat(unrealizedPnL),
			LiquidationPrice: decimal.NewFromFloat(liquidationPrice),
			Leverage:         decimal.NewFromFloat(leverageVal),
			MarginType:       pos.Leverage.Type,
		})
	}

	for _, o := range openOrders {
		order, err := a.normalizeOrder(o)
		if err == nil {
			account.Orders = append(account.Orders, *order)
		}
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
		// Hyperliquid specific: Price uses 5 significant figures, not fixed decimals.
		// Re-apply significant figures rounding.
		if params.Type == exchanges.OrderTypeLimit || params.Price.IsPositive() {
			params.Price = exchanges.RoundToSignificantFigures(params.Price, details.PricePrecision)
		}
	}

	if err := a.WsOrderConnected(ctx); err != nil {
		return nil, err
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

	req := perp.PlaceOrderRequest{
		AssetID:       assetID,
		IsBuy:         isBuy,
		Price:         params.Price.InexactFloat64(),
		Size:          params.Quantity.InexactFloat64(),
		ReduceOnly:    params.ReduceOnly,
		ClientOrderID: cloid,
		OrderType:     a.mapOrderType(params),
	}

	// REST mode: use HTTP client directly
	if a.IsRESTMode() {
		status, err := a.client.PlaceOrder(ctx, req)
		if err != nil {
			return nil, err
		}
		var oid int64
		if status.Resting != nil {
			oid = status.Resting.Oid
		} else if status.Filled != nil {
			oid = int64(status.Filled.Oid)
		}
		return &exchanges.Order{
			OrderID:       fmt.Sprintf("%d", oid),
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

	// WS mode
	if err := a.WsOrderConnected(ctx); err != nil {
		return nil, err
	}

	ch, err := a.wsClient.PlaceOrder(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("hyperliquid place order failed: %w", err)
	}

	res := <-ch
	if res.Error != nil {
		return nil, fmt.Errorf("place order error: %v", res.Error)
	}

	// Parse response
	// Response payload matches: {"status":"ok","response":{"type":"order","data":{"statuses":[{"filling":...} OR {"error":...}]}}}
	var respPayload struct {
		Status   string `json:"status"`
		Response struct {
			Type string `json:"type"`
			Data struct {
				Statuses []struct {
					Error   string `json:"error"`
					Filling struct {
						Oid int64 `json:"oid"`
					} `json:"filling"`
					Resting struct {
						Oid int64 `json:"oid"`
					} `json:"resting"`
					Filled struct {
						TotalSz string `json:"totalSz"`
						AvgPx   string `json:"avgPx"`
						Oid     int64  `json:"oid"`
					} `json:"filled"`
				} `json:"statuses"`
			} `json:"data"`
		} `json:"response"`
	}

	if err := json.Unmarshal(res.Response.Payload, &respPayload); err != nil {
		// TODO: logger.Error("Failed to unmarshal place order response", zap.Error(err), zap.String("payload", string(res.Response.Payload)))
	} else {
		if respPayload.Status == "ok" && len(respPayload.Response.Data.Statuses) > 0 {
			status := respPayload.Response.Data.Statuses[0]
			if status.Error != "" {
				return nil, fmt.Errorf("hyperliquid API error: %s", status.Error)
			}
			// Capture OID
			var oid int64
			if status.Filling.Oid > 0 {
				oid = status.Filling.Oid
			} else if status.Resting.Oid > 0 {
				oid = status.Resting.Oid
			} else if status.Filled.Oid > 0 {
				oid = status.Filled.Oid
			}

			if oid > 0 {
				// TODO: logger.Debug("PlaceOrder Success", zap.Int64("oid", oid))
				// Return success with OID
				return &exchanges.Order{
					OrderID:       fmt.Sprintf("%d", oid),
					ClientOrderID: params.ClientID,
					Symbol:        params.Symbol,
					Side:          params.Side,
					Type:          params.Type,
					Quantity:      params.Quantity,
					Price:         params.Price,
					Status:        exchanges.OrderStatusPending, // or New
					Timestamp:     time.Now().UnixMilli(),
				}, nil
			}
		} else if respPayload.Status == "error" {
			// Try to parse error from response if available (structure might differ)
			return nil, fmt.Errorf("hyperliquid API error status: %v", string(res.Response.Payload))
		}
	}

	// Fallback if parsing fails or structure mismatches (shouldn't happen on success)
	return &exchanges.Order{
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

// PlaceMarketOrder places a market order (aggressive limit)
func (a *Adapter) CancelOrder(ctx context.Context, orderID, symbol string) error {
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

	req := perp.CancelOrderRequest{
		AssetID: assetID,
		OrderID: oid,
	}

	// REST mode
	if a.IsRESTMode() {
		_, err := a.client.CancelOrder(ctx, req)
		return err
	}

	// WS mode
	if err := a.WsOrderConnected(ctx); err != nil {
		return err
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

func (a *Adapter) ModifyOrder(ctx context.Context, orderID, symbol string, params *exchanges.ModifyOrderParams) (*exchanges.Order, error) {
	if err := a.requireWriteAccess(); err != nil {
		return nil, err
	}
	if err := a.WsOrderConnected(ctx); err != nil {
		return nil, err
	}
	// Logic from trader: fetch original order, cancel replace (Modify API)
	assetID, ok := a.getAssetID(symbol)
	if !ok {
		return nil, fmt.Errorf("unknown symbol: %s", symbol)
	}
	oid, err := strconv.ParseInt(orderID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid order id: %s", orderID)
	}

	origOrder, err := a.FetchOrderByID(ctx, orderID, symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch original order: %w", err)
	}
	isBuy := origOrder.Side == exchanges.OrderSideBuy

	req := perp.ModifyOrderRequest{
		Oid: &oid,
		Order: perp.PlaceOrderRequest{
			AssetID:    assetID,
			IsBuy:      isBuy,
			Price:      params.Price.InexactFloat64(),
			Size:       params.Quantity.InexactFloat64(),
			ReduceOnly: false, // Assuming false or need from origOrder
			OrderType: perp.OrderType{
				Limit: &perp.OrderTypeLimit{
					Tif: hyperliquid.TifGtc,
				},
			},
		},
	}

	ch, err := a.wsClient.ModifyOrder(ctx, req)
	if err != nil {
		return nil, err
	}
	res := <-ch
	if res.Error != nil {
		return nil, fmt.Errorf("modify order error: %v", res.Error)
	}

	return &exchanges.Order{
		OrderID:   orderID,
		Symbol:    symbol,
		Quantity:  params.Quantity,
		Price:     params.Price,
		Status:    exchanges.OrderStatusPending,
		Timestamp: time.Now().UnixMilli(),
	}, nil
}

func (a *Adapter) FetchOrderByID(ctx context.Context, orderID, symbol string) (*exchanges.Order, error) {
	if err := a.requireAccountAccess(); err != nil {
		return nil, err
	}
	oid, err := strconv.ParseInt(orderID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid order id: %s", orderID)
	}
	status, err := a.client.OrderStatus(ctx, a.accountAddr, oid)
	if err != nil {
		if isHyperliquidOrderLookupMiss(err) {
			return nil, exchanges.ErrOrderNotFound
		}
		return nil, err
	}
	order, err := a.normalizeOrderStatus(status)
	if err != nil {
		return nil, err
	}
	if symbol != "" && a.FormatSymbol(order.Symbol) != a.FormatSymbol(symbol) {
		return nil, exchanges.ErrOrderNotFound
	}
	return order, nil
}

func isHyperliquidOrderLookupMiss(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "order") &&
		(strings.Contains(msg, "not found") ||
			strings.Contains(msg, "unknown") ||
			strings.Contains(msg, "never placed"))
}

func (a *Adapter) FetchOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	_ = ctx
	_ = symbol
	return nil, exchanges.ErrNotSupported
}

func (a *Adapter) FetchOpenOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	if err := a.requireAccountAccess(); err != nil {
		return nil, err
	}
	openOrders, err := a.client.UserOpenOrders(ctx, a.accountAddr)
	if err != nil {
		return nil, err
	}
	orders := []exchanges.Order{}
	for _, o := range openOrders {
		if symbol != "" && o.Coin != symbol {
			continue
		}
		normalized, err := a.normalizeOrder(o)
		if err == nil {
			orders = append(orders, *normalized)
		}
	}
	return orders, nil
}

func (a *Adapter) CancelAllOrders(ctx context.Context, symbol string) error {
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

func (a *Adapter) SetLeverage(ctx context.Context, symbol string, leverage int) error {
	if err := a.requireWriteAccess(); err != nil {
		return err
	}
	assetID, ok := a.getAssetID(symbol)
	if !ok {
		return fmt.Errorf("unknown symbol: %s", symbol)
	}
	req := perp.UpdateLeverageRequest{
		AssetID:  assetID,
		IsCross:  false,
		Leverage: leverage,
	}
	return a.client.UpdateLeverage(ctx, req)
}

func (a *Adapter) FetchFeeRate(ctx context.Context, symbol string) (*exchanges.FeeRate, error) {
	if err := a.requireAccountAccess(); err != nil {
		return nil, err
	}
	a.feeOnce.Do(func() {
		fees, err := a.client.GetUserFees(ctx)
		if err != nil {
			a.cachedFeeErr = err
			return
		}

		var maker, taker decimal.Decimal

		// Prefer UserAddRate / UserCrossRate when non-empty — these are the
		// actual rates computed by Hyperliquid including all discounts.
		if fees.FeeSchedule.UserAddRate != "" && fees.FeeSchedule.UserCrossRate != "" {
			maker, _ = decimal.NewFromString(fees.FeeSchedule.UserAddRate)
			taker, _ = decimal.NewFromString(fees.FeeSchedule.UserCrossRate)
		} else {
			// Fallback: base rate × (1 - referralDiscount)
			referralDiscount, _ := decimal.NewFromString(fees.FeeSchedule.ReferralDiscount)
			add, _ := decimal.NewFromString(fees.FeeSchedule.Add)
			cross, _ := decimal.NewFromString(fees.FeeSchedule.Cross)

			one := decimal.NewFromInt(1)
			discount := one.Sub(referralDiscount)

			maker = add.Mul(discount)
			taker = cross.Mul(discount)
		}

		a.cachedFeeRate = &exchanges.FeeRate{
			Maker: maker,
			Taker: taker,
		}
	})
	if a.cachedFeeErr != nil {
		return nil, a.cachedFeeErr
	}
	return a.cachedFeeRate, nil
}

// ================= Market Data =================

func (a *Adapter) FetchTicker(ctx context.Context, symbol string) (*exchanges.Ticker, error) {
	l2, err := a.client.L2Book(ctx, symbol)
	if err != nil {
		return nil, err
	}

	ticker := &exchanges.Ticker{
		Symbol:    symbol,
		Timestamp: l2.Time,
	}

	if len(l2.Levels[0]) > 0 {
		ticker.Bid = parseDecimal(l2.Levels[0][0].Px)
	}
	if len(l2.Levels[1]) > 0 {
		ticker.Ask = parseDecimal(l2.Levels[1][0].Px)
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

func (a *Adapter) FetchOrderBook(ctx context.Context, symbol string, limit int) (*exchanges.OrderBook, error) {
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
		if limit > 0 && len(ob.Bids) >= limit {
			break
		}
	}

	for _, as := range l2.Levels[1] {
		px := parseDecimal(as.Px)
		sz := parseDecimal(as.Sz)
		ob.Asks = append(ob.Asks, exchanges.Level{Price: px, Quantity: sz})
		if limit > 0 && len(ob.Asks) >= limit {
			break
		}
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

func (a *Adapter) FetchTrades(ctx context.Context, symbol string, limit int) ([]exchanges.Trade, error) {
	return []exchanges.Trade{}, nil
}

// ================= WebSocket =================

func (a *Adapter) WatchOrders(ctx context.Context, callback exchanges.OrderUpdateCallback) error {
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
			order := a.mapWsOrderUpdate(u)
			callback(order)
		}
	})
}

func (a *Adapter) mapWsOrderUpdate(u hyperliquid.WsOrderUpdate) *exchanges.Order {
	o := u.Order
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
		status = exchanges.OrderStatusNew // triggered order becomes active
	case hyperliquid.StatusRejected,
		hyperliquid.StatusTickRejected,
		hyperliquid.StatusMinTradeNtlRejected:
		status = exchanges.OrderStatusRejected
	}

	side := exchanges.OrderSideBuy
	if o.Side != "B" {
		side = exchanges.OrderSideSell
	}

	price := parseHlFloat(o.LimitPx)
	qty := parseHlFloat(o.OrigSz)
	remaining := parseHlFloat(o.Sz)
	filled := qty.Sub(remaining)

	if status == exchanges.OrderStatusNew && filled.IsPositive() {
		status = exchanges.OrderStatusPartiallyFilled
	}

	return &exchanges.Order{
		OrderID:        fmt.Sprintf("%d", o.Oid),
		ClientOrderID:  o.Cliod,
		Symbol:         o.Coin,
		Side:           side,
		Status:         status,
		Price:          price,
		Quantity:       qty,
		FilledQuantity: filled,
		Timestamp:      u.StatusTimestamp,
	}
}

func (a *Adapter) WatchPositions(ctx context.Context, callback exchanges.PositionUpdateCallback) error {
	if err := a.WsAccountConnected(ctx); err != nil {
		return err
	}
	return a.wsClient.SubscribeWebData2(a.accountAddr, func(pos perp.PerpPosition) {
		for _, ap := range pos.AssetPositions {
			q, _ := strconv.ParseFloat(ap.Position.Szi, 64)
			if q == 0 {
				continue
			}

			entryPrice, _ := strconv.ParseFloat(ap.Position.EntryPx, 64)
			unrealizedPnL, _ := strconv.ParseFloat(ap.Position.UnrealizedPnl, 64)
			liquidationPrice, _ := strconv.ParseFloat(ap.Position.LiquidationPx, 64)

			side := exchanges.PositionSideLong
			if q < 0 {
				side = exchanges.PositionSideShort
			}

			callback(&exchanges.Position{
				Symbol:           ap.Position.Coin,
				Side:             side,
				Quantity:         decimal.NewFromFloat(q),
				EntryPrice:       decimal.NewFromFloat(entryPrice),
				UnrealizedPnL:    decimal.NewFromFloat(unrealizedPnL),
				LiquidationPrice: decimal.NewFromFloat(liquidationPrice),
				MarginType:       "CROSSED", // default?
			})
		}
	})
}

func (a *Adapter) WatchTicker(ctx context.Context, symbol string, callback exchanges.TickerCallback) error {
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

func (a *Adapter) WatchKlines(ctx context.Context, symbol string, interval exchanges.Interval, callback exchanges.KlineCallback) error {
	return exchanges.ErrNotSupported
}

func (a *Adapter) WatchTrades(ctx context.Context, symbol string, callback exchanges.TradeCallback) error {
	if err := a.WsMarketConnected(ctx); err != nil {
		return err
	}
	return a.wsClient.SubscribeTrades(symbol, func(events []hyperliquid.WsTrade) {
		for _, t := range events {
			side := exchanges.TradeSideBuy
			if t.Side == "B" {
				side = exchanges.TradeSideBuy
			} else {
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

func (a *Adapter) StopWatchOrders(ctx context.Context) error    { return nil }
func (a *Adapter) StopWatchPositions(ctx context.Context) error { return nil }
func (a *Adapter) StopWatchTicker(ctx context.Context, symbol string) error {
	return a.wsClient.UnsubscribeBbo(symbol)
}
func (a *Adapter) WatchOrderBook(ctx context.Context, symbol string, depth int, cb exchanges.OrderBookCallback) error {
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

	err := a.wsClient.SubscribeL2Book(symbol, func(data hyperliquid.WsL2Book) {
		ob.ProcessSnapshot(data)
		if cb != nil {
			snapshot := a.localOrderBookSnapshot(formattedSymbol, depth)
			if snapshot != nil {
				snapshot.Symbol = symbol
				cb(snapshot)
			}
		}
	})
	if err != nil {
		return err
	}
	return a.BaseAdapter.WaitOrderBookReady(ctx, formattedSymbol)
}

func (a *Adapter) StopWatchOrderBook(ctx context.Context, symbol string) error {
	formattedSymbol := a.FormatSymbol(symbol)
	a.cancelMu.Lock()
	if cancel, ok := a.cancels[formattedSymbol]; ok {
		cancel()
		delete(a.cancels, formattedSymbol)
	}
	a.cancelMu.Unlock()
	a.RemoveLocalOrderBook(formattedSymbol)
	return a.wsClient.UnsubscribeL2Book(symbol)
}
func (a *Adapter) StopWatchKlines(ctx context.Context, symbol string, interval exchanges.Interval) error {
	return exchanges.ErrNotSupported
}
func (a *Adapter) StopWatchTrades(ctx context.Context, symbol string) error {
	return a.wsClient.UnsubscribeTrades(symbol)
}

func (a *Adapter) FetchSymbolDetails(ctx context.Context, symbol string) (*exchanges.SymbolDetails, error) {
	details, err := a.GetSymbolDetail(symbol)
	if err != nil {
		return nil, fmt.Errorf("symbol not found: %s", symbol)
	}
	return details, nil
}

// Helpers

func (a *Adapter) requireAccountAccess() error {
	if a.accountAddr == "" {
		return exchanges.NewExchangeError("HYPERLIQUID", "", "account_addr or private_key is required for account access", exchanges.ErrAuthFailed)
	}
	return nil
}

func (a *Adapter) requireWriteAccess() error {
	if err := a.requireAccountAccess(); err != nil {
		return err
	}
	if a.privateKey == "" {
		return exchanges.NewExchangeError("HYPERLIQUID", "", "private_key is required for trading operations", exchanges.ErrAuthFailed)
	}
	return nil
}

func (a *Adapter) getAssetID(symbol string) (int, bool) {
	s := a.FormatSymbol(symbol)
	a.metaMu.RLock()
	defer a.metaMu.RUnlock()
	id, ok := a.symbolToID[s]
	return id, ok
}

func (a *Adapter) normalizeOrder(o perp.Order) (*exchanges.Order, error) {
	price := parseDecimal(o.LimitPx)
	qty := parseDecimal(o.Sz)
	side := exchanges.OrderSideBuy
	if o.Side != "B" {
		side = exchanges.OrderSideSell
	}

	return &exchanges.Order{
		OrderID:   fmt.Sprintf("%d", o.Oid),
		Symbol:    o.Coin,
		Side:      side,
		Type:      exchanges.OrderTypeLimit,
		Quantity:  qty,
		Price:     price,
		Status:    exchanges.OrderStatusNew,
		Timestamp: o.Timestamp,
	}, nil
}

func (a *Adapter) normalizeOrderStatus(o *perp.OrderStatusInfo) (*exchanges.Order, error) {
	price := parseDecimal(o.LimitPx)
	qty := parseDecimal(o.Sz)
	filled := parseDecimal(o.FilledSz)
	side := exchanges.OrderSideBuy
	if o.Side != "B" {
		side = exchanges.OrderSideSell
	}

	status := exchanges.OrderStatusUnknown
	switch hyperliquid.OrderStatusValue(o.Status) {
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
		status = exchanges.OrderStatusNew // triggered order becomes active
	case hyperliquid.StatusRejected,
		hyperliquid.StatusTickRejected,
		hyperliquid.StatusMinTradeNtlRejected:
		status = exchanges.OrderStatusRejected
	}

	order := &exchanges.Order{
		OrderID:        fmt.Sprintf("%d", o.Oid),
		Symbol:         o.Coin,
		Side:           side,
		Quantity:       qty,
		Price:          price,
		Status:         status,
		FilledQuantity: filled,
		Timestamp:      o.Timestamp,
	}
	exchanges.DerivePartialFillStatus(order)
	return order, nil
}

func parseDecimal(s string) decimal.Decimal {
	d, _ := decimal.NewFromString(s)
	return d
}

func parseHlFloat(s string) decimal.Decimal {
	if s == "" {
		return decimal.Zero
	}
	d, _ := decimal.NewFromString(s)
	return d
}

func intervalToMillis(i exchanges.Interval) int64 {
	switch i {
	case exchanges.Interval1m:
		return 60 * 1000
	case exchanges.Interval5m:
		return 5 * 60 * 1000
	case exchanges.Interval15m:
		return 15 * 60 * 1000
	case exchanges.Interval1h:
		return 60 * 60 * 1000
	case exchanges.Interval4h:
		return 4 * 60 * 60 * 1000
	case exchanges.Interval1d:
		return 24 * 60 * 60 * 1000
	}
	return 60 * 1000
}

func (a *Adapter) mapOrderType(params *exchanges.OrderParams) perp.OrderType {
	var ot perp.OrderType
	switch params.Type {
	case exchanges.OrderTypeLimit:
		tif := hyperliquid.TifGtc
		switch params.TimeInForce {
		case exchanges.TimeInForceIOC:
			tif = hyperliquid.TifIoc
		case exchanges.TimeInForceFOK:
			tif = hyperliquid.TifFok
		case exchanges.TimeInForcePO:
			tif = "Alo"
		}
		ot.Limit = &perp.OrderTypeLimit{Tif: tif}
	case exchanges.OrderTypeMarket:
		// For Market, we use aggressive limit IOC or similar if price is provided, or frontend logic.
		// However, standard market order usually implies IOC or specific Trigger.
		// Detailed logic was removed from PlaceOrder, so we should replicate basic expectation:
		// If Price is 0 (Pure Market), we might default to aggressive limit if we have price,
		// but here we just set it to simple IOC limit if price is set, or rely on caller to set price for "Market".
		// NOTE: Hyperliquid "Market" is typically an aggressive Limit IOC.
		ot.Limit = &perp.OrderTypeLimit{Tif: hyperliquid.TifIoc}
	case exchanges.OrderTypePostOnly:
		ot.Limit = &perp.OrderTypeLimit{Tif: "Alo"}
	default:
		// Default to Limit Gtc
		ot.Limit = &perp.OrderTypeLimit{Tif: hyperliquid.TifGtc}
	}
	return ot
}

// WaitOrderBookReady waits for orderbook to be ready
func (a *Adapter) WaitOrderBookReady(ctx context.Context, symbol string) error {
	formattedSymbol := a.FormatSymbol(symbol)
	return a.BaseAdapter.WaitOrderBookReady(ctx, formattedSymbol)
}

// GetLocalOrderBook get local orderbook
func (a *Adapter) GetLocalOrderBook(symbol string, depth int) *exchanges.OrderBook {
	formattedSymbol := a.FormatSymbol(symbol)
	snapshot := a.localOrderBookSnapshot(formattedSymbol, depth)
	if snapshot == nil {
		return nil
	}
	snapshot.Symbol = symbol
	return snapshot
}

func (a *Adapter) localOrderBookSnapshot(symbol string, depth int) *exchanges.OrderBook {
	ob, ok := a.GetLocalOrderBookImplementation(symbol)
	if !ok {
		return nil
	}

	nadoOb := ob.(*OrderBook)
	if !nadoOb.IsInitialized() {
		return nil
	}
	return nadoOb.ToAdapterOrderBook(depth)
}
