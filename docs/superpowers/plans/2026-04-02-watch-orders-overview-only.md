# WatchOrders Overview-Only Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `WatchOrders` a consistent order-overview stream, keep execution detail in `WatchFills`, and ensure shared native order topics are subscribed only once with internal fan-out.

**Architecture:** Keep the public `Order` and `Fill` models compatible, but narrow the streaming contract: `WatchOrders` should only project order-overview fields while `WatchFills` remains the only execution-detail stream. For exchanges where the same native order event can drive both public streams, add package-local stream state that subscribes once and dispatches to both callbacks. For exchanges that already have separate native order and fill topics, keep the native subscriptions separate and only normalize the `WatchOrders` mapping.

**Tech Stack:** Go, package-local adapter helpers, `go test`, existing live/private adapter tests.

---

## File Structure

### Shared contract and docs

- Modify: `exchange.go`
  - Tighten `WatchOrders` / `WatchFills` comments to describe overview-only vs execution-only semantics.
- Modify: `README.md`
  - Update examples and guidance so `WatchOrders` no longer advertises recent execution detail.
- Modify: `README_CN.md`
  - Chinese mirror of the same guidance.
- Modify: `docs/superpowers/specs/2026-04-02-order-and-fill-stream-boundary-design.md`
  - Align the earlier boundary design with the approved overview-only behavior.
- Create or modify: `streaming_contract_test.go`
  - Keep compatibility checks for the `Order` / `Fill` structs without implying those fields are part of the `WatchOrders` contract.

### Shared-topic adapters

- Create: `binance/private_streams.go`
  - Add package-local stream state for spot/perp native order topics that can fan out to both public callbacks.
- Create: `binance/private_streams_test.go`
  - Unit tests for single-subscription fan-out and teardown behavior.
- Modify: `binance/perp_adapter.go`
  - Route `WatchOrders` / `WatchFills` through shared stream state and drop execution-detail fields from order-stream payloads.
- Modify: `binance/spot_adapter.go`
  - Same as perp, but for `executionReport`.
- Create: `aster/private_streams.go`
  - Same pattern as Binance, adapted to Aster spot/perp account streams.
- Create: `aster/private_streams_test.go`
  - Unit tests for shared-topic fan-out and callback teardown.
- Modify: `aster/perp_adapter.go`
  - Route callbacks through shared stream state and keep `WatchOrders` overview-only.
- Modify: `aster/spot_adapter.go`
  - Same as perp, but for spot `executionReport`.

### Existing single-subscription adapters

- Modify: `okx/private_streams.go`
  - Keep one-subscription fan-out, but ensure the order callback uses overview-only mapping.
- Create: `okx/private_streams_test.go`
  - Lock in “subscribe once, stop when last consumer leaves” behavior.
- Modify: `okx/perp_adapter.go`
  - Add a stream-order mapping that excludes execution-detail fields while preserving existing REST mapping.
- Modify: `okx/spot_adapter.go`
  - Same as perp.
- Modify: `backpack/streams.go`
  - Keep one-subscription fan-out and make the order callback use overview-only mapping.
- Modify: `backpack/private_mapping.go`
  - Split order-overview mapping from fill mapping.
- Create or modify: `backpack/stream_mapping_test.go`
  - Verify overview-only order mapping and fill extraction still work from one native payload.

### Separate-topic adapters that currently leak execution detail into WatchOrders

- Modify: `bitget/private_classic.go`
  - Keep native topics separate but add stream-order mappers that exclude fill-price / avg-price detail.
- Create: `bitget/order_stream_mapping_test.go`
  - Verify stream-order mapping keeps overview fields and strips execution-detail fields.
- Modify: `grvt/perp_adapter.go`
  - Introduce a dedicated stream-order mapper so `WatchOrders` no longer carries average fill price.
- Create: `grvt/order_stream_mapping_test.go`
  - Verify `WatchOrders` output is overview-only while `WatchFills` remains detailed.
- Modify: `standx/perp_adapter.go`
  - Add a stream-order mapper that omits `AverageFillPrice`.
- Create: `standx/order_stream_mapping_test.go`
  - Verify streaming order mapping remains overview-only.

### Verification-only files already present

- Reuse: `binance/fills_live_test.go`
- Reuse: `lighter/fills_live_test.go`

