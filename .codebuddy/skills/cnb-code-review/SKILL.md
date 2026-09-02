---
name: cnb-code-review
description: PR 代码评审，检查安全漏洞、Bug 和代码质量
---

# PR 代码评审

发现安全漏洞、Bug、代码质量和性能问题，并通过 Review API 发送行级评论。

## 评审步骤

### 1. 获取 PR 变更

使用 PR diff 技能（cnb-pr-diff）获取变更，按其「过滤文件」排除非代码文件。

**限制**：最多评审 30 个文件，diff 最大 100000 字符（超出截断）。

### 2. 分析代码

**评审重点**（按优先级降序）：

1. **安全漏洞**：注入、XSS、CSRF、硬编码凭证、敏感信息泄露、不安全加密
2. **潜在 Bug**：空指针、越界、并发、资源泄漏、异常处理不当
3. **代码质量**：可读性、命名、重复
4. **性能问题**：N+1 查询、低效算法、不必要 I/O

**评审原则**：
- 只评审新增代码（diff 中 `+` 开头的行，不含 `+++` 文件头）；删除、上下文及未修改的代码一律不评
- **忽略**：EOF 换行符、行尾空格、缩进风格（交 linter 处理）

**行号与上下文**：
- 新增代码的行号优先由 hunk 头 `@@ -a,b +c,d @@` 中的 `+c` 推算，`-U5` 已带必要上下文。
- 仅当 diff 不足以理解变更或无法推算行号时，才读源文件，按「理解变更所需的最小上下文」控制范围与次数，避免整文件通读与反复回读。

### 3. 输出评审结果

**分析完全部文件后，一次性输出完整 JSON**（不要逐文件、逐条生成后再拼接），再进入第 4 步统一发送。

**严格输出以下 JSON**：

```json
{
  "status": "passed | needs_modification | critical",
  "issues": [
    {
      "severity": "critical | warning | info",
      "file": "文件路径",
      "start_line": 起始行号, "end_line": 结束行号,
      "problem": "问题描述", "suggestion": "修复建议"
    }
  ]
}
```

**字段说明**：
- `status`：`passed` 通过 / `needs_modification` 需修改 / `critical` 存在严重问题
- `severity`：`critical` 严重（安全漏洞、崩溃 Bug）/ `warning` 潜在问题 / `info` 改进建议
- `start_line` / `end_line`：问题代码行号范围（新文件行号）
- `problem` / `suggestion`：Markdown 格式，用 \`代码\` 标记标识符、代码块展示修复示例
- **无问题时**：`issues` 为空数组 `[]`

### 4. 发送评审评论

仓库名与 PR 编号分别从 `CNB_REPO_SLUG`、`CNB_PULL_REQUEST_IID` 环境变量获取。

**有问题时**——发送行级评审评论：

```bash
cnb pulls post-pull-review \
  --repo "$CNB_REPO_SLUG" \
  --number "$CNB_PULL_REQUEST_IID" \
  --event comment \
  --body "**评审结果**: 需要修改" \
  --comments-path "文件路径" \
  --comments-start-line 起始行号 \
  --comments-start-side right \
  --comments-end-line 结束行号 \
  --comments-end-side right \
  --comments-subject-type line \
  --comments-body '问题: ...
建议: ...'
```

**无问题时**——发送总结评论：

```bash
cnb pulls comment --body '**评审结果**: 通过

代码质量良好，未发现明显问题。'
```

## 约束速查

### 必须
- 评审结果必须是有效 JSON
- 行号必须对应 diff **新文件行号**（右侧），推算方法见「行号与上下文」
- 每条评论包含问题原因与修复建议
- 一次性输出完整 JSON 后统一发送，不要逐条生成

### 限制
- 最多 10 条评论，按严重程度优先（critical > warning > info）
- 控制整体轮次：一次拉取 diff → 集中分析 → 一次输出 → 一次发送
- 不做任何轮询或 `sleep` 等待操作（如循环 `check-status` 等 CI 结果、`sleep` 重试等）
