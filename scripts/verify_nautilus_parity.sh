#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

GOCACHE="${GOCACHE:-/private/tmp/go-build-exchanges}"
export GOCACHE

run_gate() {
  local name="$1"
  shift
  printf '\n==> %s\n' "$name"
  "$@"
}

run_gate "targeted master parity gate" \
  go test -count=1 ./testsuite -run 'TestNautilusMaster'

run_gate "full non-SDK test suite" \
  go test -count=1 $(env GOCACHE="$GOCACHE" go list ./... | rg -v '/sdk/')

run_gate "race-sensitive core suites" \
  go test -race -count=1 ./model ./cache ./account ./execution ./risk ./portfolio ./platform ./strategy ./backtest ./live ./testsuite

run_gate "SDK compile" \
  go test -run '^$' -count=1 ./sdk/...

run_gate "go vet" \
  go vet ./...

run_gate "diff hygiene" \
  git diff --check
