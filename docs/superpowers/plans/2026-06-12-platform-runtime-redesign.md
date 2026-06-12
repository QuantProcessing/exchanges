# Platform Runtime Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Upgrade the repository from an adapter library into a Go-native trading platform while preserving direct access to native exchange SDK packages.

**Architecture:** Keep `sdk/` as the native exchange protocol layer. Promote `venue` clients into independently connectable data and execution clients, then add a `platform/` package that owns node lifecycle, event bus, data engine, execution engine, and startup reconciliation. Add a top-level `cache/` package for shared instruments, orders, fills, positions, and account snapshots; keep `account/` focused on order state machines, reconciliation application, and `TradingAccount`. Keep adapter-level `Adapter` constructors as convenience bundles, not as the central runtime abstraction.

**Tech Stack:** Go 1.26, existing `model`, `venue`, `account`, `adapter/binance`, `sdk/binance/spot`, `sdk/binance/perp`, `github.com/shopspring/decimal`, `go test`.

---

## Review Scope

This is the platformization plan, not another Binance-only cleanup pass.

The intended user experience has three valid entry points:

```go
// 1. Native SDK: raw exchange power, caller owns lifecycle and mapping.
spotSDK := spot.NewClient()
depth, err := spotSDK.Depth(ctx, "BTCUSDT", 100)
_ = depth
_ = err

// 2. Standardized clients: normalized instruments, market data, execution reports.
data, err := binance.NewSpotDataClient(ctx, binance.Options{})
exec, err := binance.NewPerpExecutionClient(ctx, binance.Options{
	APIKey:    apiKey,
	SecretKey: secretKey,
})
_ = data
_ = exec
_ = err

// 3. Platform runtime: engines, cache, bus, reconciliation, order state.
node := platform.NewNode(platform.Config{})
node.AddDataClient("binance-spot-data", data)
node.AddExecutionClient("binance-perp-exec", exec)
err = node.Start(ctx)
```

The non-negotiable design target is:

- SDK users can stay close to official exchange APIs.
- Client users get stable normalized convenience APIs without a platform runtime.
- Platform users get Nautilus-like lifecycle, cache, engines, bus, and order tracking.

## Current Branch Baseline

The current branch already made useful progress:

- `venue.Adapter` exposes `Instruments()`, `MarketData()`, and `Execution()`.
- Binance has `SpotAdapter` and `PerpAdapter` convenience structs.
- Binance has separate `marketDataClient`, `spotExecutionClient`, and `perpExecutionClient` objects.
- Execution emits `model.ExecutionEvent` carrying account, order, fill, and position reports.
- `account.TradingAccount` starts with `QueryAccount`, order reports, fill reports, and position reports before consuming the execution stream.
- Binance public market data now supports ticker, partial order book, diff-depth local book, trades, and bars.

The remaining gap versus a real platform is not file layout. It is runtime ownership:

- data clients do not yet have first-class `Connect`, `Disconnect`, and `Health`;
- `Adapter` still feels like the main construction surface;
- `ExecutionEvent` is report-shaped rather than transition-event-shaped;
- there is no platform-level endpoint/topic event bus with multiple subscribers;
- cache is still named and owned as an `account` detail instead of being a shared platform cache;
- order state is latest-report storage, not an explicit state machine;
- Binance reconciliation still relies mainly on open orders and fills, not full mass status reports across orders, fills, and positions;
- platform examples do not yet show data-only, execution-only, or mixed-client runtime assembly.

## NautilusTrader Alignment Evidence

These implementation choices are copied from NautilusTrader unless Go package boundaries require a small naming adjustment.

| Decision Area | NautilusTrader Evidence | Local Decision |
| --- | --- | --- |
| Client assembly | `BinanceLiveDataClientFactory.create` and `BinanceLiveExecClientFactory.create` construct separate data/execution clients with injected `msgbus`, `cache`, `clock`, and instrument provider. | `Adapter` is only a convenience bundle. Platform registration accepts independent `venue.DataClient` and `venue.ExecutionClient`. |
| Data lifecycle | `LiveDataClient.connect/disconnect` are first-class lifecycle methods, and `LiveDataEngine.connect/disconnect` calls them on all registered clients. | Add `venue.DataClient` instead of growing the smaller REST/subscribe `MarketDataClient` interface. |
| Execution lifecycle | `LiveExecutionClient` has `connect/disconnect`, command entry points, and abstract report generators for orders, fills, and positions. | Keep `venue.ExecutionClient` as the platform execution boundary and require reconciliation methods there. |
| Cache ownership | Live clients and engines receive a shared `Cache`; `LiveNode.cache` exposes a read-only cache facade. | Move shared cache out of `account` into top-level `cache/` with read/write cache plus read-only facade. |
| Message routing | Execution clients `send` order events and reports to `ExecEngine.*` endpoints; execution engine later `publish`es strategy topics. | `platform.Bus` supports endpoint `Register/Send` and topic `Subscribe/Publish`. Typed helpers are wrappers, not the core abstraction. |
| Order events | `OrderEvent` is in the model event hierarchy, while execution reports remain separate reconciliation objects. | `model.OrderEvent` is a domain event. `account` consumes it but does not define it. |
| Binance transport | Binance execution submits/cancels through HTTP account endpoints while user data WS is connected in `_connect` and resubscribed with reconciliation hooks. | Public execution API hides transport. Use REST/HTTP for submit/cancel/modify in this phase; private WS is for order/account/fill reports. |
| Startup reconciliation | `generate_mass_status` gathers order status, fill, and position reports with `open_only=false`; Binance futures also reconciles positions and algo order history. | A connector cannot claim startup reconciliation unless it can generate bounded order, fill, and position/account reports or returns explicit unsupported status. |

