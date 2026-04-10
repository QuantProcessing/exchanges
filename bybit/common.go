package bybit

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/bybit/sdk"
	"github.com/shopspring/decimal"
)

const exchangeName = "BYBIT"

const (
	categorySpot            = "spot"
	categoryLinear          = "linear"
	instrumentStatusTrading = "Trading"
)

type marketClient interface {
	GetInstruments(ctx context.Context, category string) ([]sdk.Instrument, error)
	GetTicker(ctx context.Context, category, symbol string) (*sdk.Ticker, error)
	GetOrderBook(ctx context.Context, category, symbol string, limit int) (*sdk.OrderBook, error)
	GetRecentTrades(ctx context.Context, category, symbol string, limit int) ([]sdk.PublicTrade, error)
	GetKlines(ctx context.Context, category, symbol, interval string, start, end int64, limit int) ([]sdk.Candle, error)
	GetWalletBalance(ctx context.Context, accountType, coin string) (*sdk.WalletBalanceResult, error)
	GetFeeRates(ctx context.Context, category, symbol string) ([]sdk.FeeRateRecord, error)
	GetPositions(ctx context.Context, category, symbol, settleCoin string) ([]sdk.PositionRecord, error)
	SetLeverage(ctx context.Context, req sdk.SetLeverageRequest) error
	PlaceOrder(ctx context.Context, req sdk.PlaceOrderRequest) (*sdk.OrderActionResponse, error)
	CancelOrder(ctx context.Context, req sdk.CancelOrderRequest) (*sdk.OrderActionResponse, error)
	CancelAllOrders(ctx context.Context, req sdk.CancelAllOrdersRequest) error
	AmendOrder(ctx context.Context, req sdk.AmendOrderRequest) (*sdk.OrderActionResponse, error)
	GetRealtimeOrders(ctx context.Context, category, symbol, settleCoin, orderID, orderLinkID string, openOnly int) ([]sdk.OrderRecord, error)
	GetOpenOrders(ctx context.Context, category, symbol string) ([]sdk.OrderRecord, error)
	GetOrderHistory(ctx context.Context, category, symbol string) ([]sdk.OrderRecord, error)
	GetOrderHistoryFiltered(ctx context.Context, category, symbol, orderID, orderLinkID string) ([]sdk.OrderRecord, error)
	HasCredentials() bool
}

type publicWSClient interface {
	Subscribe(ctx context.Context, topic string, handler func(json.RawMessage)) error
	Unsubscribe(ctx context.Context, topic string) error
	Close() error
}

type privateWSClient interface {
	Subscribe(ctx context.Context, topic string, handler func(json.RawMessage)) error
	Unsubscribe(ctx context.Context, topic string) error
	Close() error
}

type tradeWSClient interface {
	PlaceOrder(ctx context.Context, req sdk.PlaceOrderRequest) error
	CancelOrder(ctx context.Context, req sdk.CancelOrderRequest) error
	AmendOrder(ctx context.Context, req sdk.AmendOrderRequest) error
	Close() error
}

type marketCache struct {
	byBase   map[string]sdk.Instrument
	bySymbol map[string]sdk.Instrument
}

func newMarketCache() *marketCache {
	return &marketCache{
		byBase:   make(map[string]sdk.Instrument),
		bySymbol: make(map[string]sdk.Instrument),
	}
}

func buildMarketCache(instruments []sdk.Instrument, quote exchanges.QuoteCurrency) *marketCache {
	cache := newMarketCache()
	for _, inst := range instruments {
		if strings.ToUpper(inst.QuoteCoin) != string(quote) {
			continue
		}
		if !strings.EqualFold(inst.Status, instrumentStatusTrading) {
			continue
		}
		base := strings.ToUpper(inst.BaseCoin)
		symbol := strings.ToUpper(inst.Symbol)
		cache.byBase[base] = inst
		cache.bySymbol[symbol] = inst
	}
	return cache
}

func buildSymbolDetails(instruments []sdk.Instrument, quote exchanges.QuoteCurrency) map[string]*exchanges.SymbolDetails {
	details := make(map[string]*exchanges.SymbolDetails)
	for _, inst := range instruments {
		if strings.ToUpper(inst.QuoteCoin) != string(quote) {
			continue
		}
		if !strings.EqualFold(inst.Status, instrumentStatusTrading) {
			continue
		}
		detail, err := symbolDetailsFromInstrument(inst)
		if err != nil {
			continue
		}
		details[detail.Symbol] = detail
	}
	return details
}

func symbolDetailsFromInstrument(inst sdk.Instrument) (*exchanges.SymbolDetails, error) {
	pricePrecision, err := strconv.ParseInt(inst.PriceScale, 10, 32)
	if err != nil && inst.PriceScale != "" {
		return nil, err
	}

	qtyPrecision := precisionFromStep(firstNonEmpty(inst.LotSizeFilter.QtyStep, inst.LotSizeFilter.BasePrecision))
	return &exchanges.SymbolDetails{
		Symbol:            strings.ToUpper(inst.BaseCoin),
		PricePrecision:    int32(pricePrecision),
		QuantityPrecision: qtyPrecision,
		MinQuantity:       parseDecimal(inst.LotSizeFilter.MinOrderQty),
		MinNotional:       parseDecimal(firstNonEmpty(inst.LotSizeFilter.MinNotionalValue, inst.LotSizeFilter.MinOrderAmt)),
	}, nil
}

