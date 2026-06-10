package account

import (
	"context"
	"errors"
	"testing"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

type optionAccountStubExchange struct {
	placed            *exchanges.OrderParams
	cancelledOrderID  string
	cancelledSymbol   string
	fetchOrderByIDHit bool
	watchOrdersErr    error
}

func (s *optionAccountStubExchange) GetExchange() string { return "stub-option" }

func (s *optionAccountStubExchange) GetMarketType() exchanges.MarketType {
	return exchanges.MarketTypeOption
}

func (s *optionAccountStubExchange) Close() error { return nil }

func (s *optionAccountStubExchange) FormatSymbol(symbol string) string { return symbol }

func (s *optionAccountStubExchange) ExtractSymbol(symbol string) string { return symbol }

func (s *optionAccountStubExchange) ListSymbols() []string { return nil }

func (s *optionAccountStubExchange) FetchTicker(context.Context, string) (*exchanges.Ticker, error) {
	return nil, exchanges.ErrNotSupported
}

func (s *optionAccountStubExchange) FetchOrderBook(context.Context, string, int) (*exchanges.OrderBook, error) {
	return nil, exchanges.ErrNotSupported
}

func (s *optionAccountStubExchange) FetchTrades(context.Context, string, int) ([]exchanges.Trade, error) {
	return nil, exchanges.ErrNotSupported
}

func (s *optionAccountStubExchange) FetchHistoricalTrades(context.Context, string, *exchanges.HistoricalTradeOpts) ([]exchanges.Trade, error) {
	return nil, exchanges.ErrNotSupported
}

func (s *optionAccountStubExchange) FetchKlines(context.Context, string, exchanges.Interval, *exchanges.KlineOpts) ([]exchanges.Kline, error) {
	return nil, exchanges.ErrNotSupported
}

func (s *optionAccountStubExchange) PlaceOrderWS(context.Context, *exchanges.OrderParams) error {
	return exchanges.ErrNotSupported
}

func (s *optionAccountStubExchange) CancelOrderWS(context.Context, string, string) error {
	return exchanges.ErrNotSupported
}

func (s *optionAccountStubExchange) CancelAllOrders(context.Context, string) error {
	return exchanges.ErrNotSupported
}

func (s *optionAccountStubExchange) FetchOrders(context.Context, string) ([]exchanges.Order, error) {
	return nil, exchanges.ErrNotSupported
}

func (s *optionAccountStubExchange) FetchOpenOrders(context.Context, string) ([]exchanges.Order, error) {
	return nil, nil
}

func (s *optionAccountStubExchange) FetchAccount(context.Context) (*exchanges.Account, error) {
	return &exchanges.Account{}, nil
}

func (s *optionAccountStubExchange) FetchBalance(context.Context) (decimal.Decimal, error) {
	return decimal.Zero, nil
}

func (s *optionAccountStubExchange) FetchSymbolDetails(context.Context, string) (*exchanges.SymbolDetails, error) {
	return nil, exchanges.ErrNotSupported
}

func (s *optionAccountStubExchange) FetchFeeRate(context.Context, string) (*exchanges.FeeRate, error) {
	return nil, exchanges.ErrNotSupported
}

func (s *optionAccountStubExchange) WatchOrderBook(context.Context, string, int, exchanges.OrderBookCallback) error {
	return exchanges.ErrNotSupported
}

func (s *optionAccountStubExchange) GetLocalOrderBook(string, int) *exchanges.OrderBook { return nil }

func (s *optionAccountStubExchange) StopWatchOrderBook(context.Context, string) error {
	return exchanges.ErrNotSupported
}

func (s *optionAccountStubExchange) FetchOptionChain(context.Context, string, *exchanges.OptionChainOpts) ([]exchanges.OptionInstrument, error) {
	return nil, nil
}

func (s *optionAccountStubExchange) FetchExpirations(context.Context, string) ([]time.Time, error) {
	return nil, nil
}

func (s *optionAccountStubExchange) FetchGreeks(context.Context, string) (*exchanges.Greeks, error) {
	return nil, nil
}

func (s *optionAccountStubExchange) FetchOptionMark(context.Context, string) (*exchanges.OptionMark, error) {
	return nil, nil
}

func (s *optionAccountStubExchange) FetchOptionPositions(context.Context) ([]exchanges.Position, error) {
	return nil, nil
}

func (s *optionAccountStubExchange) FormatInstrument(inst *exchanges.OptionInstrument) string {
	return "BTC-251226-100000-C"
}

func (s *optionAccountStubExchange) ParseInstrument(string) (*exchanges.OptionInstrument, error) {
	return nil, nil
}

func (s *optionAccountStubExchange) WatchOrders(context.Context, exchanges.OrderUpdateCallback) error {
	if s.watchOrdersErr != nil {
		return s.watchOrdersErr
	}
	return exchanges.ErrNotSupported
}

func (s *optionAccountStubExchange) WatchFills(context.Context, exchanges.FillCallback) error {
	return exchanges.ErrNotSupported
}

func (s *optionAccountStubExchange) WatchPositions(context.Context, exchanges.PositionUpdateCallback) error {
	return exchanges.ErrNotSupported
}

func (s *optionAccountStubExchange) WatchTicker(context.Context, string, exchanges.TickerCallback) error {
	return exchanges.ErrNotSupported
}

func (s *optionAccountStubExchange) WatchTrades(context.Context, string, exchanges.TradeCallback) error {
	return exchanges.ErrNotSupported
}

func (s *optionAccountStubExchange) WatchKlines(context.Context, string, exchanges.Interval, exchanges.KlineCallback) error {
	return exchanges.ErrNotSupported
}

func (s *optionAccountStubExchange) StopWatchOrders(context.Context) error { return nil }

func (s *optionAccountStubExchange) StopWatchFills(context.Context) error { return nil }

func (s *optionAccountStubExchange) StopWatchPositions(context.Context) error { return nil }

func (s *optionAccountStubExchange) StopWatchTicker(context.Context, string) error { return nil }

func (s *optionAccountStubExchange) StopWatchTrades(context.Context, string) error { return nil }

func (s *optionAccountStubExchange) StopWatchKlines(context.Context, string, exchanges.Interval) error {
	return nil
}

func (s *optionAccountStubExchange) PlaceOrder(_ context.Context, params *exchanges.OrderParams) (*exchanges.Order, error) {
	copy := *params
	s.placed = &copy
	return &exchanges.Order{
		OrderID:       "order-1",
		ClientOrderID: params.ClientID,
		Symbol:        params.Symbol,
		Side:          params.Side,
		Type:          params.Type,
		Quantity:      params.Quantity,
		Price:         params.Price,
		Status:        exchanges.OrderStatusNew,
	}, nil
}

func (s *optionAccountStubExchange) CancelOrder(_ context.Context, orderID, symbol string) error {
	s.cancelledOrderID = orderID
	s.cancelledSymbol = symbol
	return nil
}

func (s *optionAccountStubExchange) FetchOrderByID(context.Context, string, string) (*exchanges.Order, error) {
	s.fetchOrderByIDHit = true
	return &exchanges.Order{
		OrderID: "order-1",
		Symbol:  "BTC-251226-100000-C",
		Status:  exchanges.OrderStatusCancelled,
	}, nil
}

func TestOptionTradingAccountAllowsRESTLifecycleWithoutStreams(t *testing.T) {
	t.Parallel()

	adp := &optionAccountStubExchange{}
	acct := NewOptionTradingAccount(adp, nil)
	require.NoError(t, acct.Start(context.Background()))
	defer acct.Close()

	flow, err := acct.Place(context.Background(), &OptionOrderParams{
		Instrument: &exchanges.OptionInstrument{
			Underlying: "BTC",
			Expiry:     time.Date(2025, 12, 26, 8, 0, 0, 0, time.UTC),
			Strike:     decimal.RequireFromString("100000"),
			Kind:       exchanges.OptionCall,
			Settlement: "USDT",
		},
		Side:        exchanges.OrderSideBuy,
		Type:        exchanges.OrderTypeLimit,
		Quantity:    decimal.RequireFromString("1"),
		Price:       decimal.RequireFromString("100"),
		TimeInForce: exchanges.TimeInForceGTC,
		PostOnly:    true,
	})
	require.NoError(t, err)
	defer flow.Close()

	require.NotNil(t, adp.placed)
	require.True(t, adp.placed.PostOnly)

	require.NoError(t, acct.Cancel(context.Background(), "order-1", "BTC-251226-100000-C"))
	require.Equal(t, "order-1", adp.cancelledOrderID)
	require.True(t, adp.fetchOrderByIDHit)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	cancelled, err := flow.Wait(ctx, func(o *exchanges.Order) bool {
		return o.Status == exchanges.OrderStatusCancelled
	})
	require.NoError(t, err)
	require.Equal(t, exchanges.OrderStatusCancelled, cancelled.Status)
}

func TestOptionTradingAccountStillFailsOnUnexpectedWatchOrdersError(t *testing.T) {
	t.Parallel()

	adp := &optionAccountStubExchange{}
	adp.watchOrdersErr = errors.New("boom")
	acct := NewOptionTradingAccount(adp, nil)

	err := acct.Start(context.Background())
	require.ErrorContains(t, err, "boom")
}
