# Exchange Adapter Layering Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn the exchange adapter layering spec into an enforceable repository baseline and complete the first convergence pass across `backpack`, `bitget`, `binance`, and `okx`.

**Architecture:** Implement the standard from the outside in. First create durable enforcement artifacts so future reviews have a single checklist and package gap docs. Then converge runtime behavior in the most divergent packages first: make Backpack explicitly REST-only and normalize its SDK naming, classify Bitget as a controlled hybrid transport adapter, and finally clean up sentinel-error and constructor-auth drift in Binance and OKX.

**Tech Stack:** Go, repository adapter packages, shared `testsuite`, `shopspring/decimal`, spec and checklist docs under `docs/superpowers`

---

## Planned File Map

- Modify: `docs/superpowers/specs/2026-03-23-exchange-adapter-layering-design.md`
  Purpose: keep the approved spec aligned with the implemented rollout state and explicitly record deferred decisions.
- Create: `docs/superpowers/checklists/exchange-adapter-review.md`
  Purpose: actionable review checklist derived from the spec.
- Create: `docs/superpowers/gaps/2026-03-23-backpack-adapter-gap.md`
  Purpose: Backpack-specific convergence checklist with keep/change/defer decisions.
- Create: `docs/superpowers/gaps/2026-03-23-bitget-adapter-gap.md`
  Purpose: Bitget-specific convergence checklist with keep/change/defer decisions.
- Modify: `backpack/perp_adapter.go`
- Modify: `backpack/spot_adapter.go`
- Modify: `backpack/perp_streams.go`
- Modify: `backpack/spot_streams.go`
- Modify: `backpack/sdk/client.go`
- Modify: `backpack/sdk/public_rest.go`
- Modify: `backpack/sdk/private_rest.go`
- Modify: `backpack/registry_test.go`
- Create: `backpack/sdk/private_rest_test.go`
- Create: `backpack/constructor_test.go`
- Create: `backpack/transport_test.go`
  Purpose: make Backpack explicitly REST-only, normalize naming, and strengthen behavior tests.
- Modify: `bitget/perp_adapter.go`
- Modify: `bitget/spot_adapter.go`
- Modify: `bitget/private_profile.go`
- Modify: `bitget/private_classic.go`
- Create: `bitget/funding.go`
- Create: `bitget/funding_test.go`
- Modify: `bitget/ws_order_mode_test.go`
  Purpose: classify Bitget as a controlled hybrid transport adapter and normalize file placement.
- Modify: `binance/spot_adapter.go`
- Modify: `binance/margin_adapter.go`
- Create: `binance/unsupported_test.go`
- Modify: `okx/perp_adapter.go`
- Modify: `okx/spot_adapter.go`
- Modify: `okx/adapter_test.go`
- Create: `okx/unsupported_test.go`
  Purpose: replace free-form unsupported-path errors with shared sentinel behavior.
- Modify: `binance/options.go`
- Modify: `binance/perp_adapter.go`
- Modify: `binance/spot_adapter.go`
- Create: `binance/options_test.go`
- Create: `binance/constructor_test.go`
- Modify: `okx/options.go`
- Modify: `okx/perp_adapter.go`
- Modify: `okx/spot_adapter.go`
- Create: `okx/options_test.go`
- Create: `okx/constructor_test.go`
- Modify: `backpack/options.go`
- Modify: `backpack/perp_adapter.go`
- Modify: `backpack/spot_adapter.go`
- Modify: `backpack/options_test.go`
- Modify: `backpack/constructor_test.go`
  Purpose: converge constructor credential handling on “empty creds allowed, partial creds rejected”.

### Task 1: Land Enforcement Artifacts And Gap Docs

**Files:**
- Modify: `docs/superpowers/specs/2026-03-23-exchange-adapter-layering-design.md`
- Create: `docs/superpowers/checklists/exchange-adapter-review.md`
- Create: `docs/superpowers/gaps/2026-03-23-backpack-adapter-gap.md`
- Create: `docs/superpowers/gaps/2026-03-23-bitget-adapter-gap.md`

