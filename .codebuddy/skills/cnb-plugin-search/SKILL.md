---
name: cnb-plugin-search
description: 从 CNB 插件市场搜索匹配插件并生成流水线插件配置
---

# CNB 插件搜索

> 下方 URL 中的 `${CNB_WEB_PROTOCOL:-https}`、`${CNB_WEB_HOST:-cnb.cool}` 为环境变量，使用前先 `echo` 获取实际值再拼接。

按用户需求在 CNB 插件市场搜索匹配的插件，查阅文档，生成正确的 `.cnb.yml` 配置片段；常见需求包括：发送通知、代码检查、构建部署、发布包等。

## 何时使用本 Skill

CNB 插件即封装好的 Docker 镜像 + settings 参数接口，能覆盖绝大多数 CI/CD 场景。以下场景**必须优先用本 skill 搜索插件**：

- **消息通知**：需要向 IM 群聊或 Webhook 推送构建状态、结果通知
- **代码质量**：需要对代码进行 lint、格式化、AI 评审，或检查 commit/PR 规范
- **构建与打包**：需要构建/合并 Docker 镜像，或使用市场现成的构建工具插件（Maven、Gradle、Kaniko 等；一般代码构建测试优先脚本任务，见下方「插件任务 vs 脚本任务」）
- **发布与版本**：需要自动打 tag、生成 changelog、语义化发布
- **部署与运维**：需要部署到容器平台/Serverless/边缘节点，或执行 SSH/kubectl 命令
- **安全与扫描**：需要依赖漏洞扫描或安全审计
- **Git 协作**：需要自动化创建 PR 或管理 Git 操作
- **文件与存储**：需要上传/下载 COS 文件或替换环境变量
- **微信小程序 CI**：需要微信小程序 CI 构建上传

> **核心规则**：用插件 `image` + `settings` 生成配置。
> 仅确认无匹配插件或需求极简时，才 fallback 到 script 手写方案。
> **用法必须查证**：插件具体用法（settings 参数名、类型、必填性）以插件 README 为准，**禁止凭记忆生成**——同名插件不同版本参数可能不同，凭记忆极易编错。

## 插件任务 vs 脚本任务

CNB 流水线 `Job` 分脚本任务、插件任务、内置任务三类。选择插件任务还是脚本任务，按官方文档判断：

| 场景 | 推荐 | 原因 |
|------|------|------|
| 消息通知（企微/飞书/钉钉等） | 插件任务 | 功能内聚，配置简单 |
| 通用 CI/CD 操作（发布、部署） | 插件任务 | 市场有成熟方案 |
| 代码质量检查（lint、扫描） | 插件任务 | 可复用 |
| 可复用、可分享的通用功能 | 插件任务 | 可发布到市场 |
| 代码构建和测试 | 脚本任务 | 灵活控制，适配各种构建工具 |
| 自定义业务逻辑 | 脚本任务 | 需完全控制执行流程 |
| 与业务强相关 | 脚本任务 | 不适合复用 |

**快速判断**：有现成插件满足需求 → 插件任务；需灵活执行自定义命令 → 脚本任务；功能可复用分享 → 插件任务。

**两者核心区别**（详见官方文档 `/zh/build/script-vs-plugin.md`）：
- **执行方式**：脚本任务用 `script`/`commands`；插件任务运行镜像内置 `ENTRYPOINT`
- **参数传递**：脚本任务用 `env`/`imports`；插件任务用 `settings`（自动转 `PLUGIN_*` 大写前缀传入容器）
- **自定义环境变量**：插件任务**不传递** `env`/`imports` 变量，须在 `settings`/`args` 中用 `$VAR` 引用（详见下方「生成规则」的变量引用机制）
- **复用性**：脚本任务与业务强相关；插件任务可分享到插件市场

## 模式判定

- **搜索模式**（用户描述需求但未指定插件）→ 走下方「搜索工作流程」
- **直选模式**（用户已明确指定插件名）→ 跳过搜索，直接走「获取文档 → 生成配置」

## 搜索工作流程

### 1. 意图分析

从用户描述中提取关键信息：

