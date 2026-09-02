#!/usr/bin/env bash
# ============================================================================
# 构建并推送 Docker 镜像到 cnb 制品库
#
# 由 .cnb.yml 的 web_trigger_docker_dev 手动触发调用：
#     bash ./scripts/docker.sh dev
#
# 参数：
#   $1   镜像标签，默认 dev
#
# 说明：
#   dev 镜像是单架构的（构建机自身架构），供开发自测使用。
#   正式发布的多架构镜像（amd64 + arm64 + manifest）由 $ 分支下的
#   web_trigger_docker_cnb 流水线构建，不走本脚本。
#
# 所需的环境变量均为 cnb 平台内置：
#   CNB_DOCKER_REGISTRY / CNB_REPO_SLUG_LOWERCASE  制品库地址
#   CNB_TOKEN / CNB_TOKEN_USER_NAME                制品库登录凭证
#   CNB_COMMIT_SHORT                               当前提交短 sha
# ============================================================================
set -euo pipefail

TAG="${1:-dev}"

if [ -z "${CNB_DOCKER_REGISTRY:-}" ] || [ -z "${CNB_REPO_SLUG_LOWERCASE:-}" ]; then
  echo "::error::缺少制品库地址变量（CNB_DOCKER_REGISTRY / CNB_REPO_SLUG_LOWERCASE），请确认在 cnb 流水线中运行"
  exit 1
fi
if [ -z "${CNB_TOKEN:-}" ] || [ -z "${CNB_TOKEN_USER_NAME:-}" ]; then
  echo "::error::缺少制品库登录凭证（CNB_TOKEN / CNB_TOKEN_USER_NAME）"
  exit 1
fi

REPO="${CNB_DOCKER_REGISTRY}/${CNB_REPO_SLUG_LOWERCASE}"
COMMIT_SHORT="${CNB_COMMIT_SHORT:-unknown}"

echo "镜像仓库: ${REPO}"
echo "镜像标签: ${TAG} (commit ${COMMIT_SHORT})"

echo "==> 登录制品库"
docker login "${CNB_DOCKER_REGISTRY}" -u "${CNB_TOKEN_USER_NAME}" -p "${CNB_TOKEN}"

echo "==> 构建镜像"
# 除传入的标签外，额外打一个带 commit 短 sha 的标签，便于按提交回溯测试版本
docker build \
  -t "${REPO}:${TAG}" \
  -t "${REPO}:${TAG}-${COMMIT_SHORT}" \
  .

echo "==> 推送镜像"
docker push "${REPO}:${TAG}"
docker push "${REPO}:${TAG}-${COMMIT_SHORT}"

echo "==> 完成"
echo "  ${REPO}:${TAG}"
echo "  ${REPO}:${TAG}-${COMMIT_SHORT}"
