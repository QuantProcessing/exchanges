package binance

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/binance/sdk/margin"
	"github.com/QuantProcessing/exchanges/binance/sdk/spot"

	"github.com/shopspring/decimal"
)

// MarginAdapter implements MarginExtension and Adapter for Binance Margin
type MarginAdapter struct {
	*SpotAdapter // Reuse Spot Market Data and basic logic
	marginClient *margin.Client
}

func NewMarginAdapter(ctx context.Context, opts Options) (exchanges.Exchange, error) {
	// Initialize SpotAdapter as base
	spotAdapter, err := NewSpotAdapter(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("margin adapter init: %w", err)
	}

	marginClient := margin.NewClient().WithCredentials(opts.APIKey, opts.SecretKey)

	ma := &MarginAdapter{
		SpotAdapter:  spotAdapter,
		marginClient: marginClient,
	}
	// Update logger
	// Note: We cannot easily update logger of embedded struct if it's not exported or if we don't want to affect SpotAdapter if shared (but here it is new instance).
	// We can set ma.logger if SpotAdapter.logger is accessible. It is unexported but in same package.

	return ma, nil
}

func (a *MarginAdapter) GetMarketType() exchanges.MarketType {
	return exchanges.MarketTypeSpot
}

// GetMarginAccount implements MarginExtension
func (a *MarginAdapter) GetMarginAccount(ctx context.Context) (*exchanges.MarginAccount, error) {
	resp, err := a.marginClient.GetAccount(ctx)
	if err != nil {
		return nil, err
	}

	marginLevel := parseDecimal(resp.MarginLevel)
	totalAsset := parseDecimal(resp.TotalAssetOfBtc)
	totalLiability := parseDecimal(resp.TotalLiabilityOfBtc)
	totalNet := parseDecimal(resp.TotalNetAssetOfBtc)

	assets := make([]exchanges.MarginAsset, 0, len(resp.UserAssets))
	for _, ua := range resp.UserAssets {
		assets = append(assets, exchanges.MarginAsset{
			Asset:    ua.Asset,
			Borrowed: parseDecimal(ua.Borrowed),
			Free:     parseDecimal(ua.Free),
			Interest: parseDecimal(ua.Interest),
			Locked:   parseDecimal(ua.Locked),
			NetAsset: parseDecimal(ua.NetAsset),
		})
	}

	return &exchanges.MarginAccount{
		MarginLevel:       marginLevel,
		TotalAssetBTC:     totalAsset,
		TotalLiabilityBTC: totalLiability,
		TotalNetAssetBTC:  totalNet,
		UserAssets:        assets,
	}, nil
}

// GetIsolatedMarginAccount implements MarginExtension
func (a *MarginAdapter) GetIsolatedMarginAccount(ctx context.Context, symbols string) (*exchanges.IsolatedMarginAccount, error) {
	resp, err := a.marginClient.GetIsolatedAccount(ctx, symbols)
	if err != nil {
		return nil, err
	}

	totalAsset := parseDecimal(resp.TotalAssetOfBtc)
	totalLiability := parseDecimal(resp.TotalLiabilityOfBtc)
	totalNet := parseDecimal(resp.TotalNetAssetOfBtc)

	assets := make([]exchanges.IsolatedMarginSymbol, 0, len(resp.Assets))
	for _, s := range resp.Assets {
		marginLevel := parseDecimal(s.MarginLevel)
		marginRatio := parseDecimal(s.MarginRatio)
		indexPrice := parseDecimal(s.IndexPrice)
		liqPrice := parseDecimal(s.LiquidatePrice)
		liqRate := parseDecimal(s.LiquidateRate)

		baseAsset := exchanges.IsolatedMarginAsset{
			Asset:         s.BaseAsset.Asset,
			BorrowEnabled: s.BaseAsset.BorrowEnabled,
			Borrowed:      parseDecimal(s.BaseAsset.Borrowed),
			Free:          parseDecimal(s.BaseAsset.Free),
			Interest:      parseDecimal(s.BaseAsset.Interest),
			Locked:        parseDecimal(s.BaseAsset.Locked),
			NetAsset:      parseDecimal(s.BaseAsset.NetAsset),
			TotalAsset:    parseDecimal(s.BaseAsset.TotalAsset),
		}

		quoteAsset := exchanges.IsolatedMarginAsset{
			Asset:         s.QuoteAsset.Asset,
			BorrowEnabled: s.QuoteAsset.BorrowEnabled,
			Borrowed:      parseDecimal(s.QuoteAsset.Borrowed),
			Free:          parseDecimal(s.QuoteAsset.Free),
			Interest:      parseDecimal(s.QuoteAsset.Interest),
			Locked:        parseDecimal(s.QuoteAsset.Locked),
			NetAsset:      parseDecimal(s.QuoteAsset.NetAsset),
			TotalAsset:    parseDecimal(s.QuoteAsset.TotalAsset),
		}

		// Handle infinite margin level
		infinity := decimal.NewFromFloat(999)
		if marginLevel.GreaterThan(infinity) {
			marginLevel = infinity
		}

		assets = append(assets, exchanges.IsolatedMarginSymbol{
			Symbol:         s.Symbol,
			BaseAsset:      baseAsset,
			QuoteAsset:     quoteAsset,
			MarginLevel:    marginLevel,
			MarginRatio:    marginRatio,
			IndexPrice:     indexPrice,
			LiquidatePrice: liqPrice,
			LiquidateRate:  liqRate,
			Enabled:        s.Enabled,
		})
	}

	return &exchanges.IsolatedMarginAccount{
		Assets:            assets,
		TotalAssetBTC:     totalAsset,
		TotalLiabilityBTC: totalLiability,
		TotalNetAssetBTC:  totalNet,
	}, nil
}

