package testsuite

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/shopspring/decimal"
)

type ExecTesterConfig struct {
	Execution    venue.ExecutionClient
	InstrumentID model.InstrumentID
}

type ExecTester struct {
	cfg ExecTesterConfig
}

func NewExecTester(cfg ExecTesterConfig) *ExecTester {
	return &ExecTester{cfg: cfg}
}

func (e *ExecTester) Run(ctx context.Context, t *testing.T) ContractReport {
	t.Helper()
	var submitted model.OrderStatusReport
	return runContractCases(t, "execution", []contractCase{
		{id: "TC-E01", name: "Query account snapshot", run: func() error {
			if e.cfg.Execution == nil {
				return fmt.Errorf("execution client is nil")
			}
			account, err := e.cfg.Execution.QueryAccount(ctx)
			if err != nil {
				return err
			}
			if account.AccountID != e.cfg.Execution.AccountID() {
				return fmt.Errorf("account id mismatch: %s", account.AccountID)
			}
			return nil
		}},
		{id: "TC-E02", name: "Submit market order", run: func() error {
			order := model.SubmitOrder{
				AccountID:     e.cfg.Execution.AccountID(),
				InstrumentID:  e.cfg.InstrumentID,
				ClientOrderID: "tc-e02-market",
				Side:          model.OrderSideBuy,
				Type:          model.OrderTypeMarket,
				TimeInForce:   model.TimeInForceGTC,
				Quantity:      decimal.RequireFromString("0.01"),
			}
			report, err := e.cfg.Execution.SubmitOrder(ctx, order)
			if err != nil {
				return err
			}
			if err := report.Validate(); err != nil {
				return err
			}
			if report.ClientOrderID != order.ClientOrderID {
				return fmt.Errorf("client order id mismatch: %s", report.ClientOrderID)
			}
			submitted = report
			return nil
		}},
		{id: "TC-E05", name: "Query order", run: func() error {
			if submitted.OrderID == "" {
				return fmt.Errorf("submit case did not produce an order id")
			}
			querier, ok := e.cfg.Execution.(venue.OrderQuerier)
			if !ok {
				return skipCase("execution client does not implement OrderQuerier")
			}
			report, err := querier.QueryOrder(ctx, model.QueryOrder{
				AccountID:     e.cfg.Execution.AccountID(),
				InstrumentID:  e.cfg.InstrumentID,
				OrderID:       submitted.OrderID,
				ClientOrderID: submitted.ClientOrderID,
			})
			if errors.Is(err, model.ErrNotSupported) {
				return skipCase(err.Error())
			}
			if err != nil {
				return err
			}
			if report.OrderID != submitted.OrderID {
				return fmt.Errorf("query order id mismatch: %s", report.OrderID)
			}
			return report.Validate()
		}},
		{id: "TC-E04", name: "Modify order", run: func() error {
			modifier, ok := e.cfg.Execution.(venue.OrderModifier)
			if !ok {
				return skipCase("execution client does not implement OrderModifier")
			}
			order := model.SubmitOrder{
				AccountID:     e.cfg.Execution.AccountID(),
				InstrumentID:  e.cfg.InstrumentID,
				ClientOrderID: "tc-e04-limit",
				Side:          model.OrderSideBuy,
				Type:          model.OrderTypeLimit,
				TimeInForce:   model.TimeInForceGTC,
				Quantity:      decimal.RequireFromString("0.01"),
				Price:         decimal.RequireFromString("100"),
			}
			report, err := e.cfg.Execution.SubmitOrder(ctx, order)
			if err != nil {
				return err
			}
			cleanup := func(current model.OrderStatusReport) error {
				if current.OrderID == "" && current.ClientOrderID == "" {
					return nil
				}
				_, cancelErr := e.cfg.Execution.CancelOrder(ctx, model.CancelOrder{
					AccountID:     e.cfg.Execution.AccountID(),
					InstrumentID:  e.cfg.InstrumentID,
					OrderID:       current.OrderID,
					ClientOrderID: current.ClientOrderID,
				})
				return cancelErr
			}
			modified := report
			modified, err = modifier.ModifyOrder(ctx, model.ModifyOrder{
				AccountID:     e.cfg.Execution.AccountID(),
				InstrumentID:  e.cfg.InstrumentID,
				OrderID:       report.OrderID,
				ClientOrderID: report.ClientOrderID,
				Price:         decimal.RequireFromString("101"),
			})
			if errors.Is(err, model.ErrNotSupported) {
				if cleanupErr := cleanup(report); cleanupErr != nil {
					return errors.Join(skipCase(err.Error()), cleanupErr)
				}
				return skipCase(err.Error())
			}
			if err != nil {
				return errors.Join(err, cleanup(report))
			}
			cleanupModified := func() error {
				target := modified
				if target.OrderID == "" {
					target.OrderID = report.OrderID
				}
				if target.ClientOrderID == "" {
					target.ClientOrderID = report.ClientOrderID
				}
				return cleanup(target)
			}
			if !modified.Price.Equal(decimal.RequireFromString("101")) {
				return errors.Join(fmt.Errorf("modify price mismatch: %s", modified.Price), cleanupModified())
			}
			return errors.Join(modified.Validate(), cleanupModified())
		}},
		{id: "TC-E03", name: "Cancel order", run: func() error {
			if submitted.OrderID == "" {
				return fmt.Errorf("submit case did not produce an order id")
			}
			report, err := e.cfg.Execution.CancelOrder(ctx, model.CancelOrder{
				AccountID:    e.cfg.Execution.AccountID(),
				InstrumentID: e.cfg.InstrumentID,
				OrderID:      submitted.OrderID,
			})
			if err != nil {
				return err
			}
			if report.Status != model.OrderStatusCanceled {
				return fmt.Errorf("cancel status mismatch: %s", report.Status)
			}
			return report.Validate()
		}},
		{id: "TC-E80", name: "Generate order status reports", run: func() error {
			reports, err := e.cfg.Execution.GenerateOrderStatusReports(ctx, e.cfg.InstrumentID)
			if err != nil {
				return err
			}
			for _, report := range reports {
				if err := report.Validate(); err != nil {
					return err
				}
			}
			return nil
		}},
		{id: "TC-E81", name: "Generate fill reports", run: func() error {
			generator, ok := e.cfg.Execution.(venue.FillReportGenerator)
			if !ok {
				return skipCase("execution client does not implement FillReportGenerator")
			}
			reports, err := generator.GenerateFillReports(ctx, e.cfg.InstrumentID)
			if errors.Is(err, model.ErrNotSupported) {
				return skipCase(err.Error())
			}
			if err != nil {
				return err
			}
			for _, report := range reports {
				if err := report.Validate(); err != nil {
					return err
				}
			}
			return nil
		}},
		{id: "TC-E82", name: "Generate position status reports", run: func() error {
			generator, ok := e.cfg.Execution.(venue.PositionStatusReportGenerator)
			if !ok {
				return skipCase("execution client does not implement PositionStatusReportGenerator")
			}
			reports, err := generator.GeneratePositionStatusReports(ctx, e.cfg.InstrumentID)
			if errors.Is(err, model.ErrNotSupported) {
				return skipCase(err.Error())
			}
			if err != nil {
				return err
			}
			for _, report := range reports {
				if err := report.Validate(); err != nil {
					return err
				}
			}
			return nil
		}},
		{id: "TC-E84", name: "Resubscribe private stream", run: func() error {
			resubscriber, ok := e.cfg.Execution.(venue.ExecutionResubscriber)
			if !ok {
				return skipCase("execution client does not implement ExecutionResubscriber")
			}
			err := resubscriber.ResubscribeExecution(ctx)
			if errors.Is(err, model.ErrNotSupported) {
				return skipCase(err.Error())
			}
			return err
		}},
	})
}
