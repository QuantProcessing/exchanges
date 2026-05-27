package binance

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/binance/sdk/option"
	"github.com/shopspring/decimal"
)

type OptionAdapter struct {
	*exchanges.BaseAdapter
	client *option.Client
	quote  exchanges.QuoteCurrency

	mu          sync.RWMutex
	contracts   map[string]exchanges.OptionContract
	nativeToKey map[string]string
}

func NewOptionAdapter(ctx context.Context, opts Options) (*OptionAdapter, error) {
	quote, err := opts.quoteCurrency()
	if err != nil {
		return nil, err
	}
	return newOptionAdapterWithClient(ctx, opts, quote, option.NewClient().WithCredentials(opts.APIKey, opts.SecretKey))
}

func newOptionAdapterWithClient(ctx context.Context, opts Options, quote exchanges.QuoteCurrency, client *option.Client) (*OptionAdapter, error) {
	if err := opts.validateCredentials(); err != nil {
		return nil, err
	}

	adp := &OptionAdapter{
		BaseAdapter: exchanges.NewBaseAdapter("BINANCE", exchanges.MarketTypeOption, opts.logger()),
		client:      client,
		quote:       quote,
		contracts:   make(map[string]exchanges.OptionContract),
		nativeToKey: make(map[string]string),
	}
	if err := adp.refreshContracts(ctx); err != nil {
		return nil, fmt.Errorf("binance option init: %w", err)
	}
	return adp, nil
}

func (a *OptionAdapter) Close() error { return nil }

func (a *OptionAdapter) refreshContracts(ctx context.Context) error {
	info, err := a.client.ExchangeInfo(ctx)
	if err != nil {
		return err
	}

	contracts := make(map[string]exchanges.OptionContract)
	nativeToKey := make(map[string]string)
	details := make(map[string]*exchanges.SymbolDetails)

	for _, raw := range info.OptionSymbols {
		contract := binanceOptionContract(raw)
		if contract.Symbol == "" {
			continue
		}
		contracts[contract.Symbol] = contract
		nativeToKey[strings.ToUpper(contract.ExchangeSymbol)] = contract.Symbol
		details[contract.Symbol] = &exchanges.SymbolDetails{
			Symbol:            contract.Symbol,
			PricePrecision:    raw.PriceScale,
			QuantityPrecision: raw.QuantityScale,
			MinQuantity:       contract.MinQuantity,
		}
	}

	a.mu.Lock()
	a.contracts = contracts
	a.nativeToKey = nativeToKey
	a.mu.Unlock()
	a.SetSymbolDetails(details)
	return nil
}

func binanceOptionContract(raw option.OptionSymbol) exchanges.OptionContract {
	base := strings.ToUpper(firstNonEmpty(raw.BaseAsset, strings.Split(raw.Symbol, "-")[0]))
	settle := strings.ToUpper(firstNonEmpty(raw.SettleAsset, raw.QuoteAsset))
	quote := strings.ToUpper(raw.QuoteAsset)
	strike := parseDecimal(raw.StrikePrice)
	typ := exchanges.NormalizeOptionType(raw.Side)
	if typ.Suffix() == "" {
		typ = binanceOptionTypeFromSymbol(raw.Symbol)
	}
	symbol := exchanges.NewOptionSymbol(base, quote, settle, raw.ExpiryDate, strike, typ)
	return exchanges.OptionContract{
		Symbol:         symbol,
		ExchangeSymbol: strings.ToUpper(raw.Symbol),
		Underlying:     strings.TrimSuffix(strings.ToUpper(raw.Underlying), strings.ToUpper(raw.QuoteAsset)),
		BaseAsset:      base,
		QuoteAsset:     quote,
		SettleAsset:    settle,
		Type:           typ,
		StrikePrice:    strike,
		ExpiryTime:     raw.ExpiryDate,
		ContractSize:   parseDecimal(string(raw.Unit)),
		TickSize:       decimal.New(1, -raw.PriceScale),
		LotSize:        decimal.New(1, -raw.QuantityScale),
		MinQuantity:    parseDecimal(raw.MinQty),
		Status:         raw.Status,
	}
}