func (a *MarginAdapter) Borrow(ctx context.Context, asset string, amount float64, isIsolated bool, symbol string) (string, error) {
	tranID, err := a.marginClient.Borrow(ctx, asset, amount, isIsolated, symbol)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d", tranID), nil
}

func (a *MarginAdapter) Repay(ctx context.Context, asset string, amount float64, isIsolated bool, symbol string) (string, error) {
	tranID, err := a.marginClient.Repay(ctx, asset, amount, isIsolated, symbol)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d", tranID), nil
}

// Override GetAccount to Generic Account
func (a *MarginAdapter) FetchAccount(ctx context.Context) (*exchanges.Account, error) {
	resp, err := a.marginClient.GetAccount(ctx)
	if err != nil {
		return nil, err
	}

	totalNet := parseDecimal(resp.TotalNetAssetOfBtc)

	return &exchanges.Account{
		TotalBalance:     totalNet, // In BTC
		AvailableBalance: decimal.Zero,
	}, nil
}

// Override PlaceOrder
func (a *MarginAdapter) PlaceOrder(ctx context.Context, params *exchanges.OrderParams) (*exchanges.Order, error) {
	// Use margin client
	formattedSymbol := strings.ToUpper(spot.FormatSymbol(params.Symbol))

	side := "BUY"
	if params.Side == exchanges.OrderSideSell {
		side = "SELL"
	}

	orderType := strings.ToUpper(string(params.Type))

	p := &margin.PlaceOrderParams{
		Symbol:           formattedSymbol,
		Side:             side,
		Type:             orderType,
		Quantity:         params.Quantity.InexactFloat64(),
		Price:            params.Price.InexactFloat64(),
		NewClientOrderID: params.ClientID,
		SideEffectType:   "NO_SIDE_EFFECT", // Default
		IsIsolated:       false,            // Default Cross Margin
	}

	if params.Type == exchanges.OrderTypeLimit || params.Type == exchanges.OrderTypePostOnly {
		p.TimeInForce = "GTC"
	}

	resp, err := a.marginClient.PlaceOrder(ctx, p)
	if err != nil {
		return nil, err
	}

	// Map response to exchanges.Order
	status := exchanges.OrderStatusNew
	switch resp.Status {
	case "FILLED":
		status = exchanges.OrderStatusFilled
	case "PARTIALLY_FILLED":
		status = exchanges.OrderStatusPartiallyFilled
	case "CANCELED":
		status = exchanges.OrderStatusCancelled
	case "REJECTED":
		status = exchanges.OrderStatusRejected
	}

	filledQty := parseDecimal(resp.ExecutedQty)
	origQty := parseDecimal(resp.OrigQty)
	price := parseDecimal(resp.Price)

	o := &exchanges.Order{
		OrderID:        fmt.Sprintf("%d", resp.OrderID),
		Symbol:         resp.Symbol,
		Side:           params.Side, // Or parse resp.Side
		Type:           params.Type,
		Quantity:       origQty,
		Price:          price,
		Status:         status,
		FilledQuantity: filledQty,
		Timestamp:      resp.TransactTime,
		ClientOrderID:  resp.ClientOrderID,
	}

	return o, nil
}

