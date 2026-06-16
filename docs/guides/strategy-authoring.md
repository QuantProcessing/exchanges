# Strategy Authoring

Strategies are plain Go values wrapped by `strategy.NewTyped`. A strategy
receives a `strategy.Runtime` in `OnStart`, subscribes to data, reacts through
typed callbacks, creates orders with `model.OrderFactory`, and submits commands
through the runtime.

## Mental Model

```text
OnStart
  -> store strategy.Runtime
  -> subscribe to data

market callback
  -> evaluate signal
  -> create model.SubmitOrder or model.OrderList
  -> SubmitOrder / SubmitOrderList

execution callback
  -> observe order/fill/position lifecycle
  -> update strategy-local signal state only
```

Do not call venue SDKs from a strategy. Runtime submission is what keeps risk,
execution, cache, portfolio, event callbacks, and reconciliation on one path.

## Minimal Typed Strategy

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
    if book.InstrumentID != s.instrumentID || s.submitted {
        return nil
    }
    if len(book.Bids) == 0 || len(book.Asks) == 0 {
        return nil
    }

    bidSize := book.Bids[0].Size
    askSize := book.Asks[0].Size
    if !bidSize.GreaterThan(askSize.Mul(decimal.NewFromInt(2))) {
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

Register it with either a backtest runner or live node:

```go
strat := strategy.NewTyped("imbalance", &ImbalanceStrategy{
    accountID:    "main",
    instrumentID: model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
})
```

Runnable code: [04_run_strategy_backtest.go](../../examples/04_run_strategy_backtest.go)
and [06_run_live_node_with_in_memory_venue.go](../../examples/06_run_live_node_with_in_memory_venue.go).

## Supported Typed Callbacks

`strategy.NewTyped` dispatches only the methods your handler implements.

Market data callbacks:

- `OnTicker(context.Context, model.Ticker) error`
- `OnOrderBook(context.Context, model.OrderBook) error`
- `OnTradeTick(context.Context, model.TradeTick) error`
- `OnQuoteTick(context.Context, model.QuoteTick) error`
- `OnBar(context.Context, model.Bar) error`
- `OnFundingRate(context.Context, model.FundingRate) error`
- `OnCustomData(context.Context, model.CustomData) error`

Execution callbacks:

- `OnAccount(context.Context, model.AccountSnapshot) error`
- `OnOrderStatus(context.Context, model.OrderStatusReport) error`
- `OnOrderSubmitted`, `OnOrderAccepted`, `OnOrderPartiallyFilled`,
  `OnOrderCanceled`, `OnOrderRejected`
- `OnOrderLifecycle(context.Context, model.OrderLifecycleEvent) error`
- `OnOrderFilled(context.Context, model.FillReport) error`
- `OnPosition(context.Context, model.PositionStatusReport) error`
- `OnPositionLifecycle`, `OnPositionOpened`, `OnPositionChanged`,
  `OnPositionClosed`

Runtime callbacks:

- `OnTimer(context.Context, strategy.TimerEvent) error`
- `OnError(context.Context, strategy.ErrorEvent) error`
- `OnEvent(context.Context, bus.Envelope) error` for raw event access
- `OnStop(context.Context) error`

## Runtime API Checklist

Use `strategy.Runtime` for:

- `Cache()` and `Portfolio()` state queries;
- `Clock()` for runtime time;
- `SetTimer` and `CancelTimer`;
- market-data subscriptions, including `SubscribeFundingRates` for perpetual
  funding data;
- historical or catalog-backed data requests;
- `OrderFactory(accountID)`;
- submit, modify, cancel, batch-cancel, cancel-all, and query commands;
- account queries.

Funding rates are standardized as market data for perpetual instruments:

```go
func (s *FundingStrategy) OnStart(ctx context.Context, rt strategy.Runtime) error {
    s.runtime = rt
    return rt.SubscribeFundingRates(ctx, s.instrumentID)
}

func (s *FundingStrategy) OnFundingRate(ctx context.Context, funding model.FundingRate) error {
    response, err := s.runtime.RequestData(ctx, model.DataRequest{
        RequestID:    "last-funding",
        InstrumentID: funding.InstrumentID,
        Type:         model.MarketDataTypeFundingRate,
        Limit:        1,
    })
    _ = response
    return err
}
```

Backtests can replay `model.MarketEvent{FundingRate: ...}` through the same
callback and request path. Live usage still depends on the adapter declaring
`caps.MarketData.FundingRates` or `caps.MarketData.FundingRateStream`.

## Strategy State Rules

Good strategy-local state:

- signal flags, rolling windows, indicator values, and debouncing;
- IDs of orders the strategy intentionally owns;
- last time a signal fired.

State that should come from runtime:

- open orders;
- fills;
- positions;
- account balances;
- exposure;
- realized and unrealized PnL;
- venue stream health.

## Brackets And Order Lists

For entry plus take-profit/stop-loss workflows, use:

- [Strategy Authoring With Brackets](./strategy-authoring-bracket.md)
- [05_submit_bracket_order_backtest.go](../../examples/05_submit_bracket_order_backtest.go)

Order lists preserve parent-child relationships, OTO/OCO contingency metadata,
reduce-only children, and list identity.

## Verification

Run focused strategy and example tests:

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./strategy ./examples/... -v
```

For flows involving execution, risk, portfolio, or backtest behavior, add the
relevant package tests:

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./strategy ./backtest ./execution ./risk ./portfolio ./testsuite -v
```
