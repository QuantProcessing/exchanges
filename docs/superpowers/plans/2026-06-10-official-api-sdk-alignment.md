# Official API SDK Alignment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bring the local SDK surface for Binance, OKX, Bybit, Bitget, Hyperliquid, and Lighter into explicit, testable alignment with official spot and perpetual/swap/futures API documentation.

**Architecture:** Keep `sdk/` as the exchange-native protocol layer and keep adapters focused on the unified `exchanges.Exchange` contract. Add an official API parity matrix as the source of truth, then implement missing SDK endpoints in small exchange/product slices with unit tests, mapper tests, and live-test hooks where credentials are available.

**Tech Stack:** Go, repository-local `sdk/` packages, `net/http` test transports, existing `testsuite`, official exchange API documentation, markdown parity artifacts.

---

> Execution order note: execute the architecture boundary work in `docs/superpowers/plans/2026-06-10-architecture-and-sdk-alignment.md` before this endpoint-parity plan. SDK additions remain valid, but adapter exposure must follow the architecture contract in `AGENTS.md`; breaking adapter API changes are allowed.

## Scope And Non-Goals

This plan covers spot and perpetual/swap/futures features exposed by the official API documentation for:

- Binance Spot and USD-M Futures first, COIN-M Futures as a separate Binance futures package after USD-M parity is stable.
- OKX Spot and SWAP.
- Bybit V5 Spot and Linear/Inverse Perpetuals.
- Bitget V2 Spot and Futures.
- Hyperliquid Spot and Perpetuals.
- Lighter Spot and Perpetuals.

This plan does not cover options, margin lending, copy trading, earn, broker, affiliate, tax, institutional loans, or non-trading business lines unless a given endpoint is required to complete the spot/perp trading lifecycle. Existing Binance margin and options code should be left intact, but they are outside this alignment objective.

The stop condition is not "every official product in the universe exists locally." The stop condition is: every official spot/perp endpoint or WebSocket channel in the selected docs has one explicit status in the local parity matrix:

- `missing-sdk`
- `missing-adapter`
- `implemented-sdk`
- `implemented-adapter`
- `implemented-raw`
- `intentionally-unsupported`
- `blocked-by-official-api`
- `deprecated-official`

No endpoint may remain unclassified, and no endpoint may remain `missing-sdk` or `missing-adapter` when the alignment project is declared complete.

## Official Sources To Pin

Use these as the initial source set. Record the accessed date in the parity matrix when implementing Task 1.

- Binance Spot REST/WebSocket docs: <https://developers.binance.com/docs/binance-spot-api-docs/rest-api>
- Binance USD-M Futures docs: <https://developers.binance.com/docs/derivatives/usds-margined-futures/general-info>
- OKX API v5 docs: <https://www.okx.com/docs-v5/en/>
- Bybit V5 docs: <https://bybit-exchange.github.io/docs/v5/intro>
- Bitget API docs: <https://www.bitget.com/api-doc/common/intro>
- Hyperliquid Info endpoint docs: <https://hyperliquid.gitbook.io/hyperliquid-docs/for-developers/api/info-endpoint>
- Hyperliquid Exchange endpoint docs: <https://hyperliquid.gitbook.io/hyperliquid-docs/for-developers/api/exchange-endpoint>
- Lighter API reference: <https://apidocs.lighter.xyz/reference>

## File Structure

Create:

- `docs/superpowers/gaps/official-api-parity.md` - human-readable master coverage matrix and endpoint classification rules.
- `docs/superpowers/gaps/official-api-parity-binance.md` - Binance spot/USD-M/COIN-M endpoint matrix.
- `docs/superpowers/gaps/official-api-parity-okx.md` - OKX spot/swap endpoint matrix.
- `docs/superpowers/gaps/official-api-parity-bybit.md` - Bybit spot/linear/inverse endpoint matrix.
- `docs/superpowers/gaps/official-api-parity-bitget.md` - Bitget spot/futures endpoint matrix.
- `docs/superpowers/gaps/official-api-parity-hyperliquid.md` - Hyperliquid spot/perp endpoint matrix.
- `docs/superpowers/gaps/official-api-parity-lighter.md` - Lighter spot/perp endpoint matrix.
- `sdkparity/manifest.go` - lightweight parser for local parity markdown tables used by tests.
- `sdkparity/manifest_test.go` - parser tests.
- `official_api_parity_test.go` - root-level guard that every matrix row has an allowed status and every `implemented-*` row names a local symbol.

Modify:

- `binance/sdk/spot/*.go`, `binance/sdk/perp/*.go`, and add `binance/sdk/coinm/*.go` after USD-M is stable.
- `okx/sdk/*.go`
- `bybit/sdk/*.go`
- `bitget/sdk/*.go`
- `hyperliquid/sdk/**/*.go`
- `lighter/sdk/*.go`
- Adapter files only when a newly implemented SDK endpoint maps cleanly to an existing unified interface or a small, explicit extension interface.
- `docs/contributing/adding-exchange-adapters.md` only to add the official parity matrix as a required step for future SDK expansion.

Do not restructure adapter packages during this project. Large behavior changes belong in low-level `sdk/` files first, then adapter exposure follows separately.

## Capability Model

Use three layers:

1. `sdk` typed endpoint methods: one Go method per official REST endpoint or typed WebSocket operation when the official API has stable request and response schemas.
2. `sdk` raw fallback methods: one method per endpoint family only when the official schema is broad or frequently changing; the method must still sign, route, and decode official error envelopes.
3. adapter exposure: only for existing unified interfaces or carefully reviewed extension interfaces.

Do not force every official endpoint into `Exchange`. Most official endpoints should remain in `sdk/` until there is a strong cross-exchange abstraction.

## Task 1: Create The Parity Matrix Framework

**Files:**
- Create: `docs/superpowers/gaps/official-api-parity.md`
- Create: `sdkparity/manifest.go`
- Create: `sdkparity/manifest_test.go`
- Create: `official_api_parity_test.go`

- [ ] **Step 1: Write parser tests**

Create `sdkparity/manifest_test.go` with these cases:

```go
package sdkparity

import (
	"strings"
	"testing"
)

func TestParseMarkdownTable(t *testing.T) {
	input := strings.NewReader(`
| Exchange | Product | Method | Path | Status | Local Symbol |
| --- | --- | --- | --- | --- | --- |
| BINANCE | spot | GET | /api/v3/depth | implemented-sdk | binance/sdk/spot.Client.Depth |
| BINANCE | spot | POST | /api/v3/order/oco | intentionally-unsupported |  |
`)

	rows, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0].Exchange != "BINANCE" || rows[0].Path != "/api/v3/depth" {
		t.Fatalf("unexpected first row: %#v", rows[0])
	}
	if rows[1].Status != StatusIntentionallyUnsupported {
		t.Fatalf("unexpected second status: %q", rows[1].Status)
	}
}

func TestParseRejectsImplementedWithoutLocalSymbol(t *testing.T) {
	input := strings.NewReader(`
| Exchange | Product | Method | Path | Status | Local Symbol |
| --- | --- | --- | --- | --- | --- |
| OKX | swap | POST | /api/v5/trade/order | implemented-sdk |  |
`)

	_, err := Parse(input)
	if err == nil {
		t.Fatal("expected validation error")
	}
}
```

- [ ] **Step 2: Run tests and confirm they fail**

Run: `go test ./sdkparity`

Expected: `package github.com/QuantProcessing/exchanges/sdkparity is not in std` or compile failure because `sdkparity` does not exist.

- [ ] **Step 3: Implement the parser**

Create `sdkparity/manifest.go`:

```go
package sdkparity

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

type Status string

const (
	StatusMissingSDK               Status = "missing-sdk"
	StatusMissingAdapter           Status = "missing-adapter"
	StatusImplementedSDK           Status = "implemented-sdk"
	StatusImplementedAdapter       Status = "implemented-adapter"
	StatusImplementedRaw           Status = "implemented-raw"
	StatusIntentionallyUnsupported Status = "intentionally-unsupported"
	StatusBlockedByOfficialAPI     Status = "blocked-by-official-api"
	StatusDeprecatedOfficial       Status = "deprecated-official"
)

type Row struct {
	Exchange    string
	Product     string
	Method      string
	Path        string
	Status      Status
	LocalSymbol string
}

func Parse(r io.Reader) ([]Row, error) {
	scanner := bufio.NewScanner(r)
	var rows []Row
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "|") || strings.Contains(line, "---") || strings.Contains(line, "Exchange | Product") {
			continue
		}
		cells := splitMarkdownRow(line)
		if len(cells) < 6 {
			continue
		}
		row := Row{
			Exchange:    cells[0],
			Product:     cells[1],
			Method:      cells[2],
			Path:        cells[3],
			Status:      Status(cells[4]),
			LocalSymbol: cells[5],
		}
		if err := row.Validate(); err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return rows, nil
}

func (r Row) Validate() error {
	switch r.Status {
	case StatusImplementedSDK, StatusImplementedAdapter, StatusImplementedRaw:
		if strings.TrimSpace(r.LocalSymbol) == "" {
			return fmt.Errorf("%s %s %s is %s but has no local symbol", r.Exchange, r.Method, r.Path, r.Status)
		}
	case StatusMissingSDK, StatusMissingAdapter, StatusIntentionallyUnsupported, StatusBlockedByOfficialAPI, StatusDeprecatedOfficial:
	default:
		return fmt.Errorf("%s %s %s has invalid status %q", r.Exchange, r.Method, r.Path, r.Status)
	}
	return nil
}

func splitMarkdownRow(line string) []string {
	line = strings.Trim(line, "|")
	parts := strings.Split(line, "|")
	cells := make([]string, 0, len(parts))
	for _, part := range parts {
		cells = append(cells, strings.TrimSpace(part))
	}
	return cells
}
```

- [ ] **Step 4: Run parser tests**

Run: `go test ./sdkparity`

Expected: `ok github.com/QuantProcessing/exchanges/sdkparity`.

- [ ] **Step 5: Add the root parity guard**

Create `official_api_parity_test.go`:

```go
package exchanges_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/QuantProcessing/exchanges/sdkparity"
)

func TestOfficialAPIParityMatricesAreClassified(t *testing.T) {
	files, err := filepath.Glob("docs/superpowers/gaps/official-api-parity-*.md")
	if err != nil {
		t.Fatalf("glob parity files: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("expected at least one exchange parity file")
	}

	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			f, err := os.Open(file)
			if err != nil {
				t.Fatalf("open parity file: %v", err)
			}
			defer f.Close()

			rows, err := sdkparity.Parse(f)
			if err != nil {
				t.Fatalf("parse parity file: %v", err)
			}
			if len(rows) == 0 {
				t.Fatal("expected at least one endpoint row")
			}
		})
	}
}
```

- [ ] **Step 6: Create the master parity rules document**

Create `docs/superpowers/gaps/official-api-parity.md`:

```markdown
# Official API Parity Rules

Every official spot/perp REST endpoint, WebSocket channel, and WebSocket API operation must be represented in an exchange-specific parity file.

Allowed statuses:

- `implemented-sdk`: a typed low-level SDK method exists.
- `implemented-adapter`: a typed SDK method exists and is exposed through an adapter or extension interface.
- `implemented-raw`: a signed/routed raw SDK method exists because the official schema is intentionally broad or unstable.
- `missing-sdk`: the official endpoint is in scope and no typed or raw SDK method exists yet.
- `missing-adapter`: the SDK method exists, but an existing adapter interface should expose it and does not yet.
- `intentionally-unsupported`: the endpoint is official but outside this repository's spot/perp trading scope.
- `blocked-by-official-api`: the official documentation is incomplete, contradictory, gated, or unavailable enough that implementation would be unsafe.
- `deprecated-official`: the official docs mark the endpoint as deprecated.

For `implemented-*` rows, `Local Symbol` must name the local Go method or interface that owns the implementation.

The alignment project is complete only when there are zero `missing-sdk` and zero `missing-adapter` rows for in-scope spot/perp APIs.

Accessed date for the initial source pass: 2026-06-10.
```

- [ ] **Step 7: Run the root guard and confirm it fails until exchange files exist**

Run: `go test ./... -run TestOfficialAPIParityMatricesAreClassified`

Expected: failure with `expected at least one exchange parity file`.

- [ ] **Step 8: Commit**

Commit message:

```text
Make official API parity auditable before expanding SDKs

Constraint: The project needs full official-doc endpoint accounting before implementation can safely scale across exchanges.
Rejected: Tracking gaps only in prose | It cannot be mechanically checked for unclassified endpoints.
Confidence: high
Scope-risk: narrow
Directive: Keep parity files updated in the same commit as any SDK endpoint addition or removal.
Tested: go test ./sdkparity
Not-tested: Full exchange matrices are added in follow-up tasks.
```

## Task 2: Build Initial Exchange Parity Matrices

**Files:**
- Create: `docs/superpowers/gaps/official-api-parity-binance.md`
- Create: `docs/superpowers/gaps/official-api-parity-okx.md`
- Create: `docs/superpowers/gaps/official-api-parity-bybit.md`
- Create: `docs/superpowers/gaps/official-api-parity-bitget.md`
- Create: `docs/superpowers/gaps/official-api-parity-hyperliquid.md`
- Create: `docs/superpowers/gaps/official-api-parity-lighter.md`

- [ ] **Step 1: Create Binance matrix**

Create `docs/superpowers/gaps/official-api-parity-binance.md` with sections for Spot REST, Spot WebSocket Streams, Spot WebSocket API, USD-M REST, USD-M WebSocket Streams, USD-M WebSocket API, and COIN-M REST/WebSocket.

Start with these rows and then continue until every official spot/USD-M/COIN-M row is classified:

```markdown
# Binance Official API Parity

Sources:

- Spot REST: https://developers.binance.com/docs/binance-spot-api-docs/rest-api
- USD-M Futures: https://developers.binance.com/docs/derivatives/usds-margined-futures/general-info

| Exchange | Product | Method | Path | Status | Local Symbol |
| --- | --- | --- | --- | --- | --- |
| BINANCE | spot | GET | /api/v3/depth | implemented-adapter | binance/sdk/spot.Client.Depth; binance.SpotAdapter.FetchOrderBook |
| BINANCE | spot | GET | /api/v3/klines | implemented-adapter | binance/sdk/spot.Client.Klines; binance.SpotAdapter.FetchKlines |
| BINANCE | spot | GET | /api/v3/ticker/24hr | implemented-adapter | binance/sdk/spot.Client.Ticker; binance.SpotAdapter.FetchTicker |
| BINANCE | spot | GET | /api/v3/ticker/bookTicker | implemented-sdk | binance/sdk/spot.Client.BookTicker |
| BINANCE | spot | GET | /api/v3/exchangeInfo | implemented-adapter | binance/sdk/spot.Client.ExchangeInfo; binance.SpotAdapter.FetchSymbolDetails |
| BINANCE | spot | POST | /api/v3/order | implemented-adapter | binance/sdk/spot.Client.PlaceOrder; binance.SpotAdapter.PlaceOrder |
| BINANCE | spot | DELETE | /api/v3/order | implemented-adapter | binance/sdk/spot.Client.CancelOrder; binance.SpotAdapter.CancelOrder |
| BINANCE | spot | POST | /api/v3/order/cancelReplace | implemented-adapter | binance/sdk/spot.Client.ModifyOrder; binance.SpotAdapter.ModifyOrder |
| BINANCE | spot | GET | /api/v3/order | implemented-adapter | binance/sdk/spot.Client.GetOrder; binance.SpotAdapter.FetchOrderByID |
| BINANCE | spot | GET | /api/v3/openOrders | implemented-adapter | binance/sdk/spot.Client.GetOpenOrders; binance.SpotAdapter.FetchOpenOrders |
| BINANCE | spot | GET | /api/v3/allOrders | missing-sdk |  |
| BINANCE | spot | POST | /api/v3/order/oco | intentionally-unsupported |  |
| BINANCE | spot | POST | /api/v3/orderList/oto | intentionally-unsupported |  |
| BINANCE | spot | POST | /api/v3/orderList/otoco | intentionally-unsupported |  |
| BINANCE | usd-m | GET | /fapi/v1/depth | implemented-adapter | binance/sdk/perp.Client.Depth; binance.Adapter.FetchOrderBook |
| BINANCE | usd-m | POST | /fapi/v1/order | implemented-adapter | binance/sdk/perp.Client.PlaceOrder; binance.Adapter.PlaceOrder |
| BINANCE | usd-m | POST | /fapi/v1/batchOrders | missing-sdk |  |
| BINANCE | usd-m | PUT | /fapi/v1/batchOrders | missing-sdk |  |
| BINANCE | usd-m | DELETE | /fapi/v1/batchOrders | missing-sdk |  |
| BINANCE | coin-m | GET | /dapi/v1/exchangeInfo | missing-sdk |  |
```

