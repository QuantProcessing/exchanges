# Nautilus V2 Adapter Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first testable V2 slice: shared domain model, venue contracts, account cache/reconciliation foundation, and a Binance spot/perp pilot.

**Architecture:** Keep the existing `sdk/<venue>` and `adapter/<venue>` layout. Add `model/` as a pure leaf package, `venue/` as the adapter contract package, extend `account/` for lifecycle state, and migrate only `adapter/binance` first. Do not create a `runtime/` package.

**Tech Stack:** Go, `github.com/shopspring/decimal`, current `sdk/binance/*`, current `adapter/binance`, current `account`, current `testsuite`, `go test`.

---

## Source Spec

Implement from:

- `docs/superpowers/specs/2026-06-11-nautilus-inspired-v2-adapter-redesign-design.md`

This plan intentionally covers the first V2 foundation slice only. It does not
migrate every adapter and does not remove V1 interfaces until the Binance pilot
passes.

## Target File Structure

Create:

- `model/doc.go`
- `model/errors.go`
- `model/identifiers.go`
- `model/identifiers_test.go`
- `model/money.go`
- `model/money_test.go`
- `model/instrument.go`
- `model/instrument_test.go`
- `model/account.go`
- `model/account_test.go`
- `model/order.go`
- `model/market_data.go`
- `venue/doc.go`
- `venue/interfaces.go`
- `venue/subscription.go`
- `venue/capabilities.go`
- `venue/registry.go`
- `venue/registry_test.go`
- `account/cache_v2.go`
- `account/cache_v2_test.go`
- `account/reconciler_v2.go`
- `account/trading_account_v2.go`
- `adapter/binance/v2_symbol.go`
- `adapter/binance/v2_symbol_test.go`
- `adapter/binance/v2_instruments.go`
- `adapter/binance/v2_instruments_test.go`
- `adapter/binance/v2_market_data.go`
- `adapter/binance/v2_market_data_test.go`
- `adapter/binance/v2_execution.go`
- `adapter/binance/v2_account_state.go`
- `testsuite/v2_model_suite.go`
- `testsuite/v2_venue_suite.go`
- `testsuite/v2_lifecycle_suite.go`

Modify:

- `go.mod` only if `decimal` is not already required.
- `adapter/binance/register.go` only when the pilot is ready to register V2
  declared capabilities.
- `config/all/all.go` only if the V2 registry uses blank-import registration.

Do not move existing `adapter/` or `sdk/` directories.

## Task 1: Add `model` Identity and Error Foundation

**Files:**

- Create: `model/doc.go`
- Create: `model/errors.go`
- Create: `model/identifiers.go`
- Create: `model/identifiers_test.go`

- [ ] **Step 1: Write identity tests**

Create `model/identifiers_test.go`:

```go
package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseInstrumentID(t *testing.T) {
	got, err := ParseInstrumentID("BTC-USDT-PERP.BINANCE")
	require.NoError(t, err)
	require.Equal(t, InstrumentID{Symbol: "BTC-USDT-PERP", Venue: VenueBinance}, got)
	require.Equal(t, "BTC-USDT-PERP.BINANCE", got.String())
}

func TestParseInstrumentIDRejectsMissingVenue(t *testing.T) {
	_, err := ParseInstrumentID("BTC-USDT-PERP")
	require.ErrorIs(t, err, ErrInvalidInstrumentID)
}

func TestParseInstrumentIDRejectsEmptySymbol(t *testing.T) {
	_, err := ParseInstrumentID(".BINANCE")
	require.ErrorIs(t, err, ErrInvalidInstrumentID)
}

func TestInstrumentIDValidateRejectsEmptyVenue(t *testing.T) {
	err := (InstrumentID{Symbol: "BTC-USDT-PERP"}).Validate()
	require.ErrorIs(t, err, ErrInvalidInstrumentID)
}
```

- [ ] **Step 2: Run the failing identity tests**

Run:

```bash
go test ./model -run TestParseInstrumentID -count=1
```

Expected: package `model` does not exist or tests fail because the functions are
not present yet.

- [ ] **Step 3: Add model package docs and errors**

Create `model/doc.go`:

```go
// Package model contains the normalized V2 trading domain model.
package model
```

Create `model/errors.go`:

```go
package model

import "errors"

var (
	ErrInvalidInstrumentID = errors.New("invalid instrument id")
	ErrInstrumentNotLoaded = errors.New("instrument not loaded")
	ErrInvalidMoney        = errors.New("invalid money")
	ErrInvalidInstrument   = errors.New("invalid instrument")
	ErrInvalidAccountState = errors.New("invalid account state")
	ErrNotSupported        = errors.New("not supported")
)
```

- [ ] **Step 4: Add identifiers implementation**

Create `model/identifiers.go`:

