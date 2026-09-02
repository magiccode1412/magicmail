---
name: cnb-self-hosted-runner
description: 排查自定义构建机（自托管 Runner）问题，速查平台架构、Docker 与调度限制
---

# CNB 自定义构建机

自定义构建机是 CNB 云原生构建的自托管 Runner 能力，允许将自有物理机/虚拟机接入平台作为构建节点。
适用于官方构建机无法覆盖的场景（如 iOS 构建、Windows 打包、Linux ARM 服务端）。

> **接入流程**（新增节点、执行连接指引脚本等一次性操作）请通过文档技能（cnb-docs）查询：
> - 根组织管理员自助接入：`/zh/build/build-node.md`（SaaS: `/zh/saas/build/build-node.md`，PaaS: `/zh/paas/build/build-node.md`）
> - 系统管理员接入：`/zh/paas/system-runner.md`
>
> 本 skill 聚焦接入后的**配置限制速查**与**问题排查**。

## 平台与架构支持

- **macOS (darwin)**：
  - arm64 (Apple Silicon)：✅ 根组织准入；Docker 支持 ❌（脚本直接在宿主机运行，不支持 Docker 构建环境）
  - x86_64 (Intel)：✅ 根组织准入；Docker 支持 ❌（同上）
- **Windows (win32)**：x86_64，✅ 根组织准入；Docker 支持 ❌（stage 默认走 PowerShell；不支持 Docker 构建环境）
- **Linux**：
  - arm64 / aarch64：✅ 根组织准入；Docker 支持 ✅*（取决于宿主机是否安装 Docker 及 runner 配置）
  - x86_64 / amd64：❌ 根组织暂不允许接入；如需此架构，走系统管理员途径

> \* Linux ARM 的 Docker 支持需宿主机已安装 Docker 且 `runner-config.json` 中 `useDocker: true`，否则按「无 Docker」处理。

## 关键限制速查

排查前先确认是否符合以下产品限制：

1. **不计入核时统计**：使用用户自有资源，不消耗平台核时额度。
2. **仅限云原生构建（CI）**：不能用于云原生开发（远程开发/VSCode）任务。
3. **无 Docker 构建环境的限制**（Mac/Windows，以及未装 Docker 的 Linux）：
   - 不能使用 Docker 镜像构建环境（`docker.image` / `docker.build` 不生效）
   - 代码走原生 `git` 克隆（非 `cnbcool/git-clone-yyds` 镜像），无 copy-on-write。
     并发时各自独立 clone，大仓库较慢（预期行为，非故障）
   - `runner.cpus` 配置无效，资源由宿主机决定，需自行管理工具链（Xcode、MSVC、brew 等）
   - 有 Docker 的 Linux 节点用 `cnbcool/git-clone-yyds` 镜像 + copy-on-write（默认 OverlayFS）实现秒级克隆
4. **网络要求**：构建机需能访问 CNB 站点（注册节点、上报心跳、拉取任务、上传日志/制品），内网机器需开通出网策略。
5. **节点离线时的行为**：流水线命中节点但节点不可用（离线/被禁用/资源达上限）时，系统自动尝试分配到其他可用节点；若无可用节点，流水线失败。

## 流水线调度配置

通过 `runner.namespace` + `runner.tags` 调度到自定义构建节点：

```yaml
main:
  push:
    - runner:
        namespace: group        # 使用根组织自托管节点（必需）
        tags:                   # 必需，"与"关系——列出的每个 tag 都需出现在节点标签上
          - mac                 # 节点标签为超集，例如 [mac, arm64, xcode15] 可被 [mac, arm64] 命中
          - arm64
      stages:
        - name: build
          script: uname -a
```

**配置要点**：
- **`runner.namespace: group`**：必需，表示使用当前仓库所属根组织下的自托管节点。
- **`runner.tags`**：必需，标签匹配为"与"关系（支持单值或列表，如 `tags: mac` 或 `tags: [mac, arm64]`）。
  - 节点标签 `[mac, arm64, xcode15]` 可被 `tags: [mac, arm64]` 命中。
  - ⚠️ **二者必须同时配置**：写了 `runner.namespace: group`，`runner.tags` 就不可省略，否则报 `RUNNER_TAGS_REQUIRED` 错误。
- **`runner.cpus`**：无 Docker 的构建机上无效；Linux ARM 需宿主机装 Docker 且 `useDocker: true` 才生效。

> 完整流水线语法（事件、stage、缓存等）参考流水线技能（cnb-pipeline）。

## 排查前：先识别 Runner 类型

排查前必须先分辨 Runner 的平台和 Docker 能力，不同环境的故障表现和排查路径差异较大。

> **判断以构建日志为主**，`.cnb.yml` 的 `runner.namespace` 只能区分系统/自定义构建机。
> 系统构建机也可能是 Mac/Windows，平台和 Docker 能力需从日志识别。

### 从日志识别平台和 Docker 构建环境

- `Platform detection results: darwin` → **Mac 构建机**：平台级 Docker 构建环境关闭（`docker.image` 不生效）；依赖宿主机工具链
- `Platform detection results: win32` → **Windows 构建机**：平台级 Docker 构建环境关闭；依赖宿主机工具链
- `Platform detection results: linux` + `Build with Docker? true` → **Linux 有 Docker**：容器化构建；`runner.cpus` 生效；用 `cnbcool/git-clone-yyds` 克隆
- `Platform detection results: linux` + `Build with Docker? false` → **Linux 构建机，无 Docker**：平台级 Docker 构建环境关闭；`runner.cpus` 无效；原生 `git` 克隆

