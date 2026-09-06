# ============================================================================
# Magicmail Docker 镜像
# 多阶段构建: 前端(Vue+Vite) → Go后端(Embed) → 最终运行镜像
# ============================================================================

# ---- Stage 1: 构建前端 ----
FROM node:20-alpine AS frontend-builder

WORKDIR /app/web

COPY web/package.json web/pnpm-lock.yaml* ./

# 锁定 pnpm 10（与 CI 一致），避免 corepack 拉到 pnpm 11/12
# 触发新的 ERR_PNPM_IGNORED_BUILDS 等破坏性校验；
# esbuild / vue-demi 的安装脚本白名单已在 web/package.json 的
# pnpm.onlyBuiltDependencies 中声明。
RUN corepack enable pnpm && \
    corepack prepare pnpm@10 --activate && \
    pnpm install --frozen-lockfile

COPY web/ .
RUN pnpm build

# ---- Stage 2: 构建 Go 二进制（嵌入前端产物） ----
FROM golang:1.25-alpine AS backend-builder

WORKDIR /app/server

# CGO 编译 SQLite 需要 C 编译器（已改用 modernc.org/sqlite 纯 Go 实现）
# RUN apk add --no-cache gcc musl-dev

# 先复制依赖文件，利用 Docker 缓存
COPY server/go.mod server/go.sum ./
RUN go mod download

# 复制源码和前端产物
COPY server/ .

# 从前端阶段复制构建产物到 embed 路径
COPY --from=frontend-builder /app/server/dist ./embedfs/dist

# 编译（CGO_ENABLED=0 纯静态链接，使用 modernc.org/sqlite 纯 Go 实现）
RUN CGO_ENABLED=0 go build -ldflags="-s -w -X main.isProduction=true" -o /magicmail .

# ---- Stage 3: 最终运行镜像 ----
FROM alpine:3.20

# 安装运行时依赖（su-exec=降权, wget=健康检查, ca-certificates/tzdata=基础）
RUN apk add --no-cache ca-certificates tzdata su-exec wget

# 设置时区（可通过环境变量覆盖）
ENV TZ=Asia/Shanghai

# 创建非 root 用户（固定 UID=1000，与宿主机普通用户匹配）
# 创建 magicmail 用户（UID/GID=1000），入口脚本会在运行时用 su-exec 降权切换到此用户
# 注意：adduser 的 -h 主目录必须已存在，否则报 "/app/data: No such file or directory"
RUN mkdir -p /app/data && \
    addgroup -S -g 1000 magicmail && \
    adduser -S -u 1000 -G magicmail -h /app/data -s /sbin/nologin magicmail

WORKDIR /app

# 从构建阶段复制二进制
COPY --from=backend-builder /magicmail /app/magicmail

# 数据持久化目录（归属在 entrypoint 中动态修正）
RUN mkdir -p /app/data && chown magicmail:magicmail /app/data

# 拷贝入口脚本（负责在容器启动时修正权限并降权运行）
COPY docker-entrypoint.sh /app/docker-entrypoint.sh
RUN chmod +x /app/docker-entrypoint.sh

# 注意：不再写死 USER magicmail，改由 entrypoint 以 root 进入后 chown + su-exec 降权
# 这样 Dockhand 等 NAS 管理器创建的 bind-mount 目录才能被自动修正归属

# 数据库路径、监听端口
ENV MAGICMAIL_DSN=/app/data/magicmail.db

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -q -O /dev/null http://localhost:8080/

ENTRYPOINT ["/app/docker-entrypoint.sh"]
CMD ["/app/magicmail"]
