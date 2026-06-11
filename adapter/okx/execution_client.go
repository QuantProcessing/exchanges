package okx

import (
	"context"
	"fmt"

	"github.com/QuantProcessing/exchanges/model"
	sdkokx "github.com/QuantProcessing/exchanges/sdk/okx"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/shopspring/decimal"
)

var _ venue.ExecutionClient = (*executionClient)(nil)

type okxExecutionClient interface {
	PlaceOrder(ctx context.Context, req *sdkokx.OrderRequest) ([]sdkokx.OrderId, error)
	ModifyOrder(ctx context.Context, req *sdkokx.ModifyOrderRequest) ([]sdkokx.OrderId, error)
	CancelOrder(ctx context.Context, instId, ordId, clOrdId string) ([]sdkokx.OrderId, error)
	CancelOrders(ctx context.Context, reqs []sdkokx.CancelOrderRequest) ([]sdkokx.OrderId, error)
	GetOrders(ctx context.Context, instType, instId *string) ([]sdkokx.Order, error)
	GetAccountBalance(ctx context.Context, ccy *string) ([]sdkokx.Balance, error)
	GetPositions(ctx context.Context, instType, instId *string) ([]sdkokx.Position, error)
}

type executionClient struct {
	accountID   model.AccountID
	instruments venue.InstrumentProvider
	normalizer  symbolNormalizer
	client      okxExecutionClient
	events      chan model.ExecutionEvent
	health      venue.ExecutionHealth
}

func newExecutionClient(accountID model.AccountID, instruments venue.InstrumentProvider, client okxExecutionClient) *executionClient {
	return &executionClient{
		accountID:   accountID,
		instruments: instruments,
		client:      client,
		events:      make(chan model.ExecutionEvent, 128),
	}
}

func (c *executionClient) AccountID() model.AccountID { return c.accountID }

func (c *executionClient) Venue() model.Venue { return model.VenueOKX }

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
	if c.client == nil {
		return fmt.Errorf("%w: okx execution", model.ErrNotSupported)
	}
	req := &sdkokx.OrderRequest{
		InstId:  raw,
		TdMode:  okxTdMode(inst),
		Side:    okxSide(cmd.Side),
		OrdType: okxOrderType(cmd.Type),
		Sz:      okxOrderSize(inst, cmd.Quantity),
	}
	if cmd.ClientID != "" {
		clientID := string(cmd.ClientID)
		req.ClOrdId = &clientID
	}
	if cmd.Price.IsPositive() {
		price := cmd.Price.String()
		req.Px = &price
	}
	if inst.Type == model.InstrumentTypeCurrencyPair && cmd.Type == model.OrderTypeMarket {
		tgtCcy := "base_ccy"
		req.TgtCcy = &tgtCcy
	}
	if cmd.ReduceOnly {
		reduceOnly := true
		req.ReduceOnly = &reduceOnly
	}
	ids, err := c.client.PlaceOrder(ctx, req)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return invalidOKXAccountState("empty place order response")
	}
	if err := okxOrderActionError("place order", ids[0]); err != nil {
		return err
	}
	c.emitOrder(orderAckReport(c.accountID, cmd.InstrumentID, cmd, ids[0], model.OrderStatusSubmitted))
	return nil
}

func (c *executionClient) ModifyOrder(ctx context.Context, cmd model.ModifyOrder) error {
	inst, raw, err := c.instrumentAndRaw(cmd.InstrumentID)
	if err != nil {
		return err
	}
	if c.client == nil {
		return fmt.Errorf("%w: okx execution", model.ErrNotSupported)
	}
	req := &sdkokx.ModifyOrderRequest{InstId: raw}
	if cmd.OrderID != "" {
		orderID := string(cmd.OrderID)
		req.OrdId = &orderID
	}
	if cmd.ClientID != "" {
		clientID := string(cmd.ClientID)
		req.ClOrdId = &clientID
	}
	if cmd.Quantity.IsPositive() {
		size := okxOrderSize(inst, cmd.Quantity)
		req.NewSz = &size
	}
	if cmd.Price.IsPositive() {
		price := cmd.Price.String()
		req.NewPx = &price
	}
	ids, err := c.client.ModifyOrder(ctx, req)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return invalidOKXAccountState("empty modify order response")
	}
	return okxOrderActionError("modify order", ids[0])
}