## Resolved Nautilus-Aligned Decisions

### Decision 1: `Adapter` Becomes A Convenience Bundle

`venue.Adapter` remains useful for simple users and registry-based construction:

```go
type Adapter interface {
	Venue() model.Venue
	Instruments() InstrumentProvider
	MarketData() MarketDataClient
	Execution() ExecutionClient
	Capabilities() DeclaredCapabilities
	Close() error
}
```

It should not be the platform kernel. Platform assembly should accept independent clients:

```go
node.AddDataClient("binance-spot-data", spotData)
node.AddExecutionClient("binance-perp-exec", perpExec)
```

This allows:

- data-only nodes with no credentials;
- execution-only nodes with private streams and no public market streams;
- mixed configurations, such as Binance spot data plus Binance perp execution;
- multiple data clients for the same venue with different stream endpoints or symbols.

### Decision 2: Add First-Class Data Client Lifecycle

`ExecutionClient` already has lifecycle methods. Market data should get a platform-facing lifecycle wrapper without forcing every simple REST market data implementation to keep sockets open.

Add:

```go
package venue

type DataClient interface {
	Venue() model.Venue
	ClientID() string
	Instruments() InstrumentProvider
	Connect(ctx context.Context) error
	Disconnect(ctx context.Context) error
	Health() DataHealth
	MarketDataClient
}

type DataHealth struct {
	Connected     bool
	InstrumentReady bool
	LastEventTime time.Time
	LastError     error
}
```

Keep `MarketDataClient` as the smaller fetch/subscribe capability interface. Platform engines depend on `DataClient`; simple users may still accept `MarketDataClient`.

### Decision 3: Reports And Events Are Different Types

Reports answer "what is true now or at reconciliation time." Events answer "what happened."

Keep:

```go
type OrderStatusReport struct { ... }
type FillReport struct { ... }
type PositionStatusReport struct { ... }
```

Add:

```go
type OrderEventType string

const (
	OrderEventSubmitted       OrderEventType = "submitted"
	OrderEventAccepted        OrderEventType = "accepted"
	OrderEventRejected        OrderEventType = "rejected"
	OrderEventPartiallyFilled OrderEventType = "partially_filled"
	OrderEventFilled          OrderEventType = "filled"
	OrderEventCanceled        OrderEventType = "canceled"
	OrderEventExpired         OrderEventType = "expired"
	OrderEventModified        OrderEventType = "modified"
)

type OrderEvent struct {
	EventID      string
	AccountID   AccountID
	InstrumentID InstrumentID
	OrderID     OrderID
	ClientID    ClientOrderID
	Type        OrderEventType
	Status      OrderStatus
	Side        OrderSide
	OrderType   OrderType
	Quantity    decimal.Decimal
	FilledQty   decimal.Decimal
	AvgPrice    decimal.Decimal
	Text        string
	EventTime   time.Time
}
```

Execution streams should emit transition events and reports where both are available. Startup reconciliation should emit reports. The account state machine consumes both.

### Decision 4: Platform Bus Uses Endpoints And Topics

Create a `platform.Bus` owned by `platform.Node`.

NautilusTrader uses the message bus in two distinct ways:

- `send(endpoint, msg)` for point-to-point engine commands and engine-owned processing, such as `ExecEngine.process`;
- `publish(topic, msg)` for fan-out streams, such as strategy order event topics.

Use the same split in Go:

```go
type Bus interface {
	Register(endpoint string, handler Handler) venue.Subscription
	Send(ctx context.Context, endpoint string, msg any) error
	Subscribe(topic string, buffer int) (venue.Subscription, <-chan Event)
	Publish(topic string, msg any) error
	Close() error
	Health() BusHealth
}

type Handler func(context.Context, Event) error

type Event struct {
	Endpoint string
	Topic    string
	Message  any
	Time     time.Time
}
```

Typed helpers such as `SubscribeExecution` or `PublishMarketData` may exist, but they must wrap endpoint/topic routing. The bus must not be a global singleton and must not use package-level mutable runtime state.

No global bus. No package-level mutable runtime state.

### Decision 5: `cache` Owns Shared State, `account` Owns Order Logic

Adopt the Nautilus split: a central cache is injected into engines and clients, while account/order logic consumes and updates that cache.

