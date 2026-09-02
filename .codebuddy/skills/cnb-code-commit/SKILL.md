---
name: cnb-code-commit
description: 编写代码、提交并创建 PR
---

# 自动编码并提交推送

按场景选择流程：
1. 新建分支 → 编写代码 → 推送 → 创建 PR
2. 切换到 PR 源分支 → 编写代码 → 推送更新 PR

## 步骤

### 1. 理解需求

明确要修改的文件、功能/Bug 描述、目标分支（PR base）；需要时读取工作区代码，或获取 PR diff 了解变更。

### 2. 判断场景并准备分支

- **PR 评论触发**（`CNB_PULL_REQUEST_IID` 存在）：在该源分支上继续修改
  ```bash
  git checkout "$CNB_PULL_REQUEST_BRANCH"
  git pull origin "$CNB_PULL_REQUEST_BRANCH"
  ```
- **ISSUE 评论触发且 PR 已存在**：切换到 PR 源分支继续开发
  ```bash
  git checkout <PR 源分支名> && git pull
  ```
- **ISSUE 评论触发且 PR 不存在**：基于目标分支新建分支 `auto/{关键词1}-{关键词2}-{4位随机后缀}`
  ```bash
  git checkout -b auto/{关键词1}-{关键词2}-$(openssl rand -hex 2)
  ```
  > ⚠️ **分支命名规则**：
  > - `{关键词}`：从 ISSUE 标题/描述提取 **2 个核心关键词**（名词或动词），短横线连接，每个不超过 2 个英文单词
  > - `{随机后缀}`：4 位十六进制，用 `$(openssl rand -hex 2)` 生成
  > - **长度**：`auto/` 之后不超过 20 个字符
  > - **示例**：`auto/optimize-skill-3a7f`、`auto/login-error-b2c1`

### 3. 编写代码

在工作区直接修改文件，遵循项目代码风格，确保 import/依赖完整。

**生成/修改 `.cnb.yml` 时**：
- 参照流水线配置技能（cnb-pipeline）的语法文档生成配置
- 提交前必须通过其校验，失败则修复后重新校验

### 4. 验证与提交

```bash
git diff --stat        # 查看变更概览
# 如有构建/lint 命令则执行验证
git add -A
git commit -m "{类型}: {简短描述}"
```

类型：`feat` / `fix` / `refactor` / `docs` / `chore`

### 5. 推送并判断是否需要创建 PR

```bash
git push origin HEAD
```

根据推送输出判断：

- 输出含 `There is already has pull request` → **已有 PR**：跳过创建，评论中告知用户
- 输出含 `Create a pull request` 或 `new branch` → **无 PR**：执行第 6 步创建

> ⚠️ 仅新建分支且确认无 PR 时才执行第 6 步。

**示例 1（已有 PR，跳过创建）：**
```
$ git push origin HEAD
remote:
remote: There is already has pull request for 'auto/optimize-skill-3a7f' by visiting:
remote:
remote: 	https://cnb.cool/cnb/skills/cnb-skill/-/pulls/140
remote:
To https://cnb.cool/cnb/skills/cnb-skill.git
   e3d8f95..639bd55  HEAD -> auto/optimize-skill-3a7f

# 输出含 "There is already has pull request" → 已有 PR，不再创建；评论告知代码已推送到 PR #140
```

**示例 2（无 PR，需要创建）：**
```
$ git push origin HEAD
remote:
remote: Create a pull request for 'auto/optimize-skill-3a7f' to 'main' by visiting:
remote:
remote: 	https://cnb.cool/cnb/skills/cnb-skill/-/compare/main...auto/optimize-skill-3a7f
remote:
To https://cnb.cool/cnb/skills/cnb-skill.git
 * [new branch]      auto/optimize-skill-3a7f -> auto/optimize-skill-3a7f

# 输出含 "Create a pull request" → 无 PR，走第 6 步创建
```

### 6. 创建 PR 即结束

> 本节 PR 规范为默认约定，若项目另有规定，以项目规定为准。

```bash
# 创建后立即返回，不做任何等待
cnb pulls post-pull --repo "$CNB_REPO_SLUG" \
  --base "$CNB_DEFAULT_BRANCH" \
  --head "$(git branch --show-current)" \
  --title "{类型}: {简短描述}" \
  --body '<pr-body>'
```

#### PR 标题规范（默认）

- 采用语义化提交格式，只保留要解决的问题。
- 影响可读性的信息（括号补充、版本号、日期、作者、关联编号等）禁止写入标题。
- 标题禁止出现括号，补充说明写描述区。
- 标题禁止出现分支名（如 `hotfix/1.27.x`），目标分支由 PR base 表达。

#### PR 描述规范

- 需包含 `Ref: #<ISSUE_ID>` 引用。
- 开头写清变更背景、要解决的问题、改动点与验证方式。
- 关联信息和补充说明应该写进描述区，比如 `Ref #xxx`、`cherry-pick #xxx`、版本号、日期、作者等。
- cherry-pick 目标版本写入 body 末尾。

#### 创建 PR 即结束

- `cnb pulls post-pull` 成功后立即返回，不等待/轮询 CI 与评审，失败会自动唤起 NPC。

## 约束速查

### 必须
- PR body 末尾追加 `Ref: #<ISSUE_ID>`（如 `Ref: #11`），否则 Issue 中看不到关联 PR

### 禁止
- PR 不存在时在默认分支上改代码，需新建分支并创建 PR
- `git push --force`（除非用户明确要求；需覆盖时用 `--force-with-lease`）
- 推送或创建 PR 后轮询/等待 CI、评审状态（失败会自动唤起 NPC）
- **合并/关闭 PR**：AI 只负责提交 PR，不执行合并（merge/`cnb pulls merge`）等操作，合并交由人工完成

### 建议
- 先理解项目结构与代码风格，再动手修改；不修改无关文件，推送前确认变更正确
- 需求不明确或范围过大时，先评论确认再执行
- `--body` 用单引号包裹，用**真换行**拼接，避免 `\n` 字面量与 shell 转义
