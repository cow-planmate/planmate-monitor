#!/usr/bin/env bash
# 로컬 환경(scripts/local-up.sh)에 붙어서 서버를 띄운다. 설정은 local/.env.local.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
set -a
# shellcheck disable=SC1091
source "$root/local/.env.local"
set +a
export ES_API_KEY_FILE="$root/local/.secrets/es_api_key"
cd "$root/server"
exec go run ./cmd/monitor
