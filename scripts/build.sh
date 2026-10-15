#!/usr/bin/env bash
# 프론트를 빌드해 서버 바이너리에 넣는다(로컬 확인용). 이미지 빌드는 Dockerfile 이 같은 일을 한다.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
(cd "$root/web" && npm ci && npm run build)
dist="$root/server/internal/webui/dist"
find "$dist" -mindepth 1 ! -name .gitkeep -exec rm -rf {} +
cp -r "$root/web/dist/." "$dist/"
(cd "$root/server" && go build -o "$root/bin/monitor" ./cmd/monitor)
echo "빌드 완료: bin/monitor"
