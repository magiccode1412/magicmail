# CNB 流水线最佳实践

## 1. 用 YAML 锚点复用同文件内的配置

以 `.` 开头的 key 不会被识别为分支名，适合定义可复用片段。

```yaml
.node-env: &node-env
  docker:
    image: node:20
    volumes:
      - node_modules

.install: &install
  name: install
  script: npm ci

main:
  push:
    - <<: *node-env
      stages:
        - *install
        - name: build
          script: npm run build
  pull_request:
    - <<: *node-env
      stages:
        - *install
        - name: test
          script: npm test
```

---

## 2. 按功能拆分到 `.ci/` 目录 + `!reference` 跨文件复用

配置超过 150 行时，按「功能 + 触发事件」拆分。每个文件只做一件事，文件名即职责。

```
.cnb.yml                    # 仅 include 列表
.ci/
├── .shared-config.yml      # 公共配置（Docker 环境、Go 环境、Node 安装等）
├── .variables.yml          # 全局变量（webhook robot ID 等）
├── .docker-build.yml       # Docker 镜像构建
├── .pr.yml                 # PR 检查
├── .push-deploy-dev.yml    # push → dev 部署
├── .deploy.yml             # 生产/staging 部署
├── .tag-push.yml           # tag 发版
└── .e2e-test.yml           # E2E 测试
```

**核心设计模式：**

### 2.1 公共配置文件（`.shared-config.yml`）

以 `.` 开头的 key 不被识别为分支名，专门用作 `!reference` 引用源或 YAML 锚点定义：

```yaml
# .ci/.shared-config.yml — 被其他文件通过 !reference 跨文件引用

# Docker 构建环境（对象类型，!reference 引用整个对象）
.docker-config:
  build:
    dockerfile: .ide/Dockerfile
    by: [.ide/settings.json]
    versionBy: [.ide/settings.json]
  volumes:
    - node_modules/.cache
    - /root/go/pkg/mod
    - /root/.cache/go-build

# Go 环境变量（对象类型）
.go-env: &go-env
  GOPROXY: https://goproxy.cn,direct
  GOMAXPROCS: 32

# Node 依赖安装（Stage 片段，!reference 引用为完整 Stage）
.node_module_install:
  name: node_module_install
  script:
    - npm ci --frozen-lockfile
```

### 2.2 全局变量文件（`.variables.yml`）

集中管理全局常量（如 webhook robot ID），通过多级路径引用：

```yaml
# .ci/.variables.yml
.wecom:
  warrior_robot: 2dfe45f7-xxxx-xxxx
  test_robot: a32e4c3a-xxxx-xxxx
```

### 2.3 业务配置文件中使用 `!reference`

```yaml
# .ci/.push-deploy-dev.yml — 引用公共配置

.deploy-dev: &deploy-dev
  name: deploy-dev
  docker: !reference [.docker-config]          # 引用整个 Docker 配置对象
  env: !reference [.go-env]                    # 引用环境变量对象
  imports:
    - https://cnb.cool/<org>/<secret-repo>/-/blob/main/secrets.yml
  lock:
    key: deploy-dev
    wait: true
    cancel-in-wait: true
  stages:
    - !reference [.node_module_install]         # 引用整个 Stage 片段
    - name: build
      script: npm run build
    - name: deploy
      script: ./deploy.sh
  failStages:
    - name: notify failure
      image: tencentcom/wecom-message
      settings:
        robot: !reference [.wecom, warrior_robot]  # 多级路径引用标量
        msgType: markdown
        content: "部署失败: $CNB_BUILD_ID"

# === 分支触发 ===
main:
  push:
    deploy-dev: *deploy-dev        # 同文件内用 YAML 锚点引用
```

### 2.4 锚点 + `!reference` 组合：模板继承

同文件内用 `&锚点` + `<<: *锚点` 做模板继承，跨文件用 `!reference` 引用公共配置：