- **操作类型**：构建、检查(lint)、测试、部署、发布、通知、审批、扫描、打tag、生成报告
- **语言/框架**：Node.js、Go、Python、Java/Maven/Gradle、C++、安卓、微信小程序、Helm
- **目标平台/服务**：企业微信、飞书、钉钉、TKE、EdgeOne Pages、SCF、COS、TSF、CDN、Cloudflare
- **特殊需求**：AI 评审、安全扫描、提交规范检查、多架构镜像、知识库

### 2. 搜索插件市场

获取最新插件目录：

**默认数据源**（CNB SaaS 插件市场）：
```
URL: ${CNB_WEB_PROTOCOL:-https}://${CNB_WEB_HOST:-cnb.cool}/cnb/plugins/market/-/git/raw/main/plugins/plugins.json
提示：从插件列表中找出与"{用户需求关键词}"最相关的插件，返回 id、name、description、tags、readme、images、source，按相关性排序，列出 Top 5
```

> **私有化 / 自定义数据源**：若无法获取 SaaS 数据源（私有化部署或自建市场），
> 可请用户提供 `plugins.json` 内容（格式与 SaaS 一致）或数据源 URL。

每个插件条目结构（以 wecom-message 为例）：
```json
{
  "id": "tencentcom/wecom-message",
  "name": "wecom-message",
  "description": "企业微信群里发消息的插件，通过机器人，可以向固定的群中推送消息。",
  "tags": ["企业微信", "WeCom", "消息"],
  "mark": "",
  "images": "https://hub.docker.com/r/tencentcom/wecom-message/tags",
  "source": "${CNB_WEB_PROTOCOL:-https}://${CNB_WEB_HOST:-cnb.cool}/cnb/plugins/tencentcom/wecom-message",
  "bugs": "${CNB_WEB_PROTOCOL:-https}://${CNB_WEB_HOST:-cnb.cool}/cnb/plugins/tencentcom/wecom-message/-/issues",
  "logo": "logo.png",
  "readme": "${CNB_WEB_PROTOCOL:-https}://${CNB_WEB_HOST:-cnb.cool}/cnb/plugins/market/-/git/raw/main/plugins/tencentcom/wecom-message/README.md"
}
```

插件条目各字段说明：

- `id`：插件唯一标识，格式为 `组织名/插件名`
- `name`：插件名称，全英文，中划线 `-` 分隔
- `description`：插件描述
- `tags`：标签数组，用于搜索分类
- `mark`：角标：`official`(官方) / `private`(特定组织) / 空(社区)
- `images`：Docker 镜像地址（用于获取 `image` 配置值）
- `source`：源码仓库地址
- `bugs`：问题反馈地址
- `logo`：logo 图片相对地址，可选
- `readme`：README 文档 raw 地址，直接用于获取文档

**匹配策略**（按优先级）：
1. **标签命中** — 用户关键词命中 `tags`
2. **描述命中** — 关键词命中 `name` 或 `description`
3. **语义相近** — 相关操作类型匹配（如"发通知"匹配 tags: notification/message/webhook）

### 3. 推荐确认

向用户展示 Top 3-5 匹配结果：

```
找到以下匹配插件：

1. wecom-message — 企业微信群里发消息的插件
   标签: 企业微信, WeCom, 消息, message
   镜像: tencentcom/wecom-message

2. feishu-message — 飞书群里发消息的插件
   ...

3. webhook — 通过 Webhook 发送构建状态通知
   ...

推荐 #1，是否使用？或选择其他编号。
```

### 4. 获取插件文档

确认插件后，读取插件的 README 文档：

```
URL: <插件 readme URL>
提示：提取使用说明，重点关注 settings 参数列表、每个参数的类型和含义、使用示例中的 .cnb.yml 配置片段
```