func (c *executionClient) CancelOrder(ctx context.Context, cmd model.CancelOrder) error {
	_, raw, err := c.instrumentAndRaw(cmd.InstrumentID)
	if err != nil {
		return err
	}
	if c.client == nil {
		return fmt.Errorf("%w: okx execution", model.ErrNotSupported)
	}
	ids, err := c.client.CancelOrder(ctx, raw, string(cmd.OrderID), string(cmd.ClientID))
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return invalidOKXAccountState("empty cancel order response")
	}
	if err := okxOrderActionError("cancel order", ids[0]); err != nil {
		return err
	}
	c.emitOrder(model.OrderStatusReport{
		AccountID:    c.accountID,
		InstrumentID: cmd.InstrumentID,
		OrderID:      model.OrderID(ids[0].OrdId),
		ClientID:     model.ClientOrderID(ids[0].ClOrdId),
		Status:       model.OrderStatusCanceled,
		EventTime:    timeFromOKXMillis(ids[0].Ts),
	})
	return nil
}

func (c *executionClient) CancelAllOrders(ctx context.Context, cmd model.CancelAllOrders) error {
	inst, raw, err := c.instrumentAndRaw(cmd.InstrumentID)
	if err != nil {
		return err
	}
	orders, err := c.openOrders(ctx, inst, raw)
	if err != nil {
		return err
	}
	reqs := make([]sdkokx.CancelOrderRequest, 0, len(orders))
	for _, order := range orders {
		orderID := order.OrdId
		reqs = append(reqs, sdkokx.CancelOrderRequest{InstId: raw, OrdId: &orderID})
	}
	if len(reqs) == 0 {
		return nil
	}
	ids, err := c.client.CancelOrders(ctx, reqs)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := okxOrderActionError("cancel all orders", id); err != nil {
			return err
		}
	}
	return nil
}

func (c *executionClient) QueryAccount(ctx context.Context) error {
	if c.client == nil {
		return fmt.Errorf("%w: okx execution", model.ErrNotSupported)
	}
	balances, err := c.client.GetAccountBalance(ctx, nil)
	if err != nil {
		return err
	}
	state, err := okxAccountState(c.accountID, balances)
	if err != nil {
		return err
	}
	c.emitAccountState(state)
	return nil
}

func (c *executionClient) GenerateOrderStatusReports(ctx context.Context, q venue.OrderStatusQuery) ([]model.OrderStatusReport, error) {
	inst, raw, err := c.instrumentAndRaw(q.InstrumentID)
	if err != nil {
		return nil, err
	}
	orders, err := c.openOrders(ctx, inst, raw)
	if err != nil {
		return nil, err
	}
	out := make([]model.OrderStatusReport, 0, len(orders))
	for _, order := range orders {
		out = append(out, okxOrderReport(c.accountID, q.InstrumentID, inst, order))
	}
	return out, nil
}

func (c *executionClient) GenerateFillReports(context.Context, venue.FillQuery) ([]model.FillReport, error) {
	return nil, model.ErrNotSupported
}

func (c *executionClient) GeneratePositionStatusReports(ctx context.Context, q venue.PositionQuery) ([]model.PositionStatusReport, error) {
	if q.InstrumentID == (model.InstrumentID{}) {
		return nil, model.ErrNotSupported
	}
	if c.client == nil {
		return nil, fmt.Errorf("%w: okx execution", model.ErrNotSupported)
	}
	inst, raw, err := c.instrumentAndRaw(q.InstrumentID)
	if err != nil {
		return nil, err
	}
	if inst.Type != model.InstrumentTypeCryptoPerp {
		return nil, model.ErrNotSupported
	}
	instType := "SWAP"
	positions, err := c.client.GetPositions(ctx, &instType, &raw)
	if err != nil {
		return nil, err
	}
	out := make([]model.PositionStatusReport, 0, len(positions))
	for _, pos := range positions {
		report, ok, err := okxPositionReport(c.accountID, inst, pos)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, report)
		}
	}
	return out, nil
}

