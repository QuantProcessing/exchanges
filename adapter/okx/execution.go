package okx

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	okxsdk "github.com/QuantProcessing/exchanges/sdk/okx"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/shopspring/decimal"
)

type executionClient struct {
	accountID  model.AccountID
	provider   *productProvider
	sdk        sdkClient
	instType   string
	tdMode     string
	privateWS  privateWS
	events     chan model.ExecutionEvent
	mu         sync.Mutex
	registered bool
	health     venue.ExecutionHealth
}

func newExecutionClient(accountID model.AccountID, provider *productProvider, sdk sdkClient, instType, tdMode string, creds ...string) *executionClient {
	if accountID == "" {
		accountID = model.AccountID(fmt.Sprintf("okx-%s", strings.ToLower(instType)))
	}
	client := &executionClient{accountID: accountID, provider: provider, sdk: sdk, instType: instType, tdMode: tdMode, events: make(chan model.ExecutionEvent, 256)}
	if len(creds) >= 3 && creds[0] != "" && creds[1] != "" && creds[2] != "" {
		client.privateWS = newPrivateWS(creds[0], creds[1], creds[2])
	}
	return client
}

func (c *executionClient) Venue() model.Venue         { return Venue }
func (c *executionClient) AccountID() model.AccountID { return c.accountID }

func (c *executionClient) Connect(ctx context.Context) error {
	if len(c.provider.List()) == 0 {
		if err := c.provider.LoadAll(ctx); err != nil {
			c.health.LastError = err
			return err
		}
	}
	if c.privateWS != nil {
		if err := c.subscribePrivate(ctx); err != nil {
			c.health.LastError = err
			return err
		}
	}
	c.health.Connected = true
	c.health.AccountReady = true
	c.health.LastEventTime = time.Now()
	c.health.LastError = nil
	return nil
}

func (c *executionClient) Disconnect(context.Context) error {
	c.health.Connected = false
	if c.privateWS != nil {
		return c.privateWS.Close()
	}
	return nil
}

func (c *executionClient) Health() venue.ExecutionHealth       { return c.health }
func (c *executionClient) Events() <-chan model.ExecutionEvent { return c.events }
func (c *executionClient) ResubscribeExecution(ctx context.Context) error {
	if c.privateWS == nil {
		return model.ErrNotSupported
	}
	c.mu.Lock()
	c.registered = false
	c.mu.Unlock()
	return c.subscribePrivate(ctx)
}

