# dev 分支构建校验环境（.cnb.yml 中 build-verify 流水线使用）
#
# 需要同时具备前端（Node + pnpm）与后端（Go）工具链，外加飞牛打包工具 fnpack
# 和架构校验用的 file。依赖全部预装，避免每次构建重复下载。

# 复用官方 Node 镜像的运行时，避免自己下载 Node 造成版本漂移
FROM node:22-bookworm AS node

FROM golang:1.25-bookworm

COPY --from=node /usr/local/ /usr/local/

# web/package.json 没有声明 packageManager，必须显式锁定 pnpm 大版本（与 CI 的 10 对齐）
RUN corepack enable \
    && corepack prepare pnpm@10 --activate \
    && node -v && pnpm -v && go version

# fnpack：飞牛 fnOS 应用打包工具（版本与 .github/workflows/release.yml 保持一致）
ARG FNPACK_VERSION=1.2.1
RUN curl -fsSL "https://static2.fnnas.com/fnpack/fnpack-${FNPACK_VERSION}-linux-amd64" \
      -o /usr/local/bin/fnpack \
    && chmod +x /usr/local/bin/fnpack

# file：校验编译产物的 CPU 架构，防止架构装错
RUN apt-get update \
    && apt-get install -y --no-install-recommends file \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /workspace
