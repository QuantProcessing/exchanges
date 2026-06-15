package testsuite

import (
	"fmt"

	"github.com/QuantProcessing/exchanges/venue"
)

type RequiredCases struct {
	Suite string
	IDs   []string
}

type ScoreboardSummary struct {
	Suites         int
	Cases          int
	Passed         int
	Failed         int
	Skipped        int
	Required       int
	RequiredPassed int
}

type ParityScoreboard struct {
	reports []ContractReport
}

func NautilusParityRequirements() []RequiredCases {
	return []RequiredCases{
		{Suite: "data", IDs: requiredDataCaseIDs(venue.DeclaredCapabilities{})},
		{Suite: "data-engine", IDs: []string{"TC-DE01", "TC-DE02", "TC-DE03", "TC-DE04", "TC-DE05", "TC-DE06", "TC-DE07"}},
		{Suite: "execution", IDs: requiredExecutionCaseIDs(venue.DeclaredCapabilities{}, true)},
		{Suite: "execution-engine", IDs: []string{"TC-EXENG01", "TC-EXENG02", "TC-EXENG03", "TC-EXENG04", "TC-EXENG05", "TC-EXENG06", "TC-EXENG07", "TC-EXENG08", "TC-EXENG09", "TC-EXENG10", "TC-EXENG11", "TC-EXENG12", "TC-EXENG13", "TC-EXENG14", "TC-EXENG15", "TC-EXENG16", "TC-EXENG17", "TC-EXENG18", "TC-EXENG19", "TC-EXENG20", "TC-EXENG21", "TC-EXENG22", "TC-EXENG23", "TC-EXENG24", "TC-EXENG25", "TC-EXENG26", "TC-EXENG27", "TC-EXENG28", "TC-EXENG29", "TC-EXENG30", "TC-EXENG31", "TC-EXENG32", "TC-EXENG33"}},
		{Suite: "reconciliation", IDs: []string{"TC-REC01", "TC-REC02", "TC-REC03", "TC-REC04", "TC-REC05", "TC-REC06", "TC-REC07", "TC-REC08", "TC-REC09"}},
		{Suite: "strategy", IDs: []string{"TC-S01", "TC-S02", "TC-S03", "TC-S04", "TC-S05", "TC-S06", "TC-S07", "TC-S08", "TC-S09"}},
		{Suite: "adapter", IDs: []string{"TC-A01", "TC-A02", "TC-A03", "TC-A04", "TC-A05", "TC-A06", "TC-A07"}},
		{Suite: "lifecycle", IDs: []string{"TC-L01", "TC-L02", "TC-L03", "TC-L04"}},
		{Suite: "backtest", IDs: []string{"TC-B01", "TC-B02", "TC-B03", "TC-B04", "TC-B05", "TC-B06", "TC-B07", "TC-B08", "TC-B09", "TC-B10", "TC-B11", "TC-B12", "TC-B13", "TC-B14", "TC-B15", "TC-B16"}},
		{Suite: "portfolio", IDs: []string{"TC-P01", "TC-P02", "TC-P03", "TC-P04", "TC-P05", "TC-P06", "TC-P07", "TC-P08", "TC-P09", "TC-P10", "TC-P11", "TC-P12", "TC-P13", "TC-P14", "TC-P15", "TC-P16"}},
		{Suite: "risk", IDs: []string{"TC-R01", "TC-R02", "TC-R03", "TC-R04", "TC-R05", "TC-R06", "TC-R07", "TC-R08", "TC-R09", "TC-R10", "TC-R11", "TC-R12", "TC-R13"}},
		{Suite: "live", IDs: []string{"TC-LIVE01", "TC-LIVE02", "TC-LIVE03", "TC-LIVE04", "TC-LIVE05", "TC-LIVE06", "TC-LIVE07", "TC-LIVE08", "TC-LIVE09"}},
	}
}

func NewParityScoreboard(reports ...ContractReport) ParityScoreboard {
	board := ParityScoreboard{}
	for _, report := range reports {
		board.Add(report)
	}
	return board
}

func (b *ParityScoreboard) Add(report ContractReport) {
	b.reports = append(b.reports, report)
}

func (b ParityScoreboard) Reports() []ContractReport {
	return append([]ContractReport(nil), b.reports...)
}

func (b ParityScoreboard) Suite(name string) (ContractReport, bool) {
	for _, report := range b.reports {
		if report.Suite == name {
			return report, true
		}
	}
	return ContractReport{}, false
}

func (b ParityScoreboard) Passed(required ...RequiredCases) bool {
	if len(b.reports) == 0 {
		return false
	}
	for _, report := range b.reports {
		if !report.Passed() {
			return false
		}
	}
	return len(b.MissingRequired(required...)) == 0
}

func (b ParityScoreboard) MissingRequired(required ...RequiredCases) []string {
	missing := make([]string, 0)
	for _, req := range required {
		report, ok := b.Suite(req.Suite)
		if !ok {
			for _, id := range req.IDs {
				missing = append(missing, fmt.Sprintf("%s:%s", req.Suite, id))
			}
			continue
		}
		results := make(map[string]CaseStatus, len(report.Cases))
		for _, result := range report.Cases {
			results[result.ID] = result.Status
		}
		for _, id := range req.IDs {
			if results[id] != CasePassed {
				missing = append(missing, fmt.Sprintf("%s:%s", req.Suite, id))
			}
		}
	}
	return missing
}

func (b ParityScoreboard) Summary(required ...RequiredCases) ScoreboardSummary {
	summary := ScoreboardSummary{Suites: len(b.reports)}
	for _, report := range b.reports {
		for _, result := range report.Cases {
			summary.Cases++
			switch result.Status {
			case CasePassed:
				summary.Passed++
			case CaseFailed:
				summary.Failed++
			case CaseSkipped:
				summary.Skipped++
			}
		}
	}
	for _, req := range required {
		report, ok := b.Suite(req.Suite)
		summary.Required += len(req.IDs)
		if !ok {
			continue
		}
		results := make(map[string]CaseStatus, len(report.Cases))
		for _, result := range report.Cases {
			results[result.ID] = result.Status
		}
		for _, id := range req.IDs {
			if results[id] == CasePassed {
				summary.RequiredPassed++
			}
		}
	}
	return summary
}
