# SDK-Backed Nautilus-Go Cleanroom Design

## Goal

Keep `sdk/` as the venue-native protocol layer and rebuild the rest of the
repository as an idiomatic Go trading platform inspired by NautilusTrader. The
rewrite does not preserve the old root `exchanges.Exchange` semantics.

The completion target is a locally testable platform kernel plus SDK-backed
adapters for every exchange SDK currently present in the repository. Binance is
the reference shape, but the finished rollout includes Binance, Aster, OKX,
Bybit, Bitget, Hyperliquid, Lighter, Nado, EdgeX, GRVT, StandX, and Backpack.

## Kept And Rebuilt

Kept:

- `sdk/` and its exchange-native REST/WebSocket types.
- `internal/mbx`, `internal/wsdispatch`, and `internal/testenv` because SDK
  production code and SDK tests depend on them.
- `go.mod` and `go.sum`.

Rebuilt:

- `model`: pure normalized domain model.
- `venue`: ports for instrument providers, data clients, execution clients,
  and adapter registries.
- `bus`: typed topic fan-out.
- `cache`: normalized runtime state.
- `account`: execution-event reconciliation into cache.
- `platform`: node lifecycle for data, execution, account, bus, and strategy.
- `strategy`: strategy interface and event dispatch.
- `adapter/<venue>`: SDK-to-platform translators.
- `testsuite`: reusable contract tests.
- `config/all`: blank-import registration only.

## Architecture

```text
sdk/<venue>        -> internal SDK helpers only
model              -> decimal + stdlib
venue              -> model
bus                -> stdlib
cache              -> model
account            -> cache, model
strategy           -> model, bus
platform           -> account, bus, cache, model, strategy, venue
adapter/<venue>    -> model, venue, sdk/<venue>
testsuite          -> model, venue, platform, account, cache, bus
```

Adapters do not own platform state. They own only SDK clients, stream
connections, symbol translation, and venue-to-model mapping.

## Domain Model

The model package defines:

- `Venue`, `InstrumentID`, `AccountID`, `ClientOrderID`, `OrderID`, `TradeID`.
- `Instrument` with type, base/quote/settle, raw symbol, price tick, size tick,
  and status.
- `Ticker`, `OrderBook`, `OrderBookLevel`.
- `SubmitOrder`, `CancelOrder`, `OrderStatusReport`, `FillReport`,
  `PositionStatusReport`, `AccountSnapshot`, and `ExecutionEvent`.
- Typed errors such as `ErrInvalidInstrumentID`, `ErrInstrumentNotFound`, and
  `ErrNotSupported`.

All public financial quantities use `decimal.Decimal`.

## Venue Ports

Each exchange/product implements independent clients:

```go
type InstrumentProvider interface {
    LoadAll(context.Context) error
    Get(model.InstrumentID) (model.Instrument, bool)
    List() []model.Instrument
}

type DataClient interface {
    Venue() model.Venue
    ClientID() string
    Instruments() InstrumentProvider
    Connect(context.Context) error
    Disconnect(context.Context) error
    Health() DataHealth
    FetchTicker(context.Context, model.InstrumentID) (model.Ticker, error)
    FetchOrderBook(context.Context, model.InstrumentID, int) (model.OrderBook, error)
}

type ExecutionClient interface {
    Venue() model.Venue
    AccountID() model.AccountID
    Connect(context.Context) error
    Disconnect(context.Context) error
    Health() ExecutionHealth
    QueryAccount(context.Context) (model.AccountSnapshot, error)
    SubmitOrder(context.Context, model.SubmitOrder) (model.OrderStatusReport, error)
    CancelOrder(context.Context, model.CancelOrder) (model.OrderStatusReport, error)
    GenerateOrderStatusReports(context.Context, model.InstrumentID) ([]model.OrderStatusReport, error)
    Events() <-chan model.ExecutionEvent
}
```

Product-specific clients are required when spot/perp/futures/options protocols
or account state differ.

## Platform Runtime

`platform.Node` owns:

- cache
- bus
- data clients
- execution clients
- account reconcilers
- strategies

Startup order:

1. Load all instruments from every data client provider.
2. Store instruments in shared cache.
3. Connect data clients.
4. Query execution client account snapshots.
5. Connect execution clients.
6. Generate startup order reports per cached instrument.
7. Subscribe execution events and publish them to the bus.
8. Start strategies.

Shutdown order reverses startup and closes client lifecycles.

## Strategy Runtime

Strategies receive normalized platform events through the bus. They do not call
SDKs directly. The first strategy interface is intentionally small:

```go
type Strategy interface {
    ID() string
    OnStart(context.Context, Runtime) error
    OnEvent(context.Context, bus.Envelope) error
    OnStop(context.Context) error
}
```

`Runtime` exposes order submission through execution clients in later
milestones; the first milestone validates event delivery and lifecycle.

## Adapter Rollout

`adapter/binance` is the canonical reference adapter because the SDK already
has spot and perp REST/account/order/exchange-info types. The same object-level
pattern is applied to the remaining SDK-backed venues.

Each migrated adapter has:

- an instrument provider backed by the native SDK metadata endpoint;
- a data client for ticker and order book snapshots;
- an execution client for account snapshot, submit, cancel, and startup order
  reports;
- local tests using fake SDK interfaces plus the shared venue contract suite;
- constructors that wire real SDK clients so the adapter remains SDK-backed.

WebSocket private stream hardening is a second milestone, not hidden inside
the first one.

| Venue | Product Surface | SDK Metadata Source | Order Submission Anchor |
| --- | --- | --- | --- |
| Binance | spot, perp | exchange info | venue order id |
| Aster | spot, perp | exchange info | venue order id |
| OKX | spot, swap | instruments | venue order id |
| Bybit | spot, linear | category instruments | venue order id |
| Bitget | spot, perp | product-type instruments | venue order id |
| Hyperliquid | spot, perp | meta endpoints | venue order id |
| Lighter | perp | order book details | transaction hash/order id |
| Nado | perp | contract-v2 map | digest |
| EdgeX | perp | contract metadata | StarkEx order id |
| GRVT | perp | all instruments | signed single-leg order id |
| StandX | perp | symbol info | client order id until reconciliation |
| Backpack | perp | marketType-filtered markets | venue order id |

Unsupported product surfaces return `model.ErrNotSupported` instead of no-op
success. Backpack spot, Lighter spot, Nado spot, EdgeX spot, GRVT spot, and
StandX spot are therefore explicit future work rather than false capability
claims.

## Test Architecture

Contract tests are package-level and local:

- model validation tests
- bus fan-out tests
- cache apply tests
- platform startup/order/event strategy tests
- venue provider/data/execution client contract suites
- adapter tests with fake SDK interfaces for every migrated venue

Live SDK tests stay under `sdk/` and remain environment-sensitive.

## Success Criteria

The cleanroom rewrite is complete when:

- `go test ./model ./bus ./cache ./account ./venue ./platform ./strategy ./backtest ./live ./testsuite ./adapter/binance ./adapter/aster ./adapter/okx ./adapter/bybit ./adapter/bitget ./adapter/hyperliquid ./adapter/lighter ./adapter/nado ./adapter/edgex ./adapter/grvt ./adapter/standx ./adapter/backpack ./config/all -count=1` passes.
- `go test ./sdk/... -run '^$'` compiles SDK packages.
- `go test ./... -run '^$'` compiles every package without running live SDK
  network tests.
- `git diff --check` reports no whitespace errors.
- No old root `exchanges.Exchange` package files remain.
- All adapter capability claims reflect local contract-tested behavior.