func (a *OptionAdapter) ListOptionContracts(ctx context.Context, underlying string) ([]exchanges.OptionContract, error) {
	_ = ctx
	filter := strings.ToUpper(strings.TrimSpace(underlying))

	a.mu.RLock()
	defer a.mu.RUnlock()
	out := make([]exchanges.OptionContract, 0, len(a.contracts))
	for _, contract := range a.contracts {
		if filter != "" && contract.BaseAsset != filter && contract.Underlying != filter {
			continue
		}
		out = append(out, contract)
	}
	return out, nil
}

func (a *OptionAdapter) FetchOptionContract(ctx context.Context, contractSymbol string) (*exchanges.OptionContract, error) {
	_ = ctx
	key := a.ExtractSymbol(contractSymbol)
	a.mu.RLock()
	defer a.mu.RUnlock()
	if contract, ok := a.contracts[key]; ok {
		copyContract := contract
		return &copyContract, nil
	}
	return nil, exchanges.ErrSymbolNotFound
}

func (a *OptionAdapter) FormatSymbol(symbol string) string {
	upper := strings.ToUpper(strings.TrimSpace(symbol))
	a.mu.RLock()
	if contract, ok := a.contracts[upper]; ok {
		a.mu.RUnlock()
		return contract.ExchangeSymbol
	}
	if _, ok := a.nativeToKey[upper]; ok {
		a.mu.RUnlock()
		return upper
	}
	a.mu.RUnlock()
	return canonicalToBinanceOptionSymbol(upper)
}

func (a *OptionAdapter) ExtractSymbol(symbol string) string {
	upper := strings.ToUpper(strings.TrimSpace(symbol))
	a.mu.RLock()
	if key, ok := a.nativeToKey[upper]; ok {
		a.mu.RUnlock()
		return key
	}
	if _, ok := a.contracts[upper]; ok {
		a.mu.RUnlock()
		return upper
	}
	a.mu.RUnlock()
	quote := string(a.quote)
	return binanceNativeToCanonicalOptionSymbol(upper, quote, quote)
}

func (a *OptionAdapter) FetchTicker(ctx context.Context, symbol string) (*exchanges.Ticker, error) {
	canonical := a.ExtractSymbol(symbol)
	raw, err := a.client.Ticker(ctx, a.FormatSymbol(symbol))
	if err != nil {
		return nil, err
	}
	bid := parseDecimal(raw.BidPrice)
	ask := parseDecimal(raw.AskPrice)
	ticker := &exchanges.Ticker{
		Symbol:    canonical,
		LastPrice: parseDecimal(raw.LastPrice),
		Bid:       bid,
		Ask:       ask,
		Volume24h: parseDecimal(raw.Volume),
		QuoteVol:  parseDecimal(raw.Amount),
		High24h:   parseDecimal(raw.HighPrice),
		Low24h:    parseDecimal(raw.LowPrice),
		Timestamp: raw.CloseTime,
	}
	if bid.IsPositive() && ask.IsPositive() {
		ticker.MidPrice = bid.Add(ask).Div(decimal.NewFromInt(2))
	}
	return ticker, nil
}

func (a *OptionAdapter) FetchOrderBook(ctx context.Context, symbol string, limit int) (*exchanges.OrderBook, error) {
	canonical := a.ExtractSymbol(symbol)
	raw, err := a.client.Depth(ctx, a.FormatSymbol(symbol), limit)
	if err != nil {
		return nil, err
	}
	book := &exchanges.OrderBook{
		Symbol:    canonical,
		Timestamp: raw.Time,
		Bids:      make([]exchanges.Level, 0, len(raw.Bids)),
		Asks:      make([]exchanges.Level, 0, len(raw.Asks)),
	}
	for _, level := range raw.Bids {
		if len(level) < 2 {
			continue
		}
		book.Bids = append(book.Bids, exchanges.Level{Price: parseDecimal(level[0]), Quantity: parseDecimal(level[1])})
	}
	for _, level := range raw.Asks {
		if len(level) < 2 {
			continue
		}
		book.Asks = append(book.Asks, exchanges.Level{Price: parseDecimal(level[0]), Quantity: parseDecimal(level[1])})
	}
	return book, nil
}

