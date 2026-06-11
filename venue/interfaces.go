package venue

import (
	"context"
	"time"

	"github.com/QuantProcessing/exchanges/model"
)

type Adapter interface {
	Venue() model.Venue
	Instruments() InstrumentProvider
	MarketData() MarketDataClient
	Execution() ExecutionClient
	Capabilities() DeclaredCapabilities
	Close() error
}

type InstrumentProvider interface {
	LoadAll(ctx context.Context) error
	Load(ctx context.Context, id model.InstrumentID) (model.Instrument, error)
	Find(ctx context.Context, q InstrumentQuery) ([]model.Instrument, error)
	Get(id model.InstrumentID) (model.Instrument, bool)
	List() []model.Instrument
}

type InstrumentQuery struct {
	Venue      model.Venue
	Type       model.InstrumentType
	Base       model.Currency
	Quote      model.Currency
	Settle     model.Currency
	Underlying *model.InstrumentID
	ExpiresGTE time.Time
	ExpiresLTE time.Time
	OptionKind model.OptionKind
}

type ProductHint string

const (
	ProductHintSpot   ProductHint = "spot"
	ProductHintPerp   ProductHint = "perp"
	ProductHintOption ProductHint = "option"
)

type SymbolNormalizer interface {
	ToInstrumentID(raw string, hint ProductHint) (model.InstrumentID, error)
	ToVenueSymbol(id model.InstrumentID) (string, error)
}

type TradeQuery struct {
	Limit int
	Since time.Time
}

type BarQuery struct {
	Limit int
	Since time.Time
	Until time.Time
}

type ChainQuery struct {
	Limit int
}

type OrderStatusQuery struct {
	InstrumentID model.InstrumentID
	Since        time.Time
}

type FillQuery struct {
	InstrumentID model.InstrumentID
	Since        time.Time
}

type PositionQuery struct {
	InstrumentID model.InstrumentID
}

type TickerHandler func(model.Ticker)
type OrderBookHandler func(model.OrderBook)
type TradeHandler func(model.Trade)
type BarHandler func(model.Bar)
type OptionGreeksHandler func(model.OptionGreeks)
type OptionChainHandler func(model.OptionChainSlice)

type MarketDataClient interface {
	FetchTicker(ctx context.Context, id model.InstrumentID) (model.Ticker, error)
	FetchOrderBook(ctx context.Context, id model.InstrumentID, limit int) (model.OrderBook, error)
	FetchTrades(ctx context.Context, id model.InstrumentID, q TradeQuery) ([]model.Trade, error)
	FetchBars(ctx context.Context, id model.InstrumentID, spec model.BarSpec, q BarQuery) ([]model.Bar, error)

	SubscribeTicker(ctx context.Context, id model.InstrumentID, h TickerHandler) (Subscription, error)
	SubscribeOrderBook(ctx context.Context, id model.InstrumentID, depth int, h OrderBookHandler) (Subscription, error)
	SubscribeTrades(ctx context.Context, id model.InstrumentID, h TradeHandler) (Subscription, error)
	SubscribeBars(ctx context.Context, id model.InstrumentID, spec model.BarSpec, h BarHandler) (Subscription, error)
}

type OptionMarketDataClient interface {
	FetchOptionChain(ctx context.Context, series model.OptionSeriesID, q ChainQuery) ([]model.Instrument, error)
	SubscribeOptionGreeks(ctx context.Context, id model.InstrumentID, h OptionGreeksHandler) (Subscription, error)
	SubscribeOptionChain(ctx context.Context, req OptionChainSubscription, h OptionChainHandler) (Subscription, error)
}

type OptionChainSubscription struct {
	Series model.OptionSeriesID
}

type ExecutionClient interface {
	AccountID() model.AccountID
	Venue() model.Venue
	Connect(ctx context.Context) error
	Disconnect(ctx context.Context) error
	Health() ExecutionHealth
	SubmitOrder(ctx context.Context, cmd model.SubmitOrder) error
	ModifyOrder(ctx context.Context, cmd model.ModifyOrder) error
	CancelOrder(ctx context.Context, cmd model.CancelOrder) error
	CancelAllOrders(ctx context.Context, cmd model.CancelAllOrders) error
	QueryAccount(ctx context.Context) error
	GenerateOrderStatusReports(ctx context.Context, q OrderStatusQuery) ([]model.OrderStatusReport, error)
	GenerateFillReports(ctx context.Context, q FillQuery) ([]model.FillReport, error)
	GeneratePositionStatusReports(ctx context.Context, q PositionQuery) ([]model.PositionStatusReport, error)
	Events() <-chan model.ExecutionEvent
}

type ExecutionHealth struct {
	Connected     bool
	AccountReady  bool
	LastEventTime time.Time
	LastError     error
}
