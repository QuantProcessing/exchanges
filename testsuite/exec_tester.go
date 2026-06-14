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
