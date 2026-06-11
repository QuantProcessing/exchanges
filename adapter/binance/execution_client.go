package binance

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/sdk/binance/perp"
	"github.com/QuantProcessing/exchanges/sdk/binance/spot"
	"github.com/QuantProcessing/exchanges/venue"
)

var _ venue.ExecutionClient = (*executionClient)(nil)

type binanceSpotExecutionClient interface {
	PlaceOrder(ctx context.Context, p spot.PlaceOrderParams) (*spot.OrderResponse, error)
	CancelOrder(ctx context.Context, symbol string, orderID int64, origClientOrderID string) (*spot.CancelOrderResponse, error)
	GetOpenOrders(ctx context.Context, symbol string) ([]spot.OrderResponse, error)
	MyTrades(ctx context.Context, symbol string, limit int, startTime, endTime int64, fromID int64) ([]spot.Trade, error)
	GetAccount(ctx context.Context) (*spot.AccountResponse, error)
}

type binancePerpExecutionClient interface {
	PlaceOrder(ctx context.Context, p perp.PlaceOrderParams) (*perp.OrderResponse, error)
	CancelOrder(ctx context.Context, p perp.CancelOrderParams) (*perp.OrderResponse, error)
	CancelAllOpenOrders(ctx context.Context, p perp.CancelAllOrdersParams) error
	GetOpenOrders(ctx context.Context, symbol string) ([]perp.OrderResponse, error)
	MyTrades(ctx context.Context, symbol string, limit int, startTime, endTime int64, fromID int64) ([]perp.Trade, error)
	GetAccount(ctx context.Context) (*perp.AccountResponse, error)
}

type executionClient struct {
	accountID   model.AccountID
	instruments venue.InstrumentProvider
	normalizer  symbolNormalizer
	spot        binanceSpotExecutionClient
	perp        binancePerpExecutionClient
	events      chan model.ExecutionEvent
	health      venue.ExecutionHealth
}

func newExecutionClient(accountID model.AccountID, instruments venue.InstrumentProvider, spotClient binanceSpotExecutionClient, perpClient binancePerpExecutionClient) *executionClient {
	return &executionClient{
		accountID:   accountID,
		instruments: instruments,
		spot:        spotClient,
		perp:        perpClient,
		events:      make(chan model.ExecutionEvent, 128),
	}
}

func (c *executionClient) AccountID() model.AccountID { return c.accountID }

func (c *executionClient) Venue() model.Venue { return model.VenueBinance }

func (c *executionClient) Connect(context.Context) error {
	c.health.Connected = true
	return nil
}

func (c *executionClient) Disconnect(context.Context) error {
	c.health.Connected = false
	return nil
}

func (c *executionClient) Health() venue.ExecutionHealth { return c.health }

