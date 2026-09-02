# CNB 内置任务参考

所有内置任务通过 `type` 字段指定，用 `options` 传参，用 `exports` 导出结果。

```yaml
- name: my-task
  type: <type>
  options:
    key: value
  # optionsFrom: options.yml     # 从文件加载 options
  exports:
    output_key: ENV_VAR_NAME
```

---

## 任务一览

| 类型 | 功能 | 关键参数 | 支持的触发事件 |
|------|------|----------|----------------|
| `cnb:resolve` / `cnb:await` | 多 Pipeline 协作（同事件内） | `key`, `data`(resolve) | 所有事件（不可在 endStages/failStages 中使用） |
| `cnb:apply` | 触发同仓库子流水线 | `event`, `configFrom`, `sync` | push, commit.add, branch.create, pull_request.target, pull_request.mergeable, tag_push, pull_request.merged, api_trigger, web_trigger, crontab, tag_deploy |
| `cnb:trigger` | 触发其他仓库流水线 | `slug`, `event`, `branch`, `env`, `sync` | 所有事件 |
| `cnb:read-file` | 读取文件为环境变量 | `filePath` | 所有事件 |
| `cnb:destroy-token` | 销毁 CNB_TOKEN | 无 | 所有事件 |
| `docker:cache` | 构建 Docker 缓存镜像 | `dockerfile`, `by`, `versionBy` | 所有事件 |
| `git:auto-merge` | 自动合并 PR | `mergeType`, `removeSourceBranch` | **仅 pull_request.mergeable** |
| `git:reviewer` | 添加/删除评审人 | `type`, `reviewers`, `count`, `reviewersConfig` | pull_request, pull_request.target, pull_request.update |
| `git:release` | 发布 Release | `tag`, `description`, `descriptionFromFile` | push, commit.add, branch.create, tag_push, pull_request.merged, api_trigger, web_trigger, tag_deploy |
| `git:issue-update` | 更新 Issue 状态/标签 | `state`, `label`, `when`, `fromFile` | 所有事件 |
| `git:pr-update` | 更新 PR 标签/标题 | `label`, `title` | 所有事件 |
| `git:pr-commit-message-preset` | 预设 PR 提交信息 | `message` | pull_request, pull_request.target, pull_request.update, pull_request.mergeable, pull_request.approved, pull_request.changes_requested |
| `testing:coverage` | 单测覆盖率上报 | `pattern`, `lang`, `lines`, `diffLines` | 所有事件（增量覆盖率仅 PR 事件） |
| `artifact:remove-tag` | 删除制品标签 | `name`, `tags`, `type` | push, commit.add, tag_push, tag_deploy, pull_request.merged, api_trigger, web_trigger, crontab, branch.create |
| `tapd:status-update` | 更新 TAPD 状态 | `status`, `type`, `when` | 所有事件 |
| `tapd:comment` | TAPD 评论 | `comment`, `type` | 所有事件 |
| `vscode:go` | 控制云开发环境可用时机 | 无 | **仅 vscode, branch.create, api_trigger, web_trigger** |
| `knowledge:update` | 更新知识库 | `include`, `exclude`, `chunkSize` | 所有事件 |
| `npc:go` | 执行 AI/NPC 任务 | `role`, `systemPrompt`, `userPrompt` | 所有事件 |

> **在线文档**：${CNB_WEB_PROTOCOL:-https}://docs.${CNB_WEB_HOST:-cnb.cool}/zh/build/internal-steps.md

---

## 术语提示（避免望文生义）

- `xxx:yyy` 点分名称是**内置任务类型**（如 `npc:go`、`cnb:apply`、`vscode:go`），不是普通命令，更不是游戏用语；含义以本文档和在线文档为准，不确定时**先查证再回答**，禁止凭字面猜测。
- CNB 语境下 **NPC = 自动化角色**（AI 智能助手，可被 @ 召唤、可在流水线中执行任务），与游戏 Non-Player Character 无关。`npc:go` 即"执行 NPC 任务"的内置任务。

---

## 事件限制详情

部分内置任务对触发事件有严格限制，在不受支持的事件下使用会导致构建失败或无效果：

### `git:auto-merge` — 仅 `pull_request.mergeable`

该任务在 PR 满足合并条件时自动合并，仅支持 `pull_request.mergeable` 事件。在其他事件下使用无意义。

### `vscode:go` — 仅 `vscode`, `branch.create`, `api_trigger`, `web_trigger`

该任务控制云开发环境的可用性，仅在云开发相关事件中有效。

### `git:reviewer` — 仅 PR 事件

仅支持 `pull_request`, `pull_request.target`, `pull_request.update` 事件。这些事件与 PR 上下文关联。

### `git:pr-commit-message-preset` — 仅 PR 事件

仅支持 PR 相关事件：`pull_request`, `pull_request.target`, `pull_request.update`, `pull_request.mergeable`, `pull_request.approved`, `pull_request.changes_requested`。

### `cnb:apply` — 限定事件

仅支持：`push`, `commit.add`, `branch.create`, `pull_request.target`, `pull_request.mergeable`, `tag_push`, `pull_request.merged`, `api_trigger`, `web_trigger`, `crontab`, `tag_deploy`。不支持 Issue、NPC、PR 评论等事件。