```go
package model

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

type Venue string

const (
	VenueBinance Venue = "BINANCE"
	VenueOKX     Venue = "OKX"
	VenueBybit   Venue = "BYBIT"
)

type AccountID string
type ClientOrderID string
type OrderID string
type PositionID string
type TradeID string

var clientOrderIDCounter atomic.Uint64

type InstrumentID struct {
	Symbol string
	Venue  Venue
}

func ParseInstrumentID(s string) (InstrumentID, error) {
	s = strings.TrimSpace(s)
	parts := strings.Split(s, ".")
	if len(parts) != 2 {
		return InstrumentID{}, fmt.Errorf("%w: %q", ErrInvalidInstrumentID, s)
	}
	id := InstrumentID{
		Symbol: strings.ToUpper(strings.TrimSpace(parts[0])),
		Venue:  Venue(strings.ToUpper(strings.TrimSpace(parts[1]))),
	}
	if err := id.Validate(); err != nil {
		return InstrumentID{}, err
	}
	return id, nil
}

func MustInstrumentID(s string) InstrumentID {
	id, err := ParseInstrumentID(s)
	if err != nil {
		panic(err)
	}
	return id
}

func (id InstrumentID) String() string {
	if id.Symbol == "" && id.Venue == "" {
		return ""
	}
	return id.Symbol + "." + string(id.Venue)
}

func (id InstrumentID) Validate() error {
	if strings.TrimSpace(id.Symbol) == "" || strings.TrimSpace(string(id.Venue)) == "" {
		return fmt.Errorf("%w: %q", ErrInvalidInstrumentID, id.String())
	}
	if strings.Contains(id.Symbol, ".") {
		return fmt.Errorf("%w: symbol contains venue separator: %q", ErrInvalidInstrumentID, id.String())
	}
	return nil
}

func NewClientOrderID() ClientOrderID {
	n := clientOrderIDCounter.Add(1)
	return ClientOrderID(fmt.Sprintf("cli_%x_%x", time.Now().UnixNano(), n))
}
```

- [ ] **Step 5: Run identity tests**

Run:

```bash
go test ./model -run TestParseInstrumentID -count=1
```

Expected: PASS.

## Task 2: Add Money and Instrument Types

**Files:**

- Create: `model/money.go`
- Create: `model/money_test.go`
- Create: `model/instrument.go`
- Create: `model/instrument_test.go`

- [ ] **Step 1: Write money invariant tests**

Create `model/money_test.go`:

```go
package model

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestNewMoneyRejectsMissingCurrency(t *testing.T) {
	_, err := NewMoney(decimal.NewFromInt(1), "")
	require.ErrorIs(t, err, ErrInvalidMoney)
}

func TestMoneySameCurrency(t *testing.T) {
	a := Money{Amount: decimal.NewFromInt(1), Currency: USDT}
	b := Money{Amount: decimal.NewFromInt(2), Currency: USDT}
	require.NoError(t, a.RequireSameCurrency(b))
}

func TestMoneySameCurrencyRejectsMismatch(t *testing.T) {
	a := Money{Amount: decimal.NewFromInt(1), Currency: USDT}
	b := Money{Amount: decimal.NewFromInt(2), Currency: USDC}
	require.ErrorIs(t, a.RequireSameCurrency(b), ErrInvalidMoney)
}
```

- [ ] **Step 2: Write instrument validation tests**

Create `model/instrument_test.go`:

```go
package model

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestInstrumentValidateRequiresOptionSpecForCryptoOption(t *testing.T) {
	inst := Instrument{
		ID:        MustInstrumentID("BTC-20260626-100000-C.BYBIT"),
		RawSymbol: "BTC-26JUN26-100000-C",
		Type:      InstrumentTypeCryptoOption,
		Base:      BTC,
		Quote:     USDC,
		Settle:    USDC,
		PriceStep: decimal.RequireFromString("0.1"),
		SizeStep:  decimal.RequireFromString("0.01"),
	}
	require.ErrorIs(t, inst.Validate(), ErrInvalidInstrument)
}

func TestInstrumentMakeQtyRejectsBadStep(t *testing.T) {
	inst := Instrument{
		ID:        MustInstrumentID("BTC-USDT-PERP.BINANCE"),
		RawSymbol: "BTCUSDT",
		Type:      InstrumentTypeCryptoPerp,
		Base:      BTC,
		Quote:     USDT,
		Settle:    USDT,
		PriceStep: decimal.RequireFromString("0.1"),
		SizeStep:  decimal.RequireFromString("0.001"),
	}
	_, err := inst.MakeQty(decimal.RequireFromString("0.0005"))
	require.ErrorIs(t, err, ErrInvalidInstrument)
}

func TestCryptoOptionInstrumentValidates(t *testing.T) {
	inst := Instrument{
		ID:        MustInstrumentID("BTC-20260626-100000-C.BYBIT"),
		RawSymbol: "BTC-26JUN26-100000-C",
		Type:      InstrumentTypeCryptoOption,
		Base:      BTC,
		Quote:     USDC,
		Settle:    USDC,
		PriceStep: decimal.RequireFromString("0.1"),
		SizeStep:  decimal.RequireFromString("0.01"),
		Option: &OptionSpec{
			Underlying: MustInstrumentID("BTC-USDC-PERP.BYBIT"),
			Strike:     decimal.RequireFromString("100000"),
			Kind:       OptionKindCall,
			Expiration: time.Date(2026, 6, 26, 8, 0, 0, 0, time.UTC),
			Exercise:   ExerciseStyleEuropean,
			Settlement: SettlementStyleCash,
		},
	}
	require.NoError(t, inst.Validate())
}
```

- [ ] **Step 3: Add money implementation**

Create `model/money.go`:

```go
package model

import (
	"fmt"
	"strings"

	"github.com/shopspring/decimal"
)

type Currency string

const (
	USDT Currency = "USDT"
	USDC Currency = "USDC"
	DUSD Currency = "DUSD"
	USD  Currency = "USD"
	BTC  Currency = "BTC"
	ETH  Currency = "ETH"
)

type Money struct {
	Amount   decimal.Decimal
	Currency Currency
}

func NewMoney(amount decimal.Decimal, currency Currency) (Money, error) {
	m := Money{Amount: amount, Currency: Currency(strings.ToUpper(strings.TrimSpace(string(currency))))}
	if err := m.Validate(); err != nil {
		return Money{}, err
	}
	return m, nil
}

func (m Money) Validate() error {
	if strings.TrimSpace(string(m.Currency)) == "" {
		return fmt.Errorf("%w: missing currency", ErrInvalidMoney)
	}
	return nil
}

func (m Money) RequireSameCurrency(other Money) error {
	if err := m.Validate(); err != nil {
		return err
	}
	if err := other.Validate(); err != nil {
		return err
	}
	if m.Currency != other.Currency {
		return fmt.Errorf("%w: %s != %s", ErrInvalidMoney, m.Currency, other.Currency)
	}
	return nil
}
```

