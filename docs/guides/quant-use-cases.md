# Quant Developer Use Cases

This guide shows practical ways a quant developer can use the project. The
examples are intentionally small, but each one exercises a real platform
capability: normalized instruments, strategy runtime callbacks, risk checks,
portfolio accounting, backtest/live symmetry, adapter capability honesty, and
reconciliation.

The examples use repository model types. Treat them as best-practice shapes,
not as complete trading systems.

## Best Practices

- Express strategy logic in terms of `model.InstrumentID`, `model.AccountID`,
  normalized market events, and normalized order commands.
- Use `strategy.Runtime` for subscriptions, data requests, and order submission.
  Do not call exchange SDKs from strategy code.
- Create orders through `model.OrderFactory` so IDs, order-list metadata, and
  account identity remain consistent.
- Let `platform.Node` or `live.Node` route orders through risk, execution,
  cache, and portfolio.
- Use `cache.Cache` and `portfolio.Portfolio` for runtime state queries instead
  of keeping independent shadow state in a strategy.
- Check `venue.DeclaredCapabilities` and `docs/parity/adapter-capability-matrix.md`
  before depending on optional venue behavior.
- In backtests, use deterministic data and `Result.DeterministicJSON` when
  comparing outputs in tests or research pipelines.
- Use the [examples cookbook](../../examples/README.md) as the runnable version
  of these concepts.

## Use Case 1: Order Book Imbalance Strategy

Goal: subscribe to order-book depth, detect a simple bid/ask imbalance, submit
a limit order, and observe fill and portfolio state.

This is the smallest useful live-style flow:

1. Register a data client and execution client with a live node.
2. Strategy subscribes to order-book depth in `OnStart`.
3. `OnOrderBook` evaluates the signal.
4. Strategy creates a limit order with `OrderFactory`.
5. Runtime submits the order through risk and execution.
6. Cache and portfolio update from normalized execution events.

Minimal strategy shape:

```go
type ImbalanceStrategy struct {
    runtime      strategy.Runtime
    accountID    model.AccountID
    instrumentID model.InstrumentID
    submitted    bool
}

func (s *ImbalanceStrategy) OnStart(ctx context.Context, rt strategy.Runtime) error {
    s.runtime = rt
    return rt.SubscribeOrderBookDepth(ctx, s.instrumentID, 2)
}

func (s *ImbalanceStrategy) OnOrderBook(ctx context.Context, book model.OrderBook) error {
    if s.submitted || len(book.Bids) == 0 || len(book.Asks) == 0 {
        return nil
    }
    imbalanced := book.Bids[0].Size.GreaterThan(book.Asks[0].Size.Mul(decimal.NewFromInt(2)))
    if !imbalanced {
        return nil
    }
    s.submitted = true

    order := s.runtime.OrderFactory(s.accountID).Limit(
        book.InstrumentID,
        model.OrderSideBuy,
        decimal.RequireFromString("0.01"),
        book.Asks[0].Price,
    )
    _, err := s.runtime.SubmitOrder(ctx, order)
    return err
}
```

Node assembly shape:

```go
c := cache.New()
pf := portfolio.New(c)

node, err := live.NewTradingNode(live.NodeConfig{
    Cache:     c,
    Portfolio: pf,
    Risk: risk.NewEngine(c, risk.Config{
        MaxOrderNotional: decimal.RequireFromString("100"),
    }),
    DataClients:      []venue.DataClient{dataClient},
    ExecutionClients: []venue.ExecutionClient{executionClient},
    Strategies: []strategy.Strategy{
        strategy.NewTyped("imbalance", &ImbalanceStrategy{
            accountID:    "main",
            instrumentID: model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
        }),
    },
})
```

Best-practice notes:

- Put the notional limit in `risk.Config`, not in the strategy.
- Read the resulting order from `node.Cache()` and exposure from
  `node.Portfolio()`.
- Keep venue-specific code in the data/execution client or adapter, not in the
  strategy.

Runnable reference:
[06_run_live_node_with_in_memory_venue.go](../../examples/06_run_live_node_with_in_memory_venue.go).

## Use Case 2: Bracket Entry With Take Profit And Stop Loss

Goal: submit a bracket order list that enters only when the market touches an
entry price, then releases reduce-only take-profit and stop-loss children.

This demonstrates order-list metadata, contingent order handling, and backtest
and live lifecycle symmetry.

