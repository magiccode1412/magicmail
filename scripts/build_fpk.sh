#!/usr/bin/env bash
# ============================================================================
# Magicmail FPK 打包构建脚本
# 说明:
#   本脚本产出的是**调试包**：manifest 的版本号会追加时间戳后缀（如 1.2.0.2609021548），
#   这样每次构建的版本都比设备上已安装的高，可以直接覆盖安装，不必先卸载。
#   时间戳只在打包期间写入，构建结束（无论成功、失败还是 Ctrl-C 中断）都会还原 manifest。
#
# 用法:
#   ./scripts/build_fpk.sh          # 构建 x86 调试包（platform = x86）
#   ./scripts/build_fpk.sh x86      # 同上
#   ./scripts/build_fpk.sh arm64    # 构建 ARM64 调试包（platform = arm）
#
#   DEBUG_VERSION_SUFFIX="" ./scripts/build_fpk.sh   # 不加时间戳后缀，等价于发布包
#
# 流程:
# 1. 执行 scripts/build.sh linux <amd64|arm64> 单平台构建
# 2. 复制产物到 fnapp/app/server/
# 3. 按目标架构改写 fnapp/manifest 的 platform，并给 version 追加时间戳
#    （构建结束后自动还原，不会污染 git 跟踪的 manifest）
# 4. 进入 fnapp 目录执行 fnpack build，产物重命名为
#    fnapp/magicmail-<version>-<arch>.fpk
# ============================================================================

set -euo pipefail

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

# 项目根目录（脚本在 scripts/ 下，根目录是上一级）
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN_DIR="${ROOT}/bin"
FNAPP_SERVER_DIR="${ROOT}/fnapp/app/server"
FNAPP_DIR="${ROOT}/fnapp"
BINARY_NAME="magicmail"

# ── 目标架构（默认 x86，可传 arm64）────────────────
# 飞牛 fnOS manifest 的 platform 合法取值为 x86 / arm / all，
# ARM64 设备属于 arm 平台，因此 arm64 包声明 platform = arm。
TARGET_ARCH="${1:-x86}"
case "${TARGET_ARCH}" in
    x86)
        GO_ARCH="amd64"
        FNOS_PLATFORM="x86"
        ;;
    arm64)
        GO_ARCH="arm64"
        FNOS_PLATFORM="arm"
        ;;
    *)
        echo -e "${RED}✗ 未知架构: ${TARGET_ARCH}（可选: x86 | arm64）${NC}"
        exit 1
        ;;
esac

MANIFEST_FILE="${FNAPP_DIR}/manifest"
MANIFEST_BACKUP="${ROOT}/.manifest.bak.$$"
APP_VERSION="$(sed -n 's/^version[[:space:]]*=[[:space:]]*\([^[:space:]]*\).*/\1/p' "${MANIFEST_FILE}" | head -1)"

# ── 调试包版本号：在原始版本号后追加时间戳 ──────────
# 设备已装同版本时会拒绝安装（必须先卸载），追加时间戳保证每次都比已装版本高。
# 默认 1.2.0 → 1.2.0.2609021548（多一级数字，恒大于 1.2.0，按“更新”安装）。
# 若设备要求严格三段 semver，可改成 DEBUG_VERSION_SUFFIX="-2609021548"；
# 置空则完全不加后缀，产出与发布包一致的版本号。
DEBUG_VERSION_SUFFIX="${DEBUG_VERSION_SUFFIX-.$(date '+%y%m%d%H%M')}"
PKG_VERSION="${APP_VERSION}${DEBUG_VERSION_SUFFIX}"

# manifest 是被 git 跟踪的文件，platform / version 只在打包期间临时改写，
# 任何退出路径（正常结束、失败、Ctrl-C、被 kill）都要还原，避免污染工作区。
restore_manifest() {
    if [ -f "${MANIFEST_BACKUP}" ]; then
        cp -f "${MANIFEST_BACKUP}" "${MANIFEST_FILE}"
        rm -f "${MANIFEST_BACKUP}"
        echo -e "${BLUE}  ↩ manifest 已还原为工作区内容（version=${APP_VERSION}）${NC}"
    fi
}
trap restore_manifest EXIT
trap 'restore_manifest; exit 130' INT
trap 'restore_manifest; exit 143' TERM

# 飞牛统一网关基础路径前缀。
# 必须与 fnapp/manifest 的 gatewayPrefix、fnapp/cmd/main 注入的
# MAGICMAIL_BASE_PATH 保持一致（默认 /app/magicmail）。
# 允许外部环境变量覆盖（如构建其他前缀的子路径部署）。
export BASE_URL="${BASE_URL:-/app/magicmail}"

# 后端运行时基础路径（仅用于文档/校验提示；实际由 fnapp/cmd/main 在
# 运行时注入 MAGICMAIL_BASE_PATH，这里保持一致以便人工核对）。
FNAS_BACKEND_BASE_PATH="/app/magicmail"

print_env_summary() {
    echo -e "${BLUE}  前端 BASE_URL        = ${BASE_URL}${NC}"
    echo -e "${BLUE}  后端 MAGICMAIL_BASE_PATH (运行时) = ${FNAS_BACKEND_BASE_PATH}${NC}"
    echo -e "${BLUE}  目标架构             = ${TARGET_ARCH} (GOARCH=${GO_ARCH})${NC}"
    echo -e "${BLUE}  manifest platform    = ${FNOS_PLATFORM}（构建结束后还原）${NC}"
    echo -e "${BLUE}  manifest 原始版本    = ${APP_VERSION}（构建结束后还原）${NC}"
    echo -e "${BLUE}  调试包版本           = ${PKG_VERSION}${NC}"
}

