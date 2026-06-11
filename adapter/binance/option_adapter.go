package binance

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/sdk/binance/option"

	"github.com/shopspring/decimal"
)

// OptionAdapter is the Binance European Options adapter (eapi.binance.com).
//
// Symbol convention note: unlike the perp/spot adapters, the "symbol" string
// across Exchange interface methods is interpreted as the full instrument
// ID (e.g. "BTC-251226-100000-C"), not a base currency. Use
// FormatInstrument / ParseInstrument to bridge to *OptionInstrument.
type OptionAdapter struct {
	*exchanges.BaseAdapter
	client    *option.Client
	apiKey    string
	secretKey string
}

// NewOptionAdapter creates a Binance options adapter.
func NewOptionAdapter(ctx context.Context, opts Options) (*OptionAdapter, error) {
	if err := opts.validateCredentials(); err != nil {
		return nil, err
	}
	client := option.NewClient().WithCredentials(opts.APIKey, opts.SecretKey)
	base := exchanges.NewBaseAdapter("BINANCE_OPTION", exchanges.MarketTypeOption, opts.logger())

	return &OptionAdapter{
		BaseAdapter: base,
		client:      client,
		apiKey:      opts.APIKey,
		secretKey:   opts.SecretKey,
	}, nil
}

func (a *OptionAdapter) Close() error { return nil }

// =============================================================================
// Symbol mapping — for options the "symbol" is the instrument ID itself
// =============================================================================

func (a *OptionAdapter) FormatSymbol(symbol string) string  { return symbol }
func (a *OptionAdapter) ExtractSymbol(symbol string) string { return symbol }

// =============================================================================
// OptionExchange — market data
// =============================================================================

func (a *OptionAdapter) FetchOptionChain(ctx context.Context, underlying string, opts *exchanges.OptionChainOpts) ([]exchanges.OptionInstrument, error) {
	info, err := a.client.GetExchangeInfo(ctx)
	if err != nil {
		return nil, err
	}
	want := strings.ToUpper(underlying)

	out := make([]exchanges.OptionInstrument, 0, len(info.OptionSymbols))
	for _, s := range info.OptionSymbols {
		// Binance encodes the underlying spot pair (e.g. "BTCUSDT") on OptionSymbol;
		// our typed underlying is just "BTC".
		base := stripQuoteSuffix(s.Underlying)
		if !strings.EqualFold(base, want) {
			continue
		}
		inst, parseErr := parseBinanceOptionSymbol(s.Symbol, s.QuoteAsset)
		if parseErr != nil {
			continue
		}
		if opts != nil {
			if !optionChainOptsMatch(opts, inst) {
				continue
			}
		}
		out = append(out, *inst)
	}
	return out, nil
}

func (a *OptionAdapter) FetchExpirations(ctx context.Context, underlying string) ([]time.Time, error) {
	chain, err := a.FetchOptionChain(ctx, underlying, nil)
	if err != nil {
		return nil, err
	}
	seen := make(map[time.Time]struct{}, len(chain))
	out := make([]time.Time, 0, len(chain))
	for _, inst := range chain {
		if _, ok := seen[inst.Expiry]; ok {
			continue
		}
		seen[inst.Expiry] = struct{}{}
		out = append(out, inst.Expiry)
	}
	return out, nil
}

func (a *OptionAdapter) FetchGreeks(ctx context.Context, instrumentID string) (*exchanges.Greeks, error) {
	mark, err := a.FetchOptionMark(ctx, instrumentID)
	if err != nil {
		return nil, err
	}
	return &mark.Greeks, nil
}

func (a *OptionAdapter) FetchOptionMark(ctx context.Context, instrumentID string) (*exchanges.OptionMark, error) {
	resp, err := a.client.GetMark(ctx, instrumentID)
	if err != nil {
		return nil, err
	}
	if len(resp) == 0 {
		return nil, fmt.Errorf("binance options: no mark data for %s", instrumentID)
	}
	entry := resp[0]
	return &exchanges.OptionMark{
		InstrumentID: entry.Symbol,
		MarkPrice:    parseDecimal(entry.MarkPrice),
		MarkIV:       parseDecimal(entry.MarkIV),
		Greeks: exchanges.Greeks{
			Delta: parseDecimal(entry.Delta),
			Gamma: parseDecimal(entry.Gamma),
			Vega:  parseDecimal(entry.Vega),
			Theta: parseDecimal(entry.Theta),
			IV:    parseDecimal(entry.MarkIV),
		},
		Timestamp: time.Now().UnixMilli(),
	}, nil
}

