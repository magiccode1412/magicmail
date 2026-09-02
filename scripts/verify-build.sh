#!/usr/bin/env bash
# ============================================================================
# dev 分支构建校验
#
# 目标：每次 push 到 dev 时，把「发版时才会做的事」提前做一遍，尽早暴露问题。
# 与 .github/workflows/release.yml 的区别：不发 Release、不推镜像、不产生对外产物。
#
# 重点覆盖这几类失效场景：
#   1. 前端 / 后端编译失败
#   2. 6 个目标平台交叉编译失败
#   3. 飞牛 FPK 内前端丢失 /app/magicmail 前缀（装上后白屏）
#   4. FPK 内二进制架构与 manifest 声明不一致（装上后跑不起来）
#
# 前两类在合并进 main 之前就能发现；后两类是飞牛形态最常出的问题，
# 而它们只有真正打出 FPK 才能验证，靠单元测试和前端构建都发现不了。
#
# 用法：
#   bash scripts/verify-build.sh
#   VERSION=v1.2.0 bash scripts/verify-build.sh     # 指定注入二进制的版本号
# ============================================================================
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

GATEWAY_BASE="/app/magicmail"
VERSION="${VERSION:-0.0.0-dev}"
DIST="dist"

log()  { printf '\n\033[1;36m==> %s\033[0m\n' "$*"; }
fail() { printf '\n\033[1;31m::error::%s\033[0m\n' "$*"; exit 1; }

command -v go     >/dev/null 2>&1 || fail "缺少 go"
command -v pnpm   >/dev/null 2>&1 || fail "缺少 pnpm"
command -v fnpack >/dev/null 2>&1 || fail "缺少 fnpack"
command -v file   >/dev/null 2>&1 || fail "缺少 file"

# ---------- 1. 前端（默认 base，供普通二进制使用） ----------
log "1/4 构建前端（默认 base）"
( cd web && pnpm install --frozen-lockfile && pnpm build )

# ---------- 2. 后端多平台交叉编译 ----------
log "2/4 交叉编译后端（6 个目标平台）"
rm -rf server/embedfs/dist
cp -r server/dist server/embedfs/dist
mkdir -p "$DIST"

build_binary() {
  local goos="$1" goarch="$2" goarm="$3" out="$4"
  printf '  - %s/%s -> %s\n' "$goos" "$goarch" "$out"
  (
    cd server
    CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" GOARM="$goarm" \
      go build \
        -ldflags="-s -w -X main.version=${VERSION} -X main.isProduction=true" \
        -o "../${DIST}/${out}" .
  )
}

build_binary linux   amd64 "7" magicmail
build_binary linux   arm   "7" magicmail-arm
build_binary linux   arm64 "7" magicmail-arm64
build_binary darwin  amd64 "7" magicmail-macos-x86_64
build_binary darwin  arm64 "7" magicmail-macos-arm64
build_binary windows amd64 "7" magicmail.exe

# ---------- 3. 飞牛 FPK（网关 base，x86 + arm64） ----------
log "3/4 构建飞牛 FPK（x86 / arm64，网关 base）"
( cd web && BASE_URL="$GATEWAY_BASE" pnpm install --frozen-lockfile && BASE_URL="$GATEWAY_BASE" pnpm build )

# 网关前缀断言：前缀一丢就是白屏，宁可校验失败也不要出坏包
if ! grep -q "\"${GATEWAY_BASE}/" server/dist/index.html; then
  fail "前端 index.html 未带 ${GATEWAY_BASE} 前缀，网关下会白屏"
fi
if grep -q '"src":"/icons/' server/dist/manifest.webmanifest; then
  fail "manifest.webmanifest 图标未带 ${GATEWAY_BASE} 前缀，网关下会 404"
fi
echo "OK: 前端产物已带 ${GATEWAY_BASE} 前缀"

build_fpk() {
  local arch="$1" goarch="$2" platform="$3"
  printf '\n  ---- FPK %s (%s) ----\n' "$arch" "$goarch"

  rm -rf server/embedfs/dist fnapp/app/server
  rm -f fnapp/*.fpk
  cp -r server/dist server/embedfs/dist
  mkdir -p fnapp/app/server

  (
    cd server
    CGO_ENABLED=0 GOOS=linux GOARCH="$goarch" \
      go build \
        -ldflags="-s -w -X main.version=${VERSION} -X main.isProduction=true" \
        -o ../fnapp/app/server/magicmail .
  )
  chmod +x fnapp/app/server/magicmail

  # 架构装错 = 装上就跑不起来，这里直接卡住
  local info
  info="$(file -b fnapp/app/server/magicmail)"
  echo "  binary: ${info}"
  case "$goarch" in
    amd64)
      if ! printf '%s' "$info" | grep -q 'x86-64'; then
        fail "期望 x86_64 二进制，实际: ${info}"
      fi
      ;;
    arm64)
      if ! printf '%s' "$info" | grep -qE 'aarch64|ARM'; then
        fail "期望 ARM64 二进制，实际: ${info}"
      fi
      ;;
  esac

  # 架构声明必须与包内二进制一致，否则目标设备安装时会拒绝
  sed -i "s|^platform *=.*|platform              = ${platform}|" fnapp/manifest
  grep '^platform' fnapp/manifest

  ( cd fnapp && fnpack build )

  # fnpack 输出的包名不固定，统一成 名称-版本-架构.fpk
  local src dst
  src="$(ls fnapp/*.fpk 2>/dev/null | head -1 || true)"
  if [ -z "$src" ]; then
    fail "fnapp/ 下未找到 .fpk 产物"
  fi
  dst="fnapp/magicmail-${VERSION}-${arch}.fpk"
  if [ "$src" != "$dst" ]; then
    mv "$src" "$dst"
  fi
  ls -lh fnapp/*.fpk
}

build_fpk x86   amd64 x86
build_fpk arm64 arm64 arm

# ---------- 4. 清理临时产物 ----------
log "4/4 清理临时产物"
rm -rf dist fnapp/app/server server/embedfs/dist
rm -f fnapp/*.fpk
# 上面 sed 改过平台声明，还原以避免污染工作区
git checkout -- fnapp/manifest 2>/dev/null || true

log "构建校验通过：6 个平台二进制 + 双架构 FPK 均可正常构建"
