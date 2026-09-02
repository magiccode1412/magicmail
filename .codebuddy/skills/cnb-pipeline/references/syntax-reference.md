# CNB 流水线语法参考

## 触发事件

### Git 事件

| 事件 | 时机 | 代码版本 |
|------|------|----------|
| `push` | 推送代码 | 当前分支最新 |
| `commit.add` | 推送含新提交时（提供 `CNB_NEW_COMMITS_COUNT`） | 当前分支最新 |
| `branch.create` / `branch.delete` | 分支创建/删除 | 最新/默认分支 |
| `pull_request` | PR 创建/重新打开/源分支 push | 预合并后 |
| `pull_request.update` | 同上 + title/description 修改 | 预合并后 |
| `pull_request.target` | 同 PR，但用目标分支代码执行 | 目标分支最新 |
| `pull_request.merged` | PR 合并后 | 合并后 |
| `pull_request.mergeable` | 保护分支 + 无冲突 + 评审通过（配置取自目标分支） | 目标分支最新 |
| `pull_request.approved` / `.changes_requested` | 评审操作 | 预合并后 |
| `pull_request.comment` | PR 评论 | 预合并后 |
| `tag_push` | 推送 Tag | 当前 Tag |
| `auto_tag` | 页面点击「自动生成 Tag」 | 默认分支 |

### 页面/API/定时/Issue/NPC 事件

| 事件 | 时机 | 说明 |
|------|------|------|
| `web_trigger` / `web_trigger_*` | 页面手动触发 | 支持自定义按钮（`.cnb/web_trigger.yml`） |
| `api_trigger` / `api_trigger_*` | API / `cnb:apply` / `cnb:trigger` | 可指定版本 |
| `tag_deploy.*` | Tag/Release 页面「部署」按钮 | 需配 `.cnb/tag_deploy.yml` |
| `"crontab: 30 5 * * *"` | 定时任务（cron 表达式） | 指定分支最新 |
| `issue.open/close/reopen/update/comment` | Issue 事件（挂在 `$` 下） | 默认分支 |
| `issue.comment@npc` / `pull_request.comment@npc` | @NPC 触发 | 评论者为触发者 |
| `vscode` | 云原生开发 | 当前分支 |

> **在线文档**：${CNB_WEB_PROTOCOL:-https}://docs.${CNB_WEB_HOST:-cnb.cool}/zh/build/trigger-rule.md

---

## Pipeline 全部字段

```yaml
main:
  push:
    - name: ci                   # 名称（显示在构建界面）
      runner:                      # 可选，一般无需配置（默认 8 核 16G）
        cpus: 8                  # CPU 核数（内存 = CPU x 2G）；省核时设 1/2/4，高消耗或 OOM 时设 16/32
        tags: cnb:arch:amd64     # 节点标签（支持变量替换）
      docker:                    # 构建环境（三选一）
        image: node:20           # 方式 1：指定镜像
      # docker:
      #   build:                 # 方式 2：Dockerfile 构建（自动缓存）
      #     dockerfile: ./Dockerfile
      #     by: [package.json]
      #     versionBy: [package-lock.json]
      # docker:
      #   devcontainer: .devcontainer/devcontainer.json  # 方式 3
      services:
        - docker                 # Docker-in-Docker
      env:
        NODE_ENV: production
      imports:
        - https://cnb.cool/<org>/<secret-repo>/-/blob/main/secrets.yml  # 导入密钥仓库（独立仓库，https 绝对路径）
      lock:
        key: deploy-lock         # 防并发锁
        wait: true               # 排队等待（不设则直接失败）
        cancel-in-progress: true # 取消正在执行的同 key 流水线
        cancel-in-wait: true     # 取消排队中的同 key 流水线（适合部署场景：只保留最新）
      retry: 2                   # 失败重试
      ifModify: ["src/**"]       # Pipeline 级文件变更条件
      stages: [...]              # 主阶段
      failStages: [...]          # 仅失败时执行
      endStages: [...]           # 始终执行
```

> 未指定 docker 时使用缺省镜像 `cnbcool/default-build-env`。
> **在线文档**：${CNB_WEB_PROTOCOL:-https}://docs.${CNB_WEB_HOST:-cnb.cool}/zh/build/build-env.md

---

## Stage 全部字段