- [ ] **Step 4: Add instrument implementation**

Create `model/instrument.go`:

```go
package model

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

type Metadata map[string]string

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

type OptionKind string

const (
	OptionKindCall OptionKind = "CALL"
	OptionKindPut  OptionKind = "PUT"
)

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

func (i Instrument) Validate() error {
	if err := i.ID.Validate(); err != nil {
		return err
	}
	if i.Type == "" {
		return fmt.Errorf("%w: missing type", ErrInvalidInstrument)
	}
	if i.PriceStep.LessThanOrEqual(decimal.Zero) {
		return fmt.Errorf("%w: invalid price step", ErrInvalidInstrument)
	}
	if i.SizeStep.LessThanOrEqual(decimal.Zero) {
		return fmt.Errorf("%w: invalid size step", ErrInvalidInstrument)
	}
	if i.Type == InstrumentTypeCryptoOption && i.Option == nil {
		return fmt.Errorf("%w: crypto option requires option spec", ErrInvalidInstrument)
	}
	if i.Option != nil {
		if err := i.Option.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (o OptionSpec) Validate() error {
	if err := o.Underlying.Validate(); err != nil {
		return err
	}
	if o.Strike.LessThanOrEqual(decimal.Zero) {
		return fmt.Errorf("%w: invalid option strike", ErrInvalidInstrument)
	}
	if o.Kind != OptionKindCall && o.Kind != OptionKindPut {
		return fmt.Errorf("%w: invalid option kind", ErrInvalidInstrument)
	}
	if o.Expiration.IsZero() {
		return fmt.Errorf("%w: missing option expiration", ErrInvalidInstrument)
	}
	return nil
}

func (i Instrument) MakePrice(v decimal.Decimal) (decimal.Decimal, error) {
	if i.PriceStep.LessThanOrEqual(decimal.Zero) || !v.Mod(i.PriceStep).IsZero() {
		return decimal.Zero, fmt.Errorf("%w: invalid price step", ErrInvalidInstrument)
	}
	return v, nil
}

func (i Instrument) MakeQty(v decimal.Decimal) (decimal.Decimal, error) {
	if i.SizeStep.LessThanOrEqual(decimal.Zero) || !v.Mod(i.SizeStep).IsZero() {
		return decimal.Zero, fmt.Errorf("%w: invalid size step", ErrInvalidInstrument)
	}
	return v, nil
}
```

- [ ] **Step 5: Run model tests**

Run:

```bash
go test ./model -count=1
```

Expected: PASS.

## Task 3: Add Account State, Order, and Market Data Models

**Files:**

- Create: `model/account.go`
- Create: `model/account_test.go`
- Create: `model/order.go`
- Create: `model/market_data.go`

- [ ] **Step 1: Write account invariant tests**

Create `model/account_test.go`:

```go
package model

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestNewBalanceRequiresSameCurrency(t *testing.T) {
	total := Money{Amount: decimal.NewFromInt(10), Currency: USDT}
	locked := Money{Amount: decimal.NewFromInt(1), Currency: USDC}
	free := Money{Amount: decimal.NewFromInt(9), Currency: USDT}
	_, err := NewBalance(total, locked, free)
	require.ErrorIs(t, err, ErrInvalidMoney)
}

func TestNewBalanceRequiresInvariant(t *testing.T) {
	total := Money{Amount: decimal.NewFromInt(10), Currency: USDT}
	locked := Money{Amount: decimal.NewFromInt(2), Currency: USDT}
	free := Money{Amount: decimal.NewFromInt(7), Currency: USDT}
	_, err := NewBalance(total, locked, free)
	require.ErrorIs(t, err, ErrInvalidAccountState)
}

func TestBalanceFromTotalAndFreeDerivesLocked(t *testing.T) {
	total := Money{Amount: decimal.NewFromInt(10), Currency: USDT}
	free := Money{Amount: decimal.NewFromInt(8), Currency: USDT}
	got, err := BalanceFromTotalAndFree(total, free)
	require.NoError(t, err)
	require.True(t, got.Locked.Amount.Equal(decimal.NewFromInt(2)))
}
```

- [ ] **Step 2: Add account model**

Create `model/account.go`:

```go
package model

import (
	"fmt"
	"time"
)

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

type AccountBalance struct {
	Total  Money
	Locked Money
	Free   Money
}

type MarginBalance struct {
	Initial     Money
	Maintenance Money
	Instrument  *InstrumentID
}

func NewBalance(total, locked, free Money) (AccountBalance, error) {
	if err := total.RequireSameCurrency(locked); err != nil {
		return AccountBalance{}, err
	}
	if err := total.RequireSameCurrency(free); err != nil {
		return AccountBalance{}, err
	}
	if !locked.Amount.Add(free.Amount).Equal(total.Amount) {
		return AccountBalance{}, fmt.Errorf("%w: total must equal locked plus free", ErrInvalidAccountState)
	}
	return AccountBalance{Total: total, Locked: locked, Free: free}, nil
}

func BalanceFromTotalAndFree(total, free Money) (AccountBalance, error) {
	if err := total.RequireSameCurrency(free); err != nil {
		return AccountBalance{}, err
	}
	locked := Money{Amount: total.Amount.Sub(free.Amount), Currency: total.Currency}
	return NewBalance(total, locked, free)
}

func BalanceFromTotalAndLocked(total, locked Money) (AccountBalance, error) {
	if err := total.RequireSameCurrency(locked); err != nil {
		return AccountBalance{}, err
	}
	free := Money{Amount: total.Amount.Sub(locked.Amount), Currency: total.Currency}
	return NewBalance(total, locked, free)
}
```

