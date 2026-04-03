# Order Transport Split Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Split order-writing methods by transport in the shared interfaces, remove `OrderMode`, and add explicit WS write helpers plus LocalState tracking support.

**Architecture:** Make the shared contract explicit first so the compiler and unit tests define the new boundary. Then migrate LocalState and test doubles, followed by adapters that currently branch on `OrderMode`, moving their WS logic into explicit `*WS` methods and making REST-only adapters return `ErrNotSupported`.

**Tech Stack:** Go, shared interfaces in `exchange.go`, adapter packages, `go test`, repository unit tests.

---

## File Structure

- Modify: `exchange.go`
  - Remove `OrderMode` and add `PlaceOrderWS`, `CancelOrderWS`, `ModifyOrderWS`.
- Modify: `base_adapter.go`
  - Remove stored order transport mode and helper methods.
- Modify: `local_state.go`
  - Add `PlaceOrderWS`.
- Modify: `local_state_test.go`
  - Add red-green coverage for WS submission tracking.
- Modify: `convenience_order_test.go`
  - Update root stub to satisfy the new interface.
- Modify: `config/config_test.go`
  - Update config test stub to satisfy the new interface.
- Modify: `bitget/order_request_test.go`
  - Update local stub to satisfy the new interface.
- Modify: adapter files that currently branch on `IsRESTMode()`:
  - `binance/perp_adapter.go`
  - `binance/spot_adapter.go`
  - `okx/perp_adapter.go`
  - `okx/spot_adapter.go`
  - `lighter/perp_adapter.go`
  - `lighter/spot_adapter.go`
  - `nado/perp_adapter.go`
  - `hyperliquid/perp_adapter.go`
  - `bitget/private_classic.go`
  - `standx/perp_adapter.go`
  - `grvt/perp_adapter.go`
- Modify: REST-default constructor/tests/docs that still mention `OrderMode`
  - `backpack/constructor_test.go`
  - `bitget/ws_order_mode_test.go`
  - `bitget/adapter_test.go`
  - `grvt/auth_test.go`
  - `README.md`
  - `README_CN.md`

### Task 1: Change the shared contract first

**Files:**
- Modify: `exchange.go`
- Modify: `base_adapter.go`
- Modify: `convenience_order_test.go`
- Modify: `config/config_test.go`
- Modify: `bitget/order_request_test.go`

- [ ] **Step 1: Write the failing root-interface test updates**

```go
func (s *stubExchange) PlaceOrderWS(context.Context, *exchanges.OrderParams) error {
	return nil
}

func (s *stubExchange) CancelOrderWS(context.Context, string, string) error {
	return nil
}
```

Also add the same new methods to the config and Bitget local test stubs so the repository no longer compiles against the old interface.

- [ ] **Step 2: Run focused root tests to verify compile failure against the old interface**

Run: `env GOCACHE=/tmp/go-build go test . ./config ./bitget -run 'Test(PlaceMarketOrderAcceptsOptionalPrice|PlaceMarketOrderWithSlippageAcceptsOptionalPrice|ApplySlippageUsesProvidedMarketReferencePrice|LoadManagerYAMLBuildsAdaptersAndExpandsEnv|ToPlaceOrderRequestConvertsSpotMarketBuyToQuoteQty)$' -count=1`

Expected: FAIL with interface or missing-method errors until `exchange.go` is updated.

- [ ] **Step 3: Update the shared interfaces in `exchange.go`**

```go
type Exchange interface {
	// === Trading ===
	PlaceOrder(ctx context.Context, params *OrderParams) (*Order, error)
	PlaceOrderWS(ctx context.Context, params *OrderParams) error
	CancelOrder(ctx context.Context, orderID, symbol string) error
	CancelOrderWS(ctx context.Context, orderID, symbol string) error
	CancelAllOrders(ctx context.Context, symbol string) error
	FetchOrderByID(ctx context.Context, orderID, symbol string) (*Order, error)
	FetchOrders(ctx context.Context, symbol string) ([]Order, error)
	FetchOpenOrders(ctx context.Context, symbol string) ([]Order, error)
}

type PerpExchange interface {
	Exchange
	ModifyOrder(ctx context.Context, orderID, symbol string, params *ModifyOrderParams) (*Order, error)
	ModifyOrderWS(ctx context.Context, orderID, symbol string, params *ModifyOrderParams) error
}
```

