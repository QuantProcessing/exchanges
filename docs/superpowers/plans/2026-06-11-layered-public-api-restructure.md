# Layered Public API Restructure Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restructure the repository around three first-class user entry points: venue-native SDK packages, normalized adapter packages, and the TradingAccount runtime.

**Architecture:** Keep `github.com/QuantProcessing/exchanges` as the normalized root contract. Move SDKs to `sdk/<exchange>`, adapters to `adapter/<exchange>`, keep `account` as the lifecycle runtime, and enforce the package graph with architecture tests.

**Tech Stack:** Go 1.26, existing exchange SDKs and adapters, `go test -short`, repository architecture contract tests, existing shared `testsuite`.

---

## Target File Structure

Created directories:

- `sdk/`
- `adapter/`
- `internal/errs/`
- `internal/wsdispatch/`

Created files:

- `architecture_layout_test.go`
- `sdk/types.go`
- `sdk/errors.go`
- `internal/errs/errors.go`
- `internal/wsdispatch/ws.go`
- `internal/wsdispatch/utils.go`

Moved directories:

- `aster/sdk` -> `sdk/aster`
- `backpack/sdk` -> `sdk/backpack`
- `binance/sdk` -> `sdk/binance`
- `bitget/sdk` -> `sdk/bitget`
- `bybit/sdk` -> `sdk/bybit`
- `edgex/sdk` -> `sdk/edgex`
- `grvt/sdk` -> `sdk/grvt`
- `hyperliquid/sdk` -> `sdk/hyperliquid`
- `lighter/sdk` -> `sdk/lighter`
- `nado/sdk` -> `sdk/nado`
- `okx/sdk` -> `sdk/okx`
- `standx/sdk` -> `sdk/standx`

Moved adapter directories:

- `aster` -> `adapter/aster`
- `backpack` -> `adapter/backpack`
- `binance` -> `adapter/binance`
- `bitget` -> `adapter/bitget`
- `bybit` -> `adapter/bybit`
- `edgex` -> `adapter/edgex`
- `grvt` -> `adapter/grvt`
- `hyperliquid` -> `adapter/hyperliquid`
- `lighter` -> `adapter/lighter`
- `nado` -> `adapter/nado`
- `okx` -> `adapter/okx`
- `standx` -> `adapter/standx`

After each adapter directory move, the nested `adapter/<exchange>/sdk` directory is moved out to `sdk/<exchange>`.

## Task 1: Add Architecture Layout Contract Tests

**Files:**
- Create: `architecture_layout_test.go`

- [ ] **Step 1: Write the failing architecture tests**

Create `architecture_layout_test.go` with this content:

