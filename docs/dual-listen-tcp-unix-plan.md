# 计划：飞牛应用通过向导引导「仅网关 / 同时开端口」并控制监听模式

> ⚠️ **本计划已废弃（不再实施）**
>
> 飞牛形态改为**只监听 Unix Socket**：统一网关已完成 NAS 级认证，额外开放 TCP 端口等于
> 提供一个无保护的登录入口（且存在伪造 `X-Trim-Userid` 免密登录的高危路径）。
> 需要端口访问的场景改为部署**非飞牛版本**（Docker / `build.sh`）。
>
> 已回滚的改动：安装/配置向导的「访问方式」步骤、`wizard/config` 文件、
> `cmd/main` 中 `fnos_listen_mode` / `fnos_tcp_port` 的读取与 TCP 注入。
> 后端 `MAGICMAIL_TCP_ENABLED` / `MAGICMAIL_TCP_ADDR` 作为通用能力保留，但飞牛侧不再注入。
>
> 现行方案见 **`docs/gateway-prefix-strip-plan.md`**。以下为历史记录，仅供追溯。

> 目标：在保留飞牛统一网关 Unix Socket 能力的前提下，通过安装向导让用户**选择**是否同时监听 TCP 端口，
> 端口由用户在向导中设置（默认 `23232`，非必需，仅在上一步选了「同时开启端口」时才需要填写）。
> 监听模式完全由**环境变量**驱动，后端按环境变量决定「仅 Unix」还是「Unix + TCP」。

---

## 1. 现状分析

### 1.1 当前监听架构
后端 `server/main.go` 的 `listenAndServe()` 已实现 TCP / Unix 两种模式的**二选一**切换，
由环境变量 `MAGICMAIL_LISTEN` 决定：

| 值 | 行为 |
|---|---|
| 空 | 默认 `tcp://0.0.0.0:${MAGICMAIL_PORT}`（Docker / 旧部署） |
| `tcp://HOST:PORT` | 仅 TCP |
| `unix:///path/app.sock` | 仅 Unix Socket（飞牛 fnOS 安装包当前使用） |

飞牛包 `fnapp/cmd/main` 启动进程时固定注入 `MAGICMAIL_LISTEN="unix://${SOCK}"`，
因此**当前飞牛部署下只监听 Unix Socket，外部无法通过 TCP 直连**。

### 1.2 飞牛向导（wizard）机制
- 引导界面由 `fnapp/wizard/install`（安装引导）与 `fnapp/wizard/config`（安装后配置引导）的 JSON 定义。
- 控件通过 `field` 命名，安装/配置完成后由飞牛系统以 **`fnos_<field>` 环境变量** 注入到 `cmd/main` 等脚本。
  - 参照现有实现：`wizard/uninstall` 中 `field: "wizard_data_action"` → 脚本读取变量 `wizard_data_action`（见 `fnapp/cmd/uninstall_callback`）。
  - 即：`field` 值会原样成为环境变量名（本计划沿用同一约定，使用 `fnos_` 前缀语义由飞牛注入，脚本侧按 `fnos_<field>` 读取）。
- 支持的控件类型（参考 `uninstall` 示例）：`tips`、`radio`（单选）、`text`（文本输入）等；`radio` 用 `options` + `initValue` 设置默认。

### 1.3 关键事实
- 后端核心是单个 `*fiber.App` 实例，`app.Listener(ln)` 与 `app.Listen(addr)` 都可接受同一个 app。
- Fiber 支持一个 app 实例**多次 `Listen`/`Listener`**，即可以为同一个 app 同时绑定多个 listener。
- 路由已做前缀兼容（`MAGICMAIL_BASE_PATH=/app/magicmail` 与根路径双注册），TCP 直连时若不加前缀也能工作。
- 安全中间件 `GatewayIdentity()` 只在 `/api/v1/auth/fnos/*` 路由读取 `X-Trim-Userid`，
  TCP 端口伪造该 Header 无效，**TCP 与 Unix 共用同一套认证逻辑是安全的**。