- [ ] **Step 2: Create OKX matrix**

Create `docs/superpowers/gaps/official-api-parity-okx.md` with rows for Market Data, Public Data, Trading Account, Trade, Funding, and WebSocket.

Initial rows:

```markdown
# OKX Official API Parity

Source: https://www.okx.com/docs-v5/en/

| Exchange | Product | Method | Path | Status | Local Symbol |
| --- | --- | --- | --- | --- | --- |
| OKX | spot | GET | /api/v5/market/ticker | implemented-adapter | okx/sdk.Client.GetTicker; okx.SpotAdapter.FetchTicker |
| OKX | swap | GET | /api/v5/market/ticker | implemented-adapter | okx/sdk.Client.GetTicker; okx.Adapter.FetchTicker |
| OKX | spot | GET | /api/v5/market/books | implemented-adapter | okx/sdk.Client.GetOrderBook; okx.SpotAdapter.FetchOrderBook |
| OKX | swap | GET | /api/v5/public/instruments | implemented-adapter | okx/sdk.Client.GetInstruments; okx.Adapter.FetchSymbolDetails |
| OKX | spot | POST | /api/v5/trade/order | implemented-adapter | okx/sdk.Client.PlaceOrder; okx.SpotAdapter.PlaceOrder |
| OKX | swap | POST | /api/v5/trade/batch-orders | missing-sdk |  |
| OKX | swap | POST | /api/v5/trade/order-algo | missing-sdk |  |
| OKX | swap | POST | /api/v5/account/set-leverage | implemented-adapter | okx/sdk.Client.SetLeverage; okx.Adapter.SetLeverage |
| OKX | spot | GET | /api/v5/account/bills | missing-sdk |  |
```

- [ ] **Step 3: Create Bybit matrix**

Create `docs/superpowers/gaps/official-api-parity-bybit.md`.

Initial rows:

```markdown
# Bybit Official API Parity

Source: https://bybit-exchange.github.io/docs/v5/intro

| Exchange | Product | Method | Path | Status | Local Symbol |
| --- | --- | --- | --- | --- | --- |
| BYBIT | spot | GET | /v5/market/instruments-info | implemented-adapter | bybit/sdk.Client.GetInstruments; bybit.SpotAdapter.FetchSymbolDetails |
| BYBIT | linear | GET | /v5/market/tickers | implemented-adapter | bybit/sdk.Client.GetTicker; bybit.Adapter.FetchTicker |
| BYBIT | spot | POST | /v5/order/create | implemented-adapter | bybit/sdk.Client.PlaceOrder; bybit.SpotAdapter.PlaceOrder |
| BYBIT | linear | POST | /v5/order/create-batch | missing-sdk |  |
| BYBIT | linear | POST | /v5/order/amend-batch | missing-sdk |  |
| BYBIT | linear | POST | /v5/order/cancel-batch | missing-sdk |  |
| BYBIT | linear | POST | /v5/position/trading-stop | missing-sdk |  |
| BYBIT | linear | GET | /v5/position/closed-pnl | missing-sdk |  |
```

- [ ] **Step 4: Create Bitget matrix**

Create `docs/superpowers/gaps/official-api-parity-bitget.md`.

Initial rows:

```markdown
# Bitget Official API Parity

Source: https://www.bitget.com/api-doc/common/intro

| Exchange | Product | Method | Path | Status | Local Symbol |
| --- | --- | --- | --- | --- | --- |
| BITGET | spot | GET | /api/v2/spot/public/symbols | implemented-adapter | bitget/sdk.Client.GetInstruments; bitget.SpotAdapter.FetchSymbolDetails |
| BITGET | futures | GET | /api/v2/mix/market/contracts | implemented-adapter | bitget/sdk.Client.GetInstruments; bitget.Adapter.FetchSymbolDetails |
| BITGET | spot | POST | /api/v2/spot/trade/place-order | implemented-adapter | bitget/sdk.Client.PlaceOrder; bitget.SpotAdapter.PlaceOrder |
| BITGET | spot | POST | /api/v2/spot/trade/batch-orders | missing-sdk |  |
| BITGET | futures | POST | /api/v2/mix/order/place-plan-order | missing-sdk |  |
| BITGET | futures | POST | /api/v2/mix/account/set-margin-mode | missing-sdk |  |
| BITGET | futures | GET | /api/v2/mix/position/history-position | missing-sdk |  |
```

- [ ] **Step 5: Create Hyperliquid matrix**

Create `docs/superpowers/gaps/official-api-parity-hyperliquid.md`.

Initial rows:

```markdown
# Hyperliquid Official API Parity

Sources:

- https://hyperliquid.gitbook.io/hyperliquid-docs/for-developers/api/info-endpoint
- https://hyperliquid.gitbook.io/hyperliquid-docs/for-developers/api/exchange-endpoint

| Exchange | Product | Method | Path | Status | Local Symbol |
| --- | --- | --- | --- | --- | --- |
| HYPERLIQUID | perp | POST | /info type=allMids | implemented-adapter | hyperliquid/sdk/perp.Client.AllMids; hyperliquid.Adapter.FetchTicker |
| HYPERLIQUID | spot | POST | /info type=spotMeta | implemented-adapter | hyperliquid/sdk/spot.Client.GetSpotMeta; hyperliquid.SpotAdapter.FetchSymbolDetails |
| HYPERLIQUID | perp | POST | /info type=frontendOpenOrders | missing-sdk |  |
| HYPERLIQUID | perp | POST | /info type=userFillsByTime | missing-sdk |  |
| HYPERLIQUID | perp | POST | /exchange action=scheduleCancel | missing-sdk |  |
| HYPERLIQUID | spot | POST | /exchange action=usdClassTransfer | missing-sdk |  |
| HYPERLIQUID | perp | POST | /exchange action=twapOrder | missing-sdk |  |
```

- [ ] **Step 6: Create Lighter matrix**

Create `docs/superpowers/gaps/official-api-parity-lighter.md`.

Initial rows:

```markdown
# Lighter Official API Parity

Source: https://apidocs.lighter.xyz/reference

| Exchange | Product | Method | Path | Status | Local Symbol |
| --- | --- | --- | --- | --- | --- |
| LIGHTER | perp | GET | /api/v1/orderBooks | implemented-adapter | lighter/sdk.Client.GetOrderBooks; lighter.Adapter.FetchOrderBook |
| LIGHTER | spot | GET | /api/v1/orderBooks | implemented-adapter | lighter/sdk.Client.GetOrderBooks; lighter.SpotAdapter.FetchOrderBook |
| LIGHTER | perp | POST | /api/v1/createOrder | implemented-adapter | lighter/sdk.Client.PlaceOrder; lighter.Adapter.PlaceOrder |
| LIGHTER | perp | POST | /api/v1/sendTxBatch | implemented-sdk | lighter/sdk.Client.SendTxBatch |
| LIGHTER | perp | GET | /api/v1/accountTxs | implemented-sdk | lighter/sdk.Client.GetAccountTxs |
| LIGHTER | perp | GET | /api/v1/positionFunding | implemented-sdk | lighter/sdk.Client.GetPositionFunding |
| LIGHTER | spot | GET | /api/v1/spotAvgEntryPrices | implemented-sdk | lighter/sdk.WebsocketClient.SubscribeAccountSpotAvgEntryPrices |
```

