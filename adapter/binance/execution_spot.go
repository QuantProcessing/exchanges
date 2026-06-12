package binance

import (
	"context"
	"fmt"
	"sort"
	"strconv"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/sdk/binance/spot"
	"github.com/QuantProcessing/exchanges/venue"
)

var _ venue.ExecutionClient = (*spotExecutionClient)(nil)

type binanceSpotExecutionClient interface {
	PlaceOrder(ctx context.Context, p spot.PlaceOrderParams) (*spot.OrderResponse, error)
	CancelOrder(ctx context.Context, symbol string, orderID int64, origClientOrderID string) (*spot.CancelOrderResponse, error)
	GetOpenOrders(ctx context.Context, symbol string) ([]spot.OrderResponse, error)
	AllOrders(ctx context.Context, symbol string, limit int, startTime, endTime int64, orderID int64) ([]spot.OrderResponse, error)
	MyTrades(ctx context.Context, symbol string, limit int, startTime, endTime int64, fromID int64) ([]spot.Trade, error)
	GetAccount(ctx context.Context) (*spot.AccountResponse, error)
}

type spotExecutionClient struct {
	*executionClient
	rest binanceSpotExecutionClient
}

func newSpotExecutionClient(accountID model.AccountID, instruments venue.InstrumentProvider, rest binanceSpotExecutionClient, stream privateExecutionStream) *spotExecutionClient {
	return &spotExecutionClient{
		executionClient: newExecutionClient(accountID, instruments, stream),
		rest:            rest,
	}
}

func (c *spotExecutionClient) SubmitOrder(ctx context.Context, cmd model.SubmitOrder) error {
	inst, raw, err := c.instrumentAndRaw(cmd.InstrumentID)
	if err != nil {
		return err
	}
	if inst.Type != model.InstrumentTypeCurrencyPair {
		return fmt.Errorf("%w: binance spot cannot submit %s", model.ErrNotSupported, inst.Type)
	}
	if c.rest == nil {
		return fmt.Errorf("%w: binance spot execution", model.ErrNotSupported)
	}
	resp, err := c.rest.PlaceOrder(ctx, spot.PlaceOrderParams{
		Symbol:           raw,
		Side:             binanceSide(cmd.Side),
		Type:             binanceOrderType(cmd.Type),
		Quantity:         cmd.Quantity.String(),
		Price:            optionalDecimalString(cmd.Price),
		NewClientOrderID: string(cmd.ClientID),
	})
	if err != nil {
		return err
	}
	c.emitOrder(spotOrderReport(c.accountID, cmd.InstrumentID, resp))
	return nil
}

func (c *spotExecutionClient) ModifyOrder(context.Context, model.ModifyOrder) error {
	return model.ErrNotSupported
}

func (c *spotExecutionClient) CancelOrder(ctx context.Context, cmd model.CancelOrder) error {
	inst, raw, err := c.instrumentAndRaw(cmd.InstrumentID)
	if err != nil {
		return err
	}
	if inst.Type != model.InstrumentTypeCurrencyPair {
		return fmt.Errorf("%w: binance spot cannot cancel %s", model.ErrNotSupported, inst.Type)
	}
	if c.rest == nil {
		return fmt.Errorf("%w: binance spot execution", model.ErrNotSupported)
	}
	orderID, _ := strconv.ParseInt(string(cmd.OrderID), 10, 64)
	resp, err := c.rest.CancelOrder(ctx, raw, orderID, string(cmd.ClientID))
	if err != nil {
		return err
	}
	if resp != nil {
		c.emitOrder(spotCancelReport(c.accountID, cmd.InstrumentID, resp))
	}
	return nil
}

func (c *spotExecutionClient) CancelAllOrders(context.Context, model.CancelAllOrders) error {
	return model.ErrNotSupported
}

func (c *spotExecutionClient) QueryAccount(ctx context.Context) error {
	if c.rest == nil {
		return fmt.Errorf("%w: binance spot execution", model.ErrNotSupported)
	}
	resp, err := c.rest.GetAccount(ctx)
	if err != nil {
		return err
	}
	state, err := spotAccountState(c.accountID, resp)
	if err != nil {
		return err
	}
	c.emitAccountState(state)
	return nil
}

func (c *spotExecutionClient) GenerateOrderStatusReports(ctx context.Context, q venue.OrderStatusQuery) ([]model.OrderStatusReport, error) {
	inst, raw, err := c.instrumentAndRaw(q.InstrumentID)
	if err != nil {
		return nil, err
	}
	if inst.Type != model.InstrumentTypeCurrencyPair {
		return nil, model.ErrNotSupported
	}
	orders, err := c.rest.AllOrders(ctx, raw, 1000, q.Since.UnixMilli(), 0, 0)
	if err != nil {
		return nil, err
	}
	out := make([]model.OrderStatusReport, 0, len(orders))
	for i := range orders {
		out = append(out, spotOrderReport(c.accountID, q.InstrumentID, &orders[i]))
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].EventTime.Before(out[j].EventTime)
	})
	return out, nil
}

func (c *spotExecutionClient) GenerateFillReports(ctx context.Context, q venue.FillQuery) ([]model.FillReport, error) {
	inst, raw, err := c.instrumentAndRaw(q.InstrumentID)
	if err != nil {
		return nil, err
	}
	if inst.Type != model.InstrumentTypeCurrencyPair {
		return nil, model.ErrNotSupported
	}
	trades, err := c.rest.MyTrades(ctx, raw, 1000, q.Since.UnixMilli(), 0, 0)
	if err != nil {
		return nil, err
	}
	out := make([]model.FillReport, 0, len(trades))
	for _, trade := range trades {
		out = append(out, spotFillReport(c.accountID, q.InstrumentID, trade))
	}
	return out, nil
}

func (c *spotExecutionClient) GeneratePositionStatusReports(context.Context, venue.PositionQuery) ([]model.PositionStatusReport, error) {
	return nil, model.ErrNotSupported
}
