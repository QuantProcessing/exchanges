package venue

import "github.com/QuantProcessing/exchanges/model"

type DeclaredCapabilities struct {
	Venue       model.Venue
	Instruments bool
	MarketData  MarketDataCapabilities
	Execution   ExecutionCapabilities
	Account     AccountCapabilities
}

type MarketDataCapabilities struct {
	Ticker          bool
	OrderBook       bool
	TickerStream    bool
	OrderBookStream bool
	TradeTicks      bool
	QuoteTicks      bool
	Bars            bool
	Streams         bool
}

type ExecutionCapabilities struct {
	Submit          bool
	Cancel          bool
	Modify          bool
	Query           bool
	OrderReports    bool
	FillReports     bool
	PositionReports bool
	PrivateStream   bool
}

type AccountCapabilities struct {
	Snapshot bool
}
