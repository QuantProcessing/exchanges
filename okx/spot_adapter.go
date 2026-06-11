package okx

import (
	"context"
	"fmt"
	"strings"
	"sync"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/okx/sdk"

	"github.com/shopspring/decimal"
)

// SpotAdapter OKX 现货适配器
type SpotAdapter struct {
	*exchanges.BaseAdapter

	client    *okx.Client
	wsPublic  *okx.WSClient
	wsPrivate *okx.WSClient

	// Symbol mapping: Token -> InstId (e.g. BTC -> BTC-USDT)
	symbolMap     map[string]string         // BTC -> BTC-USDT
	idMap         map[string]string         // BTC-USDT -> BTC
	instruments   map[string]okx.Instrument // InstId -> Instrument
	quoteCurrency string                    // "USDT" or "USDC"

	mu sync.RWMutex

	privateOrderStream okxPrivateOrderStreamState
}

// NewSpotAdapter creates a new OKX spot adapter
func NewSpotAdapter(ctx context.Context, opts Options) (*SpotAdapter, error) {
	quote, err := opts.quoteCurrency()
	if err != nil {
		return nil, err
	}
	return newSpotAdapterWithClient(ctx, opts, quote, okx.NewClient())
}

func newSpotAdapterWithClient(ctx context.Context, opts Options, quote exchanges.QuoteCurrency, client *okx.Client) (*SpotAdapter, error) {
	if err := opts.validateCredentials(); err != nil {
		return nil, err
	}
	wsPublic := okx.NewWSClient(ctx)
	wsPrivate := okx.NewWSClient(ctx)

	if opts.hasFullCredentials() {
		client.WithCredentials(opts.APIKey, opts.SecretKey, opts.Passphrase)
		wsPrivate.WithCredentials(opts.APIKey, opts.SecretKey, opts.Passphrase)
	}

	a := &SpotAdapter{
		BaseAdapter:   exchanges.NewBaseAdapter("OKX", exchanges.MarketTypeSpot, opts.logger()),
		client:        client,
		wsPublic:      wsPublic,
		wsPrivate:     wsPrivate,
		symbolMap:     make(map[string]string),
		idMap:         make(map[string]string),
		instruments:   make(map[string]okx.Instrument),
		quoteCurrency: string(quote),
	}

	// Load Instruments
	if err := a.RefreshSymbolDetails(context.Background()); err != nil {
		// TODO: logger.Error("Failed to load instruments", zap.Error(err))
	}

	// TODO: logger.Info("Initialized OKX Spot Adapter")
	return a, nil
}

func (a *SpotAdapter) WithCredentials(apiKey, secretKey string) exchanges.Exchange {
	// OKX requires 3 credentials, this method is not ideal
	// Use NewSpotAdapter with config instead
	// TODO: logger.Warn("WithCredentials not fully supported for OKX, use NewSpotAdapter with config")
	return a
}

func (a *SpotAdapter) Close() error {
	if a.wsPublic.Conn != nil {
		a.wsPublic.Conn.Close()
	}
	if a.wsPrivate.Conn != nil {
		a.wsPrivate.Conn.Close()
	}
	return nil
}

func (a *SpotAdapter) WsAccountConnected(ctx context.Context) error {
	if a.wsPrivate.Conn == nil {
		if err := a.wsPrivate.Connect(); err != nil {
			return err
		}
	}
	return nil
}

func (a *SpotAdapter) WsMarketConnected(ctx context.Context) error {
	if a.wsPublic.Conn == nil {
		if err := a.wsPublic.Connect(); err != nil {
			return err
		}
	}
	return nil
}

func (a *SpotAdapter) WsOrderConnected(ctx context.Context) error {
	if a.wsPrivate.Conn == nil {
		if err := a.wsPrivate.Connect(); err != nil {
			return err
		}
	}
	return nil
}