Implementation note:

- The current workspace already has unrelated uncommitted files. Execute this plan in a fresh worktree branch such as `codex/watch-orders-overview-only` to avoid mixing changes.

## Task 1: Tighten the shared contract and documentation

**Files:**
- Modify: `exchange.go`
- Modify: `README.md`
- Modify: `README_CN.md`
- Modify: `docs/superpowers/specs/2026-04-02-order-and-fill-stream-boundary-design.md`
- Modify: `streaming_contract_test.go`

- [ ] **Step 1: Update streaming comments in `exchange.go`**

```go
type Streamable interface {
	// WatchOrders emits order-overview lifecycle updates.
	// It is the source of truth for order state, order quantity,
	// submitted price, and aggregate filled quantity progress.
	WatchOrders(ctx context.Context, cb OrderUpdateCallback) error

	// WatchFills emits one callback per private execution/fill event.
	// It is the source of truth for execution price, per-fill quantity,
	// fee, and maker/taker attribution.
	WatchFills(ctx context.Context, cb FillCallback) error
}
```

- [ ] **Step 2: Update README examples so `WatchOrders` shows overview-only fields**

```go
adp.WatchOrders(ctx, func(o *exchanges.Order) {
	fmt.Printf("Order %s: %s order=%s qty=%s filled=%s\n",
		o.OrderID, o.Status, o.OrderPrice, o.Quantity, o.FilledQuantity)
})
```

- [ ] **Step 3: Update the narrative text in `README.md` and `README_CN.md`**

```md
`WatchOrders` is an order-overview stream. It should be used for order lifecycle,
submitted price, submitted quantity, and aggregate filled quantity progress.

`WatchFills` is an execution-detail stream. It should be used for execution price,
execution quantity, fees, and maker/taker attribution.
```

- [ ] **Step 4: Align the older boundary spec with the approved overview-only semantics**

```md
Compatibility note:

- `AverageFillPrice`, `LastFillPrice`, and `LastFillQuantity` remain on `Order`
  for compatibility, but `WatchOrders` should not present them as part of the
  stable streaming contract.
```

- [ ] **Step 5: Replace the root streaming test wording so it checks struct compatibility, not stream semantics**

```go
func TestOrderRetainsExplicitOrderAndFillFieldsForCompatibility(t *testing.T) {
	order := exchanges.Order{
		OrderPrice:       decimal.RequireFromString("100"),
		AverageFillPrice: decimal.RequireFromString("101"),
		LastFillPrice:    decimal.RequireFromString("102"),
		LastFillQuantity: decimal.RequireFromString("0.5"),
	}

	require.Equal(t, "100", order.OrderPrice.String())
	require.Equal(t, "101", order.AverageFillPrice.String())
	require.Equal(t, "102", order.LastFillPrice.String())
	require.Equal(t, "0.5", order.LastFillQuantity.String())
}
```

- [ ] **Step 6: Run focused root/documentation-safe tests**

Run: `env GOCACHE=/tmp/go-build go test . -run 'Test(OrderRetainsExplicitOrderAndFillFieldsForCompatibility|FillCarriesExecutionDetails|ExplicitOrderPriceFieldsCanCoexistWithLegacyPrice)$' -count=1`

Expected: PASS

- [ ] **Step 7: Commit the contract/docs pass**

```bash
git add exchange.go README.md README_CN.md docs/superpowers/specs/2026-04-02-order-and-fill-stream-boundary-design.md streaming_contract_test.go
git commit -m "docs: define watch orders as overview-only"
```

## Task 2: Add single-subscription fan-out for Binance

**Files:**
- Create: `binance/private_streams.go`
- Create: `binance/private_streams_test.go`
- Modify: `binance/perp_adapter.go`
- Modify: `binance/spot_adapter.go`

- [ ] **Step 1: Add package-local stream state for Binance spot/perp**

