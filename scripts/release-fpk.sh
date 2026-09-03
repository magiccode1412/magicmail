#!/usr/bin/env bash
# ============================================================================
# 构建单个架构的飞牛 fnOS 应用包（发版用）
#
# 用法:
#   bash scripts/release-fpk.sh <arch> <goarch> <platform>
#   bash scripts/release-fpk.sh x86   amd64 x86
#   bash scripts/release-fpk.sh arm64 arm64 arm
#
# arch     产物名里的架构标识
# goarch   Go 交叉编译目标架构
# platform fnOS manifest 的 platform 字段：合法取值为 x86 / arm / all，
#          ARM64 设备属于 arm 平台，所以 arm64 包声明 platform = arm。
#
# 注意点（与 .github/workflows/release.yml 的 build-fpk 一致）：
#   1. 不能复用普通 linux 二进制，那份嵌入的前端 base 是 '/'，网关下会白屏，
#      这里用的是 build/web-gateway（BASE_URL=/app/magicmail）。
#   2. fnapp/manifest 是被 git 跟踪的文件，只能在**副本**里改 platform，
#      不能 sed 仓库里的原件（本地 build_fpk.sh 用的是备份+还原的方式）。
#   3. 打包目录沿用 fnapp 这个名字，与本地 ./build_fpk.sh 的路径保持一致，
#      避免 fnpack 对目录名有隐含依赖；Go 源码副本放在 fnapp 外面，不进包体。
# ============================================================================
set -euo pipefail

ARCH="$1"
GOARCH_TARGET="$2"
PLATFORM="$3"

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

fail() { printf '\n::error::%s\n' "$*"; exit 1; }

WORK="${ROOT}/build/fpk-${ARCH}"
APPDIR="${WORK}/fnapp"      # fnpack 打包目录
SRCDIR="${WORK}/server"     # Go 源码副本（不进包）

rm -rf "$WORK"
mkdir -p "$WORK"
cp -r fnapp "$APPDIR"
mkdir -p "$APPDIR/app/server"

cp -r server "$SRCDIR"
mkdir -p "$SRCDIR/embedfs/dist"
cp -r build/web-gateway/. "$SRCDIR/embedfs/dist/"

(
  cd "$SRCDIR"
  CGO_ENABLED=0 GOOS=linux GOARCH="$GOARCH_TARGET" \
    go build \
      -ldflags="-s -w -X main.version=${APP_VERSION:-0.0.0} -X main.isProduction=true" \
      -o "${APPDIR}/app/server/magicmail" \
      .
)
chmod +x "$APPDIR/app/server/magicmail"

# 架构装错 = 装上就跑不起来，这里直接卡住
INFO="$(file -b "$APPDIR/app/server/magicmail")"
echo "Binary: ${INFO}"
case "$GOARCH_TARGET" in
  amd64) printf '%s' "$INFO" | grep -q 'x86-64'       || fail "期望 x86_64 二进制，实际: ${INFO}" ;;
  arm64) printf '%s' "$INFO" | grep -qE 'aarch64|ARM' || fail "期望 ARM64 二进制，实际: ${INFO}" ;;
esac

# 架构声明必须与包内二进制一致，否则目标设备安装时会拒绝
sed -i "s|^platform *=.*|platform              = ${PLATFORM}|" "$APPDIR/manifest"
grep '^platform' "$APPDIR/manifest"

( cd "$APPDIR" && fnpack build )

# fnpack 输出的包名不固定，统一成 名称-版本-架构.fpk
SRC_FPK="$(ls "$APPDIR"/*.fpk 2>/dev/null | head -1 || true)"
[ -n "$SRC_FPK" ] || fail "未找到 fnpack 产物（${APPDIR}/*.fpk）"

mkdir -p dist
mv -f "$SRC_FPK" "${ROOT}/dist/magicmail-${APP_VERSION}-${ARCH}.fpk"
ls -lh "dist/magicmail-${APP_VERSION}-${ARCH}.fpk"
