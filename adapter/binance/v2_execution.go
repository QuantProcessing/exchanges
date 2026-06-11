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

var _ venue.ExecutionClient = (*v2ExecutionClient)(nil)

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

type v2ExecutionClient struct {
	accountID   model.AccountID
	instruments venue.InstrumentProvider
	normalizer  v2SymbolNormalizer
	spot        binanceSpotExecutionClient
	perp        binancePerpExecutionClient
	events      chan model.ExecutionEvent
	health      venue.ExecutionHealth
}

func newV2ExecutionClient(accountID model.AccountID, instruments venue.InstrumentProvider, spotClient binanceSpotExecutionClient, perpClient binancePerpExecutionClient) *v2ExecutionClient {
	return &v2ExecutionClient{
		accountID:   accountID,
		instruments: instruments,
		spot:        spotClient,
		perp:        perpClient,
		events:      make(chan model.ExecutionEvent, 128),
	}
}

func (c *v2ExecutionClient) AccountID() model.AccountID { return c.accountID }

func (c *v2ExecutionClient) Venue() model.Venue { return model.VenueBinance }

func (c *v2ExecutionClient) Connect(context.Context) error {
	c.health.Connected = true
	return nil
}

func (c *v2ExecutionClient) Disconnect(context.Context) error {
	c.health.Connected = false
	return nil
}

func (c *v2ExecutionClient) Health() venue.ExecutionHealth { return c.health }

func (c *v2ExecutionClient) SubmitOrder(ctx context.Context, cmd model.SubmitOrder) error {
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
			Side:             v2BinanceSide(cmd.Side),
			Type:             v2BinanceOrderType(cmd.Type),
			Quantity:         cmd.Quantity.String(),
			Price:            optionalDecimalString(cmd.Price),
			NewClientOrderID: string(cmd.ClientID),
		})
		if err != nil {
			return err
		}
		c.emitOrder(v2SpotOrderReport(c.accountID, cmd.InstrumentID, resp))
		return nil
	case model.InstrumentTypeCryptoPerp:
		if c.perp == nil {
			return fmt.Errorf("%w: binance perp execution", model.ErrNotSupported)
		}
		resp, err := c.perp.PlaceOrder(ctx, perp.PlaceOrderParams{
			Symbol:           raw,
			Side:             v2BinanceSide(cmd.Side),
			Type:             v2BinanceOrderType(cmd.Type),
			Quantity:         cmd.Quantity.String(),
			Price:            optionalDecimalString(cmd.Price),
			NewClientOrderID: string(cmd.ClientID),
			ReduceOnly:       cmd.ReduceOnly,
		})
		if err != nil {
			return err
		}
		c.emitOrder(v2PerpOrderReport(c.accountID, cmd.InstrumentID, resp))
		return nil
	default:
		return fmt.Errorf("%w: unsupported instrument type %s", model.ErrNotSupported, inst.Type)
	}
}

func (c *v2ExecutionClient) ModifyOrder(context.Context, model.ModifyOrder) error {
	return model.ErrNotSupported
}

func (c *v2ExecutionClient) CancelOrder(ctx context.Context, cmd model.CancelOrder) error {
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
			c.emitOrder(v2SpotCancelReport(c.accountID, cmd.InstrumentID, resp))
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
		c.emitOrder(v2PerpOrderReport(c.accountID, cmd.InstrumentID, resp))
		return nil
	default:
		return fmt.Errorf("%w: unsupported instrument type %s", model.ErrNotSupported, inst.Type)
	}
}

func (c *v2ExecutionClient) CancelAllOrders(ctx context.Context, cmd model.CancelAllOrders) error {
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

func (c *v2ExecutionClient) QueryAccount(ctx context.Context) error {
	if c.spot != nil {
		resp, err := c.spot.GetAccount(ctx)
		if err != nil {
			return err
		}
		state, err := v2SpotAccountState(c.accountID, resp)
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
		state, err := v2PerpAccountState(c.accountID, resp)
		if err != nil {
			return err
		}
		c.emitAccountState(state)
	}
	return nil
}

func (c *v2ExecutionClient) GenerateOrderStatusReports(ctx context.Context, q venue.OrderStatusQuery) ([]model.OrderStatusReport, error) {
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
			out = append(out, v2SpotOrderReport(c.accountID, q.InstrumentID, &orders[i]))
		}
		return out, nil
	case model.InstrumentTypeCryptoPerp:
		orders, err := c.perp.GetOpenOrders(ctx, raw)
		if err != nil {
			return nil, err
		}
		out := make([]model.OrderStatusReport, 0, len(orders))
		for i := range orders {
			out = append(out, v2PerpOrderReport(c.accountID, q.InstrumentID, &orders[i]))
		}
		return out, nil
	default:
		return nil, model.ErrNotSupported
	}
}

func (c *v2ExecutionClient) GenerateFillReports(ctx context.Context, q venue.FillQuery) ([]model.FillReport, error) {
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
			out = append(out, v2SpotFillReport(c.accountID, q.InstrumentID, trade))
		}
		return out, nil
	case model.InstrumentTypeCryptoPerp:
		trades, err := c.perp.MyTrades(ctx, raw, 1000, q.Since.UnixMilli(), 0, 0)
		if err != nil {
			return nil, err
		}
		out := make([]model.FillReport, 0, len(trades))
		for _, trade := range trades {
			out = append(out, v2PerpFillReport(c.accountID, q.InstrumentID, trade))
		}
		return out, nil
	default:
		return nil, model.ErrNotSupported
	}
}