// Override FetchOrderByID
func (a *MarginAdapter) FetchOrderByID(ctx context.Context, orderID, symbol string) (*exchanges.Order, error) {
	formattedSymbol := spot.FormatSymbol(symbol)
	oid, err := strconv.ParseInt(orderID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid order id: %w", err)
	}

	resp, err := a.marginClient.GetOrder(ctx, formattedSymbol, oid, "", false)
	if err != nil {
		if isBinanceOrderLookupMiss(err) {
			return nil, exchanges.ErrOrderNotFound
		}
		return nil, err
	}

	status := a.mapStatus(resp.Status)

	filledQty := parseDecimal(resp.ExecutedQty)
	origQty := parseDecimal(resp.OrigQty)
	price := parseDecimal(resp.Price)

	side := exchanges.OrderSideBuy
	if resp.Side == "SELL" {
		side = exchanges.OrderSideSell
	}

	return &exchanges.Order{
		OrderID:        fmt.Sprintf("%d", resp.OrderID),
		Symbol:         resp.Symbol,
		Side:           side,
		Type:           exchanges.OrderType(resp.Type),
		Quantity:       origQty,
		Price:          price,
		Status:         status,
		FilledQuantity: filledQty,
		Timestamp:      resp.TransactTime,
		ClientOrderID:  resp.ClientOrderID,
	}, nil
}

func (a *MarginAdapter) FetchOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	return nil, exchanges.ErrNotSupported
}

// Override CancelOrder
func (a *MarginAdapter) CancelOrder(ctx context.Context, orderID, symbol string) error {
	formattedSymbol := spot.FormatSymbol(symbol)
	oid, err := strconv.ParseInt(orderID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid order id: %w", err)
	}

	_, err = a.marginClient.CancelOrder(ctx, formattedSymbol, oid, "", false)
	return err
}

// Override SubscribeOrderUpdate
func (a *MarginAdapter) WatchOrders(ctx context.Context, callback exchanges.OrderUpdateCallback) error {
	return exchanges.ErrNotSupported
}

// Helper to map status
func (a *MarginAdapter) mapStatus(s string) exchanges.OrderStatus {
	switch s {
	case "NEW":
		return exchanges.OrderStatusNew
	case "FILLED":
		return exchanges.OrderStatusFilled
	case "PARTIALLY_FILLED":
		return exchanges.OrderStatusPartiallyFilled
	case "CANCELED", "PENDING_CANCEL":
		return exchanges.OrderStatusCancelled
	case "REJECTED":
		return exchanges.OrderStatusRejected
	case "EXPIRED":
		return exchanges.OrderStatusCancelled
	default:
		return exchanges.OrderStatusUnknown
	}
}

func (a *MarginAdapter) WatchPositions(ctx context.Context, cb exchanges.PositionUpdateCallback) error {
	return exchanges.ErrNotSupported
}

func (a *MarginAdapter) WatchTicker(ctx context.Context, symbol string, cb exchanges.TickerCallback) error {
	return exchanges.ErrNotSupported
}

func (a *MarginAdapter) WatchOrderBook(ctx context.Context, symbol string, depth int, cb exchanges.OrderBookCallback) error {
	return exchanges.ErrNotSupported
}

func (a *MarginAdapter) WatchTrades(ctx context.Context, symbol string, cb exchanges.TradeCallback) error {
	return exchanges.ErrNotSupported
}

func (a *MarginAdapter) WatchKlines(ctx context.Context, symbol string, interval exchanges.Interval, cb exchanges.KlineCallback) error {
	return exchanges.ErrNotSupported
}

func (a *MarginAdapter) StopWatchOrders(ctx context.Context) error {
	return exchanges.ErrNotSupported
}

func (a *MarginAdapter) StopWatchPositions(ctx context.Context) error {
	return exchanges.ErrNotSupported
}

func (a *MarginAdapter) StopWatchTicker(ctx context.Context, symbol string) error {
	return exchanges.ErrNotSupported
}

func (a *MarginAdapter) StopWatchTrades(ctx context.Context, symbol string) error {
	return exchanges.ErrNotSupported
}

func (a *MarginAdapter) StopWatchKlines(ctx context.Context, symbol string, interval exchanges.Interval) error {
	return exchanges.ErrNotSupported
}

func (a *MarginAdapter) StopWatchOrderBook(ctx context.Context, symbol string) error {
	return exchanges.ErrNotSupported
}

func (a *MarginAdapter) GetLocalOrderBook(symbol string, depth int) *exchanges.OrderBook {
	return nil
}

func (a *MarginAdapter) FormatSymbol(symbol string) string {
	return symbol
}

func (a *MarginAdapter) ExtractSymbol(symbol string) string {
	return symbol
}
