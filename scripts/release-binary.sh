#!/usr/bin/env bash
# ============================================================================
# 交叉编译单个平台的二进制（发版用）
#
# 用法:
#   bash scripts/release-binary.sh <goos> <goarch> <goarm> <输出文件名>
#   bash scripts/release-binary.sh linux arm64 7 magicmail-arm64
#
# 多个平台在流水线里以并行 job 调用本脚本。每个平台用一份独立的源码副本：
# //go:embed all:dist 要求前端产物就位于 <模块>/embedfs/dist，并行编译时
# 多个 job 写同一个目录会互相覆盖，所以这里先 cp -r server 到 build/go-*，
# 再往各自的副本里塞前端产物。源码体积很小，复制成本远低于编译耗时。
# ============================================================================
set -euo pipefail

GOOS_TARGET="$1"
GOARCH_TARGET="$2"
GOARM_TARGET="${3:-7}"
OUT="$4"

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

WORK="${ROOT}/build/go-${GOOS_TARGET}-${GOARCH_TARGET}-${GOARM_TARGET}"
rm -rf "$WORK"
cp -r server "$WORK"
mkdir -p "$WORK/embedfs/dist"
cp -r build/web-default/. "$WORK/embedfs/dist/"

mkdir -p dist
(
  cd "$WORK"
  CGO_ENABLED=0 GOOS="$GOOS_TARGET" GOARCH="$GOARCH_TARGET" GOARM="$GOARM_TARGET" \
    go build \
      -ldflags="-s -w -X main.version=${APP_VERSION:-0.0.0} -X main.isProduction=true" \
      -o "${ROOT}/dist/${OUT}" \
      .
)

echo "Built dist/${OUT}: $(du -h "dist/${OUT}" | cut -f1)"
