# Nautilus-Inspired V2 Adapter Redesign

## Purpose

This document defines a breaking V2 redesign for the exchanges repository. The
reference architecture is NautilusTrader, but the implementation target is
idiomatic Go.

The redesign adopts five Nautilus ideas:

- instruments are first-class and uniquely identify what is traded;
- adapters translate venue protocol into normalized domain events and reports;
- account state is event-driven and separate from portfolio aggregation;
- order execution is a lifecycle stream, not a synchronous RPC wrapper;
- capability claims are backed by certification evidence.

The redesign does not copy Nautilus mechanically. Nautilus uses a Python/Cython
and Rust engine architecture with class hierarchies and message engines. This
repository should use Go package boundaries, small interfaces, explicit structs,
context-aware I/O, and composition.

The local NautilusTrader reference clone is stored at:

```text
.omx/references/nautilus_trader
```

That directory is ignored by git and is only a search/reference copy.

## Source Evidence

NautilusTrader reference points used for this design:

- `docs/concepts/instruments/index.md`: market data, orders, positions,
  accounting, portfolio calculations, and adapter symbology all refer to
  `InstrumentId` and an instrument definition.
- `docs/concepts/accounting.md`: accounts use Cash, Margin, and Betting modes;
  `AccountBalance` has the `total == locked + free` invariant; `MarginBalance`
  separates per-instrument and account-wide scopes; `AccountState` application
  is replacement-style.
- `docs/developer_guide/adapters.md`: adapter development is dependency-driven:
  protocol client, symbol normalization, instrument provider, market data,
  execution, reconciliation, config/factories, and tests.
- `docs/concepts/options.md`: options are first-class instruments with chain
  subscriptions, Greeks, per-series chain managers, and snapshot/raw modes.
- `nautilus_trader/adapters/bybit/execution.py`: a concrete execution client
  wires instrument provider initialization, account state update, user data
  stream subscription, order/fill/status report generation, and reconciliation.

Repository-local facts shaping the design:

- `exchange.go` has useful capability-family ideas, but `Exchange` is still a
  broad base-symbol API.
- `market_ref.go` and `OrderParams.Market` are early instrument-aware steps,
  but `MarketRef` is too small to be the canonical traded object.
- `account/base_trade_client.go` already isolates lifecycle-critical behavior
  around account snapshots, private order streams, optional fill streams, and
  refresh.
- `account/order_flow.go` already treats execution as order/fill event fusion.
- `account/portfolio_account.go` already keeps multi-account aggregation above
  individual accounts.
- `testsuite/*` already contains shared order, lifecycle, and TradingAccount
  suites, but capability claims are static rather than certification records.

## Design Thesis

V2 should become an instrument-first Go trading integration library:

```text
model/
  Pure domain model: identifiers, instruments, money, market data, orders,
  fills, positions, account states, reports, and typed errors. Leaf package.

venue/
  Adapter contracts: instrument provider, symbol normalizer, market data
  client, execution client, subscriptions, capabilities, and registry.

account/
  Stateful trading lifecycle: cache, reconciliation, account state application,
  order flow, private stream readiness, and TradingAccount APIs.

portfolio/
  Multi-account aggregation: balances, margins, exposure, PnL, valuation gaps.

testsuite/
  Contract and certification suites.

adapter/<venue>/
  Concrete Go adapter package such as adapter/binance, adapter/bybit,
  adapter/okx. Keep the existing adapter layer import paths.

sdk/<venue>/
  Venue-native protocol layer: official REST/WS API types, signing, and wire
  semantics.
```

Dependency direction:

```text
sdk/<venue>       -> standard library / transport helpers only
model             -> decimal + standard library only
venue             -> model
adapter/<venue>   -> sdk/<venue>, model, venue
account           -> model, venue
portfolio         -> model, account
testsuite         -> model, venue, account
```

No package should import concrete adapter packages except examples, tests, and
the optional registry wiring under `config/all`.

## Go-Specific Design Rules

These rules are part of the design, not style preferences.

- Use small interfaces owned by the consumer package. For example, `account`
  should depend on the smallest execution interface it needs, not on a giant
  adapter facade.
