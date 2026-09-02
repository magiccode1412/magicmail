# CNB 环境变量详解

## 声明和导入

```yaml
- env:                           # Pipeline/Job 级
    MY_VAR: value
  imports:                       # 导入密钥仓库（通过 https 绝对路径引用独立密钥仓库）
    - https://cnb.cool/<org>/<secret-repo>/-/blob/main/secrets.yml
    - $NEXT_FILE                 # 前面文件的变量对后面有效
```

`env` 与 `imports` key 冲突时，`env` 优先。

> **密钥仓库引用方式**：敏感信息（token、密码、webhook url 等）应存放于独立的密钥仓库，通过 https 绝对路径引用：`https://cnb.cool/<org>/<secret-repo>/-/blob/main/<file>.yml`。如遇权限校验问题可参考 [文件引用文档](https://docs.cnb.cool/zh/build/file-reference.html)。

---

## 导出和传递（跨 Stage）

### 方式 1：从脚本输出解析

```yaml
- name: set-version
  script: echo "##[set-output VERSION=1.0.0]"
  exports:
    VERSION: MY_VERSION          # from-key: to-key
```

### 方式 2：从执行结果导出

```yaml
- name: run-script
  script: echo -n "hello"
  exports:
    info: SCRIPT_OUTPUT          # info/code/stdout/stderr/skip
```

### 方式 3：从内置任务导出

```yaml
- name: release
  type: git:release
  options: { tag: v1.0.0 }
  exports:
    version: RELEASE_VERSION
```

---

## set-output 编码

`##[set-output key=value]` 支持以下编码前缀（用于含换行等特殊字符的值）：

- `##[set-output key=base64,<base64编码值>]`
- `##[set-output key=escape<转义值>]`

变量值上限 **100KiB**。

---

## 变量替换

以下字段支持 `$VAR` 替换：

- `env`、`imports`
- `options` / `optionsFrom`
- `settings` / `settingsFrom`
- `docker`（image / build / volumes）
- `stage.image`
- `ifModify`、`name`
- `lock.key`、`allowFailure`
- `runner.tags`

用 `\$` 阻止替换。

---

## 内置变量完整列表

> 所有以 `CNB_` 开头的环境变量均为系统内置只读变量，用户不可自定义同名变量。
> 完整在线文档：${CNB_WEB_PROTOCOL:-https}://docs.${CNB_WEB_HOST:-cnb.cool}/zh/build/build-in-env.md

### 基础变量（所有事件可用）

| 变量 | 说明 |
|------|------|
| `CNB` | CNB 标识，值为 `true` |
| `CNB_WEB_PROTOCOL` | Web 使用的协议 (http/https) |
| `CNB_WEB_HOST` | Web 使用的 HOST |
| `CNB_WEB_ENDPOINT` | Web 完整地址 |
| `CNB_API_ENDPOINT` | API 完整地址 |
| `CNB_GROUP_SLUG` | 仓库所属父组织路径 |
| `CNB_GROUP_SLUG_LOWERCASE` | 父组织路径小写格式 |
| `CNB_GROUP_ID` | 仓库所属父组织 ID |
| `CNB_ROOT_SLUG` | 仓库所属根组织路径 |
| `CNB_ROOT_SLUG_LOWERCASE` | 根组织路径小写格式 |
| `CNB_ROOT_ID` | 仓库所属根组织 ID |
| `CNB_EVENT` | 触发构建的事件名称 |
| `CNB_EVENT_URL` | 事件相关链接 |
| `CNB_BRANCH` | 分支名或 Tag 名 |
| `CNB_BRANCH_SHA` | 分支最近提交的 SHA |
| `CNB_DEFAULT_BRANCH` | 仓库默认分支 |
| `CNB_TOKEN_USER_NAME` | 临时令牌对应的用户名，固定为 `cnb` |
| `CNB_TOKEN` | 流水线运行期临时令牌，结束后自动销毁 |
| `CNB_IS_CRONEVENT` | 是否为定时任务事件 |
| `CNB_DOCKER_REGISTRY` | 制品库 Docker 源地址 |
| `CNB_DOCKER_MODEL_REGISTRY` | 制品库 Docker Model 源地址 |
| `CNB_HELM_REGISTRY` | 制品库 Helm 源地址 |
| `CNB_HAS_LFS_FILES` | 是否存在 LFS 文件 |

### 提交类变量

| 变量 | 说明 | 可用事件 |
|------|------|----------|
| `CNB_BEFORE_SHA` | 分支推送前最近一次提交的 SHA | push, commit.add, branch.create |
| `CNB_COMMIT` | 构建对应的代码 SHA | 所有 |
| `CNB_COMMIT_SHORT` | CNB_COMMIT 前 8 位字符 | 所有 |
| `CNB_COMMIT_MESSAGE` | 提交信息 | 所有 |
| `CNB_COMMIT_MESSAGE_TITLE` | 提交信息标题（首行） | 所有 |
| `CNB_COMMITTER` | 提交者 | 所有 |
| `CNB_COMMITTER_EMAIL` | 提交者邮箱 | 所有 |
| `CNB_NEW_COMMITS_COUNT` | 新增提交数量 | commit.add |
| `CNB_IS_TAG` | 是否为 Tag 构建 | 所有 |
| `CNB_TAG_MESSAGE` | Tag 消息 | Tag 事件 |
| `CNB_TAG_RELEASE_TITLE` | Release 标题 | Tag 事件且存在对应 Release |
| `CNB_TAG_RELEASE_DESC` | Release 描述 | Tag 事件且存在对应 Release |
| `CNB_TAG_IS_RELEASE` | Tag 是否有对应 Release | 所有 |
| `CNB_TAG_IS_PRE_RELEASE` | 是否为预发布 Release | 所有 |
| `CNB_IS_NEW_BRANCH` | 是否为新创建的分支 | 所有 |
| `CNB_IS_NEW_BRANCH_WITH_UPDATE` | 是否为新分支且带新提交 | 所有 |

### 仓库类变量

| 变量 | 说明 | 可用事件 |
|------|------|----------|
| `CNB_REPO_SLUG` | 仓库路径 | 所有 |
| `CNB_REPO_SLUG_LOWERCASE` | 仓库路径小写格式 | 所有 |
| `CNB_REPO_NAME` | 仓库名称 | 所有 |
| `CNB_REPO_NAME_LOWERCASE` | 仓库名称小写格式 | 所有 |
| `CNB_REPO_ID` | 仓库 ID | 所有 |
| `CNB_REPO_URL_HTTPS` | 仓库 HTTPS 地址 | 所有 |
| `CNB_FORK_FROM_REPO_SLUG` | Fork 仓库的源仓库路径 | Fork 仓库 |

### 构建类变量

| 变量 | 说明 | 可用事件 |
|------|------|----------|
| `CNB_BUILD_ID` | 构建流水号，全局唯一 | 所有 |
| `CNB_BUILD_WEB_URL` | 构建日志地址 | 所有 |
| `CNB_BUILD_START_TIME` | 构建开始时间（UTC 格式） | 所有 |
| `CNB_BUILD_USER` | 触发者用户名 | 所有 |
| `CNB_BUILD_USER_NICKNAME` | 触发者昵称 | 所有 |
| `CNB_BUILD_USER_EMAIL` | 触发者邮箱 | 所有 |
| `CNB_BUILD_USER_ID` | 触发者 ID | 所有 |
| `CNB_BUILD_USER_NPC_SLUG` | NPC 所属仓库路径 | NPC 触发 |
| `CNB_BUILD_USER_NPC_NAME` | NPC 角色名 | NPC 触发 |
| `CNB_BUILD_STAGE_NAME` | Stage 名称 | 所有 |
| `CNB_BUILD_JOB_NAME` | Job 名称 | 所有 |
| `CNB_BUILD_JOB_KEY` | Job Key，同 Stage 下唯一 | 所有 |
| `CNB_BUILD_WORKSPACE` | 工作空间根目录 | 所有 |
| `CNB_BUILD_FAILED_MSG` | 构建失败错误信息 | failStages 中可用 |
| `CNB_BUILD_FAILED_STAGE_NAME` | 失败的 Stage 名称 | failStages 中可用 |
| `CNB_PIPELINE_NAME` | Pipeline 的 `name` 字段，未声明时为空 | 所有 |
| `CNB_PIPELINE_KEY` | Pipeline 索引 Key；对象形式下通常为流水线 key，数组形式下为自动索引 | 所有 |
| `CNB_PIPELINE_ID` | Pipeline ID，全局唯一 | 所有 |
| `CNB_PIPELINE_DOCKER_IMAGE` | Pipeline 使用的 Docker 镜像 | 所有 |
| `CNB_PIPELINE_STATUS` | 流水线构建状态 | endStages 中可用 |
| `CNB_PIPELINE_MAX_RUN_TIME` | 流水线最大运行时间（毫秒） | 所有 |
| `CNB_RUNNER_IP` | Runner IP | 所有 |
| `CNB_CPUS` | 可用 CPU 核数 | 所有 |
| `CNB_MEMORY` | 可用内存大小（GiB） | 所有 |
| `CNB_IS_RETRY` | 是否由 rebuild 触发 | 所有 |

### 合并类变量

| 变量 | 说明 | 可用事件 |
|------|------|----------|
| `CNB_PULL_REQUEST` | 是否为 PR 触发 | pull_request, pull_request.update, pull_request.target |
| `CNB_PULL_REQUEST_LIKE` | 是否为合并类事件触发 | 合并类事件 |
| `CNB_PULL_REQUEST_PROPOSER` | PR 提出者名称 | 合并类事件 |
| `CNB_PULL_REQUEST_TITLE` | PR 标题 | 合并类事件 |
| `CNB_PULL_REQUEST_DESCRIPTION` | PR 描述 | 合并类事件 |
| `CNB_PULL_REQUEST_BRANCH` | PR 源分支名称 | 合并类事件 |
| `CNB_PULL_REQUEST_SHA` | PR 源分支最新 SHA | 合并类事件 |
| `CNB_PULL_REQUEST_TARGET_SHA` | PR 目标分支最新 SHA | 合并类事件 |
| `CNB_PULL_REQUEST_MERGE_SHA` | PR 合并后 SHA | 合并类事件 |
| `CNB_PULL_REQUEST_SLUG` | 源仓库路径 | 合并类事件 |
| `CNB_PULL_REQUEST_ACTION` | PR 操作类型 | 合并类事件 |
| `CNB_PULL_REQUEST_ID` | PR 全局唯一 ID | 合并类事件 |
| `CNB_PULL_REQUEST_IID` | PR 仓库编号 | 合并类事件 |
| `CNB_PULL_REQUEST_REVIEWERS` | 评审人列表 | 合并类事件 |
| `CNB_PULL_REQUEST_REVIEW_STATE` | 评审状态 | 合并类事件 |
| `CNB_REVIEW_REVIEWED_BY` | 同意评审人列表 | 合并类事件 |
| `CNB_REVIEW_LAST_REVIEWED_BY` | 最后同意评审人 | 合并类事件 |
| `CNB_PULL_REQUEST_IS_WIP` | PR 是否为 WIP | 合并类事件 |

### 云原生开发类变量

| 变量 | 说明 | 可用事件 |
|------|------|----------|
| `CNB_VSCODE_WEB_URL` | 云原生开发地址 | 仅声明了 `services: vscode` 时 |
| `CNB_VSCODE_MAX_RUN_TIME` | 云原生开发最大运行时间（毫秒） | 仅声明了 `services: vscode` 时 |

### Issue 类变量

| 变量 | 说明 | 可用事件 |
|------|------|----------|
| `CNB_ISSUE_ID` | Issue 全局唯一 ID | issue.* 事件 |
| `CNB_ISSUE_IID` | Issue 仓库编号 | issue.* 事件 |
| `CNB_ISSUE_TITLE` | Issue 标题 | issue.* 事件 |
| `CNB_ISSUE_DESCRIPTION` | Issue 描述 | issue.* 事件 |
| `CNB_ISSUE_OWNER` | Issue 作者用户名 | issue.* 事件 |
| `CNB_ISSUE_STATE` | Issue 状态 | issue.* 事件 |
| `CNB_ISSUE_IS_RESOLVED` | Issue 是否已解决 | issue.* 事件 |
| `CNB_ISSUE_ASSIGNEES` | Issue 处理人列表 | issue.* 事件 |
| `CNB_ISSUE_LABELS` | Issue 标签列表 | issue.* 事件 |
| `CNB_ISSUE_PRIORITY` | Issue 优先级 | issue.* 事件 |

### 评论类变量

| 变量 | 说明 | 可用事件 |
|------|------|----------|
| `CNB_COMMENT_ID` | 评论全局唯一 ID | 评论事件 |
| `CNB_COMMENT_BODY` | 评论内容 | 评论事件 |
| `CNB_COMMENT_TYPE` | 评论类型 | 评论事件 |
| `CNB_COMMENT_FILE_PATH` | 评论所在文件 | PR 代码评审评论 |
| `CNB_COMMENT_RANGE` | 评论所在代码行 | PR 代码评审评论 |
| `CNB_REVIEW_ID` | 评审 ID | PR 代码评审评论 |
| `CNB_REVIEW_DESCRIPTION` | 评审描述 | PR 代码评审 |

### NPC 类变量

| 变量 | 说明 | 可用事件 |
|------|------|----------|
| `CNB_NPC_SLUG` | NPC 所属仓库路径 | NPC 事件 |
| `CNB_NPC_SLUG_LOWERCASE` | NPC 所属仓库路径（小写） | NPC 事件 |
| `CNB_NPC_NAME` | NPC 角色名 | NPC 事件 |
| `CNB_NPC_SHA` | NPC 仓库默认分支最新 SHA | NPC 事件 |
| `CNB_NPC_PROMPT` | NPC 角色提示词 | NPC 事件 |
| `CNB_NPC_SLOGAN` | NPC 角色口号 | NPC 事件 |
| `CNB_NPC_AVATAR` | NPC 角色头像地址 | NPC 事件 |
| `CNB_NPC_ENABLE_THINKING` | NPC 是否开启思考 | NPC 事件 |
| `CNB_NPC_ENABLE_WORKMODE` | NPC 是否开启工作模式 | NPC 事件 |
| `CNB_NPC_TRIGGER_CONTENT` | 触发 NPC 的内容 | NPC 事件 |

---

## 变量命名规则

- 只能包含字母、数字、下划线
- 不能以数字开头
- 大小写敏感
- **不得使用 `CNB_` 前缀**：所有 `CNB_` 开头的变量为系统内置只读变量，用户自定义变量使用其他前缀

> **在线文档**：${CNB_WEB_PROTOCOL:-https}://docs.${CNB_WEB_HOST:-cnb.cool}/zh/build/env.md