- [ ] **Step 4: Remove `OrderMode` from `exchange.go` and `base_adapter.go`**

```go
// Delete:
type OrderMode string
const (
	OrderModeWS   OrderMode = "ws"
	OrderModeREST OrderMode = "rest"
)
```

```go
type BaseAdapter struct {
	Name       string
	MarketType MarketType
	Logger     Logger
	// no orderMode field
}
```

Delete:

```go
func (b *BaseAdapter) SetOrderMode(mode OrderMode) {}
func (b *BaseAdapter) GetOrderMode() OrderMode {}
func (b *BaseAdapter) IsRESTMode() bool {}
```

- [ ] **Step 5: Re-run the focused root tests**

Run: `env GOCACHE=/tmp/go-build go test . ./config ./bitget -run 'Test(PlaceMarketOrderAcceptsOptionalPrice|PlaceMarketOrderWithSlippageAcceptsOptionalPrice|ApplySlippageUsesProvidedMarketReferencePrice|LoadManagerYAMLBuildsAdaptersAndExpandsEnv|ToPlaceOrderRequestConvertsSpotMarketBuyToQuoteQty)$' -count=1`

Expected: PASS or move on to the next compile failures in adapter packages.

### Task 2: Add LocalState WS tracking with a red-green test

**Files:**
- Modify: `local_state.go`
- Modify: `local_state_test.go`

- [ ] **Step 1: Write the failing test for `LocalState.PlaceOrderWS`**

```go
func (s *localStateStubExchange) PlaceOrderWS(ctx context.Context, params *exchanges.OrderParams) error {
	updates := append([]*exchanges.Order(nil), s.updates...)
	if s.orderCB != nil && len(updates) > 0 {
		go func() {
			for _, update := range updates {
				select {
				case <-ctx.Done():
					return
				case <-time.After(10 * time.Millisecond):
				}
				copy := *update
				s.orderCB(&copy)
			}
		}()
	}
	return nil
}

func TestLocalStatePlaceOrderWSBackfillsOrderIDFromUpdates(t *testing.T) {
	adp := &localStateStubExchange{
		updates: []*exchanges.Order{{
			OrderID:       "ws-exch-1",
			ClientOrderID: "ws-cli-1",
			Symbol:        "ETH",
			Status:        exchanges.OrderStatusNew,
		}},
	}

	state := exchanges.NewLocalState(adp, nil)
	require.NoError(t, state.Start(context.Background()))

	result, err := state.PlaceOrderWS(context.Background(), &exchanges.OrderParams{
		Symbol:   "ETH",
		Side:     exchanges.OrderSideBuy,
		Type:     exchanges.OrderTypeLimit,
		Quantity: decimal.RequireFromString("0.1"),
		Price:    decimal.RequireFromString("100"),
		ClientID: "ws-cli-1",
	})
	require.NoError(t, err)
	require.Equal(t, "ws-cli-1", result.Order.ClientOrderID)
	require.Eventually(t, func() bool { return result.Order.OrderID == "ws-exch-1" }, time.Second, 10*time.Millisecond)
}
```

- [ ] **Step 2: Run the LocalState test and verify it fails**

Run: `env GOCACHE=/tmp/go-build go test . -run 'TestLocalStatePlaceOrder(BackfillsOrderIDFromUpdates|WSBackfillsOrderIDFromUpdates)$' -count=1`

Expected: FAIL because `LocalState` does not yet expose `PlaceOrderWS`.

- [ ] **Step 3: Implement `LocalState.PlaceOrderWS` minimally**