- [ ] **Step 3: Add order and market data model structures**

Create `model/order.go`:

```go
package model

import (
	"time"

	"github.com/shopspring/decimal"
)

type OrderSide string
type OrderType string
type OrderStatus string
type TimeInForce string

const (
	OrderSideBuy  OrderSide = "buy"
	OrderSideSell OrderSide = "sell"

	OrderTypeMarket OrderType = "market"
	OrderTypeLimit  OrderType = "limit"

	OrderStatusSubmitted       OrderStatus = "submitted"
	OrderStatusAccepted        OrderStatus = "accepted"
	OrderStatusRejected        OrderStatus = "rejected"
	OrderStatusPartiallyFilled OrderStatus = "partially_filled"
	OrderStatusFilled          OrderStatus = "filled"
	OrderStatusCanceled        OrderStatus = "canceled"
	OrderStatusExpired         OrderStatus = "expired"
)

type SubmitOrder struct {
	InstrumentID InstrumentID
	Side         OrderSide
	Type         OrderType
	Quantity     decimal.Decimal
	Price        decimal.Decimal
	ClientID     ClientOrderID
	ReduceOnly   bool
	TimeInForce  TimeInForce
}

type ModifyOrder struct {
	InstrumentID InstrumentID
	OrderID      OrderID
	ClientID     ClientOrderID
	Quantity     decimal.Decimal
	Price        decimal.Decimal
}

type CancelOrder struct {
	InstrumentID InstrumentID
	OrderID      OrderID
	ClientID     ClientOrderID
}

type CancelAllOrders struct {
	InstrumentID InstrumentID
}

type OrderStatusReport struct {
	AccountID    AccountID
	InstrumentID InstrumentID
	OrderID      OrderID
	ClientID     ClientOrderID
	Status       OrderStatus
	Side         OrderSide
	Type         OrderType
	Quantity     decimal.Decimal
	FilledQty    decimal.Decimal
	AvgPrice     decimal.Decimal
	EventTime    time.Time
}

type FillReport struct {
	AccountID    AccountID
	InstrumentID InstrumentID
	OrderID      OrderID
	ClientID     ClientOrderID
	TradeID      TradeID
	Side         OrderSide
	Quantity     decimal.Decimal
	Price        decimal.Decimal
	Fee          Money
	EventTime    time.Time
}

type PositionSide string

const (
	PositionSideLong  PositionSide = "long"
	PositionSideShort PositionSide = "short"
	PositionSideFlat  PositionSide = "flat"
)

type PositionStatusReport struct {
	AccountID    AccountID
	InstrumentID InstrumentID
	PositionID   PositionID
	Side         PositionSide
	Quantity     decimal.Decimal
	AvgPrice     decimal.Decimal
	Unrealized   Money
	EventTime    time.Time
}

type ExecutionEvent struct {
	AccountState *AccountState
	Order        *OrderStatusReport
	Fill         *FillReport
	Position     *PositionStatusReport
}
```

Create `model/market_data.go`:

```go
package model

import (
	"time"

	"github.com/shopspring/decimal"
)

type Ticker struct {
	InstrumentID InstrumentID
	Bid          decimal.Decimal
	Ask          decimal.Decimal
	Last         decimal.Decimal
	EventTime    time.Time
}

type OrderBookLevel struct {
	Price decimal.Decimal
	Size  decimal.Decimal
}

type OrderBook struct {
	InstrumentID InstrumentID
	Bids         []OrderBookLevel
	Asks         []OrderBookLevel
	EventTime    time.Time
}

type Trade struct {
	InstrumentID InstrumentID
	TradeID      TradeID
	Price        decimal.Decimal
	Size         decimal.Decimal
	Side         OrderSide
	EventTime    time.Time
}

type BarSpec struct {
	Interval string
}

type Bar struct {
	InstrumentID InstrumentID
	Spec         BarSpec
	Open         decimal.Decimal
	High         decimal.Decimal
	Low          decimal.Decimal
	Close        decimal.Decimal
	Volume       decimal.Decimal
	EventTime    time.Time
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

- [ ] **Step 4: Run model tests**

Run:

```bash
go test ./model -count=1
```

Expected: PASS.

## Task 4: Add `venue` Contracts and Registry

**Files:**

- Create: `venue/doc.go`
- Create: `venue/interfaces.go`
- Create: `venue/subscription.go`
- Create: `venue/capabilities.go`
- Create: `venue/registry.go`
- Create: `venue/registry_test.go`

- [ ] **Step 1: Write registry tests**

Create `venue/registry_test.go`:

```go
package venue

import (
	"context"
	"testing"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/stretchr/testify/require"
)

func TestRegistryRegisterAndOpen(t *testing.T) {
	reg := NewRegistry()
	reg.Register(model.VenueBinance, func(ctx context.Context, cfg map[string]string) (Adapter, error) {
		return fakeAdapter{venue: model.VenueBinance}, nil
	})
	got, err := reg.Open(context.Background(), model.VenueBinance, nil)
	require.NoError(t, err)
	require.Equal(t, model.VenueBinance, got.Venue())
}

func TestRegistryUnknownVenue(t *testing.T) {
	reg := NewRegistry()
	_, err := reg.Open(context.Background(), model.Venue("MISSING"), nil)
	require.ErrorIs(t, err, ErrUnknownVenue)
}

