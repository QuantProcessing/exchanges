package deribit

import (
	"context"
	"strings"
	"sync"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/deribit/sdk"
	"github.com/shopspring/decimal"
)

type OptionAdapter struct {
	*exchanges.BaseAdapter
	client marketClient

	mu          sync.RWMutex
	contracts   map[string]exchanges.OptionContract
	nativeToKey map[string]string
}

func NewOptionAdapter(ctx context.Context, opts Options) (*OptionAdapter, error) {
	return newOptionAdapterWithClient(ctx, opts, sdk.NewClient().WithCredentials(opts.APIKey, opts.SecretKey))
}

// newOptionAdapterWithClient eagerly loads public option metadata so
// ListSymbols, contract lookup, and FetchSymbolDetails are ready after construction.
func newOptionAdapterWithClient(ctx context.Context, opts Options, client marketClient) (*OptionAdapter, error) {
	if hasAnyCredentials(opts) && !hasFullCredentials(opts) {
		return nil, deribitAuthError("deribit: api_key and secret_key must both be set together")
	}
	adp := &OptionAdapter{
		BaseAdapter: exchanges.NewBaseAdapter(exchangeName, exchanges.MarketTypeOption, opts.logger()),
		client:      client,
		contracts:   make(map[string]exchanges.OptionContract),
		nativeToKey: make(map[string]string),
	}
	if err := adp.refreshCurrencies(ctx, opts.optionCurrencies()); err != nil {
		return nil, err
	}
	return adp, nil
}

func (a *OptionAdapter) Close() error { return nil }

