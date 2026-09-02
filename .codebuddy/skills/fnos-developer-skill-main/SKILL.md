---
name: fnos-developer
version: 1.0.0
description: 面向飞牛 fnOS 应用开发、打包、生命周期脚本、桌面入口、统一网关、文件授权与开放 API 接入的工程化 Skill。用于新建或改造 .fpk 应用、生成 manifest/config/cmd/wizard/UI 文件、接入 @trimjs/web-app、调用 Unix Socket 后端 API、排查权限和错误码，并执行最小权限与路径安全审查。
source_docs:
  - https://developer.fnnas.com/docs/guide/
  - https://developer.fnnas.com/api/overview/
source_snapshot: 2026-08-06
---

# 飞牛 fnOS 开发 Skill

## 目标

把用户的 fnOS 应用需求转化为**能打包、能安装、能测试、权限边界明确**的工程方案或代码修改。

本 Skill 覆盖：

- 创建和打包 `.fpk` 应用。
- `manifest`、`config/privilege`、`config/resource`、`app/ui/config`、`wizard/*`、`cmd/*`。
- 静态 CGI、独立端口服务、统一网关、Docker 项目。
- 前端 `@trimjs/web-app` JS SDK。
- 后端通过 Unix Socket 调用开放 API。
- 应用共享授权、用户个人授权、ACL 检查、路径转换、页面路由和页面交互。
- 安装、升级、配置、卸载、日志和测试排错。

不把未经官方文档确认的接口、字段或系统行为当作事实。涉及版本、下载地址、上架流程或新增 API 时，优先重新查看官方文档。

## 何时触发

用户出现以下意图时使用本 Skill：

- “开发/移植/打包一个飞牛、fnOS、飞牛 NAS 应用”。
- “生成或检查 fpk、manifest、privilege、resource、wizard、cmd/main”。
- “接入飞牛开放 API、TrimApp、@trimjs/web-app”。
- “文件授权、目录授权、ACL、语义路径、打开文件管理器”。
- “统一网关、gatewayPrefix、gatewaySocket、X-Trim-* Header”。
- “fnpack、appcenter-cli、安装失败、Scope/Token/Forbidden/Not Found”。

## 首要原则

1. **先分类，再写代码。** 判断是静态 CGI、独立端口、统一网关、Docker，还是仅修改现有包。
2. **最小权限。** 默认 `run-as=package`；只声明实际需要的资源、依赖和 `api-scope`。
3. **用户身份和文件权限分层处理。** 网关登录态不等于业务授权；应用授权路径也不等于所有用户均可访问。
4. **不硬编码安装路径。** 生命周期脚本和服务代码优先使用 `TRIM_APPDEST`、`TRIM_PKGETC`、`TRIM_PKGVAR`、`TRIM_PKGTMP`、`TRIM_PKGHOME`。
5. **Token 只在后端内存中使用。** 每次从 `TRIM_API_TOKEN` 读取，不持久化、不打印、不下发前端。
6. **路径一律视为不可信输入。** 标准化路径，拒绝 `..`、符号链接逃逸和越界访问；执行文件操作前检查授权范围与当前用户 ACL。
7. **输出必须可验证。** 给出目录树、关键文件、构建命令、安装命令、验收步骤和已知假设。

## 开始任务前

先读取与任务对应的参考文件，避免无关内容占用上下文：

- 应用包、生命周期、入口和运行环境：`references/package-model.md`
- 开放 API、Scope、方法和错误码：`references/open-api.md`
- 安全与审查清单：`references/security-review.md`
- 构建、安装和排错：`references/build-test.md`

用户提供现有项目时，优先检查实际文件，不要从零重写。先执行：

```bash
python scripts/validate_fnos_project.py /path/to/project
```

该脚本是启发式检查器，不替代官方 `fnpack build`。

## 任务分类

### A. 静态或极轻量页面

选择 `index.cgi`，条件通常是：

- 主要提供静态 HTML/CSS/JS。
- 无 WebSocket。
- 无常驻后台服务、流式响应、长耗时接口或高流量 API。
- 希望沿用当前 fnOS 访问域名和 NAS 登录态。

入口通常为：

```text
/cgi/ThirdParty/{appname}/index.cgi/
```

### B. 独立端口服务

选择端口服务，条件通常是：

- 应用本身已有 HTTP 服务。
- 不需要复用 NAS 登录态或网关用户上下文。
- 可以接受独立端口和相应网络暴露面。

需要明确 `service_port`、端口冲突处理和访问控制。

### C. 统一网关

选择统一网关，条件通常是：

- 需要复用 fnOS 当前域名。
- 需要 NAS 会话校验及 `X-Trim-Userid`、`X-Trim-Isadmin`、`X-Trim-Username`。
- 需要常驻服务、API、长连接或 WebSocket。
- 希望授权回调页与应用页面同域。

公开路径使用 `/app/{appname}` 或其稳定子路径；服务监听安装目录中的 Unix Socket。

### D. Docker 项目

选择 Docker 模板，条件通常是：