- Prefer explicit structs and composition. Do not create base classes or large
  embedded interface hierarchies to mimic Nautilus engines.
- Every network or blocking method accepts `context.Context`.
- Every long-lived client or subscription has an idempotent `Close() error` or
  `Stop(ctx context.Context) error`.
- Financial values use `decimal.Decimal`; no `float64` in public trading
  models.
- Public constructors validate invariants and return `(T, error)`. `Must*`
  helpers are allowed only for tests and examples.
- Use `errors.Is` and `errors.As` compatible sentinel and typed errors.
- Use channels only where streaming is intrinsic. The sender owns channel close.
  Backpressure behavior must be explicit.
- Use optional interfaces for optional behavior. Unsupported functionality
  returns `ErrNotSupported` and must not no-op successfully.
- Avoid `common`, `utils`, and broad packages in new code. Use focused files
  inside a package, or `internal/` packages for venue-private helpers.
- Avoid package names that collide with standard-library packages. In
  particular, do not create a top-level `runtime/` package; this design keeps
  lifecycle runtime in `account/`.
- Keep import paths meaningful even when package names are short. An import path
  such as `adapter/binance` can still expose `package binance`; that is
  acceptable Go and preserves the repository's SDK / adapter / account layers.
- Do not hide credentials in global state. Config structs receive credentials
  explicitly; helper constructors may read environment variables by explicit
  name.
- Keep zero values understandable, but do not pretend complex trading objects
  are valid when required fields are missing. Provide `Validate()` where useful.

## Non-Goals

- Do not preserve source compatibility with the current `Exchange` interface.
- Do not add every venue endpoint to the normalized adapter API.
- Do not put strategy logic, smart order routing, or portfolio policy inside
  adapters.
- Do not use string-only symbols as the primary V2 API.
- Do not model options as special strings inside spot/perp adapters.
- Do not let adapter capability claims exist without certification metadata.

## Core Domain Model

### Venue

`Venue` is a stable exchange identifier in `model`.

```go
package model

type Venue string

const (
    VenueBinance Venue = "BINANCE"
    VenueOKX     Venue = "OKX"
    VenueBybit   Venue = "BYBIT"
)
```

### InstrumentID

`InstrumentID` is the canonical identity for anything tradable or subscribable.
It replaces base-symbol strings as the primary API input.

```go
package model

type InstrumentID struct {
    Symbol string
    Venue  Venue
}

func ParseInstrumentID(s string) (InstrumentID, error)
func MustInstrumentID(s string) InstrumentID
func (id InstrumentID) String() string
func (id InstrumentID) Validate() error
```

Examples:

```text
BTC-USDT-SPOT.BINANCE
BTC-USDT-PERP.BINANCE
BTC-20260626-100000-C.BYBIT
BTC-20260626-100000-P.BYBIT
BTC-USDC-PERP.HYPERLIQUID
```

The symbol component is normalized by each venue adapter. The `{symbol, venue}`
pair must be unique inside the process.

### Currency and Money

V2 should stop restricting quote currencies to a fixed enum.

```go
package model

type Currency string

type Money struct {
    Amount   decimal.Decimal
    Currency Currency
}

func NewMoney(amount decimal.Decimal, currency Currency) (Money, error)
func (m Money) Validate() error
```

Known constants remain useful:

```go
const (
    USDT Currency = "USDT"
    USDC Currency = "USDC"
    DUSD Currency = "DUSD"
    USD  Currency = "USD"
    BTC  Currency = "BTC"
    ETH  Currency = "ETH"
)
```

The type must accept venue-specific and chain-native assets without code
changes.

### Instrument

`Instrument` is a tagged struct. Go should not model this with a complicated
generic union. The type tag plus optional subtype specs are easier to validate,
serialize, and evolve.

