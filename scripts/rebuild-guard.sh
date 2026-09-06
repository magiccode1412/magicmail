#!/usr/bin/env bash
# ============================================================================
# 重建入口校验（CNB web_trigger：手动重建已发布版本）
#
# 由 .cnb/release.yml 的 web_trigger_rebuild_release 流水线调用，
# 对应 .cnb/web_trigger.yml 里 main 分支上的「重建最新版本」按钮。
#
# 场景：版本已发布，发现产物有问题，main 上补了修复提交，但不想升版本号 ——
#       用 main 当前代码重新构建，覆盖同名 Release 的产物。
#
# 与 release-guard.sh 的差异：
#   1. 版本号来源不同：优先取按钮输入的 REBUILD_VERSION，留空则取 version.json
#      的 latest（最新版本号），最后回退到远端最新的 v* tag；
#      tag_push 时版本号就是 CNB_BRANCH（tag 名）。
#   2. 校验方向不同：tag_push 校验「tag 指向的提交在 main 上」，
#      这里校验「当前 HEAD 在 main 上」——构建的就是当前分支的代码。
#   3. 不合规即失败：非发版 tag 在 tag_push 下用退出码 78 静默跳过；
#      手动触发是明确意图，任何不合规都直接 exit 1 标红。
#
# 输出（供后续 stage 使用）：
#   version    完整版本号，如 v1.3.0
#   prerelease 是否预发布（版本号带 - 后缀）
#   latest     是否为最新版本（与 version.json 的 latest 一致）
#              重建历史版本时不抢 Latest 标记，避免把最新版本顶掉
# ============================================================================
set -euo pipefail

RELEASE_BRANCH="${RELEASE_BRANCH:-main}"
INPUT_VERSION="${REBUILD_VERSION:-}"

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

# 从 version.json 里取 latest 字段（不依赖 jq，镜像里没装）
LATEST_IN_JSON="$(
  sed -n 's/.*"latest"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' version.json | head -1
)"

# ---------- 1. 解析目标版本号 ----------
if [ -n "${INPUT_VERSION}" ]; then
  VERSION="${INPUT_VERSION}"
  # 允许填 1.3.0，统一成 v1.3.0
  case "${VERSION}" in
    v*) ;;
    *) VERSION="v${VERSION}" ;;
  esac
else
  VERSION="${LATEST_IN_JSON}"
fi

# ---------- 2. 补齐远端引用 ----------
# 用 CNB_TOKEN 拼 URL，不依赖工作区 origin 是否已注入凭证
REMOTE="https://${CNB_TOKEN_USER_NAME}:${CNB_TOKEN}@${CNB_WEB_HOST}/${CNB_REPO_SLUG}.git"

# 浅克隆会让下面的 merge-base 判断失真，先尝试补全历史
#（非浅克隆时该命令会报错，这里静默忽略）
git fetch --unshallow "${REMOTE}" >/dev/null 2>&1 || true

git fetch --no-tags --force "${REMOTE}" \
  "+refs/heads/${RELEASE_BRANCH}:refs/remotes/origin/${RELEASE_BRANCH}"
git fetch --tags --force "${REMOTE}"

# version.json 里没有时，回退到远端最新的 v* tag（优先正式版，全是 rc 才用 rc）
if [ -z "${VERSION}" ]; then
  VERSION="$(git tag --list 'v[0-9]*' --sort=-v:refname | grep -v -- '-' | head -1 || true)"
  [ -n "${VERSION}" ] || VERSION="$(git tag --list 'v[0-9]*' --sort=-v:refname | head -1 || true)"
fi

# ---------- 3. 校验 ----------
if ! printf '%s' "${VERSION}" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$'; then
  echo "::error::版本号 ${VERSION} 不符合 vX.Y.Z 规范（预发布可用 v1.2.0-rc.1）"
  exit 1
fi

# Release 必须挂在已存在的 tag 上：附件按 tag 定位 Release，tag 不存在会上传失败
if ! git rev-parse -q --verify "refs/tags/${VERSION}" >/dev/null; then
  echo "::error::tag ${VERSION} 不存在，重建只能覆盖已发布过的版本（新版本请走正常发版流程）"
  exit 1
fi

# 当前 HEAD 必须在 main 上：确保重建的是已冻结的发布代码，而不是 dev 上的半成品
if ! git merge-base --is-ancestor HEAD "refs/remotes/origin/${RELEASE_BRANCH}"; then
  echo "::error::当前提交 $(git rev-parse --short HEAD) 不在 ${RELEASE_BRANCH} 分支上，拒绝重建"
  exit 1
fi

TAG_COMMIT="$(git rev-parse --short "refs/tags/${VERSION}^{commit}")"
HEAD_COMMIT="$(git rev-parse --short HEAD)"
if [ "${TAG_COMMIT}" = "${HEAD_COMMIT}" ]; then
  echo "::warning::${VERSION} 已经指向当前提交 ${HEAD_COMMIT}，重建后的产物与现网一致"
fi
echo "OK: 用 ${RELEASE_BRANCH} @ ${HEAD_COMMIT} 重建 ${VERSION}（原 tag @ ${TAG_COMMIT}）"
echo "::warning::只重建 CNB 侧产物；GitHub Release 需在 Actions 手动重跑 Build & Release（版本号填 ${VERSION}）"

# ---------- 4. 输出 ----------
echo "##[set-output version=${VERSION}]"
if printf '%s' "${VERSION}" | grep -q '-'; then
  echo "##[set-output prerelease=true]"
  echo "##[set-output latest=false]"
  exit 0
fi

echo "##[set-output prerelease=false]"
if [ "${LATEST_IN_JSON}" = "${VERSION}" ]; then
  echo "##[set-output latest=true]"
else
  echo "::warning::${VERSION} 不是最新版本号（version.json 的 latest 为 ${LATEST_IN_JSON}），重建不会抢占 Latest"
  echo "##[set-output latest=false]"
fi