```go
func (s *BracketStrategy) OnOrderBook(ctx context.Context, book model.OrderBook) error {
    if s.submitted || !entryTouched(book) {
        return nil
    }
    s.submitted = true

    list := s.runtime.OrderFactory(s.accountID).Bracket(model.BracketOrderRequest{
        InstrumentID: book.InstrumentID,
        Side:         model.OrderSideBuy,
        Quantity:     decimal.RequireFromString("1"),
        EntryPrice:   decimal.RequireFromString("101"),
        TakeProfit:   decimal.RequireFromString("103"),
        StopLoss:     decimal.RequireFromString("99"),
    })

    _, err := s.runtime.SubmitOrderList(ctx, list)
    return err
}
```

Expected lifecycle:

1. Parent entry is submitted and accepted.
2. Children are held until the parent fills.
3. Take-profit and stop-loss children are released after entry fill.
4. When one child fills, the sibling is canceled.
5. Final position is flat.
6. Portfolio realized PnL reflects gross PnL minus fees.

Best-practice notes:

- Use `OrderFactory.Bracket` instead of hand-assembling unrelated orders.
- Keep `OrderListID`, `ParentClientOrderID`, contingency, and reduce-only
  fields intact.
- Listen to `OnOrderLifecycle` and `OnOrderFilled` when asserting strategy
  behavior.

Runnable reference:
[05_submit_bracket_order_backtest.go](../../examples/05_submit_bracket_order_backtest.go).

## Use Case 3: Deterministic Backtest For Strategy Research

Goal: run a small deterministic strategy test with timestamped market events,
then compare the resulting orders, fills, and positions.

```go
events := []backtest.Event{
    {
        At:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
        Topic: strategy.TopicMarketData,
        Message: model.MarketEvent{
            OrderBook: &model.OrderBook{
                InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
                Bids: []model.OrderBookLevel{{Price: decimal.RequireFromString("100"), Size: decimal.RequireFromString("3")}},
                Asks: []model.OrderBookLevel{{Price: decimal.RequireFromString("101"), Size: decimal.RequireFromString("1")}},
                Timestamp: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
            },
        },
    },
}

runner := backtest.NewRunner(backtest.Config{
    Cache:      cache.New(),
    Strategies: []strategy.Strategy{strategy.NewTyped("research", strategyImpl)},
    Events:     events,
    FillModel:  backtest.DefaultFillModel(),
})

result, err := runner.Run(ctx)
if err != nil {
    return err
}
snapshot, err := result.DeterministicJSON()
```

Best-practice notes:

- Keep event timestamps stable.
- Use deterministic fill-model configuration when comparing research runs.
- Use `Result.Summary` or `Result.DeterministicJSON` for repeatable assertions.
- Backtest strategies should use the same `strategy.Runtime` APIs as live
  strategies.

Runnable reference:
[04_run_strategy_backtest.go](../../examples/04_run_strategy_backtest.go).

## Use Case 4: Portfolio And Exposure Guardrails

Goal: enforce a notional limit before execution and query resulting exposure
after fills.

Risk is configured outside the strategy:

```go
riskEngine := risk.NewEngine(c, risk.Config{
    MaxOrderNotional:    decimal.RequireFromString("1000"),
    MaxAccountExposure:  decimal.RequireFromString("5000"),
    ExposureCurrency:    "USDT",
    MaxOpenOrders:       20,
    MaxCommandsPerWindow: 10,
    CommandRateWindow:   time.Second,
})
```

Strategy remains focused on intent:

```go
order := rt.OrderFactory(accountID).Limit(
    instrumentID,
    model.OrderSideBuy,
    decimal.RequireFromString("0.5"),
    decimal.RequireFromString("20000"),
)
report, err := rt.SubmitOrder(ctx, order)
```

After execution events update cache and portfolio:

```go
exposure := node.Portfolio().Exposure(accountID, "USDT")
unrealized := node.Portfolio().UnrealizedPnLs(accountID)
realized := node.Portfolio().RealizedPnLs(accountID)
positions := node.Cache().Positions(accountID)
```

Runnable references:
[03_validate_risk_before_execution.go](../../examples/03_validate_risk_before_execution.go)
and [06_run_live_node_with_in_memory_venue.go](../../examples/06_run_live_node_with_in_memory_venue.go).

Best-practice notes:

- Put guardrails in `risk.Config`; do not duplicate them in every strategy.
- Use `portfolio.Portfolio` for exposure and PnL queries.
- Feed mark prices through market events or explicit mark updates so unrealized
  PnL has a current reference price.
- Treat risk rejection as a normal, typed outcome.

## Use Case 5: Adapter Capability-Aware Live Setup

Goal: build live code that only relies on capabilities the adapter actually
declares.