func (c *executionClient) subscribePrivate(context.Context) error {
	c.mu.Lock()
	if c.registered {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()
	if err := c.privateWS.Connect(); err != nil {
		return err
	}
	if err := c.privateWS.SubscribeOrders(c.instType, nil, c.handleOrderUpdate); err != nil {
		return err
	}
	if err := c.privateWS.SubscribePositions(c.instType, c.handlePositionUpdate); err != nil {
		return err
	}
	c.mu.Lock()
	c.registered = true
	c.mu.Unlock()
	return nil
}

func (c *executionClient) handleOrderUpdate(order *okxsdk.Order) {
	if order == nil {
		return
	}
	id, ok := c.provider.instrumentIDByRaw(order.InstId)
	if !ok {
		c.health.LastError = fmt.Errorf("%w: %s", model.ErrInstrumentNotFound, order.InstId)
		return
	}
	report := c.mapOrder(id, *order)
	_ = c.emitExecution(model.ExecutionEvent{Order: &report})
	if !decimalOrFallback(order.FillSz, "0").IsPositive() || order.TradeId == "" {
		return
	}
	fill := model.FillReport{
		AccountID:     c.accountID,
		InstrumentID:  id,
		OrderID:       model.OrderID(order.OrdId),
		ClientOrderID: model.ClientOrderID(order.ClOrdId),
		TradeID:       model.TradeID(order.TradeId),
		Side:          fromVenueSide(order.Side),
		Price:         decimalOrFallback(order.FillPx, "0"),
		Quantity:      decimalOrFallback(order.FillSz, "0"),
		Fee:           decimalOrFallback(order.Fee, "0").Abs(),
		FeeCurrency:   model.Currency(order.FeeCcy),
		Timestamp:     parseUnixMillis(order.FillTime),
	}
	_ = c.emitExecution(model.ExecutionEvent{Fill: &fill})
}

func (c *executionClient) handlePositionUpdate(position *okxsdk.Position) {
	if position == nil {
		return
	}
	id, ok := c.provider.instrumentIDByRaw(position.InstId)
	if !ok {
		c.health.LastError = fmt.Errorf("%w: %s", model.ErrInstrumentNotFound, position.InstId)
		return
	}
	report := c.mapPosition(id, *position)
	_ = c.emitExecution(model.ExecutionEvent{Position: &report})
}

func (c *executionClient) emitExecution(event model.ExecutionEvent) error {
	if err := event.Validate(); err != nil {
		c.health.LastError = err
		return err
	}
	c.health.LastEventTime = time.Now()
	select {
	case c.events <- event:
		return nil
	default:
		err := fmt.Errorf("%w: okx execution event channel full", model.ErrInvalidExecutionEvent)
		c.health.LastError = err
		return err
	}
}

func (c *executionClient) QueryAccount(ctx context.Context) (model.AccountSnapshot, error) {
	balances, err := c.sdk.GetAccountBalance(ctx, nil)
	if err != nil {
		return model.AccountSnapshot{}, err
	}
	snapshot := model.AccountSnapshot{AccountID: c.accountID, Venue: Venue, Timestamp: time.Now()}
	for _, balance := range balances {
		for _, detail := range balance.Details {
			free := decimalOrFallback(defaultString(detail.AvailBal, detail.CashBal), "0")
			locked := decimalOrFallback(defaultString(detail.FrozenBal, detail.OrdFrozen), "0")
			total := defaultString(detail.Eq, free.Add(locked).String())
			snapshot.Balances = append(snapshot.Balances, model.Balance{
				Currency: model.Currency(detail.Ccy),
				Free:     free.String(),
				Locked:   locked.String(),
				Total:    total,
			})
		}
	}
	return snapshot, nil
}

func (c *executionClient) SubmitOrder(ctx context.Context, cmd model.SubmitOrder) (model.OrderStatusReport, error) {
	if err := cmd.Validate(); err != nil {
		return model.OrderStatusReport{}, err
	}
	raw, err := c.provider.rawSymbol(cmd.InstrumentID)
	if err != nil {
		return model.OrderStatusReport{}, err
	}
	clientOrderID := string(cmd.ClientOrderID)
	req := &okxsdk.OrderRequest{
		InstId:  raw,
		TdMode:  c.tdMode,
		ClOrdId: &clientOrderID,
		Side:    toVenueSide(cmd.Side),
		OrdType: toVenueOrderType(cmd.Type, cmd.TimeInForce),
		Sz:      cmd.Quantity.String(),
		Px:      zeroBlank(cmd.Price),
	}
	resp, err := c.sdk.PlaceOrder(ctx, req)
	if err != nil {
		return model.OrderStatusReport{}, err
	}
	if len(resp) == 0 {
		return model.OrderStatusReport{}, fmt.Errorf("%w: empty OKX order response", model.ErrInvalidOrder)
	}
	return c.mapOrderID(cmd, resp[0]), nil
}

func (c *executionClient) CancelOrder(ctx context.Context, cmd model.CancelOrder) (model.OrderStatusReport, error) {
	if err := cmd.Validate(); err != nil {
		return model.OrderStatusReport{}, err
	}
	raw, err := c.provider.rawSymbol(cmd.InstrumentID)
	if err != nil {
		return model.OrderStatusReport{}, err
	}
	resp, err := c.sdk.CancelOrder(ctx, raw, string(cmd.OrderID), string(cmd.ClientOrderID))
	if err != nil {
		return model.OrderStatusReport{}, err
	}
	if len(resp) == 0 {
		return model.OrderStatusReport{}, fmt.Errorf("%w: empty OKX cancel response", model.ErrInvalidOrder)
	}
	return model.OrderStatusReport{
		AccountID:       c.accountID,
		InstrumentID:    cmd.InstrumentID,
		OrderID:         model.OrderID(defaultString(resp[0].OrdId, string(cmd.OrderID))),
		ClientOrderID:   model.ClientOrderID(defaultString(resp[0].ClOrdId, string(cmd.ClientOrderID))),
		Status:          model.OrderStatusCanceled,
		LastUpdatedTime: time.Now(),
	}, nil
}

func (c *executionClient) GenerateOrderStatusReports(ctx context.Context, id model.InstrumentID) ([]model.OrderStatusReport, error) {
	raw, err := c.provider.rawSymbol(id)
	if err != nil {
		return nil, err
	}
	orders, err := c.sdk.GetOrders(ctx, &c.instType, &raw)
	if err != nil {
		return nil, err
	}
	reports := make([]model.OrderStatusReport, 0, len(orders))
	for _, order := range orders {
		reports = append(reports, c.mapOrder(id, order))
	}
	return reports, nil
}

func (c *executionClient) GeneratePositionStatusReports(ctx context.Context, id model.InstrumentID) ([]model.PositionStatusReport, error) {
	raw, err := c.provider.rawSymbol(id)
	if err != nil {
		return nil, err
	}
	positions, err := c.sdk.GetPositions(ctx, &c.instType, &raw)
	if err != nil {
		return nil, err
	}
	reports := make([]model.PositionStatusReport, 0, len(positions))
	for _, position := range positions {
		reports = append(reports, c.mapPosition(id, position))
	}
	return reports, nil
}

func (c *executionClient) mapOrderID(cmd model.SubmitOrder, order okxsdk.OrderId) model.OrderStatusReport {
	status := model.OrderStatusAccepted
	if order.SCode != "" && order.SCode != "0" {
		status = model.OrderStatusRejected
	}
	return model.OrderStatusReport{
		AccountID:       c.accountID,
		InstrumentID:    cmd.InstrumentID,
		OrderID:         model.OrderID(order.OrdId),
		ClientOrderID:   model.ClientOrderID(defaultString(order.ClOrdId, string(cmd.ClientOrderID))),
		Status:          status,
		Side:            cmd.Side,
		Type:            cmd.Type,
		Quantity:        cmd.Quantity,
		Price:           cmd.Price,
		LastUpdatedTime: time.Now(),
	}
}

func (c *executionClient) mapOrder(id model.InstrumentID, order okxsdk.Order) model.OrderStatusReport {
	quantity := decimalOrFallback(order.Sz, "0")
	filled := decimalOrFallback(order.AccFillSz, "0")
	leaves := quantity.Sub(filled)
	if leaves.IsNegative() {
		leaves = decimal.Zero
	}
	return model.OrderStatusReport{
		AccountID:       c.accountID,
		InstrumentID:    id,
		OrderID:         model.OrderID(order.OrdId),
		ClientOrderID:   model.ClientOrderID(order.ClOrdId),
		Status:          mapOrderStatus(order.State),
		Side:            fromVenueSide(order.Side),
		Type:            fromVenueOrderType(order.OrdType),
		Quantity:        quantity,
		FilledQuantity:  filled,
		LeavesQuantity:  leaves,
		Price:           decimalOrFallback(order.Px, "0"),
		AveragePrice:    decimalOrFallback(order.AvgPx, "0"),
		LastUpdatedTime: parseUnixMillis(defaultString(order.UTime, defaultString(order.FillTime, order.CTime))),
	}
}

func (c *executionClient) mapPosition(id model.InstrumentID, position okxsdk.Position) model.PositionStatusReport {
	qty := decimalOrFallback(position.Pos, "0")
	return model.PositionStatusReport{
		AccountID:    c.accountID,
		InstrumentID: id,
		PositionID:   model.PositionID(defaultString(position.PosId, id.String())),
		Side:         okxPositionSide(qty, string(position.PosSide)),
		Quantity:     qty.Abs(),
		EntryPrice:   decimalOrFallback(position.AvgPx, "0"),
		Timestamp:    parseUnixMillis(position.UTime),
	}
}

func okxPositionSide(qty decimal.Decimal, raw string) model.PositionSide {
	switch strings.ToLower(raw) {
	case "short":
		return model.PositionSideShort
	case "long":
		return model.PositionSideLong
	}
	if qty.IsNegative() {
		return model.PositionSideShort
	}
	if qty.IsPositive() {
		return model.PositionSideLong
	}
	return model.PositionSideFlat
}

type privateWS interface {
	Connect() error
	SubscribeOrders(string, *string, func(*okxsdk.Order)) error
	SubscribePositions(string, func(*okxsdk.Position)) error
	Close() error
}

type okxPrivateWS struct {
	client *okxsdk.WSClient
}

func newPrivateWS(apiKey, secretKey, passphrase string) *okxPrivateWS {
	return &okxPrivateWS{client: okxsdk.NewWSClient(context.Background()).WithCredentials(apiKey, secretKey, passphrase)}
}

func (w *okxPrivateWS) Connect() error {
	return w.client.Connect()
}

func (w *okxPrivateWS) SubscribeOrders(instType string, instID *string, handler func(*okxsdk.Order)) error {
	return w.client.SubscribeOrders(instType, instID, handler)
}

func (w *okxPrivateWS) SubscribePositions(instType string, handler func(*okxsdk.Position)) error {
	return w.client.SubscribePositions(instType, handler)
}

func (w *okxPrivateWS) Close() error {
	if w.client.Conn == nil {
		return nil
	}
	return w.client.Conn.Close()
}