### `git:release` — 限定事件

仅支持：`push`, `commit.add`, `branch.create`, `tag_push`, `pull_request.merged`, `api_trigger`, `web_trigger`, `tag_deploy`。

### `artifact:remove-tag` — 限定事件

仅支持：`push`, `commit.add`, `tag_push`, `tag_deploy`, `pull_request.merged`, `api_trigger`, `web_trigger`, `crontab`, `branch.create`。

---

## `npc:go` 详解

在流水线中执行 NPC（自动化角色）任务，是让 AI 在流水线里干活的核心入口。支持所有事件。

**参数**（`options`）：

| 参数 | 说明 |
|------|------|
| `role` | 指定 NPC 的角色名 |
| `systemPrompt` | 系统级提示词，定义 NPC 的角色行为 |
| `userPrompt` | 用户提示词，描述具体要执行的任务（web_trigger 场景可用 `$userPrompt` 变量接收页面输入） |

**运行环境**：默认镜像 `cnbcool/default-npc:latest`，可自定义 Docker 镜像。

**典型场景**：

1. **NPC 事件触发**：在 Issue/PR 评论中 @NPC（事件 `issue.comment@npc` / `pull_request.comment@npc`）触发流水线执行 `npc:go`，NPC 自动回复或处理任务
2. **PR 代码审查**：PR 事件中让 NPC 审查代码变更并给出建议
3. **web_trigger 手动触发**：通过 `userPrompt` 变量接收页面输入的任务
4. **失败流水线 AI 分析**：放 `failStages` 中，构建失败时让 NPC 总结根因并给出解决方案

**注意事项**：

- **工作模式**：NPC 事件评论区勾选"替我上班"开启，NPC 获得代码写权限，可自主提交代码、创建分支、提 PR
- **权限限制**：NPC 的 `CNB_TOKEN` 权限取决于触发用户的角色；Reporter/Guest 无法开启工作模式
- **配置合并**：NPC 所属仓库 `.cnb.yml` 会与系统默认配置**按事件独立合并**

---

## 常用示例

### cnb:trigger（触发其他仓库）

```yaml
- name: trigger-deploy
  type: cnb:trigger
  options:
    slug: org/deploy-repo
    branch: main
    event: api_trigger
    env:
      VERSION: $VERSION
    sync: true                   # 同步等待完成
  exports:
    sn: BUILD_SN
```

### cnb:await / cnb:resolve（多 Pipeline 协作）

```yaml
# Pipeline A：完成后通知
- name: notify-ready
  type: cnb:resolve
  options:
    key: frontend-ready
    data: { version: $VERSION }

# Pipeline B：等待通知
- name: wait-frontend
  type: cnb:await
  options:
    key: frontend-ready
  exports:
    version: FRONTEND_VERSION
```

### git:release（发布 Release）

```yaml
- name: release
  type: git:release
  options:
    tag: v1.0.0
    description: "Release v1.0.0"
    # descriptionFromFile: CHANGELOG.md
  exports:
    version: RELEASE_VERSION
```

### git:auto-merge（自动合并 PR）

```yaml
# 必须在 pull_request.mergeable 事件下使用
- name: auto-merge
  type: git:auto-merge
  options:
    mergeType: squash
    removeSourceBranch: true
```

### testing:coverage（覆盖率上报）

```yaml
- name: coverage
  type: testing:coverage
  options:
    pattern: coverage/lcov.info
    lang: javascript
    lines: 80                    # 总覆盖率阈值
    diffLines: 90                # 增量覆盖率阈值
```

### docker:cache（构建缓存镜像）

```yaml
- name: cache-deps
  type: docker:cache
  options:
    dockerfile: Dockerfile.cache
    by: [package.json]
    versionBy: [package-lock.json]
```

### npc:go（执行 AI/NPC 任务）

场景 1：NPC 事件触发（评论 @NPC 后自动执行）

```yaml
$:
  issue.comment@npc:
    - docker:
        image: cnbcool/default-npc:latest
      stages:
        - name: npc go
          type: npc:go
```

场景 2：PR 代码审查

```yaml
$:
  pull_request:
    - docker:
        image: cnbcool/default-npc:latest
      stages:
        - name: npc go
          type: npc:go
          options:
            role: 代码审查员
            systemPrompt: 你是一个专业的代码审查员，请审查代码变更并给出改进建议
            userPrompt: 审查本次 PR 的代码变更，给出改进建议
```

场景 3：web_trigger 手动触发（`$userPrompt` 接收页面输入）

```yaml
$:
  web_trigger:
    - docker:
        image: cnbcool/default-npc:latest
      stages:
        - name: npc go
          type: npc:go
          options:
            role: 小助手
            systemPrompt: 你是一个助手，负责执行用户指定的任务并回复结果
            userPrompt: $userPrompt
```

场景 4：失败流水线 AI 分析（放 failStages）

```yaml
      failStages:
        - docker:
            image: cnbcool/default-npc:latest
          stages:
            - name: npc go
              type: npc:go
              options:
                systemPrompt: 分析构建失败原因
                userPrompt: 请分析此次构建失败的原因并给出解决方案
```
