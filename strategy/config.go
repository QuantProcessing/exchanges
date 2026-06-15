package strategy

import (
	"fmt"

	"github.com/QuantProcessing/exchanges/model"
)

type StrategyConfig struct {
	ID model.StrategyID
}

func (c StrategyConfig) Validate() error {
	if c.ID == "" {
		return fmt.Errorf("%w: strategy id is required", ErrInvalidStrategyConfig)
	}
	return nil
}

var ErrInvalidStrategyConfig = fmt.Errorf("invalid strategy config")
