#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"

if [ -f web/package.json ]; then
  if [ ! -d web/node_modules ]; then
    (cd web && npm ci)
  fi
  (cd web && npm run build)
fi

go build -trimpath -ldflags "-s -w" -o "bin/gdocs$(go env GOEXE)" ./cmd/gdocs