### 1.4 待解决的核心问题
- 飞牛包当前硬编码「仅 Unix」，需改为由 **安装向导 + 环境变量** 控制。
- 安装向导需新增两步：① 选部署模式（仅网关 / 同时开端口）；②（条件显示）设置 TCP 端口。
- 后端 `listenAndServe` 是二选一，需改为「仅 Unix」与「Unix + TCP」两种由环境变量驱动的路径。
- `cmd/main` 需读取向导注入的 `fnos_listen_mode` / `fnos_tcp_port`，翻译成后端所需环境变量。

---

## 2. 部署模式控制设计（环境变量驱动）

### 2.1 新增向导字段
| field | 控件 | 说明 | 默认 |
|---|---|---|---|
| `listen_mode` | `radio` | 部署模式：`gateway`（仅统一网关） / `both`（同时开启端口） | `gateway` |
| `tcp_port` | `text` | TCP 监听端口，**非必需**；仅当 `listen_mode=both` 时需要填写 | `23232` |

飞牛注入后脚本侧读取的环境变量名为 `fnos_listen_mode` 与 `fnos_tcp_port`。

### 2.2 环境变量 → 监听模式映射
`cmd/main` 读取 `fnos_listen_mode` / `fnos_tcp_port`，翻译成后端环境变量：

| `fnos_listen_mode` | 注入后端的环境变量 | 后端行为 |
|---|---|---|
| `gateway`（默认） | `MAGICMAIL_LISTEN=unix://${SOCK}`，**不**设 `MAGICMAIL_TCP_ENABLED` | 仅 Unix Socket |
| `both` | `MAGICMAIL_LISTEN=unix://${SOCK}` + `MAGICMAIL_TCP_ENABLED=1` + `MAGICMAIL_TCP_ADDR=0.0.0.0:${fnos_tcp_port:-23232}` | Unix + TCP |

> 设计要点：
> - **默认 `gateway`**：与现有行为完全一致，向后兼容，未走向导或保持默认时仅 Unix。
> - **端口非必需**：`fnos_tcp_port` 为空时回退 `23232`；仅 `both` 模式才使用。
> - **单一事实来源**：所有监听逻辑完全由环境变量驱动，后端不感知「向导」，只认 `MAGICMAIL_LISTEN` / `MAGICMAIL_TCP_ENABLED` / `MAGICMAIL_TCP_ADDR`。

---

## 3. 改动方案

### 3.1 安装向导：`fnapp/wizard/install`

在现有「欢迎」步骤之后，新增一个步骤，包含：
1. `radio` 选择部署模式（默认 `gateway`）。
2. `text` 输入端口（默认 `23232`，非必需，提示仅选了「同时开启端口」才需填；条件显示由向导框架支持）。

目标 JSON 结构（示意）：

```json
[
  {
    "stepTitle": "欢迎安装 Magicmail",
    "items": [
      { "type": "tips", "helpText": "欢迎使用 Magicmail 魔法邮箱！..." },
      { "type": "tips", "helpText": "访问 <a target=\"_blank\" href=\"https://github.com/magiccode1412/magicmail\">GitHub</a> 了解更多信息。" }
    ]
  },
  {
    "stepTitle": "访问方式设置",
    "items": [
      {
        "type": "radio",
        "field": "listen_mode",
        "label": "访问方式",
        "initValue": "gateway",
        "options": [
          { "label": "仅使用飞牛统一网关（推荐）", "value": "gateway" },
          { "label": "同时开启 TCP 端口（支持局域网/外部直连）", "value": "both" }
        ],
        "rules": [ { "required": true, "message": "请选择访问方式" } ]
      },
      {
        "type": "text",
        "field": "tcp_port",
        "label": "TCP 监听端口",
        "initValue": "23232",
        "placeholder": "23232",
        "helpText": "仅在上方选择「同时开启 TCP 端口」时才需要配置。默认 23232，非必需。注意在飞牛防火墙/路由器放行该端口。",
        "rules": [ { "required": false, "pattern": "^[0-9]{1,5}$", "message": "端口需为 1-65535 的数字" } ]
      }
    ]
  }
]
```

> 说明：`tcp_port` 的「条件显示」（仅 `listen_mode=both` 时出现）依赖飞牛向导框架的联动能力；
> 若框架不支持条件显示，则 `tcp_port` 始终显示但标注「仅同时开启端口时生效」，并在 `cmd/main` 中仅 `both` 模式读取它。