**辅助信号**：
- **代码克隆日志**：出现 `cnbcool/git-clone-yyds` 或 OverlayFS 日志 → 有 Docker；直接走原生 `git clone` → 无 Docker。
- **stage 失败信息**：
  - `docker: command not found` 或 `docker.image` 不生效 → 无 Docker
  - cmd 内置命令（如 `ver`）找不到 → Windows 构建机（stage script 默认走 PowerShell，非 cmd）
  - `xcodebuild` 等工具找不到 → Mac 构建机，宿主机工具链缺失

> 注：日志中 `Runner[x.x.x.x] runs on ...` 后跟的是节点标签（labels），非平台名。判断平台看 `Platform detection results:`。

识别清楚后再对照下方「常见问题排查」对应章节定位。

## 常见问题排查

### 节点一直处于离线状态

1. **网络连通性**：确认构建机能访问 CNB 站点域名（HEAD 可能被网关拦截，建议用 GET）：
   ```bash
   # bash
   curl -sS -L -o /dev/null -w '%{http_code}\n' "${CNB_WEB_PROTOCOL:-https}://${CNB_WEB_HOST:-cnb.cool}"
   ```
   ```powershell
   # PowerShell
   (Invoke-WebRequest -UseBasicParsing -Uri ($env:CNB_WEB_PROTOCOL + '://' + $env:CNB_WEB_HOST)).StatusCode
   ```
   > 注：此步仅验证 HTTPS 出网；runner 实际走 `wss://` 反向接入通道，连通性需结合下方 Runner 日志确认。
2. **端口占用**：确认 8087、8088 端口未被占用且允许本机监听。
3. **查看 Runner 日志**（默认路径，Mac 因数据目录在用户家目录下而多一层 `orange-ci`）：
   - Linux: `tail -f /data/orange-runner/logs/app.log`
   - Mac: `tail -f $HOME/orange-ci/data/orange-runner/logs/app.log`
   - Windows: `Get-Content 'C:\data\orange-runner\logs\app.log' -Wait -Tail 10`
   - 路径不对时检查 `ORANGE_RUNNER_DATA_DIR`（数据目录，决定日志位置）与 `ORANGE_RUNNER_CONFIG_DIR`（配置证书目录）
4. **服务状态检查**：`orange-runner status` 查看服务运行状态。

### 流水线找不到可用节点

1. 检查 `.cnb.yml` 中 `tags` 是否与节点标签匹配（详见上方「流水线调度配置」）。
2. 确认节点状态为「在线」。
3. 确认 `namespace` 配置为 `group`（漏配会去官方节点池找）。
4. 检查节点是否被禁用或达到资源使用上限（CPU/内存/并发流水线数）。

### Windows 脚本执行报错

- **默认 shell 是 PowerShell**（实测为 Windows PowerShell 5.x，非 PowerShell 7，可用 `$PSVersionTable.PSVersion` 确认），不是 cmd。
  - `systeminfo`、`ipconfig` 等外部命令可直接调用；`ver`、`dir` 等 cmd 内置命令需用 `cmd /c` 包裹（如 `cmd /c ver`）。
- 脚本文件注意编码，避免隐藏 BOM 头导致解析错误。

### 根组织看不到构建节点入口

平台尚未对当前组织开放该能力。联系平台管理员在 `管理端 / 系统管理 / 云原生构建 / 自定义 Runner` 中开启「允许添加自定义 Runner」开关。

### 架构不支持的报错

```
SELF_HOSTED_RUNNER_ARCH_UNSUPPORTED: root organization's self-hosted Linux runner does not support architecture "x86_64"
```

根组织不支持 x86_64/amd64 Linux 接入。换成 arm64/aarch64 机器，或走系统管理员途径添加。

### Docker 构建环境不生效

排查 `pipeline.docker.image` / `docker.build` 不生效的问题：

- **Mac/Windows**：平台级 Docker 构建环境始终关闭，`docker.image`/`docker.build` 会被忽略。
- **Linux ARM**：检查宿主机是否安装 Docker 且 `runner-config.json` 中 `useDocker` 为 `true`（详见「平台与架构支持」注记）。

> 若用户询问“Mac/Windows 上能否用 Docker”：若宿主机装了 Docker Desktop，
> 在 stage 脚本里可直接调用 `docker run`/`docker build`（平台不拦截宿主机命令）。
> **但这是社区实践，非官方文档化用法**，行为和稳定性不保证，遇到问题需自行排查。

## 标签管理建议

- 建议语义化命名：`mac`/`windows`/`linux` + `arm64`/`x86_64` + 工具链（如 `xcode15`/`msvc2022`）。
- 同一标签可分给多个节点构成节点池，便于精确调度。

## 安全建议

- 自定义构建机代码运行在宿主机上（无容器隔离），请确保构建机环境安全可信。
- 通过 `runner-config.json` 的 `allowUseThisRunner` 字段限制可用节点，防止资源滥用。
  `groups` 与 `projects` 为「或」关系——命中任一即允许（`groups` 只支持根组织一级，`projects` 匹配仓库 slug）。
  - **两者均留空表示不限制（任何仓库/组织可用）**。需限制时填入具体标识，例如：
  ```json
  {
    "allowUseThisRunner": {
      "groups": ["my-root-group"],
      "projects": ["my-root-group/repo-a"]
    }
  }
  ```
  > ⚠️ 请务必将 `groups`/`projects` 中的占位标识替换为真实的根组织名与仓库 slug，
  > 否则节点不匹配任何仓库而被静默拒绝调度（表现为找不到节点）。
- `disableTls` 控制 runner 本地服务的 TLS/双向认证，**不影响与 CNB 站点的传输加密**。
  - 自托管构建机走反向接入，mTLS 自动关闭（本地服务仅监听 `127.0.0.1` 并用 loopback token 鉴权，
    与 CNB 通信走加密通道 `wss://`），无需显式配置。
  - mTLS 仅用于传统模式的系统构建机。
