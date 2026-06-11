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