```go
package model

type InstrumentType string

const (
    InstrumentTypeCurrencyPair InstrumentType = "currency_pair"
    InstrumentTypeCryptoPerp   InstrumentType = "crypto_perp"
    InstrumentTypeCryptoFuture InstrumentType = "crypto_future"
    InstrumentTypeCryptoOption InstrumentType = "crypto_option"
    InstrumentTypeOptionSpread InstrumentType = "option_spread"
    InstrumentTypeBinaryOption InstrumentType = "binary_option"
    InstrumentTypeSynthetic    InstrumentType = "synthetic"
)

type Instrument struct {
    ID          InstrumentID
    RawSymbol   string
    Type        InstrumentType
    Base        Currency
    Quote       Currency
    Settle      Currency
    PriceStep   decimal.Decimal
    SizeStep    decimal.Decimal
    PricePrec   int32
    SizePrec    int32
    Multiplier  decimal.Decimal
    MinQty      decimal.Decimal
    MaxQty      decimal.Decimal
    MinNotional Money
    MaxNotional Money
    MakerFee    decimal.Decimal
    TakerFee    decimal.Decimal
    MarginInit  decimal.Decimal
    MarginMaint decimal.Decimal
    Option      *OptionSpec
    Spread      *SpreadSpec
    Metadata    Metadata
    EventTime   time.Time
    InitTime    time.Time
}

func (i Instrument) Validate() error
func (i Instrument) MakePrice(v decimal.Decimal) (decimal.Decimal, error)
func (i Instrument) MakeQty(v decimal.Decimal) (decimal.Decimal, error)
```

`MakePrice` and `MakeQty` should round or reject according to an explicit
rounding policy. The default should reject invalid precision instead of silently
rounding live orders.

### Options

Options are instruments, not adapter modes.

```go
package model

type OptionKind string

const (
    OptionKindCall OptionKind = "CALL"
    OptionKindPut  OptionKind = "PUT"
)

type OptionSpec struct {
    Underlying InstrumentID
    Strike     decimal.Decimal
    Kind       OptionKind
    Expiration time.Time
    Exercise   ExerciseStyle
    Settlement SettlementStyle
    IsInverse  bool
    IsQuanto   bool
}
```

This solves spot/perp/option ambiguity directly: the product type is a field on
the instrument, not a naming convention inferred by user code.

### Supporting Types

The first V2 pass should define the minimum supporting types needed by the
interfaces above. Keep them plain and serializable.

```go
package model

type Metadata map[string]string

type ExerciseStyle string

const (
    ExerciseStyleEuropean ExerciseStyle = "european"
    ExerciseStyleAmerican ExerciseStyle = "american"
)

type SettlementStyle string

const (
    SettlementStyleCash     SettlementStyle = "cash"
    SettlementStylePhysical SettlementStyle = "physical"
)

type OptionSeriesID struct {
    Venue      Venue
    Underlying InstrumentID
    Expiration time.Time
    Settle     Currency
}

type SpreadLeg struct {
    InstrumentID InstrumentID
    Ratio        decimal.Decimal
}

type SpreadSpec struct {
    Legs []SpreadLeg
}

type OptionGreeks struct {
    InstrumentID    InstrumentID
    Delta           decimal.Decimal
    Gamma           decimal.Decimal
    Vega            decimal.Decimal
    Theta           decimal.Decimal
    MarkIV          decimal.Decimal
    UnderlyingPrice decimal.Decimal
    EventTime       time.Time
}

type OptionChainEntry struct {
    Instrument InstrumentID
    Bid        decimal.Decimal
    Ask        decimal.Decimal
    Greeks     *OptionGreeks
}

type OptionChainSlice struct {
    Series    OptionSeriesID
    Entries   []OptionChainEntry
    EventTime time.Time
}
```

For environment selection, use venue-local config types. Do not put every venue
environment into the shared `model` package.

```go
package binance

type Environment string

const (
    EnvironmentProduction Environment = "production"
    EnvironmentTestnet    Environment = "testnet"
)
```

## Venue Adapter Contracts

The `venue` package defines contracts. Concrete packages under
`adapter/<venue>` implement them.

### Adapter Facade

Keep the public facade small and compositional:

```go
package venue

type Adapter interface {
    Venue() model.Venue
    Instruments() InstrumentProvider
    MarketData() MarketDataClient
    Execution() ExecutionClient
    Capabilities() DeclaredCapabilities
    Close() error
}
```

If a venue supports only market data, `Execution()` returns nil and the declared
capabilities say so. Callers that require execution should fail early.

### InstrumentProvider

