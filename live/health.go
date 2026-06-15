package live

import (
	"github.com/QuantProcessing/exchanges/kernel"
	"github.com/QuantProcessing/exchanges/platform"
)

type Health struct {
	State      kernel.ComponentState
	Platform   platform.Health
	Strategies []StrategyHealth
	LastError  string
}

type StrategyHealth struct {
	ID    string
	State kernel.ComponentState
}
