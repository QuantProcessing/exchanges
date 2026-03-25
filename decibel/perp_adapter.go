package decibel

import (
	"context"
	"fmt"
	"strings"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/shopspring/decimal"
)

var bootstrapMetadata = func(context.Context, *Adapter) error {
	return nil
}

// Adapter is the Decibel perpetual futures adapter.
type Adapter struct {
	*exchanges.BaseAdapter

	apiKey         string
	privateKey     string
	subaccountAddr string
	quoteCurrency  exchanges.QuoteCurrency
}

func NewAdapter(ctx context.Context, opts Options) (*Adapter, error) {
	if err := opts.validateCredentials(); err != nil {
		return nil, err
	}

	adp := &Adapter{
		BaseAdapter:    exchanges.NewBaseAdapter("DECIBEL", exchanges.MarketTypePerp, opts.logger()),
		apiKey:         opts.APIKey,
		privateKey:     opts.PrivateKey,
		subaccountAddr: opts.SubaccountAddr,
		quoteCurrency:  opts.quoteCurrency(),
	}

	if err := bootstrapMetadata(ctx, adp); err != nil {
		return nil, fmt.Errorf("decibel init: %w", err)
	}

	return adp, nil
}

func (a *Adapter) Close() error {
	return nil
}

func (a *Adapter) FormatSymbol(symbol string) string {
	return strings.ToUpper(symbol)
}

func (a *Adapter) ExtractSymbol(symbol string) string {
	return strings.ToUpper(symbol)
}

func (a *Adapter) FetchTicker(context.Context, string) (*exchanges.Ticker, error) {
	return nil, unsupported("FetchTicker")
}

func (a *Adapter) FetchOrderBook(context.Context, string, int) (*exchanges.OrderBook, error) {
	return nil, unsupported("FetchOrderBook")
}

func (a *Adapter) FetchTrades(context.Context, string, int) ([]exchanges.Trade, error) {
	return nil, unsupported("FetchTrades")
}

func (a *Adapter) FetchKlines(context.Context, string, exchanges.Interval, *exchanges.KlineOpts) ([]exchanges.Kline, error) {
	return nil, unsupported("FetchKlines")
}

func (a *Adapter) PlaceOrder(context.Context, *exchanges.OrderParams) (*exchanges.Order, error) {
	return nil, unsupported("PlaceOrder")
}

func (a *Adapter) CancelOrder(context.Context, string, string) error {
	return unsupported("CancelOrder")
}

func (a *Adapter) CancelAllOrders(context.Context, string) error {
	return unsupported("CancelAllOrders")
}

func (a *Adapter) FetchOrderByID(context.Context, string, string) (*exchanges.Order, error) {
	return nil, unsupported("FetchOrderByID")
}

func (a *Adapter) FetchOrders(context.Context, string) ([]exchanges.Order, error) {
	return nil, unsupported("FetchOrders")
}

func (a *Adapter) FetchOpenOrders(context.Context, string) ([]exchanges.Order, error) {
	return nil, unsupported("FetchOpenOrders")
}

func (a *Adapter) FetchAccount(context.Context) (*exchanges.Account, error) {
	return nil, unsupported("FetchAccount")
}

func (a *Adapter) FetchBalance(context.Context) (decimal.Decimal, error) {
	return decimal.Zero, unsupported("FetchBalance")
}

func (a *Adapter) FetchSymbolDetails(context.Context, string) (*exchanges.SymbolDetails, error) {
	return nil, unsupported("FetchSymbolDetails")
}

func (a *Adapter) FetchFeeRate(context.Context, string) (*exchanges.FeeRate, error) {
	return nil, unsupported("FetchFeeRate")
}

func (a *Adapter) WatchOrderBook(context.Context, string, exchanges.OrderBookCallback) error {
	return unsupported("WatchOrderBook")
}

func (a *Adapter) StopWatchOrderBook(context.Context, string) error {
	return unsupported("StopWatchOrderBook")
}

func (a *Adapter) WatchOrders(context.Context, exchanges.OrderUpdateCallback) error {
	return unsupported("WatchOrders")
}

func (a *Adapter) WatchPositions(context.Context, exchanges.PositionUpdateCallback) error {
	return unsupported("WatchPositions")
}

func (a *Adapter) WatchTicker(context.Context, string, exchanges.TickerCallback) error {
	return unsupported("WatchTicker")
}

func (a *Adapter) WatchTrades(context.Context, string, exchanges.TradeCallback) error {
	return unsupported("WatchTrades")
}

func (a *Adapter) WatchKlines(context.Context, string, exchanges.Interval, exchanges.KlineCallback) error {
	return unsupported("WatchKlines")
}

func (a *Adapter) StopWatchOrders(context.Context) error {
	return unsupported("StopWatchOrders")
}

func (a *Adapter) StopWatchPositions(context.Context) error {
	return unsupported("StopWatchPositions")
}

func (a *Adapter) StopWatchTicker(context.Context, string) error {
	return unsupported("StopWatchTicker")
}

func (a *Adapter) StopWatchTrades(context.Context, string) error {
	return unsupported("StopWatchTrades")
}

func (a *Adapter) StopWatchKlines(context.Context, string, exchanges.Interval) error {
	return unsupported("StopWatchKlines")
}

func (a *Adapter) FetchPositions(context.Context) ([]exchanges.Position, error) {
	return nil, unsupported("FetchPositions")
}

func (a *Adapter) SetLeverage(context.Context, string, int) error {
	return unsupported("SetLeverage")
}

func (a *Adapter) FetchFundingRate(context.Context, string) (*exchanges.FundingRate, error) {
	return nil, unsupported("FetchFundingRate")
}

func (a *Adapter) FetchAllFundingRates(context.Context) ([]exchanges.FundingRate, error) {
	return nil, unsupported("FetchAllFundingRates")
}

func (a *Adapter) ModifyOrder(context.Context, string, string, *exchanges.ModifyOrderParams) (*exchanges.Order, error) {
	return nil, unsupported("ModifyOrder")
}

func unsupported(method string) error {
	return exchanges.NewExchangeError("DECIBEL", "", method+" not supported", exchanges.ErrNotSupported)
}
