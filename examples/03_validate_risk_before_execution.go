package examples

import (
	"github.com/QuantProcessing/exchanges/cache"
	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/risk"
	"github.com/shopspring/decimal"
)

type RiskValidationResult struct {
	Accepted  model.SubmitOrder
	Rejected  model.SubmitOrder
	RejectErr error
}

// ValidateRiskBeforeExecution demonstrates the normal execution boundary:
// put instrument metadata in cache, configure limits, then call risk.Check
// before an order is allowed to reach a venue execution client.
func ValidateRiskBeforeExecution() (RiskValidationResult, error) {
	instrumentID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	c := cache.New()
	if err := c.PutInstrument(model.Instrument{
		ID:        instrumentID,
		RawSymbol: "BTCUSDT",
		Type:      model.InstrumentTypeSpot,
		Base:      "BTC",
		Quote:     "USDT",
		PriceTick: decimal.RequireFromString("0.01"),
		SizeTick:  decimal.RequireFromString("0.001"),
		Status:    model.InstrumentStatusTrading,
	}); err != nil {
		return RiskValidationResult{}, err
	}

	engine := risk.NewEngine(c, risk.Config{
		MaxOrderNotional: decimal.RequireFromString("100"),
		ExposureCurrency: "USDT",
	})
	factory := model.NewOrderFactory("risk-account", model.WithClientOrderIDPrefix("risk"))
	accepted := factory.Limit(
		instrumentID,
		model.OrderSideBuy,
		decimal.RequireFromString("0.5"),
		decimal.RequireFromString("100.00"),
	)
	rejected := factory.Limit(
		instrumentID,
		model.OrderSideBuy,
		decimal.RequireFromString("2"),
		decimal.RequireFromString("100.00"),
	)

	if err := engine.Check(accepted); err != nil {
		return RiskValidationResult{}, err
	}
	return RiskValidationResult{
		Accepted:  accepted,
		Rejected:  rejected,
		RejectErr: engine.Check(rejected),
	}, nil
}
