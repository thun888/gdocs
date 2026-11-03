#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"

target=""

usage() {
	cat <<'EOF'
用法: ./build.sh [选项]

选项:
  -t, --target GOOS/GOARCH
                         交叉编译目标，默认本机，如 linux/amd64
  -h, --help             显示帮助
EOF
}

while [ $# -gt 0 ]; do
	case $1 in
	-t | --target) target=$2; shift 2 ;;
	-h | --help) usage; exit 0 ;;
	*)
		echo "未知参数: $1" >&2
		usage >&2
		exit 2
		;;
	esac
done

if [ -f web/package.json ]; then
  if [ ! -d web/node_modules ]; then
    (cd web && npm ci)
  fi
  (cd web && npm run build)
fi

goos=$(go env GOOS)
goarch=$(go env GOARCH)
if [ -n "$target" ]; then
  goos=${target%%/*}
  goarch=${target##*/}
fi

ext=""
if [ "$goos" = windows ]; then
  ext=.exe
fi

export CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch"

mkdir -p bin
echo "构建 $goos/$goarch -> bin/gdocs$ext"
go build -trimpath -ldflags "-s -w" -o "bin/gdocs$ext" ./cmd/gdocs
