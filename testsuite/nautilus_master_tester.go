package testsuite

import (
	"fmt"
	"testing"

	"github.com/QuantProcessing/exchanges/venue"
)

type NautilusMasterRequirement struct {
	Suite        string
	Domain       string
	Points       int
	GoPackage    string
	ReportSource string
	CaseIDs      []string
	ScenarioIDs  []string
}

type NautilusMasterTester struct {
	requirements []NautilusMasterRequirement
}

func NautilusMasterRequirements() []NautilusMasterRequirement {
	return []NautilusMasterRequirement{
		{
			Suite:        "model",
			Domain:       "Domain model and identifiers",
			Points:       90,
			GoPackage:    "model",
			ReportSource: "testsuite/model_tester.go",
			CaseIDs: []string{
				"TC-M01", "TC-M02", "TC-M03", "TC-M04", "TC-M05", "TC-M06",
			},
			ScenarioIDs: []string{"A", "B", "C", "D", "E"},
		},
		{
			Suite:        "cache",
			Domain:       "Cache and state indexes",
			Points:       80,
			GoPackage:    "cache",
			ReportSource: "testsuite/cache_tester.go",
			CaseIDs: []string{
				"TC-C01", "TC-C02", "TC-C03", "TC-C04", "TC-C05", "TC-C06", "TC-C07", "TC-C08",
			},
			ScenarioIDs: []string{"B", "C"},
		},
		{
			Suite:        "command",
			Domain:       "Command envelope and message bus",
			Points:       70,
			GoPackage:    "model,bus,kernel",
			ReportSource: "testsuite/command_tester.go",
			CaseIDs: []string{
				"TC-CMD01", "TC-CMD02", "TC-CMD03", "TC-CMD04", "TC-CMD05",
			},
			ScenarioIDs: []string{"A", "D"},
		},
		{
			Suite:        "strategy",
			Domain:       "Strategy runtime and UX",
			Points:       70,
			GoPackage:    "strategy",
			ReportSource: "testsuite/strategy_tester.go",
			CaseIDs: []string{
				"TC-S01", "TC-S02", "TC-S03", "TC-S04", "TC-S05", "TC-S06", "TC-S07", "TC-S08", "TC-S09",
			},
			ScenarioIDs: []string{"A", "D"},
		},
		{
			Suite:        "data",
			Domain:       "Data engine and catalog",
			Points:       80,
			GoPackage:    "data,venue,backtest,live",
			ReportSource: "testsuite/data_tester.go;testsuite/data_engine_tester.go",
			CaseIDs: append(requiredDataCaseIDsDefault(),
				"TC-D20", "TC-D21", "TC-D22", "TC-D23",
				"TC-DE01", "TC-DE02", "TC-DE03", "TC-DE04", "TC-DE05", "TC-DE06", "TC-DE07",
			),
			ScenarioIDs: []string{"A"},
		},
		{
			Suite:        "execution",
			Domain:       "Execution engine and lifecycle",
			Points:       130,
			GoPackage:    "execution,account",
			ReportSource: "testsuite/exec_tester.go;testsuite/lifecycle_tester.go;testsuite/execution_engine_tester.go",
			CaseIDs: append(requiredExecutionCaseIDsDefault(),
				"TC-L01", "TC-L02", "TC-L03", "TC-L04",
				"TC-E90", "TC-E91", "TC-E92", "TC-E93", "TC-E94",
				"TC-EXENG01", "TC-EXENG02", "TC-EXENG03", "TC-EXENG04",
				"TC-EXENG05", "TC-EXENG06", "TC-EXENG07", "TC-EXENG08", "TC-EXENG09", "TC-EXENG10", "TC-EXENG11", "TC-EXENG12", "TC-EXENG13", "TC-EXENG14", "TC-EXENG15", "TC-EXENG16", "TC-EXENG17", "TC-EXENG18", "TC-EXENG19", "TC-EXENG20", "TC-EXENG21", "TC-EXENG22", "TC-EXENG23", "TC-EXENG24", "TC-EXENG25", "TC-EXENG26", "TC-EXENG27", "TC-EXENG28", "TC-EXENG29", "TC-EXENG30", "TC-EXENG31", "TC-EXENG32", "TC-EXENG33", "TC-EXENG34", "TC-EXENG35",
			),
			ScenarioIDs: []string{"A", "B", "C", "D"},
		},
		{
			Suite:        "reconciliation",
			Domain:       "Reconciliation",
			Points:       90,
			GoPackage:    "execution,account,live",
			ReportSource: "testsuite/reconciliation_tester.go",
			CaseIDs: []string{
				"TC-REC01", "TC-REC02", "TC-REC03", "TC-REC04", "TC-REC05",
				"TC-REC06", "TC-REC07", "TC-REC08", "TC-REC09",
			},
			ScenarioIDs: []string{"B", "C", "E"},
		},
		{
			Suite:        "risk",
			Domain:       "Risk engine",
			Points:       70,
			GoPackage:    "risk",
			ReportSource: "testsuite/risk_tester.go",
			CaseIDs: append(requiredRiskCaseIDsDefault(),
				"TC-R20", "TC-R21", "TC-R22", "TC-R23",
			),
			ScenarioIDs: []string{"D"},
		},
		{
			Suite:        "portfolio",
			Domain:       "Portfolio/accounting",
			Points:       90,
			GoPackage:    "portfolio",
			ReportSource: "testsuite/portfolio_tester.go",
			CaseIDs: append(requiredPortfolioCaseIDsDefault(),
				"TC-P20", "TC-P21", "TC-P22", "TC-P23",
			),
			ScenarioIDs: []string{"A", "B", "C", "D"},
		},
		{
			Suite:        "backtest",
			Domain:       "Backtest engine",
			Points:       80,
			GoPackage:    "backtest",
			ReportSource: "testsuite/backtest_tester.go",
			CaseIDs: append(requiredBacktestCaseIDsDefault(),
				"TC-B20", "TC-B21", "TC-B22", "TC-B23", "TC-B24",
			),
			ScenarioIDs: []string{"A", "D"},
		},
		{
			Suite:        "live",
			Domain:       "Live node/runtime",
			Points:       60,
			GoPackage:    "live,platform",
			ReportSource: "testsuite/live_node_tester.go",
			CaseIDs: []string{
				"TC-LIVE01", "TC-LIVE02", "TC-LIVE03", "TC-LIVE04", "TC-LIVE05", "TC-LIVE06", "TC-LIVE07", "TC-LIVE08", "TC-LIVE09",
			},
			ScenarioIDs: []string{"B", "C", "E"},
		},
		{
			Suite:        "adapter",
			Domain:       "Adapters and SDK parity",
			Points:       70,
			GoPackage:    "adapter/*,sdk/*,venue",
			ReportSource: "testsuite/contracts.go",
			CaseIDs: append(requiredAdapterCaseIDsDefault(),
				"TC-A20", "TC-A21", "TC-A22", "TC-A23",
			),
			ScenarioIDs: []string{"E"},
		},
		{
			Suite:        "documentation",
			Domain:       "Documentation and examples",
			Points:       20,
			GoPackage:    "docs,examples",
			ReportSource: "testsuite/nautilus_master_tester.go",
			CaseIDs: []string{
				"TC-DOC01", "TC-DOC02", "TC-DOC03",
			},
			ScenarioIDs: []string{"A", "B", "C", "D", "E"},
		},
	}
}

