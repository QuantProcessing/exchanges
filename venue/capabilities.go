package venue

import (
	"time"

	"github.com/QuantProcessing/exchanges/model"
)

type DeclaredCapabilities struct {
	Venue           model.Venue
	AccountTypes    []model.AccountType
	InstrumentTypes []model.InstrumentType
	MarketData      MarketDataCapabilities
	Execution       ExecutionCapabilities
	AccountState    AccountStateCapabilities
	Reconciliation  ReconciliationCapabilities
}

type MarketDataCapabilities struct {
	Ticker          bool
	OrderBook       bool
	Trades          bool
	Bars            bool
	Options         bool
	StreamTicker    bool
	StreamOrderBook bool
	StreamTrades    bool
	StreamBars      bool
	PrivateData     bool
}

type ExecutionCapabilities struct {
	Submit       bool
	Cancel       bool
	Modify       bool
	CancelAll    bool
	BatchOrders  bool
	OrderReports bool
	FillReports  bool
}

type AccountStateCapabilities struct {
	Snapshot  bool
	Balances  bool
	Margins   bool
	Positions bool
}

type ReconciliationCapabilities struct {
	Startup   bool
	Reconnect bool
}

type CertifiedCapabilities struct {
	Venue          model.Venue
	AccountType    model.AccountType
	InstrumentType model.InstrumentType
	Environment    string
	TestRunID      string
	Suites         []CertifiedSuite
	CertifiedAt    time.Time
}

type CertifiedSuite struct {
	Name     string
	Passed   bool
	Skipped  bool
	Reason   string
	Duration time.Duration
}

type RuntimeHealth struct {
	Connected        bool
	AccountReady     bool
	MarketStreams    map[model.InstrumentID]StreamHealth
	ExecutionStreams map[string]StreamHealth
	LastAccountState time.Time
	LastOrderEvent   time.Time
	LastFillEvent    time.Time
	LastReconcile    time.Time
	Errors           []RuntimeError
}

type StreamHealth struct {
	Ready     bool
	Stale     bool
	LastEvent time.Time
	LastError error
}

type RuntimeError struct {
	At  time.Time
	Err error
}