```yaml
# .ci/.docker-build.yml

# 基础模板（锚点 + !reference 混用）
.docker-build-base: &docker-build-base
  name: docker-build-${CNB_PIPELINE_KEY}
  services:
    - docker
  docker: !reference [.docker-config]       # 跨文件引用
  runner:
    cpus: 16

# 通用构建步骤（锚点定义，被多个模板引用）
.docker-build-stages: &docker-build-stages
  - name: docker login
    imports:
      - https://cnb.cool/<org>/<secret-repo>/-/blob/main/registry.yml
    script: docker login -u "$USER" -p "$TOKEN" registry.example.com
  - name: docker build
    script: make $MODULE_NAME
  - name: resolve
    type: cnb:resolve
    options:
      key: docker-build-${CNB_PIPELINE_KEY}

# 派生模板：在基础模板上追加步骤
.frontend-build: &frontend-build
  <<: *docker-build-base                    # 继承基础模板
  stages:
    - name: build frontend
      script: npm run build
    - *docker-build-stages                  # 锚点引用通用步骤

# === 分支触发（批量映射） ===
main:
  push:
    frontend: *frontend-build
    backend:
      <<: *docker-build-base
      stages:
        - *docker-build-stages
```

**要点总结：**

| 机制 | 作用范围 | 用途 | 语法 |
|------|---------|------|------|
| YAML 锚点 `&`/`*` | 同文件内 | 定义+引用模板，支持 `<<:` 合并展开 | `&name` 定义，`*name` 引用，`<<: *name` 合并 |
| `!reference` | 跨文件（通过 include） | 引用值（标量/数组/对象），不支持合并展开 | `!reference [.key]` 或 `!reference [.key, subkey]` |
| `.` 开头的 key | 全局 | 不被识别为分支名，专作引用源 | `.docker-config:`、`.wecom:` |

---

## 3. 善用 `failStages` / `endStages` / `allowFailure`

```yaml
- name: build-and-deploy
  stages:
    - name: build
      script: npm run build
    - name: deploy
      script: ./deploy.sh
  failStages:                    # 仅失败时
    - name: notify-failure
      image: tencentcom/wecom-message
      settings:
        content: "失败: $CNB_BUILD_ID"
  endStages:                     # 始终执行
    - name: cleanup
      script: rm -rf dist
```

---

## 4. 用 `cnb:await` / `cnb:resolve` 编排多 Pipeline

```yaml
main:
  push:
    build-frontend:
      stages:
        - name: build
          script: npm run build
        - name: resolve
          type: cnb:resolve
          options:
            key: frontend-ready

    deploy:
      stages:
        - name: await frontend
          type: cnb:await
          options:
            key: frontend-ready
        - name: deploy
          script: ./deploy.sh
```

---

## 5. Monorepo 大仓按需构建

大仓（Monorepo）场景下，多个模块/服务在同一仓库中。通过 `ifModify` + `CNB_PIPELINE_KEY` + `cnb:resolve`/`cnb:await` 实现**按需构建 + 编排部署**。

> **参考文档**：https://docs.cnb.cool/zh/build/showcase/monorepo.html

### 5.1 核心机制

| 特性 | 说明 |
|------|------|
| `CNB_PIPELINE_KEY` | 对象形式下为流水线的 key 名（如 `frontend`），数组形式下为自动索引（如 `pipeline-0`） |
| `ifModify` | Pipeline 或 Stage 级，仅匹配路径有变更时才触发 |
| `cnb:resolve` / `cnb:await` | 模块构建完成后发出信号（resolve），部署流水线等待所有模块就绪（await） |

### 5.2 基础模式：模板 + 批量映射

定义一个构建模板，用 `${CNB_PIPELINE_KEY}` 动态引用模块名，然后在分支触发处用 key 名批量展开：

