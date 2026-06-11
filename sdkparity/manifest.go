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
