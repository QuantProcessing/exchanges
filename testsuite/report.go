package testsuite

import "testing"

type CaseStatus string

const (
	CasePassed  CaseStatus = "passed"
	CaseFailed  CaseStatus = "failed"
	CaseSkipped CaseStatus = "skipped"
)

type CaseResult struct {
	ID     string
	Name   string
	Status CaseStatus
	Error  string
}

type ContractReport struct {
	Suite string
	Cases []CaseResult
}

func (r ContractReport) Passed() bool {
	if len(r.Cases) == 0 {
		return false
	}
	for _, result := range r.Cases {
		if result.Status == CaseFailed {
			return false
		}
	}
	return true
}

func (r ContractReport) AllPassed() bool {
	if len(r.Cases) == 0 {
		return false
	}
	for _, result := range r.Cases {
		if result.Status != CasePassed {
			return false
		}
	}
	return true
}

func (r ContractReport) RequiredPassed(ids ...string) bool {
	if len(ids) == 0 {
		return r.Passed()
	}
	results := make(map[string]CaseStatus, len(r.Cases))
	for _, result := range r.Cases {
		results[result.ID] = result.Status
	}
	for _, id := range ids {
		if results[id] != CasePassed {
			return false
		}
	}
	return true
}

type contractCase struct {
	id   string
	name string
	run  func() error
}

type skippedCase string

func (s skippedCase) Error() string { return string(s) }

func skipCase(reason string) error { return skippedCase(reason) }

func runContractCases(t *testing.T, suite string, cases []contractCase) ContractReport {
	t.Helper()
	report := ContractReport{Suite: suite, Cases: make([]CaseResult, 0, len(cases))}
	for _, tc := range cases {
		result := CaseResult{ID: tc.id, Name: tc.name, Status: CasePassed}
		if err := tc.run(); err != nil {
			if _, ok := err.(skippedCase); ok {
				result.Status = CaseSkipped
			} else {
				result.Status = CaseFailed
			}
			result.Error = err.Error()
		}
		report.Cases = append(report.Cases, result)
	}
	return report
}