func (a *OptionAdapter) FetchTrades(ctx context.Context, symbol string, limit int) ([]exchanges.Trade, error) {
	canonical := a.ExtractSymbol(symbol)
	raw, err := a.client.Trades(ctx, a.FormatSymbol(symbol), limit)
	if err != nil {
		return nil, err
	}
	out := make([]exchanges.Trade, 0, len(raw))
	for _, trade := range raw {
		side := exchanges.TradeSideBuy
		if strings.EqualFold(trade.Side, "SELL") || strings.EqualFold(trade.Side, "PUT") || trade.Side == "-1" {
			side = exchanges.TradeSideSell
		}
		id := trade.ID.String()
		if id == "" {
			id = trade.TradeID.String()
		}
		out = append(out, exchanges.Trade{
			ID:        id,
			Symbol:    canonical,
			Price:     parseDecimal(trade.Price),
			Quantity:  parseDecimal(trade.Quantity),
			Side:      side,
			Timestamp: trade.Time,
		})
	}
	return out, nil
}

func (a *OptionAdapter) FetchKlines(ctx context.Context, symbol string, interval exchanges.Interval, opts *exchanges.KlineOpts) ([]exchanges.Kline, error) {
	var startTime, endTime int64
	var limit int
	if opts != nil {
		limit = opts.Limit
		if opts.Start != nil {
			startTime = opts.Start.UnixMilli()
		}
		if opts.End != nil {
			endTime = opts.End.UnixMilli()
		}
	}
	canonical := a.ExtractSymbol(symbol)
	raw, err := a.client.Klines(ctx, a.FormatSymbol(symbol), string(interval), limit, startTime, endTime)
	if err != nil {
		return nil, err
	}
	out := make([]exchanges.Kline, 0, len(raw))
	for _, row := range raw {
		if len(row) < 8 {
			continue
		}
		out = append(out, exchanges.Kline{
			Symbol:    canonical,
			Interval:  interval,
			Timestamp: parseInt64(row[0]),
			Open:      parseDecimalInterface(row[1]),
			High:      parseDecimalInterface(row[2]),
			Low:       parseDecimalInterface(row[3]),
			Close:     parseDecimalInterface(row[4]),
			Volume:    parseDecimalInterface(row[5]),
			QuoteVol:  parseDecimalInterface(row[7]),
		})
	}
	return out, nil
}

func (a *OptionAdapter) FetchSymbolDetails(ctx context.Context, symbol string) (*exchanges.SymbolDetails, error) {
	_ = ctx
	return a.GetSymbolDetail(a.ExtractSymbol(symbol))
}

func (a *OptionAdapter) PlaceOrder(ctx context.Context, params *exchanges.OrderParams) (*exchanges.Order, error) {
	if err := requireBinanceOptionPrivate(a.client); err != nil {
		return nil, err
	}
	if err := a.BaseAdapter.ApplySlippage(ctx, params, a.FetchTicker); err != nil {
		return nil, err
	}
	if params.Type == exchanges.OrderTypeMarket {
		return nil, fmt.Errorf("binance option: market orders are not documented for /eapi/v1/order: %w", exchanges.ErrNotSupported)
	}
	if err := a.BaseAdapter.ValidateOrder(params); err != nil {
		return nil, err
	}

	raw, err := a.client.PlaceOrder(ctx, option.PlaceOrderParams{
		Symbol:        a.FormatSymbol(params.Symbol),
		Side:          string(params.Side),
		Type:          binanceOptionOrderType(params.Type),
		Quantity:      params.Quantity.String(),
		Price:         binanceOptionOrderPrice(params),
		TimeInForce:   binanceOptionTimeInForce(params),
		ClientOrderID: params.ClientID,
		ReduceOnly:    params.ReduceOnly,
	})
	if err != nil {
		return nil, err
	}
	return a.normalizeOptionOrder(raw)
}

