---
name: cnb-pr-diff
description: 获取 PR 的 diff 变更信息
---

# 获取 PR Diff

获取 PR 代码变更差异，用于代码评审、变更总结等场景。

## 1. 判断场景并获取 SHA

### 场景 A：PR 评论触发（`CNB_PULL_REQUEST_IID` 存在）

直接从环境变量获取：head SHA 取 `CNB_PULL_REQUEST_SHA`，target SHA 取 `CNB_PULL_REQUEST_TARGET_SHA`。变量缺失时通过 API 查询 PR 详情。

> CI 只 clone 触发分支，**PR 另一端的 commit 通常不在本地**，两个场景都需执行下方「确保本地有 commit」。

### 场景 B：Issue 或其他场景引用 PR

1. 从用户输入解析 PR 编号（如 `!45`）
2. 通过 API 查询 PR 详情获取 SHA
3. 确保本地有对应 commit

### API 查询 PR 详情

```bash
# 查询 PR 详情，获取 head/base SHA 与分支 ref
pr_info=$(cnb pulls get-pull --repo "$CNB_REPO_SLUG" --number "${pr_iid}" --verbose)

head_sha=$(echo "$pr_info" | jq -r '.data.head.sha')
target_sha=$(echo "$pr_info" | jq -r '.data.base.sha')
head_ref="refs/pull/${pr_iid}/head"     # PR head 的合并 ref
base_ref=$(echo "$pr_info" | jq -r '.data.base.ref')  # 目标分支 ref，如 refs/heads/main
```

### 确保本地有 commit

```bash
# 0. 校验 SHA 非空：为空说明 API 查询失败或环境变量缺失，直接终止，避免落入兜底误判
[ -n "$head_sha" ]   || { echo "head_sha 为空，PR 查询失败" >&2; exit 1; }
[ -n "$target_sha" ] || { echo "target_sha 为空，PR 查询失败" >&2; exit 1; }

# 1. 先探测，本地已有就不拉。need 用空格分隔字符串以兼容 POSIX sh（勿用 bash 数组）
need=""
git cat-file -e "${head_sha}^{commit}"   2>/dev/null || need="$need $head_sha"
git cat-file -e "${target_sha}^{commit}" 2>/dev/null || need="$need $target_sha"

# 2. 优先按目标分支 ref / PR head ref 拉取：部分服务器未开启 allowAnySHA1InWant，
#    裸 SHA fetch 会被拒绝；ref 拉不到再降级为裸 SHA（探测命令的 2>/dev/null 是预期行为）
if [ -n "$need" ]; then
  git fetch -v --progress --force --no-recurse-submodules origin \
      "$base_ref" "$head_ref" 2>/dev/null \
    || git fetch -v --progress --force --no-recurse-submodules origin $need || true
fi

# 3. target 仍缺失时用 merge-base 兜底作 diff 基线，不要重试等待
if ! git cat-file -e "${target_sha}^{commit}" 2>/dev/null; then
  base=$(git merge-base "$head_sha" HEAD 2>/dev/null)
  # CI clone 的是触发分支，HEAD 可能就是 head_sha，此时 merge-base 会退化成
  # head 自己，三点 diff 变成空且退出码为 0，必须排除
  if [ -n "$base" ] && [ "$base" != "$head_sha" ]; then
    target_sha="$base"
  else
    echo "无法确定 diff 基线，请先拉取目标分支：git fetch -v --progress origin <target_branch>" >&2
  fi
fi
```

**硬性要求**：

**必须**：
- **带 `--progress`**：CI 非 tty 下 git 进度默认不输出（走 stderr），静默时大仓可能被看门狗 SIGKILL，`-v` 替代不了。

**禁止**：
- **屏蔽 fetch 输出**：`git fetch` 勿用 `2>/dev/null`、`| tail`、`> log`（探测命令 `git cat-file -e` 与「ref 探测降级」那次 fetch 除外）。
- **`--depth` / `--deepen`**：会写入 `.git/shallow` 切断祖先链，导致 `git merge-base` 和三点 diff 失败。CI 默认不限制深度，保持一致。
- **`sleep` 轮询等待 fetch**：探测失败直接走 `merge-base` 兜底。
- **`|| git fetch origin` 全量兜底**：大仓代价极高。

## 2. 获取 Diff

### 获取变更文件列表

```bash
git diff --name-only "${target_sha}...${head_sha}"
```

### 获取变更统计

```bash
git diff --stat "${target_sha}...${head_sha}"
```

### 获取完整 Diff

```bash
git diff -U5 "${target_sha}...${head_sha}"
```

### 获取特定文件的 Diff

```bash
git diff -U5 "${target_sha}...${head_sha}" -- path/to/file.ts
```

## 3. 过滤文件

按场景排除无需关注的文件：

```bash
# 常见排除模式
git diff -U5 "${target_sha}...${head_sha}" -- . \
  ':!package-lock.json' ':!yarn.lock' ':!pnpm-lock.yaml' ':!go.sum' ':!*.lock' \
  ':!*.png' ':!*.jpg' ':!*.gif' ':!*.svg' ':!*.ico' \
  ':!*.min.js' ':!*.min.css' ':!*.map' \
  ':!dist/*' ':!build/*' ':!node_modules/*'
```

**可排除的文件类型**：
- 锁文件：`package-lock.json`、`yarn.lock`、`pnpm-lock.yaml`、`go.sum`、`Cargo.lock`
- 图片/字体：`*.png`、`*.jpg`、`*.gif`、`*.svg`、`*.woff`、`*.ttf`
- 二进制/压缩包：`*.exe`、`*.dll`、`*.so`、`*.jar`、`*.pyc`、`*.zip`、`*.tar.gz`、`*.rar`
- 构建产物：`dist/`、`build/`、`out/`、`target/`、`.next/`
- 依赖目录：`node_modules/`、`vendor/`、`.venv/`
- 压缩产物：`*.min.js`、`*.min.css`、`*.map`

## 4. 使用建议

1. **diff 过大**：先 `--name-only` 看文件列表，再按需取具体文件
2. **代码评审**：排除锁文件、图片、构建产物
3. **变更总结**：可保留更多文件，但仍排除二进制与依赖目录
4. **上下文行数**：默认 `-U5`，可按需调整
