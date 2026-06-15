package testsuite

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExecutionEngineTesterReportsParityCases(t *testing.T) {
	report := NewExecutionEngineTester(ExecutionEngineTesterConfig{}).Run(context.Background(), t)

	require.Equal(t, "execution-engine", report.Suite)
	require.True(t, report.AllPassed(), "execution engine tester failed: %#v", report.Cases)
	requireCasePassed(t, report, "TC-EXENG01", "Engine routes submit commands and caches reports")
	requireCasePassed(t, report, "TC-EXENG02", "Engine routes cancel modify and query")
	requireCasePassed(t, report, "TC-EXENG03", "Engine rejects missing account clients")
	requireCasePassed(t, report, "TC-EXENG04", "Manager caches and pops submit commands")
	requireCasePassed(t, report, "TC-EXENG05", "Manager rejects closed-order regressions")
	requireCasePassed(t, report, "TC-EXENG06", "Manager deduplicates fills and rejects overfills")
	requireCasePassed(t, report, "TC-EXENG14", "Manager defers fills until order reports arrive")
	requireCasePassed(t, report, "TC-EXENG07", "Manager determines netting and hedging position IDs")
	requireCasePassed(t, report, "TC-EXENG08", "Manager releases OTO children and cancels OCO siblings")
	requireCasePassed(t, report, "TC-EXENG13", "Manager reduces OUO siblings on partial fills")
	requireCasePassed(t, report, "TC-EXENG09", "Platform routes order commands through execution engine")
	requireCasePassed(t, report, "TC-EXENG10", "Backtest routes order commands through execution engine")
	requireCasePassed(t, report, "TC-EXENG11", "Engine routes composite commands and account queries")
	requireCasePassed(t, report, "TC-EXENG12", "Engine generates execution reports and mass status")
	requireCasePassed(t, report, "TC-EXENG15", "Engine claims external order reports by instrument")
	requireCasePassed(t, report, "TC-EXENG16", "Engine snapshots and purges execution state")
	requireCasePassed(t, report, "TC-EXENG17", "Manager snapshots durable order-list state")
	requireCasePassed(t, report, "TC-EXENG18", "Manager applies leg fills without parent order reports")
	requireCasePassed(t, report, "TC-EXENG19", "Engine routes execution-algorithm orders before venue submission")
	requireCasePassed(t, report, "TC-EXENG20", "Engine emulates trigger orders until market data releases them")
	requireCasePassed(t, report, "TC-EXENG21", "Engine publishes emulated triggered and released lifecycle events")
	requireCasePassed(t, report, "TC-EXENG22", "Platform feeds data-engine market events into execution emulator")
	requireCasePassed(t, report, "TC-EXENG23", "Engine emulates trailing stop market trigger updates")
	requireCasePassed(t, report, "TC-EXENG24", "Engine transforms released emulated orders before venue submission")
	requireCasePassed(t, report, "TC-EXENG25", "Engine emulates trailing stop offset types")
	requireCasePassed(t, report, "TC-EXENG26", "Engine emulates trailing stop limit releases")
	requireCasePassed(t, report, "TC-EXENG27", "Engine emulates orders from trigger instruments")
	requireCasePassed(t, report, "TC-EXENG28", "Engine emulates bid ask triggers from order books")
	requireCasePassed(t, report, "TC-EXENG29", "Engine uses synthetic trigger instruments for emulation")
	requireCasePassed(t, report, "TC-EXENG30", "Engine initial matches emulated orders from cached market data")
	requireCasePassed(t, report, "TC-EXENG31", "Engine cancels emulated orders locally")
	requireCasePassed(t, report, "TC-EXENG32", "Engine modifies emulated orders locally and rematches")
	requireCasePassed(t, report, "TC-EXENG33", "Engine cancel-all cancels emulated orders locally")
	requireCasePassed(t, report, "TC-EXENG34", "Engine matching core releases emulated limit orders only when marketable")
	requireCasePassed(t, report, "TC-EXENG35", "Engine health tracks kernel lifecycle state")
}