```go
type sharedOrderStreamState[T any] struct {
	mu         sync.Mutex
	subscribed bool
	orderCB    exchanges.OrderUpdateCallback
	fillCB     exchanges.FillCallback
}

func (s *sharedOrderStreamState[T]) set(orderCB exchanges.OrderUpdateCallback, fillCB exchanges.FillCallback) (needSubscribe bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if orderCB != nil {
		s.orderCB = orderCB
	}
	if fillCB != nil {
		s.fillCB = fillCB
	}
	if !s.subscribed {
		s.subscribed = true
		return true
	}
	return false
}

func (s *sharedOrderStreamState[T]) callbacks() (exchanges.OrderUpdateCallback, exchanges.FillCallback) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.orderCB, s.fillCB
}
```

- [ ] **Step 2: Add unit tests for one-subscription fan-out**

```go
func TestSharedOrderStreamStateSubscribesOnce(t *testing.T) {
	var state sharedOrderStreamState[int]

	require.True(t, state.set(func(*exchanges.Order) {}, nil))
	require.False(t, state.set(nil, func(*exchanges.Fill) {}))
}

func TestSharedOrderStreamStateStopsOnlyWhenLastConsumerLeaves(t *testing.T) {
	var state sharedOrderStreamState[int]
	state.set(func(*exchanges.Order) {}, func(*exchanges.Fill) {})

	require.False(t, state.clear(true, false))
	require.True(t, state.clear(false, true))
}
```

- [ ] **Step 3: Refactor Binance perp to register one native order callback**

```go
func (a *Adapter) WatchOrders(ctx context.Context, callback exchanges.OrderUpdateCallback) error {
	return a.watchPrivateOrders(ctx, callback, nil)
}

func (a *Adapter) WatchFills(ctx context.Context, callback exchanges.FillCallback) error {
	return a.watchPrivateOrders(ctx, nil, callback)
}

func (a *Adapter) watchPrivateOrders(ctx context.Context, orderCB exchanges.OrderUpdateCallback, fillCB exchanges.FillCallback) error {
	if err := a.WsAccountConnected(ctx); err != nil {
		return err
	}
	if !a.privateOrderStream.set(orderCB, fillCB) {
		return nil
	}
	a.wsAccount.SubscribeOrderUpdate(func(e *perp.OrderUpdateEvent) {
		a.dispatchPrivateOrderUpdate(e)
	})
	return nil
}
```

- [ ] **Step 4: Make Binance order-stream mapping overview-only**

```go
order := &exchanges.Order{
	OrderID:        fmt.Sprintf("%d", e.Order.OrderID),
	ClientOrderID:  e.Order.ClientOrderID,
	Symbol:         a.ExtractSymbol(e.Order.Symbol),
	Side:           side,
	Type:           orderType,
	Price:          price,
	OrderPrice:     price,
	Quantity:       qty,
	FilledQuantity: filledQty,
	Status:         status,
	Timestamp:      e.TransactionTime,
}
```

- [ ] **Step 5: Apply the same refactor to Binance spot**

```go
func (a *SpotAdapter) dispatchPrivateExecutionReport(report *spot.ExecutionReportEvent) {
	orderCB, fillCB := a.privateOrderStream.callbacks()
	if orderCB != nil {
		orderCB(a.mapExecutionReportOrder(report))
	}
	if fillCB != nil {
		if fill := a.mapExecutionFill(report); fill != nil {
			fillCB(fill)
		}
	}
}
```

- [ ] **Step 6: Run focused Binance tests**

Run: `env GOCACHE=/tmp/go-build go test ./binance -run 'Test(SharedOrderStreamState.*|SpotOrderBookProcessUpdateAcceptsEventsWithoutPreviousUpdateID|MapOrderFill.*|SpotAdapter_WatchFillsLive)$' -count=1`

Expected: PASS for unit tests. Skip or separate the live test if `RUN_FULL` is not set.

- [ ] **Step 7: Commit the Binance refactor**

```bash
git add binance/private_streams.go binance/private_streams_test.go binance/perp_adapter.go binance/spot_adapter.go
git commit -m "refactor: share binance private order stream"
```

## Task 3: Add single-subscription fan-out for Aster

**Files:**
- Create: `aster/private_streams.go`
- Create: `aster/private_streams_test.go`
- Modify: `aster/perp_adapter.go`
- Modify: `aster/spot_adapter.go`

- [ ] **Step 1: Copy the Binance shared-stream state pattern into `aster/private_streams.go`**