> `plugins.json` 中每个条目已包含 `readme` 字段，直接指向市场仓库中缓存的 README.md raw 地址。
> 插件 README 建议包含参数表和使用示例，方便本技能精准提取。参考 [插件 README 编写建议](#插件-readme-编写建议)。
>
> **README 缺失或信息不足时**，依次回退：`source` 源码仓库 README → Docker Hub 镜像页（`images` 字段）→ 通过 `cnb-docs` 查询官方文档。仍无法确认参数时，如实告知用户，只给带占位符的示例配置并注明需查证。

### 5. 生成配置

根据文档中的 `settings` 参数和用法示例，生成正确的 YAML 配置片段。

**生成规则**：

- `image` — 从 `images` 字段提取 Docker 镜像名（不带 tag，使用默认 `latest`）。常见格式：
  - `https://hub.docker.com/r/<org>/<name>/tags` → `<org>/<name>`
  - `https://hub.docker.com/repository/docker/<org>/<name>/tags` → `<org>/<name>`
  - `docker.cnb.cool/.../<name>` → 完整路径
  - 其他格式以 `images` 值中能识别的镜像标识为准，不确定时查看插件 README 确认
  - 如需固定版本可在镜像名后加 `:<版本>`（如 `tencentcom/npm:1.2.3`），规避镜像升级导致的兼容性风险
- `settings` — 按文档参数表填写，**参数名保持小写**（CI 自动转为 `PLUGIN_` + 大写前缀传入插件；编写时无需关心此转换）
- `args` — 插件支持命令行参数时可用 `args:` 数组传递（追加到 ENTRYPOINT，等同 `docker run <image> <args...>`）。
  - 大部分 CNB 插件优先用 `settings`
- **变量引用机制（核心）**：`env` 声明的变量**不会直接传入**插件，须在 `settings` 或 `args` 中用 `$VAR` 引用。
  - `settings` 即参数白名单
  - 系统内置变量（如 `CNB_REPO_SLUG`）始终传入
- **环境变量声明**：敏感信息用 `env` 内联声明，`$VAR` 的 `VAR` 必须是 `env` 中声明的真实变量名

**配置模板**：

```yaml
# 在 .cnb.yml 中使用（用 env 内联声明环境变量）
main:
  push:
    - env:
        MY_TOKEN: <值>      # 内联声明敏感变量
      stages:
        - name: <任务名>
          image: <镜像名>    # 不带 tag 使用 latest
          settings:
            <参数>: <值>
            <参数>: $MY_TOKEN   # MY_TOKEN 必须是上方 env 中声明的变量名
```

### 6. 集成与提醒

生成配置后：

- 若用户上下文已有 `.cnb.yml` → 定位到目标 Stage 位置，合并配置
- 若是独立查询 → 给出完整的 Stage/Job YAML 片段
- **校验（必须）**：生成完整 `.cnb.yml` 后，用流水线技能（cnb-pipeline）的校验器验证，不通过则修复
- 提醒用户：敏感信息用 `env` 内联声明、镜像默认 `latest`（详见[步骤 5](#5-生成配置)）

## 常见场景速查

以下为高频需求与推荐插件的快速参考（**基于当前市场快照，完整列表以 `plugins.json` 为准**）：

**通知**
- 发企业微信通知 → wecom-message（`tencentcom/wecom-message`）
- 发飞书通知 → feishu-message（`tencentcom/feishu-message`）
- 发钉钉通知 → dingtalk-bot-msg（`tencentcom/dingtalk-bot-msg`）
- Webhook 通知 → webhook（`cnbcool/webhook`）

**代码质量**
- AI 代码评审 → code-review（`cnbcool/code-review`）
- Commit 规范检查 → commitlint（`cnbcool/commitlint`）
- PR 标题规范检查 → git-pr-title-lint（`cnbcool/git-pr-title-lint`）
- PR 变更量检查 → git-pr-limit（`cnbcool/git-pr-limit`）

**构建与发布**
- 发布 npm 包 → npm（`tencentcom/npm`）
- Maven 构建 → maven（`tencentcom/maven`）
- Gradle 构建 → gradle（`tencentcom/gradle`）
- 生成 changelog → changelog（`cnbcool/changelog`）
- Semantic Release → semantic-release（`tencentcom/semantic-release`）
- 多架构镜像合并 → manifest（`cnbcool/manifest`）
- 微信小程序 CI → miniprogram-ci（`tencentcom/miniprogram-ci`）

**部署**
- 部署到 TKE → deploy-to-tke（`tencentcom/deploy-to-tke`）
- 部署到 SCF → scf（`tencentcom/tencentyun-scf`）
- 部署 EdgeOne Pages → deploy-eopages（`tencentcom/deploy-eopages`）
- Kaniko 构建镜像 → kaniko（`banzaicloud/drone-kaniko`）
- kubectl 操作 → kubectl（`alpine/kubectl`）
- Terraform 部署 → terraform（`jmccann/drone-terraform`）

**Git & 协作**
- 自动打 tag → git-auto-tag（`cnbcool/git-auto-tag`）
- 创建 PR → create-pr（`cnbcool/create-pr`）
- SSH 远程执行 → ssh（`cnbcool/ssh`）

**文件与工具**
- 上传下载 COS → coscli（`tencentcom/coscli`）
- 环境变量替换 → envsubst（`tencentcom/envsubst`）

> **注**：open-source 类插件多为命令行工具镜像，使用时**可能需配合 `args:`** 传参（参考[步骤 5](#5-生成配置)）。完整列表走[搜索工作流程](#搜索工作流程)。

## 约束速查

### 配置
- 镜像默认不带 tag（即 `latest`）；如需规避升级风险可追加 `:<版本>` 固定版本
- `settings` 即参数白名单：`env` 声明的变量须在 `settings` 中 `$VAR` 引用才能传入插件
- Stage 执行语义：失败通知放 `failStages`，成功通知放 `stages` 最后一个任务，勿把"仅成功才发的通知"放 `endStages`（详见 cnb-pipeline）

### 无匹配时
- 如实告知，插件即 Docker 镜像，可引导用户：
  - 去 Docker Hub 等镜像源找满足需求的镜像，直接在流水线中作插件使用
  - 自行编写插件：插件写法规范见官方文档 `${CNB_WEB_PROTOCOL:-https}://docs.${CNB_WEB_HOST:-cnb.cool}/zh/build/create-plugin.md`（参数设计、Dockerfile、导出变量、发布流程）
  - 贡献到市场：官方文档 `${CNB_WEB_PROTOCOL:-https}://docs.${CNB_WEB_HOST:-cnb.cool}/zh/build/contribute-plugin.md`，市场入口 `${CNB_WEB_PROTOCOL:-https}://${CNB_WEB_HOST:-cnb.cool}/cnb/plugins/market?tabValue=CONTRIBUTING-ov-file`

## 与其他 Skill 的协作

- **被流水线技能（cnb-pipeline）委托调用**（最主要入口）：生成流水线配置需插件任务时，**必须先调用本 skill 搜索市场插件**
- **调用流水线技能（cnb-pipeline）校验**：生成完整 `.cnb.yml` 后必须用其校验器验证（见[步骤 6](#6-集成与提醒)）
- **调用文档技能（cnb-docs）**：查询平台级插件知识（插件市场总览 `/zh/plugins.md`、任务类型区别 `/zh/build/script-vs-plugin.md`、插件制作与贡献规范）时获取官方文档最新内容；单个插件的 settings 参数以插件 README 为准，由本 skill 直接读取

## 插件 README 编写建议

可控插件建议在 README.md 中包含以下内容，以便精确提取配置信息：

### 参数表（核心）

```markdown
## 参数

- `webhook`（string，必填）：企业微信群机器人 Webhook 地址
- `msg_type`（string，可选，默认 `text`）：消息类型：text / markdown
- `content`（string，必填）：消息内容
```

### 配置示例（必须有）

```yaml
# .cnb.yml 中使用（env 内联声明敏感变量）
main:
  push:
    - env:
        WECOM_WEBHOOK: https://qyapi.weixin.qq.com/cgi-bin/webhook?key=xxx
      stages:
        - name: 通知企业微信群
          image: tencentcom/wecom-message    # 默认 latest
          settings:
            webhook: $WECOM_WEBHOOK           # 变量名须与上方 env 中声明的一致
            msg_type: markdown
            content: "## 构建完成\n状态：**成功**"
```

### 要点

- **参数表五项齐全**：便于本技能直接解析并生成对应 `settings` 配置
- **配置示例可直接使用**：环境变量引用用 `$VAR` 占位，敏感信息用 `env` 内联声明