### 3.2 安装后配置向导：`fnapp/wizard/config`

当前 `wizard/config` 仅一句 tips。建议同步加入同样的「访问方式」步骤（复用 §3.1 的 JSON 片段），
使安装后用户也能在飞牛应用中心「配置」里修改模式与端口，改完由 `fnapp/cmd/main` 的 `restart`/`config` 回调重新注入并重启。

### 3.3 启动脚本：`fnapp/cmd/main`

在 `start_process()` 注入环境变量处，改为读取向导变量：

```bash
# 部署模式（飞牛向导注入，默认仅网关）
LISTEN_MODE="${fnos_listen_mode:-gateway}"
TCP_PORT="${fnos_tcp_port:-23232}"

export MAGICMAIL_LISTEN="unix://${SOCK}"   # 始终启用 Unix Socket（网关必需）

if [ "${LISTEN_MODE}" = "both" ]; then
    export MAGICMAIL_TCP_ENABLED="1"
    export MAGICMAIL_TCP_ADDR="0.0.0.0:${TCP_PORT}"
else
    # 仅网关：不设置 TCP 相关变量（向后兼容纯 Unix 模式）
    unset MAGICMAIL_TCP_ENABLED MAGICMAIL_TCP_ADDR
fi
```

- 探活逻辑：保留 `[ -S "${SOCK}" ]` 作为主判活条件（网关依赖）；TCP 端口仅作可选辅助检测。
- `stop_process()` 中 `rm -f "${PID_FILE}" "${SOCK}"` 保持不变；TCP 随进程退出自动释放。
- 若 `listen_mode=both` 但 `tcp_port` 非法/为空，回退 `23232` 并在日志提示。

**安全提示（写入脚本注释 / 文档）**：
- `0.0.0.0` 暴露意味着局域网任意设备可访问 Web 登录页；认证仍由 JWT 保护。
- 建议在文档提醒：若仅在飞牛内网使用，可在后续把 `MAGICMAIL_TCP_ADDR` 改为 `127.0.0.1:PORT`（仅本机，配合反代/SSH 隧道）。

### 3.4 后端：`server/config/config.go`

`ServerConfig` 新增字段（与 §2.2 对应）：

```go
type ServerConfig struct {
    Port       int
    Host       string
    Listen     string
    TCPEnabled bool   // 由 MAGICMAIL_TCP_ENABLED 控制（both 模式为 true）
    TCPAddr    string // 由 MAGICMAIL_TCP_ADDR 控制，如 0.0.0.0:23232
    BasePath   string
}
```

`Load()` 中：

```go
tcpEnabled := getEnvBool("MAGICMAIL_TCP_ENABLED", false)
tcpAddr := getEnv("MAGICMAIL_TCP_ADDR", "")
```

- Docker / 旧部署：不设 `MAGICMAIL_TCP_ENABLED` → 沿用现有 `Listen` 逻辑（向后兼容，零改动）。
- 飞牛 `both` 模式：`MAGICMAIL_TCP_ENABLED=1` + `MAGICMAIL_TCP_ADDR=0.0.0.0:23232`。

### 3.5 后端：`server/main.go`

将 `listenAndServe` 改造为「主监听（Unix 阻塞）+ 可选 TCP 并行」：

```go
func listenAndServe(app *fiber.App, cfg *config.Config) error {
    // 主监听：始终存在（Unix 或 TCP），沿用现有逻辑并抽出 buildListener
    mainLn, err := buildListener(cfg.Server.Listen)
    if err != nil {
        return err
    }
    go app.Listener(mainLn) // 主监听器异步运行

    // 若启用 TCP 并行监听（both 模式），再开一个 goroutine
    if cfg.Server.TCPEnabled {
        addr := cfg.Server.TCPAddr
        if addr == "" {
            addr = cfg.Server.Host + ":" + strconv.Itoa(cfg.Server.Port)
        }
        go func() {
            log.Printf("🚀 Magicmail TCP 服务启动于 http://%s", addr)
            if err := app.Listen(addr); err != nil {
                log.Printf("⚠️  TCP 监听失败: %v", err)
            }
        }()
    }

    // 主协程阻塞在主 listener 上（进程生命周期由它决定）
    return app.Listener(mainLn) // 实际改为上面的 go + 下面阻塞其中一个
}
```