func NautilusMasterRequiredCases() []RequiredCases {
	requirements := NautilusMasterRequirements()
	cases := make([]RequiredCases, 0, len(requirements))
	for _, req := range requirements {
		cases = append(cases, RequiredCases{Suite: req.Suite, IDs: append([]string(nil), req.CaseIDs...)})
	}
	return cases
}

func NautilusMasterGate(reports ...ContractReport) error {
	return NautilusMasterGateWithRequirements(NautilusMasterRequiredCases(), reports...)
}

func NautilusMasterGateWithRequirements(required []RequiredCases, reports ...ContractReport) error {
	board := NewParityScoreboard(reports...)
	if len(reports) == 0 {
		return fmt.Errorf("nautilus master gate has no reports")
	}
	missing := board.MissingRequired(required...)
	if len(missing) > 0 {
		return fmt.Errorf("nautilus master gate missing, failed, or skipped required cases: %v", missing)
	}
	return nil
}

func NewNautilusMasterTester() NautilusMasterTester {
	return NautilusMasterTester{requirements: NautilusMasterRequirements()}
}

func (n NautilusMasterTester) Run(t *testing.T) ContractReport {
	t.Helper()
	requirements := n.requirements
	if len(requirements) == 0 {
		requirements = NautilusMasterRequirements()
	}
	return runContractCases(t, "nautilus-master", []contractCase{
		{id: "TC-MASTER01", name: "Master scorecard totals 1000 points", run: func() error {
			if total := totalMasterPoints(requirements); total != 1000 {
				return fmt.Errorf("expected master scorecard to total 1000 points, got %d", total)
			}
			return nil
		}},
		{id: "TC-MASTER02", name: "Required suites define case IDs", run: func() error {
			for _, req := range requirements {
				if req.Suite == "" {
					return fmt.Errorf("requirement has empty suite")
				}
				if len(req.CaseIDs) == 0 {
					return fmt.Errorf("suite %s has no required case IDs", req.Suite)
				}
			}
			return nil
		}},
		{id: "TC-MASTER03", name: "Required suites define report sources", run: func() error {
			for _, req := range requirements {
				if req.ReportSource == "" {
					return fmt.Errorf("suite %s has no report source", req.Suite)
				}
			}
			return nil
		}},
		{id: "TC-MASTER04", name: "Golden scenarios are mapped", run: func() error {
			covered := make(map[string]bool)
			for _, req := range requirements {
				for _, id := range req.ScenarioIDs {
					covered[id] = true
				}
			}
			for _, id := range []string{"A", "B", "C", "D", "E"} {
				if !covered[id] {
					return fmt.Errorf("golden scenario %s is not mapped", id)
				}
			}
			return nil
		}},
		{id: "TC-MASTER05", name: "Required case IDs are unique within suites", run: func() error {
			for _, req := range requirements {
				seen := make(map[string]bool, len(req.CaseIDs))
				for _, id := range req.CaseIDs {
					if id == "" {
						return fmt.Errorf("suite %s has empty case ID", req.Suite)
					}
					if seen[id] {
						return fmt.Errorf("suite %s has duplicate case ID %s", req.Suite, id)
					}
					seen[id] = true
				}
			}
			return nil
		}},
		{id: "TC-MASTER06", name: "Master gate rejects missing failed or skipped required cases", run: func() error {
			required := []RequiredCases{{Suite: "sample", IDs: []string{"TC-SAMPLE01", "TC-SAMPLE02", "TC-SAMPLE03"}}}
			failing := ContractReport{Suite: "sample", Cases: []CaseResult{
				{ID: "TC-SAMPLE01", Status: CasePassed},
				{ID: "TC-SAMPLE02", Status: CaseSkipped, Error: "not supported"},
			}}
			if err := NautilusMasterGateWithRequirements(required, failing); err == nil {
				return fmt.Errorf("expected master gate to reject skipped and missing required cases")
			}
			passing := ContractReport{Suite: "sample", Cases: []CaseResult{
				{ID: "TC-SAMPLE01", Status: CasePassed},
				{ID: "TC-SAMPLE02", Status: CasePassed},
				{ID: "TC-SAMPLE03", Status: CasePassed},
			}}
			return NautilusMasterGateWithRequirements(required, passing)
		}},
	})
}

