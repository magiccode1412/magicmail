#!/usr/bin/env bash
# ============================================================================
# Magicmail 一键开发环境启动脚本
# 用法: ./scripts/dev.sh [command]
#   start   - 启动前后端开发服务器（默认）
#   stop    - 停止所有开发服务
#   status  - 查看服务状态
#   help    - 显示帮助信息
# ============================================================================

set -euo pipefail

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

# 项目根目录（脚本在 scripts/ 下，根目录是上一级）
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SERVER_DIR="${ROOT}/server"
WEB_DIR="${ROOT}/web"
PID_DIR="${ROOT}/.dev-pids"

# 服务端口
BACKEND_PORT=8080
FRONTEND_PORT=5173

# 开发环境配置
export MAGICMAIL_ENV="dev"
DEV_USERNAME="admin"
DEV_PASSWORD="admin123"

# 子进程 PID（前台模式使用）
BACKEND_PID=0
FRONTEND_PID=0

# 创建必要目录
mkdir -p "${PID_DIR}"

# 确保 embedfs/dist 占位目录存在（开发模式下 //go:embed 要求目录必须存在，
# 运行时 isEmbedded() 会检测到空目录并降级到 Vite 代理）
mkdir -p "${SERVER_DIR}/embedfs/dist"

print_banner() {
    echo -e "${CYAN}${BOLD}"
    echo "╔══════════════════════════════════════════╗"
    echo "║       Magicmail 开发环境                     ║"
    echo "╚══════════════════════════════════════════╝"
    echo -e "${NC}"
}