func authError(message string) error {
	return exchanges.NewExchangeError(exchangeName, "", message, exchanges.ErrAuthFailed)
}

func hasAnyCredentials(opts Options) bool {
	return opts.APIKey != "" || opts.SecretKey != ""
}

func hasFullCredentials(opts Options) bool {
	return opts.APIKey != "" && opts.SecretKey != ""
}

func precisionFromStep(raw string) int32 {
	if raw == "" {
		return 0
	}
	value := strings.TrimRight(strings.TrimSpace(raw), "0")
	if idx := strings.IndexByte(value, '.'); idx >= 0 {
		return int32(len(value) - idx - 1)
	}
	return 0
}

func parseDecimal(raw string) decimal.Decimal {
	if raw == "" {
		return decimal.Zero
	}
	value, err := decimal.NewFromString(raw)
	if err != nil {
		return decimal.Zero
	}
	return value
}

func parseMillis(raw string) int64 {
	if raw == "" {
		return 0
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func requirePrivateAccess(client *sdk.Client) error {
	if client == nil || !client.HasCredentials() {
		return authError("bybit: private access requires api_key and secret_key")
	}
	return nil
}

func requirePrivateClientAccess(client marketClient) error {
	if client == nil || !client.HasCredentials() {
		return authError("bybit: private access requires api_key and secret_key")
	}
	return nil
}

func orderBookTopic(symbol string) string {
	return "orderbook.50." + symbol
}

func toTicker(symbol string, raw *sdk.Ticker) *exchanges.Ticker {
	ticker := &exchanges.Ticker{
		Symbol:     strings.ToUpper(symbol),
		LastPrice:  parseDecimal(raw.LastPrice),
		IndexPrice: parseDecimal(raw.IndexPrice),
		MarkPrice:  parseDecimal(raw.MarkPrice),
		Bid:        parseDecimal(raw.Bid1Price),
		Ask:        parseDecimal(raw.Ask1Price),
		Volume24h:  parseDecimal(raw.Volume24h),
		QuoteVol:   parseDecimal(raw.Turnover24h),
		High24h:    parseDecimal(raw.HighPrice24h),
		Low24h:     parseDecimal(raw.LowPrice24h),
		Timestamp:  parseMillis(firstNonEmpty(raw.Time, raw.TS)),
	}
	if ticker.Bid.IsPositive() && ticker.Ask.IsPositive() {
		ticker.MidPrice = ticker.Bid.Add(ticker.Ask).Div(decimal.NewFromInt(2))
	}
	return ticker
}

func toOrderBook(symbol string, raw *sdk.OrderBook) *exchanges.OrderBook {
	book := &exchanges.OrderBook{
		Symbol:    strings.ToUpper(symbol),
		Timestamp: raw.TS,
		Bids:      make([]exchanges.Level, 0, len(raw.Bids)),
		Asks:      make([]exchanges.Level, 0, len(raw.Asks)),
	}
	for _, level := range raw.Bids {
		if len(level) < 2 {
			continue
		}
		book.Bids = append(book.Bids, exchanges.Level{
			Price:    parseDecimal(string(level[0])),
			Quantity: parseDecimal(string(level[1])),
		})
	}
	for _, level := range raw.Asks {
		if len(level) < 2 {
			continue
		}
		book.Asks = append(book.Asks, exchanges.Level{
			Price:    parseDecimal(string(level[0])),
			Quantity: parseDecimal(string(level[1])),
		})
	}
	return book
}

func mapTrades(symbol string, raw []sdk.PublicTrade) []exchanges.Trade {
	out := make([]exchanges.Trade, 0, len(raw))
	for _, trade := range raw {
		side := exchanges.TradeSideBuy
		if strings.EqualFold(trade.Side, "sell") {
			side = exchanges.TradeSideSell
		}
		out = append(out, exchanges.Trade{
			ID:        trade.ExecID,
			Symbol:    strings.ToUpper(symbol),
			Price:     parseDecimal(trade.Price),
			Quantity:  parseDecimal(trade.Size),
			Side:      side,
			Timestamp: parseMillis(trade.Time),
		})
	}
	return out
}

func mapKlines(symbol string, interval exchanges.Interval, raw []sdk.Candle) []exchanges.Kline {
	out := make([]exchanges.Kline, 0, len(raw))
	for _, candle := range raw {
		out = append(out, exchanges.Kline{
			Symbol:    strings.ToUpper(symbol),
			Interval:  interval,
			Timestamp: parseMillis(string(candle[0])),
			Open:      parseDecimal(string(candle[1])),
			High:      parseDecimal(string(candle[2])),
			Low:       parseDecimal(string(candle[3])),
			Close:     parseDecimal(string(candle[4])),
			Volume:    parseDecimal(string(candle[5])),
			QuoteVol:  parseDecimal(string(candle[6])),
		})
	}
	return out
}

func mapPosition(symbol string, raw sdk.PositionRecord) exchanges.Position {
	side := exchanges.PositionSideLong
	if strings.EqualFold(raw.Side, "sell") {
		side = exchanges.PositionSideShort
	}
	return exchanges.Position{
		Symbol:           strings.ToUpper(symbol),
		Side:             side,
		Quantity:         parseDecimal(raw.Size),
		EntryPrice:       parseDecimal(raw.AvgPrice),
		UnrealizedPnL:    parseDecimal(raw.UnrealisedPnl),
		RealizedPnL:      parseDecimal(raw.CumRealisedPnl),
		LiquidationPrice: parseDecimal(raw.LiqPrice),
		Leverage:         parseDecimal(raw.Leverage),
	}
}

func mapSpotBalances(raw []sdk.WalletCoin) []exchanges.SpotBalance {
	out := make([]exchanges.SpotBalance, 0, len(raw))
	for _, coin := range raw {
		free := parseDecimal(rawAvailable(coin))
		locked := parseDecimal(coin.Locked)
		total := parseDecimal(firstNonEmpty(coin.Equity, free.Add(locked).String()))
		out = append(out, exchanges.SpotBalance{
			Asset:  strings.ToUpper(coin.Coin),
			Free:   free,
			Locked: locked,
			Total:  total,
		})
	}
	return out
}

func mapExecutionFill(symbol string, raw sdk.ExecutionRecord) *exchanges.Fill {
	return &exchanges.Fill{
		TradeID:       raw.ExecID,
		OrderID:       raw.OrderID,
		ClientOrderID: raw.OrderLinkID,
		Symbol:        strings.ToUpper(symbol),
		Side:          mapOrderSide(raw.Side),
		Price:         parseDecimal(raw.ExecPrice),
		Quantity:      parseDecimal(raw.ExecQty),
		Fee:           parseDecimal(raw.ExecFee),
		FeeAsset:      raw.FeeCurrency,
		IsMaker:       raw.IsMaker,
		Timestamp:     parseMillis(raw.ExecTime),
	}
}

func rawAvailable(coin sdk.WalletCoin) string {
	return firstNonEmpty(coin.WalletBalance, coin.Equity)
}

func klineIntervalString(interval exchanges.Interval) (string, error) {
	switch interval {
	case exchanges.Interval1m:
		return "1", nil
	case exchanges.Interval3m:
		return "3", nil
	case exchanges.Interval5m:
		return "5", nil
	case exchanges.Interval15m:
		return "15", nil
	case exchanges.Interval30m:
		return "30", nil
	case exchanges.Interval1h:
		return "60", nil
	case exchanges.Interval2h:
		return "120", nil
	case exchanges.Interval4h:
		return "240", nil
	case exchanges.Interval6h:
		return "360", nil
	case exchanges.Interval12h:
		return "720", nil
	case exchanges.Interval1d:
		return "D", nil
	case exchanges.Interval1w:
		return "W", nil
	case exchanges.Interval1M:
		return "M", nil
	default:
		return "", fmt.Errorf("bybit: unsupported interval %s", interval)
	}
}

func intervalDuration(interval exchanges.Interval) (time.Duration, error) {
	switch interval {
	case exchanges.Interval1m:
		return time.Minute, nil
	case exchanges.Interval3m:
		return 3 * time.Minute, nil
	case exchanges.Interval5m:
		return 5 * time.Minute, nil
	case exchanges.Interval15m:
		return 15 * time.Minute, nil
	case exchanges.Interval30m:
		return 30 * time.Minute, nil
	case exchanges.Interval1h:
		return time.Hour, nil
	case exchanges.Interval2h:
		return 2 * time.Hour, nil
	case exchanges.Interval4h:
		return 4 * time.Hour, nil
	case exchanges.Interval6h:
		return 6 * time.Hour, nil
	case exchanges.Interval12h:
		return 12 * time.Hour, nil
	case exchanges.Interval1d:
		return 24 * time.Hour, nil
	case exchanges.Interval1w:
		return 7 * 24 * time.Hour, nil
	case exchanges.Interval1M:
		return 30 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("bybit: unsupported interval %s", interval)
	}
}

func klineTimeRange(interval exchanges.Interval, opts *exchanges.KlineOpts) (int64, int64, int, error) {
	dur, err := intervalDuration(interval)
	if err != nil {
		return 0, 0, 0, err
	}

	end := time.Now().UTC()
	if opts != nil && opts.End != nil {
		end = opts.End.UTC()
	}

	limit := 200
	if opts != nil && opts.Limit > 0 {
		limit = opts.Limit
	}

	start := end.Add(-time.Duration(limit) * dur)
	if opts != nil && opts.Start != nil {
		start = opts.Start.UTC()
	}
	return start.UnixMilli(), end.UnixMilli(), limit, nil
}
