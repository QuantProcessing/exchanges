package testenv

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joho/godotenv"
)

// LoadRepoEnv loads the repo-root .env into the current process without
// overriding shell-exported environment variables.
func LoadRepoEnv() error {
	root, err := findRepoRoot()
	if err != nil {
		return err
	}

	values, err := godotenv.Read(filepath.Join(root, ".env"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for key, value := range values {
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}

	applyLegacyAliases()

	return nil
}

func RequireEnv(t testing.TB, vars ...string) {
	t.Helper()

	if err := LoadRepoEnv(); err != nil {
		t.Fatalf("load repo .env: %v", err)
	}

	var missing []string
	for _, key := range vars {
		if os.Getenv(key) == "" {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		t.Skipf("skipping: missing required env %s", strings.Join(missing, ", "))
	}
}

func RequireFull(t testing.TB, vars ...string) {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping: full verification test excluded by -short")
	}
	if err := LoadRepoEnv(); err != nil {
		t.Fatalf("load repo .env: %v", err)
	}
	if os.Getenv("RUN_FULL") != "1" {
		t.Skip("skipping: set RUN_FULL=1 to run full verification tests")
	}
	RequireEnv(t, vars...)
}

func RequireSoak(t testing.TB, vars ...string) {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping: soak verification test excluded by -short")
	}
	if err := LoadRepoEnv(); err != nil {
		t.Fatalf("load repo .env: %v", err)
	}
	if os.Getenv("RUN_SOAK") != "1" {
		t.Skip("skipping: set RUN_SOAK=1 to run soak verification tests")
	}
	RequireEnv(t, vars...)
}

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}

func applyLegacyAliases() {
	for legacy, canonical := range map[string]string{
		"EDGEX_PRIVATE_KEY":    "EDGEX_STARK_PRIVATE_KEY",
		"NADO_SUB_ACCOUNT_NAME": "NADO_SUBACCOUNT_NAME",
		"OKX_SECRET_KEY":       "OKX_API_SECRET",
		"OKX_PASSPHRASE":       "OKX_API_PASSPHRASE",
	} {
		if _, exists := os.LookupEnv(canonical); exists {
			continue
		}
		if value, exists := os.LookupEnv(legacy); exists {
			_ = os.Setenv(canonical, value)
		}
	}
}