check_deps() {
    local missing=()

    command -v go &>/dev/null || missing+=("go")
    command -v node &>/dev/null || missing+=("node")

    if [ ${#missing[@]} -gt 0 ]; then
        echo -e "${RED}✗ 缺少依赖: ${missing[*]}${NC}"
        echo "请先安装缺失的依赖工具"
        exit 1
    fi

    # 检查 pnpm 或 npm
    if ! command -v pnpm &>/dev/null && ! command -v npm &>/dev/null; then
        echo -e "${RED}✗ 缺少包管理器: 请安装 pnpm 或 npm${NC}"
        exit 1
    fi

    # 检查 air（可选，用于后端热重载）
    if ! command -v air &>/dev/null; then
        echo -e "${YELLOW}⚠ 未安装 air，将使用 go run 模式（无热重载）${NC}"
        echo "  安装: go install github.com/cosmtrek/air@latest"
        echo ""
    fi

    # 检查前端依赖
    if [ ! -d "${WEB_DIR}/node_modules" ]; then
        echo -e "${YELLOW}📦 正在安装前端依赖...${NC}"
        (cd "${WEB_DIR}" && install_frontend_deps)
        echo ""
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

# 清理子进程
cleanup() {
    echo ""
    echo -e "${YELLOW}正在停止服务...${NC}"

    for pid in ${BACKEND_PID} ${FRONTEND_PID}; do
        if [ "${pid}" -ne 0 ] && kill -0 "${pid}" 2>/dev/null; then
            kill "${pid}" 2>/dev/null || true
            wait "${pid}" 2>/dev/null || true
        fi
    done

    rm -f "${PID_DIR}/backend.pid" "${PID_DIR}/frontend.pid"
    echo -e "${GREEN}所有服务已停止${NC}"
    exit 0
}

# 注册信号捕获（Ctrl+C / SIGTERM）
trap cleanup INT TERM

cmd_start() {
    print_banner
    check_deps

    # ── 清理旧编译产物（避免 air 使用过期二进制） ──
    local tmp_bin="${SERVER_DIR}/tmp/main"
    if [ -f "${tmp_bin}" ]; then
        rm -f "${tmp_bin}"
        echo -e "${YELLOW}  🧹 已清理旧编译产物: tmp/main${NC}"
    fi

    # ── 启动后端 ──────────────────────────────────────
    echo -e "${BLUE}▶ 启动后端服务... (端口: ${BACKEND_PORT})${NC}"
    cd "${SERVER_DIR}"

    if command -v air &>/dev/null; then
        MAGICMAIL_ENV=dev air &
        BACKEND_PID=$!
        echo -e "${GREEN}  ✅ 后端已启动 (air 热重载, PID: ${BACKEND_PID})${NC}"
    else
        MAGICMAIL_ENV=dev go run -tags dev . &
        BACKEND_PID=$!
        echo -e "${GREEN}  ✅ 后端已启动 (go run, PID: ${BACKEND_PID})${NC}"
    fi

    echo "${BACKEND_PID}" >"${PID_DIR}/backend.pid"

    # ── 启动前端 ──────────────────────────────────────
    echo -e "${BLUE}▶ 启动前端服务... (端口: ${FRONTEND_PORT})${NC}"
    cd "${WEB_DIR}"

    local pkg_mgr
    pkg_mgr="$(get_pkg_manager)"

    if [ "${pkg_mgr}" = "pnpm" ]; then
        pnpm dev &
    else
        npm run dev &
    fi
    FRONTEND_PID=$!
    echo -e "${GREEN}  ✅ 前端已启动 (Vite dev server, PID: ${FRONTEND_PID})${NC}"
    echo "${FRONTEND_PID}" >"${PID_DIR}/frontend.pid"

    # ── 就绪信息 ──────────────────────────────────────
    echo ""
    echo -e "${GREEN}═══════════════════════════════════════${NC}"
    echo -e "  ${GREEN}开发环境就绪！按 Ctrl+C 停止所有服务${NC}"
    echo ""
    echo -e "  ${CYAN}前端地址:${NC}   http://localhost:${FRONTEND_PORT}"
    echo -e "  ${CYAN}后端地址:${NC}   http://localhost:${BACKEND_PORT}"
    echo -e "  ${CYAN}健康检查:${NC}   http://localhost:${BACKEND_PORT}/health"
    echo ""
    echo -e "  ${BOLD}默认登录凭据:${NC}"
    echo -e "    用户名: ${YELLOW}${DEV_USERNAME}${NC}"
    echo -e "    密码:   ${YELLOW}${DEV_PASSWORD}${NC}"
    echo -e "${GREEN}═══════════════════════════════════════${NC}"
    echo ""

    # 前台阻塞，等待任意子进程结束
    wait -n 2>/dev/null || wait

    # 如果某个进程意外退出，停止另一个
    echo -e "${YELLOW}检测到某个服务已退出，正在停止其他服务...${NC}"
    cleanup
}

cmd_stop() {
    local stopped=0

    for service in backend frontend; do
        local pid_file="${PID_DIR}/${service}.pid"
        if [ -f "${pid_file}" ]; then
            local pid
            pid=$(cat "${pid_file}")
            if kill -0 "${pid}" 2>/dev/null; then
                kill "${pid}" 2>/dev/null || true
                wait "${pid}" 2>/dev/null || true
                echo -e "${GREEN}✅ ${service} 已停止 (PID: ${pid})${NC}"
                ((stopped++)) || true
            else
                echo -e "${YELLOW}○ ${service} 进程不存在${NC}"
            fi
            rm -f "${pid_file}"
        fi
    done

    if [ "${stopped}" -eq 0 ]; then
        echo -e "${YELLOW}没有运行中的服务${NC}"
    else
        echo ""
        echo -e "${GREEN}所有服务已停止${NC}"
    fi
}

cmd_status() {
    print_banner
    echo -e "${CYAN}服务状态:${NC}\n"

    local all_running=true

    for service in backend frontend; do
        local pid_file="${PID_DIR}/${service}.pid"
        if [ -f "${pid_file}" ]; then
            local pid port name
            pid=$(cat "${pid_file}")

            if [ "${service}" = "backend" ]; then
                port="${BACKEND_PORT}"
                name="Go Server (Fiber)"
            else
                port="${FRONTEND_PORT}"
                name="Vue Dev Server (Vite)"
            fi

            if kill -0 "${pid}" 2>/dev/null; then
                echo -e "  ${GREEN}● ${name}${NC}"
                echo -e "    PID: ${pid} | 端口: ${port}"
                echo -e "    地址: http://localhost:${port}"
            else
                echo -e "  ${RED}○ ${name} (已停止)${NC}"
                all_running=false
            fi
        else
            if [ "${service}" = "backend" ]; then
                echo -e "  ${RED}○ Go Server (未启动)${NC}"
            else
                echo -e "  ${RED}○ Vue Dev Server (未启动)${NC}"
            fi
            all_running=false
        fi
        echo ""
    done

    if "${all_running}"; then
        echo -e "${GREEN}所有服务运行中 ✓${NC}"
    else
        echo -e "${YELLOW}部分服务未运行${NC}"
    fi
}

cmd_help() {
    print_banner
    cat <<EOF
用法: ./scripts/dev.sh [命令]

命令:
  start         启动前后端开发服务器（默认，前台运行）
  stop          停止所有开发服务
  status        查看服务状态
  help          显示帮助信息

示例:
  ./scripts/dev.sh              # 启动开发环境（前台运行，Ctrl+C 退出）
  ./scripts/dev.sh start        # 同上
  ./scripts/dev.sh stop         # 停止服务
  ./scripts/dev.sh status       # 查看状态

说明:
  - 默认前台运行，日志实时输出到终端
  - 按 Ctrl+C 可同时停止前后端服务
  - 开发环境自动设置 MAGICMAIL_ENV=dev
  - 首次启动会自动创建默认管理员账号 (admin / admin123)
  - 生产构建请使用: ./scripts/build.sh [GOOS] [GOARCH]
EOF
}

# 主入口
main() {
    local cmd="${1:-start}"

    case "${cmd}" in
    start)
        check_deps
        cmd_start
        ;;
    stop)
        cmd_stop
        ;;
    status | st)
        cmd_status
        ;;
    help | --help | -h)
        cmd_help
        ;;
    *)
        echo -e "${RED}未知命令: ${cmd}${NC}"
        echo "运行 './scripts/dev.sh help' 查看帮助"
        exit 1
        ;;
    esac
}

main "$@"
