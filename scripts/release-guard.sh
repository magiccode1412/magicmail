#!/usr/bin/env bash
# ============================================================================
# 发布入口校验（CNB tag_push）
#
# 与 .github/workflows/release.yml 的 guard job 保持同口径：
#   1. tag 必须是 vX.Y.Z（允许 -rc.N 等预发布后缀）
#   2. tag 指向的提交必须在 main 分支上
#
# 不同点：
#   - 非发版 tag（不以 v 开头 / 不符合版本号规范）用退出码 78 结束：
#     CNB 约定 78 = 任务成功但中断当前流水线，既不会标红，也不会继续构建。
#   - tag_push 时工作区只检出这一个 tag，main 的引用和历史 tag 都要手动补拉，
#     否则 merge-base 判断和 changelog 回退都拿不到数据。
#
# 输出（供后续 stage 使用）：
#   version    完整版本号，如 v1.2.0-rc.1
#   prerelease 是否预发布（版本号带 - 后缀）
#   latest     是否标记为最新 Release
# ============================================================================
set -euo pipefail

RELEASE_BRANCH="${RELEASE_BRANCH:-main}"
VERSION="${CNB_BRANCH:-}"

# 非发版 tag：正常退出但中断流水线（退出码 78）
if ! printf '%s' "${VERSION}" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$'; then
  echo "跳过发布：${VERSION} 不是 vX.Y.Z 形式的发版 tag（预发布可用 v1.2.0-rc.1）"
  exit 78
fi

# tag_push 仅检出 tag 本身，main 引用与 tag 列表需要显式拉取。
# 用 CNB_TOKEN 拼 URL，不依赖工作区 origin 是否已注入凭证。
REMOTE="https://${CNB_TOKEN_USER_NAME}:${CNB_TOKEN}@${CNB_WEB_HOST}/${CNB_REPO_SLUG}.git"

# 浅克隆会让下面的 merge-base 判断失真，先尝试补全历史
#（非浅克隆时该命令会报错，这里静默忽略）
git fetch --unshallow "${REMOTE}" >/dev/null 2>&1 || true

git fetch --no-tags --force "${REMOTE}" \
  "+refs/heads/${RELEASE_BRANCH}:refs/remotes/origin/${RELEASE_BRANCH}"

# tag 列表只用于 changelog 回退时定位上一个版本，失败不阻断发布
git fetch --tags --force "${REMOTE}" \
  || echo "::warning::拉取 tag 列表失败，changelog 回退时可能缺少历史 tag"

# 预发布与正式版都要求在 main 上：rc 必须是功能冻结的候选，不能打在 dev 上
if ! git merge-base --is-ancestor HEAD "refs/remotes/origin/${RELEASE_BRANCH}"; then
  echo "::error::${VERSION} 指向的提交 $(git rev-parse --short HEAD) 不在 ${RELEASE_BRANCH} 分支上，拒绝发布"
  exit 1
fi
echo "OK: ${VERSION} @ $(git rev-parse --short HEAD) 位于 ${RELEASE_BRANCH}"

echo "##[set-output version=${VERSION}]"
if printf '%s' "${VERSION}" | grep -q '-'; then
  echo "##[set-output prerelease=true]"
  echo "##[set-output latest=false]"
else
  echo "##[set-output prerelease=false]"
  echo "##[set-output latest=true]"
fi
