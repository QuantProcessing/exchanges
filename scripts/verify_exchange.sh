#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "usage: scripts/verify_exchange.sh <exchange>" >&2
  exit 1
fi

exchange="$1"
case "$exchange" in
  backpack) pkg="./backpack/..." ;;
  aster) pkg="./aster/..." ;;
  binance) pkg="./binance/..." ;;
  bitget) pkg="./bitget/..." ;;
  edgex) pkg="./edgex/..." ;;
  grvt) pkg="./grvt/..." ;;
  hyperliquid) pkg="./hyperliquid/..." ;;
  lighter) pkg="./lighter/..." ;;
  nado) pkg="./nado/..." ;;
  okx) pkg="./okx/..." ;;
  standx) pkg="./standx/..." ;;
  *)
    echo "unknown exchange: $exchange" >&2
    exit 1
    ;;
esac

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

go test -short "$pkg"
