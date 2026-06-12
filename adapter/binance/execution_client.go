package binance

import (
	"context"
	"fmt"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/venue"
)

type privateExecutionStream interface {
	Connect(ctx context.Context) error
	Disconnect(ctx context.Context) error
}

type executionClient struct {
	accountID   model.AccountID
	instruments venue.InstrumentProvider
	normalizer  symbolNormalizer
	events      chan model.ExecutionEvent
	health      venue.ExecutionHealth
	stream      privateExecutionStream
}

func newExecutionClient(accountID model.AccountID, instruments venue.InstrumentProvider, stream privateExecutionStream) *executionClient {
	return &executionClient{
		accountID:   accountID,
		instruments: instruments,
		events:      make(chan model.ExecutionEvent, 256),
		stream:      stream,
	}
}

func (c *executionClient) AccountID() model.AccountID { return c.accountID }

func (c *executionClient) Venue() model.Venue { return model.VenueBinance }

func (c *executionClient) Connect(ctx context.Context) error {
	if c.stream != nil {
		if err := c.stream.Connect(ctx); err != nil {
			c.health.LastError = err
			return err
		}
	}
	c.health.Connected = true
	return nil
}

func (c *executionClient) Disconnect(ctx context.Context) error {
	if c.stream != nil {
		if err := c.stream.Disconnect(ctx); err != nil {
			c.health.LastError = err
			return err
		}
	}
	c.health.Connected = false
	return nil
}

func (c *executionClient) Health() venue.ExecutionHealth { return c.health }

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

func (c *executionClient) emitAccountState(state model.AccountState) {
	c.health.AccountReady = true
	c.health.LastEventTime = state.EventTime
	select {
	case c.events <- model.ExecutionEvent{AccountState: &state}:
	default:
	}
}

func (c *executionClient) emitOrder(report model.OrderStatusReport) {
	if report.EventTime.IsZero() {
		report.EventTime = time.Now()
	}
	c.health.LastEventTime = report.EventTime
	select {
	case c.events <- model.ExecutionEvent{Order: &report}:
	default:
	}
}

func (c *executionClient) emitOrderEvent(event model.OrderEvent) {
	if event.EventTime.IsZero() {
		event.EventTime = time.Now()
	}
	c.health.LastEventTime = event.EventTime
	select {
	case c.events <- model.ExecutionEvent{OrderEvent: &event}:
	default:
	}
}

func (c *executionClient) emitFill(report model.FillReport) {
	if report.EventTime.IsZero() {
		report.EventTime = time.Now()
	}
	c.health.LastEventTime = report.EventTime
	select {
	case c.events <- model.ExecutionEvent{Fill: &report}:
	default:
	}
}

func (c *executionClient) emitPosition(report model.PositionStatusReport) {
	if report.EventTime.IsZero() {
		report.EventTime = time.Now()
	}
	c.health.LastEventTime = report.EventTime
	select {
	case c.events <- model.ExecutionEvent{Position: &report}:
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
