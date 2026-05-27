package bybit

import (
	"context"
	"strings"
	"sync"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/bybit/sdk"
	"github.com/shopspring/decimal"
)

type OptionAdapter struct {
	*exchanges.BaseAdapter
	client      marketClient
	cancel      context.CancelFunc
	mu          sync.RWMutex
	contracts   map[string]exchanges.OptionContract
	nativeToKey map[string]string
}

func NewOptionAdapter(ctx context.Context, opts Options) (*OptionAdapter, error) {
	lifecycleCtx, cancel := context.WithCancel(ctx)
	adp, err := newOptionAdapterWithClient(lifecycleCtx, cancel, opts, exchanges.QuoteCurrencyUSDT, sdk.NewClient().WithCredentials(opts.APIKey, opts.SecretKey))
	if err != nil {
		cancel()
		return nil, err
	}
	return adp, nil
}

func newOptionAdapterWithClient(ctx context.Context, cancel context.CancelFunc, opts Options, _ exchanges.QuoteCurrency, client marketClient) (*OptionAdapter, error) {
	if hasAnyCredentials(opts) && !hasFullCredentials(opts) {
		return nil, authError("bybit: api_key and secret_key must both be set together")
	}

	instruments, err := fetchBybitOptionInstrumentsForUnderlyings(ctx, client, opts.optionUnderlyings())
	if err != nil {
		return nil, err
	}
	adp := &OptionAdapter{
		BaseAdapter: exchanges.NewBaseAdapter(exchangeName, exchanges.MarketTypeOption, opts.logger()),
		client:      client,
		cancel:      cancel,
		contracts:   make(map[string]exchanges.OptionContract),
		nativeToKey: make(map[string]string),
	}
	adp.replaceContracts(instruments)
	return adp, nil
}

func (a *OptionAdapter) Close() error {
	if a.cancel != nil {
		a.cancel()
	}
	return nil
}

func (a *OptionAdapter) replaceContracts(instruments []sdk.Instrument) {
	a.setContracts(instruments, true)
}

func (a *OptionAdapter) mergeContracts(instruments []sdk.Instrument) {
	a.setContracts(instruments, false)
}