```go
func (s *LocalState) PlaceOrderWS(ctx context.Context, params *OrderParams) (*OrderResult, error) {
	if strings.TrimSpace(params.ClientID) == "" {
		return nil, fmt.Errorf("client id required for PlaceOrderWS")
	}

	allSub := s.orderBus.Subscribe()
	if err := s.adp.PlaceOrderWS(ctx, params); err != nil {
		allSub.Unsubscribe()
		return nil, err
	}

	result := &OrderResult{
		Order: &Order{
			ClientOrderID: params.ClientID,
			Symbol:        params.Symbol,
			Side:          params.Side,
			Type:          params.Type,
			Quantity:      params.Quantity,
			Price:         params.Price,
			Status:        OrderStatusPending,
			Timestamp:     time.Now().UnixMilli(),
		},
		cancel: allSub.Unsubscribe,
	}

	// reuse the same filtered-subscription pattern as PlaceOrder
	return result, nil
}
```

- [ ] **Step 4: Re-run the LocalState test**

Run: `env GOCACHE=/tmp/go-build go test . -run 'TestLocalStatePlaceOrder(BackfillsOrderIDFromUpdates|WSBackfillsOrderIDFromUpdates)$' -count=1`

Expected: PASS

### Task 3: Migrate one explicit-dual-transport adapter slice first

**Files:**
- Modify: `binance/perp_adapter.go`
- Modify: `binance/spot_adapter.go`
- Modify: `okx/perp_adapter.go`
- Modify: `okx/spot_adapter.go`

- [ ] **Step 1: Write or update focused adapter tests to exercise explicit method names**

```go
func TestBinancePerpPlaceOrderWSRequiresClientID(t *testing.T) {
	adp := newTestAdapter(...)
	err := adp.PlaceOrderWS(context.Background(), &exchanges.OrderParams{
		Symbol:   "BTC",
		Side:     exchanges.OrderSideBuy,
		Type:     exchanges.OrderTypeLimit,
		Quantity: decimal.RequireFromString("0.1"),
		Price:    decimal.RequireFromString("100"),
	})
	require.Error(t, err)
}
```

Use the same pattern for any adapter-local tests you add: explicit WS method, no `SetOrderMode`, and no returned `*Order`.

- [ ] **Step 2: Run the focused adapter tests and verify failure**

Run: `env GOCACHE=/tmp/go-build go test ./binance ./okx -run 'Test.*(PlaceOrderWS|CancelOrderWS|ModifyOrderWS).*' -count=1`

Expected: FAIL because adapters still expose transport switching inside unsuffixed methods.

- [ ] **Step 3: Split the Binance and OKX methods**

```go
func (a *Adapter) PlaceOrder(ctx context.Context, params *exchanges.OrderParams) (*exchanges.Order, error) {
	resp, err := a.client.PlaceOrder(ctx, p)
	if err != nil {
		return nil, err
	}
	return a.normalizeOrderResponse(resp)
}

func (a *Adapter) PlaceOrderWS(ctx context.Context, params *exchanges.OrderParams) error {
	if strings.TrimSpace(params.ClientID) == "" {
		return fmt.Errorf("client id required for PlaceOrderWS")
	}
	if err := a.WsOrderConnected(ctx); err != nil {
		return err
	}
	_, err := a.wsAPI.PlaceOrderWS(a.apiKey, a.secretKey, p, reqID)
	return err
}
```

Use the same split for `CancelOrder` / `CancelOrderWS` and `ModifyOrder` / `ModifyOrderWS`.

- [ ] **Step 4: Re-run the focused adapter tests**

Run: `env GOCACHE=/tmp/go-build go test ./binance ./okx -run 'Test.*(PlaceOrderWS|CancelOrderWS|ModifyOrderWS).*' -count=1`

Expected: PASS

### Task 4: Remove `OrderMode` call sites and migrate the remaining adapters

**Files:**
- Modify: `lighter/perp_adapter.go`
- Modify: `lighter/spot_adapter.go`
- Modify: `nado/perp_adapter.go`
- Modify: `hyperliquid/perp_adapter.go`
- Modify: `bitget/private_classic.go`
- Modify: `standx/perp_adapter.go`
- Modify: `grvt/perp_adapter.go`
- Modify: `backpack/perp_adapter.go`
- Modify: `backpack/spot_adapter.go`
- Modify: `backpack/constructor_test.go`
- Modify: `bitget/ws_order_mode_test.go`
- Modify: `bitget/adapter_test.go`
- Modify: `grvt/auth_test.go`

