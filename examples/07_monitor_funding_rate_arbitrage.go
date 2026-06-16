package examples

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/QuantProcessing/exchanges/cache"
	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/risk"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/shopspring/decimal"
)

type FundingVenueSnapshot struct {
	Venue        model.Venue
	AccountID    model.AccountID
	RawSymbol    string
	Base         model.Currency
	Quote        model.Currency
	Funding      model.FundingRate
	TakerFeeRate decimal.Decimal
}

type FundingRateSource interface {
	Name() string
	Snapshots(context.Context) ([]FundingVenueSnapshot, error)
}

type FundingArbitrageConfig struct {
	Symbol             string
	Quantity           decimal.Decimal
	MinFundingSpread   decimal.Decimal
	MinNetEdge         decimal.Decimal
	SlippageBufferRate decimal.Decimal
	MaxOrderNotional   decimal.Decimal
}

type FundingArbitrageDecision struct {
	ShouldTrade       bool
	Reason            string
	Long              FundingVenueSnapshot
	Short             FundingVenueSnapshot
	FundingSpread     decimal.Decimal
	EstimatedCostRate decimal.Decimal
	ExpectedNetRate   decimal.Decimal
	ExpectedNetUSDT   decimal.Decimal
	Orders            []model.SubmitOrder
	Reports           []model.OrderStatusReport
}

// RunFundingRateArbitrageMonitor shows a production-shaped funding-rate
// arbitrage flow without depending on any live credentials:
//
//  1. collect funding snapshots from multiple venues;
//  2. find the venue paying the most to shorts and the venue charging longs the
//     least;
//  3. estimate taker-fee plus slippage cost;
//  4. create a delta-neutral long/short order pair with OrderFactory;
//  5. run risk checks before routing orders to execution clients by AccountID.
func RunFundingRateArbitrageMonitor(ctx context.Context) (FundingArbitrageDecision, error) {
	now := time.Date(2026, 6, 16, 8, 0, 0, 0, time.UTC)
	binance := FundingVenueSnapshot{
		Venue:     "BINANCE",
		AccountID: "binance-perp-main",
		RawSymbol: "BTCUSDT",
		Base:      "BTC",
		Quote:     "USDT",
		Funding: model.FundingRate{
			InstrumentID:    model.MustInstrumentID("BTC-USDT-PERP.BINANCE"),
			Rate:            decimal.RequireFromString("0.0012"),
			MarkPrice:       decimal.RequireFromString("50000"),
			NextFundingTime: now.Add(8 * time.Hour),
			FundingInterval: 8 * time.Hour,
			Timestamp:       now,
		},
		TakerFeeRate: decimal.RequireFromString("0.00012"),
	}
	bybit := FundingVenueSnapshot{
		Venue:     "BYBIT",
		AccountID: "bybit-perp-main",
		RawSymbol: "BTCUSDT",
		Base:      "BTC",
		Quote:     "USDT",
		Funding: model.FundingRate{
			InstrumentID:    model.MustInstrumentID("BTC-USDT-PERP.BYBIT"),
			Rate:            decimal.RequireFromString("-0.0001"),
			MarkPrice:       decimal.RequireFromString("50000"),
			NextFundingTime: now.Add(8 * time.Hour),
			FundingInterval: 8 * time.Hour,
			Timestamp:       now,
		},
		TakerFeeRate: decimal.RequireFromString("0.00010"),
	}

	riskCache := cache.New()
	for _, snapshot := range []FundingVenueSnapshot{binance, bybit} {
		if err := putFundingInstrumentAndMark(riskCache, snapshot); err != nil {
			return FundingArbitrageDecision{}, err
		}
	}

	router := newFundingExecutionRouter(
		newFundingExecutionClient(binance.AccountID, binance.Funding.InstrumentID),
		newFundingExecutionClient(bybit.AccountID, bybit.Funding.InstrumentID),
	)
	monitor := NewFundingArbitrageMonitor(
		FundingArbitrageConfig{
			Symbol:             "BTC-USDT-PERP",
			Quantity:           decimal.RequireFromString("0.02"),
			MinFundingSpread:   decimal.RequireFromString("0.0005"),
			MinNetEdge:         decimal.RequireFromString("0.0003"),
			SlippageBufferRate: decimal.RequireFromString("0.00005"),
			MaxOrderNotional:   decimal.RequireFromString("1500"),
		},
		[]FundingRateSource{
			staticFundingRateSource{name: "binance-funding", snapshots: []FundingVenueSnapshot{binance}},
			staticFundingRateSource{name: "bybit-funding", snapshots: []FundingVenueSnapshot{bybit}},
		},
		risk.NewEngine(riskCache, risk.Config{
			MaxOrderNotional: decimal.RequireFromString("1500"),
			ExposureCurrency: "USDT",
		}),
		router,
	)
	return monitor.EvaluateOnce(ctx)
}

type FundingArbitrageMonitor struct {
	cfg      FundingArbitrageConfig
	sources  []FundingRateSource
	risk     *risk.Engine
	executor *fundingExecutionRouter
}

