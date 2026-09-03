#!/usr/bin/env bash
# ============================================================================
# 构建前端两套产物（发版用）
#
# 为什么要构建两遍：
#   普通二进制的前端 base 是 '/'，直接塞进飞牛 FPK 会在统一网关的
#   /app/magicmail 前缀下 404 白屏。飞牛形态必须单独以 BASE_URL=/app/magicmail
#   构建前端并重新编译二进制（.github/workflows/release.yml 的 build-fpk
#   没有复用 build-binary 的产物，原因同此）。
#
# vite outDir 固定为 ../server/dist，两次构建不能直接并行（会互相覆盖），
# 因此这里串行构建，每构建完一份立刻挪到独立目录，供后续编译 stage 取用：
#   build/web-default   默认 base，用于 6 个平台的普通二进制
#   build/web-gateway   网关 base，用于飞牛 FPK
# ============================================================================
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

GATEWAY_BASE="${GATEWAY_BASE:-/app/magicmail}"

echo "==> 1/3 安装前端依赖"
( cd web && pnpm install --frozen-lockfile )

# ---------- 普通二进制用：默认 base ----------
echo "==> 2/3 构建前端（默认 base）"
rm -rf server/dist build/web-default
( cd web && pnpm build )
mkdir -p build/web-default
cp -r server/dist/. build/web-default/

# ---------- 飞牛 FPK 用：网关 base ----------
echo "==> 3/3 构建前端（网关 base ${GATEWAY_BASE}）"
rm -rf server/dist build/web-gateway
( cd web && BASE_URL="$GATEWAY_BASE" pnpm build )

# 网关前缀断言：前缀一丢就是白屏，宁可校验失败也不要出坏包
if ! grep -q "\"${GATEWAY_BASE}/" server/dist/index.html; then
  echo "::error::前端 index.html 未带 ${GATEWAY_BASE} 前缀，网关下会白屏"
  exit 1
fi
if grep -q '"src":"/icons/' server/dist/manifest.webmanifest; then
  echo "::error::manifest.webmanifest 图标未带 ${GATEWAY_BASE} 前缀，网关下会 404"
  exit 1
fi
echo "OK: 前端产物已带 ${GATEWAY_BASE} 前缀"

mkdir -p build/web-gateway
cp -r server/dist/. build/web-gateway/

# server/dist 只是中转目录，清掉避免被后续 go build 误当成 embed 源
rm -rf server/dist