```yaml
# 构建模板 — CNB_PIPELINE_KEY 运行时自动替换为流水线 key 名
.build-module: &build-module
  services:
    - docker
  ifModify:
    - packages/${CNB_PIPELINE_KEY}/**    # 仅该模块目录有变更时触发
  stages:
    - name: build
      script: cd packages/${CNB_PIPELINE_KEY} && docker build -t $IMAGE_TAG .
    - name: push
      script: docker push $IMAGE_TAG
    - name: resolve
      type: cnb:resolve
      options:
        key: build-${CNB_PIPELINE_KEY}   # 发出完成信号

# 分支触发 — 每个 key 就是一个模块，CNB_PIPELINE_KEY 自动等于 key 名
main:
  push:
    user-service: *build-module          # CNB_PIPELINE_KEY = "user-service"
    order-service: *build-module         # CNB_PIPELINE_KEY = "order-service"
    gateway: *build-module               # CNB_PIPELINE_KEY = "gateway"
```

> **新增模块只需加一行** `new-module: *build-module`，无需重复编写配置。

### 5.3 进阶模式：模板继承 + 差异化构建

不同模块可能需要不同的构建步骤。用基础模板 + 派生模板实现：

```yaml
# 基础配置
.build-base: &build-base
  name: build-${CNB_PIPELINE_KEY}
  services:
    - docker
  docker: !reference [.docker-config]        # 跨文件引用公共 Docker 配置
  env:
    MODULE_NAME: ${CNB_PIPELINE_KEY}

# 通用构建步骤
.build-stages: &build-stages
  - name: docker build
    script: make $MODULE_NAME
  - name: resolve
    type: cnb:resolve
    options:
      key: build-${CNB_PIPELINE_KEY}

# 通用模块（直接使用基础步骤）
.standard-module: &standard-module
  <<: *build-base
  stages:
    - *build-stages

# 前端模块（追加前置步骤）
.frontend-module: &frontend-module
  <<: *build-base
  stages:
    - name: install deps
      script: npm ci
    - name: build frontend
      script: npm run build
    - *build-stages                          # 复用通用构建步骤

# 批量映射
main:
  push:
    frontend: *frontend-module
    frontend-cdn: *frontend-module
    backend: *standard-module
    gateway: *standard-module
    docs: *standard-module
```

### 5.4 部署编排：await 等待所有模块构建完成

部署流水线使用 `cnb:await` 等待所有模块构建就绪后再统一部署：

```yaml
# await 模板（锚点复用）
.await-module: &await-module
  type: cnb:await
  options:
    key: build-${CNB_BUILD_JOB_NAME}         # CNB_BUILD_JOB_NAME = job 的 key 名

main:
  push:
    deploy:
      lock:
        key: deploy-dev
        wait: true
        cancel-in-wait: true
      stages:
        - name: await all modules
          jobs:
            frontend: *await-module          # 等待 frontend 构建完成
            backend: *await-module
            gateway: *await-module
        - name: deploy
          script: ./deploy.sh
```

### 5.5 PR 检查：ifModify + cnb:await 按需等待

PR 检查流水线中，各模块独立检查并 `resolve`，汇总流水线只 `await` 有变更的模块：

```yaml
"**":
  pull_request:
    # 各模块独立检查（并发执行）
    golang-lint:
      ifModify: &go-files
        - "**/*.go"
        - "**/go.mod"
      stages:
        - name: lint
          script: golangci-lint run ./...
        - name: resolve
          type: cnb:resolve
          options:
            key: golang-check

    frontend-lint:
      ifModify: &frontend-files
        - frontend/**
      stages:
        - name: lint
          script: cd frontend && npm run lint
        - name: resolve
          type: cnb:resolve
          options:
            key: frontend-check

    # 汇总流水线 — 只 await 有变更的模块
    summary:
      stages:
        - name: await golang
          type: cnb:await
          ifModify: *go-files               # 仅 Go 文件有变更时才等待
          options:
            key: golang-check
        - name: await frontend
          type: cnb:await
          ifModify: *frontend-files         # 仅前端文件有变更时才等待
          options:
            key: frontend-check
        - name: notify
          image: tencentcom/wecom-message
          settings:
            msgType: markdown
            content: "PR 检查通过"
```

