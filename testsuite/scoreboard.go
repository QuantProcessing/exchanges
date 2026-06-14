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
		{Suite: "execution", IDs: requiredExecutionCaseIDs(venue.DeclaredCapabilities{}, true)},
		{Suite: "adapter", IDs: []string{"TC-A01", "TC-A02", "TC-A03", "TC-A04", "TC-A05", "TC-A06", "TC-A07"}},
		{Suite: "lifecycle", IDs: []string{"TC-L01", "TC-L02", "TC-L03", "TC-L04"}},
		{Suite: "backtest", IDs: []string{"TC-B01", "TC-B02", "TC-B03", "TC-B04", "TC-B05"}},
		{Suite: "portfolio", IDs: []string{"TC-P01", "TC-P02", "TC-P03", "TC-P04", "TC-P05", "TC-P06", "TC-P07"}},
		{Suite: "risk", IDs: []string{"TC-R01", "TC-R02", "TC-R03", "TC-R04", "TC-R05", "TC-R06", "TC-R07", "TC-R08", "TC-R09", "TC-R10", "TC-R11", "TC-R12", "TC-R13"}},
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