```go
package exchanges_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

var architectureExchangeNames = []string{
	"aster",
	"backpack",
	"binance",
	"bitget",
	"bybit",
	"edgex",
	"grvt",
	"hyperliquid",
	"lighter",
	"nado",
	"okx",
	"standx",
}

func TestNoRootLevelExchangeImplementationPackages(t *testing.T) {
	for _, name := range architectureExchangeNames {
		if info, err := os.Stat(name); err == nil && info.IsDir() {
			t.Fatalf("root-level exchange package %q must move to adapter/%s and sdk/%s", name, name, name)
		}
	}
}

func TestLayeredPublicEntrypointsExist(t *testing.T) {
	for _, dir := range []string{"sdk", "adapter", "account", "config", "testsuite"} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("expected top-level directory %q: %v", dir, err)
		}
		if !info.IsDir() {
			t.Fatalf("expected %q to be a directory", dir)
		}
	}
}

func TestEveryAdapterHasRequiredEntryFiles(t *testing.T) {
	for _, name := range architectureExchangeNames {
		for _, file := range []string{"options.go", "register.go"} {
			path := filepath.Join("adapter", name, file)
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("adapter package %q missing %s: %v", name, file, err)
			}
		}
	}
}

func TestRootPackageDoesNotImportLayerPackages(t *testing.T) {
	for _, file := range rootGoFiles(t) {
		for _, imp := range importsForFile(t, file) {
			if strings.HasPrefix(imp, "github.com/QuantProcessing/exchanges/sdk") ||
				strings.HasPrefix(imp, "github.com/QuantProcessing/exchanges/adapter") ||
				imp == "github.com/QuantProcessing/exchanges/account" ||
				strings.HasPrefix(imp, "github.com/QuantProcessing/exchanges/account/") {
				t.Fatalf("root file %s imports forbidden layer package %q", file, imp)
			}
		}
	}
}

func TestSDKPackagesDoNotImportRootAdapterOrAccount(t *testing.T) {
	walkGoFiles(t, "sdk", func(path string) {
		for _, imp := range importsForFile(t, path) {
			if imp == "github.com/QuantProcessing/exchanges" {
				t.Fatalf("SDK file %s imports root package; use github.com/QuantProcessing/exchanges/sdk or internal packages", path)
			}
			if strings.HasPrefix(imp, "github.com/QuantProcessing/exchanges/adapter") ||
				imp == "github.com/QuantProcessing/exchanges/account" ||
				strings.HasPrefix(imp, "github.com/QuantProcessing/exchanges/account/") {
				t.Fatalf("SDK file %s imports forbidden package %q", path, imp)
			}
		}
	})
}

func TestSDKPackagesDoNotImportOtherExchangeSDKs(t *testing.T) {
	for _, name := range architectureExchangeNames {
		root := filepath.Join("sdk", name)
		walkGoFiles(t, root, func(path string) {
			for _, imp := range importsForFile(t, path) {
				for _, other := range architectureExchangeNames {
					if other == name {
						continue
					}
					forbidden := "github.com/QuantProcessing/exchanges/sdk/" + other
					if imp == forbidden || strings.HasPrefix(imp, forbidden+"/") {
						t.Fatalf("SDK file %s imports another exchange SDK %q", path, imp)
					}
				}
			}
		})
	}
}

func TestAdapterPackagesOnlyImportOwnSDK(t *testing.T) {
	for _, name := range architectureExchangeNames {
		root := filepath.Join("adapter", name)
		walkGoFiles(t, root, func(path string) {
			for _, imp := range importsForFile(t, path) {
				if !strings.HasPrefix(imp, "github.com/QuantProcessing/exchanges/sdk/") {
					continue
				}
				own := "github.com/QuantProcessing/exchanges/sdk/" + name
				if imp != own && !strings.HasPrefix(imp, own+"/") {
					t.Fatalf("adapter file %s imports SDK outside its own exchange: %q", path, imp)
				}
			}
		})
	}
}

func TestAccountDoesNotImportSDKOrAdapter(t *testing.T) {
	walkGoFiles(t, "account", func(path string) {
		for _, imp := range importsForFile(t, path) {
			if strings.HasPrefix(imp, "github.com/QuantProcessing/exchanges/sdk") ||
				strings.HasPrefix(imp, "github.com/QuantProcessing/exchanges/adapter") {
				t.Fatalf("account file %s imports forbidden layer package %q", path, imp)
			}
		}
	})
}

func TestConfigAllImportsCanonicalAdapters(t *testing.T) {
	imports := map[string]bool{}
	walkGoFiles(t, filepath.Join("config", "all"), func(path string) {
		for _, imp := range importsForFile(t, path) {
			imports[imp] = true
		}
	})
	for _, name := range architectureExchangeNames {
		expected := "github.com/QuantProcessing/exchanges/adapter/" + name
		if !imports[expected] {
			t.Fatalf("config/all must blank-import %s", expected)
		}
	}
}

func rootGoFiles(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".go") {
			files = append(files, name)
		}
	}
	slices.Sort(files)
	return files
}

func walkGoFiles(t *testing.T, root string, visit func(path string)) {
	t.Helper()
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".go") {
			visit(path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func importsForFile(t *testing.T, path string) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse imports for %s: %v", path, err)
	}
	imports := make([]string, 0, len(file.Imports))
	for _, spec := range file.Imports {
		imports = append(imports, strings.Trim(spec.Path.Value, `"`))
	}
	return imports
}
```

- [ ] **Step 2: Run the new tests and confirm they fail on current layout**

Run:

```bash
go test -short . -run 'Test(NoRootLevelExchangeImplementationPackages|LayeredPublicEntrypointsExist|EveryAdapterHasRequiredEntryFiles|SDKPackagesDoNotImportRootAdapterOrAccount|ConfigAllImportsCanonicalAdapters)' -count=1
```

Expected: FAIL because `sdk/` and `adapter/` do not exist and root-level exchange directories still exist.

## Task 2: Extract SDK-Wide Primitives

**Files:**
- Create: `internal/errs/errors.go`
- Create: `sdk/errors.go`
- Create: `sdk/types.go`
- Modify: `errors.go`
- Modify: `models.go`

- [ ] **Step 1: Move shared sentinels to an internal errors package**

Create `internal/errs/errors.go`:

```go
package errs