> **关键技巧**：`cnb:await` 的 `ifModify` 使用锚点引用对应模块的路径匹配规则，确保只等待有变更的模块。

---

## 6. 跨仓库公共模板：通用流程复用 + 业务变量分离

当多个业务仓库共享相同的构建部署流程（如 docker build → push → deploy TKE），可将通用流程封装到**公共模板仓库**，各业务仓库只需声明自己的变量即可。

### 6.1 架构设计

```
公共模板仓库（如 org/template/build-deploy）
└── build-deploy.yml         # 通用流水线模板（docker build + push + deploy）

密钥仓库（独立仓库，如 org/secret）
└── secret.yml               # 制品库凭证、kubeconfig 等敏感信息

业务仓库 A
├── .cnb.yml                 # include 引用公共模板
├── .cnb/
│   ├── workload_env.service-a.yml   # service-a 流水线的变量
│   └── workload_env.service-b.yml   # service-b 流水线的变量（多服务并行）
└── Dockerfile

业务仓库 B（同样引用公共模板，变量不同）
├── .cnb.yml
├── .cnb/
│   └── workload_env.my-app.yml
└── Dockerfile
```

### 6.2 公共模板仓库

模板中只写通用流程逻辑，所有业务差异通过**环境变量**注入：

```yaml
# org/template/build-deploy/build-deploy.yml

.build-and-deploy:
  services:
    - docker
  imports:
    # 密钥仓库（制品库凭证、kubeconfig 等）
    - https://cnb.cool/<org>/<secret-repo>/-/blob/main/secret.yml
    # 业务仓库本地的变量配置文件（通过 $CNB_PIPELINE_KEY 动态匹配）
    - .cnb/workload_env.$CNB_PIPELINE_KEY.yml
  stages:
    - name: build image
      script: |
        docker login -u "$DOCKER_USERNAME" -p "$DOCKER_PASSWORD" ${IMAGE_DOMAIN}
        docker build -f ${DOCKERFILE:-Dockerfile} -t ${IMAGE}:${CNB_COMMIT_SHORT} ${CONTEXT_PATH:-.}
        docker push ${IMAGE}:${CNB_COMMIT_SHORT}
    - name: deploy
      image: bitnami/kubectl
      script: |
        echo "$KUBECONFIG_DATA" > ~/.kube/config
        kubectl set image ${WORKLOAD_TYPE}/${NAME} ${CONTAINER}=${IMAGE}:${CNB_COMMIT_SHORT} -n ${NS}
```

> **关键**：`imports: .cnb/workload_env.$CNB_PIPELINE_KEY.yml` — 模板通过 `$CNB_PIPELINE_KEY` 动态加载业务仓库中对应流水线 key 的配置文件，实现一套模板支持多条流水线。

### 6.3 业务仓库

业务仓库只需 `include` 远程模板 + 声明自己的变量：

```yaml
# 业务仓库 .cnb.yml — 跨文件必须用 !reference，不能用 YAML 锚点
include:
  - https://cnb.cool/<org>/template/build-deploy/-/blob/main/build-deploy.yml

main:
  push:
    service-a: !reference [.build-and-deploy]   # CNB_PIPELINE_KEY = "service-a"
    service-b: !reference [.build-and-deploy]   # CNB_PIPELINE_KEY = "service-b"
```

```yaml
# 业务仓库 .cnb/workload_env.service-a.yml（环境变量声明）
IMAGE_DOMAIN: registry.example.com
IMAGE: registry.example.com/myteam/service-a
WORKLOAD_TYPE: deployment
NAME: service-a
CONTAINER: service-a
NS: production
DOCKERFILE: Dockerfile
CONTEXT_PATH: .
```