- [ ] **Step 7: Run matrix validation**

Run: `go test ./... -run TestOfficialAPIParityMatricesAreClassified`

Expected: pass after every `implemented-*` row names a local symbol and every row uses an allowed status.

- [ ] **Step 8: Commit**

Commit message:

```text
Classify official spot and perp API surfaces before SDK expansion

Constraint: Official exchange APIs are wider than the shared Exchange interface and need endpoint-level accounting.
Rejected: Implementing exchange gaps from memory | It would miss endpoints and make completion unverifiable.
Confidence: medium
Scope-risk: moderate
Directive: Update the relevant parity row in the same commit as every SDK endpoint implementation.
Tested: go test ./... -run TestOfficialAPIParityMatricesAreClassified
Not-tested: Live exchange behavior; this commit is documentation and classification only.
```

## Task 3: Add Shared SDK Request Options Without Changing Adapters

**Files:**
- Modify: `models.go`
- Modify: `binance/sdk/spot/client.go`
- Modify: `binance/sdk/perp/client.go`
- Modify: `okx/sdk/client.go`
- Modify: `bybit/sdk/client.go`
- Modify: `bitget/sdk/client.go`
- Modify: `hyperliquid/sdk/client.go`
- Modify: `lighter/sdk/client.go`
- Add tests beside each modified client.

- [ ] **Step 1: Add SDK request option types**

Add to `models.go` near optional parameter structs:

```go
type SDKRequestOpts struct {
	RecvWindowMillis int64
	ClientRequestID  string
}
```

Keep this type generic and optional. Do not thread it through adapter methods in this task.

- [ ] **Step 2: Add per-client support for exchange request options**

For each SDK client, add a private helper that merges default options into request params:

```go
func applyRecvWindow(params map[string]interface{}, recvWindow int64) {
	if recvWindow > 0 {
		params["recvWindow"] = recvWindow
	}
}
```

For string-map clients, use:

```go
func applyRecvWindowString(params map[string]string, recvWindow int64) {
	if recvWindow > 0 {
		params["recvWindow"] = strconv.FormatInt(recvWindow, 10)
	}
}
```

- [ ] **Step 3: Test option preservation**

Add a unit test per client package that sends a signed request through a stub `RoundTripper` and verifies the request contains the expected recv-window or request-id field without changing existing default behavior.

Example for Binance spot:

```go
func TestClientIncludesRecvWindowWhenProvided(t *testing.T) {
	var seenQuery string
	client := NewClient().WithCredentials("key", "secret")
	client.HTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		seenQuery = req.URL.RawQuery
		return jsonResponse(200, `{"symbol":"BTCUSDT","orderId":1}`)
	})}

	_, err := client.GetOrderWithOpts(context.Background(), "BTCUSDT", 1, "", exchanges.SDKRequestOpts{RecvWindowMillis: 2500})
	if err != nil {
		t.Fatalf("GetOrderWithOpts returned error: %v", err)
	}
	if !strings.Contains(seenQuery, "recvWindow=2500") {
		t.Fatalf("expected recvWindow in query, got %q", seenQuery)
	}
}
```

- [ ] **Step 4: Run focused client tests**

Run:

```bash
go test ./binance/sdk/spot ./binance/sdk/perp ./okx/sdk ./bybit/sdk ./bitget/sdk ./hyperliquid/sdk/... ./lighter/sdk
```

Expected: all packages pass.

- [ ] **Step 5: Commit**

Commit message:

```text
Prepare SDK clients for official request option parity

Constraint: Official APIs expose transport-level options that do not belong in adapter contracts.
Rejected: Adding recvWindow and request IDs to Exchange methods | It would pollute the unified interface with venue-specific transport concerns.
Confidence: medium
Scope-risk: moderate
Directive: Keep SDK request options explicit and local to SDK calls; do not leak venue-specific transport knobs into adapter contracts.
Tested: go test ./binance/sdk/spot ./binance/sdk/perp ./okx/sdk ./bybit/sdk ./bitget/sdk ./hyperliquid/sdk/... ./lighter/sdk
Not-tested: Live signed requests.
```

## Task 4: Binance Spot And USD-M Typed Endpoint Parity

**Files:**
- Modify: `binance/sdk/spot/order.go`
- Modify: `binance/sdk/spot/market.go`
- Modify: `binance/sdk/spot/account.go`
- Modify: `binance/sdk/spot/ws_order.go`
- Modify: `binance/sdk/perp/order.go`
- Modify: `binance/sdk/perp/market.go`
- Modify: `binance/sdk/perp/account.go`
- Add: `binance/sdk/coinm/client.go`
- Add: `binance/sdk/coinm/market.go`
- Add: `binance/sdk/coinm/order.go`
- Add: `binance/sdk/coinm/account.go`
- Add or modify package tests.
- Modify: `docs/superpowers/gaps/official-api-parity-binance.md`

- [ ] **Step 1: Add failing tests for Binance spot missing endpoints**

Add tests for:

- `Client.GetAllOrders`
- `Client.PlaceOCOOrder`
- `Client.GetOrderList`
- `Client.CancelOrderList`
- `Client.GetAccountCommission`
- `WsAPIClient.GetOrderWS`
- `WsAPIClient.GetOpenOrdersWS`

Each test must assert method, path, query/body fields, signing, and response decoding.

- [ ] **Step 2: Implement Binance spot methods**

Implement methods with signatures:

```go
func (c *Client) GetAllOrders(ctx context.Context, symbol string, orderID int64, startTime, endTime int64, limit int) ([]OrderResponse, error)
func (c *Client) PlaceOCOOrder(ctx context.Context, p OCOOrderParams) (*OrderListResponse, error)
func (c *Client) GetOrderList(ctx context.Context, orderListID int64, origClientOrderID string) (*OrderListResponse, error)
func (c *Client) CancelOrderList(ctx context.Context, symbol string, orderListID int64, listClientOrderID, newClientOrderID string) (*OrderListResponse, error)
func (c *Client) GetAccountCommission(ctx context.Context, symbol string) (*CommissionResponse, error)
```

Implement only typed SDK methods in this task. Do not expose OCO through `SpotAdapter`.

- [ ] **Step 3: Add failing tests for Binance USD-M missing endpoints**

Add tests for:

- `Client.PlaceBatchOrders`
- `Client.ModifyBatchOrders`
- `Client.CancelBatchOrders`
- `Client.CountdownCancelAll`
- `Client.GetAllOrders`
- `Client.GetForceOrders`
- `Client.GetIncome`
- `Client.GetLeverageBrackets`
- `Client.GetOrderRateLimit`

- [ ] **Step 4: Implement Binance USD-M methods**

Implement methods with signatures:

```go
func (c *Client) PlaceBatchOrders(ctx context.Context, orders []PlaceOrderParams) ([]OrderResponse, error)
func (c *Client) ModifyBatchOrders(ctx context.Context, orders []ModifyOrderParams) ([]OrderResponse, error)
func (c *Client) CancelBatchOrders(ctx context.Context, symbol string, orderIDs []int64, origClientOrderIDs []string) ([]OrderResponse, error)
func (c *Client) CountdownCancelAll(ctx context.Context, symbol string, countdownTime int64) error
func (c *Client) GetAllOrders(ctx context.Context, symbol string, orderID int64, startTime, endTime int64, limit int) ([]OrderResponse, error)
func (c *Client) GetForceOrders(ctx context.Context, symbol string, autoCloseType string, startTime, endTime int64, limit int) ([]ForceOrderResponse, error)
func (c *Client) GetIncome(ctx context.Context, symbol, incomeType string, startTime, endTime int64, limit int) ([]IncomeResponse, error)
func (c *Client) GetLeverageBrackets(ctx context.Context, symbol string) ([]LeverageBracket, error)
func (c *Client) GetOrderRateLimit(ctx context.Context) ([]OrderRateLimit, error)
```

- [ ] **Step 5: Create COIN-M package skeleton**