func (c *executionClient) SubmitOrder(ctx context.Context, cmd model.SubmitOrder) error {
	inst, raw, err := c.instrumentAndRaw(cmd.InstrumentID)
	if err != nil {
		return err
	}
	switch inst.Type {
	case model.InstrumentTypeCurrencyPair:
		if c.spot == nil {
			return fmt.Errorf("%w: binance spot execution", model.ErrNotSupported)
		}
		resp, err := c.spot.PlaceOrder(ctx, spot.PlaceOrderParams{
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
	case model.InstrumentTypeCryptoPerp:
		if c.perp == nil {
			return fmt.Errorf("%w: binance perp execution", model.ErrNotSupported)
		}
		resp, err := c.perp.PlaceOrder(ctx, perp.PlaceOrderParams{
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
	default:
		return fmt.Errorf("%w: unsupported instrument type %s", model.ErrNotSupported, inst.Type)
	}
}

func (c *executionClient) ModifyOrder(context.Context, model.ModifyOrder) error {
	return model.ErrNotSupported
}

func (c *executionClient) CancelOrder(ctx context.Context, cmd model.CancelOrder) error {
	inst, raw, err := c.instrumentAndRaw(cmd.InstrumentID)
	if err != nil {
		return err
	}
	switch inst.Type {
	case model.InstrumentTypeCurrencyPair:
		if c.spot == nil {
			return fmt.Errorf("%w: binance spot execution", model.ErrNotSupported)
		}
		orderID, _ := strconv.ParseInt(string(cmd.OrderID), 10, 64)
		resp, err := c.spot.CancelOrder(ctx, raw, orderID, string(cmd.ClientID))
		if err != nil {
			return err
		}
		if resp != nil {
			c.emitOrder(spotCancelReport(c.accountID, cmd.InstrumentID, resp))
		}
		return nil
	case model.InstrumentTypeCryptoPerp:
		if c.perp == nil {
			return fmt.Errorf("%w: binance perp execution", model.ErrNotSupported)
		}
		resp, err := c.perp.CancelOrder(ctx, perp.CancelOrderParams{
			Symbol:            raw,
			OrderID:           string(cmd.OrderID),
			OrigClientOrderID: string(cmd.ClientID),
		})
		if err != nil {
			return err
		}
		c.emitOrder(perpOrderReport(c.accountID, cmd.InstrumentID, resp))
		return nil
	default:
		return fmt.Errorf("%w: unsupported instrument type %s", model.ErrNotSupported, inst.Type)
	}
}

func (c *executionClient) CancelAllOrders(ctx context.Context, cmd model.CancelAllOrders) error {
	inst, raw, err := c.instrumentAndRaw(cmd.InstrumentID)
	if err != nil {
		return err
	}
	if inst.Type != model.InstrumentTypeCryptoPerp {
		return model.ErrNotSupported
	}
	if c.perp == nil {
		return fmt.Errorf("%w: binance perp execution", model.ErrNotSupported)
	}
	return c.perp.CancelAllOpenOrders(ctx, perp.CancelAllOrdersParams{Symbol: raw})
}

func (c *executionClient) QueryAccount(ctx context.Context) error {
	if c.spot != nil {
		resp, err := c.spot.GetAccount(ctx)
		if err != nil {
			return err
		}
		state, err := spotAccountState(c.accountID, resp)
		if err != nil {
			return err
		}
		c.emitAccountState(state)
	}
	if c.perp != nil {
		resp, err := c.perp.GetAccount(ctx)
		if err != nil {
			return err
		}
		state, err := perpAccountState(c.accountID, resp)
		if err != nil {
			return err
		}
		c.emitAccountState(state)
	}
	return nil
}

func (c *executionClient) GenerateOrderStatusReports(ctx context.Context, q venue.OrderStatusQuery) ([]model.OrderStatusReport, error) {
	inst, raw, err := c.instrumentAndRaw(q.InstrumentID)
	if err != nil {
		return nil, err
	}
	switch inst.Type {
	case model.InstrumentTypeCurrencyPair:
		orders, err := c.spot.GetOpenOrders(ctx, raw)
		if err != nil {
			return nil, err
		}
		out := make([]model.OrderStatusReport, 0, len(orders))
		for i := range orders {
			out = append(out, spotOrderReport(c.accountID, q.InstrumentID, &orders[i]))
		}
		return out, nil
	case model.InstrumentTypeCryptoPerp:
		orders, err := c.perp.GetOpenOrders(ctx, raw)
		if err != nil {
			return nil, err
		}
		out := make([]model.OrderStatusReport, 0, len(orders))
		for i := range orders {
			out = append(out, perpOrderReport(c.accountID, q.InstrumentID, &orders[i]))
		}
		return out, nil
	default:
		return nil, model.ErrNotSupported
	}
}

func (c *executionClient) GenerateFillReports(ctx context.Context, q venue.FillQuery) ([]model.FillReport, error) {
	inst, raw, err := c.instrumentAndRaw(q.InstrumentID)
	if err != nil {
		return nil, err
	}
	switch inst.Type {
	case model.InstrumentTypeCurrencyPair:
		trades, err := c.spot.MyTrades(ctx, raw, 1000, q.Since.UnixMilli(), 0, 0)
		if err != nil {
			return nil, err
		}
		out := make([]model.FillReport, 0, len(trades))
		for _, trade := range trades {
			out = append(out, spotFillReport(c.accountID, q.InstrumentID, trade))
		}
		return out, nil
	case model.InstrumentTypeCryptoPerp:
		trades, err := c.perp.MyTrades(ctx, raw, 1000, q.Since.UnixMilli(), 0, 0)
		if err != nil {
			return nil, err
		}
		out := make([]model.FillReport, 0, len(trades))
		for _, trade := range trades {
			out = append(out, perpFillReport(c.accountID, q.InstrumentID, trade))
		}
		return out, nil
	default:
		return nil, model.ErrNotSupported
	}
}

func (c *executionClient) GeneratePositionStatusReports(context.Context, venue.PositionQuery) ([]model.PositionStatusReport, error) {
	return nil, model.ErrNotSupported
}

func (c *executionClient) Events() <-chan model.ExecutionEvent { return c.events }

func (c *executionClient) instrumentAndRaw(id model.InstrumentID) (model.Instrument, string, error) {
	if c.instruments == nil {
		return model.Instrument{}, "", fmt.Errorf("%w: %s", model.ErrInstrumentNotLoaded, id.String())
	}
	inst, ok := c.instruments.Get(id)
	if !ok {
		return model.Instrument{}, "", fmt.Errorf("%w: %s", model.ErrInstrumentNotLoaded, id.String())
	}
	raw, err := c.normalizer.ToVenueSymbol(id)
	if err != nil {
		return model.Instrument{}, "", err
	}
	return inst, raw, nil
}

func (c *executionClient) emitOrder(report model.OrderStatusReport) {
	c.health.LastEventTime = report.EventTime
	select {
	case c.events <- model.ExecutionEvent{Order: &report}:
	default:
	}
}

func (c *executionClient) emitAccountState(state model.AccountState) {
	c.health.AccountReady = true
	c.health.LastEventTime = state.EventTime
	select {
	case c.events <- model.ExecutionEvent{AccountState: &state}:
	default:
	}
}

func binanceSide(side model.OrderSide) string {
	if side == model.OrderSideSell {
		return "SELL"
	}
	return "BUY"
}

func binanceOrderType(t model.OrderType) string {
	if t == model.OrderTypeLimit {
		return "LIMIT"
	}
	return "MARKET"
}

func optionalDecimalString(d fmt.Stringer) string {
	s := d.String()
	if s == "0" {
		return ""
	}
	return s
}

func spotOrderReport(accountID model.AccountID, id model.InstrumentID, resp *spot.OrderResponse) model.OrderStatusReport {
	if resp == nil {
		return model.OrderStatusReport{AccountID: accountID, InstrumentID: id}
	}
	return model.OrderStatusReport{
		AccountID:    accountID,
		InstrumentID: id,
		OrderID:      model.OrderID(strconv.FormatInt(resp.OrderID, 10)),
		ClientID:     model.ClientOrderID(resp.ClientOrderID),
		Status:       orderStatusFromBinance(resp.Status),
		Side:         orderSideFromBinance(resp.Side),
		Type:         orderTypeFromBinance(resp.Type),
		Quantity:     parseDecimal(resp.OrigQty),
		FilledQty:    parseDecimal(resp.ExecutedQty),
		AvgPrice:     parseDecimal(resp.Price),
		EventTime:    timeFromUnixMilli(resp.TransactTime),
	}
}

func spotCancelReport(accountID model.AccountID, id model.InstrumentID, resp *spot.CancelOrderResponse) model.OrderStatusReport {
	return model.OrderStatusReport{
		AccountID:    accountID,
		InstrumentID: id,
		OrderID:      model.OrderID(strconv.FormatInt(resp.OrderID, 10)),
		ClientID:     model.ClientOrderID(resp.ClientOrderID),
		Status:       orderStatusFromBinance(resp.Status),
		Side:         orderSideFromBinance(resp.Side),
		Type:         orderTypeFromBinance(resp.Type),
		Quantity:     parseDecimal(resp.OrigQty),
		FilledQty:    parseDecimal(resp.ExecutedQty),
		AvgPrice:     parseDecimal(resp.Price),
		EventTime:    time.Now(),
	}
}

func perpOrderReport(accountID model.AccountID, id model.InstrumentID, resp *perp.OrderResponse) model.OrderStatusReport {
	if resp == nil {
		return model.OrderStatusReport{AccountID: accountID, InstrumentID: id}
	}
	return model.OrderStatusReport{
		AccountID:    accountID,
		InstrumentID: id,
		OrderID:      model.OrderID(strconv.FormatInt(resp.OrderID, 10)),
		ClientID:     model.ClientOrderID(resp.ClientOrderID),
		Status:       orderStatusFromBinance(resp.Status),
		Side:         orderSideFromBinance(resp.Side),
		Type:         orderTypeFromBinance(resp.Type),
		Quantity:     parseDecimal(resp.OrigQty),
		FilledQty:    parseDecimal(resp.ExecutedQty),
		AvgPrice:     parseDecimal(resp.AvgPrice),
		EventTime:    timeFromUnixMilli(resp.UpdateTime),
	}
}

func spotFillReport(accountID model.AccountID, id model.InstrumentID, trade spot.Trade) model.FillReport {
	side := model.OrderSideSell
	if trade.IsBuyer {
		side = model.OrderSideBuy
	}
	return model.FillReport{
		AccountID:    accountID,
		InstrumentID: id,
		OrderID:      model.OrderID(strconv.FormatInt(trade.OrderID, 10)),
		TradeID:      model.TradeID(strconv.FormatInt(trade.ID, 10)),
		Side:         side,
		Quantity:     parseDecimal(trade.Qty),
		Price:        parseDecimal(trade.Price),
		Fee:          moneyFromCommission(trade.Commission, trade.CommissionAsset),
		EventTime:    timeFromUnixMilli(trade.Time),
	}
}

func perpFillReport(accountID model.AccountID, id model.InstrumentID, trade perp.Trade) model.FillReport {
	side := model.OrderSideSell
	if trade.IsBuyer {
		side = model.OrderSideBuy
	}
	return model.FillReport{
		AccountID:    accountID,
		InstrumentID: id,
		OrderID:      model.OrderID(strconv.FormatInt(trade.OrderID, 10)),
		TradeID:      model.TradeID(strconv.FormatInt(trade.ID, 10)),
		Side:         side,
		Quantity:     parseDecimal(trade.Qty),
		Price:        parseDecimal(trade.Price),
		Fee:          moneyFromCommission(trade.Commission, trade.CommissionAsset),
		EventTime:    timeFromUnixMilli(trade.Time),
	}
}

func orderStatusFromBinance(status string) model.OrderStatus {
	switch status {
	case "NEW":
		return model.OrderStatusAccepted
	case "PARTIALLY_FILLED":
		return model.OrderStatusPartiallyFilled
	case "FILLED":
		return model.OrderStatusFilled
	case "CANCELED":
		return model.OrderStatusCanceled
	case "REJECTED":
		return model.OrderStatusRejected
	case "EXPIRED":
		return model.OrderStatusExpired
	default:
		return model.OrderStatusSubmitted
	}
}

func orderSideFromBinance(side string) model.OrderSide {
	if side == "SELL" {
		return model.OrderSideSell
	}
	return model.OrderSideBuy
}

func orderTypeFromBinance(t string) model.OrderType {
	if t == "LIMIT" {
		return model.OrderTypeLimit
	}
	return model.OrderTypeMarket
}