### 6.4 设计要点

| 要点 | 说明 |
|------|------|
| **模板与业务分离** | 公共模板维护构建部署逻辑，业务仓库只配置变量 |
| **多流水线并行** | 通过 `$CNB_PIPELINE_KEY` 动态加载不同配置文件，一个仓库可运行多条流水线 |
| **密钥集中管理** | 敏感信息放独立密钥仓库，模板通过 https 绝对路径引用 |
| **横向扩展** | 新增业务仓库只需 include 模板 + 创建变量文件，无需重复编写流水线 |
| **远程 include + !reference** | `include` 支持跨仓库远程引用 YAML 文件，业务仓库通过 `!reference` 引用模板中的 `.` 开头 key（跨文件不能用锚点） |

---

## 7. 给 Pipeline 和 Stage 取有意义的名称

名称直接显示在构建界面。用「动词+名词」描述操作：`install-deps`、`run-unit-tests`、`deploy-to-staging`。

---

## 8. 用自定义 Dockerfile 预装系统依赖

避免在脚本中每次执行 `apt install`、`yum install` 等系统包安装命令。每次流水线运行都会重复安装，浪费时间和资源。

```yaml
# ❌ 不推荐：每次流水线都重新安装
main:
  push:
    - docker:
        image: ubuntu:22.04
      stages:
        - name: install-deps
          script: apt-get update && apt-get install -y python3 curl jq
        - name: build
          script: python3 build.py

# ✅ 推荐：通过 Dockerfile 预装依赖
# 1. 创建 Dockerfile（如 image/Dockerfile）
# FROM ubuntu:22.04
# RUN apt-get update && apt-get install -y python3 curl jq && rm -rf /var/lib/apt/lists/*
#
# 2. 配置流水线使用 Dockerfile
# main:
#   push:
#     - docker:
#         build: image/Dockerfile
#       stages:
#         - name: build
#           script: python3 build.py
```

Dockerfile 构建的镜像会被自动缓存，仅当 Dockerfile 或依赖文件变化时才重新构建。参考：${CNB_WEB_PROTOCOL:-https}://docs.${CNB_WEB_HOST:-cnb.cool}/zh/build/build-env.md

---

## 9. Stage 级 imports 按需引用密钥

Pipeline 级 `imports` 对所有 stages 生效，但并非所有步骤都需要全部密钥。将密钥引用下沉到需要它的 Stage，遵循最小权限原则：

```yaml
main:
  push:
    - name: build-and-push
      stages:
        - name: build
          script: npm run build
        - name: docker login
          imports:
            - https://cnb.cool/<org>/<secret-repo>/-/blob/main/registry-credentials.yml
          script: docker login -u "$REGISTRY_USER" -p "$REGISTRY_TOKEN" mirrors.example.com
        - name: docker push
          script: docker push mirrors.example.com/my-app:$CNB_COMMIT_SHORT
```

> **要点**：`docker login` 步骤单独引用 registry 凭证，其他步骤不会接触到这些密钥。

---

## 10. 部署场景用 lock + cancel-in-wait 避免排队堆积

频繁推送时，部署流水线可能排起长队。使用 `cancel-in-wait` 只保留最新排队的流水线：

```yaml
main:
  push:
    - name: deploy-dev
      lock:
        key: deploy-dev
        wait: true              # 排队等待（不设则直接失败）
        cancel-in-wait: true    # 取消排队中的同 key 流水线，只保留最新
      stages:
        - name: deploy
          script: ./deploy.sh
```

> **对比**：`cancel-in-progress` 取消正在执行的；`cancel-in-wait` 取消排队等待中的。部署场景通常两者都设。

---

## 11. 多架构镜像构建（amd64 + arm64）

通过 `runner.tags` 指定不同架构的构建节点，配合 `cnb:resolve`/`cnb:await` 编排，最后用 manifest 插件合并多架构镜像：

