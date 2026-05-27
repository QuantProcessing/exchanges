package okx

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	okx "github.com/QuantProcessing/exchanges/okx/sdk"
	"github.com/shopspring/decimal"
)

type OptionAdapter struct {
	*exchanges.BaseAdapter

	client *okx.Client

	optionFamilies []string
	mu             sync.RWMutex
	contracts      map[string]exchanges.OptionContract
	nativeToKey    map[string]string
}

func NewOptionAdapter(ctx context.Context, opts Options) (*OptionAdapter, error) {
	return newOptionAdapterWithClient(ctx, opts, exchanges.QuoteCurrencyUSDT, okx.NewClient())
}

func newOptionAdapterWithClient(ctx context.Context, opts Options, _ exchanges.QuoteCurrency, client *okx.Client) (*OptionAdapter, error) {
	if err := opts.validateCredentials(); err != nil {
		return nil, err
	}
	if opts.hasFullCredentials() {
		client.WithCredentials(opts.APIKey, opts.SecretKey, opts.Passphrase)
	}
	families := opts.optionFamilies()
	adp := &OptionAdapter{
		BaseAdapter:    exchanges.NewBaseAdapter("OKX", exchanges.MarketTypeOption, opts.logger()),
		client:         client,
		optionFamilies: families,
		contracts:      make(map[string]exchanges.OptionContract),
		nativeToKey:    make(map[string]string),
	}
	if err := adp.refreshFamilies(ctx, families); err != nil {
		return nil, fmt.Errorf("okx option init: %w", err)
	}
	return adp, nil
}

func (a *OptionAdapter) Close() error { return nil }