func (c *executionClient) Events() <-chan model.ExecutionEvent { return c.events }

func (c *executionClient) openOrders(ctx context.Context, inst model.Instrument, raw string) ([]sdkokx.Order, error) {
	if c.client == nil {
		return nil, fmt.Errorf("%w: okx execution", model.ErrNotSupported)
	}
	instType := "SPOT"
	if inst.Type == model.InstrumentTypeCryptoPerp {
		instType = "SWAP"
	}
	return c.client.GetOrders(ctx, &instType, &raw)
}

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

func orderAckReport(accountID model.AccountID, id model.InstrumentID, cmd model.SubmitOrder, ack sdkokx.OrderId, status model.OrderStatus) model.OrderStatusReport {
	return model.OrderStatusReport{
		AccountID:    accountID,
		InstrumentID: id,
		OrderID:      model.OrderID(ack.OrdId),
		ClientID:     model.ClientOrderID(firstNonBlank(ack.ClOrdId, string(cmd.ClientID))),
		Status:       status,
		Side:         cmd.Side,
		Type:         cmd.Type,
		Quantity:     cmd.Quantity,
		AvgPrice:     cmd.Price,
		EventTime:    timeFromOKXMillis(ack.Ts),
	}
}

func okxOrderReport(accountID model.AccountID, id model.InstrumentID, inst model.Instrument, order sdkokx.Order) model.OrderStatusReport {
	return model.OrderStatusReport{
		AccountID:    accountID,
		InstrumentID: id,
		OrderID:      model.OrderID(order.OrdId),
		ClientID:     model.ClientOrderID(order.ClOrdId),
		Status:       okxOrderStatus(order.State),
		Side:         okxReportSide(order.Side),
		Type:         okxReportType(order.OrdType),
		Quantity:     okxReportQuantity(inst, parseString(order.Sz)),
		FilledQty:    okxReportQuantity(inst, parseString(order.AccFillSz)),
		AvgPrice:     parseString(order.AvgPx),
		EventTime:    timeFromOKXMillis(firstNonBlank(order.UTime, order.CTime)),
	}
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func okxOrderStatus(status sdkokx.OrderStatus) model.OrderStatus {
	switch status {
	case sdkokx.OrderStatusLive:
		return model.OrderStatusAccepted
	case sdkokx.OrderStatusPartiallyFilled:
		return model.OrderStatusPartiallyFilled
	case sdkokx.OrderStatusFilled:
		return model.OrderStatusFilled
	case sdkokx.OrderStatusCanceled, sdkokx.OrderStatusMmpCanceled:
		return model.OrderStatusCanceled
	default:
		return model.OrderStatusSubmitted
	}
}

func okxReportSide(side sdkokx.Side) model.OrderSide {
	if side == sdkokx.SideSell {
		return model.OrderSideSell
	}
	return model.OrderSideBuy
}

func okxReportType(orderType sdkokx.OrderType) model.OrderType {
	if orderType == sdkokx.OrderTypeLimit || orderType == sdkokx.OrderTypePostOnly || orderType == sdkokx.OrderTypeFok || orderType == sdkokx.OrderTypeIoc {
		return model.OrderTypeLimit
	}
	return model.OrderTypeMarket
}

func okxSide(side model.OrderSide) string {
	if side == model.OrderSideSell {
		return "sell"
	}
	return "buy"
}

func okxOrderType(orderType model.OrderType) string {
	if orderType == model.OrderTypeLimit {
		return "limit"
	}
	return "market"
}

func okxTdMode(inst model.Instrument) string {
	if inst.Type == model.InstrumentTypeCurrencyPair {
		return "cash"
	}
	return "isolated"
}

func okxOrderSize(inst model.Instrument, quantity decimal.Decimal) string {
	if inst.Type == model.InstrumentTypeCryptoPerp && inst.Multiplier.IsPositive() {
		return quantity.Div(inst.Multiplier).String()
	}
	return quantity.String()
}

func okxReportQuantity(inst model.Instrument, raw decimal.Decimal) decimal.Decimal {
	if inst.Type == model.InstrumentTypeCryptoPerp && inst.Multiplier.IsPositive() {
		return raw.Mul(inst.Multiplier)
	}
	return raw
}
