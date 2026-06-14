package testsuite

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/QuantProcessing/exchanges/internal/testenv"
	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/venue"
)

const marketDataStreamAttempts = 3

type DataTesterConfig struct {
	Provider     venue.InstrumentProvider
	Data         venue.DataClient
	InstrumentID model.InstrumentID
}

type DataTester struct {
	cfg DataTesterConfig
}

func NewDataTester(cfg DataTesterConfig) *DataTester {
	return &DataTester{cfg: cfg}
}

func (d *DataTester) Run(ctx context.Context, t *testing.T) ContractReport {
	t.Helper()
	return runContractCases(t, "data", []contractCase{
		{id: "TC-D01", name: "Request instruments", run: func() error {
			if d.cfg.Provider == nil {
				return fmt.Errorf("provider is nil")
			}
			if err := d.cfg.Provider.LoadAll(ctx); err != nil {
				return err
			}
			inst, ok := d.cfg.Provider.Get(d.cfg.InstrumentID)
			if !ok {
				return fmt.Errorf("instrument %s not found", d.cfg.InstrumentID)
			}
			return inst.Validate()
		}},
		{id: "TC-D02", name: "Fetch ticker", run: func() error {
			if d.cfg.Data == nil {
				return fmt.Errorf("data client is nil")
			}
			ticker, err := d.cfg.Data.FetchTicker(ctx, d.cfg.InstrumentID)
			if err != nil {
				return err
			}
			if ticker.InstrumentID != d.cfg.InstrumentID {
				return fmt.Errorf("ticker instrument mismatch: %s", ticker.InstrumentID)
			}
			return ticker.Validate()
		}},
		{id: "TC-D03", name: "Fetch order book", run: func() error {
			if d.cfg.Data == nil {
				return fmt.Errorf("data client is nil")
			}
			book, err := d.cfg.Data.FetchOrderBook(ctx, d.cfg.InstrumentID, 10)
			if err != nil {
				return err
			}
			if book.InstrumentID != d.cfg.InstrumentID {
				return fmt.Errorf("order book instrument mismatch: %s", book.InstrumentID)
			}
			return book.Validate()
		}},
		{id: "TC-D11", name: "Subscribe ticker", run: func() error {
			streaming, ok := d.cfg.Data.(venue.StreamingDataClient)
			if !ok {
				return skipCase("data client does not implement StreamingDataClient")
			}
			sub := model.SubscribeMarketData{
				InstrumentID: d.cfg.InstrumentID,
				Type:         model.MarketDataTypeTicker,
			}
			if err := d.subscribeMarketData(ctx, streaming, sub); err != nil {
				if errors.Is(err, model.ErrNotSupported) {
					return skipCase(err.Error())
				}
				return err
			}
			return d.unsubscribeMarketData(ctx, streaming, sub)
		}},
		{id: "TC-D12", name: "Subscribe book depth", run: func() error {
			streaming, ok := d.cfg.Data.(venue.StreamingDataClient)
			if !ok {
				return skipCase("data client does not implement StreamingDataClient")
			}
			sub := model.SubscribeMarketData{
				InstrumentID: d.cfg.InstrumentID,
				Type:         model.MarketDataTypeOrderBook,
				Depth:        10,
			}
			if err := d.subscribeMarketData(ctx, streaming, sub); err != nil {
				if errors.Is(err, model.ErrNotSupported) {
					return skipCase(err.Error())
				}
				return err
			}
			return d.unsubscribeMarketData(ctx, streaming, sub)
		}},
		{id: "TC-D13", name: "Subscribe trade ticks", run: func() error {
			streaming, ok := d.cfg.Data.(venue.StreamingDataClient)
			if !ok {
				return skipCase("data client does not implement StreamingDataClient")
			}
			sub := model.SubscribeMarketData{
				InstrumentID: d.cfg.InstrumentID,
				Type:         model.MarketDataTypeTradeTick,
			}
			if err := d.subscribeMarketData(ctx, streaming, sub); err != nil {
				if errors.Is(err, model.ErrNotSupported) {
					return skipCase(err.Error())
				}
				return err
			}
			return d.unsubscribeMarketData(ctx, streaming, sub)
		}},
		{id: "TC-D15", name: "Subscribe quote ticks", run: func() error {
			streaming, ok := d.cfg.Data.(venue.StreamingDataClient)
			if !ok {
				return skipCase("data client does not implement StreamingDataClient")
			}
			sub := model.SubscribeMarketData{
				InstrumentID: d.cfg.InstrumentID,
				Type:         model.MarketDataTypeQuoteTick,
			}
			if err := d.subscribeMarketData(ctx, streaming, sub); err != nil {
				if errors.Is(err, model.ErrNotSupported) {
					return skipCase(err.Error())
				}
				return err
			}
			return d.unsubscribeMarketData(ctx, streaming, sub)
		}},
		{id: "TC-D14", name: "Subscribe bars", run: func() error {
			streaming, ok := d.cfg.Data.(venue.StreamingDataClient)
			if !ok {
				return skipCase("data client does not implement StreamingDataClient")
			}
			barType := model.NewTimeBarType(d.cfg.InstrumentID, time.Minute)
			sub := model.SubscribeMarketData{
				InstrumentID: d.cfg.InstrumentID,
				Type:         model.MarketDataTypeBar,
				BarType:      barType,
			}
			if err := d.subscribeMarketData(ctx, streaming, sub); err != nil {
				if errors.Is(err, model.ErrNotSupported) {
					return skipCase(err.Error())
				}
				return err
			}
			return d.unsubscribeMarketData(ctx, streaming, sub)
		}},
	})
}

func (d *DataTester) subscribeMarketData(ctx context.Context, streaming venue.StreamingDataClient, sub model.SubscribeMarketData) error {
	return d.withTransientStreamRetry(ctx, func() error {
		return streaming.SubscribeMarketData(ctx, sub)
	})
}

func (d *DataTester) unsubscribeMarketData(ctx context.Context, streaming venue.StreamingDataClient, sub model.SubscribeMarketData) error {
	return d.withTransientStreamRetry(ctx, func() error {
		return streaming.UnsubscribeMarketData(ctx, sub)
	})
}

func (d *DataTester) withTransientStreamRetry(ctx context.Context, run func() error) error {
	var lastErr error
	for attempt := 0; attempt < marketDataStreamAttempts; attempt++ {
		err := run()
		if err == nil {
			return nil
		}
		if errors.Is(err, model.ErrNotSupported) || !testenv.IsTransientLiveNetworkError(err) {
			return err
		}
		lastErr = err
		if recoverErr := d.reconnectData(ctx); recoverErr != nil {
			return errors.Join(err, recoverErr)
		}
	}
	return lastErr
}

func (d *DataTester) reconnectData(ctx context.Context) error {
	if d.cfg.Data == nil {
		return nil
	}
	var reconnectErr error
	if err := d.cfg.Data.Disconnect(ctx); err != nil && !testenv.IsTransientLiveNetworkError(err) {
		reconnectErr = errors.Join(reconnectErr, err)
	}
	if err := d.cfg.Data.Connect(ctx); err != nil {
		reconnectErr = errors.Join(reconnectErr, err)
	}
	return reconnectErr
}