func totalMasterPoints(requirements []NautilusMasterRequirement) int {
	total := 0
	for _, req := range requirements {
		total += req.Points
	}
	return total
}

func requiredDataCaseIDsDefault() []string {
	return requiredDataCaseIDs(venue.DeclaredCapabilities{})
}

func requiredExecutionCaseIDsDefault() []string {
	return requiredExecutionCaseIDs(venue.DeclaredCapabilities{}, true)
}

func requiredAdapterCaseIDsDefault() []string {
	return []string{"TC-A01", "TC-A02", "TC-A03", "TC-A04", "TC-A05", "TC-A06", "TC-A07"}
}

func requiredBacktestCaseIDsDefault() []string {
	return []string{"TC-B01", "TC-B02", "TC-B03", "TC-B04", "TC-B05", "TC-B06", "TC-B07", "TC-B08", "TC-B09", "TC-B10", "TC-B11", "TC-B12", "TC-B13", "TC-B14", "TC-B15", "TC-B16"}
}

func requiredPortfolioCaseIDsDefault() []string {
	return []string{"TC-P01", "TC-P02", "TC-P03", "TC-P04", "TC-P05", "TC-P06", "TC-P07", "TC-P08", "TC-P09", "TC-P10", "TC-P11", "TC-P12", "TC-P13", "TC-P14", "TC-P15", "TC-P16"}
}

func requiredRiskCaseIDsDefault() []string {
	return []string{
		"TC-R01", "TC-R02", "TC-R03", "TC-R04", "TC-R05", "TC-R06", "TC-R07",
		"TC-R08", "TC-R09", "TC-R10", "TC-R11", "TC-R12", "TC-R13",
	}
}