Create a top-level `cache/` package for:

- instrument definitions keyed by instrument ID and venue symbol;
- account snapshots;
- order status reports and live order state;
- fill reports;
- position reports;
- read-only facade for platform users.

`account` remains the right package for:

- order tracker;
- order state machine;
- reconciler;
- `TradingAccount`.

`platform` owns:

- `Node`;
- data engine;
- execution engine;
- bus;
- lifecycle startup/shutdown;
- client registration;
- engine health snapshots.

This preserves Go package direction:

```text
sdk/<venue>      -> no platform dependency
model            -> leaf package
venue            -> model
adapter/<venue>  -> sdk/<venue>, model, venue
cache            -> model
account          -> model, venue, cache
platform         -> model, venue, account, cache
examples         -> platform, adapter/<venue>
```

### Decision 6: Binance Order Transport Is Hidden And REST-First

Use the NautilusTrader Binance execution model:

- `SubmitOrder`, `CancelOrder`, and `ModifyOrder` stay normalized command methods;
- Binance execution clients use HTTP/REST account endpoints for submit/cancel/modify in this phase;
- private user data WebSocket is initialized during `Connect`;
- private WS remains connected across orders and is disconnected only by `Disconnect`;
- private WS resubscribe must trigger reconciliation through an `onResubscribe` hook;
- native SDK packages may still expose WS order APIs, but the platform execution API does not expose transport choice.

Do not add `OrderTransportPolicy` to `binance.Options` in this phase. It is not how Nautilus presents the execution client, and it creates an unnecessary public axis before we have a platform order state machine.

### Decision 7: Startup Reconciliation Requires Mass Status Sources

`Reconciliation.Startup=true` means the connector can generate a bounded startup mass status:

- order status reports from all-orders/history where the venue supports it;
- fill reports from user trades;
- position reports for derivative venues;
- account snapshot reports.

If an exchange cannot provide one of these sources, the connector must return explicit `ErrNotSupported` for that report family and the capability claim must distinguish partial support from certified startup reconciliation.

## Target File Structure

Create:

- `cache/doc.go` - shared cache contract and package boundaries.
- `cache/cache.go` - thread-safe instruments, accounts, orders, fills, and positions cache.
- `cache/facade.go` - read-only cache facade exposed by platform node.
- `cache/cache_test.go` - cache upsert, snapshot, and facade immutability tests.
- `platform/doc.go` - package contract and usage modes.
- `platform/config.go` - node and engine configuration.
- `platform/node.go` - node lifecycle and client registration.
- `platform/node_test.go` - start/stop and mixed-client assembly tests.
- `platform/bus.go` - endpoint send/register plus topic publish/subscribe bus.
- `platform/bus_test.go` - endpoint dispatch, multiple subscriber, backpressure, and close semantics.
- `platform/data_engine.go` - data client lifecycle, instrument load, market subscriptions.
- `platform/data_engine_test.go` - data-only runtime tests.
- `platform/execution_engine.go` - execution client lifecycle, reconciliation, event forwarding.
- `platform/execution_engine_test.go` - startup reconciliation and event forwarding tests.
- `model/order_event.go` - order transition event model.
- `model/market_event.go` - optional market data event envelope for platform bus.
- `account/order_state_machine.go` - transition rules and report application.
- `account/order_state_machine_test.go` - legal and illegal transition tests.
- `examples/platform_spread_arbitrage/main.go` - platform-style pseudo-real example.

Modify:

- `venue/interfaces.go` - add `DataClient`, `DataHealth`, optional constructor-facing client IDs.
- `model/order.go` - add event-aware fields only if they belong on existing report structs.
- `model/market_data.go` - add market data event envelope only if bus needs a normalized union.
- `account/cache.go` - move shared cache implementation to `cache/cache.go` or delete after migration.
- `account/cache_test.go` - move cache tests to `cache/cache_test.go` or delete after migration.
- `account/reconciler.go` - apply `OrderEvent` through state machine while preserving report application.
- `account/order_tracker.go` - expose event stream in addition to report and fill streams.
- `account/trading_account.go` - use platform-compatible execution event semantics.
- `adapter/binance/adapter_spot.go` - keep as convenience bundle.
- `adapter/binance/adapter_perp.go` - keep as convenience bundle.
- `adapter/binance/market_data_client.go` - implement `venue.DataClient`.
- `adapter/binance/execution_client.go` - provide stable client ID and event semantics.
- `adapter/binance/execution_spot.go` - expose `NewSpotExecutionClient` and improve reconciliation.
- `adapter/binance/execution_perp.go` - expose `NewPerpExecutionClient` and improve reconciliation.
- `adapter/binance/execution_reports.go` - split report mapping from order event mapping.
- `adapter/binance/private_stream.go` - emit order transition events where exchange payloads support them.
- `sdk/binance/spot/order.go` - add `AllOrders` if missing.
- `sdk/binance/perp/order.go` - add `AllOrders` if missing.
- `testsuite/venue_contract_suite.go` - add platform client contract checks.
- `testsuite/lifecycle_suite.go` - add startup reconciliation and event-state checks.