func (a *OptionAdapter) refreshCurrencies(ctx context.Context, currencies []string) error {
	contracts := make(map[string]exchanges.OptionContract)
	nativeToKey := make(map[string]string)
	details := make(map[string]*exchanges.SymbolDetails)

	a.mu.RLock()
	for key, contract := range a.contracts {
		contracts[key] = contract
	}
	for native, key := range a.nativeToKey {
		nativeToKey[native] = key
	}
	a.mu.RUnlock()

	for _, currency := range currencies {
		instruments, err := a.client.GetInstruments(ctx, deribitCurrencyParam(currency), kindOption, false)
		if err != nil {
			return err
		}
		for _, inst := range instruments {
			if !strings.EqualFold(inst.Kind, kindOption) || (!inst.IsActive && !strings.EqualFold(inst.State, "open")) {
				continue
			}
			contract := deribitOptionContract(inst)
			if contract.Symbol == "" {
				continue
			}
			contracts[contract.Symbol] = contract
			nativeToKey[strings.ToUpper(contract.ExchangeSymbol)] = contract.Symbol
		}
	}

	for _, contract := range contracts {
		details[contract.Symbol] = &exchanges.SymbolDetails{
			Symbol:            contract.Symbol,
			PricePrecision:    exchanges.CountDecimalPlaces(contract.TickSize.String()),
			QuantityPrecision: exchanges.CountDecimalPlaces(contract.LotSize.String()),
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

func deribitOptionContract(inst sdk.Instrument) exchanges.OptionContract {
	base := strings.ToUpper(firstNonEmpty(inst.BaseCurrency, strings.Split(inst.InstrumentName, "-")[0]))
	expiry := inst.ExpirationTimestamp
	if expiry == 0 {
		parts := strings.Split(strings.ToUpper(inst.InstrumentName), "-")
		if len(parts) >= 2 {
			if parsed, ok := parseNativeDate(parts[1]); ok {
				expiry = parsed
			}
		}
	}
	strike := decimalFromFloat(inst.Strike)
	if strike.IsZero() {
		parts := strings.Split(strings.ToUpper(inst.InstrumentName), "-")
		if len(parts) >= 3 {
			strike, _ = decimal.NewFromString(parts[2])
		}
	}
	typ := exchanges.NormalizeOptionType(inst.OptionType)
	if strings.HasSuffix(strings.ToUpper(inst.InstrumentName), "-P") {
		typ = exchanges.OptionTypePut
	}
	if strings.HasSuffix(strings.ToUpper(inst.InstrumentName), "-C") {
		typ = exchanges.OptionTypeCall
	}
	quote := strings.ToUpper(firstNonEmpty(inst.QuoteCurrency, inst.CounterCurrency))
	settle := strings.ToUpper(firstNonEmpty(inst.SettlementCurrency, quote))
	return exchanges.OptionContract{
		Symbol:         exchanges.NewOptionSymbol(base, quote, settle, expiry, strike, typ),
		ExchangeSymbol: strings.ToUpper(inst.InstrumentName),
		Underlying:     base,
		BaseAsset:      base,
		QuoteAsset:     quote,
		SettleAsset:    settle,
		Type:           typ,
		StrikePrice:    strike,
		ExpiryTime:     expiry,
		ContractSize:   decimalFromFloat(inst.ContractSize),
		TickSize:       decimalFromFloat(inst.TickSize),
		LotSize:        decimalFromFloat(inst.MinTradeAmount),
		MinQuantity:    decimalFromFloat(inst.MinTradeAmount),
		Status:         inst.State,
	}
}

func (a *OptionAdapter) ListOptionContracts(ctx context.Context, underlying string) ([]exchanges.OptionContract, error) {
	filter := strings.ToUpper(strings.TrimSpace(underlying))
	if filter != "" && !a.hasContractsForUnderlying(filter) {
		if err := a.refreshCurrencies(ctx, []string{filter}); err != nil {
			return nil, err
		}
	}

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

func (a *OptionAdapter) hasContractsForUnderlying(underlying string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	for _, contract := range a.contracts {
		if contract.BaseAsset == underlying || contract.Underlying == underlying {
			return true
		}
	}
	return false
}

func (a *OptionAdapter) FetchOptionContract(ctx context.Context, contractSymbol string) (*exchanges.OptionContract, error) {
	key := a.ExtractSymbol(contractSymbol)
	a.mu.RLock()
	if contract, ok := a.contracts[key]; ok {
		a.mu.RUnlock()
		copyContract := contract
		return &copyContract, nil
	}
	a.mu.RUnlock()

	if underlying := optionUnderlying(contractSymbol); underlying != "" {
		if err := a.refreshCurrencies(ctx, []string{underlying}); err != nil {
			return nil, err
		}
	}

	key = a.ExtractSymbol(contractSymbol)
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
	return canonicalToDeribitOptionSymbol(upper)
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
	return deribitNativeToCanonicalOptionSymbol(upper)
}

func (a *OptionAdapter) FetchTicker(ctx context.Context, symbol string) (*exchanges.Ticker, error) {
	raw, err := a.client.GetTicker(ctx, a.FormatSymbol(symbol))
	if err != nil {
		return nil, err
	}
	return toTicker(a.ExtractSymbol(firstNonEmpty(raw.InstrumentName, symbol)), raw), nil
}

func (a *OptionAdapter) FetchOrderBook(ctx context.Context, symbol string, limit int) (*exchanges.OrderBook, error) {
	raw, err := a.client.GetOrderBook(ctx, a.FormatSymbol(symbol), deribitDepth(limit))
	if err != nil {
		return nil, err
	}
	return toOrderBook(a.ExtractSymbol(firstNonEmpty(raw.InstrumentName, symbol)), raw), nil
}

func (a *OptionAdapter) FetchTrades(ctx context.Context, symbol string, limit int) ([]exchanges.Trade, error) {
	raw, err := a.client.GetLastTradesByInstrument(ctx, a.FormatSymbol(symbol), deribitCount(limit))
	if err != nil {
		return nil, err
	}
	return mapTrades(a.ExtractSymbol(symbol), raw.Trades), nil
}

func (a *OptionAdapter) FetchKlines(ctx context.Context, symbol string, interval exchanges.Interval, opts *exchanges.KlineOpts) ([]exchanges.Kline, error) {
	resolution, err := deribitResolution(interval)
	if err != nil {
		return nil, err
	}
	start, end, err := klineTimeRange(interval, opts)
	if err != nil {
		return nil, err
	}
	raw, err := a.client.GetTradingViewChartData(ctx, a.FormatSymbol(symbol), start, end, resolution)
	if err != nil {
		return nil, err
	}
	return mapKlines(a.ExtractSymbol(symbol), interval, raw), nil
}

func (a *OptionAdapter) FetchSymbolDetails(ctx context.Context, symbol string) (*exchanges.SymbolDetails, error) {
	contract, err := a.FetchOptionContract(ctx, symbol)
	if err != nil {
		return nil, err
	}
	return a.GetSymbolDetail(contract.Symbol)
}

func canonicalToDeribitOptionSymbol(symbol string) string {
	parts, ok := exchanges.ParseOptionSymbol(symbol)
	if !ok {
		return symbol
	}
	expiry := time.UnixMilli(parts.ExpiryTime).UTC()
	return strings.Join([]string{parts.BaseAsset, strings.ToUpper(expiry.Format("02Jan06")), parts.StrikePrice.String(), parts.Type.Suffix()}, "-")
}

func deribitNativeToCanonicalOptionSymbol(symbol string) string {
	parts := strings.Split(strings.ToUpper(strings.TrimSpace(symbol)), "-")
	if len(parts) != 4 {
		return symbol
	}
	expiry, ok := parseNativeDate(parts[1])
	if !ok {
		return symbol
	}
	strike, _ := decimal.NewFromString(parts[2])
	return exchanges.NewOptionSymbol(parts[0], parts[0], parts[0], expiry, strike, exchanges.NormalizeOptionType(parts[3]))
}

func optionUnderlying(symbol string) string {
	parts := strings.Split(strings.ToUpper(strings.TrimSpace(symbol)), "-")
	if len(parts) >= 4 {
		return parts[0]
	}
	return ""
}

func deribitOrderType(orderType exchanges.OrderType) string {
	if orderType == exchanges.OrderTypeMarket {
		return "market"
	}
	return "limit"
}

func deribitOrderPrice(params *exchanges.OrderParams) string {
	if params.Price.IsPositive() && params.Type != exchanges.OrderTypeMarket {
		return params.Price.String()
	}
	return ""
}

func deribitTimeInForce(params *exchanges.OrderParams) string {
	switch params.TimeInForce {
	case exchanges.TimeInForceIOC:
		return "immediate_or_cancel"
	case exchanges.TimeInForceFOK:
		return "fill_or_kill"
	default:
		return "good_til_cancelled"
	}
}

func (a *OptionAdapter) mapOrder(raw sdk.OrderRecord) *exchanges.Order {
	side := exchanges.OrderSideBuy
	if strings.EqualFold(raw.Direction, "sell") {
		side = exchanges.OrderSideSell
	}
	ts := raw.UpdateTime
	if ts == 0 {
		ts = raw.LastUpdateTime
	}
	if ts == 0 {
		ts = raw.CreationTime
	}
	if ts == 0 {
		ts = time.Now().UnixMilli()
	}
	order := &exchanges.Order{
		OrderID:          raw.OrderID,
		ClientOrderID:    raw.Label,
		Symbol:           a.ExtractSymbol(raw.InstrumentName),
		Side:             side,
		Type:             deribitMapOrderType(raw.OrderType, raw.PostOnly),
		Quantity:         decimalFromFloat(raw.Amount),
		FilledQuantity:   decimalFromFloat(raw.FilledAmount),
		Price:            decimalFromFloat(firstPositiveFloat(raw.Price, raw.AveragePrice)),
		OrderPrice:       decimalFromFloat(raw.Price),
		AverageFillPrice: decimalFromFloat(raw.AveragePrice),
		Status:           deribitMapOrderStatus(raw.OrderState),
		Timestamp:        ts,
		Fee:              decimalFromFloat(raw.Commission),
		ReduceOnly:       raw.ReduceOnly,
		TimeInForce:      deribitMapTimeInForce(raw.TimeInForce),
	}
	exchanges.DerivePartialFillStatus(order)
	return order
}

func deribitMapOrderType(orderType string, postOnly bool) exchanges.OrderType {
	if postOnly {
		return exchanges.OrderTypePostOnly
	}
	switch strings.ToLower(orderType) {
	case "market":
		return exchanges.OrderTypeMarket
	case "limit":
		return exchanges.OrderTypeLimit
	default:
		return exchanges.OrderTypeUnknown
	}
}

func deribitMapOrderStatus(status string) exchanges.OrderStatus {
	switch strings.ToLower(status) {
	case "open", "untriggered":
		return exchanges.OrderStatusNew
	case "filled":
		return exchanges.OrderStatusFilled
	case "cancelled", "canceled":
		return exchanges.OrderStatusCancelled
	case "rejected":
		return exchanges.OrderStatusRejected
	default:
		return exchanges.OrderStatusUnknown
	}
}

func deribitMapTimeInForce(tif string) exchanges.TimeInForce {
	switch strings.ToLower(tif) {
	case "immediate_or_cancel":
		return exchanges.TimeInForceIOC
	case "fill_or_kill":
		return exchanges.TimeInForceFOK
	case "good_til_cancelled":
		return exchanges.TimeInForceGTC
	default:
		return ""
	}
}

func isDeribitOrderLookupMiss(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "order") &&
		(strings.Contains(msg, "not found") ||
			strings.Contains(msg, "not_found") ||
			strings.Contains(msg, "not open"))
}

func (a *OptionAdapter) PlaceOrder(ctx context.Context, params *exchanges.OrderParams) (*exchanges.Order, error) {
	if err := requirePrivateClientAccess(a.client); err != nil {
		return nil, err
	}
	if err := a.BaseAdapter.ValidateOrder(params); err != nil {
		return nil, err
	}
	req := sdk.OrderRequest{
		InstrumentName: a.FormatSymbol(params.Symbol),
		Amount:         params.Quantity.String(),
		Type:           deribitOrderType(params.Type),
		Price:          deribitOrderPrice(params),
		TimeInForce:    deribitTimeInForce(params),
		Label:          params.ClientID,
		ReduceOnly:     params.ReduceOnly,
		PostOnly:       params.Type == exchanges.OrderTypePostOnly || params.TimeInForce == exchanges.TimeInForcePO,
	}
	var raw *sdk.OrderResult
	var err error
	if params.Side == exchanges.OrderSideSell {
		raw, err = a.client.Sell(ctx, req)
	} else {
		raw, err = a.client.Buy(ctx, req)
	}
	if err != nil {
		return nil, err
	}
	if raw == nil || raw.Order.OrderID == "" {
		return nil, exchanges.ErrOrderNotFound
	}
	return a.mapOrder(raw.Order), nil
}
func (a *OptionAdapter) PlaceOrderWS(context.Context, *exchanges.OrderParams) error {
	return exchanges.ErrNotSupported
}
func (a *OptionAdapter) CancelOrder(ctx context.Context, orderID, symbol string) error {
	_ = symbol
	if err := requirePrivateClientAccess(a.client); err != nil {
		return err
	}
	_, err := a.client.CancelOrder(ctx, orderID)
	return err
}
func (a *OptionAdapter) CancelOrderWS(context.Context, string, string) error {
	return exchanges.ErrNotSupported
}
func (a *OptionAdapter) CancelAllOrders(ctx context.Context, symbol string) error {
	if err := requirePrivateClientAccess(a.client); err != nil {
		return err
	}
	if strings.TrimSpace(symbol) == "" {
		_, err := a.client.CancelAll(ctx)
		return err
	}
	_, err := a.client.CancelAllByInstrument(ctx, a.FormatSymbol(symbol))
	return err
}
func (a *OptionAdapter) FetchOrderByID(ctx context.Context, orderID, symbol string) (*exchanges.Order, error) {
	_ = symbol
	if err := requirePrivateClientAccess(a.client); err != nil {
		return nil, err
	}
	raw, err := a.client.GetOrderState(ctx, orderID)
	if err != nil {
		if isDeribitOrderLookupMiss(err) {
			return nil, exchanges.ErrOrderNotFound
		}
		return nil, err
	}
	if raw == nil || raw.OrderID == "" {
		return nil, exchanges.ErrOrderNotFound
	}
	return a.mapOrder(*raw), nil
}
func (a *OptionAdapter) FetchOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	if err := requirePrivateClientAccess(a.client); err != nil {
		return nil, err
	}
	if strings.TrimSpace(symbol) == "" {
		return nil, exchanges.ErrNotSupported
	}
	raw, err := a.client.GetOrderHistoryByInstrument(ctx, a.FormatSymbol(symbol), 100)
	if err != nil {
		return nil, err
	}
	out := make([]exchanges.Order, 0, len(raw))
	for _, record := range raw {
		out = append(out, *a.mapOrder(record))
	}
	return out, nil
}
func (a *OptionAdapter) FetchOpenOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	if err := requirePrivateClientAccess(a.client); err != nil {
		return nil, err
	}
	if strings.TrimSpace(symbol) == "" {
		return nil, exchanges.ErrNotSupported
	}
	raw, err := a.client.GetOpenOrdersByInstrument(ctx, a.FormatSymbol(symbol))
	if err != nil {
		return nil, err
	}
	out := make([]exchanges.Order, 0, len(raw))
	for _, record := range raw {
		out = append(out, *a.mapOrder(record))
	}
	return out, nil
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