> 实现细节：Fiber 的 `app.Listener(ln)` 会阻塞直到 listener 关闭。建议主监听（Unix）阻塞主协程，
> TCP 放 `go app.Listen(addr)`；若主监听是 TCP，则反过来。进程生命周期由主监听决定，符合飞牛现状。
> Unix 的「启动前删旧文件 / 退出 defer 删 / 权限 0666」逻辑抽进 `buildListener` 复用。

### 3.6 清单/打包与文档

**`fnapp/manifest`**
- 当前 `checkport = false`（飞牛网关模式，不检查端口）。
- 端口由向导自定义（`both` 模式），无法在 manifest 写死 → **保持 `checkport=false`**，TCP 直连作为「隐藏能力」，由用户自行在防火墙放行。
- `changelog` 说明「新增安装向导：可选同时开启 TCP 端口（默认 23232）」。

**`docs/` 新增使用说明**
- 飞牛模式下 TCP 直连用法：端口默认 `23232`（或向导所设）。
- 访问：`http://<飞牛IP>:<PORT>/app/magicmail`（带前缀）。
- 防火墙放行示例（复用 `deploy.sh` 中已有的 `ufw` / `firewall-cmd` 提示）。

### 3.7 前端：无需结构性改动
- 前端 `BASE_URL` 由网关注入（`/app/magicmail`），经网关访问不变。
- TCP 直连时访问 `http://IP:PORT/app/magicmail`（带前缀）即可复用现有嵌入 SPA 与 API 基址，前端零改动。

---

## 4. 改动文件清单

| 文件 | 改动类型 | 说明 |
|---|---|---|
| `fnapp/wizard/install` | 修改 | 新增「访问方式设置」步骤：`listen_mode` 单选（默认 gateway）+ `tcp_port` 文本（默认 23232，非必需） |
| `fnapp/wizard/config` | 修改 | 同步加入「访问方式设置」步骤（安装后也可配置） |
| `fnapp/cmd/main` | 修改 | 读取 `fnos_listen_mode` / `fnos_tcp_port` → 注入 `MAGICMAIL_LISTEN` / `MAGICMAIL_TCP_ENABLED` / `MAGICMAIL_TCP_ADDR` |
| `server/config/config.go` | 修改 | 新增 `TCPEnabled` / `TCPAddr`，`Load()` 读取对应环境变量 |
| `server/main.go` | 修改 | `listenAndServe` 改为「主监听阻塞 + TCP 并行 goroutine」，抽出 `buildListener` |
| `fnapp/manifest` | 修改（小） | changelog 说明；`checkport` 维持 `false` |
| `docs/` 文档 | 新增/修改 | 补充飞牛向导选择 + TCP 直连用法、防火墙放行 |

---

## 5. 实施步骤（建议顺序）

1. **`config.go`**：加字段 + `Load()` 解析 `MAGICMAIL_TCP_ENABLED` / `MAGICMAIL_TCP_ADDR`。
2. **`main.go`**：重构 `listenAndServe`，抽出 `buildListener`；主监听阻塞，TCP 并行 `go app.Listen`。
3. **本地验证**：`MAGICMAIL_LISTEN=unix:///tmp/t.sock MAGICMAIL_TCP_ENABLED=1 MAGICMAIL_TCP_ADDR=127.0.0.1:23232 ./magicmail`，
   确认 Unix socket 与 `curl http://127.0.0.1:23232/` 均 200。
4. **`fnapp/wizard/install` + `wizard/config`**：加入「访问方式设置」步骤。
5. **`fnapp/cmd/main`**：读取 `fnos_listen_mode` / `fnos_tcp_port`，按 §2.2 注入环境变量（默认 gateway / 23232）。
6. **`fnapp/manifest`** + **文档**：说明向导能力与端口默认 23232、防火墙放行。
7. **构建验证**：`build_fpk.sh` 打包，在 fnOS 测试：① 默认安装仅网关；② 选 `both` 后可 TCP 直连指定端口。

