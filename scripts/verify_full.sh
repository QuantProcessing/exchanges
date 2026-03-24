#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
env_file="$repo_root/.env"

load_env_file() {
  local file="$1"
  local line key value

  while IFS= read -r line || [[ -n "$line" ]]; do
    line="${line#"${line%%[![:space:]]*}"}"
    [[ -z "$line" || "${line:0:1}" == "#" ]] && continue
    [[ "$line" != *=* ]] && continue

    key="${line%%=*}"
    value="${line#*=}"
    key="${key%"${key##*[![:space:]]}"}"

    if [[ -n ${!key+x} ]]; then
      continue
    fi

    printf -v "$key" '%s' "$value"
    export "$key"
  done < "$file"
}

apply_legacy_aliases() {
  local legacy canonical
  while IFS='=' read -r legacy canonical; do
    if [[ -n ${!canonical+x} ]]; then
      continue
    fi
    if [[ -n ${!legacy+x} ]]; then
      printf -v "$canonical" '%s' "${!legacy}"
      export "$canonical"
    fi
  done <<'EOF'
EDGEX_PRIVATE_KEY=EDGEX_STARK_PRIVATE_KEY
NADO_SUB_ACCOUNT_NAME=NADO_SUBACCOUNT_NAME
OKX_SECRET_KEY=OKX_API_SECRET
OKX_PASSPHRASE=OKX_API_PASSPHRASE
EOF
}

if [[ ! -f "$env_file" ]]; then
  echo "missing $env_file" >&2
  exit 1
fi

load_env_file "$env_file"
apply_legacy_aliases

required_vars=(
  EDGEX_STARK_PRIVATE_KEY
  EDGEX_ACCOUNT_ID
  GRVT_API_KEY
  GRVT_SUB_ACCOUNT_ID
  GRVT_PRIVATE_KEY
  HYPERLIQUID_PRIVATE_KEY
  HYPERLIQUID_ACCOUNT_ADDR
  LIGHTER_PRIVATE_KEY
  LIGHTER_ACCOUNT_INDEX
  LIGHTER_KEY_INDEX
  NADO_PRIVATE_KEY
  NADO_SUBACCOUNT_NAME
  OKX_API_KEY
  OKX_API_SECRET
  OKX_API_PASSPHRASE
  STANDX_PRIVATE_KEY
)

missing=()
for key in "${required_vars[@]}"; do
  if [[ -z "${!key:-}" ]]; then
    missing+=("$key")
  fi
done

if [[ ${#missing[@]} -gt 0 ]]; then
  printf 'missing required env vars: %s\n' "${missing[*]}" >&2
  exit 1
fi

cd "$repo_root"
export RUN_FULL=1

go test -short ./...
go test ./aster/sdk/perp ./binance/sdk/perp ./edgex/sdk/perp ./grvt/sdk ./hyperliquid/sdk ./hyperliquid/sdk/perp ./lighter/sdk ./lighter/sdk/common ./nado/sdk ./okx/sdk ./standx/sdk