import (
	"errors"
	"fmt"
)

var (
	ErrInsufficientBalance = errors.New("insufficient balance")
	ErrRateLimited         = errors.New("rate limited")
	ErrInvalidPrecision    = errors.New("invalid precision")
	ErrOrderNotFound       = errors.New("order not found")
	ErrSymbolNotFound      = errors.New("symbol not found")
	ErrMinNotional         = errors.New("below minimum notional")
	ErrMinQuantity         = errors.New("below minimum quantity")
	ErrAuthFailed          = errors.New("authentication failed")
	ErrNetworkTimeout      = errors.New("network timeout")
	ErrNotSupported        = errors.New("not supported")
)

type ExchangeError struct {
	Exchange string // Exchange name, e.g. "BINANCE"
	Code     string // Exchange-specific error code
	Message  string // Original error message from exchange
	Err      error  // Sentinel error for errors.Is matching
}

func (e *ExchangeError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("[%s] %s: %s", e.Exchange, e.Code, e.Message)
	}
	return fmt.Sprintf("[%s] %s", e.Exchange, e.Message)
}

func (e *ExchangeError) Unwrap() error {
	return e.Err
}

func NewExchangeError(exchange, code, message string, sentinel error) *ExchangeError {
	return &ExchangeError{
		Exchange: exchange,
		Code:     code,
		Message:  message,
		Err:      sentinel,
	}
}
```

- [ ] **Step 2: Re-export root unified errors from `errors.go`**

Replace root error sentinel ownership with re-exports from `internal/errs`:

```go
package exchanges

import "github.com/QuantProcessing/exchanges/internal/errs"

var (
	ErrInsufficientBalance = errs.ErrInsufficientBalance
	ErrRateLimited         = errs.ErrRateLimited
	ErrInvalidPrecision    = errs.ErrInvalidPrecision
	ErrOrderNotFound       = errs.ErrOrderNotFound
	ErrSymbolNotFound      = errs.ErrSymbolNotFound
	ErrMinNotional         = errs.ErrMinNotional
	ErrMinQuantity         = errs.ErrMinQuantity
	ErrAuthFailed          = errs.ErrAuthFailed
	ErrNetworkTimeout      = errs.ErrNetworkTimeout
	ErrNotSupported        = errs.ErrNotSupported
)

type ExchangeError = errs.ExchangeError

func NewExchangeError(exchange, code, message string, sentinel error) *ExchangeError {
	return errs.NewExchangeError(exchange, code, message, sentinel)
}
```

- [ ] **Step 3: Create SDK request options and SDK error re-exports**

Create `sdk/types.go`:

```go
package sdk

type RequestOpts struct {
	RecvWindowMillis int64
	ClientRequestID  string
}
```

Create `sdk/errors.go`:

```go
package sdk

import "github.com/QuantProcessing/exchanges/internal/errs"

var (
	ErrAuthFailed     = errs.ErrAuthFailed
	ErrOrderNotFound  = errs.ErrOrderNotFound
	ErrSymbolNotFound = errs.ErrSymbolNotFound
	ErrRateLimited    = errs.ErrRateLimited
)

type ExchangeError = errs.ExchangeError