type fakeAdapter struct{ venue model.Venue }

func (f fakeAdapter) Venue() model.Venue { return f.venue }
func (f fakeAdapter) Instruments() InstrumentProvider { return nil }
func (f fakeAdapter) MarketData() MarketDataClient { return nil }
func (f fakeAdapter) Execution() ExecutionClient { return nil }
func (f fakeAdapter) Capabilities() DeclaredCapabilities { return DeclaredCapabilities{Venue: f.venue} }
func (f fakeAdapter) Close() error { return nil }
```

- [ ] **Step 2: Add venue contracts**

Create `venue/doc.go`:

```go
// Package venue defines V2 adapter contracts implemented by concrete exchange adapters.
package venue
```

Create `venue/interfaces.go` using the design spec signatures. Use concrete
query structs even if they start small:

```go
package venue

import (
	"context"
	"time"

	"github.com/QuantProcessing/exchanges/model"
)

type Adapter interface {
	Venue() model.Venue
	Instruments() InstrumentProvider
	MarketData() MarketDataClient
	Execution() ExecutionClient
	Capabilities() DeclaredCapabilities
	Close() error
}

type InstrumentProvider interface {
	LoadAll(ctx context.Context) error
	Load(ctx context.Context, id model.InstrumentID) (model.Instrument, error)
	Find(ctx context.Context, q InstrumentQuery) ([]model.Instrument, error)
	Get(id model.InstrumentID) (model.Instrument, bool)
	List() []model.Instrument
}

type InstrumentQuery struct {
	Venue      model.Venue
	Type       model.InstrumentType
	Base       model.Currency
	Quote      model.Currency
	Settle     model.Currency
	Underlying *model.InstrumentID
	ExpiresGTE time.Time
	ExpiresLTE time.Time
	OptionKind model.OptionKind
}

type ProductHint string

const (
	ProductHintSpot   ProductHint = "spot"
	ProductHintPerp   ProductHint = "perp"
	ProductHintOption ProductHint = "option"
)

type SymbolNormalizer interface {
	ToInstrumentID(raw string, hint ProductHint) (model.InstrumentID, error)
	ToVenueSymbol(id model.InstrumentID) (string, error)
}
```

Add market data and execution contracts to `venue/interfaces.go`:

```go
type TradeQuery struct {
	Limit int
	Since time.Time
}

type BarQuery struct {
	Limit int
	Since time.Time
	Until time.Time
}

type ChainQuery struct {
	Limit int
}

type OrderStatusQuery struct {
	InstrumentID model.InstrumentID
	Since        time.Time
}

type FillQuery struct {
	InstrumentID model.InstrumentID
	Since        time.Time
}

type PositionQuery struct {
	InstrumentID model.InstrumentID
}

type TickerHandler func(model.Ticker)
type OrderBookHandler func(model.OrderBook)
type TradeHandler func(model.Trade)
type BarHandler func(model.Bar)
type OptionGreeksHandler func(model.OptionGreeks)
type OptionChainHandler func(model.OptionChainSlice)

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

type OptionMarketDataClient interface {
	FetchOptionChain(ctx context.Context, series model.OptionSeriesID, q ChainQuery) ([]model.Instrument, error)
	SubscribeOptionGreeks(ctx context.Context, id model.InstrumentID, h OptionGreeksHandler) (Subscription, error)
	SubscribeOptionChain(ctx context.Context, req OptionChainSubscription, h OptionChainHandler) (Subscription, error)
}

type OptionChainSubscription struct {
	Series model.OptionSeriesID
}

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

type ExecutionHealth struct {
	Connected     bool
	AccountReady  bool
	LastEventTime time.Time
	LastError     error
}
```

- [ ] **Step 3: Add subscription and capabilities**

Create `venue/subscription.go`:

```go
package venue

type Subscription interface {
	ID() string
	Close() error
	Done() <-chan struct{}
	Err() error
}
```

Create `venue/capabilities.go`:

```go
package venue

import (
	"time"

	"github.com/QuantProcessing/exchanges/model"
)

type DeclaredCapabilities struct {
	Venue           model.Venue
	AccountTypes    []model.AccountType
	InstrumentTypes []model.InstrumentType
	MarketData      MarketDataCapabilities
	Execution       ExecutionCapabilities
	AccountState    AccountStateCapabilities
	Reconciliation  ReconciliationCapabilities
}

type MarketDataCapabilities struct {
	Ticker      bool
	OrderBook   bool
	Trades      bool
	Bars        bool
	Options     bool
	PrivateData bool
}

type ExecutionCapabilities struct {
	Submit       bool
	Cancel       bool
	Modify       bool
	CancelAll    bool
	BatchOrders  bool
	OrderReports bool
	FillReports  bool
}

type AccountStateCapabilities struct {
	Snapshot  bool
	Balances  bool
	Margins   bool
	Positions bool
}

type ReconciliationCapabilities struct {
	Startup   bool
	Reconnect bool
}

type CertifiedCapabilities struct {
	Venue          model.Venue
	AccountType    model.AccountType
	InstrumentType model.InstrumentType
	Environment    string
	TestRunID      string
	Suites         []CertifiedSuite
	CertifiedAt    time.Time
}

type CertifiedSuite struct {
	Name     string
	Passed   bool
	Skipped  bool
	Reason   string
	Duration time.Duration
}

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

type StreamHealth struct {
	Ready     bool
	Stale     bool
	LastEvent time.Time
	LastError error
}

type RuntimeError struct {
	At  time.Time
	Err error
}
```

- [ ] **Step 4: Add registry**

Create `venue/registry.go`:

```go
package venue

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/QuantProcessing/exchanges/model"
)

var ErrUnknownVenue = errors.New("unknown venue")