Add `binance/sdk/coinm` using the existing USD-M client shape with `/dapi` base paths. Implement:

```go
func NewClient() *Client
func (c *Client) ExchangeInfo(ctx context.Context) (*ExchangeInfoResponse, error)
func (c *Client) Depth(ctx context.Context, symbol string, limit int) (*DepthResponse, error)
func (c *Client) PlaceOrder(ctx context.Context, p PlaceOrderParams) (*OrderResponse, error)
func (c *Client) CancelOrder(ctx context.Context, p CancelOrderParams) (*OrderResponse, error)
func (c *Client) GetOrder(ctx context.Context, symbol string, orderID int64, origClientOrderID string) (*OrderResponse, error)
```

Do not register a COIN-M adapter yet.

- [ ] **Step 6: Update Binance parity matrix**

For each implemented endpoint, change the row status to `implemented-sdk` and set `Local Symbol` to the exact method.

- [ ] **Step 7: Run Binance SDK tests**

Run:

```bash
go test ./binance/sdk/spot ./binance/sdk/perp ./binance/sdk/coinm
go test ./binance -run 'Test.*Binance|Test.*Spot|Test.*Perp'
```

Expected: all focused tests pass.

- [ ] **Step 8: Commit**

Commit message:

```text
Close Binance spot and USD-M SDK endpoint gaps

Constraint: Binance official docs include order-list, batch, account, and COIN-M surfaces beyond the existing adapter contract.
Rejected: Exposing every new endpoint through SpotAdapter and Adapter immediately | Most endpoints are venue-specific and should mature in sdk first.
Confidence: medium
Scope-risk: broad
Directive: Do not register a COIN-M adapter until shared tests and capability claims are designed.
Tested: go test ./binance/sdk/spot ./binance/sdk/perp ./binance/sdk/coinm; go test ./binance -run 'Test.*Binance|Test.*Spot|Test.*Perp'
Not-tested: Live Binance order-list, batch, and COIN-M calls.
```

## Task 5: OKX Spot/SWAP Typed Endpoint Parity

**Files:**
- Modify: `okx/sdk/order.go`
- Modify: `okx/sdk/account.go`
- Modify: `okx/sdk/market.go`
- Modify: `okx/sdk/ws_order.go`
- Modify: `okx/sdk/ws_account.go`
- Modify: `docs/superpowers/gaps/official-api-parity-okx.md`

- [ ] **Step 1: Add failing tests for OKX trade endpoints**

Add tests for:

- `Client.PlaceBatchOrders`
- `Client.CancelBatchOrders`
- `Client.ModifyBatchOrders`
- `Client.PlaceAlgoOrder`
- `Client.CancelAlgoOrders`
- `Client.GetAlgoOrders`
- `Client.GetOrderHistory`
- `Client.GetFills`

- [ ] **Step 2: Implement OKX trade methods**

Implement:

```go
func (c *Client) PlaceBatchOrders(ctx context.Context, reqs []OrderRequest) ([]OrderId, error)
func (c *Client) CancelBatchOrders(ctx context.Context, reqs []CancelOrderRequest) ([]OrderId, error)
func (c *Client) ModifyBatchOrders(ctx context.Context, reqs []ModifyOrderRequest) ([]OrderId, error)
func (c *Client) PlaceAlgoOrder(ctx context.Context, req *AlgoOrderRequest) ([]OrderId, error)
func (c *Client) CancelAlgoOrders(ctx context.Context, reqs []CancelAlgoOrderRequest) ([]OrderId, error)
func (c *Client) GetAlgoOrders(ctx context.Context, ordType string, instType, instId *string) ([]AlgoOrder, error)
func (c *Client) GetOrderHistory(ctx context.Context, instType string, instId *string, state *string, limit *int) ([]Order, error)
func (c *Client) GetFills(ctx context.Context, instType *string, instId *string, ordId *string, limit *int) ([]Fill, error)
```

- [ ] **Step 3: Add failing tests for OKX account/public gaps**

Add tests for:

- `Client.GetAccountBills`
- `Client.GetMaxOrderSize`
- `Client.GetMaxAvailableTradeAmount`
- `Client.SetMarginMode`
- `Client.GetPositionHistory`
- `Client.GetMarkPrice`
- `Client.GetIndexTicker`

- [ ] **Step 4: Implement OKX account/public methods**

Implement:

```go
func (c *Client) GetAccountBills(ctx context.Context, ccy *string, instType *string, limit *int) ([]AccountBill, error)
func (c *Client) GetMaxOrderSize(ctx context.Context, instId, tdMode string, ccy *string, px *string) ([]MaxOrderSize, error)
func (c *Client) GetMaxAvailableTradeAmount(ctx context.Context, instId, tdMode string, ccy *string, reduceOnly *bool) ([]MaxAvailableTradeAmount, error)
func (c *Client) SetMarginMode(ctx context.Context, instId, mgnMode string) error
func (c *Client) GetPositionHistory(ctx context.Context, instType, instId *string, limit *int) ([]PositionHistory, error)
func (c *Client) GetMarkPrice(ctx context.Context, instType string, instId *string) ([]MarkPrice, error)
func (c *Client) GetIndexTicker(ctx context.Context, instId string) ([]IndexTicker, error)
```

- [ ] **Step 5: Add OKX WebSocket typed channels**

Add typed subscription helpers for trades and candles, since adapter currently has unsupported tests for `WatchTrades` and `WatchKlines`:

```go
func (c *WSClient) SubscribeTrades(instId string, handler func([]PublicTrade)) error
func (c *WSClient) SubscribeCandles(instId, bar string, handler func([]Candle)) error
func (c *WSClient) UnsubscribeTrades(instId string) error
func (c *WSClient) UnsubscribeCandles(instId, bar string) error
```

After SDK tests pass, add adapter support in `okx/perp_adapter.go` and `okx/spot_adapter.go` for existing `WatchTrades` and `WatchKlines`.

- [ ] **Step 6: Update OKX parity matrix**

Set every newly implemented row to `implemented-sdk` or `implemented-adapter`.

- [ ] **Step 7: Run OKX tests**

Run:

```bash
go test ./okx/sdk ./okx
```

Expected: all OKX tests pass, including newly enabled `WatchTrades` and `WatchKlines` unit tests.

- [ ] **Step 8: Commit**

Commit message:

```text
Expand OKX spot and swap SDK parity for trade and account APIs

Constraint: OKX v5 groups spot and swap behind shared trade/account APIs that are wider than the current adapter.
Rejected: Adding OKX algo orders to the unified Exchange interface | Algo orders are venue-specific and need a mature cross-exchange design first.
Confidence: medium
Scope-risk: broad
Directive: Keep algo and batch APIs in okx/sdk until at least two other exchanges expose compatible abstractions.
Tested: go test ./okx/sdk ./okx
Not-tested: Live OKX algo and batch order submission.
```

## Task 6: Bybit V5 Spot/Linear/Inverse Typed Endpoint Parity

**Files:**
- Modify: `bybit/sdk/public_rest.go`
- Modify: `bybit/sdk/order_rest.go`
- Modify: `bybit/sdk/private_rest.go`
- Modify: `bybit/sdk/trade_ws.go`
- Modify: `docs/superpowers/gaps/official-api-parity-bybit.md`

- [ ] **Step 1: Add failing tests for Bybit Market endpoints**

Add tests for:

- `Client.GetMarkPriceKlines`
- `Client.GetIndexPriceKlines`
- `Client.GetPremiumIndexPriceKlines`
- `Client.GetRiskLimit`
- `Client.GetDeliveryPrice`
- `Client.GetLongShortRatio`
- `Client.GetOrderPriceLimit`
- `Client.GetInsurance`

- [ ] **Step 2: Implement Bybit Market methods**

Implement:

```go
func (c *Client) GetMarkPriceKlines(ctx context.Context, category, symbol, interval string, start, end int64, limit int) ([]Candle, error)
func (c *Client) GetIndexPriceKlines(ctx context.Context, category, symbol, interval string, start, end int64, limit int) ([]Candle, error)
func (c *Client) GetPremiumIndexPriceKlines(ctx context.Context, category, symbol, interval string, start, end int64, limit int) ([]Candle, error)
func (c *Client) GetRiskLimit(ctx context.Context, category, symbol string) ([]RiskLimitRecord, error)
func (c *Client) GetDeliveryPrice(ctx context.Context, category, symbol string, limit int, cursor string) (*DeliveryPriceResult, error)
func (c *Client) GetLongShortRatio(ctx context.Context, category, symbol, period string, limit int) (*LongShortRatioResult, error)
func (c *Client) GetOrderPriceLimit(ctx context.Context, category, symbol string) (*OrderPriceLimit, error)
func (c *Client) GetInsurance(ctx context.Context, coin string) (*InsuranceResult, error)
```

- [ ] **Step 3: Add failing tests for Bybit Trade endpoints**

Add tests for:

- `Client.PlaceBatchOrders`
- `Client.AmendBatchOrders`
- `Client.CancelBatchOrders`
- `Client.PreCheckOrder`
- `Client.SetDisconnectCancelAll`

- [ ] **Step 4: Implement Bybit Trade methods**

Implement:

```go
func (c *Client) PlaceBatchOrders(ctx context.Context, category string, reqs []PlaceOrderRequest) ([]OrderActionResponse, error)
func (c *Client) AmendBatchOrders(ctx context.Context, category string, reqs []AmendOrderRequest) ([]OrderActionResponse, error)
func (c *Client) CancelBatchOrders(ctx context.Context, category string, reqs []CancelOrderRequest) ([]OrderActionResponse, error)
func (c *Client) PreCheckOrder(ctx context.Context, req PlaceOrderRequest) (*OrderActionResponse, error)
func (c *Client) SetDisconnectCancelAll(ctx context.Context, product string, timeoutSeconds int) error
```

- [ ] **Step 5: Add failing tests for Bybit Position/Account endpoints**

Add tests for:

- `Client.SwitchPositionMode`
- `Client.SetTradingStop`
- `Client.SetAutoAddMargin`
- `Client.AddOrReduceMargin`
- `Client.GetClosedPnL`
- `Client.GetTransactionLog`
- `Client.GetBorrowHistory`

- [ ] **Step 6: Implement Bybit Position/Account methods**

Implement:

```go
func (c *Client) SwitchPositionMode(ctx context.Context, category, symbol, coin string, mode int) error
func (c *Client) SetTradingStop(ctx context.Context, req TradingStopRequest) error
func (c *Client) SetAutoAddMargin(ctx context.Context, category, symbol string, autoAddMargin int, positionIdx int) error
func (c *Client) AddOrReduceMargin(ctx context.Context, category, symbol, margin string, positionIdx int) error
func (c *Client) GetClosedPnL(ctx context.Context, category, symbol string, startTime, endTime int64, limit int, cursor string) (*ClosedPnLResult, error)
func (c *Client) GetTransactionLog(ctx context.Context, accountType, category, currency string, startTime, endTime int64, limit int, cursor string) (*TransactionLogResult, error)
func (c *Client) GetBorrowHistory(ctx context.Context, currency string, startTime, endTime int64, limit int, cursor string) (*BorrowHistoryResult, error)
```

- [ ] **Step 7: Add Bybit trade WebSocket batch operations**

Implement:

```go
func (c *TradeWSClient) PlaceBatchOrders(ctx context.Context, category string, reqs []PlaceOrderRequest) error
func (c *TradeWSClient) AmendBatchOrders(ctx context.Context, category string, reqs []AmendOrderRequest) error
func (c *TradeWSClient) CancelBatchOrders(ctx context.Context, category string, reqs []CancelOrderRequest) error
```

- [ ] **Step 8: Update matrix and run tests**

Run:

```bash
go test ./bybit/sdk ./bybit
```

Expected: all Bybit tests pass.

- [ ] **Step 9: Commit**

Commit message:

```text
Expand Bybit V5 SDK coverage beyond core order lifecycle

Constraint: Bybit V5 official APIs include batch trade, position risk, and market-derived endpoints missing from the local sdk.
Rejected: Treating linear-only support as full V5 parity | Official V5 includes spot, linear, and inverse categories through shared request shapes.
Confidence: medium
Scope-risk: broad
Directive: Keep category handling explicit in every Bybit request and test spot, linear, and inverse routing separately.
Tested: go test ./bybit/sdk ./bybit
Not-tested: Live Bybit batch and position-risk requests.
```

## Task 7: Bitget V2 Spot/Futures Typed Endpoint Parity

**Files:**
- Modify: `bitget/sdk/public_rest.go`
- Modify: `bitget/sdk/private_rest.go`
- Modify: `bitget/sdk/private_ws_trade_classic.go`
- Modify: `docs/superpowers/gaps/official-api-parity-bitget.md`

- [ ] **Step 1: Add failing tests for Bitget spot gaps**

Add tests for:

- `Client.GetSpotCoinInfo`
- `Client.GetSpotVIPFeeRate`
- `Client.GetSpotMergeDepth`
- `Client.GetSpotHistoryCandles`
- `Client.PlaceSpotBatchOrders`
- `Client.CancelReplaceSpotOrder`
- `Client.GetSpotFills`
- `Client.PlaceSpotPlanOrder`
- `Client.CancelSpotPlanOrder`

- [ ] **Step 2: Implement Bitget spot methods**

Implement:

```go
func (c *Client) GetSpotCoinInfo(ctx context.Context, coin string) ([]SpotCoinInfo, error)
func (c *Client) GetSpotVIPFeeRate(ctx context.Context) ([]SpotVIPFeeRate, error)
func (c *Client) GetSpotMergeDepth(ctx context.Context, symbol string, precision string, limit int) (*OrderBook, error)
func (c *Client) GetSpotHistoryCandles(ctx context.Context, symbol, granularity string, startTime, endTime int64, limit int) ([]Candle, error)
func (c *Client) PlaceSpotBatchOrders(ctx context.Context, reqs []PlaceOrderRequest) ([]PlaceOrderResponse, error)
func (c *Client) CancelReplaceSpotOrder(ctx context.Context, req CancelReplaceOrderRequest) (*PlaceOrderResponse, error)
func (c *Client) GetSpotFills(ctx context.Context, symbol, orderID string, startTime, endTime int64, limit int) ([]FillRecord, error)
func (c *Client) PlaceSpotPlanOrder(ctx context.Context, req PlanOrderRequest) (*PlaceOrderResponse, error)
func (c *Client) CancelSpotPlanOrder(ctx context.Context, symbol, orderID, clientOID string) error
```

- [ ] **Step 3: Add failing tests for Bitget futures gaps**

Add tests for:

- `Client.GetFuturesAllTickers`
- `Client.GetFuturesHistoryTrades`
- `Client.GetFuturesMarkPrice`
- `Client.GetFuturesIndexPrice`
- `Client.GetFuturesHistoryCandles`
- `Client.SetMarginMode`
- `Client.SetPositionMode`
- `Client.CloseAllPositions`
- `Client.GetHistoryPositions`
- `Client.PlaceFuturesBatchOrders`
- `Client.PlacePlanOrder`
- `Client.GetFuturesFills`

- [ ] **Step 4: Implement Bitget futures methods**

Implement:

```go
func (c *Client) GetFuturesAllTickers(ctx context.Context, productType string) ([]Ticker, error)
func (c *Client) GetFuturesHistoryTrades(ctx context.Context, symbol, productType string, limit int) ([]PublicFill, error)
func (c *Client) GetFuturesMarkPrice(ctx context.Context, symbol, productType string) (*MarkPrice, error)
func (c *Client) GetFuturesIndexPrice(ctx context.Context, symbol, productType string) (*IndexPrice, error)
func (c *Client) GetFuturesHistoryCandles(ctx context.Context, symbol, productType, granularity string, startTime, endTime int64, limit int) ([]Candle, error)
func (c *Client) SetMarginMode(ctx context.Context, req SetMarginModeRequest) error
func (c *Client) SetPositionMode(ctx context.Context, productType, posMode string) error
func (c *Client) CloseAllPositions(ctx context.Context, productType, symbol string) error
func (c *Client) GetHistoryPositions(ctx context.Context, productType, symbol string, startTime, endTime int64, limit int) ([]PositionRecord, error)
func (c *Client) PlaceFuturesBatchOrders(ctx context.Context, reqs []PlaceOrderRequest) ([]PlaceOrderResponse, error)
func (c *Client) PlacePlanOrder(ctx context.Context, req PlanOrderRequest) (*PlaceOrderResponse, error)
func (c *Client) GetFuturesFills(ctx context.Context, productType, symbol, orderID string, startTime, endTime int64, limit int) ([]FillRecord, error)
```

