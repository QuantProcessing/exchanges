#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "usage: scripts/verify_exchange.sh <exchange>" >&2
  exit 1
fi

exchange="$1"
case "$exchange" in
  aster | backpack | binance | bitget | bybit | edgex | grvt | hyperliquid | lighter | nado | okx | standx) ;;
  *)
    echo "unknown exchange: $exchange" >&2
    exit 1
    ;;
esac

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

go test -short "./sdk/${exchange}/..." "./adapter/${exchange}/..."