The instrument provider loads and caches instruments. It is mandatory for market
data and execution.

```go
package venue

type InstrumentProvider interface {
    LoadAll(ctx context.Context) error
    Load(ctx context.Context, id model.InstrumentID) (model.Instrument, error)
    Find(ctx context.Context, q InstrumentQuery) ([]model.Instrument, error)
    Get(id model.InstrumentID) (model.Instrument, bool)
    List() []model.Instrument
}
```

Order and subscription methods must require the instrument to exist in the
provider cache. If it does not, return `model.ErrInstrumentNotLoaded` rather
than guessing from a string.

`InstrumentQuery` should support venue, type, base, quote, settle, underlying,
expiration range, strike range, and option kind.

### Symbol Normalizer

Venue-native symbols remain adapter-local.

```go
package venue

type SymbolNormalizer interface {
    ToInstrumentID(raw string, hint ProductHint) (model.InstrumentID, error)
    ToVenueSymbol(id model.InstrumentID) (string, error)
}
```

Examples:

- Binance spot `BTCUSDT` -> `BTC-USDT-SPOT.BINANCE`
- Binance USD-M perp `BTCUSDT` -> `BTC-USDT-PERP.BINANCE`
- Bybit option raw symbol -> `BTC-20260626-100000-C.BYBIT`
- OKX swap `BTC-USDT-SWAP` -> `BTC-USDT-PERP.OKX`

This is the only place product suffix rules belong.

### MarketDataClient

Market data is instrument-first.

```go
package venue

type MarketDataClient interface {
    FetchTicker(ctx context.Context, id model.InstrumentID) (model.Ticker, error)
    FetchOrderBook(ctx context.Context, id model.InstrumentID, limit int) (model.OrderBook, error)
    FetchTrades(ctx context.Context, id model.InstrumentID, q TradeQuery) ([]model.Trade, error)
    FetchBars(ctx context.Context, id model.InstrumentID, spec model.BarSpec, q BarQuery) ([]model.Bar, error)

    SubscribeTicker(ctx context.Context, id model.InstrumentID, h TickerHandler) (Subscription, error)
    SubscribeOrderBook(ctx context.Context, id model.InstrumentID, depth int, h OrderBookHandler) (Subscription, error)
    SubscribeTrades(ctx context.Context, id model.InstrumentID, h TradeHandler) (Subscription, error)
    SubscribeBars(ctx context.Context, id model.InstrumentID, spec model.BarSpec, h BarHandler) (Subscription, error)
}
```

Subscription shape:

```go
type Subscription interface {
    ID() string
    Close() error
    Done() <-chan struct{}
    Err() error
}
```

The adapter owns goroutines and channel closure. `Close` must be idempotent.

### OptionMarketDataClient

Options are optional behavior through a separate interface.

```go
package venue

type OptionMarketDataClient interface {
    FetchOptionChain(ctx context.Context, series model.OptionSeriesID, q ChainQuery) ([]model.Instrument, error)
    SubscribeOptionGreeks(ctx context.Context, id model.InstrumentID, h OptionGreeksHandler) (Subscription, error)
    SubscribeOptionChain(ctx context.Context, req OptionChainSubscription, h OptionChainHandler) (Subscription, error)
}
```

Option chain management should be separate from the venue transport. The venue
data client emits quotes and Greeks; an account- or market-data-level chain
manager aggregates per series.

### ExecutionClient

Execution is event/report oriented.

```go
package venue

type ExecutionClient interface {
    AccountID() model.AccountID
    Venue() model.Venue

    Connect(ctx context.Context) error
    Disconnect(ctx context.Context) error
    Health() ExecutionHealth

    SubmitOrder(ctx context.Context, cmd model.SubmitOrder) error
    ModifyOrder(ctx context.Context, cmd model.ModifyOrder) error
    CancelOrder(ctx context.Context, cmd model.CancelOrder) error
    CancelAllOrders(ctx context.Context, cmd model.CancelAllOrders) error

    QueryAccount(ctx context.Context) error
    GenerateOrderStatusReports(ctx context.Context, q OrderStatusQuery) ([]model.OrderStatusReport, error)
    GenerateFillReports(ctx context.Context, q FillQuery) ([]model.FillReport, error)
    GeneratePositionStatusReports(ctx context.Context, q PositionQuery) ([]model.PositionStatusReport, error)

    Events() <-chan model.ExecutionEvent
}
```

