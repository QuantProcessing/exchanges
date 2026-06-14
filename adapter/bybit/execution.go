package bybit

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	bybitsdk "github.com/QuantProcessing/exchanges/sdk/bybit"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/shopspring/decimal"
)

type executionClient struct {
	accountID         model.AccountID
	provider          *productProvider
	sdk               sdkClient
	category          string
	privateWS         privateWS
	events            chan model.ExecutionEvent
	mu                sync.Mutex
	privateRegistered bool
	health            venue.ExecutionHealth
}

func newExecutionClient(accountID model.AccountID, provider *productProvider, sdk sdkClient, category string, creds ...string) *executionClient {
	if accountID == "" {
		accountID = model.AccountID(fmt.Sprintf("bybit-%s", strings.ToLower(category)))
	}
	client := &executionClient{accountID: accountID, provider: provider, sdk: sdk, category: category, events: make(chan model.ExecutionEvent, 256)}
	if len(creds) >= 2 && creds[0] != "" && creds[1] != "" {
		client.privateWS = bybitsdk.NewPrivateWSClient().WithCredentials(creds[0], creds[1])
	}
	return client
}

func (c *executionClient) Venue() model.Venue         { return Venue }
func (c *executionClient) AccountID() model.AccountID { return c.accountID }

func (c *executionClient) Connect(ctx context.Context) error {
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
	c.privateRegistered = false
	c.mu.Unlock()
	return c.subscribePrivate(ctx)
}

func (c *executionClient) subscribePrivate(ctx context.Context) error {
	c.mu.Lock()
	if c.privateRegistered {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	topics := []struct {
		topic   string
		handler func(json.RawMessage)
	}{
		{topic: "order", handler: c.handleOrderPayload},
		{topic: "execution", handler: c.handleExecutionPayload},
		{topic: "position", handler: c.handlePositionPayload},
	}
	for _, topic := range topics {
		if err := c.privateWS.Subscribe(ctx, topic.topic, topic.handler); err != nil {
			return err
		}
	}
	c.mu.Lock()
	c.privateRegistered = true
	c.mu.Unlock()
	return nil
}

func (c *executionClient) handleOrderPayload(payload json.RawMessage) {
	msg, err := bybitsdk.DecodeOrderMessage(payload)
	if err != nil {
		c.health.LastError = err
		return
	}
	for _, order := range msg.Data {
		id, ok := c.provider.instrumentIDByRaw(order.Symbol)
		if !ok {
			c.health.LastError = fmt.Errorf("%w: %s", model.ErrInstrumentNotFound, order.Symbol)
			continue
		}
		report := c.mapOrder(id, order)
		report.LastUpdatedTime = parseUnixMillis(order.UpdatedTime)
		_ = c.emitExecution(model.ExecutionEvent{Order: &report})
	}
}

func (c *executionClient) handleExecutionPayload(payload json.RawMessage) {
	msg, err := bybitsdk.DecodeExecutionMessage(payload)
	if err != nil {
		c.health.LastError = err
		return
	}
	for _, fill := range msg.Data {
		id, ok := c.provider.instrumentIDByRaw(fill.Symbol)
		if !ok {
			c.health.LastError = fmt.Errorf("%w: %s", model.ErrInstrumentNotFound, fill.Symbol)
			continue
		}
		report := model.FillReport{
			AccountID:     c.accountID,
			InstrumentID:  id,
			OrderID:       model.OrderID(fill.OrderID),
			ClientOrderID: model.ClientOrderID(fill.OrderLinkID),
			TradeID:       model.TradeID(fill.ExecID),
			Side:          fromVenueSide(fill.Side),
			Price:         decimalOrFallback(fill.ExecPrice, "0"),
			Quantity:      decimalOrFallback(fill.ExecQty, "0"),
			Fee:           decimalOrFallback(fill.ExecFee, "0"),
			FeeCurrency:   model.Currency(fill.FeeCurrency),
			Timestamp:     parseUnixMillis(fill.ExecTime),
		}
		_ = c.emitExecution(model.ExecutionEvent{Fill: &report})
	}
}

func (c *executionClient) handlePositionPayload(payload json.RawMessage) {
	msg, err := bybitsdk.DecodePositionMessage(payload)
	if err != nil {
		c.health.LastError = err
		return
	}
	for _, pos := range msg.Data {
		id, ok := c.provider.instrumentIDByRaw(pos.Symbol)
		if !ok {
			c.health.LastError = fmt.Errorf("%w: %s", model.ErrInstrumentNotFound, pos.Symbol)
			continue
		}
		qty := decimalOrFallback(pos.Size, "0")
		report := model.PositionStatusReport{
			AccountID:    c.accountID,
			InstrumentID: id,
			PositionID:   model.PositionID(id.String()),
			Side:         bybitPositionSide(qty, pos.Side),
			Quantity:     qty.Abs(),
			EntryPrice:   decimalOrFallback(pos.AvgPrice, "0"),
			Timestamp:    time.Now(),
		}
		_ = c.emitExecution(model.ExecutionEvent{Position: &report})
	}
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
		err := fmt.Errorf("%w: bybit execution event channel full", model.ErrInvalidExecutionEvent)
		c.health.LastError = err
		return err
	}
}

