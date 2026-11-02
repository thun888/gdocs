#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"

if [ -f web/package.json ]; then
  (cd web && npm ci && npm run build)
fi

go build -trimpath -ldflags "-s -w" -o "bin/zdocs$(go env GOEXE)" ./cmd/zdocs