`SubmitOrder` does not return the final order. It accepts a command and emits
events such as submitted, accepted, rejected, partially filled, filled,
canceled, expired, and updated.

For convenience, `account.TradingAccount.Submit` may return an `OrderFlow`, but
the adapter contract itself stays event/report oriented.

## Account State

Adapters translate venue REST/WS account responses into account state events.

```go
package model

type AccountType string

const (
    AccountTypeCash    AccountType = "cash"
    AccountTypeMargin  AccountType = "margin"
    AccountTypeBetting AccountType = "betting"
)

type AccountState struct {
    AccountID    AccountID
    Venue        Venue
    Type         AccountType
    BaseCurrency Currency
    Reported     bool
    Balances     []AccountBalance
    Margins      []MarginBalance
    Positions    []PositionStatusReport
    Metadata     Metadata
    EventID      string
    EventTime    time.Time
    InitTime     time.Time
}
```

Balances:

```go
type AccountBalance struct {
    Total  Money
    Locked Money
    Free   Money
}

func NewBalance(total, locked, free Money) (AccountBalance, error)
func BalanceFromTotalAndFree(total, free Money) (AccountBalance, error)
func BalanceFromTotalAndLocked(total, locked Money) (AccountBalance, error)
```

The invariant is `total == locked + free` for a single currency.

Margins:

```go
type MarginBalance struct {
    Initial     Money
    Maintenance Money
    Instrument  *InstrumentID
}
```

If `Instrument` is nil, the margin is account-wide and keyed by collateral
currency. If non-nil, it is per-instrument. The account package must keep those
stores separate.

`AccountState` application semantics are replacement-style:

- incoming balances replace all balances for the account;
- incoming account-wide margins replace account-wide margins for the account;
- incoming per-instrument margins replace per-instrument margins for the account;
- partial venue updates must either be expanded to a full state before emit or
  emitted later as a different event type such as `AccountDelta`.

V2 should start with full-state events only.

## Account Lifecycle Runtime

The current `account/` package already points in the right direction. V2 should
make it the explicit lifecycle runtime instead of creating a new `runtime/`
package.

### Cache

The account cache stores:

- instruments;
- latest market data required for account valuation;
- open and closed orders;
- fills;
- positions;
- account states;
- stream health;
- reconciliation status.

Strategy-facing reads should go through `account` and `portfolio` APIs, not
directly through adapter REST methods.

### TradingAccount

Use one account runtime plus typed helper facades.

```go
package account

type TradingAccount struct {
    accountID model.AccountID
    exec      venue.ExecutionClient
    cache     *Cache
}
```

Typed facades:

- `CashAccount` for spot/cash behavior;
- `MarginAccount` for perp/future/options margin behavior;
- `OptionAccount` for option-specific helpers and Greek-aware position queries.

The account runtime owns:

- startup snapshot;
- private stream connection;
- order/fill fusion;
- account state application;
- periodic and reconnect reconciliation;
- stream health;
- readiness gates for orders, fills, positions, balances, and margins.

### OrderFlow

Keep the current `OrderFlow` concept, but make it report-driven.

```go
package account

type OrderFlow struct {
    ClientOrderID model.ClientOrderID
    InstrumentID  model.InstrumentID
}
```

It should expose latest order status, fills, terminal wait, cancellation wait,
rejection reason, and a diagnostic event stream.

### Reconciliation

Reconciliation is a required V2 concept, not a background convenience.

Startup sequence:

1. Load required instruments.
2. Query account state.
3. Generate order status reports.
4. Generate fill reports for active or configured lookback instruments.
5. Generate position reports.
6. Apply reports into cache.
7. Connect private streams.
8. Mark the account ready only after snapshot plus stream readiness.

Reconnect sequence:

1. Mark affected streams stale.
2. Reconnect private streams.
3. Generate reports since the last known event time.
4. Apply reports.
5. Mark streams ready.