Avoid:

- top-level `runtime/` package;
- global default node;
- hidden credential reads in constructors;
- platform imports inside SDK packages;
- strategy logic inside adapters.

## Public API Shape

### Native SDK

No breaking change is required for native SDK users.

```go
client := spot.NewClient().WithCredentials(apiKey, secret)
orders, err := client.GetOpenOrders(ctx, "BTCUSDT")
```

### Standardized Client Layer

Add direct constructors that return independent clients.

```go
spotData, err := binance.NewSpotDataClient(ctx, binance.Options{
	BaseURLHTTP: "https://api.binance.com",
})

perpExec, err := binance.NewPerpExecutionClient(ctx, binance.Options{
	APIKey:          apiKey,
	SecretKey:       secretKey,
	BaseURLHTTP:     "https://fapi.binance.com",
	BaseURLWSStream: "wss://fstream.binance.com/ws",
})
```

Convenience adapters call those constructors internally:

```go
adapter, err := binance.NewPerpAdapter(ctx, opts)
data := adapter.MarketData()
exec := adapter.Execution()
```

### Platform Layer

Platform node assembly should look like this:

```go
sharedCache := cache.New()
node := platform.NewNode(platform.Config{Cache: sharedCache})

spotData, err := binance.NewSpotDataClient(ctx, binance.Options{})
if err != nil {
	return err
}
perpExec, err := binance.NewPerpExecutionClient(ctx, binance.Options{
	APIKey:    apiKey,
	SecretKey: secretKey,
})
if err != nil {
	return err
}

node.AddDataClient("binance-spot", spotData)
node.AddExecutionClient("binance-perp", perpExec)

if err := node.Start(ctx); err != nil {
	return err
}
defer node.Stop(context.Background())
```

The platform must support data and execution clients with different account types and endpoint configurations.

## Task 1: Add Platform Package Skeleton And Bus

**Files:**

- Create: `platform/doc.go`
- Create: `platform/config.go`
- Create: `platform/bus.go`
- Create: `platform/bus_test.go`

- [ ] **Step 1: Write bus multi-subscriber test**

Create `platform/bus_test.go` with tests covering:

- `Register("ExecEngine.process", handler)` then `Send(ctx, "ExecEngine.process", msg)` dispatches exactly once to the endpoint handler;
- sending to an unregistered endpoint returns `platform.ErrEndpointNotFound`;
- two `events.execution` topic subscribers both receive the same event;
- closing one subscription does not close the other;
- `Bus.Close` closes all subscription channels;
- publishing after close returns without panic and records a closed-bus error in health.

Expected command:

```bash
env GOCACHE=/tmp/go-build go test ./platform -run TestBus -count=1
```

Expected first result: FAIL because `platform` does not exist.

- [ ] **Step 2: Implement bounded bus**

Create `platform/bus.go` with:

- `type Bus struct`;
- endpoint handler map guarded by `sync.RWMutex`;
- topic subscriber maps guarded by `sync.RWMutex`;
- `Register(endpoint string, handler Handler) venue.Subscription`;
- `Send(ctx context.Context, endpoint string, msg any) error`;
- `Subscribe(topic string, buffer int) (venue.Subscription, <-chan Event)`;
- `Publish(topic string, msg any) error`;
- `Close() error`;
- drop-count tracking for full subscriber channels.

Use direct endpoint dispatch for `Send`. Use non-blocking topic sends for `Publish`; a slow subscriber must not block execution streams.

- [ ] **Step 3: Run bus tests**

```bash
env GOCACHE=/tmp/go-build go test ./platform -run TestBus -count=1
```

Expected: PASS.

## Task 2: Add Central Cache Package

**Files:**

- Create: `cache/doc.go`
- Create: `cache/cache.go`
- Create: `cache/facade.go`
- Create: `cache/cache_test.go`
- Modify: `account/cache.go`
- Modify: `account/cache_test.go`

- [ ] **Step 1: Write cache package tests**

Create `cache/cache_test.go` with tests covering:

- upserting and reading an instrument by `model.InstrumentID`;
- upserting and reading an order status report by account and client order ID;
- upserting and reading fill reports by account/order/trade ID;
- upserting and reading position status by account and instrument ID;
- facade reads return snapshots that cannot mutate the cache.

Expected command:

```bash
env GOCACHE=/tmp/go-build go test ./cache -run TestCache -count=1
```

Expected first result: FAIL because `cache` does not exist.

- [ ] **Step 2: Move shared cache out of `account`**

Create:

- `cache.Cache` with mutex-protected maps;
- `cache.New() *Cache`;
- `cache.Facade` as a read-only view over `Cache`;
- upsert/read/snapshot methods needed by `account` and `platform`.

Keep temporary aliases in `account` only if needed for incremental compilation:

```go
type Cache = cache.Cache

func NewCache() *cache.Cache {
	return cache.New()
}
```

Remove those aliases before final verification if no public compatibility is required.