```go
type sharedOrderStreamState struct {
	mu         sync.Mutex
	subscribed bool
	orderCB    exchanges.OrderUpdateCallback
	fillCB     exchanges.FillCallback
}
```

- [ ] **Step 2: Write the Aster stream-state tests**

```go
func TestSharedOrderStreamStateSubscribesOnce(t *testing.T) {
	var state sharedOrderStreamState

	require.True(t, state.set(func(*exchanges.Order) {}, nil))
	require.False(t, state.set(nil, func(*exchanges.Fill) {}))
}
```

- [ ] **Step 3: Refactor Aster perp to use `watchPrivateOrders` and `dispatchPrivateOrderUpdate`**

```go
func (a *Adapter) WatchOrders(ctx context.Context, callback exchanges.OrderUpdateCallback) error {
	return a.watchPrivateOrders(ctx, callback, nil)
}

func (a *Adapter) WatchFills(ctx context.Context, callback exchanges.FillCallback) error {
	return a.watchPrivateOrders(ctx, nil, callback)
}
```

- [ ] **Step 4: Make Aster perp and spot order mappings overview-only**

```go
return &exchanges.Order{
	OrderID:        fmt.Sprintf("%d", report.OrderID),
	ClientOrderID:  report.ClientOrderID,
	Symbol:         report.Symbol,
	Side:           side,
	Type:           exchanges.OrderType(report.OrderType),
	Quantity:       parseDecimal(report.Quantity),
	Price:          parseDecimal(report.Price),
	OrderPrice:     parseDecimal(report.Price),
	FilledQuantity: parseDecimal(report.CumulativeFilledQuantity),
	Status:         status,
	Timestamp:      report.TransactionTime,
}
```

- [ ] **Step 5: Run focused Aster tests**

Run: `env GOCACHE=/tmp/go-build go test ./aster -run 'Test(SharedOrderStreamState.*|MapOrderFill.*)$' -count=1`

Expected: PASS

- [ ] **Step 6: Commit the Aster refactor**

```bash
git add aster/private_streams.go aster/private_streams_test.go aster/perp_adapter.go aster/spot_adapter.go
git commit -m "refactor: share aster private order stream"
```

## Task 4: Keep OKX and Backpack single-subscription, but make WatchOrders overview-only

**Files:**
- Modify: `okx/private_streams.go`
- Modify: `okx/perp_adapter.go`
- Modify: `okx/spot_adapter.go`
- Create: `okx/private_streams_test.go`
- Modify: `backpack/streams.go`
- Modify: `backpack/private_mapping.go`
- Modify: `backpack/stream_mapping_test.go`

- [ ] **Step 1: Split OKX order mapping into REST vs stream projections**

```go
func (a *Adapter) mapOrderStream(o *okx.Order) *exchanges.Order {
	return &exchanges.Order{
		OrderID:        o.OrdId,
		ClientOrderID:  o.ClOrdId,
		Symbol:         a.ExtractSymbol(o.InstId),
		Side:           mapOrderSide(o.Side),
		Type:           mapOrderType(o.OrdType),
		Quantity:       parseString(o.Sz).Mul(a.getCtVal(context.Background(), o.InstId)),
		Price:          parseString(o.Px),
		OrderPrice:     parseString(o.Px),
		FilledQuantity: parseString(o.AccFillSz).Mul(a.getCtVal(context.Background(), o.InstId)),
		Status:         mapOrderStatus(o.State),
		Timestamp:      parseTime(o.UTime),
	}
}
```

- [ ] **Step 2: Make `dispatchPrivateOrder` use the stream mapper and keep `mapOrderFill` untouched**

```go
if orderCB != nil {
	orderCB(a.mapOrderStream(order))
}
if fillCB != nil {
	if fill := a.mapOrderFill(order); fill != nil {
		fillCB(fill)
	}
}
```

- [ ] **Step 3: Add OKX tests for single-subscription stop behavior and overview-only order projection**

```go
func TestStopPrivateOrdersKeepsSubscriptionUntilLastConsumerLeaves(t *testing.T) {
	state := okxPrivateOrderStreamState{
		subscribed: true,
		orderCB:    func(*exchanges.Order) {},
		fillCB:     func(*exchanges.Fill) {},
	}

	require.False(t, shouldStopAfterClear(&state, true, false))
	require.True(t, shouldStopAfterClear(&state, false, true))
}
```