type Constructor func(ctx context.Context, cfg map[string]string) (Adapter, error)

type Registry struct {
	mu    sync.RWMutex
	ctors map[model.Venue]Constructor
}

func NewRegistry() *Registry {
	return &Registry{ctors: make(map[model.Venue]Constructor)}
}

func (r *Registry) Register(v model.Venue, ctor Constructor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ctors[v] = ctor
}

func (r *Registry) Open(ctx context.Context, v model.Venue, cfg map[string]string) (Adapter, error) {
	r.mu.RLock()
	ctor := r.ctors[v]
	r.mu.RUnlock()
	if ctor == nil {
		return nil, fmt.Errorf("%w: %s", ErrUnknownVenue, v)
	}
	return ctor(ctx, cfg)
}
```

- [ ] **Step 5: Run venue tests**

Run:

```bash
go test ./venue -count=1
```

Expected: PASS.

## Task 5: Add Account Cache Foundation

**Files:**

- Create: `account/cache_v2.go`
- Create: `account/cache_v2_test.go`

- [ ] **Step 1: Write cache tests**

Create `account/cache_v2_test.go`:

```go
package account

import (
	"testing"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestV2CacheStoresInstrument(t *testing.T) {
	cache := NewV2Cache()
	inst := model.Instrument{
		ID:        model.MustInstrumentID("BTC-USDT-PERP.BINANCE"),
		RawSymbol: "BTCUSDT",
		Type:      model.InstrumentTypeCryptoPerp,
		Base:      model.BTC,
		Quote:     model.USDT,
		Settle:    model.USDT,
		PriceStep: decimal.RequireFromString("0.1"),
		SizeStep:  decimal.RequireFromString("0.001"),
	}
	require.NoError(t, cache.PutInstrument(inst))
	got, ok := cache.Instrument(inst.ID)
	require.True(t, ok)
	require.Equal(t, inst.ID, got.ID)
}

func TestV2CacheAppliesAccountStateReplacement(t *testing.T) {
	cache := NewV2Cache()
	total := model.Money{Amount: decimal.NewFromInt(10), Currency: model.USDT}
	free := model.Money{Amount: decimal.NewFromInt(8), Currency: model.USDT}
	bal, err := model.BalanceFromTotalAndFree(total, free)
	require.NoError(t, err)

	state := model.AccountState{
		AccountID: "acct",
		Venue:     model.VenueBinance,
		Type:      model.AccountTypeMargin,
		Balances:  []model.AccountBalance{bal},
	}
	require.NoError(t, cache.ApplyAccountState(state))

	got, ok := cache.AccountState(model.VenueBinance, "acct")
	require.True(t, ok)
	require.Len(t, got.Balances, 1)
	require.True(t, got.Balances[0].Free.Amount.Equal(decimal.NewFromInt(8)))
}
```

- [ ] **Step 2: Implement cache**

Create `account/cache_v2.go`:

```go
package account

import (
	"sync"

	"github.com/QuantProcessing/exchanges/model"
)

type V2Cache struct {
	mu          sync.RWMutex
	instruments map[model.InstrumentID]model.Instrument
	accounts    map[accountKey]model.AccountState
}

type accountKey struct {
	venue model.Venue
	id    model.AccountID
}

func NewV2Cache() *V2Cache {
	return &V2Cache{
		instruments: make(map[model.InstrumentID]model.Instrument),
		accounts:    make(map[accountKey]model.AccountState),
	}
}

func (c *V2Cache) PutInstrument(inst model.Instrument) error {
	if err := inst.Validate(); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.instruments[inst.ID] = inst
	return nil
}

func (c *V2Cache) Instrument(id model.InstrumentID) (model.Instrument, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	inst, ok := c.instruments[id]
	return inst, ok
}

func (c *V2Cache) ApplyAccountState(state model.AccountState) error {
	if state.Venue == "" || state.AccountID == "" {
		return model.ErrInvalidAccountState
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.accounts[accountKey{venue: state.Venue, id: state.AccountID}] = state
	return nil
}

func (c *V2Cache) AccountState(v model.Venue, id model.AccountID) (model.AccountState, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	state, ok := c.accounts[accountKey{venue: v, id: id}]
	return state, ok
}
```

- [ ] **Step 3: Run account cache tests**

Run:

```bash
go test ./account -run 'TestV2Cache' -count=1
```

Expected: PASS.

## Task 6: Add Binance V2 Symbol Normalizer and Instrument Provider

**Files:**

- Create: `adapter/binance/v2_symbol.go`
- Create: `adapter/binance/v2_symbol_test.go`
- Create: `adapter/binance/v2_instruments.go`
- Create: `adapter/binance/v2_instruments_test.go`

- [ ] **Step 1: Write symbol normalizer tests**

Create `adapter/binance/v2_symbol_test.go`:

```go
package binance

import (
	"testing"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/stretchr/testify/require"
)

func TestV2SymbolNormalizerSpot(t *testing.T) {
	n := v2SymbolNormalizer{}
	got, err := n.ToInstrumentID("BTCUSDT", venue.ProductHintSpot)
	require.NoError(t, err)
	require.Equal(t, model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"), got)
	raw, err := n.ToVenueSymbol(got)
	require.NoError(t, err)
	require.Equal(t, "BTCUSDT", raw)
}

func TestV2SymbolNormalizerPerp(t *testing.T) {
	n := v2SymbolNormalizer{}
	got, err := n.ToInstrumentID("BTCUSDT", venue.ProductHintPerp)
	require.NoError(t, err)
	require.Equal(t, model.MustInstrumentID("BTC-USDT-PERP.BINANCE"), got)
	raw, err := n.ToVenueSymbol(got)
	require.NoError(t, err)
	require.Equal(t, "BTCUSDT", raw)
}
```

- [ ] **Step 2: Implement symbol normalizer**

Create `adapter/binance/v2_symbol.go`:

```go
package binance

import (
	"fmt"
	"strings"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/venue"
)

type v2SymbolNormalizer struct{}

func (v2SymbolNormalizer) ToInstrumentID(raw string, hint venue.ProductHint) (model.InstrumentID, error) {
	raw = strings.ToUpper(strings.TrimSpace(raw))
	base, quote, ok := splitBinanceSymbol(raw)
	if !ok {
		return model.InstrumentID{}, fmt.Errorf("%w: %s", model.ErrInvalidInstrumentID, raw)
	}
	product := "SPOT"
	if hint == venue.ProductHintPerp {
		product = "PERP"
	}
	return model.ParseInstrumentID(base + "-" + quote + "-" + product + ".BINANCE")
}

func (v2SymbolNormalizer) ToVenueSymbol(id model.InstrumentID) (string, error) {
	if id.Venue != model.VenueBinance {
		return "", fmt.Errorf("%w: %s", model.ErrInvalidInstrumentID, id.String())
	}
	parts := strings.Split(id.Symbol, "-")
	if len(parts) < 3 {
		return "", fmt.Errorf("%w: %s", model.ErrInvalidInstrumentID, id.String())
	}
	return parts[0] + parts[1], nil
}

func splitBinanceSymbol(raw string) (base, quote string, ok bool) {
	for _, q := range []string{"USDT", "USDC", "BUSD", "FDUSD", "BTC", "ETH"} {
		if strings.HasSuffix(raw, q) && len(raw) > len(q) {
			return raw[:len(raw)-len(q)], q, true
		}
	}
	return "", "", false
}
```

- [ ] **Step 3: Add instrument provider tests**

Use SDK fixtures or live public endpoint mapping. The first test can build from
handcrafted SDK-like records to keep model mapping deterministic:

```go
func TestV2InstrumentProviderCachesSpotAndPerpSeparately(t *testing.T) {
	provider := newV2InstrumentProviderForTest([]v2InstrumentSeed{
		{RawSymbol: "BTCUSDT", Product: venue.ProductHintSpot, Base: "BTC", Quote: "USDT"},
		{RawSymbol: "BTCUSDT", Product: venue.ProductHintPerp, Base: "BTC", Quote: "USDT"},
	})
	require.NoError(t, provider.LoadAll(context.Background()))
	require.Len(t, provider.List(), 2)
	_, ok := provider.Get(model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"))
	require.True(t, ok)
	_, ok = provider.Get(model.MustInstrumentID("BTC-USDT-PERP.BINANCE"))
	require.True(t, ok)
}
```

- [ ] **Step 4: Implement instrument provider**

Create `adapter/binance/v2_instruments.go` with:

- a provider struct holding spot and perp SDK clients;
- a mutex-protected `map[model.InstrumentID]model.Instrument`;
- `LoadAll` that loads spot exchange info and USD-M exchange info;
- mapping functions from Binance filters to `PriceStep`, `SizeStep`,
  `MinQty`, and `MinNotional`;
- `Find`, `Get`, and `List`.

The implementation must return `model.ErrInstrumentNotLoaded` from `Load` when
an ID cannot be loaded.

- [ ] **Step 5: Run Binance V2 symbol and instrument tests**

Run:

```bash
go test ./adapter/binance -run 'TestV2(Symbol|Instrument)' -count=1
```

Expected: PASS.

## Task 7: Add Binance V2 Market Data Client

**Files:**

- Create: `adapter/binance/v2_market_data.go`
- Create: `adapter/binance/v2_market_data_test.go`

- [ ] **Step 1: Write market data tests with a fake provider**

Create tests that verify:

- `FetchTicker` rejects unloaded instruments with `model.ErrInstrumentNotLoaded`;
- `FetchOrderBook` uses the instrument provider to convert
  `BTC-USDT-PERP.BINANCE` to native `BTCUSDT`;
- spot and perp route to the correct SDK client.

- [ ] **Step 2: Implement market data client**

Create `adapter/binance/v2_market_data.go`:

- define `type v2MarketDataClient struct`;
- store `venue.InstrumentProvider`;
- store spot and perp SDK clients;
- implement `FetchTicker` and `FetchOrderBook` first;
- return `model.ErrNotSupported` for trades and bars until implemented;
- return `model.ErrNotSupported` for subscriptions until V2 stream handling is
  designed.

- [ ] **Step 3: Run market data tests**

Run:

```bash
go test ./adapter/binance -run TestV2MarketData -count=1
```

Expected: PASS.

## Task 8: Add Binance V2 Execution and Account State Translation

**Files:**

- Create: `adapter/binance/v2_execution.go`
- Create: `adapter/binance/v2_account_state.go`

- [ ] **Step 1: Define execution mapping tests**

Add tests in `adapter/binance` that verify:

- a V2 market order command maps to Binance spot request fields;
- a V2 market order command maps to Binance perp request fields;
- an unknown instrument returns `model.ErrInstrumentNotLoaded`;
- account balances map through `model.BalanceFromTotalAndFree`;
- cross-margin fields become account-wide `model.MarginBalance` entries.

- [ ] **Step 2: Implement initial execution client**

Create `adapter/binance/v2_execution.go`:

- `AccountID() model.AccountID`
- `Venue() model.Venue`
- `Connect(ctx)` and `Disconnect(ctx)` for private stream lifecycle;
- `SubmitOrder(ctx, cmd model.SubmitOrder) error`;
- `CancelOrder(ctx, cmd model.CancelOrder) error`;
- `QueryAccount(ctx) error`;
- `GenerateOrderStatusReports(ctx, q venue.OrderStatusQuery)`;
- `GenerateFillReports(ctx, q venue.FillQuery)`;
- `GeneratePositionStatusReports(ctx, q venue.PositionQuery)`;
- `Events() <-chan model.ExecutionEvent`.

For the first pass, `SubmitOrder` may emit accepted/rejected events from REST
responses and rely on report generation for full reconciliation.

- [ ] **Step 3: Implement account state mapper**

Create `adapter/binance/v2_account_state.go`:

- spot balances map to `AccountTypeCash`;
- USD-M futures balances map to `AccountTypeMargin`;
- total/free/locked invariants are enforced through `model` constructors;
- margin entries are account-wide unless Binance reports isolated
  per-position values tied to an instrument.

- [ ] **Step 4: Run execution mapping tests**

Run:

```bash
go test ./adapter/binance -run 'TestV2(Execution|AccountState)' -count=1
```

Expected: PASS.

## Task 9: Add V2 TradingAccount Startup and Reconciliation

**Files:**

- Create: `account/reconciler_v2.go`
- Create: `account/trading_account_v2.go`
- Create or modify: `account/account_runtime_test.go`

- [ ] **Step 1: Write startup readiness test**

Add a test with a fake `venue.ExecutionClient` that records calls. It must
verify startup order:

1. `QueryAccount`
2. `GenerateOrderStatusReports`
3. `GenerateFillReports`
4. `GeneratePositionStatusReports`
5. `Connect`
6. ready state true

- [ ] **Step 2: Implement V2 reconciler**

Create `account/reconciler_v2.go` with:

- `type Reconciler struct`;
- `Startup(ctx context.Context) error`;
- report application into `V2Cache`;
- stale/ready state updates.

- [ ] **Step 3: Implement V2 trading account facade**

Create `account/trading_account_v2.go` with:

- `NewTradingAccount(exec venue.ExecutionClient, cfg Config) *TradingAccount`;
- `Start(ctx context.Context) error`;
- `Stop(ctx context.Context) error`;
- `Submit(ctx context.Context, cmd model.SubmitOrder) (*OrderFlow, error)`;
- event loop that applies `model.ExecutionEvent` into cache and order flows.

- [ ] **Step 4: Run account lifecycle tests**

Run:

```bash
go test ./account -run 'TestV2|TestTradingAccount' -count=1
```

Expected: PASS.

## Task 10: Add Contract and Certification Suites

**Files:**

- Create: `testsuite/v2_model_suite.go`
- Create: `testsuite/v2_venue_suite.go`
- Create: `testsuite/v2_lifecycle_suite.go`
- Modify: `adapter/binance/register.go`

- [ ] **Step 1: Add V2 model suite**

Create `testsuite/v2_model_suite.go` with reusable assertions for:

- instrument ID parsing;
- instrument validation;
- balance invariant;
- account state replacement.

- [ ] **Step 2: Add V2 venue suite**

Create `testsuite/v2_venue_suite.go` with tests for:

- provider `LoadAll`;
- `Get` for known IDs;
- ticker fetch by instrument ID;
- orderbook fetch by instrument ID;
- unsupported methods returning `model.ErrNotSupported`.

- [ ] **Step 3: Add V2 lifecycle suite**

Create `testsuite/v2_lifecycle_suite.go` with tests for:

- `QueryAccount`;
- report generation;
- startup reconciliation;
- stream readiness;
- order flow terminal state with fake execution client.

- [ ] **Step 4: Register Binance declared V2 capabilities**

Modify `adapter/binance/register.go` only after the Binance V2 pilot passes
local contract tests. Declared capabilities must distinguish:

- market data support;
- basic execution support;
- account state support;
- lifecycle certified support.

Lifecycle certified support must remain false until private streams and
reconciliation have passed the V2 lifecycle suite.

- [ ] **Step 5: Run focused certification tests**

Run:

```bash
go test ./model ./venue ./account ./adapter/binance ./testsuite -count=1
```

Expected: PASS.

## Task 11: Full Verification

**Files:**

- No new files.

- [ ] **Step 1: Run package tests for changed areas**

Run:

```bash
go test ./model ./venue ./account ./adapter/binance ./testsuite -count=1
```

Expected: PASS.

- [ ] **Step 2: Run repository tests**

Run:

```bash
go test ./...
```

Expected: PASS or documented pre-existing failures unrelated to the V2 slice.

- [ ] **Step 3: Run race check for account package**

Run:

```bash
go test -race ./account -run 'TestV2|TestTradingAccount' -count=1
```

Expected: PASS.

## Commit Plan

Use small commits in this order:

1. `model`: identifiers, money, instruments, account state.
2. `venue`: contracts, subscriptions, capabilities, registry.
3. `account`: V2 cache and reconciliation foundation.
4. `adapter/binance`: symbol normalizer and instrument provider.
5. `adapter/binance`: market data client.
6. `adapter/binance`: execution and account state translation.
7. `testsuite`: V2 contract/certification suites.

Each commit must use the repository Lore Commit Protocol.

## Completion Criteria

The foundation slice is complete when:

- `model` has no imports from repository packages;
- `venue` imports only `model` plus standard library dependencies;
- `account` imports `model` and `venue`, not `sdk` or `adapter`;
- Binance V2 can load spot/perp instruments and distinguish spot from perp by
  `model.InstrumentID`;
- Binance V2 market data fetches ticker/orderbook by instrument ID;
- Binance V2 account state maps balances and margins with invariants;
- V2 TradingAccount startup performs snapshot, reports, private stream connect,
  and readiness in order;
- focused V2 tests pass.