func NewExchangeError(exchange, code, message string, sentinel error) *ExchangeError {
	return errs.NewExchangeError(exchange, code, message, sentinel)
}
```

- [ ] **Step 4: Remove `SDKRequestOpts` from root models**

Delete this type from `models.go`:

```go
type SDKRequestOpts struct {
	Limit int
}
```

- [ ] **Step 5: Update current SDK request option call sites before moving directories**

Replace root SDK option references:

```text
exchanges.SDKRequestOpts -> sdk.RequestOpts
```

Affected files:

- `binance/sdk/perp/client.go`
- `binance/sdk/perp/request_opts_test.go`
- `binance/sdk/spot/client.go`
- `binance/sdk/spot/request_opts_test.go`
- `bitget/sdk/client.go`
- `bitget/sdk/request_opts_test.go`
- `bybit/sdk/client.go`
- `bybit/sdk/request_opts_test.go`
- `hyperliquid/sdk/client.go`
- `hyperliquid/sdk/request_opts_test.go`
- `lighter/sdk/client.go`
- `lighter/sdk/request_opts_test.go`
- `okx/sdk/client.go`
- `okx/sdk/request_opts_test.go`

Each affected SDK file imports:

```go
sdkcore "github.com/QuantProcessing/exchanges/sdk"
```

and changes helper signatures from:

```go
func applySDKRequestOpts(params map[string]string, opts exchanges.SDKRequestOpts)
```

to:

```go
func applySDKRequestOpts(params map[string]string, opts sdkcore.RequestOpts)
```

- [ ] **Step 6: Update SDK rate-limit errors before moving directories**

Replace SDK root error usage:

```text
exchanges.NewExchangeError -> sdkcore.NewExchangeError
exchanges.ErrRateLimited -> sdkcore.ErrRateLimited
```

Affected files:

- `edgex/sdk/perp/client.go`
- `grvt/sdk/client.go`
- `hyperliquid/sdk/client.go`
- `lighter/sdk/client.go`
- `nado/sdk/client.go`
- `okx/sdk/client.go`
- `standx/sdk/client.go`

- [ ] **Step 7: Run SDK primitive tests**

Run:

```bash
go test -short ./binance/sdk/perp ./binance/sdk/spot ./bitget/sdk ./bybit/sdk ./hyperliquid/sdk ./lighter/sdk ./okx/sdk -run RequestOpts -count=1
```

Expected: PASS.

## Task 3: Move Exchange-Neutral SDK Helpers Out Of Exchange SDKs

**Files:**
- Create: `internal/wsdispatch/ws.go`
- Create: `internal/wsdispatch/utils.go`
- Modify: `aster/sdk/perp/ws_client.go`
- Modify: `aster/sdk/spot/ws_client.go`
- Modify: `binance/sdk/perp/ws_client.go`
- Modify: `binance/sdk/spot/ws_client.go`

- [ ] **Step 1: Create exchange-neutral websocket dispatch helpers**

Create `internal/wsdispatch/ws.go`:

```go
package wsdispatch

type MsgDispatcher interface {
	Dispatch(data []byte) error
}
```

Create `internal/wsdispatch/utils.go`:

```go
package wsdispatch

import "math/rand"

func GenerateRandomID() int64 {
	return rand.Int63()
}
```

- [ ] **Step 2: Replace Binance SDK common imports**

Change imports:

```text
github.com/QuantProcessing/exchanges/binance/sdk/common
```

to:

```text
github.com/QuantProcessing/exchanges/internal/wsdispatch
```

In the affected files, replace `common.MsgDispatcher` with
`wsdispatch.MsgDispatcher` and `common.GenerateRandomID` with
`wsdispatch.GenerateRandomID`.

- [ ] **Step 3: Delete old Binance SDK common helper files after replacements**

Delete:

- `binance/sdk/common/ws.go`
- `binance/sdk/common/utils.go`

- [ ] **Step 4: Run websocket helper package tests**

Run:

```bash
go test -short ./aster/sdk/perp ./aster/sdk/spot ./binance/sdk/perp ./binance/sdk/spot -run 'Test.*WS|Test.*WebSocket|Test.*Dispatcher' -count=1
```

Expected: PASS or SKIP for credential-gated tests. There must be no compile error.

## Task 4: Move SDK And Adapter Directories

**Files:**
- Move all exchange directories listed in the Target File Structure section

- [ ] **Step 1: Create layer directories**

Run:

```bash
mkdir -p adapter sdk
```

- [ ] **Step 2: Move each exchange directory and its SDK subtree**

Run these commands in order:

```bash
git mv aster adapter/aster
git mv adapter/aster/sdk sdk/aster