func (c *executionClient) QueryAccount(ctx context.Context) (model.AccountSnapshot, error) {
	wallet, err := c.sdk.GetWalletBalance(ctx, "UNIFIED", "")
	if err != nil {
		return model.AccountSnapshot{}, err
	}
	snapshot := model.AccountSnapshot{AccountID: c.accountID, Venue: Venue, Timestamp: time.Now()}
	for _, account := range wallet.List {
		for _, coin := range account.Coin {
			free := decimalOrFallback(coin.WalletBalance, "0")
			locked := decimalOrFallback(coin.Locked, "0")
			snapshot.Balances = append(snapshot.Balances, model.Balance{
				Currency: model.Currency(coin.Coin),
				Free:     free.String(),
				Locked:   locked.String(),
				Total:    defaultString(coin.Equity, free.Add(locked).String()),
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
	resp, err := c.sdk.PlaceOrder(ctx, bybitsdk.PlaceOrderRequest{
		Category:    c.category,
		Symbol:      raw,
		Side:        toVenueSide(cmd.Side),
		OrderType:   toVenueOrderType(cmd.Type),
		Qty:         cmd.Quantity.String(),
		Price:       zeroBlank(cmd.Price),
		TimeInForce: toVenueTIF(cmd.TimeInForce),
		OrderLinkID: string(cmd.ClientOrderID),
	})
	if err != nil {
		return model.OrderStatusReport{}, err
	}
	return c.mapActionResponse(cmd, resp, model.OrderStatusAccepted), nil
}

func (c *executionClient) CancelOrder(ctx context.Context, cmd model.CancelOrder) (model.OrderStatusReport, error) {
	if err := cmd.Validate(); err != nil {
		return model.OrderStatusReport{}, err
	}
	raw, err := c.provider.rawSymbol(cmd.InstrumentID)
	if err != nil {
		return model.OrderStatusReport{}, err
	}
	resp, err := c.sdk.CancelOrder(ctx, bybitsdk.CancelOrderRequest{Category: c.category, Symbol: raw, OrderID: string(cmd.OrderID), OrderLinkID: string(cmd.ClientOrderID)})
	if err != nil {
		return model.OrderStatusReport{}, err
	}
	return model.OrderStatusReport{
		AccountID:       c.accountID,
		InstrumentID:    cmd.InstrumentID,
		OrderID:         model.OrderID(defaultString(resp.OrderID, string(cmd.OrderID))),
		ClientOrderID:   model.ClientOrderID(defaultString(resp.OrderLinkID, string(cmd.ClientOrderID))),
		Status:          model.OrderStatusCanceled,
		LastUpdatedTime: time.Now(),
	}, nil
}

func (c *executionClient) GenerateOrderStatusReports(ctx context.Context, id model.InstrumentID) ([]model.OrderStatusReport, error) {
	raw, err := c.provider.rawSymbol(id)
	if err != nil {
		return nil, err
	}
	orders, err := c.sdk.GetOpenOrders(ctx, c.category, raw)
	if err != nil {
		return nil, err
	}
	reports := make([]model.OrderStatusReport, 0, len(orders))
	for _, order := range orders {
		reports = append(reports, c.mapOrder(id, order))
	}
	return reports, nil
}

func (c *executionClient) mapActionResponse(cmd model.SubmitOrder, resp *bybitsdk.OrderActionResponse, status model.OrderStatus) model.OrderStatusReport {
	return model.OrderStatusReport{
		AccountID:       c.accountID,
		InstrumentID:    cmd.InstrumentID,
		OrderID:         model.OrderID(resp.OrderID),
		ClientOrderID:   model.ClientOrderID(defaultString(resp.OrderLinkID, string(cmd.ClientOrderID))),
		Status:          status,
		Side:            cmd.Side,
		Type:            cmd.Type,
		Quantity:        cmd.Quantity,
		Price:           cmd.Price,
		LastUpdatedTime: time.Now(),
	}
}

func (c *executionClient) mapOrder(id model.InstrumentID, order bybitsdk.OrderRecord) model.OrderStatusReport {
	return model.OrderStatusReport{
		AccountID:       c.accountID,
		InstrumentID:    id,
		OrderID:         model.OrderID(order.OrderID),
		ClientOrderID:   model.ClientOrderID(order.OrderLinkID),
		Status:          mapOrderStatus(order.OrderStatus),
		Side:            fromVenueSide(order.Side),
		Type:            fromVenueOrderType(order.OrderType),
		Quantity:        decimalOrFallback(order.Qty, "0"),
		FilledQuantity:  decimalOrFallback(order.CumExecQty, "0"),
		Price:           decimalOrFallback(order.Price, "0"),
		AveragePrice:    decimalOrFallback(order.AvgPrice, "0"),
		LastUpdatedTime: time.Now(),
	}
}

func bybitPositionSide(qty decimal.Decimal, raw string) model.PositionSide {
	switch strings.ToLower(raw) {
	case "sell", "short":
		return model.PositionSideShort
	case "buy", "long":
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
	Subscribe(context.Context, string, func(json.RawMessage)) error
	Close() error
}