func (a *OptionAdapter) ListOptionContracts(ctx context.Context, underlying string) ([]exchanges.OptionContract, error) {
	families := optionFamilies(underlying)
	if len(families) == 0 {
		families = a.optionFamilies
	}
	if err := a.refreshFamilies(ctx, families); err != nil {
		return nil, err
	}

	filter := strings.ToUpper(strings.TrimSpace(underlying))
	filter = strings.TrimSuffix(filter, "-USD")

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
	key := a.ExtractSymbol(contractSymbol)
	a.mu.RLock()
	contract, ok := a.contracts[key]
	a.mu.RUnlock()
	if ok {
		copyContract := contract
		return &copyContract, nil
	}

	if family := okxFamilyFromOptionSymbol(contractSymbol); family != "" {
		if err := a.refreshFamilies(ctx, []string{family}); err != nil {
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

func (a *OptionAdapter) refreshFamilies(ctx context.Context, families []string) error {
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

	for _, family := range families {
		insts, err := a.client.GetInstrumentsByFamily(ctx, "OPTION", family)
		if err != nil {
			return err
		}
		for _, inst := range insts {
			contract := okxOptionContract(inst)
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

func okxOptionContract(inst okx.Instrument) exchanges.OptionContract {
	family := strings.ToUpper(firstNonEmptyString(inst.InstFamily, inst.Uly, okxFamilyFromOptionSymbol(inst.InstId)))
	familyParts := strings.Split(family, "-")
	base := strings.ToUpper(firstNonEmptyString(inst.CtValCcy, inst.BaseCcy))
	if base == "" && len(familyParts) > 0 {
		base = familyParts[0]
	}
	quote := strings.ToUpper(inst.QuoteCcy)
	if quote == "" && len(familyParts) > 1 {
		quote = familyParts[1]
	}

	expiry := parseInt64(inst.ExpTime)
	strike := parseDecimal(inst.Stk)
	typ := exchanges.NormalizeOptionType(inst.OptType)
	contractSize := parseDecimal(inst.CtVal)
	if mult := parseDecimal(inst.CtMult); mult.IsPositive() {
		contractSize = contractSize.Mul(mult)
	}

	settle := strings.ToUpper(firstNonEmptyString(inst.SettleCcy, inst.SettCcy))
	return exchanges.OptionContract{
		Symbol:         exchanges.NewOptionSymbol(base, quote, settle, expiry, strike, typ),
		ExchangeSymbol: strings.ToUpper(inst.InstId),
		Underlying:     strings.TrimSuffix(family, "-USD"),
		BaseAsset:      base,
		QuoteAsset:     quote,
		SettleAsset:    settle,
		Type:           typ,
		StrikePrice:    strike,
		ExpiryTime:     expiry,
		ContractSize:   contractSize,
		TickSize:       parseDecimal(inst.TickSz),
		LotSize:        parseDecimal(inst.LotSz),
		MinQuantity:    parseDecimal(inst.MinSz),
		Status:         inst.State,
	}
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
	return canonicalToOKXOptionSymbol(upper)
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
	return okxNativeToCanonicalOptionSymbol(upper)
}

func (a *OptionAdapter) FetchTicker(ctx context.Context, symbol string) (*exchanges.Ticker, error) {
	canonical := a.ExtractSymbol(symbol)
	res, err := a.client.GetTicker(ctx, a.FormatSymbol(symbol))
	if err != nil {
		return nil, err
	}
	if len(res) == 0 {
		return nil, fmt.Errorf("no ticker")
	}
	t := res[0]
	bid := parseDecimal(t.BidPx)
	ask := parseDecimal(t.AskPx)
	ticker := &exchanges.Ticker{
		Symbol:    canonical,
		LastPrice: parseDecimal(t.Last),
		Bid:       bid,
		Ask:       ask,
		High24h:   parseDecimal(t.High24h),
		Low24h:    parseDecimal(t.Low24h),
		Volume24h: parseDecimal(t.Vol24h),
		QuoteVol:  parseDecimal(t.VolCcy24h),
		Timestamp: parseTime(t.Ts),
	}
	if bid.IsPositive() && ask.IsPositive() {
		ticker.MidPrice = bid.Add(ask).Div(decimal.NewFromInt(2))
	}
	return ticker, nil
}

func (a *OptionAdapter) FetchOrderBook(ctx context.Context, symbol string, limit int) (*exchanges.OrderBook, error) {
	sz := 400
	if limit > 0 && limit < 400 {
		sz = limit
	}
	res, err := a.client.GetOrderBook(ctx, a.FormatSymbol(symbol), &sz)
	if err != nil {
		return nil, err
	}
	if len(res) == 0 {
		return nil, fmt.Errorf("no orderbook")
	}
	book := res[0]
	out := &exchanges.OrderBook{
		Symbol:    a.ExtractSymbol(symbol),
		Timestamp: parseTime(book.Ts),
		Bids:      make([]exchanges.Level, 0, len(book.Bids)),
		Asks:      make([]exchanges.Level, 0, len(book.Asks)),
	}
	for _, bid := range book.Bids {
		if len(bid) >= 2 {
			out.Bids = append(out.Bids, exchanges.Level{Price: parseDecimal(bid[0]), Quantity: parseDecimal(bid[1])})
		}
	}
	for _, ask := range book.Asks {
		if len(ask) >= 2 {
			out.Asks = append(out.Asks, exchanges.Level{Price: parseDecimal(ask[0]), Quantity: parseDecimal(ask[1])})
		}
	}
	return out, nil
}

func (a *OptionAdapter) FetchTrades(ctx context.Context, symbol string, limit int) ([]exchanges.Trade, error) {
	var rawLimit *int
	if limit > 0 {
		rawLimit = &limit
	}
	raw, err := a.client.GetTrades(ctx, a.FormatSymbol(symbol), rawLimit)
	if err != nil {
		return nil, err
	}
	canonical := a.ExtractSymbol(symbol)
	out := make([]exchanges.Trade, 0, len(raw))
	for _, trade := range raw {
		side := exchanges.TradeSideBuy
		if strings.EqualFold(trade.Side, "sell") {
			side = exchanges.TradeSideSell
		}
		out = append(out, exchanges.Trade{
			ID:        trade.TradeId,
			Symbol:    canonical,
			Price:     parseDecimal(trade.Px),
			Quantity:  parseDecimal(trade.Sz),
			Side:      side,
			Timestamp: parseTime(trade.Ts),
		})
	}
	return out, nil
}

func (a *OptionAdapter) FetchKlines(ctx context.Context, symbol string, interval exchanges.Interval, opts *exchanges.KlineOpts) ([]exchanges.Kline, error) {
	bar := okxKlineBar(interval)
	var after *string
	var before *string
	var limit *int
	if opts != nil {
		if opts.End != nil {
			value := fmt.Sprintf("%d", opts.End.UnixMilli())
			after = &value
		}
		if opts.Start != nil {
			value := fmt.Sprintf("%d", opts.Start.UnixMilli())
			before = &value
		}
		if opts.Limit > 0 {
			limit = &opts.Limit
		}
	}
	res, err := a.client.GetCandles(ctx, a.FormatSymbol(symbol), &bar, after, before, limit)
	if err != nil {
		return nil, err
	}
	canonical := a.ExtractSymbol(symbol)
	out := make([]exchanges.Kline, len(res))
	for i, k := range res {
		idx := len(res) - 1 - i
		out[idx] = exchanges.Kline{
			Symbol:    canonical,
			Interval:  interval,
			Timestamp: parseTime(k[0]),
			Open:      parseDecimal(k[1]),
			High:      parseDecimal(k[2]),
			Low:       parseDecimal(k[3]),
			Close:     parseDecimal(k[4]),
			Volume:    parseDecimal(k[5]),
			QuoteVol:  parseDecimal(k[7]),
		}
	}
	return out, nil
}

func (a *OptionAdapter) FetchSymbolDetails(ctx context.Context, symbol string) (*exchanges.SymbolDetails, error) {
	contract, err := a.FetchOptionContract(ctx, symbol)
	if err != nil {
		return nil, err
	}
	return a.GetSymbolDetail(contract.Symbol)
}

func (a *OptionAdapter) PlaceOrder(ctx context.Context, params *exchanges.OrderParams) (*exchanges.Order, error) {
	if err := a.BaseAdapter.ValidateOrder(params); err != nil {
		return nil, err
	}
	instID := a.FormatSymbol(params.Symbol)
	side := "buy"
	if params.Side == exchanges.OrderSideSell {
		side = "sell"
	}

	size := params.Quantity.String()
	price := params.Price.String()
	if detail, err := a.GetSymbolDetail(a.ExtractSymbol(params.Symbol)); err == nil {
		size = params.Quantity.StringFixed(detail.QuantityPrecision)
		if params.Price.IsPositive() {
			price = params.Price.StringFixed(detail.PricePrecision)
		}
	}

	var px *string
	if params.Price.IsPositive() {
		px = &price
	}
	var clOrdID *string
	if strings.TrimSpace(params.ClientID) != "" {
		clOrdID = &params.ClientID
	}

	req := &okx.OrderRequest{
		InstId:  instID,
		TdMode:  "cross",
		Side:    side,
		OrdType: okxOptionOrderType(params),
		Sz:      size,
		Px:      px,
		ClOrdId: clOrdID,
	}
	if params.ReduceOnly {
		reduceOnly := true
		req.ReduceOnly = &reduceOnly
	}

	ids, err := a.client.PlaceOrder(ctx, req)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("no response")
	}
	return &exchanges.Order{
		OrderID:       ids[0].OrdId,
		ClientOrderID: ids[0].ClOrdId,
		Symbol:        a.ExtractSymbol(params.Symbol),
		Side:          params.Side,
		Type:          params.Type,
		Quantity:      params.Quantity,
		Price:         params.Price,
		Status:        exchanges.OrderStatusPending,
		Timestamp:     time.Now().UnixMilli(),
		ReduceOnly:    params.ReduceOnly,
		TimeInForce:   params.TimeInForce,
	}, nil
}
func (a *OptionAdapter) PlaceOrderWS(context.Context, *exchanges.OrderParams) error {
	return exchanges.ErrNotSupported
}
func (a *OptionAdapter) CancelOrder(ctx context.Context, orderID, symbol string) error {
	_, err := a.client.CancelOrder(ctx, a.FormatSymbol(symbol), orderID, "")
	return err
}
func (a *OptionAdapter) CancelOrderWS(context.Context, string, string) error {
	return exchanges.ErrNotSupported
}
func (a *OptionAdapter) CancelAllOrders(ctx context.Context, symbol string) error {
	instType := "OPTION"
	var instID *string
	if strings.TrimSpace(symbol) != "" {
		value := a.FormatSymbol(symbol)
		instID = &value
	}
	openOrders, err := a.client.GetOrders(ctx, &instType, instID)
	if err != nil {
		return err
	}
	if len(openOrders) == 0 {
		return nil
	}

	reqs := make([]okx.CancelOrderRequest, 0, len(openOrders))
	for _, order := range openOrders {
		ordID := order.OrdId
		reqs = append(reqs, okx.CancelOrderRequest{InstId: order.InstId, OrdId: &ordID})
	}
	for i := 0; i < len(reqs); i += 20 {
		end := i + 20
		if end > len(reqs) {
			end = len(reqs)
		}
		if _, err := a.client.CancelOrders(ctx, reqs[i:end]); err != nil {
			return err
		}
	}
	return nil
}
func (a *OptionAdapter) FetchOrderByID(ctx context.Context, orderID, symbol string) (*exchanges.Order, error) {
	res, err := a.client.GetOrder(ctx, a.FormatSymbol(symbol), orderID, "")
	if err != nil {
		if isOKXOrderLookupMiss(err) {
			return nil, exchanges.ErrOrderNotFound
		}
		return nil, err
	}
	if len(res) == 0 {
		return nil, exchanges.ErrOrderNotFound
	}
	return a.mapOptionOrderRest(&res[0]), nil
}
func (a *OptionAdapter) FetchOrders(context.Context, string) ([]exchanges.Order, error) {
	return nil, exchanges.ErrNotSupported
}
func (a *OptionAdapter) FetchOpenOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	instType := "OPTION"
	var instID *string
	if strings.TrimSpace(symbol) != "" {
		value := a.FormatSymbol(symbol)
		instID = &value
	}
	res, err := a.client.GetOrders(ctx, &instType, instID)
	if err != nil {
		return nil, err
	}

	orders := make([]exchanges.Order, 0, len(res))
	for _, raw := range res {
		orders = append(orders, *a.mapOptionOrderRest(&raw))
	}
	return orders, nil
}
func (a *OptionAdapter) FetchAccount(ctx context.Context) (*exchanges.Account, error) {
	bals, err := a.client.GetAccountBalance(ctx, nil)
	if err != nil {
		return nil, err
	}
	openOrders, err := a.FetchOpenOrders(ctx, "")
	if err != nil {
		return nil, err
	}
	account := &exchanges.Account{Orders: openOrders}
	if len(bals) > 0 {
		account.TotalBalance = parseDecimal(bals[0].TotalEq)
		if len(bals[0].Details) > 0 {
			account.AvailableBalance = parseDecimal(bals[0].Details[0].AvailEq)
		}
		account.UnrealizedPnL = parseDecimal(bals[0].Upl)
	}
	return account, nil
}
func (a *OptionAdapter) FetchBalance(ctx context.Context) (decimal.Decimal, error) {
	account, err := a.FetchAccount(ctx)
	if err != nil {
		return decimal.Zero, err
	}
	return account.TotalBalance, nil
}
func (a *OptionAdapter) FetchFeeRate(ctx context.Context, symbol string) (*exchanges.FeeRate, error) {
	var instID *string
	if strings.TrimSpace(symbol) != "" {
		value := a.FormatSymbol(symbol)
		instID = &value
	}
	res, err := a.client.GetTradeFee(ctx, "OPTION", instID)
	if err != nil {
		return nil, err
	}
	if len(res) == 0 || len(res[0].FeeGroups) == 0 {
		return nil, exchanges.ErrNotSupported
	}
	group := res[0].FeeGroups[0]
	return &exchanges.FeeRate{
		Maker: parseDecimal(group.Maker).Abs(),
		Taker: parseDecimal(group.Taker).Abs(),
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

func optionFamilies(underlying string) []string {
	underlying = strings.ToUpper(strings.TrimSpace(underlying))
	if underlying == "" {
		return nil
	}
	if strings.Contains(underlying, "-") {
		return []string{underlying}
	}
	return []string{underlying + "-USD"}
}

func okxFamilyFromOptionSymbol(symbol string) string {
	parts := strings.Split(strings.ToUpper(strings.TrimSpace(symbol)), "-")
	if len(parts) >= 5 {
		return parts[0] + "-" + parts[1]
	}
	if len(parts) == 4 {
		return parts[0] + "-USD"
	}
	return ""
}

func canonicalToOKXOptionSymbol(symbol string) string {
	parts, ok := exchanges.ParseOptionSymbol(symbol)
	if !ok {
		return symbol
	}
	quote := parts.QuoteAsset
	if quote == "" {
		quote = "USD"
	}
	expiry := time.UnixMilli(parts.ExpiryTime).UTC().Format("060102")
	return strings.Join([]string{parts.BaseAsset, quote, expiry, parts.StrikePrice.String(), parts.Type.Suffix()}, "-")
}

func okxNativeToCanonicalOptionSymbol(symbol string) string {
	parts := strings.Split(symbol, "-")
	if len(parts) != 5 || len(parts[2]) != 6 {
		return symbol
	}
	expiry, err := time.Parse("20060102", "20"+parts[2])
	if err != nil {
		return symbol
	}
	strike := parseDecimal(parts[3])
	return exchanges.NewOptionSymbol(parts[0], parts[1], parts[0], expiry.UTC().UnixMilli(), strike, exchanges.NormalizeOptionType(parts[4]))
}

func okxOptionOrderType(params *exchanges.OrderParams) string {
	switch params.Type {
	case exchanges.OrderTypeMarket:
		return "market"
	case exchanges.OrderTypePostOnly:
		return "post_only"
	case exchanges.OrderTypeLimit:
		switch params.TimeInForce {
		case exchanges.TimeInForceIOC:
			return "ioc"
		case exchanges.TimeInForceFOK:
			return "fok"
		default:
			return "limit"
		}
	default:
		return "limit"
	}
}

func (a *OptionAdapter) mapOptionOrderRest(raw *okx.Order) *exchanges.Order {
	status := exchanges.OrderStatusUnknown
	switch raw.State {
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
	if raw.Side == okx.SideSell {
		side = exchanges.OrderSideSell
	}

	ts := parseTime(firstNonEmptyString(raw.UTime, raw.CTime))
	if ts == 0 {
		ts = time.Now().UnixMilli()
	}
	reduceOnly := strings.EqualFold(raw.ReduceOnly, "true")
	return &exchanges.Order{
		OrderID:          raw.OrdId,
		ClientOrderID:    raw.ClOrdId,
		Symbol:           a.ExtractSymbol(raw.InstId),
		Side:             side,
		Type:             mapOKXOrderType(raw.OrdType),
		Quantity:         parseDecimal(raw.Sz),
		FilledQuantity:   parseDecimal(raw.AccFillSz),
		Price:            parseDecimal(firstNonEmptyString(raw.Px, raw.AvgPx)),
		OrderPrice:       parseDecimal(raw.Px),
		AverageFillPrice: parseDecimal(raw.AvgPx),
		LastFillPrice:    parseDecimal(raw.FillPx),
		LastFillQuantity: parseDecimal(raw.FillSz),
		Fee:              parseDecimal(raw.Fee),
		Status:           status,
		Timestamp:        ts,
		ReduceOnly:       reduceOnly,
	}
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func okxKlineBar(interval exchanges.Interval) string {
	switch interval {
	case exchanges.Interval1m:
		return "1m"
	case exchanges.Interval3m:
		return "3m"
	case exchanges.Interval5m:
		return "5m"
	case exchanges.Interval15m:
		return "15m"
	case exchanges.Interval30m:
		return "30m"
	case exchanges.Interval1h:
		return "1H"
	case exchanges.Interval2h:
		return "2H"
	case exchanges.Interval4h:
		return "4H"
	case exchanges.Interval6h:
		return "6H"
	case exchanges.Interval12h:
		return "12H"
	case exchanges.Interval1d:
		return "1D"
	case exchanges.Interval1w:
		return "1W"
	case exchanges.Interval1M:
		return "1M"
	default:
		return string(interval)
	}
}