git mv backpack adapter/backpack
git mv adapter/backpack/sdk sdk/backpack

git mv binance adapter/binance
git mv adapter/binance/sdk sdk/binance

git mv bitget adapter/bitget
git mv adapter/bitget/sdk sdk/bitget

git mv bybit adapter/bybit
git mv adapter/bybit/sdk sdk/bybit

git mv edgex adapter/edgex
git mv adapter/edgex/sdk sdk/edgex

git mv grvt adapter/grvt
git mv adapter/grvt/sdk sdk/grvt

git mv hyperliquid adapter/hyperliquid
git mv adapter/hyperliquid/sdk sdk/hyperliquid

git mv lighter adapter/lighter
git mv adapter/lighter/sdk sdk/lighter

git mv nado adapter/nado
git mv adapter/nado/sdk sdk/nado

git mv okx adapter/okx
git mv adapter/okx/sdk sdk/okx

git mv standx adapter/standx
git mv adapter/standx/sdk sdk/standx
```

- [ ] **Step 3: Confirm no root exchange directories remain**

Run:

```bash
for d in aster backpack binance bitget bybit edgex grvt hyperliquid lighter nado okx standx; do test ! -d "$d" || exit 1; done
```

Expected: command exits with status 0.

## Task 5: Rewrite Import Paths

**Files:**
- Modify all `.go` files under the repository
- Modify: `README.md`
- Modify: `README_CN.md`
- Modify: `docs/contributing/adding-exchange-adapters.md`
- Modify: `docs/contributing/adding-option-adapters.md`
- Modify parity and plan docs only when they contain active instructions

- [ ] **Step 1: Rewrite SDK import paths**

For every exchange name, replace:

```text
github.com/QuantProcessing/exchanges/<exchange>/sdk
```

with:

```text
github.com/QuantProcessing/exchanges/sdk/<exchange>
```

Examples:

```text
github.com/QuantProcessing/exchanges/binance/sdk/perp -> github.com/QuantProcessing/exchanges/sdk/binance/perp
github.com/QuantProcessing/exchanges/hyperliquid/sdk -> github.com/QuantProcessing/exchanges/sdk/hyperliquid
github.com/QuantProcessing/exchanges/edgex/sdk/starkcurve -> github.com/QuantProcessing/exchanges/sdk/edgex/starkcurve
```

- [ ] **Step 2: Rewrite adapter import paths**

For every exchange name, replace:

```text
github.com/QuantProcessing/exchanges/<exchange>
```

with:

```text
github.com/QuantProcessing/exchanges/adapter/<exchange>
```

Do this only for imports that refer to adapter packages, not for newly rewritten
SDK imports.

- [ ] **Step 3: Update `config/all/all.go`**

Replace the file with:

```go
package all