Adapters that cannot generate reports may still be usable, but they cannot be
certified as lifecycle-ready.

## Portfolio

Portfolio remains above accounts and adapters.

V2 portfolio should support:

- account lookup by venue and account id;
- balances by currency;
- margins by currency and instrument;
- positions by instrument and account;
- open orders by instrument and account;
- realized and unrealized PnL by currency;
- mark value and net exposure by instrument;
- missing-price tracking for valuation gaps.

No smart order routing in the first V2 pass. Routing policy requires a separate
design.

## Capabilities and Certification

Replace single static capability bools with three records.

### DeclaredCapabilities

What the adapter package claims by construction.

```go
package venue

type DeclaredCapabilities struct {
    Venue           model.Venue
    AccountTypes    []model.AccountType
    InstrumentTypes []model.InstrumentType
    MarketData      MarketDataCapabilities
    Execution       ExecutionCapabilities
    AccountState    AccountStateCapabilities
    Reconciliation  ReconciliationCapabilities
}
```

### CertifiedCapabilities

What has passed test suites for a venue/account/product/environment.

```go
package venue

type CertifiedCapabilities struct {
    Venue          model.Venue
    AccountType    model.AccountType
    InstrumentType model.InstrumentType
    Environment    string
    TestRunID      string
    Suites         []CertifiedSuite
    CertifiedAt    time.Time
}
```

### RuntimeHealth

What is currently working.

```go
package venue

type RuntimeHealth struct {
    Connected        bool
    AccountReady     bool
    MarketStreams    map[model.InstrumentID]StreamHealth
    ExecutionStreams map[string]StreamHealth
    LastAccountState time.Time
    LastOrderEvent   time.Time
    LastFillEvent    time.Time
    LastReconcile    time.Time
    Errors           []RuntimeError
}
```

Users should not confuse these:

- declared means available in code;
- certified means verified by tests;
- runtime means alive now.

## Concrete Adapter Package Shape

Keep concrete adapters under the existing adapter layer:

```text
adapter/binance/
  config.go
  adapter.go
  instruments.go
  market_data.go
  execution.go
  account_state.go
  streams.go
  mappers.go
  capabilities.go
  internal/
    symbol/
    urls/
    retry/
```

Do not create `common/` unless it is unavoidable. In Go, a file such as
`symbol.go` inside the venue package is often enough. Use `internal/` only when
private helpers become a real subdomain.

Constructor shape:

```go
package binance

type Config struct {
    APIKey     string
    SecretKey  string
    Account    model.AccountType
    Environment Environment
    HTTPBaseURL string
    WSBaseURL   string
}

func (c Config) Validate() error

func New(ctx context.Context, cfg Config) (*Adapter, error)
```

`New` should construct clients and validate config. It should not load all
instruments or connect private streams unless explicitly documented. Expensive
I/O belongs in `LoadAll`, `Connect`, or `account.TradingAccount.Start`.

### Root Package Policy

The module root should not remain a large cross-exchange interface package.
During V2 it can temporarily hold compatibility aliases and migration docs, but
the long-term public imports should be explicit:

```go
import (
    "github.com/QuantProcessing/exchanges/account"
    "github.com/QuantProcessing/exchanges/adapter/binance"
    "github.com/QuantProcessing/exchanges/model"
    "github.com/QuantProcessing/exchanges/venue"
)
```

Avoid a new root-level `Exchange` replacement unless a real consumer needs it.
The useful abstraction is the component set in `venue`, not a single large
facade.

## Testing and Certification

V2 testing should be contract-driven.

### Model Tests

- `InstrumentID` parsing and formatting.
- Symbol normalizer round trips.
- Instrument precision and increment validation.
- Account balance invariants.
- Margin balance scope routing.
- Account state replacement semantics.

### Adapter Unit Tests

- Venue error mapping.
- Venue symbol parsing.
- Wire model mapping.
- Request construction.
- Signature generation.
- Unsupported features returning `ErrNotSupported`.

Unit tests may use local fixtures. They do not prove official endpoint
availability.

### SDK Tests

Continue current repository policy:

- public read methods call real official endpoints by default;
- private read methods skip only when credentials are missing;
- write methods require explicit live-write flags.