- [ ] **Step 3: Run cache tests**

```bash
env GOCACHE=/tmp/go-build go test ./cache ./account -run 'TestCache|TestReconciler' -count=1
```

Expected: PASS.

## Task 3: Add Order Events And State Machine

**Files:**

- Create: `model/order_event.go`
- Create: `account/order_state_machine.go`
- Create: `account/order_state_machine_test.go`
- Modify: `model/order.go`
- Modify: `account/reconciler.go`
- Modify: `account/order_tracker.go`

- [ ] **Step 1: Write state machine tests**

Cover these transitions:

- submitted -> accepted;
- accepted -> partially_filled;
- partially_filled -> filled;
- accepted -> canceled;
- submitted -> rejected;
- filled cannot move back to accepted;
- canceled cannot move to filled unless the event is an already-seen duplicate.

Expected command:

```bash
env GOCACHE=/tmp/go-build go test ./account -run TestOrderStateMachine -count=1
```

Expected first result: FAIL because `OrderEvent` and state machine do not exist.

- [ ] **Step 2: Add `model.OrderEvent`**

Create `model/order_event.go` with `OrderEventType`, constants, and `OrderEvent`.

Rule: `OrderStatusReport` remains a report. It must not be renamed into an event.

- [ ] **Step 3: Implement account state machine**

Create `account/order_state_machine.go`.

Rules:

- terminal statuses are `filled`, `canceled`, `rejected`, and `expired`;
- duplicate terminal events with same order ID, client ID, status, filled quantity, and event time are ignored;
- invalid backward transitions return `model.ErrInvalidAccountState`;
- reports may initialize state during reconciliation;
- live events update state and publish through `OrderTracker`.

- [ ] **Step 4: Run account tests**

```bash
env GOCACHE=/tmp/go-build go test ./account -run 'TestOrderStateMachine|TestReconciler' -count=1
```

Expected: PASS.

## Task 4: Add `venue.DataClient`

**Files:**

- Modify: `venue/interfaces.go`
- Modify: `venue/capabilities.go`
- Create: `venue/data_client_test.go`

- [ ] **Step 1: Add interface compile tests**

Create a stub implementation that satisfies `venue.DataClient` and verify it can be registered by the platform tests in Task 5.

Expected command:

```bash
env GOCACHE=/tmp/go-build go test ./venue -run TestDataClientContract -count=1
```

Expected first result: FAIL because `DataClient` does not exist.

- [ ] **Step 2: Add data lifecycle types**

Add:

- `type DataClient interface`;
- `type DataHealth struct`;
- `ClientID() string` requirement for data clients;
- `ClientID() string` on `ExecutionClient` if platform registration should not rely on caller keys alone.

Keep `MarketDataClient` as the small data capability interface.

- [ ] **Step 3: Run venue tests**

```bash
env GOCACHE=/tmp/go-build go test ./venue -count=1
```

Expected: PASS.

## Task 5: Add Platform Node And Engines

**Files:**

- Create: `platform/node.go`
- Create: `platform/node_test.go`
- Create: `platform/data_engine.go`
- Create: `platform/data_engine_test.go`
- Create: `platform/execution_engine.go`
- Create: `platform/execution_engine_test.go`
- Modify: `account/trading_account.go`

- [ ] **Step 1: Write data-only node test**

Test:

- create node with fake data client;
- add data client;
- `Start` calls data client `Connect`;
- `Stop` calls data client `Disconnect`;
- no execution client is required.

Expected first command:

```bash
env GOCACHE=/tmp/go-build go test ./platform -run TestNodeStartsDataOnly -count=1
```

Expected first result: FAIL.

- [ ] **Step 2: Write mixed-client node test**

Test:

- one data client and one execution client can be registered under different names;
- startup loads instruments once;
- execution engine runs account query and reconciliation before marking ready;
- bus receives execution events after startup.

Expected command:

```bash
env GOCACHE=/tmp/go-build go test ./platform -run TestNodeStartsMixedClients -count=1
```

Expected first result: FAIL.

- [ ] **Step 3: Implement `platform.Node`**

`Node` owns:

- `*cache.Cache`;
- `*Bus`;
- `*DataEngine`;
- `*ExecutionEngine`;
- lifecycle mutex;
- health snapshot.

`Start(ctx)` order:

1. data engine connects and loads instruments into cache;
2. execution engine queries accounts and reconciliation reports;
3. execution streams connect;
4. engines forward events to bus;
5. node health marks ready.

`Stop(ctx)` order:

1. stop execution streams;
2. stop market data streams;
3. close bus subscriptions;
4. close order trackers.

- [ ] **Step 4: Run platform tests**

```bash
env GOCACHE=/tmp/go-build go test ./platform -count=1
```

Expected: PASS.

## Task 6: Expose Independent Binance Clients

**Files:**

