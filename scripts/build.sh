#!/usr/bin/env bash
# ============================================================================
# Magicmail 生产构建脚本
# 用法: ./scripts/build.sh [GOOS] [GOARCH] [GOARM]
#   不带参数      - 构建全平台版本 (linux/amd64, linux/arm, linux/arm64,
#                  darwin/arm64, windows/amd64) → bin/
#   linux amd64     - 交叉编译 Linux x86_64
#   linux arm       - 交叉编译 Linux ARM32 (默认 armv7, 可选 5/6/7)
#   linux arm64     - 交叉编译 Linux ARM64 (AArch64)
#   darwin arm64    - 交叉编译 macOS Apple Silicon
#   windows amd64   - 交叉编译 Windows x86_64
#
# ARM32 说明:
#   GOARM 可选值: 5 (armv5), 6 (armv6), 7 (armv7, 默认)
#   示例: ./scripts/build.sh linux arm 7    # Raspberry Pi 2+
#         ./scripts/build.sh linux arm 6     # Raspberry Pi Zero/1
#         ./scripts/build.sh linux arm 5     # 老旧 ARM 设备
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
SERVER_DIR="${ROOT}/server"
WEB_DIR="${ROOT}/web"
OUTPUT_DIR="${ROOT}/bin"
BINARY_NAME="magicmail"

print_banner() {
    echo -e "${CYAN}${BOLD}"
    echo "╔══════════════════════════════════════════╗"
    echo "║       Magicmail 构建工具                     ║"
    echo "╚══════════════════════════════════════════╝"
    echo -e "${NC}"
}