- [ ] **Step 1: Replace each `if a.IsRESTMode()` branch with explicit methods**

```go
// Before:
if a.IsRESTMode() {
	...
}
...

// After:
func (a *Adapter) PlaceOrder(ctx context.Context, params *exchanges.OrderParams) (*exchanges.Order, error) {
	// old REST body only
}

func (a *Adapter) PlaceOrderWS(ctx context.Context, params *exchanges.OrderParams) error {
	if strings.TrimSpace(params.ClientID) == "" {
		return fmt.Errorf("client id required for PlaceOrderWS")
	}
	// old WS body only
}
```

- [ ] **Step 2: Make REST-only adapters explicit**

```go
func (a *Adapter) PlaceOrderWS(context.Context, *exchanges.OrderParams) error {
	return exchanges.ErrNotSupported
}

func (a *Adapter) CancelOrderWS(context.Context, string, string) error {
	return exchanges.ErrNotSupported
}
```

Use the same pattern for `ModifyOrderWS` on perp adapters that do not support WS modify.

- [ ] **Step 3: Rewrite `OrderMode`-based tests to explicit WS tests**

```go
func TestClassicPlaceOrderWSRoutesToWS(t *testing.T) {
	adp := newClassicSpotOrderModeTestAdapter(t, restServer.URL, wsServer)
	err := adp.PlaceOrderWS(context.Background(), &exchanges.OrderParams{
		Symbol:      "BTC",
		Side:        exchanges.OrderSideBuy,
		Type:        exchanges.OrderTypeLimit,
		Quantity:    decimal.RequireFromString("0.1"),
		Price:       decimal.RequireFromString("100"),
		TimeInForce: exchanges.TimeInForceGTC,
		ClientID:    "cid-classic",
	})
	require.NoError(t, err)
	require.Equal(t, int32(0), restHits.Load())
}
```

Delete assertions that constructors default to `OrderModeREST`; they are obsolete once `OrderMode` is removed.

- [ ] **Step 4: Run the focused package tests**

Run: `env GOCACHE=/tmp/go-build go test ./backpack ./bitget ./grvt ./lighter ./nado ./hyperliquid ./standx -run 'Test(UnsupportedMethodsReturnErrNotSupported|Classic.*WS.*|BitgetWS.*|LocalStatePlaceOrder.*|.*Unsupported.*|.*PlaceOrderWS.*|.*CancelOrderWS.*|.*ModifyOrderWS.*)$' -count=1`

Expected: PASS except for tests that still depend on local listeners in the sandbox. If a `httptest`-based case is blocked only by the sandbox, record that explicitly and keep the compile-focused coverage green.

### Task 5: Update docs and run final focused verification

**Files:**
- Modify: `README.md`
- Modify: `README_CN.md`

- [ ] **Step 1: Remove `OrderMode` guidance and document explicit WS methods**

```md
Use `PlaceOrder` when you want the adapter's primary non-WS write path and a returned order object.

Use `PlaceOrderWS` when you want explicit WebSocket submission. `PlaceOrderWS` returns only an error and requires `OrderParams.ClientID` so the order can be tracked later through `WatchOrders`.
```

- [ ] **Step 2: Run final focused verification**

Run: `env GOCACHE=/tmp/go-build go test . ./config ./backpack ./binance ./bitget ./grvt ./hyperliquid ./lighter ./nado ./okx ./standx -run 'Test(LocalStatePlaceOrder(BackfillsOrderIDFromUpdates|WSBackfillsOrderIDFromUpdates)|PlaceMarketOrderAcceptsOptionalPrice|PlaceMarketOrderWithSlippageAcceptsOptionalPrice|ApplySlippageUsesProvidedMarketReferencePrice|LoadManagerYAMLBuildsAdaptersAndExpandsEnv|ToPlaceOrderRequestConvertsSpotMarketBuyToQuoteQty|.*Unsupported.*|.*PlaceOrderWS.*|.*CancelOrderWS.*|.*ModifyOrderWS.*|Classic.*WS.*|BitgetWS.*)$' -count=1`

Expected: PASS for the non-network, non-live, non-sandbox-blocked tests that cover this feature.