func (a *OptionAdapter) PlaceOrderWS(context.Context, *exchanges.OrderParams) error {
	return exchanges.ErrNotSupported
}

func (a *OptionAdapter) CancelOrder(ctx context.Context, orderID, symbol string) error {
	if err := requireBinanceOptionPrivate(a.client); err != nil {
		return err
	}
	_, err := a.client.CancelOrder(ctx, option.CancelOrderParams{
		Symbol:  a.FormatSymbol(symbol),
		OrderID: orderID,
	})
	return err
}
func (a *OptionAdapter) CancelOrderWS(context.Context, string, string) error {
	return exchanges.ErrNotSupported
}
func (a *OptionAdapter) CancelAllOrders(ctx context.Context, symbol string) error {
	if err := requireBinanceOptionPrivate(a.client); err != nil {
		return err
	}
	formatted := ""
	if strings.TrimSpace(symbol) != "" {
		formatted = a.FormatSymbol(symbol)
	}
	return a.client.CancelAllOpenOrders(ctx, option.CancelAllOrdersParams{Symbol: formatted})
}
func (a *OptionAdapter) FetchOrderByID(ctx context.Context, orderID, symbol string) (*exchanges.Order, error) {
	if err := requireBinanceOptionPrivate(a.client); err != nil {
		return nil, err
	}
	raw, err := a.client.GetOrder(ctx, a.FormatSymbol(symbol), orderID, "")
	if err != nil {
		if isBinanceOptionOrderLookupMiss(err) {
			return nil, exchanges.ErrOrderNotFound
		}
		return nil, err
	}
	if raw == nil || raw.IDString() == "" {
		return nil, exchanges.ErrOrderNotFound
	}
	return a.normalizeOptionOrder(raw)
}
func (a *OptionAdapter) FetchOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	if err := requireBinanceOptionPrivate(a.client); err != nil {
		return nil, err
	}
	raw, err := a.client.GetOrderHistory(ctx, a.FormatSymbol(symbol))
	if err != nil {
		return nil, err
	}
	orders := make([]exchanges.Order, 0, len(raw))
	for _, record := range raw {
		order, err := a.normalizeOptionOrder(&record)
		if err != nil {
			continue
		}
		orders = append(orders, *order)
	}
	return orders, nil
}
func (a *OptionAdapter) FetchOpenOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	if err := requireBinanceOptionPrivate(a.client); err != nil {
		return nil, err
	}
	formatted := ""
	if strings.TrimSpace(symbol) != "" {
		formatted = a.FormatSymbol(symbol)
	}
	raw, err := a.client.GetOpenOrders(ctx, formatted)
	if err != nil {
		return nil, err
	}
	orders := make([]exchanges.Order, 0, len(raw))
	for _, record := range raw {
		order, err := a.normalizeOptionOrder(&record)
		if err != nil {
			continue
		}
		orders = append(orders, *order)
	}
	return orders, nil
}
func (a *OptionAdapter) FetchAccount(context.Context) (*exchanges.Account, error) {
	return nil, exchanges.ErrNotSupported
}
func (a *OptionAdapter) FetchBalance(context.Context) (decimal.Decimal, error) {
	return decimal.Zero, exchanges.ErrNotSupported
}
func (a *OptionAdapter) FetchFeeRate(context.Context, string) (*exchanges.FeeRate, error) {
	return nil, exchanges.ErrNotSupported
}
func (a *OptionAdapter) WatchOrderBook(context.Context, string, int, exchanges.OrderBookCallback) error {
	return exchanges.ErrNotSupported
}
func (a *OptionAdapter) StopWatchOrderBook(context.Context, string) error {
	return exchanges.ErrNotSupported
}
func (a *OptionAdapter) WatchOrders(context.Context, exchanges.OrderUpdateCallback) error {
	return exchanges.ErrNotSupported
}
func (a *OptionAdapter) WatchFills(context.Context, exchanges.FillCallback) error {
	return exchanges.ErrNotSupported
}
func (a *OptionAdapter) WatchPositions(context.Context, exchanges.PositionUpdateCallback) error {
	return exchanges.ErrNotSupported
}
func (a *OptionAdapter) WatchTicker(context.Context, string, exchanges.TickerCallback) error {
	return exchanges.ErrNotSupported
}
func (a *OptionAdapter) WatchTrades(context.Context, string, exchanges.TradeCallback) error {
	return exchanges.ErrNotSupported
}
func (a *OptionAdapter) WatchKlines(context.Context, string, exchanges.Interval, exchanges.KlineCallback) error {
	return exchanges.ErrNotSupported
}
func (a *OptionAdapter) StopWatchOrders(context.Context) error { return exchanges.ErrNotSupported }
func (a *OptionAdapter) StopWatchFills(context.Context) error  { return exchanges.ErrNotSupported }
func (a *OptionAdapter) StopWatchPositions(context.Context) error {
	return exchanges.ErrNotSupported
}
func (a *OptionAdapter) StopWatchTicker(context.Context, string) error {
	return exchanges.ErrNotSupported
}
func (a *OptionAdapter) StopWatchTrades(context.Context, string) error {
	return exchanges.ErrNotSupported
}
func (a *OptionAdapter) StopWatchKlines(context.Context, string, exchanges.Interval) error {
	return exchanges.ErrNotSupported
}