func (a *SpotAdapter) fetchInstruments(ctx context.Context) error {
	insts, err := a.client.GetInstruments(ctx, "SPOT")
	if err != nil {
		return err
	}

	a.symbolMap = make(map[string]string)
	a.idMap = make(map[string]string)
	a.instruments = make(map[string]okx.Instrument)

	for _, inst := range insts {
		market := exchanges.ParseMarketRef(inst.InstId, exchanges.QuoteCurrency(a.quoteCurrency), exchanges.MarketTypeSpot)
		if inst.State == "live" && market.Base != "" && isSupportedOKXQuote(market.Quote) {
			a.symbolMap[market.Symbol()] = inst.InstId
			if string(market.Quote) == a.quoteCurrency {
				a.symbolMap[market.Base] = inst.InstId
			}
			a.idMap[inst.InstId] = market.Symbol()
			a.instruments[inst.InstId] = inst
		}
	}

	// Populate BaseAdapter symbol details so ListSymbols() works
	details := make(map[string]*exchanges.SymbolDetails)
	for token, instId := range a.symbolMap {
		inst := a.instruments[instId]
		details[token] = &exchanges.SymbolDetails{
			Symbol:            token,
			MinQuantity:       parseString(inst.MinSz),
			PricePrecision:    exchanges.CountDecimalPlaces(inst.TickSz),
			QuantityPrecision: exchanges.CountDecimalPlaces(inst.LotSz),
		}
	}
	a.SetSymbolDetails(details)

	return nil
}

func (a *SpotAdapter) IsConnected(ctx context.Context) (bool, error) {
	return a.IsMarketConnected() && a.IsAccountConnected(), nil
}

// FormatSymbol converts internal symbol (e.g., BTC) to OKX spot InstId (e.g., BTC-USDT)
func (a *SpotAdapter) FormatSymbol(symbol string) string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if strings.Contains(symbol, "-") {
		return symbol
	}
	if instId, ok := a.symbolMap[symbol]; ok {
		return instId
	}
	return FormatSpotSymbolWithQuote(symbol, a.quoteCurrency)
}

// ExtractSymbol converts OKX InstId (e.g., BTC-USDT) to internal symbol (e.g., BTC)
func (a *SpotAdapter) ExtractSymbol(instId string) string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if sym, ok := a.idMap[instId]; ok {
		return sym
	}
	return instId
}

func (a *SpotAdapter) RefreshSymbolDetails(ctx context.Context) error {
	return a.fetchInstruments(ctx)
}

// ================= Account & Trading =================

func (a *SpotAdapter) FetchAccount(ctx context.Context) (*exchanges.Account, error) {
	bals, err := a.client.GetAccountBalance(ctx, nil)
	if err != nil {
		return nil, err
	}

	totalEq := decimal.Zero
	availEq := decimal.Zero
	if len(bals) > 0 {
		totalEq = parseString(bals[0].TotalEq)
		if len(bals[0].Details) > 0 {
			availEq = parseString(bals[0].Details[0].AvailEq)
		}
	}

	// Get open orders
	orders, err := a.client.GetOrders(ctx, nil, nil)
	if err != nil {
		return nil, err
	}
	var orderList []exchanges.Order
	for _, o := range orders {
		orderList = append(orderList, *a.mapOrderRest(&o))
	}

	return &exchanges.Account{
		TotalBalance:     totalEq,
		AvailableBalance: availEq,
		Positions:        nil, // Spot has no positions
		Orders:           orderList,
	}, nil
}

func (a *SpotAdapter) FetchBalance(ctx context.Context) (decimal.Decimal, error) {
	acc, err := a.FetchAccount(ctx)
	if err != nil {
		return decimal.Zero, err
	}
	return acc.TotalBalance, nil
}

func (a *SpotAdapter) FetchSpotBalances(ctx context.Context) ([]exchanges.SpotBalance, error) {
	bals, err := a.client.GetAccountBalance(ctx, nil)
	if err != nil {
		return nil, err
	}

	var spotBalances []exchanges.SpotBalance
	if len(bals) > 0 {
		for _, detail := range bals[0].Details {
			free := parseString(detail.AvailBal)
			locked := parseString(detail.FrozenBal)
			// CashBal is usually the total for spot/cash accounts
			total := parseString(detail.CashBal)
			if total.IsZero() {
				total = free.Add(locked)
			}

			spotBalances = append(spotBalances, exchanges.SpotBalance{
				Asset:  detail.Ccy,
				Free:   free,
				Locked: locked,
				Total:  total,
			})
		}
	}

	return spotBalances, nil
}

func (a *SpotAdapter) TransferAsset(ctx context.Context, params *exchanges.TransferParams) error {
	return exchanges.ErrNotSupported
}

func (a *SpotAdapter) FetchFeeRate(ctx context.Context, symbol string) (*exchanges.FeeRate, error) {
	instId := a.FormatSymbol(symbol)

	res, err := a.client.GetTradeFee(ctx, "SPOT", &instId)
	if err != nil {
		return nil, err
	}
	if len(res) == 0 || len(res[0].FeeGroups) == 0 {
		return nil, fmt.Errorf("no fee data")
	}

	// Default spot fees
	return &exchanges.FeeRate{Maker: decimal.NewFromFloat(0.0008), Taker: decimal.NewFromFloat(0.001)}, nil
}