func (a *OptionAdapter) setContracts(instruments []sdk.Instrument, replace bool) {
	contracts := make(map[string]exchanges.OptionContract)
	nativeToKey := make(map[string]string)
	details := make(map[string]*exchanges.SymbolDetails)

	if !replace {
		a.mu.RLock()
		for key, contract := range a.contracts {
			contracts[key] = contract
		}
		for native, key := range a.nativeToKey {
			nativeToKey[native] = key
		}
		a.mu.RUnlock()
		for _, symbol := range a.ListSymbols() {
			if detail, err := a.GetSymbolDetail(symbol); err == nil {
				details[symbol] = detail
			}
		}
	}

	for _, inst := range instruments {
		if !strings.EqualFold(inst.Status, instrumentStatusTrading) {
			continue
		}
		contract := bybitOptionContract(inst)
		if contract.Symbol == "" {
			continue
		}
		contracts[contract.Symbol] = contract
		nativeToKey[strings.ToUpper(contract.ExchangeSymbol)] = contract.Symbol
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
}

func bybitOptionContract(inst sdk.Instrument) exchanges.OptionContract {
	base := strings.ToUpper(inst.BaseCoin)
	if base == "" {
		base = strings.Split(strings.ToUpper(inst.Symbol), "-")[0]
	}
	expiry := parseMillis(inst.DeliveryTime)
	strike := bybitStrikeFromSymbol(inst.Symbol)
	typ := exchanges.NormalizeOptionType(inst.OptionsType)
	if symbolType := bybitOptionTypeFromSymbol(inst.Symbol); symbolType.Suffix() != "" {
		typ = symbolType
	}
	if expiry == 0 {
		expiry = bybitExpiryFromSymbol(inst.Symbol)
	}
	quote := strings.ToUpper(inst.QuoteCoin)
	settle := strings.ToUpper(firstNonEmpty(inst.SettleCoin, inst.QuoteCoin))
	return exchanges.OptionContract{
		Symbol:         exchanges.NewOptionSymbol(base, quote, settle, expiry, strike, typ),
		ExchangeSymbol: strings.ToUpper(inst.Symbol),
		Underlying:     base,
		BaseAsset:      base,
		QuoteAsset:     quote,
		SettleAsset:    settle,
		Type:           typ,
		StrikePrice:    strike,
		ExpiryTime:     expiry,
		ContractSize:   decimal.NewFromInt(1),
		TickSize:       parseDecimal(inst.PriceFilter.TickSize),
		LotSize:        parseDecimal(firstNonEmpty(inst.LotSizeFilter.QtyStep, inst.LotSizeFilter.BasePrecision)),
		MinQuantity:    parseDecimal(inst.LotSizeFilter.MinOrderQty),
		Status:         inst.Status,
	}
}

func (a *OptionAdapter) ListOptionContracts(ctx context.Context, underlying string) ([]exchanges.OptionContract, error) {
	filter := strings.ToUpper(strings.TrimSpace(underlying))
	if filter != "" && !a.hasContractsForUnderlying(filter) {
		instruments, err := fetchBybitOptionInstruments(ctx, a.client, filter)
		if err != nil {
			return nil, err
		}
		a.mergeContracts(instruments)
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

	if underlying := bybitUnderlyingFromOptionSymbol(contractSymbol); underlying != "" {
		instruments, err := fetchBybitOptionInstruments(ctx, a.client, underlying)
		if err != nil {
			return nil, err
		}
		a.mergeContracts(instruments)
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
	return canonicalToBybitOptionSymbol(upper)
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
	return bybitNativeToCanonicalOptionSymbol(upper, "", "")
}

func (a *OptionAdapter) FetchTicker(ctx context.Context, symbol string) (*exchanges.Ticker, error) {
	raw, err := a.client.GetTicker(ctx, categoryOption, a.FormatSymbol(symbol))
	if err != nil {
		return nil, err
	}
	return toTicker(a.ExtractSymbol(symbol), raw), nil
}

func (a *OptionAdapter) FetchOrderBook(ctx context.Context, symbol string, limit int) (*exchanges.OrderBook, error) {
	raw, err := a.client.GetOrderBook(ctx, categoryOption, a.FormatSymbol(symbol), limit)
	if err != nil {
		return nil, err
	}
	return toOrderBook(a.ExtractSymbol(symbol), raw), nil
}

func (a *OptionAdapter) FetchTrades(ctx context.Context, symbol string, limit int) ([]exchanges.Trade, error) {
	raw, err := a.client.GetRecentTrades(ctx, categoryOption, a.FormatSymbol(symbol), limit)
	if err != nil {
		return nil, err
	}
	return mapTrades(a.ExtractSymbol(symbol), raw), nil
}

func (a *OptionAdapter) FetchKlines(ctx context.Context, symbol string, interval exchanges.Interval, opts *exchanges.KlineOpts) ([]exchanges.Kline, error) {
	rawInterval, err := klineIntervalString(interval)
	if err != nil {
		return nil, err
	}
	start, end, limit, err := klineTimeRange(interval, opts)
	if err != nil {
		return nil, err
	}
	raw, err := a.client.GetKlines(ctx, categoryOption, a.FormatSymbol(symbol), rawInterval, start, end, limit)
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

func (a *OptionAdapter) PlaceOrder(ctx context.Context, params *exchanges.OrderParams) (*exchanges.Order, error) {
	if err := requirePrivateClientAccess(a.client); err != nil {
		return nil, err
	}
	if err := a.BaseAdapter.ValidateOrder(params); err != nil {
		return nil, err
	}
	req, err := toPlaceOrderRequest(ctx, a, categoryOption, params)
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
		Symbol:        a.ExtractSymbol(params.Symbol),
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
func (a *OptionAdapter) PlaceOrderWS(context.Context, *exchanges.OrderParams) error {
	return exchanges.ErrNotSupported
}
func (a *OptionAdapter) CancelOrder(ctx context.Context, orderID, symbol string) error {
	if err := requirePrivateClientAccess(a.client); err != nil {
		return err
	}
	_, err := a.client.CancelOrder(ctx, sdk.CancelOrderRequest{
		Category: categoryOption,
		Symbol:   a.FormatSymbol(symbol),
		OrderID:  orderID,
	})
	return err
}
func (a *OptionAdapter) CancelOrderWS(context.Context, string, string) error {
	return exchanges.ErrNotSupported
}
func (a *OptionAdapter) CancelAllOrders(ctx context.Context, symbol string) error {
	if err := requirePrivateClientAccess(a.client); err != nil {
		return err
	}
	req := sdk.CancelAllOrdersRequest{Category: categoryOption}
	if strings.TrimSpace(symbol) != "" {
		req.Symbol = a.FormatSymbol(symbol)
	}
	return a.client.CancelAllOrders(ctx, req)
}
func (a *OptionAdapter) FetchOrderByID(ctx context.Context, orderID, symbol string) (*exchanges.Order, error) {
	if err := requirePrivateClientAccess(a.client); err != nil {
		return nil, err
	}
	formatted := a.FormatSymbol(symbol)
	realtime, err := a.client.GetRealtimeOrders(ctx, categoryOption, formatted, "", orderID, "", -1)
	if err != nil {
		return nil, err
	}
	for _, order := range realtime {
		if order.OrderID == orderID || order.OrderLinkID == orderID {
			return mapOrder(a.ExtractSymbol(order.Symbol), order), nil
		}
	}

	history, err := a.client.GetOrderHistoryFiltered(ctx, categoryOption, formatted, orderID, "")
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
func (a *OptionAdapter) FetchOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	if err := requirePrivateClientAccess(a.client); err != nil {
		return nil, err
	}
	formatted := a.FormatSymbol(symbol)
	recentClosed, err := a.client.GetRealtimeOrders(ctx, categoryOption, formatted, "", "", "", 1)
	if err != nil {
		return nil, err
	}
	openOrders, err := a.client.GetRealtimeOrders(ctx, categoryOption, formatted, "", "", "", 0)
	if err != nil {
		return nil, err
	}
	history, err := a.client.GetOrderHistory(ctx, categoryOption, formatted)
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
func (a *OptionAdapter) FetchOpenOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	if err := requirePrivateClientAccess(a.client); err != nil {
		return nil, err
	}
	formatted := ""
	if strings.TrimSpace(symbol) != "" {
		formatted = a.FormatSymbol(symbol)
	}
	raw, err := a.client.GetRealtimeOrders(ctx, categoryOption, formatted, "", "", "", 0)
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
func (a *OptionAdapter) FetchAccount(ctx context.Context) (*exchanges.Account, error) {
	if err := requirePrivateClientAccess(a.client); err != nil {
		return nil, err
	}
	wallet, err := a.client.GetWalletBalance(ctx, "UNIFIED", "")
	if err != nil {
		return nil, err
	}
	openOrders, err := a.FetchOpenOrders(ctx, "")
	if err != nil {
		return nil, err
	}
	account := &exchanges.Account{Orders: openOrders}
	if len(wallet.List) > 0 {
		account.TotalBalance = parseDecimal(wallet.List[0].TotalEquity)
		account.AvailableBalance = parseDecimal(wallet.List[0].TotalAvailableBalance)
		account.UnrealizedPnL = parseDecimal(wallet.List[0].TotalPerpUPL)
	}
	return account, nil
}
func (a *OptionAdapter) FetchBalance(ctx context.Context) (decimal.Decimal, error) {
	if err := requirePrivateClientAccess(a.client); err != nil {
		return decimal.Zero, err
	}
	balance, err := a.client.GetWalletBalance(ctx, "UNIFIED", "")
	if err != nil {
		return decimal.Zero, err
	}
	if len(balance.List) == 0 {
		return decimal.Zero, nil
	}
	return parseDecimal(balance.List[0].TotalAvailableBalance), nil
}
func (a *OptionAdapter) FetchFeeRate(ctx context.Context, symbol string) (*exchanges.FeeRate, error) {
	if err := requirePrivateClientAccess(a.client); err != nil {
		return nil, err
	}
	raw, err := a.client.GetFeeRates(ctx, categoryOption, a.FormatSymbol(symbol))
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

func bybitStrikeFromSymbol(symbol string) decimal.Decimal {
	parts := strings.Split(strings.ToUpper(symbol), "-")
	for _, part := range parts {
		if value := parseDecimal(part); value.IsPositive() {
			return value
		}
	}
	return decimal.Zero
}

func bybitOptionTypeFromSymbol(symbol string) exchanges.OptionType {
	parts := strings.Split(strings.ToUpper(strings.TrimSpace(symbol)), "-")
	if len(parts) >= 4 {
		return exchanges.NormalizeOptionType(parts[3])
	}
	return ""
}

func bybitExpiryFromSymbol(symbol string) int64 {
	parts := strings.Split(strings.ToUpper(symbol), "-")
	if len(parts) < 2 || len(parts[1]) != 7 {
		return 0
	}
	value := strings.ToUpper(parts[1][:2]) + strings.Title(strings.ToLower(parts[1][2:5])) + parts[1][5:]
	expiry, err := time.Parse("02Jan06", value)
	if err != nil {
		return 0
	}
	return expiry.UTC().UnixMilli()
}

func canonicalToBybitOptionSymbol(symbol string) string {
	parts, ok := exchanges.ParseOptionSymbol(symbol)
	if !ok {
		return symbol
	}
	expiry := time.UnixMilli(parts.ExpiryTime).UTC()
	return strings.Join([]string{
		parts.BaseAsset,
		strings.ToUpper(expiry.Format("02Jan06")),
		parts.StrikePrice.String(),
		parts.Type.Suffix(),
	}, "-")
}

func bybitNativeToCanonicalOptionSymbol(symbol, quote, settle string) string {
	parts := strings.Split(symbol, "-")
	if len(parts) < 4 {
		return symbol
	}
	expiry := bybitExpiryFromSymbol(symbol)
	strike := parseDecimal(parts[2])
	typ := exchanges.NormalizeOptionType(parts[3])
	if len(parts) >= 5 && strings.TrimSpace(parts[4]) != "" {
		quote = parts[4]
		if settle == "" {
			settle = parts[4]
		}
	}
	canonical := exchanges.NewOptionSymbol(parts[0], quote, settle, expiry, strike, typ)
	if canonical == "" {
		return symbol
	}
	return canonical
}

func bybitUnderlyingFromOptionSymbol(symbol string) string {
	parts := strings.Split(strings.ToUpper(strings.TrimSpace(symbol)), "-")
	if len(parts) >= 4 {
		return parts[0]
	}
	return ""
}

type bybitOptionInstrumentClient interface {
	GetInstrumentsForBase(ctx context.Context, category, baseCoin string) ([]sdk.Instrument, error)
}

func fetchBybitOptionInstruments(ctx context.Context, client marketClient, underlying string) ([]sdk.Instrument, error) {
	underlying = strings.ToUpper(strings.TrimSpace(underlying))
	if typed, ok := client.(bybitOptionInstrumentClient); ok {
		if underlying != "" {
			return typed.GetInstrumentsForBase(ctx, categoryOption, underlying)
		}
	}
	return client.GetInstruments(ctx, categoryOption)
}

func fetchBybitOptionInstrumentsForUnderlyings(ctx context.Context, client marketClient, underlyings []string) ([]sdk.Instrument, error) {
	if len(underlyings) == 0 {
		return fetchBybitOptionInstruments(ctx, client, "")
	}
	var out []sdk.Instrument
	for _, underlying := range underlyings {
		instruments, err := fetchBybitOptionInstruments(ctx, client, underlying)
		if err != nil {
			return nil, err
		}
		out = append(out, instruments...)
	}
	return out, nil
}