func NewFundingArbitrageMonitor(cfg FundingArbitrageConfig, sources []FundingRateSource, riskEngine *risk.Engine, executor *fundingExecutionRouter) *FundingArbitrageMonitor {
	return &FundingArbitrageMonitor{cfg: cfg, sources: sources, risk: riskEngine, executor: executor}
}

func (m *FundingArbitrageMonitor) EvaluateOnce(ctx context.Context) (FundingArbitrageDecision, error) {
	snapshots, err := m.collect(ctx)
	if err != nil {
		return FundingArbitrageDecision{}, err
	}
	long, short, ok := bestFundingPair(m.cfg.Symbol, snapshots)
	if !ok {
		return FundingArbitrageDecision{Reason: "need at least two funding snapshots for the symbol"}, nil
	}

	spread := short.Funding.Rate.Sub(long.Funding.Rate)
	costRate := short.TakerFeeRate.Add(long.TakerFeeRate).Add(m.cfg.SlippageBufferRate)
	netRate := spread.Sub(costRate)
	notional := m.cfg.Quantity.Mul(short.Funding.MarkPrice.Add(long.Funding.MarkPrice).Div(decimal.NewFromInt(2)))
	decision := FundingArbitrageDecision{
		Long:              long,
		Short:             short,
		FundingSpread:     spread,
		EstimatedCostRate: costRate,
		ExpectedNetRate:   netRate,
		ExpectedNetUSDT:   notional.Mul(netRate),
	}
	if spread.LessThan(m.cfg.MinFundingSpread) {
		decision.Reason = "funding spread below threshold"
		return decision, nil
	}
	if netRate.LessThan(m.cfg.MinNetEdge) {
		decision.Reason = "net edge below threshold after taker fees and slippage"
		return decision, nil
	}

	orders := fundingArbitrageOrders(m.cfg, long, short)
	for _, order := range orders {
		if err := m.risk.Check(order); err != nil {
			decision.Reason = "risk rejected hedge order"
			return decision, err
		}
	}
	reports, err := m.executor.SubmitOrders(ctx, orders)
	if err != nil {
		return FundingArbitrageDecision{}, err
	}
	decision.ShouldTrade = true
	decision.Reason = "short high funding venue and long low funding venue"
	decision.Orders = orders
	decision.Reports = reports
	return decision, nil
}

func (m *FundingArbitrageMonitor) collect(ctx context.Context) ([]FundingVenueSnapshot, error) {
	var snapshots []FundingVenueSnapshot
	for _, source := range m.sources {
		sourceSnapshots, err := source.Snapshots(ctx)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", source.Name(), err)
		}
		snapshots = append(snapshots, sourceSnapshots...)
	}
	return snapshots, nil
}

func bestFundingPair(symbol string, snapshots []FundingVenueSnapshot) (long FundingVenueSnapshot, short FundingVenueSnapshot, ok bool) {
	var candidates []FundingVenueSnapshot
	for _, snapshot := range snapshots {
		if snapshot.Funding.InstrumentID.Symbol == symbol {
			candidates = append(candidates, snapshot)
		}
	}
	if len(candidates) < 2 {
		return FundingVenueSnapshot{}, FundingVenueSnapshot{}, false
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].Funding.Rate.LessThan(candidates[j].Funding.Rate)
	})
	return candidates[0], candidates[len(candidates)-1], true
}

func fundingArbitrageOrders(cfg FundingArbitrageConfig, long FundingVenueSnapshot, short FundingVenueSnapshot) []model.SubmitOrder {
	metadata := model.CommandMetadata{
		TraderID:      "funding-arb-trader",
		StrategyID:    "funding-rate-arbitrage",
		CorrelationID: model.CorrelationID(fmt.Sprintf("%s-vs-%s-%s", short.Venue, long.Venue, cfg.Symbol)),
		TsInit:        time.Date(2026, 6, 16, 8, 0, 0, 0, time.UTC),
		Params: map[string]string{
			"example":     "07_monitor_funding_rate_arbitrage",
			"short_venue": string(short.Venue),
			"long_venue":  string(long.Venue),
		},
	}
	shortFactory := model.NewOrderFactory(short.AccountID, model.WithClientOrderIDPrefix("funding-short"), model.WithOrderMetadata(metadata))
	longFactory := model.NewOrderFactory(long.AccountID, model.WithClientOrderIDPrefix("funding-long"), model.WithOrderMetadata(metadata))
	return []model.SubmitOrder{
		shortFactory.Market(short.Funding.InstrumentID, model.OrderSideSell, cfg.Quantity),
		longFactory.Market(long.Funding.InstrumentID, model.OrderSideBuy, cfg.Quantity),
	}
}