- [ ] **Step 5: Add WebSocket trade operation coverage**

Implement typed private WS requests for Bitget batch place/cancel and futures plan order operations only if official WebSocket docs list them as supported. If the official docs list REST-only support, mark WebSocket rows `intentionally-unsupported`.

- [ ] **Step 6: Update matrix and run tests**

Run:

```bash
go test ./bitget/sdk ./bitget
```

Expected: all Bitget tests pass.

- [ ] **Step 7: Commit**

Commit message:

```text
Fill Bitget V2 spot and futures SDK gaps

Constraint: Bitget V2 splits spot and futures APIs into separate business lines with different path and request shapes.
Rejected: Reusing classic v1 methods for V2-only coverage | It hides official V2 behavior and makes parity claims inaccurate.
Confidence: medium
Scope-risk: broad
Directive: Keep classic compatibility methods separate from V2 parity methods.
Tested: go test ./bitget/sdk ./bitget
Not-tested: Live Bitget plan-order and batch-order requests.
```

## Task 8: Hyperliquid Info/Exchange Endpoint Parity

**Files:**
- Modify: `hyperliquid/sdk/client.go`
- Modify: `hyperliquid/sdk/perp/account.go`
- Modify: `hyperliquid/sdk/perp/order.go`
- Modify: `hyperliquid/sdk/perp/market.go`
- Modify: `hyperliquid/sdk/spot/account.go`
- Modify: `hyperliquid/sdk/spot/order.go`
- Modify: `hyperliquid/sdk/action_types.go`
- Modify: `docs/superpowers/gaps/official-api-parity-hyperliquid.md`

- [ ] **Step 1: Add failing tests for Info endpoint gaps**

Add tests for:

- `FrontendOpenOrders`
- `UserFillsByTime`
- `UserRateLimit`
- `HistoricalOrders`
- `Portfolio`
- `ClearinghouseState`
- `SpotClearinghouseState`
- `UserFunding`
- `Delegations`

- [ ] **Step 2: Implement Info endpoint methods**

Implement shared info helpers on `hyperliquid/sdk.Client`, then expose product-specific wrappers:

```go
func (c *Client) Info(ctx context.Context, req any, out any) error
func (c *Client) UserRateLimit(ctx context.Context, user string) (*UserRateLimit, error)
func (c *Client) Portfolio(ctx context.Context, user string) (*Portfolio, error)
func (c *Client) HistoricalOrders(ctx context.Context, user string) ([]HistoricalOrder, error)
```

Product wrappers:

```go
func (c *perp.Client) FrontendOpenOrders(ctx context.Context, user string) ([]Order, error)
func (c *perp.Client) UserFillsByTime(ctx context.Context, user string, startTime, endTime int64, aggregateByTime bool) ([]UserFill, error)
func (c *spot.Client) FrontendOpenOrders(ctx context.Context, user string) ([]Order, error)
func (c *spot.Client) UserFillsByTime(ctx context.Context, user string, startTime, endTime int64, aggregateByTime bool) ([]UserFill, error)
```

- [ ] **Step 3: Add failing tests for Exchange endpoint actions**

Add tests for:

- `CancelByCloid`
- `ScheduleCancel`
- `BatchCancel`
- `BatchModify`
- `PlaceTWAPOrder`
- `CancelTWAPOrder`
- `ApproveBuilderFee`
- `USDClassTransfer`
- `SpotTransfer`
- `VaultTransfer`

- [ ] **Step 4: Implement Exchange endpoint actions**

Implement:

```go
func (c *perp.Client) CancelByCloid(ctx context.Context, coin string, cloid string) (*string, error)
func (c *perp.Client) ScheduleCancel(ctx context.Context, deadlineMillis *int64) error
func (c *perp.Client) BatchCancel(ctx context.Context, reqs []CancelOrderRequest) ([]string, error)
func (c *perp.Client) BatchModify(ctx context.Context, reqs []ModifyOrderRequest) ([]OrderStatus, error)
func (c *perp.Client) PlaceTWAPOrder(ctx context.Context, req TWAPOrderRequest) (*OrderStatus, error)
func (c *perp.Client) CancelTWAPOrder(ctx context.Context, coin string, twapID int64) error
func (c *perp.Client) ApproveBuilderFee(ctx context.Context, builder string, maxFeeRate string) error
func (c *spot.Client) USDClassTransfer(ctx context.Context, amount string, toPerp bool) error
func (c *spot.Client) SpotTransfer(ctx context.Context, destination string, token string, amount string) error
func (c *Client) VaultTransfer(ctx context.Context, vaultAddress string, isDeposit bool, amount string) error
```

- [ ] **Step 5: Update status mapping**

Extend Hyperliquid order status mapping to include documented statuses such as `scheduledCancel`, `selfTradeCanceled`, `openInterestCapCanceled`, `reduceOnlyRejected`, and `insufficientSpotBalanceRejected`. Map unknown statuses to a raw status field if the model has one; otherwise return `OrderStatusUnknown` and preserve the native status in a metadata field added to the SDK type.

- [ ] **Step 6: Update matrix and run tests**

Run:

```bash
go test ./hyperliquid/sdk/... ./hyperliquid
```

Expected: all Hyperliquid tests pass.

- [ ] **Step 7: Commit**

Commit message:

```text
Align Hyperliquid sdk with documented info and exchange actions

Constraint: Hyperliquid exposes broad typed actions through info and exchange endpoints rather than many REST paths.
Rejected: Encoding every action as ad hoc map payloads in adapters | It would leak protocol details above the sdk boundary.
Confidence: medium
Scope-risk: broad
Directive: Add new Hyperliquid exchange actions as typed sdk methods before adapter exposure.
Tested: go test ./hyperliquid/sdk/... ./hyperliquid
Not-tested: Live TWAP, vault, and transfer actions.
```

## Task 9: Lighter API Reference Parity

**Files:**
- Modify: `lighter/sdk/market.go`
- Modify: `lighter/sdk/account.go`
- Modify: `lighter/sdk/order.go`
- Modify: `lighter/sdk/token.go`
- Modify: `lighter/sdk/ws_account.go`
- Modify: `docs/superpowers/gaps/official-api-parity-lighter.md`

- [ ] **Step 1: Reconcile current Lighter SDK against official reference**

Update the Lighter matrix first because the current local SDK is already broad. Classify every Lighter reference endpoint and WebSocket channel before adding code.

- [ ] **Step 2: Add failing tests for remaining REST gaps**

Add tests only for rows classified as not implemented. Expected candidates:

- deposit/withdrawal status endpoints
- transfer endpoints
- pool and liquidity endpoints
- referral endpoints
- account metadata and limits variants
- token/API key lifecycle endpoints not already covered by `token.go`

- [ ] **Step 3: Implement remaining REST methods**

Use existing `lighter/sdk.Client.Get`, `Post`, and `PostForm` patterns. Do not change signing logic unless an official endpoint requires a different auth envelope.

- [ ] **Step 4: Add failing tests for remaining WS gaps**

Add typed WS subscription tests for any official channels missing from `lighter/sdk/ws_account.go` or `lighter/sdk/ws_market.go`.

- [ ] **Step 5: Implement remaining WS methods**

Follow the existing channel naming pattern:

```go
func (c *WebsocketClient) SubscribeChannelName(args..., cb func([]byte)) error
func (c *WebsocketClient) UnsubscribeChannelName(args...) error
```

- [ ] **Step 6: Update matrix and run tests**

Run:

```bash
go test ./lighter/sdk ./lighter
```