```yaml
stages:
  - name: install
    script: npm ci               # 单行/多行脚本
    timeout: 10m                 # 超时（ms/s/m/h）
    retry: 2
    allowFailure: true           # 支持变量替换
    ifModify: ["**/*.ts"]        # 文件变更条件（支持变量替换）
    ifNewBranch: true            # 仅新分支执行
    if: '[ "$RUN" = "true" ]'    # 自定义条件脚本
    image: node:18               # 覆盖 Pipeline 镜像（仅支持 image）
    env:
      KEY: value
    imports: https://cnb.cool/<org>/<secret-repo>/-/blob/main/stage-env.yml  # https 绝对路径引用密钥仓库
    jobs: [...]                  # 多 job
```

---

## Job 并发模式

```yaml
# 串行（数组）          # 并行（对象）
jobs:                   jobs:
  - name: test            test:
    script: npm test        script: npm test
  - name: lint            lint:
    script: npm run lint    script: npm run lint
```

---

## Include 和 !reference

```yaml
# .cnb.yml
include:
  - .ci/build.yml                               # 本地
  - "${CNB_WEB_ENDPOINT:-https://cnb.cool}/org/tpl/-/blob/main/ci.yml"  # 远程
  - path: .ci/optional.yml
    ignoreError: true

# 合并规则：数组追加，对象同名键后者覆盖，本地优先
```

```yaml
# !reference 跨文件引用值（标量/数组/对象片段）
docker: !reference [.docker-config]              # 引用对象 → 整个 docker 配置
env: !reference [.go-env]                        # 引用对象 → 环境变量 map
robot: !reference [.wecom, warrior_robot]        # 引用标量 → 多级路径取值
stages:
  - !reference [.node_module_install]            # 引用为 Stage 片段 → 插入到 stages 数组中
  - name: build
    script: npm run build
```

> **关键规则：**
> - `!reference` 只能引用值，不能做 `<<: *anchor` 的合并展开。合并展开只能用 YAML 锚点（同文件内）
> - 引用源 key 以 `.` 开头（如 `.docker-config`、`.wecom`），不会被识别为分支名
> - 同文件内优先用 YAML 锚点 `&`/`*`（支持 `<<:` 合并），跨文件用 `!reference`
> - 可在同一文件中混用：`docker: !reference [.docker-config]`（跨文件引用公共配置）+ `<<: *local-base`（同文件模板继承）
> - 详细实战模式见 `references/best-practices.md` 第 2 节
>
> **在线文档**：${CNB_WEB_PROTOCOL:-https}://docs.${CNB_WEB_HOST:-cnb.cool}/zh/build/simplify-configuration.md

---

## 数据卷缓存

```yaml
docker:
  volumes:
    - node_modules              # cow（默认，写时复制，支持并发）
    - cache:/root/.npm:rw       # 读写（串行场景）
    - shared:/data:data         # 临时数据卷（Pipeline 内共享）
    - readonly:/data:ro         # 只读
```

---

## 变量替换

以下字段支持 `$VAR` 替换：`env`、`imports`、`options`/`optionsFrom`、`settings`/`settingsFrom`、`docker`(image/build/volumes)、`stage.image`、`ifModify`、`name`、`lock.key`、`allowFailure`、`runner.tags`。用 `\$` 阻止替换。

---

## 部署配置

通过 `.cnb/tag_deploy.yml` 定义部署环境、审批流程、部署按钮，触发 `tag_deploy.*` 事件。

```yaml
# .cnb/tag_deploy.yml
environments:
  - name: staging
    env: { tag_name: $CNB_BRANCH }
  - name: production
    require:
      - environmentName: staging       # 前置环境
        after: 1800                    # 等待秒数
      - approver:                      # 审批
          users: [deployer1]
          roles: [master]
        title: 生产部署审批

# .cnb.yml 中配置部署流水线
$:
  tag_deploy.staging:
    - stages:
        - name: deploy
          script: ./deploy.sh staging
  tag_deploy.production:
    - stages:
        - name: deploy
          script: ./deploy.sh production
```

> **在线文档**：${CNB_WEB_PROTOCOL:-https}://docs.${CNB_WEB_HOST:-cnb.cool}/zh/build/deploy.md

---

## 手动触发（Web Trigger）

通过 `.cnb/web_trigger.yml` 配置自定义按钮，支持输入参数（input/textarea/select/switch/radio）、权限控制、分组。

```yaml
# .cnb/web_trigger.yml
branch:
  - reg: ^release
    buttons:
      - name: 部署
        event: web_trigger_deploy
        permissions:
          roles: [master, developer]
        inputs:
          target_env:
            name: 目标环境
            type: select
            options:
              - { name: staging, value: staging }
              - { name: production, value: production }
```

> **在线文档**：${CNB_WEB_PROTOCOL:-https}://docs.${CNB_WEB_HOST:-cnb.cool}/zh/build/web-trigger.html