- [ ] **Step 4: Split Backpack order mapping into overview-only order mapping and fill mapping**

```go
func mapOrderOverview(raw sdk.OrderUpdateEvent) *exchanges.Order {
	return &exchanges.Order{
		OrderID:        raw.OrderID,
		ClientOrderID:  raw.ClientID.String(),
		Symbol:         extractBaseSymbol(raw.Symbol),
		Side:           mapOrderSide(raw.Side),
		Type:           mapOrderType(raw.OrderType),
		Quantity:       parseDecimal(raw.Quantity),
		Price:          parseDecimal(raw.Price),
		OrderPrice:     parseDecimal(raw.Price),
		FilledQuantity: parseDecimal(raw.ExecutedQuantity),
		Status:         mapOrderStatus(raw.Status),
		Timestamp:      microsToMillis(raw.EngineTimestamp),
	}
}
```

- [ ] **Step 5: Update Backpack dispatch tests to assert fills still come from the same payload**

```go
func TestDispatchPrivateOrderUpdateEmitsOverviewOrderAndFill(t *testing.T) {
	event := sdk.OrderUpdateEvent{
		OrderID:       "o-1",
		Price:         "100",
		FillPrice:     "101",
		FillQuantity:  "0.2",
		ExecutedQuantity: "0.2",
	}

	order := mapOrderOverview(event)
	fill := mapOrderFill(event)

	require.Equal(t, "100", order.OrderPrice.String())
	require.True(t, order.LastFillQuantity.IsZero())
	require.Equal(t, "101", fill.Price.String())
}
```

- [ ] **Step 6: Run focused OKX and Backpack tests**

Run: `env GOCACHE=/tmp/go-build go test ./okx ./backpack -run 'Test(StopPrivateOrdersKeepsSubscriptionUntilLastConsumerLeaves|DispatchPrivateOrderUpdateEmitsOverviewOrderAndFill|MapOrderFill.*)$' -count=1`

Expected: PASS

- [ ] **Step 7: Commit the OKX / Backpack pass**

```bash
git add okx/private_streams.go okx/private_streams_test.go okx/perp_adapter.go okx/spot_adapter.go backpack/streams.go backpack/private_mapping.go backpack/stream_mapping_test.go
git commit -m "refactor: keep watch orders overview-only on shared streams"
```

## Task 5: Normalize separate-topic adapters and run verification

**Files:**
- Modify: `bitget/private_classic.go`
- Create: `bitget/order_stream_mapping_test.go`
- Modify: `grvt/perp_adapter.go`
- Create: `grvt/order_stream_mapping_test.go`
- Modify: `standx/perp_adapter.go`
- Create: `standx/order_stream_mapping_test.go`

- [ ] **Step 1: Add stream-only order mapping wrappers for Bitget**

```go
func mapClassicSpotOrderStream(symbol string, raw sdk.ClassicSpotOrderRecord) *exchanges.Order {
	return &exchanges.Order{
		OrderID:        raw.OrderID,
		ClientOrderID:  raw.ClientOID,
		Symbol:         symbol,
		Side:           mapOrderSide(raw.Side),
		Type:           mapOrderType(raw.OrderType, raw.Force),
		Quantity:       parseDecimal(firstNonEmpty(raw.NewSize, raw.Size)),
		Price:          parseDecimal(raw.Price),
		OrderPrice:     parseDecimal(raw.Price),
		FilledQuantity: parseDecimal(firstNonEmpty(raw.AccBaseVolume, raw.BaseVolume)),
		Status:         mapOrderStatus(raw.Status),
		Timestamp:      ts,
		TimeInForce:    mapTimeInForce(raw.Force),
	}
}
```

- [ ] **Step 2: Update Bitget `WatchOrders` to use the stream-only wrappers and add tests**

```go
func TestMapClassicSpotOrderStreamOmitsExecutionDetail(t *testing.T) {
	order := mapClassicSpotOrderStream("BTC", sdk.ClassicSpotOrderRecord{
		OrderID:       "1",
		Price:         "100",
		FillPrice:     "101",
		PriceAvg:      "102",
		AccBaseVolume: "0.5",
	})

	require.Equal(t, "100", order.OrderPrice.String())
	require.True(t, order.LastFillPrice.IsZero())
	require.True(t, order.AverageFillPrice.IsZero())
}
```