func putFundingInstrumentAndMark(c *cache.Cache, snapshot FundingVenueSnapshot) error {
	if err := c.PutInstrument(model.Instrument{
		ID:          snapshot.Funding.InstrumentID,
		RawSymbol:   snapshot.RawSymbol,
		Type:        model.InstrumentTypePerp,
		Base:        snapshot.Base,
		Quote:       snapshot.Quote,
		Settle:      snapshot.Quote,
		PriceTick:   decimal.RequireFromString("0.1"),
		SizeTick:    decimal.RequireFromString("0.001"),
		TakerFee:    snapshot.TakerFeeRate,
		MarginInit:  decimal.RequireFromString("0.01"),
		MarginMaint: decimal.RequireFromString("0.005"),
		Status:      model.InstrumentStatusTrading,
	}); err != nil {
		return err
	}
	if err := c.PutMarketEvent(model.MarketEvent{Ticker: &model.Ticker{
		InstrumentID: snapshot.Funding.InstrumentID,
		Bid:          snapshot.Funding.MarkPrice.Sub(decimal.RequireFromString("1")),
		Ask:          snapshot.Funding.MarkPrice.Add(decimal.RequireFromString("1")),
		Last:         snapshot.Funding.MarkPrice,
		Timestamp:    snapshot.Funding.Timestamp,
	}}); err != nil {
		return err
	}
	funding := snapshot.Funding
	return c.PutMarketEvent(model.MarketEvent{FundingRate: &funding})
}

type staticFundingRateSource struct {
	name      string
	snapshots []FundingVenueSnapshot
}

func (s staticFundingRateSource) Name() string { return s.name }
func (s staticFundingRateSource) Snapshots(context.Context) ([]FundingVenueSnapshot, error) {
	return append([]FundingVenueSnapshot(nil), s.snapshots...), nil
}

type fundingExecutionRouter struct {
	clients map[model.AccountID]venue.ExecutionClient
}

func newFundingExecutionRouter(clients ...venue.ExecutionClient) *fundingExecutionRouter {
	router := &fundingExecutionRouter{clients: make(map[model.AccountID]venue.ExecutionClient, len(clients))}
	for _, client := range clients {
		router.clients[client.AccountID()] = client
	}
	return router
}

func (r *fundingExecutionRouter) SubmitOrders(ctx context.Context, orders []model.SubmitOrder) ([]model.OrderStatusReport, error) {
	reports := make([]model.OrderStatusReport, 0, len(orders))
	for _, order := range orders {
		client, ok := r.clients[order.AccountID]
		if !ok {
			return reports, fmt.Errorf("no execution client for account %s", order.AccountID)
		}
		report, err := client.SubmitOrder(ctx, order)
		if err != nil {
			return reports, err
		}
		reports = append(reports, report)
	}
	return reports, nil
}

type fundingExecutionClient struct {
	accountID    model.AccountID
	instrumentID model.InstrumentID
	nextOrder    int
	events       chan model.ExecutionEvent
}

func newFundingExecutionClient(accountID model.AccountID, instrumentID model.InstrumentID) *fundingExecutionClient {
	return &fundingExecutionClient{accountID: accountID, instrumentID: instrumentID, events: make(chan model.ExecutionEvent)}
}

func (c *fundingExecutionClient) Venue() model.Venue               { return c.instrumentID.Venue }
func (c *fundingExecutionClient) AccountID() model.AccountID       { return c.accountID }
func (c *fundingExecutionClient) Connect(context.Context) error    { return nil }
func (c *fundingExecutionClient) Disconnect(context.Context) error { return nil }
func (c *fundingExecutionClient) Health() venue.ExecutionHealth {
	return venue.ExecutionHealth{Connected: true, AccountReady: true, LastEventTime: time.Now()}
}
func (c *fundingExecutionClient) QueryAccount(context.Context) (model.AccountSnapshot, error) {
	return model.AccountSnapshot{AccountID: c.accountID, Venue: c.instrumentID.Venue, Type: model.AccountTypeMargin}, nil
}
func (c *fundingExecutionClient) SubmitOrder(_ context.Context, order model.SubmitOrder) (model.OrderStatusReport, error) {
	c.nextOrder++
	return model.OrderStatusReport{
		Metadata:        order.Metadata,
		AccountID:       order.AccountID,
		InstrumentID:    order.InstrumentID,
		OrderID:         model.OrderID(fmt.Sprintf("funding-order-%d", c.nextOrder)),
		ClientOrderID:   order.ClientOrderID,
		Status:          model.OrderStatusAccepted,
		Side:            order.Side,
		Type:            order.Type,
		Quantity:        order.Quantity,
		LeavesQuantity:  order.Quantity,
		TimeInForce:     order.TimeInForce,
		LastUpdatedTime: time.Now(),
	}, nil
}
func (c *fundingExecutionClient) CancelOrder(_ context.Context, cancel model.CancelOrder) (model.OrderStatusReport, error) {
	return model.OrderStatusReport{
		AccountID:     cancel.AccountID,
		InstrumentID:  cancel.InstrumentID,
		OrderID:       cancel.OrderID,
		ClientOrderID: cancel.ClientOrderID,
		Status:        model.OrderStatusCanceled,
	}, nil
}
func (c *fundingExecutionClient) GenerateOrderStatusReports(context.Context, model.InstrumentID) ([]model.OrderStatusReport, error) {
	return nil, nil
}
func (c *fundingExecutionClient) Events() <-chan model.ExecutionEvent { return c.events }