---

## 6. 风险与回滚

- **风险 1（端口冲突）**：用户设的端口可能被占用 → 提示 1-65535，运行时监听失败仅记日志，不影响网关。
- **风险 2（安全暴露）**：`0.0.0.0` 暴露登录页 → 文档提示，并说明 `127.0.0.1` 仅本机模式。
- **风险 3（向导条件显示）**：若飞牛框架不支持 `tcp_port` 仅在 `both` 时显示，则始终显示但标注「仅同时开启端口时生效」，`cmd/main` 仅 `both` 读取。
- **风险 4（Fiber 多 listener 稳定性）**：单 app 多 `Listen` 已被社区验证可行，需压测确认。
- **回滚**：向导选回 `gateway`（或不设）→ `MAGICMAIL_TCP_ENABLED` 不设置 → 回到纯 Unix 模式，后端代码向后兼容，零残留。

---

## 7. 验证清单

- [ ] 向导默认进入「仅网关」，安装后仅 Unix Socket 监听，行为不变。
- [ ] 向导选「同时开启端口」并设端口（如 23232）→ 进程同时监听 Unix + TCP:23232。
- [ ] `tcp_port` 留空时回退 23232；填非法值时提示或回退。
- [ ] 飞牛网关内访问 `/app/magicmail` 正常（Unix Socket）。
- [ ] 局域网 `http://<IP>:<PORT>/app/magicmail` 可登录使用（TCP）。
- [ ] 进程退出后 Unix Socket 文件被清理、TCP 端口释放。
- [ ] Docker 部署行为不变（仍仅 TCP 8080，不受向导变量影响）。
- [ ] 安装后「配置」向导修改模式/端口并重启生效。

---

## 8. 实施记录（已落地）

以下改动已在本次执行中完成：

| 文件 | 改动 |
|---|---|
| `server/config/config.go` | `ServerConfig` 新增 `TCPEnabled` / `TCPAddr`；`Load()` 解析 `MAGICMAIL_TCP_ENABLED` / `MAGICMAIL_TCP_ADDR`（gofmt 已格式化，config 包编译通过） |
| `server/main.go` | 抽出 `buildListener()`（Unix/TCP 统一建监听）；`listenAndServe()` 改为「主监听阻塞 + TCP 并行 goroutine」；新增 `unixSocketPath()` 辅助；进程退出 defer 清理 socket |
| `fnapp/cmd/main` | 读取 `fnos_listen_mode`（默认 gateway）/ `fnos_tcp_port`（默认 23232，非法回退）；`gateway` 仅 Unix，`both` 额外注入 `MAGICMAIL_TCP_ENABLED=1` + `MAGICMAIL_TCP_ADDR=0.0.0.0:PORT`；诊断日志补充模式与端口 |
| `fnapp/wizard/install` | 新增「访问方式设置」步骤：`listen_mode` radio（默认 gateway）+ `tcp_port` text（默认 23232，非必需，正则校验） |
| `fnapp/wizard/config` | 同步新增「访问方式设置」步骤（安装后亦可配置） |
| `fnapp/cmd/config_callback` | 原空操作改为配置变更后 `stop`+`start` 重启，使新模式/端口生效 |
| `fnapp/manifest` | changelog 说明向导新增「访问方式」选择 |

**验证说明**：`go build ./...` 仍报 `embedfs/embed.go: pattern all:dist` 错误，该错误源于前端 `dist` 目录未构建（embed 硬依赖），属预存问题，将由 `build_fpk.sh` 正常流程自动解决，与本次改动无关；`config` 包已独立编译通过，`main.go`/`config.go` 经 `gofmt` 与 `go vet` 校验无新增问题。

**待人工验证（需真实 fnOS / 构建前端后）**：
1. `build_fpk.sh` 打包并在 fnOS 安装，确认默认走纯 Unix 网关。
2. 向导选 `both` + 端口，确认进程同时监听 Unix + TCP，局域网 `http://<IP>:<PORT>/app/magicmail` 可登录。
3. 安装后「配置」向导改模式/端口，确认 `config_callback` 重启生效。