Expected: all Lighter tests pass.

- [ ] **Step 7: Commit**

Commit message:

```text
Finish Lighter sdk parity against the official reference

Constraint: Lighter's local sdk is already broad, so the work is reconciliation and precise gap closure.
Rejected: Rewriting the Lighter client shape | Existing request, signing, and websocket abstractions already match the API style.
Confidence: medium
Scope-risk: moderate
Directive: Keep new Lighter endpoints in existing market/account/order/token groupings unless the official reference introduces a new business domain.
Tested: go test ./lighter/sdk ./lighter
Not-tested: Live Lighter transfer and pool operations.
```

## Task 10: Adapter Exposure For Existing Unified Interfaces

**Files:**
- Modify: `okx/perp_adapter.go`
- Modify: `okx/spot_adapter.go`
- Modify: `bybit/perp_adapter.go`
- Modify: `bybit/spot_adapter.go`
- Modify: `bitget/perp_adapter.go`
- Modify: `bitget/spot_adapter.go`
- Modify: `hyperliquid/perp_adapter.go`
- Modify: `hyperliquid/spot_adapter.go`
- Modify: `capabilities.go`
- Modify: exchange-specific `register.go` files only if a capability changes.

- [ ] **Step 1: Identify SDK endpoints that satisfy existing interfaces**

For each exchange, list only endpoints that map to existing methods in `Exchange`, `PerpExchange`, or `SpotExchange`:

- `FetchHistoricalTrades`
- `WatchTrades`
- `WatchKlines`
- `FetchOrders`
- `FetchFundingRateHistory`
- `FetchOpenInterest`
- `TransferAsset`
- `SetLeverage`
- `ModifyOrder`

- [ ] **Step 2: Add failing adapter tests**

For each new adapter exposure, write one unit test that stubs the SDK client and verifies:

- symbol conversion
- option propagation
- native status mapping
- `ErrNotSupported` remains for unsupported official endpoints
- capability claims match implementation

- [ ] **Step 3: Implement adapter exposure**

Use existing mapper helpers. Do not add new cross-exchange interfaces for batch orders, algo orders, TP/SL, or transfers in this task.

- [ ] **Step 4: Update capabilities**

Only set a capability to true after the method is implemented and covered by tests. Follow `docs/contributing/adding-exchange-adapters.md`: lifecycle and trading account readiness require real private order streams.

- [ ] **Step 5: Run shared suites**

Run:

```bash
go test ./...
```

Expected: full test suite passes.

- [ ] **Step 6: Commit**

Commit message:

```text
Expose newly aligned SDK behavior through existing adapter contracts

Constraint: Adapter capability claims must match shared tests and real stream behavior.
Rejected: Adding broad venue-specific interfaces during parity work | It would expand public API design before the sdk surface is stable.
Confidence: medium
Scope-risk: broad
Directive: Keep venue-specific advanced order APIs in sdk until a separate abstraction design is approved.
Tested: go test ./...
Not-tested: Live adapter behavior without exchange credentials.
```

## Task 11: Live Test Gates And Documentation

**Files:**
- Modify: `.env.example`
- Modify: `testsuite/livetest/*`
- Modify: each exchange `adapter_test.go` where live suite coverage changes.
- Modify: `docs/contributing/adding-exchange-adapters.md`
- Modify: `README.md`
- Modify: `README_CN.md`

- [ ] **Step 1: Add live test environment variables**

For every newly live-testable product line, add exchange-prefixed env vars:

```text
BINANCE_COINM_API_KEY=
BINANCE_COINM_SECRET_KEY=
BINANCE_COINM_PERP_TEST_SYMBOL=BTCUSD
OKX_ENABLE_ALGO_ORDER_TESTS=0
BYBIT_ENABLE_BATCH_ORDER_TESTS=0
BITGET_ENABLE_PLAN_ORDER_TESTS=0
HYPERLIQUID_ENABLE_TRANSFER_TESTS=0
LIGHTER_ENABLE_TRANSFER_TESTS=0
```

Use opt-in flags for advanced write operations.

- [ ] **Step 2: Add live read tests**

Add read-only live tests for:

- Binance COIN-M exchange info and depth.
- OKX mark/index price and account config when credentials exist.
- Bybit risk limit and order price limit.
- Bitget mark/index price and history candles.
- Hyperliquid frontend open orders and user rate limit.
- Lighter account metadata and market stats.

- [ ] **Step 3: Add guarded live write tests**

Only add guarded tests for write APIs that can safely use testnet/demo mode or tiny orders. Every live write test must require both `RUN_FULL=1` and an exchange-specific `*_ENABLE_*_TESTS=1`.

- [ ] **Step 4: Update contributor docs**

Add a section to `docs/contributing/adding-exchange-adapters.md`:

```markdown
## Official API Parity Matrix

When adding or expanding an SDK, update `docs/superpowers/gaps/official-api-parity-<exchange>.md` in the same commit. Every official spot/perp endpoint must have an explicit status. Do not claim official API parity while any row is unclassified.
```

- [ ] **Step 5: Update READMEs**

Document that:

- The shared adapter interface remains intentionally smaller than each official API.
- Full venue-specific coverage lives under `sdk/`.
- The parity matrix is the source of truth for implemented, intentionally unsupported, and blocked official endpoints.

- [ ] **Step 6: Run final verification**

Run:

```bash
go test ./...
```

If credentials are available, also run:

```bash
RUN_FULL=1 go test ./testsuite/livetest -run 'Test.*Official.*Read' -count=1 -v
```

Expected: local full suite passes; live read suite passes when credentials and network are configured.

- [ ] **Step 7: Commit**

Commit message:

```text
Document and gate official API parity verification

Constraint: Official API alignment needs repeatable local and live evidence without unsafe default writes.
Rejected: Making advanced write live tests run by default | That can create external orders or transfers unexpectedly.
Confidence: high
Scope-risk: moderate
Directive: Keep dangerous live tests opt-in with exchange-specific flags.
Tested: go test ./...
Not-tested: Opt-in live write tests unless credentials and explicit flags are provided.
```

## Recommended Execution Order

1. Task 1: parity framework.
2. Task 2: full endpoint classification.
3. Task 3: request option foundation.
4. Tasks 4 through 9: one exchange at a time. Do not run more than two exchange implementations concurrently because shared model changes can conflict.
5. Task 10: adapter exposure after SDKs are stable.
6. Task 11: live gates and docs.

## Risk Controls

- Breaking adapter API changes are allowed when they support the architecture contract in `AGENTS.md`.
- Do not preserve base-symbol-only adapter signatures merely for compatibility; prefer quote-aware or instrument-aware APIs.
- Add no new dependencies unless the official docs require an encoding format not supported by the standard library or existing dependencies.
- For order placement, batch orders, algo orders, transfers, and leverage/margin changes, unit tests come first and live tests are opt-in.
- For official docs that are dynamic or ambiguous, classify rows as `blocked-by-official-api` with a short note in the parity file instead of guessing.

## Verification Checklist

- `go test ./sdkparity`
- `go test ./binance/sdk/... ./binance`
- `go test ./okx/sdk ./okx`
- `go test ./bybit/sdk ./bybit`
- `go test ./bitget/sdk ./bitget`
- `go test ./hyperliquid/sdk/... ./hyperliquid`
- `go test ./lighter/sdk ./lighter`
- `go test ./...`
- Optional live read tests with `RUN_FULL=1`.
- Optional live write tests only with `RUN_FULL=1` and the matching `*_ENABLE_*_TESTS=1` flag.

## Self-Review

Spec coverage: The plan covers all six requested exchanges and explicitly limits scope to spot and perpetual/swap/futures. It includes official documentation sources, endpoint classification, SDK implementation, adapter exposure, tests, live gates, and docs.

Placeholder scan: The plan avoids open-ended implementation placeholders. Where official endpoint lists are too large to fully inline, Task 2 requires every endpoint to be classified in the matrix before code work proceeds.

Type consistency: New method names use exchange-specific SDK clients and do not change the shared `Exchange` interface. Adapter exposure is delayed until SDK methods and tests exist.