func (a *OptionAdapter) FetchOptionPositions(ctx context.Context) ([]exchanges.Position, error) {
	resp, err := a.client.GetPositions(ctx, "")
	if err != nil {
		return nil, err
	}
	out := make([]exchanges.Position, 0, len(resp))
	for _, p := range resp {
		inst, parseErr := parseBinanceOptionSymbol(p.Symbol, p.QuoteAsset)
		if parseErr != nil {
			continue
		}
		side := exchanges.PositionSideLong
		if strings.EqualFold(p.Side, "SHORT") {
			side = exchanges.PositionSideShort
		}
		qty := parseDecimal(p.Quantity).Abs()
		greeks := exchanges.Greeks{}
		markIV := decimal.Zero
		premium := parseDecimal(p.MarkPrice)
		if mark, markErr := a.FetchOptionMark(ctx, p.Symbol); markErr == nil && mark != nil {
			greeks = mark.Greeks
			markIV = mark.MarkIV
			premium = mark.MarkPrice
		}
		out = append(out, exchanges.Position{
			Symbol:         p.Symbol,
			InstrumentType: exchanges.InstrumentTypeOption,
			Side:           side,
			Quantity:       qty,
			EntryPrice:     parseDecimal(p.EntryPrice),
			UnrealizedPnL:  parseDecimal(p.UnrealizedPNL),
			Option: &exchanges.OptionPositionData{
				Instrument:   *inst,
				Greeks:       greeks,
				MarkIV:       markIV,
				Premium:      premium,
				ContractSize: decimal.NewFromInt(1),
			},
		})
	}
	return out, nil
}

// =============================================================================
// OptionExchange — instrument formatting
// =============================================================================

// FormatInstrument renders an OptionInstrument as Binance's wire ID.
// Wire format: "<UNDERLYING>-<YYMMDD>-<STRIKE>-<C|P>".
func (a *OptionAdapter) FormatInstrument(inst *exchanges.OptionInstrument) string {
	if inst == nil {
		return ""
	}
	date := inst.Expiry.UTC().Format("060102")
	strike := formatStrikeInt(inst.Strike)
	return fmt.Sprintf("%s-%s-%s-%s",
		strings.ToUpper(inst.Underlying),
		date,
		strike,
		string(inst.Kind),
	)
}

// ParseInstrument is the inverse of FormatInstrument. Settlement is left
// empty because Binance encodes it on the parent contract, not the symbol.
func (a *OptionAdapter) ParseInstrument(id string) (*exchanges.OptionInstrument, error) {
	return parseBinanceOptionSymbol(id, "")
}

// =============================================================================
// Exchange — minimal trading surface (Symbol = instrument ID)
// =============================================================================

func (a *OptionAdapter) PlaceOrder(ctx context.Context, params *exchanges.OrderParams) (*exchanges.Order, error) {
	if params == nil {
		return nil, fmt.Errorf("binance options: nil params")
	}
	if params.Type != exchanges.OrderTypeLimit && params.Type != exchanges.OrderTypePostOnly {
		return nil, fmt.Errorf("binance options: %w: only LIMIT orders are supported", exchanges.ErrNotSupported)
	}
	req := &option.OrderRequest{
		Symbol:        params.Symbol,
		Side:          orderSideToBinance(params.Side),
		Type:          "LIMIT",
		Quantity:      params.Quantity.String(),
		ClientOrderID: params.ClientID,
	}
	if !params.Price.IsZero() {
		req.Price = params.Price.String()
	}
	if params.TimeInForce != "" {
		req.TimeInForce = string(params.TimeInForce)
	}
	req.ReduceOnly = params.ReduceOnly
	req.PostOnly = params.PostOnly || params.Type == exchanges.OrderTypePostOnly

	resp, err := a.client.PlaceOrder(ctx, req)
	if err != nil {
		return nil, err
	}
	return binanceOptionOrderToUnified(resp), nil
}

func (a *OptionAdapter) PlaceOrderWS(context.Context, *exchanges.OrderParams) error {
	return exchanges.ErrNotSupported
}

func (a *OptionAdapter) CancelOrder(ctx context.Context, orderID, symbol string) error {
	_, err := a.client.CancelOrder(ctx, symbol, orderID, "")
	return err
}

func (a *OptionAdapter) CancelOrderWS(context.Context, string, string) error {
	return exchanges.ErrNotSupported
}

func (a *OptionAdapter) CancelAllOrders(context.Context, string) error {
	return exchanges.ErrNotSupported
}

func (a *OptionAdapter) FetchOrderByID(ctx context.Context, orderID, symbol string) (*exchanges.Order, error) {
	resp, err := a.client.GetOrder(ctx, symbol, orderID, "")
	if err != nil {
		return nil, err
	}
	return binanceOptionOrderToUnified(resp), nil
}

func (a *OptionAdapter) FetchOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	return nil, exchanges.ErrNotSupported
}