func canonicalToBinanceOptionSymbol(symbol string) string {
	parts, ok := exchanges.ParseOptionSymbol(symbol)
	if !ok {
		return symbol
	}
	expiry := time.UnixMilli(parts.ExpiryTime).UTC().Format("060102")
	return strings.Join([]string{parts.BaseAsset, expiry, parts.StrikePrice.String(), parts.Type.Suffix()}, "-")
}

func binanceNativeToCanonicalOptionSymbol(symbol, quote, settle string) string {
	parts := strings.Split(symbol, "-")
	if len(parts) != 4 || len(parts[1]) != 6 {
		return symbol
	}
	expiry, err := time.Parse("20060102", "20"+parts[1])
	if err != nil {
		return symbol
	}
	strike, err := decimal.NewFromString(parts[2])
	if err != nil {
		return symbol
	}
	return exchanges.NewOptionSymbol(parts[0], quote, settle, expiry.UTC().UnixMilli(), strike, exchanges.NormalizeOptionType(parts[3]))
}

func binanceOptionTypeFromSymbol(symbol string) exchanges.OptionType {
	parts := strings.Split(strings.ToUpper(strings.TrimSpace(symbol)), "-")
	if len(parts) == 0 {
		return ""
	}
	return exchanges.NormalizeOptionType(parts[len(parts)-1])
}

func requireBinanceOptionPrivate(client *option.Client) error {
	if client == nil || !client.HasCredentials() {
		return exchanges.NewExchangeError("BINANCE", "", "binance option: private access requires api_key and secret_key", exchanges.ErrAuthFailed)
	}
	return nil
}

func binanceOptionOrderType(orderType exchanges.OrderType) string {
	switch orderType {
	case exchanges.OrderTypeMarket:
		return "MARKET"
	case exchanges.OrderTypePostOnly:
		return "LIMIT"
	case exchanges.OrderTypeLimit:
		return "LIMIT"
	default:
		return string(orderType)
	}
}