### Adapter Contract Tests

Public data certification:

- instrument provider load;
- ticker/orderbook/trades/bars by instrument id;
- subscription start/stop;
- precision and min-notional behavior.

Execution certification:

- submit/cancel/modify;
- accepted/rejected event mapping;
- partial fill;
- terminal fill;
- cancel terminal state;
- order status reports;
- fill reports;
- position reports where applicable.

Account certification:

- account state snapshot;
- balance invariant;
- margin scope correctness;
- cash vs margin account behavior;
- full replacement semantics.

Lifecycle certification:

- startup reconciliation;
- private stream readiness;
- order flow wait;
- disconnect/reconnect reconciliation;
- stale stream health.

Option certification:

- option instrument provider;
- option chain query;
- per-instrument Greeks;
- option chain aggregation;
- option order placement where venue supports it;
- option position reporting.

## Migration Strategy

This is intentionally breaking. The migration should still be staged so every
phase leaves the repository testable.

### Phase 0: Freeze V1 Behavior

Add tests around current behavior that informs deliberate breakage:

- current base-symbol parsing;
- existing account snapshot shape;
- existing TradingAccount startup;
- current capability claims;
- quote-aware `MarketRef` behavior.

These tests can be deleted or rewritten when V2 phases replace the behavior.

### Phase 1: Add V2 Model Package

Create `model/` with:

- identifiers;
- currency and money;
- instruments;
- account balances and margins;
- account state events;
- orders, fills, reports;
- market data events.

No adapter migration yet.

### Phase 2: Add Venue Contracts

Create `venue/` with:

- `Adapter`;
- `InstrumentProvider`;
- `SymbolNormalizer`;
- `MarketDataClient`;
- `OptionMarketDataClient`;
- `ExecutionClient`;
- `Subscription`;
- declared/certified/runtime capability records.

No concrete venue should need to import `account` or `portfolio`.

### Phase 3: Instrument Catalog on One Venue

Migrate one venue first, preferably Binance because it has spot, perp, and
existing instrument-aware tests.

Exit criteria:

- `BTC-USDT-SPOT.BINANCE` and `BTC-USDC-SPOT.BINANCE` can coexist;
- `BTC-USDT-PERP.BINANCE` and spot are distinct instruments;
- order validation uses instrument definitions, not ad hoc symbol detail cache.

### Phase 4: Execution Event Model

Introduce V2 execution reports and events alongside V1 methods if needed for
the transition.

Migrate the current `OrderFlow` to consume V2 reports/events while V1 adapters
still exist behind a shim.

### Phase 5: AccountState Runtime

Introduce `AccountState`, balance/margin stores, and full-state application.

Replace `FetchAccount` as the primary runtime primitive with:

```go
QueryAccount(ctx) error
Events() <-chan model.ExecutionEvent
```

User-facing account queries read cached state.

### Phase 6: Reconciliation Contract

Add required report-generation methods and tests.

Adapters that cannot produce order/fill/position reports remain market-data or
basic-execution capable, but not lifecycle certified.

### Phase 7: Options First-Class

Replace ad hoc option surfaces with option instruments and option market data.

Implement:

- `OptionSpec`;
- `OptionSeriesID`;
- option chain query;
- Greeks events;
- option chain manager;
- option account state and positions.

### Phase 8: Remove V1 Interfaces

Remove or move legacy APIs:

- root `Exchange`;
- base-symbol `FetchTicker(ctx, "BTC")` as core contract;
- public `FormatSymbol` / `ExtractSymbol` adapter APIs;
- static-only `Capabilities` as user-facing truth.

If convenience helpers remain, they should resolve an `InstrumentID` through the
instrument provider.

## Concrete API Sketch

Example V2 usage:

```go
ctx := context.Background()

adp, err := binance.New(ctx, binance.Config{
    APIKey:    "...",
    SecretKey: "...",
    Account:   model.AccountTypeMargin,
})
if err != nil {
    return err
}
defer adp.Close()

if err := adp.Instruments().LoadAll(ctx); err != nil {
    return err
}

id := model.MustInstrumentID("BTC-USDT-PERP.BINANCE")
instrument, ok := adp.Instruments().Get(id)
if !ok {
    return model.ErrInstrumentNotLoaded
}

acct := account.NewTradingAccount(adp.Execution(), account.Config{})
if err := acct.Start(ctx); err != nil {
    return err
}
defer acct.Stop(context.Background())

qty, err := instrument.MakeQty(decimal.RequireFromString("0.001"))
if err != nil {
    return err
}

flow, err := acct.Submit(ctx, model.SubmitOrder{
    InstrumentID: id,
    Side:         model.OrderSideBuy,
    Type:         model.OrderTypeMarket,
    Quantity:     qty,
    ClientID:     model.NewClientOrderID(),
})
if err != nil {
    return err
}

filled, err := flow.WaitFilled(ctx)
if err != nil {
    return err
}

_ = filled
```

Spot and option orders use the same execution path with different instruments.

## Design Decisions

### Decision 1: Package Layout

Use `model/`, `venue/`, `account/`, `portfolio/`, `sdk/<venue>`, and the
existing `adapter/<venue>` concrete adapter packages. Do not create `runtime/`.

Reason: this preserves the repository's documented SDK / adapter / account
layers, avoids standard library name collision, and keeps concrete package names
short.

### Decision 2: Instrument Symbol Format

Use normalized symbols such as `BTC-USDT-PERP.BINANCE`, not raw venue symbols,
for V2 `InstrumentID`.

Reason: raw venue symbols are often ambiguous across spot/perp/option product
lines. The raw symbol remains on `Instrument.RawSymbol`.

### Decision 3: Account State Scope

Start with full `AccountState` replacement events only. Add `AccountDelta`
later if real venues need partial updates.

Reason: replacement semantics are easier to test and prevent silent stale
margin or balance entries.

### Decision 4: Options

Model options as instruments first. Add chain managers only after the
instrument provider and Greeks events are stable.

Reason: option chains depend on reliable option identity, quote events, Greeks
events, and series grouping.

### Decision 5: V1 Compatibility

No compatibility guarantee. Provide a temporary `legacy/` package only if it
helps preserve examples or phased tests during migration.

Reason: preserving V1 will preserve string-symbol ambiguity and broad adapter
interfaces.

### Decision 6: Certification Artifacts

Produce machine-readable certification JSON under
`docs/superpowers/certifications/` for human-reviewed stable results. Keep
ephemeral live-test output under `.omx/`.

Reason: static bools in code cannot distinguish declared support, verified
support, and runtime health.

## Open Questions

These questions do not block the first implementation slice, but they should be
decided before migrating all venues.

1. Should V2 support both netting and hedging position modes in the first pass,
   or start with one position model per account?
2. Should live V2 only consume venue-reported margin initially, leaving
   calculated margin models to later backtesting work?
3. Should option chain aggregation live in `account/marketdata`, `portfolio`,
   or a separate `options/` package once it grows?
4. Should the certification runner emit JSON directly from Go tests, or should
   a wrapper command collect `go test -json` output and write certification
   summaries?

## Recommended First Implementation Slice

The first implementation slice should prove the architecture on one venue and
two product types.

Recommended slice:

1. Add `model/` identifiers, money, instruments, account state, and reports.
2. Add `venue/` contracts.
3. Add `account.Cache` for instruments, account states, orders, fills, and
   stream health.
4. Add Binance V2 instrument provider for spot and USD-M perp.
5. Add Binance V2 market data client for ticker and orderbook.
6. Add Binance V2 execution client for submit, cancel, and order reports.
7. Add account state translation for Binance spot and perp.
8. Add V2 `account.TradingAccount` startup, reconciliation, private stream
   readiness, and market order lifecycle.
9. Add certification suite for that slice.

Only after this slice passes should other adapters migrate.

## Expected End State

After V2:

- users trade instruments, not strings;
- spot, perp, future, option, and binary option are distinct instrument types;
- account assets are queried from account state, not ad hoc balance RPCs;
- margin is scoped by currency or instrument, matching venue reporting shape;
- adapters emit normalized events and reports;
- `account.TradingAccount` and `portfolio.Portfolio` own stateful lifecycle and
  aggregation;
- capability claims are auditable through certification results.
