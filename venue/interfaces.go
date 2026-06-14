package venue

import (
	"context"
	"time"

	"github.com/QuantProcessing/exchanges/model"
)

type Adapter interface {
	Venue() model.Venue
	Instruments() InstrumentProvider
	Data() DataClient
	Execution() ExecutionClient
	Capabilities() DeclaredCapabilities
	Close(context.Context) error
}

type InstrumentProvider interface {
	LoadAll(context.Context) error
	Get(model.InstrumentID) (model.Instrument, bool)
	List() []model.Instrument
}

type DataClient interface {
	Venue() model.Venue
	ClientID() string
	Instruments() InstrumentProvider
	Connect(context.Context) error
	Disconnect(context.Context) error
	Health() DataHealth
	FetchTicker(context.Context, model.InstrumentID) (model.Ticker, error)
	FetchOrderBook(context.Context, model.InstrumentID, int) (model.OrderBook, error)
}

type StreamingDataClient interface {
	SubscribeMarketData(context.Context, model.SubscribeMarketData) error
	UnsubscribeMarketData(context.Context, model.SubscribeMarketData) error
	Events() <-chan model.MarketEvent
}

type DataHealth struct {
	Connected       bool
	InstrumentReady bool
	LastEventTime   time.Time
	LastError       error
}

type ExecutionClient interface {
	Venue() model.Venue
	AccountID() model.AccountID
	Connect(context.Context) error
	Disconnect(context.Context) error
	Health() ExecutionHealth
	QueryAccount(context.Context) (model.AccountSnapshot, error)
	SubmitOrder(context.Context, model.SubmitOrder) (model.OrderStatusReport, error)
	CancelOrder(context.Context, model.CancelOrder) (model.OrderStatusReport, error)
	GenerateOrderStatusReports(context.Context, model.InstrumentID) ([]model.OrderStatusReport, error)
	Events() <-chan model.ExecutionEvent
}

type OrderModifier interface {
	ModifyOrder(context.Context, model.ModifyOrder) (model.OrderStatusReport, error)
}

type OrderQuerier interface {
	QueryOrder(context.Context, model.QueryOrder) (model.OrderStatusReport, error)
}

type ExecutionResubscriber interface {
	ResubscribeExecution(context.Context) error
}

type FillReportGenerator interface {
	GenerateFillReports(context.Context, model.InstrumentID) ([]model.FillReport, error)
}

type PositionStatusReportGenerator interface {
	GeneratePositionStatusReports(context.Context, model.InstrumentID) ([]model.PositionStatusReport, error)
}

type ExecutionHealth struct {
	Connected     bool
	AccountReady  bool
	LastEventTime time.Time
	LastError     error
}