import (
	_ "github.com/QuantProcessing/exchanges/adapter/aster"
	_ "github.com/QuantProcessing/exchanges/adapter/backpack"
	_ "github.com/QuantProcessing/exchanges/adapter/binance"
	_ "github.com/QuantProcessing/exchanges/adapter/bitget"
	_ "github.com/QuantProcessing/exchanges/adapter/bybit"
	_ "github.com/QuantProcessing/exchanges/adapter/edgex"
	_ "github.com/QuantProcessing/exchanges/adapter/grvt"
	_ "github.com/QuantProcessing/exchanges/adapter/hyperliquid"
	_ "github.com/QuantProcessing/exchanges/adapter/lighter"
	_ "github.com/QuantProcessing/exchanges/adapter/nado"
	_ "github.com/QuantProcessing/exchanges/adapter/okx"
	_ "github.com/QuantProcessing/exchanges/adapter/standx"
)
```

- [ ] **Step 4: Run gofmt**

Run:

```bash
gofmt -w .
```

- [ ] **Step 5: Verify import rewrite**

Run:

```bash
rg 'github.com/QuantProcessing/exchanges/(aster|backpack|binance|bitget|bybit|edgex|grvt|hyperliquid|lighter|nado|okx|standx)(/|")' --glob '*.go' --glob '*.md'
```

Expected: no matches for old root adapter or SDK import paths. Matches under archived docs are acceptable only if they are clearly historical examples and not active instructions.

## Task 6: Fix SDK Root Dependency Violations

**Files:**
- Modify files reported by `TestSDKPackagesDoNotImportRootAdapterOrAccount`

- [ ] **Step 1: Run SDK dependency test**

Run:

```bash
go test -short . -run TestSDKPackagesDoNotImportRootAdapterOrAccount -count=1
```

Expected before fixes: FAIL if any SDK file still imports root.

- [ ] **Step 2: Replace remaining SDK root imports**

For request options, use:

```go
sdkcore "github.com/QuantProcessing/exchanges/sdk"
```

For SDK rate-limit errors, use:

```go
sdkcore "github.com/QuantProcessing/exchanges/sdk"
```

No file under `sdk/` may import:

```go
exchanges "github.com/QuantProcessing/exchanges"
```

- [ ] **Step 3: Re-run SDK dependency test**

Run:

```bash
go test -short . -run TestSDKPackagesDoNotImportRootAdapterOrAccount -count=1
```

Expected: PASS.

## Task 7: Fix Adapter Own-SDK Dependency Violations

**Files:**
- Modify files reported by `TestAdapterPackagesOnlyImportOwnSDK`

- [ ] **Step 1: Run adapter dependency test**

Run:

```bash
go test -short . -run TestAdapterPackagesOnlyImportOwnSDK -count=1
```

Expected before fixes: FAIL if any adapter imports an SDK outside its own exchange.

- [ ] **Step 2: Move truly shared code into `internal/`**

If an adapter imports another exchange's SDK only for generic helpers, move the
helper to a focused `internal/` package and update both callers.

Allowed examples:

```go
import "github.com/QuantProcessing/exchanges/internal/wsdispatch"
import "github.com/QuantProcessing/exchanges/internal/mbx"
```

Forbidden examples:

```go
import "github.com/QuantProcessing/exchanges/sdk/binance/common"
import "github.com/QuantProcessing/exchanges/sdk/okx"
```

- [ ] **Step 3: Re-run adapter dependency test**

Run:

```bash
go test -short . -run TestAdapterPackagesOnlyImportOwnSDK -count=1
```

Expected: PASS.

## Task 8: Update Public Documentation

**Files:**
- Modify: `README.md`
- Modify: `README_CN.md`
- Modify: `doc.go`
- Modify: `docs/contributing/adding-exchange-adapters.md`
- Modify: `docs/contributing/adding-option-adapters.md`

- [ ] **Step 1: Update root package architecture docs**

In `doc.go`, update the architecture section to name the three user entry
points:

```go
// # Architecture
//
// The module exposes three public entry layers:
//
//   - Root package (exchanges): normalized interfaces, models, errors, registry,
//     capabilities, and helpers.
//   - SDK packages (sdk/binance, sdk/okx, ...): venue-native REST and WebSocket
//     clients aligned with official exchange APIs.
//   - Adapter packages (adapter/binance, adapter/okx, ...): normalized
//     cross-exchange convenience implementations of the root interfaces.
//   - Account package (account): TradingAccount, OrderFlow, stream health, and
//     portfolio-level lifecycle runtime.
```

- [ ] **Step 2: Update README quick-start imports**

Adapter quick start:

```go
import (
	"context"
	"fmt"

	exchanges "github.com/QuantProcessing/exchanges"
	binance "github.com/QuantProcessing/exchanges/adapter/binance"
	"github.com/shopspring/decimal"
)
```

SDK quick start:

```go
import (
	"context"
	"fmt"

	binanceperp "github.com/QuantProcessing/exchanges/sdk/binance/perp"
)
```

TradingAccount quick start:

```go
import (
	"context"

	"github.com/QuantProcessing/exchanges/account"
	binance "github.com/QuantProcessing/exchanges/adapter/binance"
)
```

- [ ] **Step 3: Update adapter contribution docs**

Replace every new-exchange package path example with:

```text
adapter/<exchange>/
sdk/<exchange>/
```

The guide must state that SDK additions do not imply adapter additions and that
adapter exposure requires a separate root-interface or optional-capability
decision.

## Task 9: Update Verification Scripts

**Files:**
- Modify: `scripts/verify_exchange.sh` when present
- Modify: `scripts/verify_full.sh` when present
- Modify: `scripts/verify_soak.sh` when present

- [ ] **Step 1: Update exchange package mappings**

`scripts/verify_exchange.sh <exchange>` should run:

```bash
go test -short "./sdk/${exchange}/..." "./adapter/${exchange}/..."
```

For exchanges with nested SDK packages, `./sdk/${exchange}/...` covers all
product SDKs.

- [ ] **Step 2: Update full verification package paths**

Every old path such as:

```text
./binance/sdk/perp
```

becomes:

```text
./sdk/binance/perp
```

Every old adapter path such as:

```text
./binance
```

becomes:

```text
./adapter/binance
```

- [ ] **Step 3: Run quick script syntax checks**

Run:

```bash
bash -n scripts/verify_exchange.sh
bash -n scripts/verify_full.sh
bash -n scripts/verify_soak.sh
```

Expected: PASS for every script that exists.

## Task 10: Run Layered Verification

**Files:**
- No source edits unless failures reveal missed import rewrites

- [ ] **Step 1: Run root architecture tests**

Run:

```bash
go test -short . -run 'Test(NoRootLevelExchangeImplementationPackages|LayeredPublicEntrypointsExist|EveryAdapterHasRequiredEntryFiles|RootPackageDoesNotImportLayerPackages|SDKPackagesDoNotImportRootAdapterOrAccount|SDKPackagesDoNotImportOtherExchangeSDKs|AdapterPackagesOnlyImportOwnSDK|AccountDoesNotImportSDKOrAdapter|ConfigAllImportsCanonicalAdapters)' -count=1
```

Expected: PASS.

- [ ] **Step 2: Run package layer tests**

Run:

```bash
go test -short .
go test -short ./sdk/...
go test -short ./adapter/...
go test -short ./account
go test -short ./config/...
go test -short ./testsuite
```

Expected: PASS or SKIP for tests that are explicitly credential-gated.

- [ ] **Step 3: Run full quick gate**

Run:

```bash
go test -short ./...
```

Expected: PASS.

- [ ] **Step 4: Run exchange-focused smoke checks**

Run:

```bash
go test -short ./sdk/binance/... ./adapter/binance/...
go test -short ./sdk/okx/... ./adapter/okx/...
go test -short ./sdk/bybit/... ./adapter/bybit/...
go test -short ./sdk/hyperliquid/... ./adapter/hyperliquid/...
```

Expected: PASS or SKIP for tests that are explicitly credential-gated.

## Task 11: Commit

**Files:**
- All changed files

- [ ] **Step 1: Review changed files**

Run:

```bash
git status --short
```

Expected: only restructure, import rewrite, architecture test, docs, and
verification script changes are present.

- [ ] **Step 2: Commit with Lore protocol**

Run:

```bash
git add .
git commit -m "Reshape imports around layered user entry points

Constraint: Repository has no stable public release, so breaking import paths are allowed.
Rejected: Keep SDKs nested below adapter packages | SDK is a first-class user entry point, not adapter internals.
Confidence: high
Scope-risk: broad
Directive: Keep root as normalized contract, sdk/* as venue-native protocol, adapter/* as normalized implementations, and account as lifecycle runtime.
Tested: go test -short ./...
Not-tested: live credential-gated full regression unless scripts/verify_full.sh is run separately."
```

Expected: commit succeeds.

## Self-Review

- The plan covers the design requirement that users can choose SDK, adapter, or
  TradingAccount through import paths.
- The plan includes explicit package graph tests to prevent architectural drift.
- The plan handles current SDK root imports by introducing `sdk` primitives and
  private internal sentinel ownership.
- The plan handles current cross-exchange SDK helper reuse by moving generic
  helpers to `internal/wsdispatch`.
- The plan intentionally does not preserve old root-level exchange import paths
  because this is an approved breaking restructure.