check_deps() {
    local missing=()

    command -v go &>/dev/null || missing+=("go")
    command -v node &>/dev/null || missing+=("node")

    if [ ${#missing[@]} -gt 0 ]; then
        echo -e "${RED}✗ 缺少依赖: ${missing[*]}${NC}"
        exit 1
    fi

    # 检查 pnpm 或 npm
    if ! command -v pnpm &>/dev/null && ! command -v npm &>/dev/null; then
        echo -e "${RED}✗ 缺少包管理器: 请安装 pnpm 或 npm${NC}"
        exit 1
    fi

    # 检查前端依赖
    if [ ! -d "${WEB_DIR}/node_modules" ]; then
        echo -e "${YELLOW}📦 正在安装前端依赖...${NC}"
        (cd "${WEB_DIR}" && install_frontend_deps)
    fi
}

install_frontend_deps() {
    if command -v pnpm &>/dev/null; then
        pnpm install
    else
        npm install
    fi
}

get_pkg_manager() {
    if command -v pnpm &>/dev/null; then
        echo "pnpm"
    else
        echo "npm"
    fi
}

# 全平台构建目标列表
ALL_PLATFORMS=(
    "linux:amd64"
    "linux:arm:7"
    "linux:arm64"
    "darwin:arm64"
    "windows:amd64"
)

# ── 构建前端（所有平台共用） ──────────────────────
build_frontend() {
    echo -e "${BLUE}▶ [1/N] 构建前端...${NC}"
    cd "${WEB_DIR}"

    local pkg_mgr
    pkg_mgr="$(get_pkg_manager)"

    if [ "${pkg_mgr}" = "pnpm" ]; then
        pnpm build
    else
        npm run build
    fi

    echo -e "${GREEN}  ✅ 前端构建完成${NC}"
}

# ── 嵌入前端产物到 Go embed 路径 ──────────────────
embed_frontend() {
    local EMBED_DIST="${SERVER_DIR}/embedfs/dist"
    local FRONTEND_DIST="${SERVER_DIR}/dist"

    rm -rf "${EMBED_DIST}"
    if [ -d "${FRONTEND_DIST}" ]; then
        cp -r "${FRONTEND_DIST}" "${EMBED_DIST}"
        echo "  📎 前端产物已嵌入 Go 二进制"
    else
        echo -e "${RED}  ✗ 未找到前端构建产物: ${FRONTEND_DIST}${NC}"
        exit 1
    fi
}

# ── 编译单个平台二进制 ────────────────────────────
build_single_platform() {
    local target_os="$1"
    local target_arch="$2"
    local target_arm="${3:-}"
    local platform_index="$4"
    local total_count="$5"

    printf "${BLUE}▶ [%d/%d] 编译 %s/%s ...${NC}\n" "$platform_index" "$total_count" "$target_os" "$target_arch"
    cd "${SERVER_DIR}"

    # 每次编译前重新嵌入（防止上次清理）
    embed_frontend

    local ldflags="-s -w -X main.version=$(date +%Y%m%d-%H%M) -X main.isProduction=true"

    # 确定输出文件名
    local output="${OUTPUT_DIR}/${BINARY_NAME}-${target_os}-${target_arch}"
    if [ "${target_os}" = "windows" ]; then
        output="${output}.exe"
    fi

    # 设置交叉编译环境变量
    export GOOS="${target_os}"
    export GOARCH="${target_arch}"

    # ARM32 需要 GOARM 指令集版本（默认 7）
    if [ "${target_arch}" = "arm" ] && [ -n "${target_arm}" ]; then
        export GOARM="${target_arm}"
        echo "  🔧 ARM 指令集版本: v${target_arm}"
    elif [ "${target_arch}" = "arm" ]; then
        export GOARM="7"
        echo "  🔧 ARM 指令集版本: v7 (默认)"
    fi

    go build -ldflags="${ldflags}" -o "${output}" .

    unset GOOS GOARCH GOARM

    echo -e "${GREEN}  ✅ 完成: ${output} ($(du -h "${output}" | cut -f1))${NC}"
}

# ── 全平台构建 ─────────────────────────────────────
cmd_build_all() {
    print_banner
    check_deps

    mkdir -p "${OUTPUT_DIR}"
    local total=${#ALL_PLATFORMS[@]}

    echo -e "${YELLOW}🚀 开始全平台构建 (${total} 个架构)${NC}"
    echo ""

    # Step 1: 构建前端（只做一次）
    build_frontend

    # Step 2: 循环交叉编译各平台
    echo ""
    echo -e "${BLUE}▶ [2/${total}] 交叉编译各平台...${NC}"

    local i=1
    for platform in "${ALL_PLATFORMS[@]}"; do
        IFS=':' read -r os arch arm <<< "${platform}"
        build_single_platform "${os:-}" "${arch:-}" "${arm:-}" "${i}" "${total}"
        ((i++))
    done

    # 汇总输出
    echo ""
    echo -e "${GREEN}═══════════════════════════════════════${NC}"
    echo -e "  ${GREEN}✅ 全平台构建完成！共 ${total} 个二进制${NC}"
    echo -e "  输出目录: ${CYAN}${OUTPUT_DIR}/${NC}"
    echo -e ""
    echo -e "  构建产物:"
    ls -lh "${OUTPUT_DIR}/${BINARY_NAME}"-* 2>/dev/null || true
    echo -e "${GREEN}═══════════════════════════════════════${NC}"
}

# ── 单平台构建 ─────────────────────────────────────
cmd_build_single() {
    print_banner
    check_deps

    local target_os="${1:-}"
    local target_arch="${2:-}"
    local target_arm="${3:-}"

    mkdir -p "${OUTPUT_DIR}"

    # 确定输出文件名（Windows 加 .exe 后缀）
    local output="${OUTPUT_DIR}/${BINARY_NAME}"
    if [ "${target_os}" = "windows" ]; then
        output="${output}.exe"
    fi

    # Step 1: 构建前端
    build_frontend

    # Step 2: 编译指定平台
    echo ""
    echo -e "${BLUE}▶ [2/2] 编译 Go 后端（嵌入前端产物）...${NC}"
    cd "${SERVER_DIR}"

    embed_frontend

    local ldflags="-s -w -X main.version=$(date +%Y%m%d-%H%M) -X main.isProduction=true"

    if [ -n "${target_os}" ]; then
        export GOOS="${target_os}"
    fi
    if [ -n "${target_arch}" ]; then
        export GOARCH="${target_arch}"
    fi
    if [ "${target_arch}" = "arm" ] && [ -n "${target_arm}" ]; then
        export GOARM="${target_arm}"
        echo "  🔧 ARM 指令集版本: v${target_arm}"
    elif [ "${target_arch}" = "arm" ]; then
        export GOARM="7"
        echo "  🔧 ARM 指令集版本: v7 (默认)"
    fi

    go build -ldflags="${ldflags}" -o "${output}" .

    unset GOOS GOARCH GOARM

    echo -e "${GREEN}  ✅ 编译完成${NC}"
    local size
    size=$(du -h "${output}" | cut -f1)

    local platform="${target_os:-$(go env GOOS)}/${target_arch:-$(go env GOARCH)}"

    echo ""
    echo -e "${GREEN}═══════════════════════════════════════${NC}"
    echo -e "  ${GREEN}构建成功！${NC}"
    echo -e "  目标平台: ${CYAN}${platform}${NC}"
    echo -e "  输出文件: ${CYAN}${output}${NC} (${size})"
    echo -e "  运行方式: ${YELLOW}.${output}${NC}"
    echo -e "${GREEN}═══════════════════════════════════════${NC}"
}

cmd_build() {
    # 无参数 → 全平台，有参数 → 单平台
    if [ $# -eq 0 ]; then
        cmd_build_all
    else
        cmd_build_single "$@"
    fi
}

cmd_clean() {
    print_banner
    echo -e "${BLUE}▶ 清理构建产物...${NC}"

    # 清理全平台构建产物
    rm -f ${OUTPUT_DIR}/${BINARY_NAME}-* 2>/dev/null || true
    # 清理单平台构建产物
    rm -f "${OUTPUT_DIR}/${BINARY_NAME}" "${OUTPUT_DIR}/${BINARY_NAME}.exe"
    # 清理临时文件
    rm -rf "${SERVER_DIR}/embedfs/dist"
    rm -rf "${SERVER_DIR}/dist"
    rm -rf "${WEB_DIR}/dist"

    echo -e "${GREEN}  ✅ 清理完成${NC}"
}

cmd_help() {
    print_banner
    cat <<EOF
用法: ./scripts/build.sh [命令] [GOOS] [GOARCH] [GOARM]

命令:
  (无参数)     构建全平台版本 (5 个架构)
  clean        清理所有构建产物
  help         显示帮助信息

单平台示例 (指定 GOOS GOARCH):
  ./scripts/build.sh linux amd64          # Linux x86_64
  ./scripts/build.sh linux arm            # Linux ARM32 (armv7, 默认)
  ./scripts/build.sh linux arm 7          # Linux ARM32 (Raspberry Pi 2+)
  ./scripts/build.sh linux arm 6          # Linux ARM32 (Raspberry Pi Zero/1)
  ./scripts/build.sh linux arm 5          # Linux ARM32 (老旧设备)
  ./scripts/build.sh linux arm64          # Linux ARM64 (AArch64, 如树莓派4/5)
  ./scripts/build.sh darwin arm64         # macOS Apple Silicon
  ./scripts/build.sh windows amd64        # Windows x86_64

全平台构建输出:
  bin/magicmail-linux-amd64       Linux x86_64 服务器
  bin/magicmail-linux-arm         Linux ARM32 (armv7)
  bin/magicmail-linux-arm64       Linux ARM64 (树莓派4/5等)
  bin/magicmail-darwin-arm64      macOS Apple Silicon
  bin/magicmail-windows-amd64.exe Windows x86_64

说明:
  - 前端产物通过 //go:embed 嵌入 Go 二进制，每个二进制独立完整
  - 不带参数时自动构建全部 5 个平台，前端只编译一次
  - ARM32 的 GOARM 参数可选: 5(armv5), 6(armv6), 7(armv7/默认)
  - 开发环境请使用: ./scripts/dev.sh start

常见设备对应架构:
  - Raspberry Pi Zero/1    → linux arm 6
  - Raspberry Pi 2/3       → linux arm 7
  - Raspberry Pi 4/5       → linux arm64
  - 标准服务器/PC          → linux amd64
  - Mac M1/M2/M3           → darwin arm64
EOF
}

# 主入口
main() {
    local cmd="${1:-build}"
    case "${cmd}" in
    clean)
        cmd_clean
        ;;
    help | --help | -h)
        cmd_help
        ;;
    *)
        cmd_build "$@"
        ;;
    esac
}

main "$@"