func (a *OptionAdapter) FetchOpenOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	resp, err := a.client.GetOpenOrders(ctx, symbol)
	if err != nil {
		return nil, err
	}
	out := make([]exchanges.Order, 0, len(resp))
	for i := range resp {
		out = append(out, *binanceOptionOrderToUnified(&resp[i]))
	}
	return out, nil
}

// =============================================================================
// Exchange — account
// =============================================================================

func (a *OptionAdapter) FetchAccount(ctx context.Context) (*exchanges.Account, error) {
	resp, err := a.client.GetAccount(ctx)
	if err != nil {
		return nil, err
	}
	acc := &exchanges.Account{}
	for _, asset := range resp.Asset {
		acc.TotalBalance = acc.TotalBalance.Add(parseDecimal(asset.Equity))
		acc.AvailableBalance = acc.AvailableBalance.Add(parseDecimal(asset.Available))
		acc.UnrealizedPnL = acc.UnrealizedPnL.Add(parseDecimal(asset.UnrealizedPNL))
	}
	positions, posErr := a.FetchOptionPositions(ctx)
	if posErr == nil {
		acc.Positions = positions
	}
	return acc, nil
}

func (a *OptionAdapter) FetchBalance(ctx context.Context) (decimal.Decimal, error) {
	acc, err := a.FetchAccount(ctx)
	if err != nil {
		return decimal.Zero, err
	}
	return acc.TotalBalance, nil
}

func (a *OptionAdapter) FetchSymbolDetails(context.Context, string) (*exchanges.SymbolDetails, error) {
	return nil, exchanges.ErrNotSupported
}

func (a *OptionAdapter) FetchFeeRate(context.Context, string) (*exchanges.FeeRate, error) {
	return nil, exchanges.ErrNotSupported
}

// =============================================================================
// Public market data — option-specific helpers exist; the symbol-based
// generic methods on Exchange are not appropriate here.
// =============================================================================

func (a *OptionAdapter) FetchTicker(context.Context, string) (*exchanges.Ticker, error) {
	return nil, exchanges.ErrNotSupported
}

func (a *OptionAdapter) FetchOrderBook(context.Context, string, int) (*exchanges.OrderBook, error) {
	return nil, exchanges.ErrNotSupported
}

func (a *OptionAdapter) FetchTrades(context.Context, string, int) ([]exchanges.Trade, error) {
	return nil, exchanges.ErrNotSupported
}

func (a *OptionAdapter) FetchHistoricalTrades(context.Context, string, *exchanges.HistoricalTradeOpts) ([]exchanges.Trade, error) {
	return nil, exchanges.ErrNotSupported
}

func (a *OptionAdapter) FetchKlines(context.Context, string, exchanges.Interval, *exchanges.KlineOpts) ([]exchanges.Kline, error) {
	return nil, exchanges.ErrNotSupported
}

// =============================================================================
// Local OrderBook — not supported in v1 (WS deferred).
// =============================================================================

func (a *OptionAdapter) WatchOrderBook(context.Context, string, int, exchanges.OrderBookCallback) error {
	return exchanges.ErrNotSupported
}

func (a *OptionAdapter) StopWatchOrderBook(context.Context, string) error {
	return exchanges.ErrNotSupported
}

// =============================================================================
// Streamable — all stubs in v1 (WS deferred).
// =============================================================================

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
func (a *OptionAdapter) StopWatchOrders(context.Context) error         { return nil }
func (a *OptionAdapter) StopWatchFills(context.Context) error          { return nil }
func (a *OptionAdapter) StopWatchPositions(context.Context) error      { return nil }
func (a *OptionAdapter) StopWatchTicker(context.Context, string) error { return nil }
func (a *OptionAdapter) StopWatchTrades(context.Context, string) error { return nil }
func (a *OptionAdapter) StopWatchKlines(context.Context, string, exchanges.Interval) error {
	return nil
}

// =============================================================================
// Internal helpers
// =============================================================================

func stripQuoteSuffix(s string) string {
	// "BTCUSDT" → "BTC", "ETHUSDT" → "ETH", etc. Heuristic.
	for _, q := range []string{"USDT", "USDC", "USD", "BUSD"} {
		if strings.HasSuffix(s, q) {
			return strings.TrimSuffix(s, q)
		}
	}
	return s
}

func optionChainOptsMatch(opts *exchanges.OptionChainOpts, inst *exchanges.OptionInstrument) bool {
	if opts == nil {
		return true
	}
	if opts.Expiry != nil && !opts.Expiry.Equal(inst.Expiry) {
		return false
	}
	if !opts.MinStrike.IsZero() && inst.Strike.LessThan(opts.MinStrike) {
		return false
	}
	if !opts.MaxStrike.IsZero() && inst.Strike.GreaterThan(opts.MaxStrike) {
		return false
	}
	if opts.Kind != nil && inst.Kind != *opts.Kind {
		return false
	}
	return true
}