```go
adapter, err := venue.Open(ctx, model.VenueBinance, map[string]string{
    "api_key":    os.Getenv("BINANCE_API_KEY"),
    "secret_key": os.Getenv("BINANCE_SECRET_KEY"),
    "account_id": "binance-main",
})
if err != nil {
    return err
}
defer adapter.Close(ctx)

caps := adapter.Capabilities()
if !caps.Execution.Submit || !caps.Execution.Cancel || !caps.Execution.OrderReports {
    return fmt.Errorf("adapter lacks required execution lifecycle capabilities")
}
if !caps.Execution.PrivateStream {
    return fmt.Errorf("adapter lacks private stream support for live lifecycle")
}

node, err := live.NewNodeBuilder().
    AddDataClient(adapter.Data()).
    AddExecutionClient(adapter.Execution()).
    AddStrategy(strategy.NewTyped("live-strategy", strategyImpl)).
    Build()
```

Best-practice notes:

- Use `Capabilities()` for live startup validation.
- For optional features such as modify, fill reports, position reports, mass
  status, or order lists, check the specific capability before calling.
- If a capability is false, design a fallback workflow explicitly or fail fast.
- Capability truth is tracked in `docs/parity/adapter-capability-matrix.md`.

## Use Case 6: Reconciliation After A Private Stream Gap

Goal: keep local state aligned with venue truth after startup or private stream
disconnects.

The lifecycle shape is:

1. Execution client connects.
2. Account snapshot and order reports load into cache.
3. Optional fill and position reports repair missing state.
4. Private stream resumes forwarding execution events.
5. Reconciliation audit trail records resolved and unresolved discrepancies.

Operational checks:

```go
health := node.Health()
if !health.Ready {
    return fmt.Errorf("node is not ready: %v", health.LastError)
}

for _, execHealth := range health.Execution {
    if !execHealth.Health.Connected || !execHealth.Health.AccountReady {
        return fmt.Errorf("execution stream not ready for account %s", execHealth.AccountID)
    }
}
```

Best-practice notes:

- Treat unresolved reconciliation discrepancies as state that must be observed.
- Do not continue live trading if private stream readiness is required and not
  available.
- Use explicit capability checks for fill reports, position reports, and mass
  status.
- Keep command IDs and client order IDs stable so repairs can deduplicate
  reports.

## Use Case 7: Funding Rate Arbitrage Across Venues

Goal: monitor perpetual funding rates across venues, short the venue paying the
highest funding to shorts, long the venue with the lowest funding cost, and
validate both hedge legs before execution.

Funding rates are standardized as `model.FundingRate` market data for perpetual
instruments. Strategies can subscribe with `strategy.Runtime.SubscribeFundingRates`,
receive `OnFundingRate`, request `model.MarketDataTypeFundingRate`, and replay
`model.MarketEvent{FundingRate: ...}` in backtests. The example wraps each
standard funding payload with account and fee metadata so the execution legs can
still route through `model.AccountID`, `model.OrderFactory`, `risk.Engine`, and
`venue.ExecutionClient`.

Live funding use must still check adapter capability truth first:
`caps.MarketData.FundingRates` for snapshots and
`caps.MarketData.FundingRateStream` for streaming.

Current funding snapshot providers are Binance Perp, Aster Perp, OKX Swap,
Hyperliquid Perp, Lighter, Nado, EdgeX, GRVT, and Backpack. Bybit Linear,
Bitget Perp, and StandX expose latest-known snapshots backed by venue funding
history. Strategies should validate non-zero mark/index prices and funding
intervals before using those fields in sizing or execution decisions.

Runnable reference:
[07_monitor_funding_rate_arbitrage.go](../../examples/07_monitor_funding_rate_arbitrage.go).

## Choosing A Development Path

| Goal | Start with | Why |
| --- | --- | --- |
| Research a signal | `backtest.Runner` | Deterministic data, fast assertions, no venue credentials. |
| Build strategy logic | `strategy.NewTyped` and `strategy.Runtime` | Same authoring shape works in backtest and live. |
| Wire a live bot | `live.NewNodeBuilder` | Centralizes data, execution, risk, portfolio, and health. |
| Add venue behavior | `sdk/` then `adapter/` | Keeps native protocol coverage separate from stable runtime contracts. |
| Rely on optional feature | `adapter.Capabilities()` and matrix | Prevents accidental dependence on unsupported venue behavior. |

## Verification Checklist

For strategy examples:

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./examples/...
```

For runtime behavior:

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./backtest ./strategy ./risk ./portfolio ./testsuite
```

For adapter capability assumptions:

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./venue ./testsuite ./adapter/... ./config/all -run 'Adapter|Capability|Contract|PrivateStream|Resubscribe'
```

For release-facing confidence:

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./testsuite -run 'Master|Score|Requirement'
```