- 多服务编排。
- 依赖数据库、缓存或受控容器运行环境。
- 已有 Docker Compose 方案，且能处理挂载、权限、升级和数据持久化。

不要因为“部署方便”默认使用 Docker；先评估镜像架构、离线依赖、体积、数据目录和升级策略。

## 最少必要信息

仅在无法安全推断时询问，通常包括：

- `appname`、显示名称、版本。
- 目标架构：`x86`、`arm` 或真正无原生依赖时的 `all`。
- 静态 CGI、端口服务、统一网关或 Docker。
- 运行时及版本依赖。
- 是否需要桌面入口、文件打开入口、管理员专用入口。
- 是否访问用户文件；属于管理员共享授权还是按用户授权。
- 是否需要 WebSocket、外部网络、GPU/视频设备、中间件。
- 需要保留的数据、卸载时是否允许删除。

信息不完整但不影响主体方案时，使用明显占位符，例如 `${APPNAME}`，并列出待替换项，不要阻塞任务。

## 标准工作流

### 1. 形成设计摘要

用少量文字明确：

- 访问模型。
- 运行用户。
- 持久化目录。
- 所需依赖和资源。
- 所需 API Scope。
- 用户身份与文件权限模型。

### 2. 生成目录树

至少标出：

```text
{appname}/
├── app/
│   └── ui/                  # 有桌面入口时
├── cmd/
├── config/
│   ├── privilege
│   └── resource
├── wizard/
├── manifest
├── ICON.PNG
└── ICON_256.PNG
```

按实际需要增加 `app/www`、`app/server`、`app/docker` 等，不创建无用途的文件。

### 3. 生成 `manifest`

遵循以下规则：

- `manifest` 是根目录无扩展名的键值文件，不是 JSON。
- 必须保持 `appname` 稳定；不要在升级中随意改名。
- 第三方应用使用 `source=thirdparty`。
- 仅在无架构相关二进制和依赖时使用 `platform=all`。
- `os_min_version` 依据真实用到的能力及测试结果设置。
- 静态应用可用 `ctl_stop=false`；服务应用通常保留运行控制。
- 使用 JS SDK 时设置 `micro_app=true`。
- 依赖用 `install_dep_apps`，只声明直接依赖；多个依赖的准备顺序按官方规则处理。

### 4. 生成权限与资源

`config/privilege` 默认：

```json
{
  "defaults": {
    "run-as": "package"
  },
  "username": "${APP_USER}",
  "groupname": "${APP_GROUP}"
}
```

只有特定硬件资源确实由系统组控制时才添加 `join-groups`。Root 模式必须解释原因，并尽早降权启动长期服务。

`config/resource` 只放实际使用项，例如：

- `api-scope`
- `data-share`
- `usr-local-linker`
- `docker-project`

不要把应用内部数据目录暴露成共享目录。

### 5. 生成入口

入口 ID 使用稳定的应用前缀，例如 `${APPNAME}.main`。

- 桌面窗口：`type=iframe`
- 需要完整浏览器能力：`type=url`
- 管理员入口：通常 `allUsers=false` 且 `control.accessPerm=readonly`
- 文件入口：只声明真实支持的 `fileTypes`，并把 `path` 查询参数视为用户输入
- 统一网关：由 `gatewayPrefix` 和 `gatewaySocket` 决定路由，`protocol`/`port` 不参与

### 6. 生成生命周期脚本

`cmd/main` 至少正确处理：

- `start`：启动并避免重复实例。
- `stop`：优雅停止，清理 PID/Socket。
- `status`：运行返回 `0`，未运行返回 `3`。
- 未知参数返回 `1`。

安装、升级、配置脚本应可重复执行。用户可见失败信息先写入 `TRIM_TEMP_LOGFILE`，再以非零状态退出。

升级时：

- 先备份或验证可回滚数据。
- 数据和配置迁移幂等。
- 不删除无法恢复的用户数据。

卸载时：

- 尊重向导中的保留/删除选择。
- 不删除应用目录之外未明确拥有的数据。

### 7. 接入前端 JS SDK

安装：

```bash
npm install @trimjs/web-app
```

初始化：

```ts
import { TrimApp } from '@trimjs/web-app';
const sdk = new TrimApp();
```

文件选择/授权前判断运行环境：

- 宿主环境：直接调用 `pickSharedFile`、`pickUserFile` 等。
- `sdk.isStandaloneWeb === true`：通过用户点击触发 `openAppAuth`。
- 授权回调校验业务 `state`，同源校验 `postMessage`，并保留“刷新授权状态”按钮。
- `$on('os/theme')` 和 `$on('os/language')` 仅在 `sdk.isWeb === true && sdk.isStandaloneWeb === false` 时使用。

优先复用 `templates/frontend-auth-flow.ts`，再按业务缩减。

### 8. 接入后端开放 API

后端 API 固定通过：

```text
POST /api/v1/trimapp
Unix Socket: /var/run/trim_open_gateway_apiscope.socket
Authorization: Bearer ${TRIM_API_TOKEN}
```

请求体：