// parseBinanceOptionSymbol turns "BTC-251226-100000-C" into an OptionInstrument.
// settlement is optional; pass "" if unknown.
func parseBinanceOptionSymbol(symbol, settlement string) (*exchanges.OptionInstrument, error) {
	parts := strings.Split(symbol, "-")
	if len(parts) != 4 {
		return nil, fmt.Errorf("binance option symbol: expected 4 dash-separated parts, got %q", symbol)
	}
	underlying := strings.ToUpper(parts[0])

	expiry, err := time.Parse("060102", parts[1])
	if err != nil {
		return nil, fmt.Errorf("binance option symbol: parse expiry %q: %w", parts[1], err)
	}
	// Binance options expire 08:00 UTC on the date encoded in YYMMDD.
	expiry = time.Date(expiry.Year(), expiry.Month(), expiry.Day(), 8, 0, 0, 0, time.UTC)

	strike, err := decimal.NewFromString(parts[2])
	if err != nil {
		return nil, fmt.Errorf("binance option symbol: parse strike %q: %w", parts[2], err)
	}

	var kind exchanges.OptionKind
	switch strings.ToUpper(parts[3]) {
	case "C":
		kind = exchanges.OptionCall
	case "P":
		kind = exchanges.OptionPut
	default:
		return nil, fmt.Errorf("binance option symbol: unknown kind %q", parts[3])
	}

	if settlement == "" {
		settlement = "USDT"
	}

	return &exchanges.OptionInstrument{
		Underlying: underlying,
		Expiry:     expiry,
		Strike:     strike,
		Kind:       kind,
		Settlement: settlement,
	}, nil
}

func formatStrikeInt(strike decimal.Decimal) string {
	// Binance encodes strikes as integers when whole, otherwise with the
	// minimum decimal places needed. Most BTC/ETH strikes are integers.
	if strike.IsInteger() {
		return strike.StringFixed(0)
	}
	return strings.TrimRight(strings.TrimRight(strike.String(), "0"), ".")
}

func orderSideToBinance(s exchanges.OrderSide) string {
	switch s {
	case exchanges.OrderSideBuy:
		return "BUY"
	case exchanges.OrderSideSell:
		return "SELL"
	}
	return strings.ToUpper(string(s))
}

func orderTypeToBinance(t exchanges.OrderType) string {
	switch t {
	case exchanges.OrderTypeLimit:
		return "LIMIT"
	case exchanges.OrderTypeMarket:
		return "MARKET"
	}
	return strings.ToUpper(string(t))
}

func binanceOptionStatusToUnified(s string) exchanges.OrderStatus {
	switch strings.ToUpper(s) {
	case "ACCEPTED", "NEW":
		return exchanges.OrderStatusNew
	case "PARTIALLY_FILLED":
		return exchanges.OrderStatusPartiallyFilled
	case "FILLED":
		return exchanges.OrderStatusFilled
	case "CANCELLED", "CANCELED":
		return exchanges.OrderStatusCancelled
	case "REJECTED":
		return exchanges.OrderStatusRejected
	}
	return exchanges.OrderStatusPending
}

func binanceOptionOrderToUnified(r *option.OrderResponse) *exchanges.Order {
	if r == nil {
		return nil
	}
	return &exchanges.Order{
		OrderID:          strconv.FormatInt(r.OrderID, 10),
		ClientOrderID:    r.ClientOrderID,
		Symbol:           r.Symbol,
		Side:             binanceOptionSideToUnified(r.Side),
		Type:             binanceOptionTypeToUnified(r.Type),
		Quantity:         parseDecimal(r.Quantity),
		Price:            parseDecimal(r.Price),
		FilledQuantity:   parseDecimal(r.ExecutedQty),
		AverageFillPrice: parseDecimal(r.AvgPrice),
		Status:           binanceOptionStatusToUnified(r.Status),
		ReduceOnly:       r.ReduceOnly,
		TimeInForce:      exchanges.TimeInForce(r.TimeInForce),
		Timestamp:        r.UpdateTime,
	}
}

func binanceOptionSideToUnified(s string) exchanges.OrderSide {
	switch strings.ToUpper(s) {
	case "BUY":
		return exchanges.OrderSideBuy
	case "SELL":
		return exchanges.OrderSideSell
	}
	return exchanges.OrderSide(s)
}

func binanceOptionTypeToUnified(t string) exchanges.OrderType {
	switch strings.ToUpper(t) {
	case "LIMIT":
		return exchanges.OrderTypeLimit
	case "MARKET":
		return exchanges.OrderTypeMarket
	}
	return exchanges.OrderType(t)
}
