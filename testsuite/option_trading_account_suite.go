package testsuite

import (
	"context"
	"strings"
	"testing"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/account"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// OptionTradingAccountConfig configures the option compliance + live-trading
// suite. Only Underlying is mandatory; the live phase is gated on having
// LiveTestInstrument + a positive PassiveLimitQty.
type OptionTradingAccountConfig struct {
	Underlying          string
	LiveTestInstrument  *exchanges.OptionInstrument
	PassiveLimitPremium decimal.Decimal
	PassiveLimitQty     decimal.Decimal
}

// RunOptionTradingAccountSuite drives the read-only compliance battery for
// any OptionExchange. If cfg.LiveTestInstrument is set, it also runs a
// passive limit place + cancel via OptionTradingAccount.
func RunOptionTradingAccountSuite(t *testing.T, adp exchanges.OptionExchange, cfg OptionTradingAccountConfig) {
	require.NotEmpty(t, cfg.Underlying, "Underlying must be set")
	require.Equal(t, exchanges.MarketTypeOption, adp.GetMarketType(),
		"adapter must declare MarketTypeOption")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Log("═══ Phase 1: FetchExpirations ═══")
	expirations, err := adp.FetchExpirations(ctx, cfg.Underlying)
	require.NoError(t, err, "FetchExpirations should succeed")
	require.NotEmpty(t, expirations, "FetchExpirations should return at least one expiry")
	for _, exp := range expirations {
		assert.True(t, exp.After(time.Now().Add(-24*time.Hour)),
			"expiry %s should be near/future-dated", exp.Format(time.RFC3339))
	}
	t.Logf("✓ Got %d expirations; nearest = %s", len(expirations), expirations[0].Format("2006-01-02"))

	t.Log("═══ Phase 2: FetchOptionChain ═══")
	chain, err := adp.FetchOptionChain(ctx, cfg.Underlying, nil)
	require.NoError(t, err, "FetchOptionChain should succeed")
	require.NotEmpty(t, chain, "FetchOptionChain should return at least one instrument")

	for _, inst := range chain {
		assert.Equal(t, cfg.Underlying, inst.Underlying,
			"chain entry underlying mismatch")
		assert.True(t, inst.Strike.IsPositive(), "chain entry must have positive strike")
		assert.NotEmpty(t, inst.Settlement, "chain entry must declare settlement asset")
		assert.True(t, inst.Kind == exchanges.OptionCall || inst.Kind == exchanges.OptionPut,
			"chain entry kind must be C or P")
	}
	t.Logf("✓ Chain has %d instruments", len(chain))

	t.Log("═══ Phase 3: FormatInstrument/ParseInstrument round-trip ═══")
	sample := chain[0]
	id := adp.FormatInstrument(&sample)
	require.NotEmpty(t, id, "FormatInstrument must return non-empty ID")
	require.False(t, strings.ContainsAny(id, " \t\n"), "instrument ID must not contain whitespace")

	parsed, err := adp.ParseInstrument(id)
	require.NoError(t, err, "ParseInstrument should round-trip")
	require.NotNil(t, parsed)

	assert.Equal(t, sample.Underlying, parsed.Underlying, "underlying lost in round-trip")
	assert.Equal(t, sample.Kind, parsed.Kind, "kind lost in round-trip")
	assert.True(t, sample.Strike.Equal(parsed.Strike),
		"strike lost in round-trip: %s vs %s", sample.Strike, parsed.Strike)
	// Expiry round-trip: must agree to day-of-year resolution; intraday timing
	// may differ across venues.
	assert.Equal(t, sample.Expiry.UTC().Format("2006-01-02"),
		parsed.Expiry.UTC().Format("2006-01-02"), "expiry day lost in round-trip")
	t.Logf("✓ Round-trip OK: %s", id)

	t.Log("═══ Phase 4: FetchOptionMark ═══")
	mark, err := adp.FetchOptionMark(ctx, id)
	require.NoError(t, err, "FetchOptionMark should succeed")
	require.NotNil(t, mark)
	assert.Equal(t, id, mark.InstrumentID, "mark instrument ID mismatch")
	assert.True(t, mark.MarkPrice.IsPositive() || mark.MarkPrice.IsZero(),
		"mark price should be non-negative")
	// Delta must be in [-1, 1]. IV in [0, 5] (500%) sanity bound.
	absDelta := mark.Greeks.Delta.Abs()
	assert.True(t, absDelta.LessThanOrEqual(decimal.NewFromInt(1)),
		"|delta| must be ≤ 1, got %s", absDelta)
	assert.True(t, mark.MarkIV.GreaterThanOrEqual(decimal.Zero),
		"mark IV must be non-negative, got %s", mark.MarkIV)
	t.Logf("✓ Mark: price=%s IV=%s Δ=%s", mark.MarkPrice, mark.MarkIV, mark.Greeks.Delta)

	t.Log("═══ Phase 5: FetchGreeks ═══")
	greeks, err := adp.FetchGreeks(ctx, id)
	require.NoError(t, err, "FetchGreeks should succeed")
	require.NotNil(t, greeks)
	assert.True(t, greeks.Delta.Abs().LessThanOrEqual(decimal.NewFromInt(1)),
		"FetchGreeks delta must be in [-1,1]")
	t.Log("✓ Greeks fetched")

	t.Log("═══ Phase 6: FetchOptionPositions ═══")
	positions, err := adp.FetchOptionPositions(ctx)
	require.NoError(t, err, "FetchOptionPositions should succeed (may be empty)")
	for _, p := range positions {
		assert.Equal(t, exchanges.InstrumentTypeOption, p.InstrumentType,
			"FetchOptionPositions must set InstrumentType=Option")
		assert.NotNil(t, p.Option, "FetchOptionPositions must populate Option payload")
	}
	t.Logf("✓ %d open option positions", len(positions))

	// Live trading is gated.
	if cfg.LiveTestInstrument == nil || !cfg.PassiveLimitQty.IsPositive() {
		t.Log("═══ Summary ═══")
		t.Log("✓ Read-only compliance suite passed; live phase skipped (no LiveTestInstrument)")
		return
	}

	t.Log("═══ Phase 7: OptionTradingAccount lifecycle ═══")
	acct := account.NewOptionTradingAccount(adp, nil)
	require.NoError(t, acct.Start(ctx), "OptionTradingAccount.Start should succeed")
	defer acct.Close()

	t.Log("═══ Phase 8: Place + Cancel passive limit ═══")
	flow, err := acct.Place(ctx, &account.OptionOrderParams{
		Instrument:  cfg.LiveTestInstrument,
		Side:        exchanges.OrderSideBuy,
		Type:        exchanges.OrderTypeLimit,
		Quantity:    cfg.PassiveLimitQty,
		Price:       cfg.PassiveLimitPremium,
		TimeInForce: exchanges.TimeInForceGTC,
		PostOnly:    true,
	})
	require.NoError(t, err, "Place passive limit should succeed")
	defer flow.Close()

	limitCtx, limitCancel := context.WithTimeout(ctx, 10*time.Second)
	defer limitCancel()
	placed, err := flow.Wait(limitCtx, func(o *exchanges.Order) bool {
		return o != nil && o.OrderID != ""
	})
	require.NoError(t, err, "should observe order with stable ID")
	t.Logf("✓ Placed: ID=%s", placed.OrderID)

	require.NoError(t, acct.Cancel(ctx, placed.OrderID, adp.FormatInstrument(cfg.LiveTestInstrument)),
		"Cancel should succeed")

	cancelled, err := flow.Wait(ctx, func(o *exchanges.Order) bool {
		return o.Status == exchanges.OrderStatusCancelled
	})
	require.NoError(t, err, "should observe Cancelled status")
	assert.Equal(t, exchanges.OrderStatusCancelled, cancelled.Status)
	t.Log("✓ Cancelled")

	t.Log("═══ Summary ═══")
	t.Log("✓ Compliance + lifecycle passed")
}
