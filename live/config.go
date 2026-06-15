package live

import (
	"log/slog"

	"github.com/QuantProcessing/exchanges/bus"
	"github.com/QuantProcessing/exchanges/cache"
	"github.com/QuantProcessing/exchanges/portfolio"
	"github.com/QuantProcessing/exchanges/risk"
	"github.com/QuantProcessing/exchanges/strategy"
	"github.com/QuantProcessing/exchanges/venue"
)

type NodeConfig struct {
	Bus              *bus.Bus
	Cache            *cache.Cache
	Risk             *risk.Engine
	RiskConfig       risk.Config
	Portfolio        *portfolio.Portfolio
	ReconnectPolicy  RetryPolicy
	Logger           *slog.Logger
	DataClients      []venue.DataClient
	ExecutionClients []venue.ExecutionClient
	Strategies       []strategy.Strategy
}