- [ ] **Step 1: Verify the new enforcement files do not exist yet**

Run: `test -f docs/superpowers/checklists/exchange-adapter-review.md || echo missing`
Expected: output contains `missing`

Run: `test -f docs/superpowers/gaps/2026-03-23-backpack-adapter-gap.md || echo missing`
Expected: output contains `missing`

- [ ] **Step 2: Align the spec with the initial rollout scope**

Update the spec so it:

- records the package classifications being implemented in this first pass
- keeps unresolved repository-wide decisions explicitly deferred
- does not claim that later-phase naming or file-layout questions have already been settled

- [ ] **Step 3: Write the review checklist document**

Create `docs/superpowers/checklists/exchange-adapter-review.md` by copying the full 10-item review checklist from the spec and turning it into an execution artifact. Do not shorten it.

```md
# Exchange Adapter Review Checklist

1. constructor semantics are explicit and stable
2. base-symbol semantics are preserved or compatibility exceptions are explicit
3. sentinel errors are used consistently
4. transport classification is explicit and accurate
5. adapter/helper/SDK boundaries are clear
6. exchange-specific complexity is concentrated
7. main adapter files remain the primary entrypoints
8. SDK naming aligns with repository conventions
9. each concrete market adapter satisfies the minimum test matrix
10. deviations are explicitly justified
```

- [ ] **Step 4: Write the Backpack gap doc**

Create a short gap doc with three sections:

- Keep
- Change
- Defer

The `Change` section must include:

- explicit REST-only classification
- SDK naming convergence
- sentinel-error cleanup
- stronger SDK-level tests

- [ ] **Step 5: Write the Bitget gap doc**

Create the same three-section structure for Bitget.

The `Change` section must include:

- controlled hybrid transport classification
- funding code placement
- private-profile boundary documentation
- tests that explicitly cover the hybrid contract

- [ ] **Step 6: Commit the enforcement artifacts**

```bash
git add docs/superpowers/specs/2026-03-23-exchange-adapter-layering-design.md docs/superpowers/checklists/exchange-adapter-review.md docs/superpowers/gaps/2026-03-23-backpack-adapter-gap.md docs/superpowers/gaps/2026-03-23-bitget-adapter-gap.md
git commit -m "docs: add adapter review artifacts"
```

### Task 2: Make Backpack Explicitly REST-Only And Fix Sentinel Semantics

**Files:**
- Modify: `backpack/perp_adapter.go`
- Modify: `backpack/spot_adapter.go`
- Modify: `backpack/perp_streams.go`
- Modify: `backpack/spot_streams.go`
- Modify: `backpack/sdk/client.go`
- Modify: `backpack/registry_test.go`
- Create: `backpack/constructor_test.go`
- Create: `backpack/transport_test.go`

- [ ] **Step 1: Add deterministic Backpack constructor seams**

First add SDK-level test seams in `backpack/sdk/client.go`:

```go
func (c *Client) WithBaseURL(baseURL string) *Client
func (c *Client) WithHTTPClient(hc *http.Client) *Client
```

Then introduce unexported adapter helpers that mirror Bitget’s testable constructor shape:

```go
func newPerpAdapterWithClient(ctx context.Context, cancel context.CancelFunc, opts Options, quote exchanges.QuoteCurrency, client *sdk.Client) (*Adapter, error)
func newSpotAdapterWithClient(ctx context.Context, cancel context.CancelFunc, opts Options, quote exchanges.QuoteCurrency, client *sdk.Client) (*SpotAdapter, error)
```

Use the SDK seam plus these helpers in the exported constructors immediately so later tests hit the real constructor path.

- [ ] **Step 2: Write the failing transport and sentinel tests**

Add tests like:

```go
func TestNewPerpAdapterWithClientDefaultsToRESTOrderMode(t *testing.T) {
	adp, err := newPerpAdapterWithClient(context.Background(), func() {}, Options{}, exchanges.QuoteCurrencyUSDC, newTestClient(...))
	require.NoError(t, err)
	require.Equal(t, exchanges.OrderModeREST, adp.GetOrderMode())
}

func TestSpotFetchBalanceMissingQuoteReturnsErrSymbolNotFound(t *testing.T) {
	adp, err := newSpotAdapterWithClient(context.Background(), func() {}, Options{}, exchanges.QuoteCurrencyUSDC, newTestClientReturningBalancesWithoutQuote(...))
	require.NoError(t, err)
	_, err := adp.FetchBalance(context.Background())
	require.ErrorIs(t, err, exchanges.ErrSymbolNotFound)
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./backpack -run 'TestNewPerpAdapterWithClientDefaultsToRESTOrderMode|TestSpotFetchBalanceMissingQuoteReturnsErrSymbolNotFound' -v`
Expected: FAIL because the new helper constructors do not exist yet and missing quote balance still returns a string error.

- [ ] **Step 4: Set explicit REST-only transport in Backpack constructors**

Update both constructors to set:

```go
base := exchanges.NewBaseAdapter("BACKPACK", marketType, opts.logger())
base.SetOrderMode(exchanges.OrderModeREST)
```

Also add short comments in the adapter files explaining that Backpack is currently a REST-only order transport adapter.

- [ ] **Step 5: Replace Backpack’s stable free-form errors with sentinel errors**

At minimum:

- change missing quote-balance failures to `exchanges.ErrSymbolNotFound`
- verify all unsupported stream/account methods return `exchanges.ErrNotSupported`
- keep protocol/detail text only when wrapping with shared sentinel semantics

- [ ] **Step 6: Re-run Backpack package tests**

Run: `go test ./backpack -run 'TestNewPerpAdapterWithClientDefaultsToRESTOrderMode|TestSpotFetchBalanceMissingQuoteReturnsErrSymbolNotFound|TestUnsupportedMethodsReturnErrNotSupported|TestSpotWatchPositionsReturnErrNotSupported' -v`
Expected: PASS

- [ ] **Step 7: Commit the Backpack transport/error cleanup**

```bash
git add backpack/perp_adapter.go backpack/spot_adapter.go backpack/perp_streams.go backpack/spot_streams.go backpack/sdk/client.go backpack/registry_test.go backpack/constructor_test.go backpack/transport_test.go
git commit -m "refactor: classify backpack as rest only"
```

### Task 3: Normalize Backpack SDK Naming With Compatibility Shims

**Files:**
- Modify: `backpack/sdk/public_rest.go`
- Modify: `backpack/sdk/private_rest.go`
- Modify: `backpack/perp_adapter.go`
- Modify: `backpack/spot_adapter.go`
- Modify: `backpack/sdk/public_rest_test.go`
- Create: `backpack/sdk/private_rest_test.go`

- [ ] **Step 1: Write the failing SDK naming tests**

Add tests that assert the preferred names exist and delegate correctly:

```go
func TestClientPlaceOrderDelegatesToExistingOrderExecutionPath(t *testing.T) {}
func TestClientGetOrderBookDelegatesToDepthEndpoint(t *testing.T) {}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./backpack/sdk -run 'TestClientPlaceOrderDelegatesToExistingOrderExecutionPath|TestClientGetOrderBookDelegatesToDepthEndpoint' -v`
Expected: FAIL because the preferred wrapper names do not exist yet.

- [ ] **Step 3: Add preferred SDK wrapper names**

Introduce preferred names while keeping compatibility wrappers temporarily:

```go
func (c *Client) GetOrderBook(ctx context.Context, symbol string, limit int) (*Depth, error) {
	return c.GetDepth(ctx, symbol, limit)
}

func (c *Client) PlaceOrder(ctx context.Context, req CreateOrderRequest) (*Order, error) {
	return c.ExecuteOrder(ctx, req)
}
```

- [ ] **Step 4: Switch Backpack adapters to the preferred names**

Update `backpack/perp_adapter.go` and `backpack/spot_adapter.go` to call `GetOrderBook` and `PlaceOrder` instead of `GetDepth` and `ExecuteOrder`.

- [ ] **Step 5: Re-run Backpack SDK and adapter tests**

Run: `go test ./backpack/sdk ./backpack -run 'TestClientPlaceOrderDelegatesToExistingOrderExecutionPath|TestClientGetOrderBookDelegatesToDepthEndpoint|TestPerpFormatSymbolUsesMarketCache|TestSpotFormatSymbolUsesMarketCache' -v`
Expected: PASS

- [ ] **Step 6: Leave WS client naming explicitly deferred**

Do not rename `WSClient` in this task. Add one short note to the Backpack gap doc stating that WS client naming remains deferred until the repository resolves the spec’s open WS naming decision.

- [ ] **Step 7: Commit the Backpack SDK naming convergence**

```bash
git add backpack/sdk/public_rest.go backpack/sdk/private_rest.go backpack/sdk/public_rest_test.go backpack/sdk/private_rest_test.go backpack/perp_adapter.go backpack/spot_adapter.go docs/superpowers/gaps/2026-03-23-backpack-adapter-gap.md
git commit -m "refactor: normalize backpack sdk naming"
```

### Task 4: Classify Bitget As A Controlled Hybrid Adapter

**Files:**
- Modify: `bitget/perp_adapter.go`
- Modify: `bitget/spot_adapter.go`
- Modify: `bitget/private_profile.go`
- Modify: `bitget/private_classic.go`
- Modify: `bitget/ws_order_mode_test.go`
- Create: `bitget/funding.go`
- Create: `bitget/funding_test.go`

- [ ] **Step 1: Capture the current Bitget transport/funding inventory**

Run: `rg -n "FetchFundingRate|FetchAllFundingRates|SetOrderMode|IsRESTMode" bitget`
Expected: output shows funding methods still live in `bitget/perp_adapter.go` and transport switching logic is split across the adapter and `private_classic.go`.

- [ ] **Step 2: Move perp funding behavior into `bitget/funding.go`**

Extract:

```go
func (a *Adapter) FetchFundingRate(ctx context.Context, symbol string) (*exchanges.FundingRate, error)
func (a *Adapter) FetchAllFundingRates(ctx context.Context) ([]exchanges.FundingRate, error)
```

Keep the current explicit unsupported behavior unless a real implementation is being added in the same change.

- [ ] **Step 3: Add deterministic funding behavior tests**

Create `bitget/funding_test.go` with explicit unsupported assertions:

```go
func TestBitgetFundingMethodsRemainExplicitlyUnsupported(t *testing.T) {
	adp := &Adapter{}
	_, err := adp.FetchFundingRate(context.Background(), "BTC")
	require.ErrorIs(t, err, exchanges.ErrNotSupported)
}
```

- [ ] **Step 4: Document the hybrid/private-profile boundary in code**

Add concise comments explaining:

- why `privateProfile` exists
- why classic mode is currently the only private transport/profile implementation
- which order operations currently participate in transport switching

- [ ] **Step 5: Re-run Bitget tests**

Run: `go test ./bitget -run 'TestClassicOrderModeWSRoutesPlaceOrderToWS|TestClassicOrderModeWSRoutesCancelOrderToWS|TestBitgetWSOrderModeDoesNotSilentlyFallbackToREST|TestBitgetConstructorsDefaultToRESTOrderMode|TestBitgetFundingMethodsRemainExplicitlyUnsupported' -v`
Expected: PASS

- [ ] **Step 6: Commit the Bitget hybrid cleanup**

