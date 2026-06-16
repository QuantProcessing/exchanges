package examples

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/risk"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestFetchTickerWithDataClient(t *testing.T) {
	instrumentID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	ticker, err := FetchTickerWithDataClient(context.Background(), newTickerExampleClient(instrumentID), instrumentID)
	require.NoError(t, err)
	require.Equal(t, "101", ticker.Last.String())
}

func TestBuildOrdersWithOrderFactory(t *testing.T) {
	result := BuildOrdersWithOrderFactory()
	require.Equal(t, model.OrderTypeMarket, result.Market.Type)
	require.Equal(t, model.ClientOrderID("intro-1"), result.Market.ClientOrderID)
	require.True(t, result.PostOnly.PostOnly)
	require.True(t, result.StopMarket.ReduceOnly)
	require.Equal(t, model.OrderTypeTrailingStopMarket, result.TrailingStop.Type)
	require.Len(t, result.Bracket.Orders, 3)
	require.Equal(t, result.Bracket.ID, result.Bracket.Orders[0].OrderListID)
	require.Equal(t, result.Bracket.Orders[0].ClientOrderID, result.Bracket.Orders[1].ParentClientOrderID)
}

func TestValidateRiskBeforeExecution(t *testing.T) {
	result, err := ValidateRiskBeforeExecution()
	require.NoError(t, err)
	require.Equal(t, model.ClientOrderID("risk-1"), result.Accepted.ClientOrderID)
	require.Equal(t, model.ClientOrderID("risk-2"), result.Rejected.ClientOrderID)
	require.Error(t, result.RejectErr)
	require.True(t, errors.Is(result.RejectErr, risk.ErrRiskRejected))
}

func TestRunThresholdStrategyBacktest(t *testing.T) {
	result, err := RunThresholdStrategyBacktest(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2, result.EventsProcessed)
	require.Equal(t, model.OrderStatusFilled, result.Order.Status)
	require.Equal(t, model.PositionSideLong, result.Position.Side)
	require.Len(t, result.Fills, 1)
	require.Equal(t, "1.01", result.ExposureUSDT.String())
}

func TestRunBracketOrderBacktest(t *testing.T) {
	result, err := RunBracketOrderBacktest(context.Background())
	require.NoError(t, err)
	require.Equal(t, model.OrderStatusFilled, result.Entry.Status)
	require.Equal(t, model.OrderStatusCanceled, result.StopLoss.Status)
	require.Equal(t, model.OrderStatusFilled, result.TakeProfit.Status)
	require.Len(t, result.Fills, 2)
}

func TestRunLiveNodeWithInMemoryVenue(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := RunLiveNodeWithInMemoryVenue(ctx)
	require.NoError(t, err)
	require.Equal(t, model.OrderStatusFilled, result.SubmittedOrder.Status)
	require.Len(t, result.Fills, 1)
	require.Equal(t, model.PositionSideLong, result.Position.Side)
	require.Equal(t, "1.01", result.ExposureUSDT.String())
	require.Contains(t, result.EventLog, "market:order_book")
	require.Contains(t, result.EventLog, "execution:order:filled")
}

func TestRunFundingRateArbitrageMonitor(t *testing.T) {
	decision, err := RunFundingRateArbitrageMonitor(context.Background())
	require.NoError(t, err)
	require.True(t, decision.ShouldTrade)
	require.Equal(t, model.Venue("BYBIT"), decision.Long.Venue)
	require.Equal(t, model.Venue("BINANCE"), decision.Short.Venue)
	require.True(t, decision.FundingSpread.Equal(decimal.RequireFromString("0.0013")))
	require.True(t, decision.EstimatedCostRate.Equal(decimal.RequireFromString("0.00027")))
	require.True(t, decision.ExpectedNetRate.Equal(decimal.RequireFromString("0.00103")))
	require.True(t, decision.ExpectedNetUSDT.Equal(decimal.RequireFromString("1.03")))
	require.Len(t, decision.Orders, 2)
	require.Equal(t, model.OrderSideSell, decision.Orders[0].Side)
	require.Equal(t, model.AccountID("binance-perp-main"), decision.Orders[0].AccountID)
	require.Equal(t, model.OrderSideBuy, decision.Orders[1].Side)
	require.Equal(t, model.AccountID("bybit-perp-main"), decision.Orders[1].AccountID)
	require.Len(t, decision.Reports, 2)
	require.Equal(t, model.OrderStatusAccepted, decision.Reports[0].Status)
	require.Equal(t, model.OrderStatusAccepted, decision.Reports[1].Status)
}

type tickerExampleClient struct {
	provider *memoryInstrumentProvider
	ticker   model.Ticker
}

func newTickerExampleClient(instrumentID model.InstrumentID) *tickerExampleClient {
	return &tickerExampleClient{
		provider: newMemoryInstrumentProvider(instrumentID),
		ticker: model.Ticker{
			InstrumentID: instrumentID,
			Bid:          decimal.RequireFromString("100"),
			Ask:          decimal.RequireFromString("102"),
			Last:         decimal.RequireFromString("101"),
			Timestamp:    time.Now(),
		},
	}
}

func (c *tickerExampleClient) Venue() model.Venue                    { return c.provider.instrument.ID.Venue }
func (c *tickerExampleClient) ClientID() string                      { return "ticker-example" }
func (c *tickerExampleClient) Instruments() venue.InstrumentProvider { return c.provider }
func (c *tickerExampleClient) Connect(context.Context) error         { return nil }
func (c *tickerExampleClient) Disconnect(context.Context) error      { return nil }
func (c *tickerExampleClient) Health() venue.DataHealth {
	return venue.DataHealth{Connected: true, InstrumentReady: true}
}
func (c *tickerExampleClient) FetchTicker(context.Context, model.InstrumentID) (model.Ticker, error) {
	return c.ticker, nil
}
func (c *tickerExampleClient) FetchOrderBook(context.Context, model.InstrumentID, int) (model.OrderBook, error) {
	return model.OrderBook{}, model.ErrNotSupported
}
