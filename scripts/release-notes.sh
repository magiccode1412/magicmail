#!/usr/bin/env bash
# ============================================================================
# 生成 Release 正文（写入 release-notes.md）
#
# 依赖环境变量：APP_VERSION（由 release-guard.sh 导出）
#
# 优先取 docs/guide/changelog.md 中对应版本的条目。
# 不直接用 git log 的原因：main 只在发版时接收 dev 的合并，历史很短，
# 自动生成只能覆盖到 merge commit，内容严重缺失。
# changelog.md 里没有对应条目时才回退到 git log。
# ============================================================================
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

VERSION="${APP_VERSION:?需要 APP_VERSION}"
BASE="${VERSION%%-*}"          # v1.2.0-rc.1 → v1.2.0
NOTES_FILE="release-notes.md"

NOTES=""
if [ -f docs/guide/changelog.md ]; then
  NOTES="$(awk -v base="${BASE}" '
    BEGIN { want = "## [" base "]" }
    !found && index($0, want) == 1 {
      rest = substr($0, length(want) + 1)
      if (rest == "" || rest ~ /^[ -]/) { found = 1 }
      next
    }
    found && /^## \[/ { exit }
    found { lines[++n] = $0 }
    END {
      while (n > 0 && lines[n] ~ /^[[:space:]]*$/) n--
      for (i = 1; i <= n; i++) print lines[i]
    }
  ' docs/guide/changelog.md)"
fi

# 回退到 git log
if [ -z "${NOTES}" ]; then
  echo "::warning::docs/guide/changelog.md 中未找到 ${BASE} 条目，回退到 git log"
  PREV_TAG="$(git describe --tags --abbrev=0 HEAD^ 2>/dev/null || echo '')"
  if [ -n "${PREV_TAG}" ]; then
    NOTES="$(git log --no-merges --pretty='%h%x09%s' "${PREV_TAG}..HEAD" \
             | awk -F'\t' '{ printf "- %s (%s)\n", $2, $1 }')"
  else
    NOTES="$(git log --no-merges --pretty='%h%x09%s' \
             | awk -F'\t' '{ printf "- %s (%s)\n", $2, $1 }')"
  fi
fi

[ -n "${NOTES}" ] || NOTES="- 本次无详细变更记录"

{
  echo "## Magicmail ${VERSION}"
  echo ""
  echo "### 变更内容"
  echo ""
  printf '%s\n' "${NOTES}"
  echo ""
  echo "### 下载"
  echo ""
  echo "| 平台 | 架构 | 文件 |"
  echo "|------|------|------|"
  echo "| Linux | x86_64 | \`magicmail\` |"
  echo "| Linux | ARM32 (armv7) | \`magicmail-arm\` |"
  echo "| Linux | ARM64 | \`magicmail-arm64\` |"
  echo "| macOS | Intel | \`magicmail-macos-x86_64\` |"
  echo "| macOS | Apple Silicon | \`magicmail-macos-arm64\` |"
  echo "| Windows | x86_64 | \`magicmail.exe\` |"
  echo ""
  echo "### 飞牛 fnOS 应用包"
  echo ""
  echo "| 平台 | 文件 | 说明 |"
  echo "|------|------|------|"
  echo "| 飞牛 fnOS (x86_64) | \`magicmail-${VERSION}-x86.fpk\` | 一键安装到飞牛系统 |"
  echo "| 飞牛 fnOS (ARM64) | \`magicmail-${VERSION}-arm64.fpk\` | 一键安装到飞牛系统 |"
  echo ""
  echo "> 解压后直接运行，无需额外依赖。默认监听 \`http://localhost:8080\`"
} > "${NOTES_FILE}"

echo "==> 已生成 ${NOTES_FILE}"
cat "${NOTES_FILE}"
