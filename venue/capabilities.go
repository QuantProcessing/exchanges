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
	Snapshots         bool
	Ticker            bool
	OrderBook         bool
	TickerStream      bool
	OrderBookStream   bool
	TradeTicks        bool
	QuoteTicks        bool
	Bars              bool
	FundingRates      bool
	FundingRateStream bool
	Streams           bool
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
	Resubscribe     bool
	MassStatus      bool
	OrderLists      bool
}

type AccountCapabilities struct {
	Snapshot bool
}