func (a *SpotAdapter) FetchSymbolDetails(ctx context.Context, symbol string) (*exchanges.SymbolDetails, error) {
	instId := a.FormatSymbol(symbol)

	a.mu.RLock()
	inst, ok := a.instruments[instId]
	a.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("symbol not found: %s", symbol)
	}

	return &exchanges.SymbolDetails{
		Symbol:            symbol,
		PricePrecision:    exchanges.CountDecimalPlaces(inst.TickSz),
		QuantityPrecision: exchanges.CountDecimalPlaces(inst.LotSz),
		MinQuantity:       parseString(inst.MinSz),
		MinNotional:       decimal.Zero, // OKX doesn't provide this directly
	}, nil
}

// ================= WebSocket =================

func (a *SpotAdapter) WatchOrders(ctx context.Context, callback exchanges.OrderUpdateCallback) error {
	return a.watchPrivateOrders(ctx, callback, nil)
}

func (a *SpotAdapter) WatchTicker(ctx context.Context, symbol string, callback exchanges.TickerCallback) error {
	if err := a.WsMarketConnected(ctx); err != nil {
		return err
	}

	instId := a.FormatSymbol(symbol)
	return a.wsPublic.SubscribeTicker(instId, func(t *okx.Ticker) {
		callback(&exchanges.Ticker{
			Symbol:    symbol,
			LastPrice: parseString(t.Last),
			Bid:       parseString(t.BidPx),
			Ask:       parseString(t.AskPx),
			High24h:   parseString(t.High24h),
			Low24h:    parseString(t.Low24h),
			Volume24h: parseString(t.Vol24h),
			Timestamp: parseTime(t.Ts),
		})
	})
}

func (a *SpotAdapter) WatchKlines(ctx context.Context, symbol string, interval exchanges.Interval, callback exchanges.KlineCallback) error {
	return exchanges.ErrNotSupported
}

func (a *SpotAdapter) WatchTrades(ctx context.Context, symbol string, callback exchanges.TradeCallback) error {
	return exchanges.ErrNotSupported
}

// Unsubscribe methods
func (a *SpotAdapter) StopWatchOrders(ctx context.Context) error {
	return a.stopPrivateOrders(ctx, true, false)
}

func (a *SpotAdapter) WatchFills(ctx context.Context, callback exchanges.FillCallback) error {
	return a.watchPrivateOrders(ctx, nil, callback)
}

func (a *SpotAdapter) StopWatchFills(ctx context.Context) error {
	return a.stopPrivateOrders(ctx, false, true)
}

func (a *SpotAdapter) StopWatchTicker(ctx context.Context, symbol string) error {
	instId := a.FormatSymbol(symbol)
	channel := okx.WsSubscribeArgs{
		Channel: "tickers",
		InstId:  instId,
	}
	return a.wsPublic.Unsubscribe(channel)
}

func (a *SpotAdapter) StopWatchKlines(ctx context.Context, symbol string, interval exchanges.Interval) error {
	return exchanges.ErrNotSupported
}

func (a *SpotAdapter) StopWatchTrades(ctx context.Context, symbol string) error {
	return exchanges.ErrNotSupported
}

// WaitOrderBookReady waits for orderbook to be ready
func (a *SpotAdapter) WaitOrderBookReady(ctx context.Context, symbol string) error {
	instId := a.FormatSymbol(symbol)
	return a.BaseAdapter.WaitOrderBookReady(ctx, instId)
}

// GetLocalOrderBook retrieves locally maintained order book
func (a *SpotAdapter) GetLocalOrderBook(symbol string, depth int) *exchanges.OrderBook {
	instId := a.FormatSymbol(symbol)

	ob, ok := a.GetLocalOrderBookImplementation(instId)
	if !ok {
		return nil
	}

	bids, asks := ob.GetDepth(depth)
	return &exchanges.OrderBook{
		Symbol:    symbol,
		Timestamp: ob.Timestamp(),
		Bids:      bids,
		Asks:      asks,
	}
}

// ================= Helper Functions =================

