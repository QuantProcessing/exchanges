#!/usr/bin/env bash
set -euo pipefail

if [[ "${RUN_SOAK:-}" != "1" ]]; then
  echo "set RUN_SOAK=1 to run soak verification" >&2
  exit 1
fi

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

go test ./aster/sdk/perp ./binance/sdk/perp ./hyperliquid/sdk/perp -run '^(TestKline|TestSubscribeOrderUpdates|TestSubscribeWebData2)$'