- Modify: `adapter/binance/adapter_spot.go`
- Modify: `adapter/binance/adapter_perp.go`
- Modify: `adapter/binance/market_data_client.go`
- Modify: `adapter/binance/execution_client.go`
- Modify: `adapter/binance/execution_spot.go`
- Modify: `adapter/binance/execution_perp.go`
- Modify: `adapter/binance/options.go`
- Create: `adapter/binance/client_constructor_test.go`

- [ ] **Step 1: Write constructor tests**

Tests:

- `NewSpotDataClient` works without credentials;
- `NewPerpDataClient` works without credentials;
- `NewSpotExecutionClient` validates paired credentials;
- `NewPerpExecutionClient` validates paired credentials;
- `NewSpotAdapter` uses the same client constructors internally;
- `NewPerpAdapter` uses the same client constructors internally.

Expected command:

```bash
env GOCACHE=/tmp/go-build go test ./adapter/binance -run TestBinanceIndependentClientConstructors -count=1
```

Expected first result: FAIL.

- [ ] **Step 2: Implement client constructors**

Add constructors:

```go
func NewSpotDataClient(ctx context.Context, opts Options) (venue.DataClient, error)
func NewPerpDataClient(ctx context.Context, opts Options) (venue.DataClient, error)
func NewSpotExecutionClient(ctx context.Context, opts Options) (venue.ExecutionClient, error)
func NewPerpExecutionClient(ctx context.Context, opts Options) (venue.ExecutionClient, error)
func NewSpotInstrumentProvider(opts Options) (venue.InstrumentProvider, error)
func NewPerpInstrumentProvider(opts Options) (venue.InstrumentProvider, error)
```

`SpotAdapter` and `PerpAdapter` become thin convenience bundles over those constructors.

- [ ] **Step 3: Implement data lifecycle on Binance market clients**

`marketDataClient.Connect(ctx)`:

- connects public market stream if it exists;
- loads instruments through provider;
- marks `DataHealth.InstrumentReady`;
- does not require credentials.

`Disconnect(ctx)`:

- closes public market stream;
- clears connected health;
- leaves REST clients reusable.

- [ ] **Step 4: Run Binance constructor tests**

```bash
env GOCACHE=/tmp/go-build go test ./adapter/binance -run TestBinanceIndependentClientConstructors -count=1
```

Expected: PASS.

## Task 7: Upgrade Binance Execution Events

**Files:**

- Modify: `adapter/binance/execution_reports.go`
- Modify: `adapter/binance/private_stream.go`
- Modify: `adapter/binance/private_stream_test.go`
- Modify: `adapter/binance/execution_client_test.go`

- [ ] **Step 1: Write stream event mapping tests**

Tests:

- spot `executionReport` with `X=NEW` emits `OrderEventAccepted`;
- spot `executionReport` with trade ID and last executed quantity emits both `OrderEventPartiallyFilled` or `OrderEventFilled` and `FillReport`;
- perp `ORDER_TRADE_UPDATE` maps `NEW`, `PARTIALLY_FILLED`, `FILLED`, `CANCELED`, `EXPIRED`, and `REJECTED`;
- duplicate fill trade IDs are left to account reconciler, not silently dropped in adapter.

Expected command:

```bash
env GOCACHE=/tmp/go-build go test ./adapter/binance -run Test.*PrivateStream.*OrderEvent -count=1
```

Expected first result: FAIL.

- [ ] **Step 2: Split report mappers from event mappers**

Keep report mappers:

- `spotOrderReport`;
- `perpOrderReport`;
- `spotFillReport`;
- `perpFillReport`.

Add event mappers:

- `spotOrderEventFromStream`;
- `perpOrderEventFromStream`;
- `spotOrderEventFromSubmitResponse`;
- `perpOrderEventFromSubmitResponse`;
- `spotOrderEventFromCancelResponse`;
- `perpOrderEventFromCancelResponse`.

- [ ] **Step 3: Update execution event union**

Add `OrderEvent *model.OrderEvent` to `model.ExecutionEvent`.

Keep `Order *model.OrderStatusReport` for reconciliation and snapshot reports.

- [ ] **Step 4: Run execution tests**

```bash
env GOCACHE=/tmp/go-build go test ./adapter/binance ./account -run 'Test.*OrderEvent|Test.*Execution' -count=1
```

Expected: PASS.

## Task 8: Upgrade Binance Reconciliation

**Files:**

- Modify: `sdk/binance/spot/order.go`
- Modify: `sdk/binance/spot/order_test.go`
- Modify: `sdk/binance/perp/order.go`
- Modify: `sdk/binance/perp/order_test.go`
- Modify: `adapter/binance/execution_spot.go`
- Modify: `adapter/binance/execution_perp.go`
- Modify: `adapter/binance/execution_client_test.go`

- [ ] **Step 1: Add SDK all-orders tests**

Spot test method:

```go
func TestClient_AllOrders(t *testing.T)
```

Perp test method:

```go
func TestClient_AllOrders(t *testing.T)
```

Private read tests skip only when credentials are missing.

Expected targeted command:

```bash
env GOCACHE=/tmp/go-build go test ./sdk/binance/spot ./sdk/binance/perp -run TestClient_AllOrders -count=1
```