func (a *SpotAdapter) mapOrderRest(o *okx.Order) *exchanges.Order {
	status := exchanges.OrderStatusPending
	switch o.State {
	case okx.OrderStatusLive:
		status = exchanges.OrderStatusNew
	case okx.OrderStatusPartiallyFilled:
		status = exchanges.OrderStatusPartiallyFilled
	case okx.OrderStatusFilled:
		status = exchanges.OrderStatusFilled
	case okx.OrderStatusCanceled, okx.OrderStatusMmpCanceled:
		status = exchanges.OrderStatusCancelled
	}

	side := exchanges.OrderSideBuy
	if o.Side == okx.SideSell {
		side = exchanges.OrderSideSell
	}

	qty := parseString(o.Sz)
	filledQty := parseString(o.AccFillSz)

	return &exchanges.Order{
		OrderID:          o.OrdId,
		ClientOrderID:    o.ClOrdId,
		Symbol:           a.ExtractSymbol(o.InstId),
		Side:             side,
		Type:             mapOKXOrderType(o.OrdType),
		Quantity:         qty,
		Price:            parseString(o.Px),
		OrderPrice:       parseString(o.Px),
		AverageFillPrice: parseString(o.AvgPx),
		LastFillPrice:    parseString(o.FillPx),
		Status:           status,
		FilledQuantity:   filledQty,
		LastFillQuantity: parseString(o.FillSz),
		Timestamp:        parseTime(o.CTime),
		Fee:              parseString(o.Fee),
	}
}

func (a *SpotAdapter) mapOrderStream(o *okx.Order) *exchanges.Order {
	order := a.mapOrderRest(o)
	order.Price = order.OrderPrice
	order.AverageFillPrice = decimal.Zero
	order.LastFillPrice = decimal.Zero
	order.LastFillQuantity = decimal.Zero
	order.Fee = decimal.Zero
	return order
}

func (a *SpotAdapter) mapOrderFill(o *okx.Order) *exchanges.Fill {
	qty := parseString(o.FillSz)
	if qty.IsZero() {
		return nil
	}

	side := exchanges.OrderSideBuy
	if o.Side == okx.SideSell {
		side = exchanges.OrderSideSell
	}

	ts := parseTime(o.FillTime)
	if ts == 0 {
		ts = parseTime(o.UTime)
	}
	if ts == 0 {
		ts = parseTime(o.CTime)
	}

	return &exchanges.Fill{
		TradeID:       o.TradeId,
		OrderID:       o.OrdId,
		ClientOrderID: o.ClOrdId,
		Symbol:        a.ExtractSymbol(o.InstId),
		Side:          side,
		Price:         parseString(o.FillPx),
		Quantity:      qty,
		Fee:           parseString(o.Fee),
		FeeAsset:      o.FeeCcy,
		IsMaker:       strings.EqualFold(o.ExecType, "M"),
		Timestamp:     ts,
	}
}

func mapOKXOrderType(t okx.OrderType) exchanges.OrderType {
	switch t {
	case okx.OrderTypeMarket:
		return exchanges.OrderTypeMarket
	case okx.OrderTypeLimit:
		return exchanges.OrderTypeLimit
	case okx.OrderTypePostOnly:
		return exchanges.OrderTypePostOnly
	case okx.OrderTypeFok:
		return exchanges.OrderTypeLimit
	case okx.OrderTypeIoc:
		return exchanges.OrderTypeLimit
	default:
		return exchanges.OrderTypeUnknown
	}
}

func (a *SpotAdapter) WatchPositions(ctx context.Context, cb exchanges.PositionUpdateCallback) error {
	return exchanges.ErrNotSupported
}

func (a *SpotAdapter) StopWatchPositions(ctx context.Context) error {
	return exchanges.ErrNotSupported
}

func (a *SpotAdapter) StopWatchOrderBook(ctx context.Context, symbol string) error {
	instId := a.FormatSymbol(symbol)
	channel := okx.WsSubscribeArgs{
		Channel: "books",
		InstId:  instId,
	}
	a.RemoveLocalOrderBook(instId)
	return a.wsPublic.Unsubscribe(channel)
}

func (a *SpotAdapter) WatchOrderBook(ctx context.Context, symbol string, depth int, callback exchanges.OrderBookCallback) error {
	if err := a.WsMarketConnected(ctx); err != nil {
		return err
	}
	instId := a.FormatSymbol(symbol)

	// Spot orderbook: sizes are in base currency (no ctVal conversion needed)
	ob := NewOrderBook(symbol, decimal.NewFromInt(1))
	a.SetLocalOrderBook(instId, ob) // Store under instId to match GetLocalOrderBook override

	if err := a.wsPublic.SubscribeOrderBook(instId, func(data *okx.OrderBook, action string) {
		ob.ProcessUpdate(data, action)
		if callback != nil {
			callback(ob.ToAdapterOrderBook(depth))
		}
	}); err != nil {
		return err
	}
	return a.WaitOrderBookReady(ctx, symbol)
}