```yaml
# 构建步骤模板
.build-by-arch: &build-by-arch
  - name: docker build & push
    script:
      - docker build -t $IMAGE:$VERSION-linux-$BUILD_ARCH .
      - docker push $IMAGE:$VERSION-linux-$BUILD_ARCH
  - name: resolve
    type: cnb:resolve
    options:
      key: build-$BUILD_ARCH

# amd64 构建
.amd64-build: &amd64-build
  name: build-amd64
  runner:
    tags: cnb:arch:amd64
  services:
    - docker
  env:
    BUILD_ARCH: amd64
  stages:
    - *build-by-arch

# arm64 构建
.arm64-build: &arm64-build
  name: build-arm64
  runner:
    tags: cnb:arch:arm64:v8
  services:
    - docker
  env:
    BUILD_ARCH: arm64
  stages:
    - *build-by-arch

main:
  push:
    - *amd64-build                   # 并发构建 amd64
    - *arm64-build                   # 并发构建 arm64
    - name: combine-arch
      services:
        - docker
      stages:
        - name: await amd64
          type: cnb:await
          options:
            key: build-amd64
        - name: await arm64
          type: cnb:await
          options:
            key: build-arm64
        - name: manifest               # 合并多架构镜像
          image: cnbcool/manifest
          settings:
            target: $IMAGE:$VERSION
            template: $IMAGE:$VERSION-OS-ARCH
            platforms:
              - linux/amd64
              - linux/arm64
```

> **要点**：`runner.tags: cnb:arch:arm64:v8` 指定 ARM 节点；两个架构并发构建，合并 Pipeline 通过 `cnb:await` 等待两者完成后用 manifest 插件生成统一镜像。

---

## 12. 端到端示例：Node.js 项目完整 CI/CD

以下是一个 Node.js 项目的完整 `.cnb.yml`，覆盖推送测试、PR 检查、Tag 发版三种场景：

```yaml
# ── 公共配置（YAML 锚点） ─────────────────────────
.node-env: &node-env
  docker:
    image: node:20
    volumes:
      - node_modules

# ── 推送：测试 + 构建 ────────────────────────────
main:
  push:
    - name: test-and-build
      <<: *node-env
      ifModify:
        - src/**
        - package.json
      stages:
        - name: install
          script: npm ci
        - name: lint
          script: npm run lint
        - name: test
          script: npm test
        - name: build
          script: npm run build
      endStages:
        - name: notify
          script: echo "Build $CNB_BUILD_ID completed"

# ── PR：检查 + 覆盖率 ────────────────────────────
"**":
  pull_request:
    - name: pr-check
      <<: *node-env
      stages:
        - name: install
          script: npm ci
        - name: lint
          script: npm run lint
        - name: test-with-coverage
          script: npm test -- --coverage
        - name: coverage-report
          type: testing:coverage
          options:
            pattern: coverage/lcov.info
            lines: 80
            diffLines: 90

# ── Tag：发版 ───────────────────────────────────
"$":
  tag_push:
    - name: release
      docker:
        build: ./Dockerfile
      services:
        - docker
      stages:
        - name: build-image
          script: |
            docker build -t $CNB_DOCKER_REGISTRY/$CNB_REPO_SLUG:$CNB_BRANCH .
            docker push $CNB_DOCKER_REGISTRY/$CNB_REPO_SLUG:$CNB_BRANCH
        - name: create-release
          type: git:release
          options:
            descriptionFromFile: CHANGELOG.md
```

**要点：**
- YAML 锚点 `&node-env` 复用 Docker 配置，避免重复
- `ifModify` 跳过无关文件变更的构建
- PR 和 push 使用不同分支匹配，避免重复触发
- Tag 发版用 `docker.build` 预装构建依赖，用 `git:release` 自动发布
- `testing:coverage` 仅在 PR 事件下有增量覆盖率，此处正好用于 PR 检查