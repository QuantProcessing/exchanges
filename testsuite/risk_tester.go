package testsuite

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/risk"
	"github.com/shopspring/decimal"
)

type RiskTesterConfig struct {
	Engine                   *risk.Engine
	OrderNotionalOnlyEngine  *risk.Engine
	MarginEngine             *risk.Engine
	OpenOrderEngine          *risk.Engine
	OpenOrderMarginEngine    *risk.Engine
	BaseCurrencyEngine       *risk.Engine
	AccountID                model.AccountID
	InstrumentID             model.InstrumentID
	QuoteOnlyInstrumentID    model.InstrumentID
	UnpricedInstrumentID     model.InstrumentID
	MarginInstrumentID       model.InstrumentID
	OpenOrderInstrumentID    model.InstrumentID
	BaseCurrencyInstrumentID model.InstrumentID
}

type RiskTester struct {
	cfg RiskTesterConfig
}

func NewRiskTester(cfg RiskTesterConfig) *RiskTester {
	return &RiskTester{cfg: cfg}
}

func (r *RiskTester) Run(ctx context.Context, t *testing.T) ContractReport {
	t.Helper()
	_ = ctx
	return runContractCases(t, "risk", []contractCase{
		{id: "TC-R01", name: "Precision rejection", run: func() error {
			err := r.cfg.Engine.Check(model.SubmitOrder{
				AccountID:     r.cfg.AccountID,
				InstrumentID:  r.cfg.InstrumentID,
				ClientOrderID: "tc-r01",
				Side:          model.OrderSideBuy,
				Type:          model.OrderTypeLimit,
				TimeInForce:   model.TimeInForceGTC,
				Quantity:      decimal.RequireFromString("1"),
				Price:         decimal.RequireFromString("100.001"),
			})
			return requireError(err, model.ErrInvalidOrder)
		}},
		{id: "TC-R02", name: "Market notional rejection", run: func() error {
			err := r.cfg.Engine.Check(model.SubmitOrder{
				AccountID:     r.cfg.AccountID,
				InstrumentID:  r.cfg.InstrumentID,
				ClientOrderID: "tc-r02",
				Side:          model.OrderSideBuy,
				Type:          model.OrderTypeMarket,
				TimeInForce:   model.TimeInForceIOC,
				Quantity:      decimal.RequireFromString("10"),
			})
			return requireError(err, risk.ErrRiskRejected)
		}},
		{id: "TC-R03", name: "Reduce-only exposure rejection", run: func() error {
			err := r.cfg.Engine.Check(model.SubmitOrder{
				AccountID:     r.cfg.AccountID,
				InstrumentID:  r.cfg.InstrumentID,
				ClientOrderID: "tc-r03",
				Side:          model.OrderSideBuy,
				Type:          model.OrderTypeMarket,
				TimeInForce:   model.TimeInForceIOC,
				Quantity:      decimal.RequireFromString("0.1"),
				ReduceOnly:    true,
			})
			return requireError(err, risk.ErrRiskRejected)
		}},
		{id: "TC-R04", name: "Invalid time-in-force rejection", run: func() error {
			err := r.cfg.Engine.Check(model.SubmitOrder{
				AccountID:     r.cfg.AccountID,
				InstrumentID:  r.cfg.InstrumentID,
				ClientOrderID: "tc-r04",
				Side:          model.OrderSideBuy,
				Type:          model.OrderTypeLimit,
				TimeInForce:   model.TimeInForce("foreverish"),
				Quantity:      decimal.RequireFromString("1"),
				Price:         decimal.RequireFromString("100"),
			})
			return requireError(err, model.ErrInvalidOrder)
		}},
		{id: "TC-R05", name: "Projected position notional rejection", run: func() error {
			err := r.cfg.Engine.Check(model.SubmitOrder{
				AccountID:     r.cfg.AccountID,
				InstrumentID:  r.cfg.InstrumentID,
				ClientOrderID: "tc-r05",
				Side:          model.OrderSideBuy,
				Type:          model.OrderTypeLimit,
				TimeInForce:   model.TimeInForceGTC,
				Quantity:      decimal.RequireFromString("2"),
				Price:         decimal.RequireFromString("100"),
			})
			return requireError(err, risk.ErrRiskRejected)
		}},
		{id: "TC-R06", name: "Projected account exposure rejection", run: func() error {
			err := r.cfg.Engine.Check(model.SubmitOrder{
				AccountID:     r.cfg.AccountID,
				InstrumentID:  r.cfg.InstrumentID,
				ClientOrderID: "tc-r06",
				Side:          model.OrderSideBuy,
				Type:          model.OrderTypeLimit,
				TimeInForce:   model.TimeInForceGTC,
				Quantity:      decimal.RequireFromString("1"),
				Price:         decimal.RequireFromString("100"),
			})
			return requireError(err, risk.ErrRiskRejected)
		}},
		{id: "TC-R07", name: "Reduce-only flip rejection", run: func() error {
			err := r.cfg.Engine.Check(model.SubmitOrder{
				AccountID:     r.cfg.AccountID,
				InstrumentID:  r.cfg.InstrumentID,
				ClientOrderID: "tc-r07",
				Side:          model.OrderSideSell,
				Type:          model.OrderTypeMarket,
				TimeInForce:   model.TimeInForceIOC,
				Quantity:      decimal.RequireFromString("2"),
				ReduceOnly:    true,
			})
			return requireError(err, risk.ErrRiskRejected)
		}},
		{id: "TC-R08", name: "Quote tick market notional rejection", run: func() error {
			instrumentID := r.cfg.QuoteOnlyInstrumentID
			if instrumentID.String() == "" {
				instrumentID = r.cfg.InstrumentID
			}
			err := r.cfg.Engine.Check(model.SubmitOrder{
				AccountID:     r.cfg.AccountID,
				InstrumentID:  instrumentID,
				ClientOrderID: "tc-r08",
				Side:          model.OrderSideBuy,
				Type:          model.OrderTypeMarket,
				TimeInForce:   model.TimeInForceIOC,
				Quantity:      decimal.RequireFromString("10"),
			})
			return requireError(err, risk.ErrRiskRejected)
		}},
		{id: "TC-R09", name: "Missing market price notional rejection", run: func() error {
			instrumentID := r.cfg.UnpricedInstrumentID
			if instrumentID.String() == "" {
				instrumentID = r.cfg.InstrumentID
			}
			engine := r.cfg.OrderNotionalOnlyEngine
			if engine == nil {
				engine = r.cfg.Engine
			}
			err := engine.Check(model.SubmitOrder{
				AccountID:     r.cfg.AccountID,
				InstrumentID:  instrumentID,
				ClientOrderID: "tc-r09",
				Side:          model.OrderSideBuy,
				Type:          model.OrderTypeMarket,
				TimeInForce:   model.TimeInForceIOC,
				Quantity:      decimal.RequireFromString("1"),
			})
			return requireError(err, risk.ErrRiskRejected)
		}},
		{id: "TC-R10", name: "Available initial margin rejection", run: func() error {
			engine := r.cfg.MarginEngine
			if engine == nil {
				return fmt.Errorf("margin engine is required")
			}
			instrumentID := r.cfg.MarginInstrumentID
			if instrumentID.String() == "" {
				instrumentID = r.cfg.InstrumentID
			}
			err := engine.Check(model.SubmitOrder{
				AccountID:     r.cfg.AccountID,
				InstrumentID:  instrumentID,
				ClientOrderID: "tc-r10",
				Side:          model.OrderSideBuy,
				Type:          model.OrderTypeLimit,
				TimeInForce:   model.TimeInForceGTC,
				Quantity:      decimal.RequireFromString("1"),
				Price:         decimal.RequireFromString("1000"),
			})
			return requireError(err, risk.ErrRiskRejected)
		}},
		{id: "TC-R11", name: "Open order projected position rejection", run: func() error {
			engine := r.cfg.OpenOrderEngine
			if engine == nil {
				return fmt.Errorf("open order engine is required")
			}
			instrumentID := r.cfg.OpenOrderInstrumentID
			if instrumentID.String() == "" {
				instrumentID = r.cfg.InstrumentID
			}
			err := engine.Check(model.SubmitOrder{
				AccountID:     r.cfg.AccountID,
				InstrumentID:  instrumentID,
				ClientOrderID: "tc-r11",
				Side:          model.OrderSideBuy,
				Type:          model.OrderTypeLimit,
				TimeInForce:   model.TimeInForceGTC,
				Quantity:      decimal.RequireFromString("2"),
				Price:         decimal.RequireFromString("100"),
			})
			return requireError(err, risk.ErrRiskRejected)
		}},
		{id: "TC-R12", name: "Open order initial margin rejection", run: func() error {
			engine := r.cfg.OpenOrderMarginEngine
			if engine == nil {
				return fmt.Errorf("open order margin engine is required")
			}
			instrumentID := r.cfg.OpenOrderInstrumentID
			if instrumentID.String() == "" {
				instrumentID = r.cfg.InstrumentID
			}
			err := engine.Check(model.SubmitOrder{
				AccountID:     r.cfg.AccountID,
				InstrumentID:  instrumentID,
				ClientOrderID: "tc-r12",
				Side:          model.OrderSideBuy,
				Type:          model.OrderTypeLimit,
				TimeInForce:   model.TimeInForceGTC,
				Quantity:      decimal.RequireFromString("2"),
				Price:         decimal.RequireFromString("100"),
			})
			return requireError(err, risk.ErrRiskRejected)
		}},
		{id: "TC-R13", name: "Base currency account exposure rejection", run: func() error {
			engine := r.cfg.BaseCurrencyEngine
			if engine == nil {
				return fmt.Errorf("base currency engine is required")
			}
			instrumentID := r.cfg.BaseCurrencyInstrumentID
			if instrumentID.String() == "" {
				return fmt.Errorf("base currency instrument is required")
			}
			err := engine.Check(model.SubmitOrder{
				AccountID:     r.cfg.AccountID,
				InstrumentID:  instrumentID,
				ClientOrderID: "tc-r13",
				Side:          model.OrderSideBuy,
				Type:          model.OrderTypeLimit,
				TimeInForce:   model.TimeInForceGTC,
				Quantity:      decimal.RequireFromString("1"),
				Price:         decimal.RequireFromString("10"),
			})
			return requireError(err, risk.ErrRiskRejected)
		}},
	})
}

func requireError(got error, want error) error {
	if got == nil {
		return fmt.Errorf("expected %v, got nil", want)
	}
	if !errors.Is(got, want) {
		return fmt.Errorf("expected %v, got %v", want, got)
	}
	return nil
}