print_banner() {
    echo -e "${CYAN}${BOLD}"
    echo "╔══════════════════════════════════════════╗"
    echo "║     Magicmail FPK 构建工具                ║"
    echo "╚══════════════════════════════════════════╝"
    echo -e "${NC}"
}

# Step 1: 执行 build.sh 进行单平台构建 (linux amd64)
step1_build() {
    echo -e "${BLUE}▶ [Step 1/4] 执行 build.sh linux ${GO_ARCH} 构建（含前端，BASE_URL=${BASE_URL}）...${NC}"
    print_env_summary

    cd "${ROOT}"
    # BASE_URL 已 export，build.sh 内部 pnpm build 会继承该环境变量，
    # 使前端产物以 /app/magicmail 为 base 打包并嵌入 Go 二进制。
    bash "${ROOT}/scripts/build.sh" linux "${GO_ARCH}"

    # 检查构建产物是否存在
    if [ ! -f "${BIN_DIR}/${BINARY_NAME}" ]; then
        echo -e "${RED}✗ 构建产物不存在: ${BIN_DIR}/${BINARY_NAME}${NC}"
        exit 1
    fi

    echo -e "${GREEN}  ✅ 单平台构建完成${NC}"
}

# Step 2: 复制产物到 fnapp/app/server/
step2_copy() {
    echo -e "${BLUE}▶ [Step 2/4] 复制 ${BINARY_NAME} 到 fnapp/app/server/...${NC}"
    
    mkdir -p "${FNAPP_SERVER_DIR}"
    
    cp -f "${BIN_DIR}/${BINARY_NAME}" "${FNAPP_SERVER_DIR}/magicmail"
    chmod +x "${FNAPP_SERVER_DIR}/magicmail"
    
    local size
    size=$(du -h "${FNAPP_SERVER_DIR}/magicmail" | cut -f1)
    echo -e "${GREEN}  ✅ 已复制: ${FNAPP_SERVER_DIR}/magicmail (${size})${NC}"
}

# Step 3: 按目标架构改写 manifest 的 platform（退出时自动还原，避免污染工作区）
step3_manifest() {
    echo -e "${BLUE}▶ [Step 3/4] 改写 manifest: platform = ${FNOS_PLATFORM}, version = ${PKG_VERSION}...${NC}"

    cp -f "${MANIFEST_FILE}" "${MANIFEST_BACKUP}"
    # 用临时文件代替 sed -i，兼容 GNU/BSD sed
    sed "s|^platform *=.*|platform              = ${FNOS_PLATFORM}|" "${MANIFEST_FILE}" > "${MANIFEST_FILE}.tmp"
    mv -f "${MANIFEST_FILE}.tmp" "${MANIFEST_FILE}"

    if [ -n "${DEBUG_VERSION_SUFFIX}" ]; then
        sed "s|^version *=.*|version               = ${PKG_VERSION}|" "${MANIFEST_FILE}" > "${MANIFEST_FILE}.tmp"
        mv -f "${MANIFEST_FILE}.tmp" "${MANIFEST_FILE}"
    fi

    grep -E '^(version|platform)' "${MANIFEST_FILE}"
    echo -e "${GREEN}  ✅ manifest 已改写（原始内容已备份，退出时还原）${NC}"
}

# Step 4: 进入 fnapp 目录执行 fnpack build
step4_fnpack() {
    echo -e "${BLUE}▶ [Step 4/4] 执行 fnpack build...${NC}"

    cd "${FNAPP_DIR}"

    # 打包前必须清理历史 .fpk。
    # 旧实现用 `for f in *.fpk; do src="$f"; break; done` 取第一个文件，
    # 但 'magicmail-1.2.0-x86.fpk' 会排在 'magicmail.fpk' 之前（'-' 0x2D < '.' 0x2E），
    # 于是永远命中上一轮的残留包；又因 src == dst 跳过重命名，结果旧包原封不动，
    # 新包却以 magicmail.fpk 留在目录里 —— 极易装错包。
    rm -f ./*.fpk

    fnpack build

    # fnpack 输出的包名不固定，统一成 名称-版本-架构.fpk
    local src="" dst
    for f in *.fpk; do
        [ -f "${f}" ] || continue
        src="${f}"
        break
    done
    if [ -z "${src}" ]; then
        echo -e "${RED}✗ 未找到 fnpack 产物（${FNAPP_DIR}/*.fpk）${NC}"
        exit 1
    fi

    dst="magicmail-${APP_VERSION}-${TARGET_ARCH}.fpk"
    if [ "${src}" != "${dst}" ]; then
        mv -f "${src}" "${dst}"
    fi

    echo -e "${GREEN}  ✅ FPK 打包完成: ${FNAPP_DIR}/${dst}${NC}"
}

# 主入口
main() {
    print_banner
    
    step1_build
    step2_copy
    step3_manifest
    step4_fnpack
    
    echo ""
    echo -e "${GREEN}═══════════════════════════════════════${NC}"
    echo -e "  ${GREEN}✅ FPK 构建流程全部完成！${NC}"
    echo -e "  架构:     ${CYAN}${TARGET_ARCH}${NC} (GOARCH=${GO_ARCH}, platform=${FNOS_PLATFORM})"
    echo -e "  包内版本: ${CYAN}${PKG_VERSION}${NC}（manifest 原始版本 ${APP_VERSION} 已还原）"
    echo -e "  输出文件: ${CYAN}${FNAPP_DIR}/magicmail-${APP_VERSION}-${TARGET_ARCH}.fpk${NC}"
    echo -e "${GREEN}═══════════════════════════════════════${NC}"
}

main "$@"