```bash
git add bitget/perp_adapter.go bitget/spot_adapter.go bitget/private_profile.go bitget/private_classic.go bitget/ws_order_mode_test.go bitget/funding.go bitget/funding_test.go
git commit -m "refactor: classify bitget hybrid transport"
```

### Task 5: Normalize Binance And OKX Unsupported-Path Errors

**Files:**
- Modify: `binance/spot_adapter.go`
- Modify: `binance/margin_adapter.go`
- Create: `binance/unsupported_test.go`
- Modify: `okx/perp_adapter.go`
- Modify: `okx/spot_adapter.go`
- Modify: `okx/adapter_test.go`
- Create: `okx/unsupported_test.go`

- [ ] **Step 1: Write the failing sentinel tests**

Add tests like:

```go
func TestBinanceSpotUnsupportedPathsUseSentinelErrors(t *testing.T) {}
func TestOKXUnsupportedPathsUseSentinelErrors(t *testing.T) {}
```

Cover at minimum:

- `TransferAsset`
- `WatchPositions`
- `StopWatchPositions`
- obvious stable “not implemented” paths

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./binance ./okx -run 'TestBinanceSpotUnsupportedPathsUseSentinelErrors|TestOKXUnsupportedPathsUseSentinelErrors' -v`
Expected: FAIL because several methods still return `fmt.Errorf(...)` or `nil`.

- [ ] **Step 3: Replace free-form unsupported responses with shared sentinel errors**

At minimum make these paths consistent:

```go
return exchanges.ErrNotSupported
```

Use `ErrSymbolNotFound` for stable symbol-miss cases where that is the correct contract.

- [ ] **Step 4: Add the missing OKX spot local-state suite**

Update `okx/adapter_test.go` to add the missing shared suite coverage:

```go
func TestSpotAdapter_LocalState(t *testing.T) {
	adp := setupSpotAdapter(t)
	testsuite.RunLocalStateSuite(t, adp, testsuite.LocalStateConfig{Symbol: "DOGE"})
}
```

- [ ] **Step 5: Re-run targeted tests**

Run: `go test ./binance ./okx -run 'TestBinanceSpotUnsupportedPathsUseSentinelErrors|TestOKXUnsupportedPathsUseSentinelErrors|TestSpotAdapter_LocalState' -v`
Expected: PASS

- [ ] **Step 6: Commit the sentinel cleanup**

```bash
git add binance/spot_adapter.go binance/margin_adapter.go binance/unsupported_test.go okx/perp_adapter.go okx/spot_adapter.go okx/adapter_test.go okx/unsupported_test.go
git commit -m "refactor: normalize adapter unsupported errors"
```

### Task 6: Converge Constructor Credential Handling

**Files:**
- Modify: `binance/options.go`
- Modify: `binance/perp_adapter.go`
- Modify: `binance/spot_adapter.go`
- Create: `binance/options_test.go`
- Create: `binance/constructor_test.go`
- Modify: `okx/options.go`
- Modify: `okx/perp_adapter.go`
- Modify: `okx/spot_adapter.go`
- Create: `okx/options_test.go`
- Create: `okx/constructor_test.go`
- Modify: `backpack/options.go`
- Modify: `backpack/perp_adapter.go`
- Modify: `backpack/spot_adapter.go`
- Modify: `backpack/options_test.go`
- Modify: `backpack/constructor_test.go`

- [ ] **Step 1: Write the failing credential-validation helper tests**

Add deterministic tests around package-local validation helpers rather than exported constructors:

```go
func TestValidateCredentialsAllowsEmptySet(t *testing.T) {}
func TestValidateCredentialsRejectsPartialSet(t *testing.T) {}
```

For OKX, partial means any subset of `api_key`, `secret_key`, `passphrase`.
For Backpack, partial means `api_key` without `private_key`, or vice versa.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./binance ./okx ./backpack -run 'TestValidateCredentialsAllowsEmptySet|TestValidateCredentialsRejectsPartialSet' -v`
Expected: FAIL because the helpers do not exist yet.