```json
{
  "reqId": "string",
  "req": "trim.system.getPlatformConfig",
  "appName": "your-app",
  "data": {}
}
```

强制要求：

- 只在服务端调用。
- 每次从当前进程环境读取 `TRIM_API_TOKEN`。
- 检查 HTTP 状态、响应 `code`、`msg`。
- 记录 `reqId`，但日志不得包含 Token、敏感路径内容或用户密钥。
- 使用 `templates/node-trim-api-client.mjs` 时先替换 `appName`，并根据业务限制允许的 `req`。

### 9. 文件访问决策

#### 管理员共享授权

适用于所有用户共享的一批固定目录：

- Scope：`trim.file.sharedAccess`
- 前端：`pickSharedFile` / `authorizeSharedFile`
- 后端：查询或删除共享授权目录
- 只有管理员可操作，且只支持目录授权

#### 用户个人授权

适用于每个用户不同的目录或文件：

- Scope：`trim.file.userAccess`
- 前端：`pickUserFile` / `authorizeUserFile`
- 后端：按可信网关 UID 查询或删除用户授权目录
- 文件授权结果直接来自选择器/回调，不会出现在目录查询接口中

#### ACL 检查

在向当前用户返回文件列表、预览、下载、写入或删除前：

- Scope：`trim.file.userAcl`
- 调用 `trim.file.checkUserACL`
- 读取要求 `readable=true`
- 写入要求 `writable=true`
- 删除要求 `deletable=true`

路径不存在或不可检查时按全部 `false` 处理。

#### 路径展示

页面不要直接暴露 `/vol1/...` 内部路径。需要展示时：

- Scope：`trim.file.path`
- 调用 `trim.file.convertPath`
- 必须传当前界面 `language`

### 10. 验证

生成或修改完成后：

```bash
python scripts/validate_fnos_project.py /path/to/project
fnpack build --directory /path/to/project
```

在测试设备上：

```bash
appcenter-cli install-fpk myapp.fpk
appcenter-cli list
appcenter-cli start myapp
```

包含向导时可用：

```bash
appcenter-cli install-fpk myapp.fpk --env config.env
```

最终说明中必须给出：

- 构建命令。
- 安装/升级测试路径。
- 关键正向测试。
- 权限拒绝测试。
- 依赖不可用、端口冲突、路径不存在或 Token/Scope 错误测试。

## 代码审查顺序

审查现有项目时按以下顺序：

1. `manifest` 与实际运行模型是否一致。
2. `config/privilege` 是否最小权限。
3. `config/resource` 是否过度声明。
4. 持久化目录是否正确，是否硬编码路径。
5. `cmd/*` 是否幂等、状态码正确、能清理进程和 Socket。
6. UI 入口是否稳定、可见性是否正确。
7. JS SDK 是否区分宿主/独立浏览器环境。
8. 后端 API 是否只走 Unix Socket，Token 是否泄露。
9. 网关 UID 是否来自 `X-Trim-*`，而非客户端参数。
10. 文件操作是否同时满足授权范围、ACL 和路径边界。
11. 升级/卸载是否保护用户数据。
12. 构建、安装和回归步骤是否完整。

问题按严重度输出：

- **阻断**：会导致安装失败、越权、Token 泄露、任意文件访问/命令执行、数据破坏。
- **高**：权限过宽、身份信任错误、升级不可逆、网关/Socket 暴露错误。
- **中**：错误处理、兼容性、幂等、端口、资源声明问题。
- **低**：可维护性、命名、用户体验、日志和文档问题。

每个问题给出：文件/位置、原因、可复现影响、最小修复方案。

## 输出格式

新建应用时，按此顺序输出：

1. 设计摘要与假设。
2. 目录树。
3. 每个关键文件的完整内容。
4. 构建和安装命令。
5. 验收与安全测试。
6. 仍需用户替换的值。

修改现有项目时，优先输出补丁或明确的逐文件替换内容，不要无故重写整个工程。

排错时，先给最可能的根因和验证命令，再给修复。不要把“重装系统”当作默认建议。

## 禁止事项

- 不在浏览器前端调用 `/api/v1/trimapp`。
- 不把 `TRIM_API_TOKEN` 写入数据库、文件、配置、前端 bundle、URL 或日志。
- 不默认使用 root，不以 root 长期运行面向用户的网络服务。
- 不信任请求体、查询参数或 WebSocket 消息中的 UID/管理员标记。
- 不仅凭“目录已授权”就向所有登录用户暴露内容。
- 不拼接未经验证的路径到 shell 命令。
- 不使用 `..` 检查作为唯一的路径安全措施；还应标准化并验证最终路径位于允许根目录内。
- 不在页面加载、定时器或无用户手势的异步回调中自动弹出授权窗口。
- 不声明全部 Scope 以“省事”。
- 不声称启发式校验脚本等同于官方构建和设备测试。

## 官方资料

- 开发指南：https://developer.fnnas.com/docs/guide/
- 开放 API：https://developer.fnnas.com/api/overview/

文档可能更新。遇到版本相关、未收录字段或新接口时，重新查询官方页面并在结果中标注依据日期。