- [ ] **Step 3: Add a stream-only order mapper for GRVT**

```go
func (a *Adapter) mapGrvtOrderStream(o *grvt.Order) *exchanges.Order {
	order := &exchanges.Order{
		OrderID:        o.OrderID,
		ClientOrderID:  o.Metadata.ClientOrderID,
		Symbol:         a.ExtractSymbol(instrument),
		Side:           side,
		Quantity:       parseGrvtFloat(qty),
		FilledQuantity: filledQuantity,
		Price:          parseGrvtFloat(firstGrvtLimitPrice(o.Legs)),
		OrderPrice:     parseGrvtFloat(firstGrvtLimitPrice(o.Legs)),
		Status:         status,
		Timestamp:      parseGrvtTimestamp(o.Metadata.CreatedTime),
	}
	exchanges.DerivePartialFillStatus(order)
	return order
}
```

- [ ] **Step 4: Add a stream-only order mapper for StandX**

```go
func (a *Adapter) mapSDKOrderToStreamOrder(o standx.Order) exchanges.Order {
	order := exchanges.Order{
		OrderID:        fmt.Sprintf("%d", o.ID),
		Symbol:         a.toAdapterSymbol(o.Symbol),
		Side:           mapSDKSide(o.Side),
		Type:           mapSDKOrderType(o.Type),
		Quantity:       parseDecimal(o.Qty),
		FilledQuantity: parseDecimal(o.FilledQty),
		Price:          parseDecimal(o.Price),
		OrderPrice:     parseDecimal(o.Price),
		Status:         mapSDKStatus(o.Status),
		ClientOrderID:  o.ClOrdID,
	}
	exchanges.DerivePartialFillStatus(&order)
	return order
}
```

- [ ] **Step 5: Run focused adapter-mapping tests**

Run: `env GOCACHE=/tmp/go-build go test ./bitget ./grvt ./standx -run 'Test(MapClassic.*OrderStreamOmitsExecutionDetail|MapGrvtOrderStreamOmitsExecutionDetail|MapSDKOrderToStreamOrderOmitsExecutionDetail)$' -count=1`

Expected: PASS

- [ ] **Step 6: Run repository-wide compile and selected live verification**

Run: `env GOCACHE=/tmp/go-build go test ./... -run '^$' -count=1`

Expected: PASS

Run: `env GOCACHE=/tmp/go-build RUN_FULL=1 go test ./binance -run TestSpotAdapter_WatchFillsLive -count=1 -v -timeout 10m`

Expected: PASS, proving a shared-topic exchange still produces fills correctly.

Run: `env GOCACHE=/tmp/go-build RUN_FULL=1 go test ./lighter -run 'Test(Perp|Spot)Adapter_WatchFillsLive' -count=1 -v -timeout 12m`

Expected: PASS, proving a separate-topic exchange still behaves correctly.

- [ ] **Step 7: Commit the remaining mapping changes**

```bash
git add bitget/private_classic.go bitget/order_stream_mapping_test.go grvt/perp_adapter.go grvt/order_stream_mapping_test.go standx/perp_adapter.go standx/order_stream_mapping_test.go
git commit -m "refactor: keep watch orders free of execution detail"
```

## Self-Review Checklist

- [ ] Every shared-topic adapter in scope is covered: Binance, Aster, OKX, Backpack.
- [ ] Every separate-topic adapter that currently leaks execution-detail into `WatchOrders` is covered: Bitget, GRVT, StandX.
- [ ] REST order mappers are not accidentally downgraded when the change only targets streaming semantics.
- [ ] The plan keeps `OrderPrice`, `Quantity`, `FilledQuantity`, and `Status` intact for `WatchOrders`.
- [ ] Live verification still covers one shared-topic exchange and one separate-topic exchange.

## Notes For Execution

- Start implementation in a fresh worktree from `main`, because the current workspace has unrelated dirty files.
- Keep commits small and scoped to each task group.
- Do not remove `AverageFillPrice`, `LastFillPrice`, or `LastFillQuantity` from the `Order` struct in this pass; only stop relying on them in `WatchOrders`.