- [ ] **Step 3: Add explicit credential validation helpers**

Implement small helpers near `Options` or constructor glue:

```go
func validateCredentials(...) error {
	if allEmpty {
		return nil
	}
	if partial {
		return exchanges.NewExchangeError(exchangeName, "", "all private credential fields must be set together", exchanges.ErrAuthFailed)
	}
	return nil
}
```

- [ ] **Step 4: Add deterministic constructor seams where missing**

For Binance and OKX, add unexported helpers matching the Bitget/Backpack pattern so constructor behavior can be tested without live network calls:

```go
func newPerpAdapterWithClient(...)
func newSpotAdapterWithClient(...)
```

Use the exported constructors as thin wrappers around those helpers.

- [ ] **Step 5: Wire the validation into constructors**

Run validation before private clients are fully constructed so that:

- empty credentials still permit public-only adapters
- partial credentials fail fast with `ErrAuthFailed`

- [ ] **Step 6: Add deterministic constructor tests on the new seams**

Add tests like:

```go
func TestNewAdapterWithClientAllowsPublicOnlyConstruction(t *testing.T) {}
func TestNewAdapterWithClientRejectsPartialCredentials(t *testing.T) {}
```

Reuse Backpack’s constructor seam from Task 2.

- [ ] **Step 7: Re-run validation and constructor tests**

Run: `go test ./binance ./okx ./backpack -run 'TestValidateCredentialsAllowsEmptySet|TestValidateCredentialsRejectsPartialSet|TestNewAdapterWithClientAllowsPublicOnlyConstruction|TestNewAdapterWithClientRejectsPartialCredentials|TestOptionsRejectsUnsupportedQuoteCurrency' -v`
Expected: PASS

- [ ] **Step 8: Commit the constructor-policy convergence**

```bash
git add binance/options.go binance/perp_adapter.go binance/spot_adapter.go binance/options_test.go binance/constructor_test.go okx/options.go okx/perp_adapter.go okx/spot_adapter.go okx/options_test.go okx/constructor_test.go backpack/options.go backpack/perp_adapter.go backpack/spot_adapter.go backpack/options_test.go backpack/constructor_test.go
git commit -m "refactor: reject partial adapter credentials"
```

### Task 7: Final Verification And Documentation Sync

**Files:**
- Modify: `docs/superpowers/specs/2026-03-23-exchange-adapter-layering-design.md`
- Modify: `docs/superpowers/checklists/exchange-adapter-review.md`
- Modify: `docs/superpowers/gaps/2026-03-23-backpack-adapter-gap.md`
- Modify: `docs/superpowers/gaps/2026-03-23-bitget-adapter-gap.md`

- [ ] **Step 1: Run package-level verification on all touched adapters**

Run: `go test ./backpack ./bitget ./binance ./okx -v`
Expected: PASS in the touched packages, including newly added deterministic tests.

- [ ] **Step 2: Run repository compile smoke**

Run: `go test ./... -run TestDoesNotExist`
Expected: PASS compile across the full repository.

- [ ] **Step 3: Update the spec and gap docs to reflect the actual landed state**

Close or narrow any remaining open decisions that the implementation resolved.

Update the gap docs so each unresolved item is explicitly marked:

- deferred
- blocked by exchange constraints
- queued for a later pass

- [ ] **Step 4: Commit the final verification sync**

```bash
git add docs/superpowers/specs/2026-03-23-exchange-adapter-layering-design.md docs/superpowers/checklists/exchange-adapter-review.md docs/superpowers/gaps/2026-03-23-backpack-adapter-gap.md docs/superpowers/gaps/2026-03-23-bitget-adapter-gap.md
git commit -m "docs: finalize adapter layering rollout status"
```