func binanceOptionTimeInForce(params *exchanges.OrderParams) string {
	if params.Type == exchanges.OrderTypePostOnly {
		return "GTC"
	}
	if params.Type != exchanges.OrderTypeLimit {
		return ""
	}
	switch params.TimeInForce {
	case exchanges.TimeInForceIOC:
		return "IOC"
	case exchanges.TimeInForceFOK:
		return "FOK"
	default:
		return "GTC"
	}
}

func binanceOptionOrderPrice(params *exchanges.OrderParams) string {
	if params.Price.IsPositive() && (params.Type == exchanges.OrderTypeLimit || params.Type == exchanges.OrderTypePostOnly) {
		return params.Price.String()
	}
	return ""
}

func (a *OptionAdapter) normalizeOptionOrder(raw *option.OrderResponse) (*exchanges.Order, error) {
	if raw == nil {
		return nil, exchanges.ErrOrderNotFound
	}
	status := mapBinanceOptionOrderStatus(raw.Status)
	orderType := mapBinanceOptionOrderType(raw.Type, raw.TimeInForce)
	side := exchanges.OrderSideBuy
	if strings.EqualFold(raw.Side, "SELL") {
		side = exchanges.OrderSideSell
	}
	ts := raw.UpdateTime
	if ts == 0 {
		ts = raw.CreateDate
	}
	if ts == 0 {
		ts = time.Now().UnixMilli()
	}
	return &exchanges.Order{
		OrderID:          raw.IDString(),
		ClientOrderID:    raw.ClientOrderID,
		Symbol:           a.ExtractSymbol(raw.Symbol),
		Side:             side,
		Type:             orderType,
		Quantity:         parseDecimal(raw.Quantity),
		FilledQuantity:   parseDecimal(raw.ExecutedQty),
		Price:            parseDecimal(firstNonEmpty(raw.Price, raw.AvgPrice)),
		OrderPrice:       parseDecimal(raw.Price),
		AverageFillPrice: parseDecimal(raw.AvgPrice),
		Status:           status,
		Timestamp:        ts,
		Fee:              parseDecimal(raw.Fee),
		ReduceOnly:       raw.ReduceOnly,
		TimeInForce:      mapBinanceOptionTimeInForce(raw.TimeInForce),
	}, nil
}

func mapBinanceOptionOrderStatus(status string) exchanges.OrderStatus {
	switch strings.ToUpper(status) {
	case "ACCEPTED", "NEW":
		return exchanges.OrderStatusNew
	case "PARTIALLY_FILLED":
		return exchanges.OrderStatusPartiallyFilled
	case "FILLED":
		return exchanges.OrderStatusFilled
	case "CANCELED", "CANCELLED", "EXPIRED":
		return exchanges.OrderStatusCancelled
	case "REJECTED":
		return exchanges.OrderStatusRejected
	default:
		return exchanges.OrderStatusUnknown
	}
}

func mapBinanceOptionOrderType(orderType, tif string) exchanges.OrderType {
	switch strings.ToUpper(orderType) {
	case "MARKET":
		return exchanges.OrderTypeMarket
	case "LIMIT":
		if strings.EqualFold(tif, "GTX") {
			return exchanges.OrderTypePostOnly
		}
		return exchanges.OrderTypeLimit
	default:
		return exchanges.OrderTypeUnknown
	}
}

func mapBinanceOptionTimeInForce(tif string) exchanges.TimeInForce {
	switch strings.ToUpper(tif) {
	case "IOC":
		return exchanges.TimeInForceIOC
	case "FOK":
		return exchanges.TimeInForceFOK
	case "GTC":
		return exchanges.TimeInForceGTC
	default:
		return ""
	}
}

func isBinanceOptionOrderLookupMiss(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "order") &&
		(strings.Contains(msg, "does not exist") ||
			strings.Contains(msg, "unknown order") ||
			strings.Contains(msg, "not found"))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
