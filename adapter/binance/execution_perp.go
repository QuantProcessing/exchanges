package binance

import (
	"context"
	"fmt"
	"sort"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/sdk/binance/perp"
	"github.com/QuantProcessing/exchanges/venue"
)

var _ venue.ExecutionClient = (*perpExecutionClient)(nil)

type binancePerpExecutionClient interface {
	PlaceOrder(ctx context.Context, p perp.PlaceOrderParams) (*perp.OrderResponse, error)
	CancelOrder(ctx context.Context, p perp.CancelOrderParams) (*perp.OrderResponse, error)
	CancelAllOpenOrders(ctx context.Context, p perp.CancelAllOrdersParams) error
	GetOpenOrders(ctx context.Context, symbol string) ([]perp.OrderResponse, error)
	AllOrders(ctx context.Context, symbol string, limit int, startTime, endTime int64, orderID int64) ([]perp.OrderResponse, error)
	MyTrades(ctx context.Context, symbol string, limit int, startTime, endTime int64, fromID int64) ([]perp.Trade, error)
	GetPositionRisk(ctx context.Context, symbol string) ([]perp.PositionRiskResponse, error)
	GetAccount(ctx context.Context) (*perp.AccountResponse, error)
}

type perpExecutionClient struct {
	*executionClient
	rest binancePerpExecutionClient
}

func newPerpExecutionClient(accountID model.AccountID, instruments venue.InstrumentProvider, rest binancePerpExecutionClient, stream privateExecutionStream) *perpExecutionClient {
	return &perpExecutionClient{
		executionClient: newExecutionClient(accountID, instruments, stream),
		rest:            rest,
	}
}

func (c *perpExecutionClient) SubmitOrder(ctx context.Context, cmd model.SubmitOrder) error {
	inst, raw, err := c.instrumentAndRaw(cmd.InstrumentID)
	if err != nil {
		return err
	}
	if inst.Type != model.InstrumentTypeCryptoPerp {
		return fmt.Errorf("%w: binance perp cannot submit %s", model.ErrNotSupported, inst.Type)
	}
	if c.rest == nil {
		return fmt.Errorf("%w: binance perp execution", model.ErrNotSupported)
	}
	resp, err := c.rest.PlaceOrder(ctx, perp.PlaceOrderParams{
		Symbol:           raw,
		Side:             binanceSide(cmd.Side),
		Type:             binanceOrderType(cmd.Type),
		Quantity:         cmd.Quantity.String(),
		Price:            optionalDecimalString(cmd.Price),
		NewClientOrderID: string(cmd.ClientID),
		ReduceOnly:       cmd.ReduceOnly,
	})
	if err != nil {
		return err
	}
	c.emitOrder(perpOrderReport(c.accountID, cmd.InstrumentID, resp))
	return nil
}

func (c *perpExecutionClient) ModifyOrder(context.Context, model.ModifyOrder) error {
	return model.ErrNotSupported
}

func (c *perpExecutionClient) CancelOrder(ctx context.Context, cmd model.CancelOrder) error {
	inst, raw, err := c.instrumentAndRaw(cmd.InstrumentID)
	if err != nil {
		return err
	}
	if inst.Type != model.InstrumentTypeCryptoPerp {
		return fmt.Errorf("%w: binance perp cannot cancel %s", model.ErrNotSupported, inst.Type)
	}
	if c.rest == nil {
		return fmt.Errorf("%w: binance perp execution", model.ErrNotSupported)
	}
	resp, err := c.rest.CancelOrder(ctx, perp.CancelOrderParams{
		Symbol:            raw,
		OrderID:           string(cmd.OrderID),
		OrigClientOrderID: string(cmd.ClientID),
	})
	if err != nil {
		return err
	}
	c.emitOrder(perpOrderReport(c.accountID, cmd.InstrumentID, resp))
	return nil
}

func (c *perpExecutionClient) CancelAllOrders(ctx context.Context, cmd model.CancelAllOrders) error {
	inst, raw, err := c.instrumentAndRaw(cmd.InstrumentID)
	if err != nil {
		return err
	}
	if inst.Type != model.InstrumentTypeCryptoPerp {
		return fmt.Errorf("%w: binance perp cannot cancel all %s", model.ErrNotSupported, inst.Type)
	}
	if c.rest == nil {
		return fmt.Errorf("%w: binance perp execution", model.ErrNotSupported)
	}
	return c.rest.CancelAllOpenOrders(ctx, perp.CancelAllOrdersParams{Symbol: raw})
}

func (c *perpExecutionClient) QueryAccount(ctx context.Context) error {
	if c.rest == nil {
		return fmt.Errorf("%w: binance perp execution", model.ErrNotSupported)
	}
	resp, err := c.rest.GetAccount(ctx)
	if err != nil {
		return err
	}
	state, err := perpAccountState(c.accountID, resp)
	if err != nil {
		return err
	}
	c.emitAccountState(state)
	return nil
}

func (c *perpExecutionClient) GenerateOrderStatusReports(ctx context.Context, q venue.OrderStatusQuery) ([]model.OrderStatusReport, error) {
	inst, raw, err := c.instrumentAndRaw(q.InstrumentID)
	if err != nil {
		return nil, err
	}
	if inst.Type != model.InstrumentTypeCryptoPerp {
		return nil, model.ErrNotSupported
	}
	orders, err := c.rest.AllOrders(ctx, raw, 1000, q.Since.UnixMilli(), 0, 0)
	if err != nil {
		return nil, err
	}
	out := make([]model.OrderStatusReport, 0, len(orders))
	for i := range orders {
		out = append(out, perpOrderReport(c.accountID, q.InstrumentID, &orders[i]))
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].EventTime.Before(out[j].EventTime)
	})
	return out, nil
}

func (c *perpExecutionClient) GenerateFillReports(ctx context.Context, q venue.FillQuery) ([]model.FillReport, error) {
	inst, raw, err := c.instrumentAndRaw(q.InstrumentID)
	if err != nil {
		return nil, err
	}
	if inst.Type != model.InstrumentTypeCryptoPerp {
		return nil, model.ErrNotSupported
	}
	trades, err := c.rest.MyTrades(ctx, raw, 1000, q.Since.UnixMilli(), 0, 0)
	if err != nil {
		return nil, err
	}
	out := make([]model.FillReport, 0, len(trades))
	for _, trade := range trades {
		out = append(out, perpFillReport(c.accountID, q.InstrumentID, trade))
	}
	return out, nil
}

func (c *perpExecutionClient) GeneratePositionStatusReports(ctx context.Context, q venue.PositionQuery) ([]model.PositionStatusReport, error) {
	inst, raw, err := c.instrumentAndRaw(q.InstrumentID)
	if err != nil {
		return nil, err
	}
	if inst.Type != model.InstrumentTypeCryptoPerp {
		return nil, model.ErrNotSupported
	}
	positions, err := c.rest.GetPositionRisk(ctx, raw)
	if err != nil {
		return nil, err
	}
	out := make([]model.PositionStatusReport, 0, len(positions))
	for _, position := range positions {
		report := perpPositionReportFromRisk(c.accountID, q.InstrumentID, position)
		if report.Quantity.IsZero() {
			continue
		}
		out = append(out, report)
	}
	return out, nil
}