Expected first result: FAIL if SDK methods are missing.

- [ ] **Step 2: Add SDK all-orders methods**

Spot endpoint:

- `GET /api/v3/allOrders`

Perp endpoint:

- `GET /fapi/v1/allOrders`

Both methods accept:

- symbol;
- order ID or start time when the official endpoint supports it;
- limit.

- [ ] **Step 3: Use all orders for order status reports**

`GenerateOrderStatusReports` should:

- use `AllOrders` when available;
- include open and recently closed orders;
- preserve `GetOpenOrders` only as a fallback when all-orders is unsupported;
- sort reports by event/update time ascending.

- [ ] **Step 4: Use trades for fill reports**

`GenerateFillReports` should:

- call `MyTrades`;
- include trade ID, commission, commission asset, price, quantity, side, and event time;
- sort by event time ascending;
- leave duplicate handling to `account.Reconciler`.

- [ ] **Step 5: Add perp position reconciliation**

`GeneratePositionStatusReports` should use:

- `GetPositionRisk` when available;
- `GetAccount` positions as fallback;
- position side from venue response;
- zero quantity as flat position only when the venue response includes the position.

- [ ] **Step 6: Run reconciliation tests**

```bash
env GOCACHE=/tmp/go-build go test ./adapter/binance ./account -run 'Test.*Generate.*Reports|TestTradingAccount.*Reconcile' -count=1
```

Expected: PASS.

## Task 9: Hide Binance Transport Behind Execution Client

**Files:**

- Modify: `adapter/binance/options.go`
- Modify: `adapter/binance/execution_spot.go`
- Modify: `adapter/binance/execution_perp.go`
- Modify: `adapter/binance/execution_client_test.go`
- Modify: `sdk/binance/spot/ws_order.go` only if public SDK compile tests require cleanup.
- Modify: `sdk/binance/perp/ws_order.go` only if public SDK compile tests require cleanup.

- [ ] **Step 1: Add hidden transport contract tests**

Tests:

- `SubmitOrder` calls the REST/HTTP order path;
- `CancelOrder` calls the REST/HTTP cancel path;
- `ModifyOrder` calls the REST/HTTP amend path when supported, or returns explicit `ErrNotSupported`;
- `Connect` initializes and subscribes the private user data stream;
- placing an order does not disconnect the private stream;
- private stream resubscribe invokes reconciliation hook;
- public `venue.ExecutionClient` does not expose `SubmitOrderWS`, `CancelOrderWS`, or transport policy knobs.

Expected command:

```bash
env GOCACHE=/tmp/go-build go test ./adapter/binance -run TestExecutionTransportHidden -count=1
```

Expected first result: FAIL.

- [ ] **Step 2: Keep command transport internal**

`SubmitOrder`, `CancelOrder`, and `ModifyOrder` use Binance REST/HTTP account endpoints in this phase.

The normalized public interface remains:

```go
SubmitOrder(ctx context.Context, cmd model.SubmitOrder) error
CancelOrder(ctx context.Context, cmd model.CancelOrder) error
ModifyOrder(ctx context.Context, cmd model.ModifyOrder) error
```

Do not add `OrderTransportPolicy` to `Options`. Keep WS order API methods available only in `sdk/binance/*` if they already exist or are useful for native SDK users.

- [ ] **Step 3: Wire private stream lifecycle**

`ExecutionClient.Connect(ctx)` must:

- initialize account state;
- initialize instrument provider if needed;
- connect the private stream;
- subscribe the user data stream;
- install an `onResubscribe` hook that generates order, fill, position, and account reports through the same reconciliation path as startup.

`ExecutionClient.Disconnect(ctx)` must unsubscribe and close the private stream. `SubmitOrder` and `CancelOrder` must never open or close that stream.

- [ ] **Step 4: Run hidden transport tests**

```bash
env GOCACHE=/tmp/go-build go test ./adapter/binance -run TestExecutionTransportHidden -count=1
```

Expected: PASS.

## Task 10: Add Platform Example And Contract Tests

**Files:**

- Create: `examples/platform_spread_arbitrage/main.go`
- Create: `testsuite/platform_suite.go`
- Modify: `docs/capabilities.md`
- Modify: `docs/contributing/adding-exchange-adapters.md`

- [ ] **Step 1: Add example compile test**

The example should compile without credentials when it only constructs data clients and platform node.

Expected command:

```bash
env GOCACHE=/tmp/go-build go test ./examples/platform_spread_arbitrage -run '^$'
```

Expected first result: FAIL until example package exists.

- [ ] **Step 2: Write spread-arbitrage platform example**

The example should show:

- independent spot data and perp execution client construction;
- platform node assembly;
- data subscription through data engine;
- order submission through execution engine;
- listening to bus execution events.

It must not place live orders by default.

- [ ] **Step 3: Add platform contract suite**

Contract checks:

- data client can load instruments;
- data client can start and stop idempotently;
- execution client can start and stop idempotently;
- execution startup reconciliation either succeeds or returns explicit `ErrNotSupported`;
- multiple bus subscribers receive the same execution event.

- [ ] **Step 4: Run platform and example tests**

```bash
env GOCACHE=/tmp/go-build go test ./platform ./examples/platform_spread_arbitrage ./testsuite -count=1
```

Expected: PASS.

## Task 11: Final Verification

**Files:**

- No new files.
- Fix only files touched by earlier tasks.

- [ ] **Step 1: Run targeted platform verification**

```bash
env GOCACHE=/tmp/go-build go test ./model ./venue ./account ./platform ./adapter/binance ./testsuite -count=1
```

Expected: PASS.

- [ ] **Step 2: Run SDK compile and method-local tests**

```bash
env GOCACHE=/tmp/go-build go test ./sdk/binance/spot ./sdk/binance/perp -run 'TestClient_AllOrders|TestWsMarketClient|TestWSClient' -count=1
```

Expected: PASS or credential-gated skips for private live reads when credentials are absent.

- [ ] **Step 3: Run repo compile sweep**

```bash
env GOCACHE=/tmp/go-build go test ./... -run '^$'
```

Expected: PASS.

- [ ] **Step 4: Report live-test gaps separately**

Do not block merge on live Binance network failures unless the failing test is a local unit or compile test. Report live failures with:

- command;
- endpoint family;
- whether credentials were present;
- error message;
- whether it is a network, auth, or local behavior failure.

## Acceptance Criteria

The platformization is complete for the Binance pilot when all of these are true:

- Native `sdk/binance/spot` and `sdk/binance/perp` remain directly usable.
- Shared cache lives in top-level `cache/`, with platform exposing a read-only facade.
- Binance exposes independent spot/perp data client constructors.
- Binance exposes independent spot/perp execution client constructors.
- `SpotAdapter` and `PerpAdapter` are convenience bundles over independent clients.
- A `platform.Node` can run data-only.
- A `platform.Node` can run execution-only when credentials are supplied.
- A `platform.Node` can run spot data and perp execution together.
- Public market data subscriptions support multiple platform consumers through bus fan-out.
- Execution events support multiple platform consumers through bus fan-out.
- Startup reconciliation produces order, fill, account, and perp position reports where Binance supports the source endpoint.
- Live private stream events become `OrderEvent` transitions plus fill reports.
- `account` applies order transitions through an explicit state machine.
- Binance execution hides transport: REST/HTTP for order commands, private WS for lifecycle reports, no public transport policy.
- Capability claims distinguish declared support from certified support.
- `env GOCACHE=/tmp/go-build go test ./model ./venue ./account ./platform ./adapter/binance ./testsuite -count=1` passes.
- `env GOCACHE=/tmp/go-build go test ./... -run '^$'` passes.

## Risks And Mitigations

| Risk | Mitigation |
| --- | --- |
| Platform layer becomes a large framework too early | Keep Node, DataEngine, ExecutionEngine, Bus, and cache wiring small. Do not add strategy scheduler, portfolio optimizer, or risk engine in this phase. |
| Event bus hides backpressure bugs | Use bounded subscriptions, explicit drop counters, and health reporting. |
| Order event model duplicates reports confusingly | Document reports as snapshots/reconciliation and events as transitions. Keep both names explicit. |
| Binance all-orders endpoints are rate-limited | Query per instrument, use caller-supplied instruments, support since/limit, and keep startup reconciliation bounded. |
| Existing adapter tests become too coupled to platform | Keep `venue.MarketDataClient` and `venue.ExecutionClient` contract tests separate from platform engine tests. |
| Transport details leak into platform API | Keep `SubmitOrderWS`, `CancelOrderWS`, and transport policy out of `venue.ExecutionClient` and `binance.Options`; expose WS order methods only in native SDK packages if needed. |

## Resolved Review Checklist

- `platform/` remains the runtime orchestration package because Nautilus separates node/engines from exchange adapters.
- `venue.DataClient` is a new interface; `MarketDataClient` remains the smaller fetch/subscribe capability.
- `model.OrderEvent` owns order transition events; `account` consumes them through the state machine.
- Shared cache moves to top-level `cache/`; neither `account.Cache` nor `platform.Cache` is the long-term owner.
- Binance execution does not expose transport policy. It uses REST/HTTP order commands plus private WS lifecycle reports.
- Startup reconciliation certification requires bounded mass status: orders, fills, positions where relevant, and account snapshot.

## Self-Review

- Spec coverage: the plan covers SDK preservation, independent clients, central cache, platform node, endpoint/topic bus, order events, state machine, reconciliation, hidden Binance transport, examples, and verification.
- Completeness scan: this document uses no empty sections and no unspecified task bodies.
- Type consistency: `venue.DataClient`, `platform.Node`, `platform.Bus`, `model.OrderEvent`, and `account` state machine names are used consistently across tasks.
- Scope check: this is intentionally a Binance pilot plus shared platform kernel. Other venues are not migrated in this plan.