func (c *v2ExecutionClient) GeneratePositionStatusReports(context.Context, venue.PositionQuery) ([]model.PositionStatusReport, error) {
	return nil, model.ErrNotSupported
}

func (c *v2ExecutionClient) Events() <-chan model.ExecutionEvent { return c.events }

func (c *v2ExecutionClient) instrumentAndRaw(id model.InstrumentID) (model.Instrument, string, error) {
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

func (c *v2ExecutionClient) emitOrder(report model.OrderStatusReport) {
	c.health.LastEventTime = report.EventTime
	select {
	case c.events <- model.ExecutionEvent{Order: &report}:
	default:
	}
}

func (c *v2ExecutionClient) emitAccountState(state model.AccountState) {
	c.health.AccountReady = true
	c.health.LastEventTime = state.EventTime
	select {
	case c.events <- model.ExecutionEvent{AccountState: &state}:
	default:
	}
}

func v2BinanceSide(side model.OrderSide) string {
	if side == model.OrderSideSell {
		return "SELL"
	}
	return "BUY"
}

func v2BinanceOrderType(t model.OrderType) string {
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

func v2SpotOrderReport(accountID model.AccountID, id model.InstrumentID, resp *spot.OrderResponse) model.OrderStatusReport {
	if resp == nil {
		return model.OrderStatusReport{AccountID: accountID, InstrumentID: id}
	}
	return model.OrderStatusReport{
		AccountID:    accountID,
		InstrumentID: id,
		OrderID:      model.OrderID(strconv.FormatInt(resp.OrderID, 10)),
		ClientID:     model.ClientOrderID(resp.ClientOrderID),
		Status:       v2OrderStatusFromBinance(resp.Status),
		Side:         v2OrderSideFromBinance(resp.Side),
		Type:         v2OrderTypeFromBinance(resp.Type),
		Quantity:     parseDecimal(resp.OrigQty),
		FilledQty:    parseDecimal(resp.ExecutedQty),
		AvgPrice:     parseDecimal(resp.Price),
		EventTime:    timeFromUnixMilli(resp.TransactTime),
	}
}

func v2SpotCancelReport(accountID model.AccountID, id model.InstrumentID, resp *spot.CancelOrderResponse) model.OrderStatusReport {
	return model.OrderStatusReport{
		AccountID:    accountID,
		InstrumentID: id,
		OrderID:      model.OrderID(strconv.FormatInt(resp.OrderID, 10)),
		ClientID:     model.ClientOrderID(resp.ClientOrderID),
		Status:       v2OrderStatusFromBinance(resp.Status),
		Side:         v2OrderSideFromBinance(resp.Side),
		Type:         v2OrderTypeFromBinance(resp.Type),
		Quantity:     parseDecimal(resp.OrigQty),
		FilledQty:    parseDecimal(resp.ExecutedQty),
		AvgPrice:     parseDecimal(resp.Price),
		EventTime:    time.Now(),
	}
}

func v2PerpOrderReport(accountID model.AccountID, id model.InstrumentID, resp *perp.OrderResponse) model.OrderStatusReport {
	if resp == nil {
		return model.OrderStatusReport{AccountID: accountID, InstrumentID: id}
	}
	return model.OrderStatusReport{
		AccountID:    accountID,
		InstrumentID: id,
		OrderID:      model.OrderID(strconv.FormatInt(resp.OrderID, 10)),
		ClientID:     model.ClientOrderID(resp.ClientOrderID),
		Status:       v2OrderStatusFromBinance(resp.Status),
		Side:         v2OrderSideFromBinance(resp.Side),
		Type:         v2OrderTypeFromBinance(resp.Type),
		Quantity:     parseDecimal(resp.OrigQty),
		FilledQty:    parseDecimal(resp.ExecutedQty),
		AvgPrice:     parseDecimal(resp.AvgPrice),
		EventTime:    timeFromUnixMilli(resp.UpdateTime),
	}
}

func v2SpotFillReport(accountID model.AccountID, id model.InstrumentID, trade spot.Trade) model.FillReport {
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
		Fee:          v2MoneyFromCommission(trade.Commission, trade.CommissionAsset),
		EventTime:    timeFromUnixMilli(trade.Time),
	}
}

func v2PerpFillReport(accountID model.AccountID, id model.InstrumentID, trade perp.Trade) model.FillReport {
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
		Fee:          v2MoneyFromCommission(trade.Commission, trade.CommissionAsset),
		EventTime:    timeFromUnixMilli(trade.Time),
	}
}

func v2OrderStatusFromBinance(status string) model.OrderStatus {
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

func v2OrderSideFromBinance(side string) model.OrderSide {
	if side == "SELL" {
		return model.OrderSideSell
	}
	return model.OrderSideBuy
}

func v2OrderTypeFromBinance(t string) model.OrderType {
	if t == "LIMIT" {
		return model.OrderTypeLimit
	}
	return model.OrderTypeMarket
}
